package skyfmt

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

func TestRun_FormatStdin(t *testing.T) {
	input := `def foo():
  return   1
`
	expected := `def foo():
    return 1
`

	stdin := bytes.NewBufferString(input)
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{}, stdin, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(stdin) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	if stdout.String() != expected {
		t.Errorf("RunWithIO(stdin) output = %q, want %q", stdout.String(), expected)
	}
}

func TestRun_FormatFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.star")
	content := `def foo():
  return   1
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_FormatFileInPlace(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.star")
	content := `def foo():
  return   1
`
	expected := `def foo():
    return 1
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-w", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-w file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Check file was modified in place
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if string(got) != expected {
		t.Errorf("file content = %q, want %q", string(got), expected)
	}
}

func TestRun_CheckMode(t *testing.T) {
	dir := t.TempDir()

	// Well-formatted file
	cleanFile := filepath.Join(dir, "clean.star")
	cleanContent := `def foo():
    return 1
`
	if err := os.WriteFile(cleanFile, []byte(cleanContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Poorly-formatted file
	dirtyFile := filepath.Join(dir, "dirty.star")
	dirtyContent := `def foo():
  return   1
`
	if err := os.WriteFile(dirtyFile, []byte(dirtyContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("clean file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), []string{"-check", cleanFile}, nil, &stdout, &stderr)

		if code != 0 {
			t.Errorf("RunWithIO(-check clean) returned %d, want 0", code)
		}
	})

	t.Run("dirty file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), []string{"-check", dirtyFile}, nil, &stdout, &stderr)

		if code == 0 {
			t.Error("RunWithIO(-check dirty) returned 0, want non-zero")
		}
	})
}

func TestRun_DiffMode(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.star")
	content := `def foo():
  return   1
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-d", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-d file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should output a diff
	if !bytes.Contains(stdout.Bytes(), []byte("-")) && !bytes.Contains(stdout.Bytes(), []byte("+")) {
		t.Error("RunWithIO(-d) did not output a diff")
	}
}

func TestRun_FormatMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("x=1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y=2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{file1, file2}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(multiple files) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_FormatDirectory(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.star")
	file2 := filepath.Join(dir, "b.star")

	if err := os.WriteFile(file1, []byte("x=1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y=2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{dir}, nil, &stdout, &stderr)

	// Should be able to format a directory
	if code != 0 {
		t.Errorf("RunWithIO(directory) returned %d, want 0\nstderr: %s", code, stderr.String())
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
