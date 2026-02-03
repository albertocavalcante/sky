package counter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyze(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bzl")

	content := `"""Test file."""

load("@rules_go//go:def.bzl", "go_library")
load(":utils.bzl", "helper")

MESSAGE = "hello"

def greet(name):
    """Greet someone."""
    return "Hello, " + name

def add(a, b):
    """Add two numbers."""
    return a + b

go_library(
    name = "test",
    srcs = ["test.go"],
)

result = greet("world")
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Analyze(testFile)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if stats.Defs != 2 {
		t.Errorf("Defs = %d, want 2", stats.Defs)
	}

	if stats.Loads != 2 {
		t.Errorf("Loads = %d, want 2", stats.Loads)
	}

	// go_library call + greet call in the def + greet("world") call
	if stats.Calls < 2 {
		t.Errorf("Calls = %d, want at least 2", stats.Calls)
	}

	if stats.Lines < 20 {
		t.Errorf("Lines = %d, want at least 20", stats.Lines)
	}
}

func TestAnalyze_InvalidFile(t *testing.T) {
	_, err := Analyze("/nonexistent/file.bzl")
	if err == nil {
		t.Error("Analyze() expected error for nonexistent file")
	}
}

func TestAnalyze_InvalidSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.bzl")

	content := `def broken(
    # missing closing paren
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Analyze(testFile)
	if err == nil {
		t.Error("Analyze() expected error for invalid syntax")
	}
}
