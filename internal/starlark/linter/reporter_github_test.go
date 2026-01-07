package linter

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestGitHubReporter_WarningAnnotation verifies warning annotation format.
func TestGitHubReporter_WarningAnnotation(t *testing.T) {
	reporter := NewGitHubReporter()
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
				Message:  "Test warning message",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	expected := "::warning file=test.star,line=10,col=5,title=test-rule (correctness)::Test warning message"
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain:\n%s\nGot:\n%s", expected, output)
	}
}

// TestGitHubReporter_ErrorAnnotation verifies error annotation format.
func TestGitHubReporter_ErrorAnnotation(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     5,
				Column:   1,
				Rule:     "error-rule",
				Category: "correctness",
				Severity: SeverityError,
				Message:  "Error message",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "::error ") {
		t.Errorf("Expected output to start with '::error ', got: %s", output)
	}
	if !strings.Contains(output, "Error message") {
		t.Errorf("Expected output to contain error message")
	}
}

// TestGitHubReporter_NoticeAnnotation verifies notice annotation for info/hint.
func TestGitHubReporter_NoticeAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
	}{
		{"info", SeverityInfo},
		{"hint", SeverityHint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := NewGitHubReporter()
			result := &Result{
				Files: 1,
				Findings: []Finding{
					{
						FilePath: "test.star",
						Line:     1,
						Rule:     "rule",
						Severity: tt.severity,
						Message:  "Message",
					},
				},
				Errors: []FileError{},
			}

			var buf bytes.Buffer
			if err := reporter.Report(&buf, result); err != nil {
				t.Fatalf("Report failed: %v", err)
			}

			output := buf.String()
			if !strings.HasPrefix(output, "::notice ") {
				t.Errorf("Expected output to start with '::notice ', got: %s", output)
			}
		})
	}
}

// TestGitHubReporter_SpecialCharacters verifies special character escaping.
func TestGitHubReporter_SpecialCharacters(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     1,
				Rule:     "rule",
				Severity: SeverityWarning,
				Message:  "Message with special: %s, %d, \n newline",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	// The output should contain the message (escaping handled by GitHub)
	output := buf.String()
	if !strings.Contains(output, "Message with special") {
		t.Errorf("Expected message to be present in output")
	}
}

// TestGitHubReporter_MultiLineMessage verifies multi-line messages.
func TestGitHubReporter_MultiLineMessage(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     1,
				Rule:     "rule",
				Severity: SeverityWarning,
				Message:  "Line 1\nLine 2\nLine 3",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Line 1") {
		t.Errorf("Expected first line of message")
	}
}

// TestGitHubReporter_EndLineColumn verifies endLine and endColumn parameters.
func TestGitHubReporter_EndLineColumn(t *testing.T) {
	reporter := NewGitHubReporter()
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
				Message:   "Message",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "endLine=15") {
		t.Errorf("Expected endLine=15 in output: %s", output)
	}
	if !strings.Contains(output, "endColumn=20") {
		t.Errorf("Expected endColumn=20 in output: %s", output)
	}
}

// TestGitHubReporter_NoColumn verifies handling when column is 0.
func TestGitHubReporter_NoColumn(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     10,
				Column:   0, // No column
				Rule:     "rule",
				Severity: SeverityWarning,
				Message:  "Message",
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	// Should not include col= when column is 0
	if strings.Contains(output, ",col=0") {
		t.Errorf("Should not include col=0 in output: %s", output)
	}
}

// TestGitHubReporter_Sorting verifies findings are sorted.
func TestGitHubReporter_Sorting(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 2,
		Findings: []Finding{
			{FilePath: "b.star", Line: 2, Rule: "rule", Severity: SeverityWarning, Message: "b2"},
			{FilePath: "a.star", Line: 5, Rule: "rule", Severity: SeverityWarning, Message: "a5"},
			{FilePath: "a.star", Line: 1, Column: 10, Rule: "rule", Severity: SeverityWarning, Message: "a1c10"},
			{FilePath: "b.star", Line: 1, Rule: "rule", Severity: SeverityWarning, Message: "b1"},
			{FilePath: "a.star", Line: 1, Column: 5, Rule: "rule", Severity: SeverityWarning, Message: "a1c5"},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("Expected 5 lines, got %d", len(lines))
	}

	// Expected order: a.star:1:5, a.star:1:10, a.star:5, b.star:1, b.star:2
	expectedMessages := []string{"a1c5", "a1c10", "a5", "b1", "b2"}
	for i, line := range lines {
		if !strings.Contains(line, expectedMessages[i]) {
			t.Errorf("Line %d: expected to contain %s, got: %s", i, expectedMessages[i], line)
		}
	}
}

