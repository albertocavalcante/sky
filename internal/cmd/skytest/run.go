package skytest

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/albertocavalcante/sky/internal/starlark/coverage"
	"github.com/albertocavalcante/sky/internal/starlark/tester"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK     = 0
	exitFailed = 1
	exitError  = 2
)

// stringSliceFlag allows a flag to be specified multiple times.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Run executes skytest with the given arguments.
// Returns exit code.
func Run(args []string) int {
	return RunWithIO(context.Background(), args, os.Stdin, os.Stdout, os.Stderr)
}

// RunWithIO allows custom IO for embedding/testing.
func RunWithIO(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		jsonFlag      bool
		junitFlag     bool
		versionFlag   bool
		verboseFlag   bool
		recursiveFlag bool
		prefixFlag    string
		durationFlag  bool
		coverageFlag  bool
		coverageOut   string
		filterFlag    string
		markerFilter  string
		preludeFlags  stringSliceFlag
		timeoutFlag   time.Duration
		bailFlag      bool
		bailShortFlag bool
	)

	fs := flag.NewFlagSet("skytest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&jsonFlag, "json", false, "output results as JSON")
	fs.BoolVar(&junitFlag, "junit", false, "output results as JUnit XML")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")
	fs.BoolVar(&verboseFlag, "v", false, "verbose output")
	fs.BoolVar(&recursiveFlag, "r", false, "search directories recursively")
	fs.StringVar(&prefixFlag, "prefix", "test_", "test function prefix")
	fs.BoolVar(&durationFlag, "duration", false, "show test durations")
	fs.StringVar(&filterFlag, "k", "", "filter tests by name pattern (supports 'not' prefix)")
	fs.StringVar(&markerFilter, "m", "", "filter tests by marker (supports 'not' prefix, e.g., '-m slow', '-m \"not slow\"')")
	fs.Var(&preludeFlags, "prelude", "prelude file to load before tests (can be specified multiple times)")
	fs.DurationVar(&timeoutFlag, "timeout", 30*time.Second, "timeout per test (0 to disable)")
	fs.BoolVar(&bailFlag, "bail", false, "stop on first test failure")
	fs.BoolVar(&bailShortFlag, "x", false, "stop on first test failure (short for --bail)")
	// EXPERIMENTAL: Coverage collection requires starlark-go-x with OnExec hook.
	// Uncomment the replace directive in go.mod to enable.
	// TODO(upstream): Remove experimental note once OnExec is merged.
	fs.BoolVar(&coverageFlag, "coverage", false, "collect coverage data (EXPERIMENTAL)")
	fs.StringVar(&coverageOut, "coverprofile", "coverage.json", "coverage output file")

	fs.Usage = func() {
		writeln(stderr, "Usage: skytest [flags] <paths...>")
		writeln(stderr)
		writeln(stderr, "Starlark test runner.")
		writeln(stderr)
		writeln(stderr, "Discovers and runs test functions in Starlark files.")
		writeln(stderr, "Test files match: *_test.star, test_*.star")
		writeln(stderr, "Test functions match: test_* prefix (configurable)")
		writeln(stderr)
		writeln(stderr, "Features:")
		writeln(stderr, "  - Built-in assert module (assert.eq, assert.true, etc.)")
		writeln(stderr, "  - Per-file setup() and teardown() functions")
		writeln(stderr, "  - Multiple output formats (text, JSON, JUnit)")
		writeln(stderr, "  - Test filtering with -k flag")
		writeln(stderr, "  - Prelude files for shared helpers (--prelude)")
		writeln(stderr, "  - Per-test timeouts (--timeout)")
		writeln(stderr, "  - Fail-fast mode (--bail / -x)")
		writeln(stderr, "  - Coverage collection (EXPERIMENTAL, requires starlark-go-x)")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skytest .                       # Run tests in current directory")
		writeln(stderr, "  skytest -r .                    # Run tests recursively")
		writeln(stderr, "  skytest test.star               # Run specific test file")
		writeln(stderr, "  skytest -k parse                # Run tests containing 'parse'")
		writeln(stderr, "  skytest -k 'not slow'           # Exclude tests containing 'slow'")
		writeln(stderr, "  skytest test.star::test_foo     # Run specific test function")
		writeln(stderr, "  skytest --prelude=helpers.star  # Load prelude before tests")
		writeln(stderr, "  skytest --timeout=10s           # Set test timeout")
		writeln(stderr, "  skytest --timeout=0             # Disable timeouts")
		writeln(stderr, "  skytest --bail                  # Stop on first failure")
		writeln(stderr, "  skytest -x                      # Stop on first failure (short)")
		writeln(stderr, "  skytest -json tests/            # JSON output")
		writeln(stderr, "  skytest -junit tests/ > out.xml # JUnit output for CI")
		writeln(stderr)
		writeln(stderr, "Assert module:")
		writeln(stderr, "  assert.eq(a, b, msg=None)       # Assert a == b")
		writeln(stderr, "  assert.ne(a, b, msg=None)       # Assert a != b")
		writeln(stderr, "  assert.true(cond, msg=None)     # Assert cond is truthy")
		writeln(stderr, "  assert.false(cond, msg=None)    # Assert cond is falsy")
		writeln(stderr, "  assert.contains(c, item)        # Assert item in c")
		writeln(stderr, "  assert.fails(fn, pattern=None)  # Assert fn() raises error")
		writeln(stderr, "  assert.lt(a, b), assert.le(a, b), assert.gt(a, b), assert.ge(a, b)")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skytest %s\n", version.String())
		return exitOK
	}

	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Parse paths for :: syntax (file::test_name)
	// fileTestNames maps file paths to specific test names
	fileTestNames := make(map[string][]string)
	var cleanPaths []string
	for _, p := range paths {
		if idx := strings.Index(p, "::"); idx != -1 {
			filePath := p[:idx]
			testName := p[idx+2:]
			fileTestNames[filePath] = append(fileTestNames[filePath], testName)
			cleanPaths = append(cleanPaths, filePath)
		} else {
			cleanPaths = append(cleanPaths, p)
		}
	}

	// Discover test files
	files, err := tester.ExpandPaths(cleanPaths, nil, recursiveFlag)
	if err != nil {
		writef(stderr, "skytest: %v\n", err)
		return exitError
	}

	if len(files) == 0 {
		writeln(stderr, "skytest: no test files found")
		return exitError
	}

	// Create base options for runners
	opts := tester.DefaultOptions()
	opts.TestPrefix = prefixFlag
	opts.Verbose = verboseFlag
	opts.Coverage = coverageFlag
	opts.Filter = filterFlag
	opts.MarkerFilter = markerFilter
	opts.Preludes = preludeFlags
	opts.Timeout = timeoutFlag
	opts.FailFast = bailFlag || bailShortFlag

	// Create a single runner for coverage reporting (if enabled)
	// Note: We create per-file runners for execution to support :: syntax,
	// but use a single runner to aggregate coverage data.
	var coverageRunner *tester.Runner
	if coverageFlag {
		coverageRunner = tester.New(opts)
	}

	// Select reporter
	var reporter tester.Reporter
	switch {
	case jsonFlag:
		reporter = &tester.JSONReporter{}
	case junitFlag:
		reporter = &tester.JUnitReporter{}
	default:
		reporter = &tester.TextReporter{
			Verbose:      verboseFlag,
			ShowDuration: durationFlag,
		}
	}

	// Run tests
	result := &tester.RunResult{}
	start := time.Now()

	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			writef(stderr, "skytest: %v\n", err)
			return exitError
		}

		// Convert to absolute path for clearer output
		absPath, _ := filepath.Abs(file)
		if absPath == "" {
			absPath = file
		}

		// Check if this file has specific test names from :: syntax
		// We need to check both the original path and the absolute path
		var testNames []string
		for origPath, names := range fileTestNames {
			origAbs, _ := filepath.Abs(origPath)
			if origPath == file || origAbs == absPath {
				testNames = names
				break
			}
		}

		// Create a runner with the appropriate test names for this file
		fileOpts := opts
		fileOpts.TestNames = testNames
		fileRunner := tester.New(fileOpts)

		fileResult, err := fileRunner.RunFile(absPath, src)
		if err != nil {
			writef(stderr, "skytest: %s: %v\n", file, err)
			return exitError
		}

		result.Files = append(result.Files, *fileResult)

		// Report file immediately in text mode
		if _, ok := reporter.(*tester.TextReporter); ok {
			reporter.ReportFile(stdout, fileResult)
		}

		// Fail-fast: stop processing more files after first failure
		if opts.FailFast && fileResult.HasFailures() {
			break
		}
	}

	result.Duration = time.Since(start)

	// Report summary
	reporter.ReportSummary(stdout, result)

	// Write coverage output if enabled
	// EXPERIMENTAL: Coverage collection requires starlark-go-x with OnExec hook.
	// TODO(upstream): Remove experimental note once OnExec is merged.
	if coverageFlag && coverageRunner != nil {
		if err := writeCoverageReport(coverageRunner, coverageOut, stderr); err != nil {
			writef(stderr, "skytest: coverage: %v\n", err)
			// Don't fail the run for coverage errors, just warn
		}
	}

	if result.HasFailures() {
		return exitFailed
	}
	return exitOK
}

