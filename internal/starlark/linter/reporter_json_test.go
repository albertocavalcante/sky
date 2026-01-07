package linter

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestJSONReporter_EmptyResult verifies JSON output for empty result.
func TestJSONReporter_EmptyResult(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files:    0,
		Findings: []Finding{},
		Errors:   []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(output.Files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(output.Files))
	}
	if output.Summary.TotalFindings != 0 {
		t.Errorf("Expected 0 total findings, got %d", output.Summary.TotalFindings)
	}
}

// TestJSONReporter_SingleFinding verifies JSON output for single finding.
func TestJSONReporter_SingleFinding(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     10,
				Column:   5,
				Rule:     "test-rule",
				Category: "correctness",
				Severity: SeverityWarning,
				Message:  "Test message",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(output.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(output.Files))
	}

	file := output.Files[0]
	if file.Path != "test.star" {
		t.Errorf("File path: got %s, want test.star", file.Path)
	}

	if len(file.Findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(file.Findings))
	}

	finding := file.Findings[0]
	if finding.Rule != "test-rule" {
		t.Errorf("Rule: got %s, want test-rule", finding.Rule)
	}
	if finding.Line != 10 {
		t.Errorf("Line: got %d, want 10", finding.Line)
	}
	if finding.Column != 5 {
		t.Errorf("Column: got %d, want 5", finding.Column)
	}
}

// TestJSONReporter_MultipleFindings verifies JSON output for multiple findings across files.
func TestJSONReporter_MultipleFindings(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 3,
		Findings: []Finding{
			{FilePath: "b.star", Line: 2, Column: 1, Rule: "rule2", Category: "style", Severity: SeverityInfo, Message: "msg2"},
			{FilePath: "a.star", Line: 1, Column: 1, Rule: "rule1", Category: "correctness", Severity: SeverityError, Message: "msg1"},
			{FilePath: "c.star", Line: 3, Column: 1, Rule: "rule3", Category: "performance", Severity: SeverityHint, Message: "msg3"},
			{FilePath: "a.star", Line: 5, Column: 10, Rule: "rule1", Category: "correctness", Severity: SeverityWarning, Message: "msg4"},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Files should be sorted alphabetically
	if len(output.Files) != 3 {
		t.Fatalf("Expected 3 files, got %d", len(output.Files))
	}

	if output.Files[0].Path != "a.star" {
		t.Errorf("First file should be a.star, got %s", output.Files[0].Path)
	}
	if output.Files[1].Path != "b.star" {
		t.Errorf("Second file should be b.star, got %s", output.Files[1].Path)
	}
	if output.Files[2].Path != "c.star" {
		t.Errorf("Third file should be c.star, got %s", output.Files[2].Path)
	}

	// Findings within a.star should be sorted by line
	aFindings := output.Files[0].Findings
	if len(aFindings) != 2 {
		t.Fatalf("Expected 2 findings in a.star, got %d", len(aFindings))
	}
	if aFindings[0].Line != 1 {
		t.Errorf("First finding in a.star should be line 1, got %d", aFindings[0].Line)
	}
	if aFindings[1].Line != 5 {
		t.Errorf("Second finding in a.star should be line 5, got %d", aFindings[1].Line)
	}
}

// TestJSONReporter_AllSeverities verifies all severity levels are handled.
func TestJSONReporter_AllSeverities(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{FilePath: "test.star", Line: 1, Rule: "r1", Severity: SeverityError, Message: "error"},
			{FilePath: "test.star", Line: 2, Rule: "r2", Severity: SeverityWarning, Message: "warning"},
			{FilePath: "test.star", Line: 3, Rule: "r3", Severity: SeverityInfo, Message: "info"},
			{FilePath: "test.star", Line: 4, Rule: "r4", Severity: SeverityHint, Message: "hint"},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Summary.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", output.Summary.Errors)
	}
	if output.Summary.Warnings != 1 {
		t.Errorf("Expected 1 warning, got %d", output.Summary.Warnings)
	}
	if output.Summary.Infos != 1 {
		t.Errorf("Expected 1 info, got %d", output.Summary.Infos)
	}
	if output.Summary.Hints != 1 {
		t.Errorf("Expected 1 hint, got %d", output.Summary.Hints)
	}
}

// TestJSONReporter_Summary verifies summary statistics are accurate.
func TestJSONReporter_Summary(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 2,
		Findings: []Finding{
			{FilePath: "a.star", Rule: "rule1", Category: "cat1", Severity: SeverityError},
			{FilePath: "a.star", Rule: "rule1", Category: "cat1", Severity: SeverityWarning},
			{FilePath: "b.star", Rule: "rule2", Category: "cat2", Severity: SeverityInfo},
			{FilePath: "b.star", Rule: "rule1", Category: "cat1", Severity: SeverityHint},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	summary := output.Summary

	// Verify total files and findings
	if summary.TotalFiles != 2 {
		t.Errorf("TotalFiles: got %d, want 2", summary.TotalFiles)
	}
	if summary.TotalFindings != 4 {
		t.Errorf("TotalFindings: got %d, want 4", summary.TotalFindings)
	}

	// Verify by severity
	if summary.BySeverity["error"] != 1 {
		t.Errorf("BySeverity[error]: got %d, want 1", summary.BySeverity["error"])
	}
	if summary.BySeverity["warning"] != 1 {
		t.Errorf("BySeverity[warning]: got %d, want 1", summary.BySeverity["warning"])
	}

	// Verify by rule
	if summary.ByRule["rule1"] != 3 {
		t.Errorf("ByRule[rule1]: got %d, want 3", summary.ByRule["rule1"])
	}
	if summary.ByRule["rule2"] != 1 {
		t.Errorf("ByRule[rule2]: got %d, want 1", summary.ByRule["rule2"])
	}

	// Verify by category
	if summary.ByCategory["cat1"] != 3 {
		t.Errorf("ByCategory[cat1]: got %d, want 3", summary.ByCategory["cat1"])
	}
	if summary.ByCategory["cat2"] != 1 {
		t.Errorf("ByCategory[cat2]: got %d, want 1", summary.ByCategory["cat2"])
	}
}

// TestJSONReporter_ValidJSONOutput verifies the output is valid JSON.
func TestJSONReporter_ValidJSONOutput(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{FilePath: "test.star", Line: 1, Rule: "rule", Severity: SeverityWarning, Message: "test"},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	// Verify it's valid JSON by unmarshaling
	var output interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}
}

