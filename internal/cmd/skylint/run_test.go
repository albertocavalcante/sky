package skylint

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
	content := `def greet(name):
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

	if err := os.WriteFile(file1, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y = 2\n"), 0644); err != nil {
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

	if err := os.WriteFile(file1, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y = 2\n"), 0644); err != nil {
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
	if err := os.WriteFile(file, []byte("x = 1\n"), 0644); err != nil {
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
