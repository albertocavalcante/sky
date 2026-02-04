package skycov

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

func TestRun_CoverageReport(t *testing.T) {
	dir := t.TempDir()

	// Create a library file
	libFile := filepath.Join(dir, "lib.star")
	libContent := `def add(a, b):
    return a + b

def subtract(a, b):
    return a - b

def multiply(a, b):
    return a * b
`
	if err := os.WriteFile(libFile, []byte(libContent), 0644); err != nil {
		t.Fatalf("failed to write lib file: %v", err)
	}

	// Create a test file that only covers some functions
	testFile := filepath.Join(dir, "lib_test.star")
	testContent := `load("lib.star", "add", "subtract")

def test_add():
    assert_eq(add(1, 2), 3)

def test_subtract():
    assert_eq(subtract(5, 3), 2)

# Note: multiply is not tested
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{testFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(coverage) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should show coverage information
	output := stdout.String()
	if !strings.Contains(output, "coverage") && !strings.Contains(output, "%") {
		t.Errorf("output does not contain coverage info\noutput: %s", output)
	}
}

func TestRun_CoverageOutputFormats(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "lib.star")
	if err := os.WriteFile(file, []byte("def foo():\n    return 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	testFile := filepath.Join(dir, "lib_test.star")
	testContent := `load("lib.star", "foo")

def test_foo():
    assert_eq(foo(), 1)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	formats := []struct {
		name string
		flag string
	}{
		{"text", "text"},
		{"json", "json"},
		{"html", "html"},
		{"lcov", "lcov"},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-format", tc.flag, testFile}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-format %s) returned %d, want 0\nstderr: %s", tc.flag, code, stderr.String())
			}
		})
	}
}

func TestRun_CoverageOutputToFile(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "lib.star")
	if err := os.WriteFile(file, []byte("def foo():\n    return 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	testFile := filepath.Join(dir, "lib_test.star")
	testContent := `load("lib.star", "foo")

def test_foo():
    assert_eq(foo(), 1)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	outputFile := filepath.Join(dir, "coverage.txt")

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-o", outputFile, testFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-o file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Check output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("output file was not created")
	}
}

func TestRun_CoverageThreshold(t *testing.T) {
	dir := t.TempDir()

	// Create a file with partial coverage
	file := filepath.Join(dir, "lib.star")
	content := `def covered():
    return 1

def not_covered():
    return 2
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	testFile := filepath.Join(dir, "lib_test.star")
	testContent := `load("lib.star", "covered")

def test_covered():
    assert_eq(covered(), 1)
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("threshold met", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), []string{"-threshold", "40", testFile}, nil, &stdout, &stderr)

		if code != 0 {
			t.Errorf("RunWithIO(-threshold 40) returned %d, want 0\nstderr: %s", code, stderr.String())
		}
	})

	t.Run("threshold not met", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), []string{"-threshold", "90", testFile}, nil, &stdout, &stderr)

		if code == 0 {
			t.Error("RunWithIO(-threshold 90) returned 0, want non-zero for low coverage")
		}
	})
}

func TestRun_NoTestFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{}, nil, &stdout, &stderr)

	// Should show usage or error when no files provided
	if code == 0 && stdout.Len() == 0 && stderr.Len() == 0 {
		t.Error("RunWithIO() with no args produced no output")
	}
}

func TestRun_NonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"/nonexistent/test.star"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent file) returned 0, want non-zero")
	}
}
