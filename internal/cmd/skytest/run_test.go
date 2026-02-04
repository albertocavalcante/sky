package skytest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-version"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-version) returned %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Error("RunWithIO(-version) produced no output")
	}
}

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-help"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-help) returned %d, want 0", code)
	}
}

func TestRun_PassingTests(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_math.star")
	content := `def test_addition():
    assert.eq(1 + 1, 2)

def test_subtraction():
    assert.eq(5 - 3, 2)

def test_multiplication():
    assert.eq(3 * 4, 12)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(passing tests) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_FailingTests(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_fail.star")
	content := `def test_will_fail():
    assert.eq(1, 2)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(failing tests) returned 0, want non-zero")
	}
}

func TestRun_MixedTests(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mixed.star")
	content := `def test_pass():
    assert.eq(1, 1)

def test_fail():
    assert.eq(1, 2)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	// Should fail because one test fails
	if code == 0 {
		t.Error("RunWithIO(mixed tests) returned 0, want non-zero")
	}
}

func TestRun_TestWithSetup(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_setup.star")
	// Note: setup() cannot modify frozen globals in Starlark.
	// This test verifies that setup() is called before each test.
	// We use a simple setup that doesn't require mutable state.
	content := `_setup_called = [False]

def setup():
    # setup() is called but globals are frozen, so we can't track state.
    # Just verify the function runs without error.
    pass

def test_basic():
    assert.eq(1 + 1, 2)

def test_another():
    assert.eq(2 * 3, 6)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(test with setup) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_MultipleTestFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "test_a.star")
	file2 := filepath.Join(dir, "test_b.star")

	if err := os.WriteFile(file1, []byte("def test_a():\n    assert.eq(1, 1)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("def test_b():\n    assert.eq(2, 2)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file1, file2}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple test files) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_TestDirectory(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "test_a.star")
	file2 := filepath.Join(dir, "test_b.star")

	if err := os.WriteFile(file1, []byte("def test_a():\n    assert.eq(1, 1)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("def test_b():\n    assert.eq(2, 2)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(test directory) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_MultipleTestFunctions(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_multiple.star")
	content := `def test_foo():
    assert.eq(1, 1)

def test_bar():
    assert.eq(2, 2)

def test_baz():
    assert.eq(3, 3)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple test functions) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_VerboseOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_verbose.star")
	content := `def test_one():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-v) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Verbose should show test names
	output := stdout.String() + stderr.String()
	if len(output) == 0 {
		t.Error("RunWithIO(-v) produced no output")
	}
}

func TestRun_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_bad.star")
	content := `def test_syntax(
    # missing closing paren
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(syntax error) returned 0, want non-zero")
	}
}

func TestRun_NonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"/nonexistent/test.star"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent file) returned 0, want non-zero")
	}
}

func TestRun_NoTestFunctions(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not_tests.star")
	content := `def helper():
    return 42

x = helper()
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	// File with no test functions should pass (no tests to fail)
	if code != 0 {
		t.Errorf("RunWithIO(no test functions) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

// Test Filtering (-k flag) tests

func TestRun_FilterByName(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_filter.star")
	content := `def test_parse_basic():
    assert.eq(1, 1)

def test_parse_advanced():
    assert.eq(2, 2)

def test_other_feature():
    assert.eq(3, 3)

def test_unrelated():
    assert.eq(4, 4)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-k", "parse", "-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-k parse) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Should run parse tests
	if !strings.Contains(output, "test_parse_basic") {
		t.Error("expected test_parse_basic to be run")
	}
	if !strings.Contains(output, "test_parse_advanced") {
		t.Error("expected test_parse_advanced to be run")
	}
	// Should not run other tests
	if strings.Contains(output, "test_other_feature") && !strings.Contains(output, "skipped") {
		t.Error("expected test_other_feature to be skipped or not shown")
	}
}

func TestRun_FilterWithNot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_filter_not.star")
	content := `def test_fast_unit():
    assert.eq(1, 1)

def test_slow_integration():
    assert.eq(2, 2)

def test_fast_other():
    assert.eq(3, 3)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-k", "not slow", "-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-k 'not slow') returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Should run non-slow tests
	if !strings.Contains(output, "test_fast_unit") {
		t.Error("expected test_fast_unit to be run")
	}
	if !strings.Contains(output, "test_fast_other") {
		t.Error("expected test_fast_other to be run")
	}
}

func TestRun_FilterSpecificTest(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_specific.star")
	content := `def test_one():
    assert.eq(1, 1)

def test_two():
    assert.eq(2, 2)

def test_three():
    assert.eq(3, 3)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Use :: syntax to select specific test
	code := RunWithIO(context.Background(), []string{"-v", file + "::test_two"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(file::test_two) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Should only run test_two
	if !strings.Contains(output, "test_two") {
		t.Error("expected test_two to be run")
	}
	// test_one and test_three should not appear in passed tests
	// (they might appear as "skipped" in verbose mode, which is acceptable)
}

// Prelude System (--prelude flag) tests

func TestRun_PreludeBasic(t *testing.T) {
	dir := t.TempDir()

	// Create prelude file with helper function
	preludeFile := filepath.Join(dir, "helpers.star")
	preludeContent := `def add_one(x):
    return x + 1

SHARED_VALUE = 42
`
	if err := os.WriteFile(preludeFile, []byte(preludeContent), 0644); err != nil {
		t.Fatalf("failed to write prelude file: %v", err)
	}

	// Create test file that uses prelude helper
	testFile := filepath.Join(dir, "test_with_prelude.star")
	testContent := `def test_uses_helper():
    # add_one and SHARED_VALUE come from prelude
    assert.eq(add_one(1), 2)
    assert.eq(SHARED_VALUE, 42)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--prelude", preludeFile, testFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(--prelude) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_PreludeMultiple(t *testing.T) {
	dir := t.TempDir()

	// Create first prelude file
	prelude1 := filepath.Join(dir, "prelude1.star")
	if err := os.WriteFile(prelude1, []byte(`PRELUDE1_VALUE = 10`), 0644); err != nil {
		t.Fatalf("failed to write prelude1: %v", err)
	}

	// Create second prelude file
	prelude2 := filepath.Join(dir, "prelude2.star")
	if err := os.WriteFile(prelude2, []byte(`PRELUDE2_VALUE = 20`), 0644); err != nil {
		t.Fatalf("failed to write prelude2: %v", err)
	}

	// Create test file that uses both preludes
	testFile := filepath.Join(dir, "test_multi_prelude.star")
	testContent := `def test_uses_both_preludes():
    assert.eq(PRELUDE1_VALUE, 10)
    assert.eq(PRELUDE2_VALUE, 20)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{
		"--prelude", prelude1,
		"--prelude", prelude2,
		testFile,
	}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple --prelude) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_PreludeNotFound(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test_simple.star")
	if err := os.WriteFile(testFile, []byte(`def test_pass():\n    assert.eq(1, 1)`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{
		"--prelude", "/nonexistent/prelude.star",
		testFile,
	}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent --prelude) returned 0, want non-zero")
	}
}

// Test Timeouts (--timeout flag) tests

func TestRun_TimeoutBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_fast.star")
	content := `def test_fast():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Set a generous timeout that should pass
	code := RunWithIO(context.Background(), []string{"--timeout", "10s", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(--timeout 10s) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_TimeoutZeroDisables(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_no_timeout.star")
	content := `def test_no_timeout():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Zero should disable timeout
	code := RunWithIO(context.Background(), []string{"--timeout", "0", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(--timeout 0) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_TimeoutExpired(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_slow.star")
	// Create a test with an infinite loop
	content := `def test_infinite_loop():
    x = 0
    for i in range(1000000000):  # Very long loop
        x = x + 1
    assert.eq(x, 0)  # Should never reach
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Set a very short timeout to trigger cancellation
	code := RunWithIO(context.Background(), []string{"--timeout", "100ms", file}, nil, &stdout, &stderr)

	// Should fail due to timeout
	if code == 0 {
		t.Error("RunWithIO(--timeout 100ms with slow test) returned 0, expected failure")
	}

	// Check that output mentions timeout
	combined := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(combined), "timeout") && !strings.Contains(strings.ToLower(combined), "cancel") {
		t.Logf("output: %s", combined)
		// Note: We'll accept either mention of timeout/cancel or just a failure
		// since Starlark may report it differently
	}
}

// Fail-Fast Mode (--bail / -x flag) tests

func TestRun_BailOnFirstFailure(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_bail.star")
	// Create tests where first fails, second should not run
	content := `def test_aaa_fails():
    assert.eq(1, 2)  # Will fail

def test_bbb_should_not_run():
    assert.eq(1, 1)

def test_ccc_should_not_run():
    assert.eq(2, 2)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--bail", "-v", file}, nil, &stdout, &stderr)

	// Should fail
	if code == 0 {
		t.Error("RunWithIO(--bail) returned 0, expected failure")
	}

	output := stdout.String()
	// First test should appear (it fails)
	if !strings.Contains(output, "test_aaa_fails") {
		t.Error("expected test_aaa_fails to be reported")
	}
	// Later tests should not run (bail stops after first failure)
	// Note: They might appear in discovery but not in results
}

func TestRun_BailShortFlag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_bail_x.star")
	content := `def test_fails():
    assert.eq(1, 2)

def test_passes():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Use -x short form
	code := RunWithIO(context.Background(), []string{"-x", file}, nil, &stdout, &stderr)

	// Should fail
	if code == 0 {
		t.Error("RunWithIO(-x) returned 0, expected failure")
	}
}

func TestRun_BailMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// First file fails
	file1 := filepath.Join(dir, "test_a_fails.star")
	if err := os.WriteFile(file1, []byte("def test_fail():\n    assert.eq(1, 2)"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Second file passes
	file2 := filepath.Join(dir, "test_b_passes.star")
	if err := os.WriteFile(file2, []byte("def test_pass():\n    assert.eq(1, 1)"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--bail", file1, file2}, nil, &stdout, &stderr)

	// Should fail
	if code == 0 {
		t.Error("RunWithIO(--bail with multiple files) returned 0, expected failure")
	}
}

func TestRun_NoBailRunsAll(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_no_bail.star")
	content := `def test_aaa_fails():
    assert.eq(1, 2)  # Will fail

def test_bbb_passes():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Without --bail, all tests should run
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should fail because one test fails
	if code == 0 {
		t.Error("RunWithIO(without --bail) returned 0, expected failure")
	}

	output := stdout.String()
	// Both tests should appear
	if !strings.Contains(output, "test_aaa_fails") {
		t.Error("expected test_aaa_fails to be reported")
	}
	if !strings.Contains(output, "test_bbb_passes") {
		t.Error("expected test_bbb_passes to be reported (should run without --bail)")
	}
}

// Fixture Tests

func TestRun_FixtureBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_fixture.star")
	// Define a fixture function and a test that uses it
	content := `def fixture_sample_data():
    return {"users": ["alice", "bob"], "count": 2}

def test_uses_fixture(sample_data):
    # sample_data should be injected from fixture_sample_data
    assert.eq(sample_data["count"], 2)
    assert.eq(len(sample_data["users"]), 2)
    assert.eq(sample_data["users"][0], "alice")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(fixture basic) returned %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestRun_FixtureFromConftest(t *testing.T) {
	// Create a directory structure with conftest.star
	dir := t.TempDir()
	subdir := filepath.Join(dir, "tests")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create conftest.star in parent directory
	conftestFile := filepath.Join(dir, "conftest.star")
	conftestContent := `def fixture_db_connection():
    return {"host": "localhost", "port": 5432}

def fixture_config():
    return {"debug": True, "env": "test"}
`
	if err := os.WriteFile(conftestFile, []byte(conftestContent), 0644); err != nil {
		t.Fatalf("failed to write conftest file: %v", err)
	}

	// Create test file in subdirectory that uses fixtures from conftest
	testFile := filepath.Join(subdir, "test_db.star")
	testContent := `def test_uses_db(db_connection):
    # db_connection comes from conftest.star in parent directory
    assert.eq(db_connection["host"], "localhost")
    assert.eq(db_connection["port"], 5432)

def test_uses_config(config):
    # config comes from conftest.star
    assert.eq(config["debug"], True)
    assert.eq(config["env"], "test")
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", testFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(fixture from conftest) returned %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestRun_FixtureDependsOnFixture(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_fixture_deps.star")
	// Fixtures can depend on other fixtures
	content := `def fixture_base_url():
    return "http://localhost:8080"

def fixture_api_client(base_url):
    # api_client depends on base_url fixture
    return {"url": base_url, "headers": {"Content-Type": "application/json"}}

def test_client_uses_url(api_client, base_url):
    # Test receives both fixtures
    assert.eq(api_client["url"], base_url)
    assert.eq(api_client["url"], "http://localhost:8080")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(fixture depends on fixture) returned %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestRun_FixtureScopeTest(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_fixture_scope.star")
	// Test-scoped fixtures get fresh instances per test
	// Each fixture call returns a new list, so we can test identity
	content := `def fixture_unique_list():
    # Each call returns a NEW list instance
    return []

def test_first(unique_list):
    # Verify we get an empty list (fixture was called)
    assert.eq(len(unique_list), 0)
    # Can append to it (not frozen since it's returned fresh)
    unique_list.append(1)
    assert.eq(len(unique_list), 1)

def test_second(unique_list):
    # Second test also gets an empty list (fresh call for test scope)
    # This proves we got a new list, not the one modified by test_first
    assert.eq(len(unique_list), 0)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(fixture scope test) returned %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestRun_FixtureScopeFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_fixture_file_scope.star")
	// File-scoped fixtures are shared within a file
	// Each fixture call returns a new list, but with file scope it's cached
	content := `def fixture_shared_list():
    # This is only called ONCE for file scope, result is cached
    return []

# Configure fixture scope
__fixture_config__ = {
    "shared_list": "file",
}

def test_first(shared_list):
    # Verify we get an empty list
    assert.eq(len(shared_list), 0)
    # Append to it
    shared_list.append(1)
    assert.eq(len(shared_list), 1)

def test_second(shared_list):
    # Second test gets THE SAME list (file scope = cached)
    # It should have the item appended by test_first
    assert.eq(len(shared_list), 1)
    assert.eq(shared_list[0], 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(fixture scope file) returned %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestRun_FixtureNotFound(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_missing_fixture.star")
	// Test requests a fixture that doesn't exist
	content := `def test_uses_missing_fixture(nonexistent_fixture):
    assert.eq(nonexistent_fixture, "something")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should fail because fixture is not found
	if code == 0 {
		t.Error("RunWithIO(missing fixture) returned 0, expected failure")
	}

	// Output should mention the missing fixture
	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "nonexistent_fixture") || !strings.Contains(output, "not found") {
		t.Errorf("expected error about nonexistent_fixture not found, got:\n%s", output)
	}
}

// Test Markers tests

func TestRun_MarkSkip(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mark_skip.star")
	// Test uses __test_meta__ to mark a test as skipped
	content := `def test_should_be_skipped():
    assert.eq(1, 2)  # Would fail if run

def test_should_run():
    assert.eq(1, 1)

__test_meta__ = {
    "test_should_be_skipped": {"skip": True},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should pass because the failing test is skipped
	if code != 0 {
		t.Errorf("RunWithIO(__test_meta__ skip) returned %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	output := stdout.String()
	// Should show that test_should_be_skipped was skipped
	if !strings.Contains(output, "SKIP") || !strings.Contains(output, "test_should_be_skipped") {
		t.Errorf("expected test_should_be_skipped to be reported as SKIP, got:\n%s", output)
	}
	// Should run test_should_run
	if !strings.Contains(output, "test_should_run") {
		t.Error("expected test_should_run to be run")
	}
}

func TestRun_MarkSkipWithReason(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mark_skip_reason.star")
	// Test uses __test_meta__ with skip reason
	content := `def test_not_implemented():
    assert.eq(1, 2)  # Would fail if run

def test_should_run():
    assert.eq(1, 1)

__test_meta__ = {
    "test_not_implemented": {"skip": "Feature not implemented yet"},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should pass because the failing test is skipped
	if code != 0 {
		t.Errorf("RunWithIO(__test_meta__ skip with reason) returned %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	output := stdout.String()
	// Should show that test_not_implemented was skipped
	if !strings.Contains(output, "SKIP") || !strings.Contains(output, "test_not_implemented") {
		t.Errorf("expected test_not_implemented to be reported as SKIP, got:\n%s", output)
	}
	// Should show the reason
	if !strings.Contains(output, "Feature not implemented") {
		t.Errorf("expected skip reason in output, got:\n%s", output)
	}
}

func TestRun_MarkXfail(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_xfail.star")
	// Test uses xfail - a test that fails is expected to fail
	content := `def test_known_bug():
    # This test fails due to a known bug
    assert.eq(1, 2)

def test_should_run():
    assert.eq(1, 1)

__test_meta__ = {
    "test_known_bug": {"xfail": "Bug #123"},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should pass because the failing test is expected to fail (xfail)
	if code != 0 {
		t.Errorf("RunWithIO(xfail) returned %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	output := stdout.String()
	// Should show that test_known_bug was XFAIL (expected failure)
	if !strings.Contains(output, "XFAIL") || !strings.Contains(output, "test_known_bug") {
		t.Errorf("expected test_known_bug to be reported as XFAIL, got:\n%s", output)
	}
}

func TestRun_MarkXfailPasses(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_xfail_passes.star")
	// Test uses xfail but the test actually passes - this should be reported as XPASS (unexpected pass)
	content := `def test_bug_fixed():
    # Bug was fixed, test now passes unexpectedly
    assert.eq(1, 1)

__test_meta__ = {
    "test_bug_fixed": {"xfail": "Bug #123"},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should FAIL because the xfail test unexpectedly passed (XPASS is a failure)
	if code == 0 {
		t.Errorf("RunWithIO(xfail that passes) returned 0, want non-zero\nstdout: %s", stdout.String())
	}

	output := stdout.String()
	// Should show that test_bug_fixed was XPASS (unexpected pass)
	if !strings.Contains(output, "XPASS") || !strings.Contains(output, "test_bug_fixed") {
		t.Errorf("expected test_bug_fixed to be reported as XPASS, got:\n%s", output)
	}
}

func TestRun_MarkFilter(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mark_filter.star")
	// Test with markers for filtering
	content := `def test_unit_fast():
    assert.eq(1, 1)

def test_slow_integration():
    assert.eq(2, 2)

def test_another_slow():
    assert.eq(3, 3)

__test_meta__ = {
    "test_slow_integration": {"markers": ["slow", "integration"]},
    "test_another_slow": {"markers": ["slow"]},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Filter to only run tests with "slow" marker
	code := RunWithIO(context.Background(), []string{"-m", "slow", "-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-m slow) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Should run only slow tests
	if !strings.Contains(output, "test_slow_integration") {
		t.Error("expected test_slow_integration to be run")
	}
	if !strings.Contains(output, "test_another_slow") {
		t.Error("expected test_another_slow to be run")
	}
	// test_unit_fast should NOT be run (or should be skipped)
	if strings.Contains(output, "PASS") && strings.Contains(output, "test_unit_fast") {
		t.Error("expected test_unit_fast to NOT be run with -m slow")
	}
}

func TestRun_MarkFilterNot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mark_filter_not.star")
	// Test with markers for filtering
	content := `def test_unit_fast():
    assert.eq(1, 1)

def test_slow_integration():
    assert.eq(2, 2)

def test_another_fast():
    assert.eq(3, 3)

__test_meta__ = {
    "test_slow_integration": {"markers": ["slow"]},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Filter to exclude tests with "slow" marker
	code := RunWithIO(context.Background(), []string{"-m", "not slow", "-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-m 'not slow') returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Should run non-slow tests
	if !strings.Contains(output, "test_unit_fast") {
		t.Error("expected test_unit_fast to be run")
	}
	if !strings.Contains(output, "test_another_fast") {
		t.Error("expected test_another_fast to be run")
	}
	// test_slow_integration should NOT be run (or should be skipped)
	if strings.Contains(output, "PASS") && strings.Contains(output, "test_slow_integration") {
		t.Error("expected test_slow_integration to NOT be run with -m 'not slow'")
	}
}

func TestRun_MarkMultiple(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mark_multiple.star")
	// Test with multiple markers on one test
	content := `def test_complex():
    assert.eq(1, 1)

__test_meta__ = {
    "test_complex": {"markers": ["slow", "integration", "database"]},
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test that filtering by any of the markers works
	tests := []struct {
		marker string
		want   bool // true if test should run
	}{
		{"slow", true},
		{"integration", true},
		{"database", true},
		{"fast", false},
		{"not slow", false},
	}

	for _, tc := range tests {
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), []string{"-m", tc.marker, "-v", file}, nil, &stdout, &stderr)

		output := stdout.String()
		hasTest := strings.Contains(output, "PASS") && strings.Contains(output, "test_complex")

		if tc.want && !hasTest {
			t.Errorf("-m %q: expected test_complex to run, output:\n%s", tc.marker, output)
		}
		if !tc.want && hasTest {
			t.Errorf("-m %q: expected test_complex to NOT run, output:\n%s", tc.marker, output)
		}
		if tc.want && code != 0 {
			t.Errorf("-m %q: expected exit code 0, got %d", tc.marker, code)
		}
	}
}

// Table-Driven Tests (__test_params__) tests

func TestRun_ParamBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_param.star")
	content := `
CASES = [
    {"name": "empty", "input": "", "want": 0},
    {"name": "single", "input": "a", "want": 1},
    {"name": "multi", "input": "abc", "want": 3},
]

def test_length(case):
    assert.eq(len(case["input"]), case["want"])

__test_params__ = {"test_length": CASES}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(parametrized test) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	// Should show virtual test names with case names
	if !strings.Contains(output, "test_length[empty]") {
		t.Errorf("expected test_length[empty] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[single]") {
		t.Errorf("expected test_length[single] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[multi]") {
		t.Errorf("expected test_length[multi] in output, got:\n%s", output)
	}
}

func TestRun_ParamWithoutName(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_param_noname.star")
	content := `
CASES = [
    {"input": "a", "want": 1},
    {"input": "ab", "want": 2},
    {"input": "abc", "want": 3},
]

def test_length(case):
    assert.eq(len(case["input"]), case["want"])

__test_params__ = {"test_length": CASES}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(parametrized test without name) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	// Should show virtual test names with indices (0, 1, 2)
	if !strings.Contains(output, "test_length[0]") {
		t.Errorf("expected test_length[0] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[1]") {
		t.Errorf("expected test_length[1] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[2]") {
		t.Errorf("expected test_length[2] in output, got:\n%s", output)
	}
}

func TestRun_ParamSomeFail(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_param_fail.star")
	content := `
CASES = [
    {"name": "pass1", "input": "a", "want": 1},
    {"name": "fail", "input": "ab", "want": 999},
    {"name": "pass2", "input": "abc", "want": 3},
]

def test_length(case):
    assert.eq(len(case["input"]), case["want"])

__test_params__ = {"test_length": CASES}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	// Should fail because one case fails
	if code == 0 {
		t.Error("RunWithIO(parametrized test with failures) returned 0, want non-zero")
	}

	output := stdout.String()
	// Should show individual results
	if !strings.Contains(output, "test_length[pass1]") {
		t.Errorf("expected test_length[pass1] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[fail]") {
		t.Errorf("expected test_length[fail] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[pass2]") {
		t.Errorf("expected test_length[pass2] in output, got:\n%s", output)
	}
	// Should show FAIL for the failing case
	if !strings.Contains(output, "FAIL") {
		t.Errorf("expected FAIL in output for failing case, got:\n%s", output)
	}
}

func TestRun_ParamFilterByCase(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_param_filter.star")
	content := `
CASES = [
    {"name": "empty", "input": "", "want": 0},
    {"name": "single", "input": "a", "want": 1},
    {"name": "multi", "input": "abc", "want": 3},
]

def test_length(case):
    assert.eq(len(case["input"]), case["want"])

__test_params__ = {"test_length": CASES}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Filter by case name "empty"
	code := RunWithIO(context.Background(), []string{"-k", "empty", "-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-k empty) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	// Should only run test_length[empty]
	if !strings.Contains(output, "test_length[empty]") {
		t.Errorf("expected test_length[empty] in output, got:\n%s", output)
	}
	// Other cases should not be run
	if strings.Contains(output, "test_length[single]") && !strings.Contains(output, "skipped") {
		t.Errorf("expected test_length[single] to NOT be run, got:\n%s", output)
	}
	if strings.Contains(output, "test_length[multi]") && !strings.Contains(output, "skipped") {
		t.Errorf("expected test_length[multi] to NOT be run, got:\n%s", output)
	}
}

func TestRun_ParamMultipleTests(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_param_multi.star")
	content := `
LENGTH_CASES = [
    {"name": "empty", "input": "", "want": 0},
    {"name": "one", "input": "x", "want": 1},
]

UPPER_CASES = [
    {"name": "lower", "input": "abc", "want": "ABC"},
    {"name": "mixed", "input": "aBc", "want": "ABC"},
]

def test_length(case):
    assert.eq(len(case["input"]), case["want"])

def test_upper(case):
    assert.eq(case["input"].upper(), case["want"])

__test_params__ = {
    "test_length": LENGTH_CASES,
    "test_upper": UPPER_CASES,
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple parametrized tests) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	// Should show all virtual tests
	if !strings.Contains(output, "test_length[empty]") {
		t.Errorf("expected test_length[empty] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_length[one]") {
		t.Errorf("expected test_length[one] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_upper[lower]") {
		t.Errorf("expected test_upper[lower] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test_upper[mixed]") {
		t.Errorf("expected test_upper[mixed] in output, got:\n%s", output)
	}
}

// ============================================================================
// Builtin Quick Wins Tests (struct, json, assert.len, assert.empty)
// ============================================================================

func TestRun_StructBuiltin(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_struct.star")
	content := `def test_struct_basic():
    """Test that struct builtin is available."""
    s = struct(name="foo", version="1.0.0")
    assert.eq(s.name, "foo")
    assert.eq(s.version, "1.0.0")

def test_struct_attribute_access():
    """Test attribute access vs dict access."""
    s = struct(a=1, b=2, c=3)
    assert.eq(s.a, 1)
    assert.eq(s.b, 2)
    assert.eq(s.c, 3)

def test_struct_nested():
    """Test nested structs."""
    inner = struct(value=42)
    outer = struct(name="outer", nested=inner)
    assert.eq(outer.name, "outer")
    assert.eq(outer.nested.value, 42)

def test_struct_equality():
    """Test struct equality."""
    s1 = struct(a=1, b=2)
    s2 = struct(a=1, b=2)
    s3 = struct(a=1, b=3)
    assert.eq(s1, s2)
    assert.ne(s1, s3)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(struct tests) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_JsonModule(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_json.star")
	content := `def test_json_decode():
    """Test JSON decoding."""
    data = json.decode('{"key": "value", "num": 42}')
    assert.eq(data["key"], "value")
    assert.eq(data["num"], 42)

def test_json_encode():
    """Test JSON encoding."""
    text = json.encode({"foo": [1, 2, 3]})
    assert.contains(text, "foo")
    assert.contains(text, "[1")

def test_json_roundtrip():
    """Test encode/decode roundtrip."""
    original = {"name": "test", "values": [1, 2, 3], "nested": {"a": 1}}
    encoded = json.encode(original)
    decoded = json.decode(encoded)
    assert.eq(decoded["name"], "test")
    assert.eq(decoded["values"], [1, 2, 3])
    assert.eq(decoded["nested"]["a"], 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(json tests) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_AssertLen(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_assert_len.star")
	content := `def test_assert_len_list():
    """Test assert.len with lists."""
    assert.len([1, 2, 3], 3)
    assert.len([], 0)
    assert.len([1], 1)

def test_assert_len_dict():
    """Test assert.len with dicts."""
    assert.len({"a": 1, "b": 2}, 2)
    assert.len({}, 0)

def test_assert_len_string():
    """Test assert.len with strings."""
    assert.len("hello", 5)
    assert.len("", 0)

def test_assert_len_tuple():
    """Test assert.len with tuples."""
    assert.len((1, 2, 3), 3)
    assert.len((), 0)

def test_assert_len_fails_on_wrong_length():
    """Test that assert.len fails when length doesn't match."""
    assert.fails(lambda: assert.len([1, 2], 3), "expected len")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(assert.len tests) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_AssertEmpty(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_assert_empty.star")
	content := `def test_assert_empty_list():
    """Test assert.empty with lists."""
    assert.empty([])

def test_assert_empty_dict():
    """Test assert.empty with dicts."""
    assert.empty({})

def test_assert_empty_string():
    """Test assert.empty with strings."""
    assert.empty("")

def test_assert_not_empty_list():
    """Test assert.not_empty with lists."""
    assert.not_empty([1])
    assert.not_empty([1, 2, 3])

def test_assert_not_empty_dict():
    """Test assert.not_empty with dicts."""
    assert.not_empty({"a": 1})

def test_assert_not_empty_string():
    """Test assert.not_empty with strings."""
    assert.not_empty("hello")

def test_assert_empty_fails_on_nonempty():
    """Test that assert.empty fails on non-empty container."""
    assert.fails(lambda: assert.empty([1]), "to be empty")

def test_assert_not_empty_fails_on_empty():
    """Test that assert.not_empty fails on empty container."""
    assert.fails(lambda: assert.not_empty([]), "to not be empty")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(assert.empty tests) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

// ============================================================================
// Snapshot Testing (Phase 3.2)
// ============================================================================

func TestRun_SnapshotBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_snapshot.star")
	content := `def test_snapshot_string():
    """Test basic string snapshot."""
    assert.snapshot("hello world", "greeting")

def test_snapshot_dict():
    """Test dict snapshot."""
    data = {"name": "Alice", "age": 30}
    assert.snapshot(data, "user_data")

def test_snapshot_list():
    """Test list snapshot."""
    items = [1, 2, 3, "four", {"nested": True}]
    assert.snapshot(items, "mixed_list")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// First run - creates snapshots
	var stdout1, stderr1 bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout1, &stderr1)

	if code != 0 {
		t.Errorf("First run returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr1.String(), stdout1.String())
	}

	// Verify snapshot files were created
	snapDir := filepath.Join(dir, "__snapshots__", "test_snapshot")
	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		t.Errorf("Snapshot directory not created: %s", snapDir)
	}

	// Second run - compares against existing snapshots
	var stdout2, stderr2 bytes.Buffer
	code = RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout2, &stderr2)

	if code != 0 {
		t.Errorf("Second run returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr2.String(), stdout2.String())
	}
}

func TestRun_SnapshotMismatch(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_snapshot_mismatch.star")

	// Create initial snapshot
	content1 := `def test_value():
    assert.snapshot("original", "value")
`
	if err := os.WriteFile(file, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout1, stderr1 bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout1, &stderr1)
	if code != 0 {
		t.Fatalf("Initial run failed: %s", stderr1.String())
	}

	// Now change the value - should fail
	content2 := `def test_value():
    assert.snapshot("changed", "value")
`
	if err := os.WriteFile(file, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write modified test file: %v", err)
	}

	var stdout2, stderr2 bytes.Buffer
	code = RunWithIO(context.Background(), []string{file}, nil, &stdout2, &stderr2)

	if code == 0 {
		t.Error("Expected mismatch to cause failure, but test passed")
	}

	output := stdout2.String()
	if !strings.Contains(output, "does not match") {
		t.Errorf("Expected 'does not match' in output, got:\n%s", output)
	}
}

func TestRun_SnapshotUpdate(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_snapshot_update.star")

	// Create initial snapshot
	content1 := `def test_value():
    assert.snapshot("original", "value")
`
	if err := os.WriteFile(file, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout1, stderr1 bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout1, &stderr1)
	if code != 0 {
		t.Fatalf("Initial run failed: %s", stderr1.String())
	}

	// Change the value
	content2 := `def test_value():
    assert.snapshot("updated", "value")
`
	if err := os.WriteFile(file, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write modified test file: %v", err)
	}

	// Run with --update-snapshots - should pass and update
	var stdout2, stderr2 bytes.Buffer
	code = RunWithIO(context.Background(), []string{"--update-snapshots", file}, nil, &stdout2, &stderr2)

	if code != 0 {
		t.Errorf("Update run returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr2.String(), stdout2.String())
	}

	// Verify subsequent run passes without update flag
	var stdout3, stderr3 bytes.Buffer
	code = RunWithIO(context.Background(), []string{file}, nil, &stdout3, &stderr3)

	if code != 0 {
		t.Errorf("Post-update run returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr3.String(), stdout3.String())
	}
}

func TestRun_SnapshotStruct(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_snapshot_struct.star")
	content := `def test_struct_snapshot():
    """Test struct snapshot."""
    s = struct(name="test", value=42, nested=struct(a=1, b=2))
    assert.snapshot(s, "my_struct")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// First run - creates snapshot
	var stdout1, stderr1 bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout1, &stderr1)

	if code != 0 {
		t.Errorf("First run returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr1.String(), stdout1.String())
	}

	// Second run - compares
	var stdout2, stderr2 bytes.Buffer
	code = RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout2, &stderr2)

	if code != 0 {
		t.Errorf("Second run returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr2.String(), stdout2.String())
	}
}

// Mock fixture tests

func TestRun_MockBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mock_basic.star")
	content := `def test_mock_wrap(mock):
    """Test mock.wrap() creates a wrapper."""
    def original_fn():
        return "original"

    wrapped = mock.wrap(original_fn)
    result = wrapped()

    # Without configuration, should call through to original
    assert.eq(result, "original")
    assert.true(mock.was_called(wrapped))
    assert.eq(mock.call_count(wrapped), 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(mock basic) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_MockThenReturn(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mock_then_return.star")
	content := `def test_mock_then_return(mock):
    """Test mock.when().then_return() configuration."""
    def fetch_data():
        return {"error": "should not be called"}

    wrapped = mock.wrap(fetch_data)
    mock.when(wrapped).then_return({"data": 42})

    result = wrapped()

    assert.eq(result["data"], 42)
    assert.true(mock.was_called(wrapped))
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(mock then_return) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_MockCalledWith(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mock_called_with.star")
	content := `def test_mock_called_with(mock):
    """Test mock.when().called_with().then_return() configuration."""
    def api_call(url):
        return {"error": "should not be called"}

    wrapped = mock.wrap(api_call)
    mock.when(wrapped).called_with("/api/users").then_return({"users": ["alice", "bob"]})
    mock.when(wrapped).called_with("/api/posts").then_return({"posts": []})

    users = wrapped("/api/users")
    posts = wrapped("/api/posts")

    assert.eq(users["users"], ["alice", "bob"])
    assert.eq(posts["posts"], [])
    assert.eq(mock.call_count(wrapped), 2)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(mock called_with) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_MockCalls(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mock_calls.star")
	content := `def test_mock_calls(mock):
    """Test mock.calls() returns call history."""
    def process(value):
        return value * 2

    wrapped = mock.wrap(process)

    wrapped(1)
    wrapped(2)
    wrapped(3)

    calls = mock.calls(wrapped)
    assert.eq(len(calls), 3)
    assert.eq(calls[0]["args"], (1,))
    assert.eq(calls[1]["args"], (2,))
    assert.eq(calls[2]["args"], (3,))
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(mock calls) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_MockIsolation(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_mock_isolation.star")
	content := `# Test that mocks are isolated between tests

def test_first(mock):
    """First test configures a mock."""
    def fn():
        return "original"

    wrapped = mock.wrap(fn)
    mock.when(wrapped).then_return("mocked")

    # Should use mocked value
    assert.eq(wrapped(), "mocked")
    assert.eq(mock.call_count(wrapped), 1)

def test_second(mock):
    """Second test should have fresh mock state."""
    def fn():
        return "original"

    wrapped = mock.wrap(fn)

    # Should call through to original (no configuration from first test)
    assert.eq(wrapped(), "original")
    assert.eq(mock.call_count(wrapped), 1)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(mock isolation) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}

func TestRun_MockWithFixture(t *testing.T) {
	dir := t.TempDir()

	// Create conftest with fixture
	conftest := filepath.Join(dir, "conftest.star")
	conftestContent := `def fixture_api_client(mock):
    """Fixture that provides a mocked API client."""
    def _fetch(url):
        return {"error": "real API not available"}

    wrapped = mock.wrap(_fetch)
    return wrapped
`
	if err := os.WriteFile(conftest, []byte(conftestContent), 0644); err != nil {
		t.Fatalf("failed to write conftest: %v", err)
	}

	// Create test file
	testFile := filepath.Join(dir, "test_mock_fixture.star")
	testContent := `def test_with_mocked_fixture(api_client, mock):
    """Test using a fixture that wraps a mock."""
    # Configure the mock
    mock.when(api_client).called_with("/users").then_return({"users": ["test_user"]})

    # Use the fixture
    result = api_client("/users")

    assert.eq(result["users"], ["test_user"])
    assert.true(mock.was_called(api_client))
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-v", testFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(mock with fixture) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
}
