package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()

	idx := New(tmpDir)
	if idx == nil {
		t.Fatal("New() returned nil")
	}
	if idx.Root() != tmpDir {
		t.Errorf("Root() = %q, want %q", idx.Root(), tmpDir)
	}
	if idx.Count() != 0 {
		t.Errorf("Count() = %d, want 0", idx.Count())
	}
}

func TestIndex_Add(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	content := `
load("//lib:utils.bzl", "helper")

def my_rule(ctx):
    """My rule docstring."""
    pass

cc_library(
    name = "mylib",
    srcs = ["main.cc"],
)

DEFAULT_VALUE = 42
`
	testFile := filepath.Join(tmpDir, "defs.bzl")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	idx := New(tmpDir)

	// Add the file
	err := idx.Add(testFile)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Verify file was added
	if idx.Count() != 1 {
		t.Errorf("Count() = %d, want 1", idx.Count())
	}

	// Get the file and verify its contents
	f := idx.Get("defs.bzl")
	if f == nil {
		t.Fatal("Get() returned nil")
	}

	if f.Kind != filekind.KindBzl {
		t.Errorf("Kind = %v, want %v", f.Kind, filekind.KindBzl)
	}
	if len(f.Loads) != 1 {
		t.Errorf("len(Loads) = %d, want 1", len(f.Loads))
	}
	if len(f.Defs) != 1 {
		t.Errorf("len(Defs) = %d, want 1", len(f.Defs))
	}
	if len(f.Calls) != 1 {
		t.Errorf("len(Calls) = %d, want 1", len(f.Calls))
	}
	if len(f.Assigns) != 1 {
		t.Errorf("len(Assigns) = %d, want 1", len(f.Assigns))
	}
}

func TestIndex_AddPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"BUILD":         `cc_library(name = "lib")`,
		"defs.bzl":      `def foo(): pass`,
		"pkg/BUILD":     `cc_library(name = "pkg_lib")`,
		"pkg/rules.bzl": `def bar(): pass`,
		"README.md":     "# README",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	idx := New(tmpDir)

	// Add all files with //...
	count, errors := idx.AddPattern("//...")
	if len(errors) > 0 {
		t.Errorf("AddPattern() returned errors: %v", errors)
	}

	// Should have added 4 Starlark files (not README.md)
	if count != 4 {
		t.Errorf("AddPattern() count = %d, want 4", count)
	}
	if idx.Count() != 4 {
		t.Errorf("Count() = %d, want 4", idx.Count())
	}
}

func TestIndex_AddPattern_Subdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"BUILD":         `cc_library(name = "root")`,
		"defs.bzl":      `def root(): pass`,
		"pkg/BUILD":     `cc_library(name = "pkg")`,
		"pkg/rules.bzl": `def pkg(): pass`,
		"other/BUILD":   `cc_library(name = "other")`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	idx := New(tmpDir)

	// Add only files under pkg/
	count, errors := idx.AddPattern("//pkg/...")
	if len(errors) > 0 {
		t.Errorf("AddPattern() returned errors: %v", errors)
	}

	// Should have added 2 files under pkg/
	if count != 2 {
		t.Errorf("AddPattern() count = %d, want 2", count)
	}
}

func TestIndex_Files(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(tmpDir, "a.bzl"), []byte(`def a(): pass`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.bzl"), []byte(`def b(): pass`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	idx := New(tmpDir)
	idx.AddPattern("//...")

	files := idx.Files()
	if len(files) != 2 {
		t.Errorf("Files() returned %d files, want 2", len(files))
	}

	// Verify paths
	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.Path] = true
	}
	if !paths["a.bzl"] {
		t.Error("Files() missing a.bzl")
	}
	if !paths["b.bzl"] {
		t.Error("Files() missing b.bzl")
	}
}

