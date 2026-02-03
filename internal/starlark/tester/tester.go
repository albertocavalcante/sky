// Package tester provides a test runner for Starlark files.
//
// It discovers test functions, executes them using starlark-go,
// and reports results. Supports setup/teardown functions and
// provides a built-in assertion module.
package tester

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// TestResult represents the result of running a single test.
type TestResult struct {
	// Name is the test function name.
	Name string

	// File is the source file containing the test.
	File string

	// Passed indicates whether the test passed.
	Passed bool

	// Duration is how long the test took.
	Duration time.Duration

	// Error contains the error if the test failed.
	Error error

	// Output contains any output captured during the test.
	Output string
}

// FileResult represents the results of running all tests in a file.
type FileResult struct {
	// File is the path to the test file.
	File string

	// Tests contains results for each test function.
	Tests []TestResult

	// SetupError contains any error from setup().
	SetupError error

	// TeardownError contains any error from teardown().
	TeardownError error

	// Duration is total time for all tests in this file.
	Duration time.Duration
}

// Summary returns counts of passed and failed tests.
func (fr *FileResult) Summary() (passed, failed int) {
	for _, t := range fr.Tests {
		if t.Passed {
			passed++
		} else {
			failed++
		}
	}
	return
}

// RunResult contains all results from a test run.
type RunResult struct {
	// Files contains results for each file.
	Files []FileResult

	// Duration is total time for the entire run.
	Duration time.Duration
}

// Summary returns total counts of passed and failed tests.
func (rr *RunResult) Summary() (passed, failed, files int) {
	files = len(rr.Files)
	for _, fr := range rr.Files {
		p, f := fr.Summary()
		passed += p
		failed += f
	}
	return
}

// HasFailures returns true if any test failed.
func (rr *RunResult) HasFailures() bool {
	_, failed, _ := rr.Summary()
	return failed > 0
}

// Options configures the test runner.
type Options struct {
	// TestPrefix is the prefix for test functions (default: "test_").
	TestPrefix string

	// Predeclared contains additional predeclared values.
	// The assert module is always available unless DisableAssert is true.
	Predeclared starlark.StringDict

	// DisableAssert disables the built-in assert module.
	DisableAssert bool

	// Verbose enables verbose output.
	Verbose bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		TestPrefix:  "test_",
		Predeclared: make(starlark.StringDict),
	}
}

// Runner executes Starlark tests.
type Runner struct {
	opts Options
}

// New creates a new test runner.
func New(opts Options) *Runner {
	if opts.TestPrefix == "" {
		opts.TestPrefix = "test_"
	}
	if opts.Predeclared == nil {
		opts.Predeclared = make(starlark.StringDict)
	}
	return &Runner{opts: opts}
}

// RunFile runs all tests in a single file.
func (r *Runner) RunFile(filename string, src []byte) (*FileResult, error) {
	start := time.Now()
	result := &FileResult{File: filename}

	// Build predeclared with assert module
	predeclared := r.buildPredeclared()

	// Parse and execute the file
	thread := &starlark.Thread{Name: filename}
	globals, err := starlark.ExecFile(thread, filename, src, predeclared)
	if err != nil {
		return nil, fmt.Errorf("executing %s: %w", filename, err)
	}

	// Find test functions
	testFuncs := r.findTestFunctions(globals)

	// Look for setup and teardown
	setupFn, _ := globals["setup"].(*starlark.Function)
	teardownFn, _ := globals["teardown"].(*starlark.Function)

	// Run tests
	for _, name := range testFuncs {
		fn := globals[name].(*starlark.Function)
		testResult := r.runSingleTest(thread, name, fn, setupFn, teardownFn, predeclared)
		testResult.File = filename
		result.Tests = append(result.Tests, testResult)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// buildPredeclared constructs the predeclared values including assert.
func (r *Runner) buildPredeclared() starlark.StringDict {
	predeclared := make(starlark.StringDict)

	// Copy user predeclared
	for k, v := range r.opts.Predeclared {
		predeclared[k] = v
	}

	// Add assert module unless disabled
	if !r.opts.DisableAssert {
		predeclared["assert"] = NewAssertModule()
	}

	return predeclared
}

// findTestFunctions returns sorted list of test function names.
func (r *Runner) findTestFunctions(globals starlark.StringDict) []string {
	var names []string
	for name, val := range globals {
		if _, ok := val.(*starlark.Function); ok {
			if strings.HasPrefix(name, r.opts.TestPrefix) {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// runSingleTest executes one test function with setup/teardown.
func (r *Runner) runSingleTest(
	_ *starlark.Thread,
	name string,
	testFn *starlark.Function,
	setupFn *starlark.Function,
	teardownFn *starlark.Function,
	_ starlark.StringDict,
) TestResult {
	result := TestResult{Name: name}
	start := time.Now()

	// Create a fresh thread for this test
	testThread := &starlark.Thread{Name: name}

	// Run setup if present
	if setupFn != nil {
		_, err := starlark.Call(testThread, setupFn, nil, nil)
		if err != nil {
			result.Error = fmt.Errorf("setup failed: %w", err)
			result.Duration = time.Since(start)
			return result
		}
	}

	// Run the test
	_, err := starlark.Call(testThread, testFn, nil, nil)
	if err != nil {
		result.Error = err
	} else {
		result.Passed = true
	}

	// Run teardown if present (even if test failed)
	if teardownFn != nil {
		_, teardownErr := starlark.Call(testThread, teardownFn, nil, nil)
		if teardownErr != nil && result.Error == nil {
			result.Error = fmt.Errorf("teardown failed: %w", teardownErr)
			result.Passed = false
		}
	}

	result.Duration = time.Since(start)
	return result
}

// DiscoverTests finds test functions in a file without executing it.
func DiscoverTests(filename string, src []byte, prefix string) ([]string, error) {
	if prefix == "" {
		prefix = "test_"
	}

	// Parse the file
	f, err := syntax.Parse(filename, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}

	// Find function definitions
	var tests []string
	for _, stmt := range f.Stmts {
		if def, ok := stmt.(*syntax.DefStmt); ok {
			if strings.HasPrefix(def.Name.Name, prefix) {
				tests = append(tests, def.Name.Name)
			}
		}
	}

	sort.Strings(tests)
	return tests, nil
}
