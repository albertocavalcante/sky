package skytest

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/albertocavalcante/sky/internal/skyconfig"
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
		jsonFlag            bool
		junitFlag           bool
		versionFlag         bool
		verboseFlag         bool
		recursiveFlag       bool
		prefixFlag          string
		durationFlag        bool
		coverageFlag        bool
		coverageOut         string
		filterFlag          string
		markerFilter        string
		preludeFlags        stringSliceFlag
		timeoutFlag         time.Duration
		bailFlag            bool
		bailShortFlag       bool
		updateSnapshotsFlag bool
		watchFlag           bool
		affectedOnlyFlag    bool
		parallelFlag        string
		configFlag          string
		configTimeoutFlag   time.Duration
	)

	fs := flag.NewFlagSet("skytest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&jsonFlag, "json", false, "output results as JSON")
	fs.BoolVar(&junitFlag, "junit", false, "output results as JUnit XML")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")
	fs.BoolVar(&verboseFlag, "v", false, "verbose output")
	fs.BoolVar(&recursiveFlag, "r", false, "search directories recursively")
	fs.StringVar(&prefixFlag, "prefix", "", "test function prefix (default: from config or test_)")
	fs.BoolVar(&durationFlag, "duration", false, "show test durations")
	fs.StringVar(&filterFlag, "k", "", "filter tests by name pattern (supports 'not' prefix)")
	fs.StringVar(&markerFilter, "m", "", "filter tests by marker (supports 'not' prefix, e.g., '-m slow', '-m \"not slow\"')")
	fs.Var(&preludeFlags, "prelude", "prelude file to load before tests (can be specified multiple times)")
	fs.DurationVar(&timeoutFlag, "timeout", 0, "timeout per test (0 to use config default)")
	fs.BoolVar(&bailFlag, "bail", false, "stop on first test failure")
	fs.BoolVar(&bailShortFlag, "x", false, "stop on first test failure (short for --bail)")
	// EXPERIMENTAL: Coverage collection requires starlark-go-x with OnExec hook.
	// Uncomment the replace directive in go.mod to enable.
	// TODO(upstream): Remove experimental note once OnExec is merged.
	fs.BoolVar(&coverageFlag, "coverage", false, "collect coverage data (EXPERIMENTAL)")
	fs.StringVar(&coverageOut, "coverprofile", "", "coverage output file (default: from config or coverage.json)")
	fs.BoolVar(&updateSnapshotsFlag, "update-snapshots", false, "update snapshots instead of comparing")
	fs.BoolVar(&updateSnapshotsFlag, "u", false, "update snapshots (short for --update-snapshots)")
	fs.BoolVar(&watchFlag, "watch", false, "watch for file changes and re-run tests")
	fs.BoolVar(&watchFlag, "w", false, "watch mode (short for --watch)")
	fs.BoolVar(&affectedOnlyFlag, "affected-only", false, "in watch mode, only run tests affected by changes")
	fs.StringVar(&parallelFlag, "j", "", "number of parallel test files (auto, 1-N)")
	fs.StringVar(&configFlag, "config", "", "config file path (config.sky, sky.star, or sky.toml)")
	fs.DurationVar(&configTimeoutFlag, "config-timeout", skyconfig.DefaultStarlarkTimeout, "timeout for Starlark config execution")

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
		writeln(stderr, "  - Parallel test execution (-j)")
		writeln(stderr, "  - Watch mode for continuous testing (--watch / -w)")
		writeln(stderr, "  - Coverage collection (EXPERIMENTAL, requires starlark-go-x)")
		writeln(stderr, "  - Unified configuration via config.sky, sky.star, or sky.toml")
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
		writeln(stderr, "  skytest --watch tests/          # Watch mode, re-run on changes")
		writeln(stderr, "  skytest -w --affected-only .    # Watch, only run affected tests")
		writeln(stderr, "  skytest -j auto tests/          # Run tests in parallel (auto-detect CPUs)")
		writeln(stderr, "  skytest -j 4 tests/             # Run tests with 4 parallel workers")
		writeln(stderr, "  skytest --config=config.sky     # Use specific config file")
		writeln(stderr, "  SKY_CONFIG=path/to/config.sky   # Config via environment variable")
		writeln(stderr)
		writeln(stderr, "Configuration:")
		writeln(stderr, "  Config resolution order:")
		writeln(stderr, "    1. --config flag (if specified)")
		writeln(stderr, "    2. SKY_CONFIG environment variable (if set)")
		writeln(stderr, "    3. Walk up directories looking for: config.sky, sky.star, sky.toml")
		writeln(stderr)
		writeln(stderr, "  Only one config file may exist per directory. CLI flags override config.")
		writeln(stderr)
		writeln(stderr, "  sky.toml example:")
		writeln(stderr, "    [test]")
		writeln(stderr, "    timeout = \"60s\"")
		writeln(stderr, "    parallel = \"auto\"")
		writeln(stderr, "    prelude = [\"test/helpers.star\"]")
		writeln(stderr)
		writeln(stderr, "  config.sky example (dynamic):")
		writeln(stderr, "    def configure():")
		writeln(stderr, "        ci = getenv(\"CI\", \"\") != \"\"")
		writeln(stderr, "        return {")
		writeln(stderr, "            \"test\": {")
		writeln(stderr, "                \"timeout\": \"120s\" if ci else \"30s\",")
		writeln(stderr, "                \"parallel\": \"1\" if ci else \"auto\",")
		writeln(stderr, "            },")
		writeln(stderr, "        }")
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

	// Load configuration (config file provides defaults, CLI overrides)
	var cfg *skyconfig.Config
	if configFlag != "" {
		// Explicit config file specified
		var err error
		ext := filepath.Ext(configFlag)
		if ext == ".star" || ext == ".sky" {
			cfg, err = skyconfig.LoadStarlarkConfig(configFlag, configTimeoutFlag)
		} else {
			cfg, err = skyconfig.LoadConfig(configFlag)
		}
		if err != nil {
			writef(stderr, "skytest: loading config %s: %v\n", configFlag, err)
			return exitError
		}
	} else {
		// Auto-discover config
		var configPath string
		var err error
		cfg, configPath, err = skyconfig.DiscoverConfig("")
		if err != nil {
			writef(stderr, "skytest: %v\n", err)
			return exitError
		}
		if configPath != "" && verboseFlag {
			writef(stderr, "skytest: using config %s\n", configPath)
		}
	}

	// Apply config defaults, then CLI overrides
	// Timeout: CLI > config > default (30s)
	effectiveTimeout := cfg.Test.Timeout.Duration
	if effectiveTimeout == 0 {
		effectiveTimeout = 30 * time.Second
	}
	if timeoutFlag != 0 {
		effectiveTimeout = timeoutFlag
	}

	// Parallel: CLI > config > default (empty = sequential)
	effectiveParallel := cfg.Test.Parallel
	if parallelFlag != "" {
		effectiveParallel = parallelFlag
	}

	// Prefix: CLI > config > default (test_)
	effectivePrefix := cfg.Test.Prefix
	if effectivePrefix == "" {
		effectivePrefix = "test_"
	}
	if prefixFlag != "" {
		effectivePrefix = prefixFlag
	}

	// Prelude: config + CLI (additive)
	effectivePreludes := append([]string{}, cfg.Test.Prelude...)
	effectivePreludes = append(effectivePreludes, preludeFlags...)

	// FailFast: CLI > config
	effectiveFailFast := cfg.Test.FailFast || bailFlag || bailShortFlag

	// Verbose: CLI > config
	effectiveVerbose := cfg.Test.Verbose || verboseFlag

	// Coverage: CLI > config
	effectiveCoverage := cfg.Test.Coverage.Enabled || coverageFlag

	// Coverage output: CLI > config > default
	effectiveCoverageOut := cfg.Test.Coverage.Output
	if effectiveCoverageOut == "" {
		effectiveCoverageOut = "coverage.json"
	}
	if coverageOut != "" {
		effectiveCoverageOut = coverageOut
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
	opts.TestPrefix = effectivePrefix
	opts.Verbose = effectiveVerbose
	opts.Coverage = effectiveCoverage
	opts.Filter = filterFlag
	opts.MarkerFilter = markerFilter
	opts.Preludes = effectivePreludes
	opts.Timeout = effectiveTimeout
	opts.FailFast = effectiveFailFast
	opts.UpdateSnapshots = updateSnapshotsFlag

	// Create a single runner for coverage reporting (if enabled)
	// Note: We create per-file runners for execution to support :: syntax,
	// but use a single runner to aggregate coverage data.
	var coverageRunner *tester.Runner
	if effectiveCoverage {
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
			Verbose:      effectiveVerbose,
			ShowDuration: durationFlag,
		}
	}

	// Watch mode
	if watchFlag {
		return runWatchMode(files, opts, fileTestNames, reporter, affectedOnlyFlag, stdout, stderr)
	}

	// Determine parallelism level
	workers := parseParallelism(effectiveParallel)

	// Run tests (parallel or sequential)
	var result *tester.RunResult
	var runErr error
	if workers > 1 && len(files) > 1 {
		result, runErr = runParallel(files, workers, opts, fileTestNames, reporter, stdout, stderr)
	} else {
		result, runErr = runSequential(files, opts, fileTestNames, reporter, stdout, stderr)
	}

	if runErr != nil {
		writef(stderr, "skytest: %v\n", runErr)
		return exitError
	}

	// Report summary
	reporter.ReportSummary(stdout, result)

	// Write coverage output if enabled
	// EXPERIMENTAL: Coverage collection requires starlark-go-x with OnExec hook.
	// TODO(upstream): Remove experimental note once OnExec is merged.
	if effectiveCoverage && coverageRunner != nil {
		if err := writeCoverageReport(coverageRunner, effectiveCoverageOut, stderr); err != nil {
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

// runWatchMode runs tests in watch mode, re-running on file changes.
func runWatchMode(
	files []string,
	opts tester.Options,
	fileTestNames map[string][]string,
	reporter tester.Reporter,
	affectedOnly bool,
	stdout, stderr io.Writer,
) int {
	// Get root directory for watching
	cwd, err := os.Getwd()
	if err != nil {
		writef(stderr, "skytest: getting working directory: %v\n", err)
		return exitError
	}

	// Create watcher
	watcher, err := tester.NewWatcher(cwd)
	if err != nil {
		writef(stderr, "skytest: creating watcher: %v\n", err)
		return exitError
	}
	defer func() { _ = watcher.Close() }()

	// Add all test files to the watcher
	for _, file := range files {
		if err := watcher.Add(file); err != nil {
			writef(stderr, "skytest: watching %s: %v\n", file, err)
		}
	}

	writef(stdout, "\n🔍 Watch mode active. Watching %d test file(s).\n", len(files))
	writef(stdout, "   Press Ctrl+C to stop.\n\n")

	// Run initial tests
	runTests(files, opts, fileTestNames, reporter, stdout, stderr)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Watch for changes
	for {
		select {
		case <-sigCh:
			writef(stdout, "\n\n👋 Stopping watch mode.\n")
			return exitOK

		case event := <-watcher.Events:
			// Clear screen for fresh output
			writef(stdout, "\033[2J\033[H") // ANSI escape to clear screen and move cursor home

			writef(stdout, "📝 File changed: %s\n\n", filepath.Base(event.File))

			// Determine which files to run
			var filesToRun []string
			if affectedOnly {
				filesToRun = event.AffectedTests
			} else {
				filesToRun = files
			}

			if len(filesToRun) == 0 {
				writef(stdout, "No affected tests to run.\n")
				continue
			}

			// Refresh dependencies for changed file (in case loads changed)
			if err := watcher.RefreshDependencies(event.File); err != nil {
				writef(stderr, "skytest: refreshing dependencies: %v\n", err)
			}

			runTests(filesToRun, opts, fileTestNames, reporter, stdout, stderr)

			writef(stdout, "\n🔍 Watching for changes...\n")

		case err := <-watcher.Errors:
			writef(stderr, "skytest: watcher error: %v\n", err)
		}
	}
}

// runTests runs the given test files and reports results.
func runTests(
	files []string,
	opts tester.Options,
	fileTestNames map[string][]string,
	reporter tester.Reporter,
	stdout, stderr io.Writer,
) {
	result := &tester.RunResult{}
	start := time.Now()

	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			writef(stderr, "skytest: %v\n", err)
			continue
		}

		// Convert to absolute path for clearer output
		absPath, _ := filepath.Abs(file)
		if absPath == "" {
			absPath = file
		}

		// Check if this file has specific test names from :: syntax
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
			continue
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
}

// parseParallelism parses the -j flag value and returns the number of workers.
// Returns 1 for sequential execution (empty, "1", invalid values).
// Returns runtime.NumCPU() for "auto".
// Returns the parsed number for valid numeric values > 0.
func parseParallelism(flag string) int {
	if flag == "" || flag == "1" {
		return 1
	}
	if strings.EqualFold(flag, "auto") {
		return runtime.NumCPU()
	}
	n, err := strconv.Atoi(flag)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// runSequential runs test files sequentially.
// Returns an error if a file cannot be read or has a syntax error.
func runSequential(
	files []string,
	opts tester.Options,
	fileTestNames map[string][]string,
	reporter tester.Reporter,
	stdout, _ io.Writer,
) (*tester.RunResult, error) {
	result := &tester.RunResult{}
	start := time.Now()

	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", file, err)
		}

		// Convert to absolute path for clearer output
		absPath, _ := filepath.Abs(file)
		if absPath == "" {
			absPath = file
		}

		// Check if this file has specific test names from :: syntax
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
			return nil, fmt.Errorf("%s: %w", file, err)
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
	return result, nil
}

// fileRunResult holds the result of running a single file, used for parallel execution.
type fileRunResult struct {
	file       string
	fileResult *tester.FileResult
	err        error
	output     []byte // Buffered output for this file
}

// runParallel runs test files in parallel using a worker pool.
// Returns an error if any file cannot be read or has a syntax error.
func runParallel(
	files []string,
	workers int,
	opts tester.Options,
	fileTestNames map[string][]string,
	reporter tester.Reporter,
	stdout, _ io.Writer,
) (*tester.RunResult, error) {
	start := time.Now()

	// Channel for file paths to process
	jobs := make(chan string, len(files))
	// Channel for results
	results := make(chan fileRunResult, len(files))

	// Track if we should stop early (fail-fast or error)
	var stopFlag int32 // 0 = running, 1 = stop
	shouldStop := func() bool {
		return atomic.LoadInt32(&stopFlag) == 1
	}
	setStop := func() {
		atomic.StoreInt32(&stopFlag, 1)
	}

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				// Check if we should stop early
				if shouldStop() {
					continue // Drain the channel but don't process
				}

				result := runFileForParallel(file, opts, fileTestNames, reporter)
				results <- result

				// Set stop flag on error or fail-fast failure
				if result.err != nil {
					setStop()
				} else if opts.FailFast && result.fileResult != nil && result.fileResult.HasFailures() {
					setStop()
				}
			}
		}()
	}

	// Send jobs
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in a map to preserve file order
	resultMap := make(map[string]fileRunResult)
	for r := range results {
		resultMap[r.file] = r
	}

	// Build final result in original file order
	// Check for errors first
	runResult := &tester.RunResult{}
	for _, file := range files {
		r, ok := resultMap[file]
		if !ok {
			continue
		}

		// Return first error encountered (in file order)
		if r.err != nil {
			return nil, fmt.Errorf("%s: %w", file, r.err)
		}

		if r.fileResult != nil {
			runResult.Files = append(runResult.Files, *r.fileResult)

			// Output buffered content for text reporter
			if _, ok := reporter.(*tester.TextReporter); ok {
				_, _ = stdout.Write(r.output)
			}
		}

		// Stop reporting after first failure in fail-fast mode
		if opts.FailFast && r.fileResult != nil && r.fileResult.HasFailures() {
			break
		}
	}

	runResult.Duration = time.Since(start)
	return runResult, nil
}

// runFileForParallel runs a single file and captures its output.
func runFileForParallel(
	file string,
	opts tester.Options,
	fileTestNames map[string][]string,
	reporter tester.Reporter,
) fileRunResult {
	result := fileRunResult{file: file}

	src, err := os.ReadFile(file)
	if err != nil {
		result.err = err
		return result
	}

	// Convert to absolute path for clearer output
	absPath, _ := filepath.Abs(file)
	if absPath == "" {
		absPath = file
	}

	// Check if this file has specific test names from :: syntax
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
		result.err = err
		return result
	}

	result.fileResult = fileResult

	// Buffer the output for text reporter
	if _, ok := reporter.(*tester.TextReporter); ok {
		var buf bytes.Buffer
		reporter.ReportFile(&buf, fileResult)
		result.output = buf.Bytes()
	}

	return result
}
