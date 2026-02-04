// Package tester provides a test runner for Starlark files.
//
// It discovers test functions, executes them using starlark-go,
// and reports results. Supports setup/teardown functions and
// provides a built-in assertion module.
package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/albertocavalcante/sky/internal/starlark/coverage"

	"go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
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

	// Skipped indicates the test was skipped (not run).
	Skipped bool

	// SkipReason is the reason for skipping (if any).
	SkipReason string

	// XFail indicates the test was expected to fail.
	XFail bool

	// XFailReason is the reason for expecting failure (if any).
	XFailReason string

	// XPass indicates an xfail test unexpectedly passed (counted as failure).
	XPass bool

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

// Summary returns counts of passed, failed, and skipped tests.
func (fr *FileResult) Summary() (passed, failed int) {
	for _, t := range fr.Tests {
		if t.Skipped {
			// Skipped tests don't count as pass or fail
			continue
		}
		if t.XPass {
			// Unexpected pass is a failure
			failed++
		} else if t.XFail && !t.Passed {
			// Expected failure that failed - counts as pass
			passed++
		} else if t.Passed {
			passed++
		} else {
			failed++
		}
	}
	return
}

// SkippedCount returns the number of skipped tests.
func (fr *FileResult) SkippedCount() int {
	count := 0
	for _, t := range fr.Tests {
		if t.Skipped {
			count++
		}
	}
	return count
}