// TestGitHubReporter_FileErrors verifies file errors are reported.
func TestGitHubReporter_FileErrors(t *testing.T) {
	reporter := NewGitHubReporter()
	testErr := errors.New("test error")
	result := &Result{
		Files:    1,
		Findings: []Finding{},
		Errors: []FileError{
			{Path: "error.star", Err: testErr},
		},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "::error ") {
		t.Errorf("File error should be reported as ::error")
	}
	if !strings.Contains(output, "error.star") {
		t.Errorf("Error should mention file path")
	}
}

// TestGitHubReporter_EmptyResult verifies handling of empty result.
func TestGitHubReporter_EmptyResult(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files:    0,
		Findings: []Finding{},
		Errors:   []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("Expected empty output, got: %s", buf.String())
	}
}

// TestGitHubReporter_NoRuleOrCategory verifies handling when rule/category are empty.
func TestGitHubReporter_NoRuleOrCategory(t *testing.T) {
	tests := []struct {
		name     string
		rule     string
		category string
		want     string
	}{
		{"both empty", "", "", "title=lint"},
		{"only category", "", "style", "title=style"},
		{"only rule", "test-rule", "", "title=test-rule"},
		{"both present", "test-rule", "style", "title=test-rule (style)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := NewGitHubReporter()
			result := &Result{
				Files: 1,
				Findings: []Finding{
					{
						FilePath: "test.star",
						Line:     1,
						Rule:     tt.rule,
						Category: tt.category,
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

			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("Expected output to contain %s, got: %s", tt.want, output)
			}
		})
	}
}

// TestGitHubReporter_UnicodePath verifies Unicode in file paths.
func TestGitHubReporter_UnicodePath(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "path/文件.star",
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

	output := buf.String()
	if !strings.Contains(output, "path/文件.star") {
		t.Errorf("Unicode path not preserved in output")
	}
}

// TestGitHubReporter_LongMessage verifies handling of very long messages.
func TestGitHubReporter_LongMessage(t *testing.T) {
	reporter := NewGitHubReporter()

	// Create a very long message
	longMessage := strings.Repeat("This is a very long message. ", 100)

	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath: "test.star",
				Line:     1,
				Rule:     "rule",
				Severity: SeverityWarning,
				Message:  longMessage,
			},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	// Should complete without error
	output := buf.String()
	if !strings.Contains(output, "This is a very long message") {
		t.Errorf("Long message not in output")
	}
}

// TestGitHubReporter_MultipleFiles verifies handling of multiple files.
func TestGitHubReporter_MultipleFiles(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 3,
		Findings: []Finding{
			{FilePath: "a.star", Line: 1, Rule: "rule1", Severity: SeverityError, Message: "error in a"},
			{FilePath: "b.star", Line: 1, Rule: "rule2", Severity: SeverityWarning, Message: "warning in b"},
			{FilePath: "c.star", Line: 1, Rule: "rule3", Severity: SeverityInfo, Message: "info in c"},
		},
		Errors: []FileError{},
	}

	var buf bytes.Buffer
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of output, got %d", len(lines))
	}

	// Verify each file is mentioned
	for _, file := range []string{"a.star", "b.star", "c.star"} {
		if !strings.Contains(output, file) {
			t.Errorf("Expected output to contain %s", file)
		}
	}
}

// TestGitHubReporter_EndLineEqualsLine verifies handling when endLine equals line.
func TestGitHubReporter_EndLineEqualsLine(t *testing.T) {
	reporter := NewGitHubReporter()
	result := &Result{
		Files: 1,
		Findings: []Finding{
			{
				FilePath:  "test.star",
				Line:      10,
				EndLine:   10, // Same as Line
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

	output := buf.String()
	// Should not include endLine when it equals Line
	if strings.Contains(output, "endLine=10") {
		t.Errorf("Should not include endLine when it equals Line: %s", output)
	}
}
