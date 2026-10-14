package skylint

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestRun_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{}, nil, &stdout, &stderr)

	// Should show usage or error when no files provided
	if code == 0 && stdout.Len() == 0 && stderr.Len() == 0 {
		t.Error("RunWithIO() with no args produced no output")
	}
}

func TestRun_LintCleanFile(t *testing.T) {
	// Create a temporary clean Starlark file
	dir := t.TempDir()
	file := filepath.Join(dir, "clean.star")
	content := `"""A clean module."""

def greet(name):
    """Greet someone by name."""
    return "Hello, " + name
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(clean file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_LintFileWithIssues(t *testing.T) {
	// Create a temporary file with lint issues
	dir := t.TempDir()
	file := filepath.Join(dir, "issues.star")
	content := `def BadName():  # function name should be snake_case
    x = 1  # unused variable
    return None
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	// Should return non-zero for files with issues
	if code == 0 {
		t.Error("RunWithIO(file with issues) returned 0, want non-zero")
	}
}

func TestRun_LintMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("\"\"\"Module a.\"\"\"\n\nx = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("\"\"\"Module b.\"\"\"\n\ny = 2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file1, file2}, nil, &stdout, &stderr)

	// Should process multiple files
	if code != 0 {
		t.Errorf("RunWithIO(multiple files) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_LintNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"/nonexistent/file.star"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent file) returned 0, want non-zero")
	}
}

func TestRun_LintDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create some .star files in the directory
	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("\"\"\"Module a.\"\"\"\n\nx = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("\"\"\"Module b.\"\"\"\n\ny = 2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{dir}, nil, &stdout, &stderr)

	// Should be able to lint a directory
	if code != 0 {
		t.Errorf("RunWithIO(directory) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_OutputFormats(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.star")
	if err := os.WriteFile(file, []byte("\"\"\"Test module.\"\"\"\n\nx = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	formats := []string{"text", "json", "github"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-format", format, file}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-format %s) returned %d, want 0\nstderr: %s", format, code, stderr.String())
			}
		})
	}
}

const fixableSrc = "load(\"//a:b.bzl\", \"x\")\n\ndef f(a):\n    return 1\n"

func TestRun_FixExitReflectsRemainingFindings(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.bzl")
	if err := os.WriteFile(file, []byte(fixableSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--fix", "--format", "json", file}, nil, &stdout, &stderr)
	if code != exitWarning {
		t.Errorf("--fix with remaining warnings: code = %d, want %d\nstderr: %s", code, exitWarning, stderr.String())
	}
	var parsed any
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Errorf("--format json ignored in fix mode: %v\n%s", err, stdout.String())
	}
	b, _ := os.ReadFile(file)
	if strings.Contains(string(b), "load(") {
		t.Errorf("unused load was not fixed: %s", b)
	}
}

func TestRun_FixDiffDoesNotWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.bzl")
	if err := os.WriteFile(file, []byte(fixableSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--fix", "--diff", file}, nil, &stdout, &stderr)
	if code != exitWarning {
		t.Errorf("code = %d, want %d", code, exitWarning)
	}
	if b, _ := os.ReadFile(file); string(b) != fixableSrc {
		t.Errorf("--diff modified file: %s", b)
	}
}

func TestRun_FlagValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.bzl")
	if err := os.WriteFile(file, []byte(fixableSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--format", "bogus", file},
		{"--fix", "--format", "bogus", file},
		{"--diff", file},
	} {
		var stdout, stderr bytes.Buffer
		if code := RunWithIO(context.Background(), args, nil, &stdout, &stderr); code != exitError {
			t.Errorf("%v: code = %d, want %d", args, code, exitError)
		}
	}
	if b, _ := os.ReadFile(file); string(b) != fixableSrc {
		t.Errorf("file modified by invalid invocation: %s", b)
	}
}
