package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/albertocavalcante/sky/internal/starlark/tester"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK     = 0
	exitFailed = 1
	exitError  = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var (
		jsonFlag      bool
		junitFlag     bool
		versionFlag   bool
		verboseFlag   bool
		recursiveFlag bool
		prefixFlag    string
		durationFlag  bool
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
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skytest .                       # Run tests in current directory")
		writeln(stderr, "  skytest -r .                    # Run tests recursively")
		writeln(stderr, "  skytest test.star               # Run specific test file")
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

	// Discover test files
	files, err := tester.ExpandPaths(paths, nil, recursiveFlag)
	if err != nil {
		writef(stderr, "skytest: %v\n", err)
		return exitError
	}

	if len(files) == 0 {
		writeln(stderr, "skytest: no test files found")
		return exitError
	}

	// Create runner
	opts := tester.DefaultOptions()
	opts.TestPrefix = prefixFlag
	opts.Verbose = verboseFlag
	runner := tester.New(opts)

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

		fileResult, err := runner.RunFile(absPath, src)
		if err != nil {
			writef(stderr, "skytest: %s: %v\n", file, err)
			return exitError
		}

		result.Files = append(result.Files, *fileResult)

		// Report file immediately in text mode
		if _, ok := reporter.(*tester.TextReporter); ok {
			reporter.ReportFile(stdout, fileResult)
		}
	}

	result.Duration = time.Since(start)

	// Report summary
	reporter.ReportSummary(stdout, result)

	if result.HasFailures() {
		return exitFailed
	}
	return exitOK
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
