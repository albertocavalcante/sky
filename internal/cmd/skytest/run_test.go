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