func TestIndex_Get(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	if err := os.WriteFile(filepath.Join(tmpDir, "test.bzl"), []byte(`def test(): pass`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	idx := New(tmpDir)
	idx.AddPattern("//...")

	// Get existing file
	f := idx.Get("test.bzl")
	if f == nil {
		t.Error("Get() returned nil for existing file")
	}

	// Get with absolute path
	f = idx.Get(filepath.Join(tmpDir, "test.bzl"))
	if f == nil {
		t.Error("Get() with absolute path returned nil for existing file")
	}

	// Get non-existing file
	f = idx.Get("nonexistent.bzl")
	if f != nil {
		t.Error("Get() returned non-nil for non-existing file")
	}
}

func TestIndex_Clear(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(tmpDir, "test.bzl"), []byte(`def test(): pass`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	idx := New(tmpDir)
	idx.AddPattern("//...")

	if idx.Count() != 1 {
		t.Errorf("Count() before Clear() = %d, want 1", idx.Count())
	}

	idx.Clear()

	if idx.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", idx.Count())
	}
}

func TestIndex_MatchFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"BUILD":             `cc_library(name = "root")`,
		"defs.bzl":          `def root(): pass`,
		"pkg/BUILD":         `cc_library(name = "pkg")`,
		"pkg/rules.bzl":     `def pkg_rules(): pass`,
		"pkg/utils.bzl":     `def pkg_utils(): pass`,
		"other/BUILD":       `cc_library(name = "other")`,
		"other/script.star": `def other_script(): pass`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	idx := New(tmpDir)
	idx.AddPattern("//...")

	tests := []struct {
		name    string
		pattern string
		want    int
	}{
		{"all files //...", "//...", 7},
		{"pkg recursive //pkg/...", "//pkg/...", 3},
		{"other recursive //other/...", "//other/...", 2},
		{"specific file //defs.bzl", "//defs.bzl", 1},
		{"label style //pkg:rules.bzl", "//pkg:rules.bzl", 1},
		{"glob *.bzl", "*.bzl", 1},                   // Only matches root level
		{"recursive glob **/*.bzl", "**/*.bzl", 3},   // All .bzl files (defs.bzl, pkg/rules.bzl, pkg/utils.bzl)
		{"recursive glob **/*.star", "**/*.star", 1}, // All .star files
		{"non-existent //nonexistent.bzl", "//nonexistent.bzl", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.MatchFiles(tt.pattern)
			if len(got) != tt.want {
				t.Errorf("MatchFiles(%q) returned %d files, want %d", tt.pattern, len(got), tt.want)
				for _, f := range got {
					t.Logf("  - %s", f.Path)
				}
			}
		})
	}
}

func TestIndex_AddNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	idx := New(tmpDir)

	err := idx.Add(filepath.Join(tmpDir, "nonexistent.bzl"))
	if err == nil {
		t.Error("Add() should return error for non-existent file")
	}
}

func TestIndex_AddInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with invalid Starlark syntax
	invalidFile := filepath.Join(tmpDir, "invalid.bzl")
	if err := os.WriteFile(invalidFile, []byte(`def broken(`), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	idx := New(tmpDir)
	err := idx.Add(invalidFile)
	if err == nil {
		t.Error("Add() should return error for invalid Starlark file")
	}
}

func TestIndex_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	for i := 0; i < 10; i++ {
		filename := filepath.Join(tmpDir, filepath.Base(filepath.Join(tmpDir, "test"+string(rune('0'+i))+".bzl")))
		if err := os.WriteFile(filename, []byte(`def test(): pass`), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	idx := New(tmpDir)
	idx.AddPattern("//...")

	// Concurrent reads should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = idx.Files()
			_ = idx.Count()
			_ = idx.Get("test0.bzl")
			_ = idx.MatchFiles("//...")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkIndex_Add(b *testing.B) {
	tmpDir := b.TempDir()

	// Create a test file
	content := `
load("//lib:utils.bzl", "helper", "another")

DEFAULT_VALUE = 42

def my_rule(ctx, name = "default"):
    """My rule docstring."""
    pass

def another_rule(ctx):
    pass

cc_library(
    name = "mylib",
    srcs = ["main.cc"],
    deps = [":other"],
)

go_library(
    name = "golib",
    srcs = ["main.go"],
)
`
	testFile := filepath.Join(tmpDir, "test.bzl")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		b.Fatalf("Failed to create file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := New(tmpDir)
		if err := idx.Add(testFile); err != nil {
			b.Fatalf("Add() error: %v", err)
		}
	}
}

func BenchmarkIndex_MatchFiles(b *testing.B) {
	tmpDir := b.TempDir()

	// Create many test files
	for i := 0; i < 100; i++ {
		dir := filepath.Join(tmpDir, "pkg"+string(rune('0'+i/10)), "sub"+string(rune('0'+i%10)))
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "BUILD"), []byte(`cc_library(name = "lib")`), 0644); err != nil {
			b.Fatalf("Failed to create file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "rules.bzl"), []byte(`def rule(): pass`), 0644); err != nil {
			b.Fatalf("Failed to create file: %v", err)
		}
	}

	idx := New(tmpDir)
	idx.AddPattern("//...")

	patterns := []string{"//...", "//pkg0/...", "**/*.bzl", "*.bzl"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		_ = idx.MatchFiles(pattern)
	}
}