// TestJSONReporter_FileErrors verifies file errors in output.
func TestJSONReporter_FileErrors(t *testing.T) {
	reporter := NewJSONReporter()
	testErr := errors.New("test error")
	result := &Result{
		Files:    2,
		Findings: []Finding{},
		Errors: []FileError{
			{Path: "error1.star", Err: testErr},
			{Path: "error2.star", Err: testErr},
		},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(output.Summary.FileErrors) != 2 {
		t.Errorf("Expected 2 file errors, got %d", len(output.Summary.FileErrors))
	}
}

// TestJSONReporter_EndLineColumn verifies EndLine and EndColumn are included.
func TestJSONReporter_EndLineColumn(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath:  "test.star",
				Line:      10,
				Column:    5,
				EndLine:   15,
				EndColumn: 20,
				Rule:      "rule",
				Severity:  SeverityWarning,
				Message:   "test",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	finding := output.Files[0].Findings[0]
	if finding.EndLine != 15 {
		t.Errorf("EndLine: got %d, want 15", finding.EndLine)
	}
	if finding.EndColumn != 20 {
		t.Errorf("EndColumn: got %d, want 20", finding.EndColumn)
	}
}

// TestJSONReporter_UnicodeInMessages verifies Unicode characters are handled correctly.
func TestJSONReporter_UnicodeInMessages(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     1,
				Rule:     "rule",
				Severity: SeverityWarning,
				Message:  "Unicode: 你好世界 🚀",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	finding := output.Files[0].Findings[0]
	if finding.Message != "Unicode: 你好世界 🚀" {
		t.Errorf("Unicode message not preserved: %s", finding.Message)
	}
}

// TestJSONReporter_SpecialCharactersInPath verifies special characters in file paths.
func TestJSONReporter_SpecialCharactersInPath(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "path/with spaces/file.star",
				Line:     1,
				Rule:     "rule",
				Severity: SeverityWarning,
				Message:  "test",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Files[0].Path != "path/with spaces/file.star" {
		t.Errorf("Path with spaces not preserved")
	}
}

// TestJSONReporter_Deterministic verifies output is deterministic.
func TestJSONReporter_Deterministic(t *testing.T) {
	reporter := NewJSONReporter()
	result := &Result{
		Files: 3,
		Findings: []Finding{
			{FilePath: "c.star", Line: 1, Rule: "rule1", Severity: SeverityWarning, Message: "c1"},
			{FilePath: "a.star", Line: 2, Column: 5, Rule: "rule2", Severity: SeverityError, Message: "a2"},
			{FilePath: "b.star", Line: 1, Rule: "rule3", Severity: SeverityInfo, Message: "b1"},
			{FilePath: "a.star", Line: 1, Rule: "rule1", Severity: SeverityWarning, Message: "a1"},
		},
		Errors: []FileError{},
	}

	// Run twice and compare
	var buf1, buf2 bytes.Buffer
	if err := reporter.Report(&buf1, result); err != nil {
		t.Fatalf("First report failed: %v", err)
	}
	if err := reporter.Report(&buf2, result); err != nil {
		t.Fatalf("Second report failed: %v", err)
	}

	if diff := cmp.Diff(buf1.String(), buf2.String()); diff != "" {
		t.Errorf("Output is not deterministic (-first +second):\n%s", diff)
	}

	// Verify sorting
	var output jsonOutput
	if err := json.Unmarshal(buf1.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Files should be sorted: a.star, b.star, c.star
	expectedOrder := []string{"a.star", "b.star", "c.star"}
	for i, file := range output.Files {
		if file.Path != expectedOrder[i] {
			t.Errorf("File %d: got %s, want %s", i, file.Path, expectedOrder[i])
		}
	}

	// Findings in a.star should be sorted by line, then column
	aFindings := output.Files[0].Findings
	if aFindings[0].Line != 1 || aFindings[1].Line != 2 {
		t.Error("Findings in a.star not sorted by line")
	}
}

// TestJSONReporter_LargeNumberOfFindings verifies performance with many findings.
func TestJSONReporter_LargeNumberOfFindings(t *testing.T) {
	reporter := NewJSONReporter()

	// Create 1000 findings
	findings := make([]Finding, 1000)
	for i := 0; i < 1000; i++ {
		findings[i] = Finding{
			FilePath: "test.star",
			Line:     i + 1,
			Rule:     "rule",
			Severity: SeverityWarning,
			Message:  "test",
		}
	}

	result := &Result{
		Files:    1,
		Findings: findings,
		Errors:   []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	// Just verify it succeeds
	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Summary.TotalFindings != 1000 {
		t.Errorf("Expected 1000 findings, got %d", output.Summary.TotalFindings)
	}
}
