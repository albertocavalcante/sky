// Package tester provides a test runner for Starlark files.
//
// It discovers test functions, executes them using starlark-go,
// and reports results. Supports setup/teardown functions and
// provides a built-in assertion module.
package tester

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/albertocavalcante/sky/internal/starlark/coverage"

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

	// Coverage enables coverage collection.
	// EXPERIMENTAL: Requires starlark-go-x with OnExec hook.
	// TODO(upstream): Remove experimental note once OnExec is merged upstream.
	Coverage bool

	// CoverageCollector is the collector to use for coverage.
	// If nil and Coverage is true, a default collector is created.
	CoverageCollector *coverage.DefaultCollector

	// Filter is a test name filter pattern.
	// Supports "not <pattern>" to exclude tests matching pattern.
	Filter string

	// TestNames filters to specific test function names.
	// If set, only these tests will run (used by :: syntax).
	TestNames []string

	// Preludes is a list of prelude file paths to load before each test file.
	// Prelude globals become available in the test scope.
	Preludes []string
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
	opts     Options
	coverage *coverage.DefaultCollector
}

// New creates a new test runner.
func New(opts Options) *Runner {
	if opts.TestPrefix == "" {
		opts.TestPrefix = "test_"
	}
	if opts.Predeclared == nil {
		opts.Predeclared = make(starlark.StringDict)
	}

	r := &Runner{opts: opts}

	// Set up coverage collector if enabled
	if opts.Coverage {
		if opts.CoverageCollector != nil {
			r.coverage = opts.CoverageCollector
		} else {
			r.coverage = coverage.NewCollector()
		}
	}

	return r
}

// CoverageReport returns the coverage report if coverage is enabled.
// Returns nil if coverage is disabled.
func (r *Runner) CoverageReport() *coverage.Report {
	if r.coverage == nil {
		return nil
	}
	return r.coverage.Report()
}

// RunFile runs all tests in a single file.
func (r *Runner) RunFile(filename string, src []byte) (*FileResult, error) {
	start := time.Now()
	result := &FileResult{File: filename}

	// Build predeclared with assert module
	basePredeclared := r.buildPredeclared()

	// Load preludes (if any) to get additional predeclared values
	predeclared, err := r.loadPreludes(basePredeclared)
	if err != nil {
		return nil, err
	}

	// Parse and execute the file
	thread := &starlark.Thread{Name: filename}

	// EXPERIMENTAL: Enable coverage collection via OnExec hook.
	// This only works when starlark-go-x replace directive is enabled in go.mod.
	// TODO(upstream): Simplify once OnExec is merged upstream.
	r.setupCoverageHook(thread)

	globals, err := starlark.ExecFile(thread, filename, src, predeclared)
	if err != nil {
		return nil, fmt.Errorf("executing %s: %w", filename, err)
	}

	// Find test functions
	testFuncs := r.findTestFunctions(globals)

	// Look for setup and teardown
	setupFn, _ := globals["setup"].(*starlark.Function)
	teardownFn, _ := globals["teardown"].(*starlark.Function)

	// Run tests (applying filter)
	for _, name := range testFuncs {
		if !r.matchesFilter(name) {
			continue // Skip tests that don't match filter
		}
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

// loadPreludes loads prelude files and returns their combined globals.
// Returns an error if any prelude file fails to load.
func (r *Runner) loadPreludes(basePredeclared starlark.StringDict) (starlark.StringDict, error) {
	if len(r.opts.Preludes) == 0 {
		return basePredeclared, nil
	}

	// Start with base predeclared
	combined := make(starlark.StringDict)
	for k, v := range basePredeclared {
		combined[k] = v
	}

	// Load each prelude file in order
	for _, preludePath := range r.opts.Preludes {
		src, err := os.ReadFile(preludePath)
		if err != nil {
			return nil, fmt.Errorf("reading prelude %s: %w", preludePath, err)
		}

		thread := &starlark.Thread{Name: preludePath}
		globals, err := starlark.ExecFile(thread, preludePath, src, combined)
		if err != nil {
			return nil, fmt.Errorf("executing prelude %s: %w", preludePath, err)
		}

		// Add prelude globals to combined (later preludes can shadow earlier ones)
		for k, v := range globals {
			combined[k] = v
		}
	}

	return combined, nil
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

// matchesFilter checks if a test name matches the current filter options.
func (r *Runner) matchesFilter(name string) bool {
	// Check TestNames first (from :: syntax)
	if len(r.opts.TestNames) > 0 {
		for _, allowed := range r.opts.TestNames {
			if name == allowed {
				return true
			}
		}
		return false
	}

	// Check Filter pattern (from -k flag)
	if r.opts.Filter == "" {
		return true
	}

	filter := r.opts.Filter

	// Handle "not <pattern>" syntax
	negate := false
	if strings.HasPrefix(strings.ToLower(filter), "not ") {
		negate = true
		filter = strings.TrimPrefix(filter, "not ")
		filter = strings.TrimPrefix(filter, "NOT ")
	}

	// Simple substring match (case-insensitive)
	matches := strings.Contains(strings.ToLower(name), strings.ToLower(filter))

	if negate {
		return !matches
	}
	return matches
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

	// EXPERIMENTAL: Enable coverage collection for this test thread
	r.setupCoverageHook(testThread)

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
