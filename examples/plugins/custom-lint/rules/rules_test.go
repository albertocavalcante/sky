package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNoPrint(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bzl")

	content := `"""Test file."""

def foo():
    print("debug")
    return 1

def bar():
    x = 1
    return x
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := LintFile(testFile)
	if err != nil {
		t.Fatalf("LintFile() error: %v", err)
	}

	// Should find one print statement
	printFindings := filterByRule(findings, "no-print")
	if len(printFindings) != 1 {
		t.Errorf("expected 1 no-print finding, got %d", len(printFindings))
	}
}

func TestMaxParams(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bzl")

	content := `"""Test file."""

def ok(a, b, c):
    pass

def too_many(a, b, c, d, e, f, g):
    pass
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := LintFile(testFile)
	if err != nil {
		t.Fatalf("LintFile() error: %v", err)
	}

	// Should find one function with too many params
	maxParamFindings := filterByRule(findings, "max-params")
	if len(maxParamFindings) != 1 {
		t.Errorf("expected 1 max-params finding, got %d", len(maxParamFindings))
	}
}

func TestNoUnderscore(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bzl")

	content := `"""Test file."""

def _private_with_doc():
    """This is documented."""
    pass

def _private_no_doc():
    pass

def public():
    """Public function."""
    pass
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := LintFile(testFile)
	if err != nil {
		t.Fatalf("LintFile() error: %v", err)
	}

	// Should find one underscore function with docstring
	underscoreFindings := filterByRule(findings, "no-underscore-public")
	if len(underscoreFindings) != 1 {
		t.Errorf("expected 1 no-underscore-public finding, got %d", len(underscoreFindings))
	}
}

func filterByRule(findings []Finding, rule string) []Finding {
	var filtered []Finding
	for _, f := range findings {
		if f.Rule == rule {
			filtered = append(filtered, f)
		}
	}
	return filtered
}
