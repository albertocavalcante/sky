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

func TestRun_QueryLoads(t *testing.T) {
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
	code := RunWithIO(context.Background(), []string{"-workspace", dir, "loads(//...)"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(loads query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should list lib.star as the loaded module
	if !strings.Contains(stdout.String(), "lib.star") {
		t.Errorf("loads query did not return lib.star\noutput: %s", stdout.String())
	}
}

func TestRun_QueryLoadedBy(t *testing.T) {
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
	code := RunWithIO(context.Background(), []string{"-workspace", dir, `loadedby("lib.star")`}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(loadedby query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should list main.star as a file that loads lib.star
	if !strings.Contains(stdout.String(), "main.star") {
		t.Errorf("loadedby query did not return main.star\noutput: %s", stdout.String())
	}
}

func TestRun_QueryFiles(t *testing.T) {
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
	code := RunWithIO(context.Background(), []string{"-workspace", dir, "files(//...)"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(files query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "a.star") {
		t.Errorf("files query did not return a.star\noutput: %s", output)
	}
	if !strings.Contains(output, "b.star") {
		t.Errorf("files query did not return b.star\noutput: %s", output)
	}
}

func TestRun_QueryCalls(t *testing.T) {
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
	code := RunWithIO(context.Background(), []string{"-workspace", dir, "calls(library, //...)"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(calls query) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should find the library call
	if !strings.Contains(stdout.String(), "library") {
		t.Errorf("calls query did not return library call\noutput: %s", stdout.String())
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
	code := RunWithIO(context.Background(), []string{"-workspace", dir, `filter(".*_test\.star", files(//...))`}, nil, &stdout, &stderr)

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

	// Test the supported output formats: name, location, json, count
	formats := []string{"name", "location", "json", "count"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-workspace", dir, "-output", format, "files(//...)"}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-output %s) returned %d, want 0\nstderr: %s", format, code, stderr.String())
			}

			// Verify output is not empty (should have at least one file)
			if stdout.Len() == 0 {
				t.Errorf("RunWithIO(-output %s) produced no output", format)
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