// writeCoverageReport writes the coverage data to a JSON file.
// EXPERIMENTAL: Coverage data is only collected when starlark-go-x OnExec hook is enabled.
func writeCoverageReport(runner *tester.Runner, outPath string, stderr io.Writer) error {
	report := runner.CoverageReport()
	if report == nil {
		writeln(stderr, "skytest: coverage: no data collected (starlark-go-x OnExec hook not enabled)")
		writeln(stderr, "         To enable, uncomment the replace directive in go.mod:")
		writeln(stderr, "         replace go.starlark.net => ../starlark-go-x/coverage-hooks")
		return nil
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(coverageJSON(report), "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling coverage: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	writef(stderr, "skytest: coverage written to %s\n", outPath)
	return nil
}

// coverageJSONOutput represents the top-level JSON coverage output.
// Uses snake_case keys for consistency with internal/starlark/coverage/reporter.go.
type coverageJSONOutput struct {
	Files        map[string]coverageFileJSON `json:"files"`
	TotalLines   int                         `json:"total_lines"`
	CoveredLines int                         `json:"covered_lines"`
	Percentage   float64                     `json:"percentage"`
}

// coverageFileJSON represents per-file coverage data in JSON output.
type coverageFileJSON struct {
	Lines map[int]int `json:"lines"`
}

// coverageJSON converts a coverage.Report to a JSON-serializable structure.
func coverageJSON(r *coverage.Report) coverageJSONOutput {
	files := make(map[string]coverageFileJSON)
	for path, fc := range r.Files {
		files[path] = coverageFileJSON{
			Lines: fc.Lines.Hits,
		}
	}
	return coverageJSONOutput{
		Files:        files,
		TotalLines:   r.TotalLines,
		CoveredLines: r.CoveredLines,
		Percentage:   r.Percentage(),
	}
}

// Helper functions for writing output.
// Write errors are intentionally ignored because:
//  1. These functions write to stdout/stderr where there's no reasonable recovery
//     if the terminal/pipe is broken (EPIPE, etc.)
//  2. If we can't write error messages, we can't report the write failure either
//  3. The exit code still reflects the actual operation status
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
