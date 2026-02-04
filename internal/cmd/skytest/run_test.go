package skytest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
    assert_eq(1 + 1, 2)

def test_subtraction():
    assert_eq(5 - 3, 2)

def test_multiplication():
    assert_eq(3 * 4, 12)
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
    assert_eq(1, 2)
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
    assert_eq(1, 1)

def test_fail():
    assert_eq(1, 2)
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
	content := `_counter = [0]

def setup():
    _counter[0] = 0

def test_counter():
    _counter[0] += 1
    assert_eq(_counter[0], 1)
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

	if err := os.WriteFile(file1, []byte("def test_a():\n    assert_eq(1, 1)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("def test_b():\n    assert_eq(2, 2)\n"), 0644); err != nil {
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

	if err := os.WriteFile(file1, []byte("def test_a():\n    assert_eq(1, 1)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("def test_b():\n    assert_eq(2, 2)\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(test directory) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_FilterTests(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_filter.star")
	content := `def test_foo():
    assert_eq(1, 1)

def test_bar():
    assert_eq(2, 2)

def test_baz():
    assert_eq(3, 3)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-run", "foo", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-run filter) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_VerboseOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_verbose.star")
	content := `def test_one():
    assert_eq(1, 1)
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

	// Should pass (no tests to fail) or warn about no tests
	// Behavior depends on implementation
	_ = code
}
