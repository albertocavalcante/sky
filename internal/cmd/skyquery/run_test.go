package skyquery

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

func TestRun_QueryDeps(t *testing.T) {
	dir := t.TempDir()

	// Create a simple dependency graph
	libFile := filepath.Join(dir, "lib.star")
	if err := os.WriteFile(libFile, []byte("def helper():\n    return 42\n"), 0644); err != nil {
		t.Fatalf("failed to write lib file: %v", err)
	}

	mainFile := filepath.Join(dir, "main.star")
	mainContent := `load("lib.star", "helper")

result = helper()
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"deps(" + mainFile + ")"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(deps query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should list lib.star as dependency
	if !strings.Contains(stdout.String(), "lib.star") {
		t.Errorf("deps query did not return lib.star\noutput: %s", stdout.String())
	}
}

func TestRun_QueryRdeps(t *testing.T) {
	dir := t.TempDir()

	// Create a dependency graph
	libFile := filepath.Join(dir, "lib.star")
	if err := os.WriteFile(libFile, []byte("def helper():\n    return 42\n"), 0644); err != nil {
		t.Fatalf("failed to write lib file: %v", err)
	}

	mainFile := filepath.Join(dir, "main.star")
	mainContent := `load("lib.star", "helper")
result = helper()
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"rdeps(" + dir + ", " + libFile + ")"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(rdeps query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should list main.star as reverse dependency
	if !strings.Contains(stdout.String(), "main.star") {
		t.Errorf("rdeps query did not return main.star\noutput: %s", stdout.String())
	}
}

func TestRun_QueryAllFiles(t *testing.T) {
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
	code := RunWithIO(context.Background(), []string{"allfiles(" + dir + ")"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(allfiles query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "a.star") {
		t.Errorf("allfiles query did not return a.star\noutput: %s", output)
	}
	if !strings.Contains(output, "b.star") {
		t.Errorf("allfiles query did not return b.star\noutput: %s", output)
	}
}

func TestRun_QueryKind(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "build.star")
	content := `def library(name, srcs):
    pass

def binary(name, deps):
    pass

library(name = "mylib", srcs = ["a.star"])
binary(name = "mybin", deps = [":mylib"])
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"kind(library, " + dir + ")"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(kind query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_QueryFilter(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "foo_test.star")
	file2 := filepath.Join(dir, "bar_test.star")
	file3 := filepath.Join(dir, "lib.star")

	if err := os.WriteFile(file1, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("y = 2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(file3, []byte("z = 3\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{`filter(".*_test\.star", allfiles(` + dir + `))`}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(filter query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "foo_test.star") {
		t.Errorf("filter query did not return foo_test.star\noutput: %s", output)
	}
	if !strings.Contains(output, "bar_test.star") {
		t.Errorf("filter query did not return bar_test.star\noutput: %s", output)
	}
	if strings.Contains(output, "lib.star") {
		t.Errorf("filter query incorrectly returned lib.star\noutput: %s", output)
	}
}

func TestRun_OutputFormats(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "test.star")
	if err := os.WriteFile(file, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	formats := []string{"label", "json", "proto"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-output", format, "allfiles(" + dir + ")"}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-output %s) returned %d, want 0\nstderr: %s", format, code, stderr.String())
			}
		})
	}
}

func TestRun_InvalidQuery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"invalid_function()"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(invalid query) returned 0, want non-zero")
	}
}

func TestRun_NoQuery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{}, nil, &stdout, &stderr)

	// Should show usage or error when no query provided
	if code == 0 && stdout.Len() == 0 && stderr.Len() == 0 {
		t.Error("RunWithIO() with no args produced no output")
	}
}
