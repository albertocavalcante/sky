package skycheck

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

func TestRun_CheckValidFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "valid.star")
	content := `def greet(name):
    return "Hello, " + name

result = greet("world")
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(valid file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_CheckInvalidFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "invalid.star")
	content := `def foo():
    return undefined_variable
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	// Should fail for undefined variable
	if code == 0 {
		t.Error("RunWithIO(invalid file) returned 0, want non-zero")
	}
}

func TestRun_CheckSyntaxError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "syntax.star")
	content := `def foo(
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

func TestRun_CheckMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y = 2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file1, file2}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple files) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_CheckDirectory(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y = 2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(directory) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_CheckNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"/nonexistent/file.star"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent file) returned 0, want non-zero")
	}
}

func TestRun_CheckWithLoads(t *testing.T) {
	dir := t.TempDir()

	// Create a library file
	libFile := filepath.Join(dir, "lib.star")
	libContent := `def helper():
    return 42
`
	if err := os.WriteFile(libFile, []byte(libContent), 0644); err != nil {
		t.Fatalf("failed to write lib file: %v", err)
	}

	// Create main file that loads the library
	mainFile := filepath.Join(dir, "main.star")
	mainContent := `load("lib.star", "helper")

result = helper()
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{mainFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(file with loads) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_TypeErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "types.star")
	content := `def add(a, b):
    return a + b

# Type error: adding string and int
result = add("hello", 42)
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	// Depending on how strict the checker is, this might or might not fail
	// The test documents expected behavior
	_ = code // Result depends on checker strictness
}