// HasFailures returns true if any test in this file failed.
func (fr *FileResult) HasFailures() bool {
	_, failed := fr.Summary()
	return failed > 0
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

	// MarkerFilter filters tests by marker.
	// Supports "not <marker>" to exclude tests with marker.
	MarkerFilter string

	// TestNames filters to specific test function names.
	// If set, only these tests will run (used by :: syntax).
	TestNames []string

	// Preludes is a list of prelude file paths to load before each test file.
	// Prelude globals become available in the test scope.
	Preludes []string

	// Timeout is the maximum duration for each test.
	// If zero, no timeout is applied.
	Timeout time.Duration

	// FailFast stops running tests after the first failure.
	FailFast bool

	// UpdateSnapshots when true, updates snapshots instead of comparing.
	// Use with -u or --update-snapshots flag.
	UpdateSnapshots bool
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
	snapshot *SnapshotManager
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

	// Create snapshot manager (always available, but compare behavior depends on UpdateSnapshots)
	r.snapshot = NewSnapshotManager("", opts.UpdateSnapshots)

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

	// Load conftest.star files for fixtures
	conftestFixtures, err := r.loadConftestFixtures(filename, predeclared)
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

	// Find fixtures in this file
	fileFixtures := FindFixtures(globals)

	// Merge conftest fixtures with file fixtures (file fixtures override)
	fixtureRegistry := r.mergeFixtureRegistries(conftestFixtures, fileFixtures)

	// Extract __test_params__ for parametrized tests
	testParams := r.extractTestParams(globals)

	// Extract __test_meta__ for markers, skip, xfail
	testMeta := r.extractTestMeta(globals)

	// Look for setup and teardown
	setupFn, _ := globals["setup"].(*starlark.Function)
	teardownFn, _ := globals["teardown"].(*starlark.Function)

	// Run tests (applying filter)
	for _, name := range testFuncs {
		fn := globals[name].(*starlark.Function)

		// Get metadata for this test
		meta := testMeta[name]

		// Check marker filter
		if !r.matchesMarkerFilter(meta) {
			continue // Skip tests that don't match marker filter
		}

		// Check if this test has parameters
		if params, ok := testParams[name]; ok {
			// Run parametrized test for each case
			for _, pc := range params {
				virtualName := pc.virtualName(name)
				if !r.matchesFilter(virtualName) {
					continue // Skip tests that don't match filter
				}

				// Check for skip
				if meta.Skip {
					testResult := TestResult{
						Name:       virtualName,
						File:       filename,
						Skipped:    true,
						SkipReason: meta.SkipReason,
						Passed:     true, // Skipped counts as passed for exit code
					}
					result.Tests = append(result.Tests, testResult)
					continue
				}

				testResult := r.runParametrizedTest(thread, virtualName, fn, setupFn, teardownFn, predeclared, fixtureRegistry, pc.caseDict)
				testResult.File = filename

				// Handle xfail
				if meta.XFail {
					testResult.XFail = true
					testResult.XFailReason = meta.XFailReason
					if testResult.Passed {
						// Test passed but was expected to fail - this is XPASS (failure)
						testResult.XPass = true
						testResult.Passed = false
					} else {
						// Test failed as expected - this is success
						testResult.Passed = true
						testResult.Error = nil
					}
				}

				result.Tests = append(result.Tests, testResult)

				// Clear test-scoped fixture cache between tests
				fixtureRegistry.ClearTestCache()

				// Fail-fast: stop after first failure
				if r.opts.FailFast && !testResult.Passed {
					break
				}
			}
			// Check fail-fast at file level too
			if r.opts.FailFast && result.HasFailures() {
				break
			}
		} else {
			// Regular non-parametrized test
			if !r.matchesFilter(name) {
				continue // Skip tests that don't match filter
			}

			// Check for skip
			if meta.Skip {
				testResult := TestResult{
					Name:       name,
					File:       filename,
					Skipped:    true,
					SkipReason: meta.SkipReason,
					Passed:     true, // Skipped counts as passed for exit code
				}
				result.Tests = append(result.Tests, testResult)
				continue
			}

			testResult := r.runSingleTest(thread, name, filename, fn, setupFn, teardownFn, predeclared, fixtureRegistry)
			testResult.File = filename

			// Handle xfail
			if meta.XFail {
				testResult.XFail = true
				testResult.XFailReason = meta.XFailReason
				if testResult.Passed {
					// Test passed but was expected to fail - this is XPASS (failure)
					testResult.XPass = true
					testResult.Passed = false
				} else {
					// Test failed as expected - this is success
					testResult.Passed = true
					testResult.Error = nil
				}
			}

			result.Tests = append(result.Tests, testResult)

			// Clear test-scoped fixture cache between tests
			fixtureRegistry.ClearTestCache()

			// Fail-fast: stop after first failure
			if r.opts.FailFast && !testResult.Passed {
				break
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// loadConftestFixtures searches for conftest.star files up the directory tree
// and loads fixtures from them.
func (r *Runner) loadConftestFixtures(filename string, predeclared starlark.StringDict) (*FixtureRegistry, error) {
	registry := NewFixtureRegistry()

	// Find conftest.star files from the test file's directory up to root
	conftestPaths := r.findConftestFiles(filename)

	// Load conftest files in order from root to leaf (so closer ones override)
	for i := len(conftestPaths) - 1; i >= 0; i-- {
		conftestPath := conftestPaths[i]
		src, err := os.ReadFile(conftestPath)
		if err != nil {
			return nil, fmt.Errorf("reading conftest %s: %w", conftestPath, err)
		}

		thread := &starlark.Thread{Name: conftestPath}
		globals, err := starlark.ExecFile(thread, conftestPath, src, predeclared)
		if err != nil {
			return nil, fmt.Errorf("executing conftest %s: %w", conftestPath, err)
		}

		// Extract fixtures from conftest
		conftestFixtures := FindFixtures(globals)
		for name, fixture := range conftestFixtures.fixtures {
			registry.Register(&Fixture{
				Name:  name,
				Fn:    fixture.Fn,
				Scope: fixture.Scope,
			})
		}
	}

	return registry, nil
}

// findConftestFiles finds conftest.star files from the test file's directory up to root.
func (r *Runner) findConftestFiles(filename string) []string {
	var conftestPaths []string

	dir := filepath.Dir(filename)
	if !filepath.IsAbs(dir) {
		absDir, err := filepath.Abs(dir)
		if err == nil {
			dir = absDir
		}
	}

	// Walk up the directory tree looking for conftest.star files
	for {
		conftestPath := filepath.Join(dir, "conftest.star")
		if _, err := os.Stat(conftestPath); err == nil {
			conftestPaths = append(conftestPaths, conftestPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	return conftestPaths
}

// mergeFixtureRegistries merges multiple fixture registries.
// Later registries override earlier ones.
func (r *Runner) mergeFixtureRegistries(registries ...*FixtureRegistry) *FixtureRegistry {
	merged := NewFixtureRegistry()
	for _, reg := range registries {
		if reg == nil {
			continue
		}
		for name, fixture := range reg.fixtures {
			merged.Register(&Fixture{
				Name:  name,
				Fn:    fixture.Fn,
				Scope: fixture.Scope,
			})
		}
	}
	return merged
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

	// Add struct builtin (ubiquitous in Bazel/Starlark codebases)
	predeclared["struct"] = starlark.NewBuiltin("struct", starlarkstruct.Make)

	// Add json module for JSON parsing/serialization in tests
	predeclared["json"] = json.Module

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

// TestMeta holds metadata for a test function from __test_meta__.
type TestMeta struct {
	// Skip indicates the test should be skipped.
	Skip bool
	// SkipReason is the reason for skipping (optional).
	SkipReason string
	// XFail indicates the test is expected to fail.
	XFail bool
	// XFailReason is the reason for expecting failure (optional).
	XFailReason string
	// Markers is a list of marker names for filtering.
	Markers []string
}

// extractTestMeta extracts __test_meta__ from globals.
// Returns a map from test name to metadata.
func (r *Runner) extractTestMeta(globals starlark.StringDict) map[string]TestMeta {
	result := make(map[string]TestMeta)

	metaVal, ok := globals["__test_meta__"]
	if !ok {
		return result
	}

	metaDict, ok := metaVal.(*starlark.Dict)
	if !ok {
		return result
	}

	for _, item := range metaDict.Items() {
		testName, ok := starlark.AsString(item[0])
		if !ok {
			continue
		}

		testMetaDict, ok := item[1].(*starlark.Dict)
		if !ok {
			continue
		}

		meta := TestMeta{}

		// Check for "skip" key
		if skipVal, found, _ := testMetaDict.Get(starlark.String("skip")); found {
			switch v := skipVal.(type) {
			case starlark.Bool:
				meta.Skip = bool(v)
			case starlark.String:
				meta.Skip = true
				meta.SkipReason = string(v)
			}
		}

		// Check for "xfail" key
		if xfailVal, found, _ := testMetaDict.Get(starlark.String("xfail")); found {
			switch v := xfailVal.(type) {
			case starlark.Bool:
				meta.XFail = bool(v)
			case starlark.String:
				meta.XFail = true
				meta.XFailReason = string(v)
			}
		}

		// Check for "markers" key
		if markersVal, found, _ := testMetaDict.Get(starlark.String("markers")); found {
			if markersList, ok := markersVal.(*starlark.List); ok {
				for i := 0; i < markersList.Len(); i++ {
					if marker, ok := starlark.AsString(markersList.Index(i)); ok {
						meta.Markers = append(meta.Markers, marker)
					}
				}
			}
		}

		result[testName] = meta
	}

	return result
}

// matchesMarkerFilter checks if a test matches the marker filter.
// Returns true if no marker filter is set or if the test matches.
func (r *Runner) matchesMarkerFilter(testMeta TestMeta) bool {
	if r.opts.MarkerFilter == "" {
		return true
	}

	filter := r.opts.MarkerFilter
	negate := false

	// Handle "not <marker>" syntax
	if strings.HasPrefix(strings.ToLower(filter), "not ") {
		negate = true
		filter = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(filter), "not "))
	}

	// Check if the test has the marker
	hasMarker := false
	for _, m := range testMeta.Markers {
		if strings.EqualFold(m, filter) {
			hasMarker = true
			break
		}
	}

	if negate {
		return !hasMarker
	}
	return hasMarker
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
	filename string,
	testFn *starlark.Function,
	setupFn *starlark.Function,
	teardownFn *starlark.Function,
	_ starlark.StringDict,
	fixtureRegistry *FixtureRegistry,
) TestResult {
	result := TestResult{Name: name}
	start := time.Now()

	// Create a fresh thread for this test
	testThread := &starlark.Thread{Name: name}

	// EXPERIMENTAL: Enable coverage collection for this test thread
	r.setupCoverageHook(testThread)

	// Set up snapshot manager in thread local storage
	if r.snapshot != nil {
		r.snapshot.SetContext(filename, name)
		testThread.SetLocal(SnapshotManagerKey, r.snapshot)
	}

	// Set up timeout cancellation if configured
	var timer *time.Timer
	if r.opts.Timeout > 0 {
		timer = time.AfterFunc(r.opts.Timeout, func() {
			testThread.Cancel(fmt.Sprintf("test timeout after %s", r.opts.Timeout))
		})
		defer timer.Stop()
	}

	// Run setup if present
	if setupFn != nil {
		_, err := starlark.Call(testThread, setupFn, nil, nil)
		if err != nil {
			result.Error = fmt.Errorf("setup failed: %w", err)
			result.Duration = time.Since(start)
			return result
		}
	}

	// Resolve fixture arguments for the test function
	var args starlark.Tuple
	if fixtureRegistry != nil && testFn.NumParams() > 0 {
		var err error
		args, err = ResolveTestArgs(testThread, testFn, fixtureRegistry)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result
		}
	}

	// Run the test with fixture arguments
	_, err := starlark.Call(testThread, testFn, args, nil)
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

// paramCase represents a single test case from __test_params__.
type paramCase struct {
	name     string         // Case name (from "name" key or index)
	caseDict *starlark.Dict // The full case dictionary
}

// virtualName returns the virtual test name for this case.
// Format: test_name[case_name]
func (pc *paramCase) virtualName(testName string) string {
	return fmt.Sprintf("%s[%s]", testName, pc.name)
}

// extractTestParams extracts __test_params__ from globals.
// Returns a map from test function name to list of parameter cases.
func (r *Runner) extractTestParams(globals starlark.StringDict) map[string][]paramCase {
	result := make(map[string][]paramCase)

	paramsVal, ok := globals["__test_params__"]
	if !ok {
		return result
	}

	paramsDict, ok := paramsVal.(*starlark.Dict)
	if !ok {
		return result
	}

	// Iterate over __test_params__ dict
	for _, item := range paramsDict.Items() {
		testName, ok := starlark.AsString(item[0])
		if !ok {
			continue
		}

		// Get the list of cases for this test
		casesList, ok := item[1].(*starlark.List)
		if !ok {
			continue
		}

		var cases []paramCase
		iter := casesList.Iterate()
		defer iter.Done()
		var caseVal starlark.Value
		idx := 0
		for iter.Next(&caseVal) {
			caseDict, ok := caseVal.(*starlark.Dict)
			if !ok {
				idx++
				continue
			}

			// Extract the case name (from "name" key or use index)
			caseName := fmt.Sprintf("%d", idx)
			if nameVal, found, _ := caseDict.Get(starlark.String("name")); found {
				if nameStr, ok := starlark.AsString(nameVal); ok {
					caseName = nameStr
				}
			}

			cases = append(cases, paramCase{
				name:     caseName,
				caseDict: caseDict,
			})
			idx++
		}

		if len(cases) > 0 {
			result[testName] = cases
		}
	}

	return result
}

// runParametrizedTest executes a parametrized test with the given case.
func (r *Runner) runParametrizedTest(
	_ *starlark.Thread,
	name string,
	testFn *starlark.Function,
	setupFn *starlark.Function,
	teardownFn *starlark.Function,
	_ starlark.StringDict,
	fixtureRegistry *FixtureRegistry,
	caseDict *starlark.Dict,
) TestResult {
	result := TestResult{Name: name}
	start := time.Now()

	// Create a fresh thread for this test
	testThread := &starlark.Thread{Name: name}

	// EXPERIMENTAL: Enable coverage collection for this test thread
	r.setupCoverageHook(testThread)

	// Set up timeout cancellation if configured
	var timer *time.Timer
	if r.opts.Timeout > 0 {
		timer = time.AfterFunc(r.opts.Timeout, func() {
			testThread.Cancel(fmt.Sprintf("test timeout after %s", r.opts.Timeout))
		})
		defer timer.Stop()
	}

	// Run setup if present
	if setupFn != nil {
		_, err := starlark.Call(testThread, setupFn, nil, nil)
		if err != nil {
			result.Error = fmt.Errorf("setup failed: %w", err)
			result.Duration = time.Since(start)
			return result
		}
	}

	// For parametrized tests, the case dict is passed as the first argument
	args := starlark.Tuple{caseDict}

	// If the test function expects additional fixture arguments beyond the first,
	// resolve them from the fixture registry
	if fixtureRegistry != nil && testFn.NumParams() > 1 {
		// Skip the first parameter (case dict) and resolve fixtures for the rest
		fixtureArgs, err := r.resolveFixtureArgsSkipFirst(testThread, testFn, fixtureRegistry)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result
		}
		args = append(args, fixtureArgs...)
	}

	// Run the test with the case dict as argument
	_, err := starlark.Call(testThread, testFn, args, nil)
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

// resolveFixtureArgsSkipFirst resolves fixture arguments for a test function,
// skipping the first parameter (used for parametrized tests where first arg is case dict).
func (r *Runner) resolveFixtureArgsSkipFirst(thread *starlark.Thread, fn *starlark.Function, registry *FixtureRegistry) (starlark.Tuple, error) {
	numParams := fn.NumParams()
	if numParams <= 1 {
		return nil, nil
	}

	var args starlark.Tuple
	for i := 1; i < numParams; i++ {
		paramName, _ := fn.Param(i)
		value, err := registry.GetOrCompute(thread, paramName, registry)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}

	return args, nil
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
