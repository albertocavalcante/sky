package skydoc

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

func TestRun_GenerateDocForFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "lib.star")
	content := `"""A sample library for testing.

This library provides utility functions.
"""

def greet(name):
    """Greet someone by name.

    Args:
        name: The name of the person to greet.

    Returns:
        A greeting string.
    """
    return "Hello, " + name

def add(a, b):
    """Add two numbers.

    Args:
        a: First number.
        b: Second number.

    Returns:
        The sum of a and b.
    """
    return a + b

# A constant
DEFAULT_GREETING = "Hello"
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()

	// Check that functions are documented
	if !strings.Contains(output, "greet") {
		t.Error("output does not contain 'greet' function")
	}
	if !strings.Contains(output, "add") {
		t.Error("output does not contain 'add' function")
	}
}

func TestRun_OutputFormats(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "lib.star")
	content := `def foo():
    """A simple function."""
    pass
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	formats := []struct {
		name  string
		flag  string
		check string
	}{
		{"markdown", "md", "#"},
		{"json", "json", "{"},
		{"html", "html", "<"},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-format", tc.flag, file}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-format %s) returned %d, want 0\nstderr: %s", tc.flag, code, stderr.String())
			}

			if !strings.Contains(stdout.String(), tc.check) {
				t.Errorf("output for format %s does not contain expected %q", tc.flag, tc.check)
			}
		})
	}
}

func TestRun_OutputToFile(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "lib.star")
	outputFile := filepath.Join(dir, "lib.md")

	content := `def foo():
    """A function."""
    pass
`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-o", outputFile, inputFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-o file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Check output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("output file was not created")
	}
}

func TestRun_NoDocstrings(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "nodocs.star")
	content := `def foo():
    pass

def bar():
    pass
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	// Should still work, just with minimal docs
	if code != 0 {
		t.Errorf("RunWithIO(file without docstrings) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("def foo():\n    pass\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("def bar():\n    pass\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file1, file2}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple files) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.star")
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

func TestRun_NonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"/nonexistent/file.star"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent file) returned 0, want non-zero")
	}
}
