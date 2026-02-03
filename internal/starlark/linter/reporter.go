package linter

import (
	"fmt"
	"io"
	"strings"

	"github.com/albertocavalcante/sky/internal/starlark/sortutil"
)

// Reporter formats and outputs lint results.
type Reporter interface {
	// Report writes the lint results to the writer.
	Report(w io.Writer, result *Result) error
}

// TextReporter outputs findings in human-readable text format.
type TextReporter struct {
	// ShowRule includes the rule name in the output
	ShowRule bool

	// ShowCategory includes the category in the output
	ShowCategory bool

	// ColorOutput enables colored output (for terminals)
	ColorOutput bool
}

// NewTextReporter creates a new text reporter with default settings.
func NewTextReporter() *TextReporter {
	return &TextReporter{
		ShowRule:     true,
		ShowCategory: false,
		ColorOutput:  false,
	}
}

// Report implements the Reporter interface for text output.
func (r *TextReporter) Report(w io.Writer, result *Result) error {
	if len(result.Findings) == 0 && len(result.Errors) == 0 {
		return nil
	}

	// Sort findings by file, then line, then column
	sortedFindings := make([]Finding, len(result.Findings))
	copy(sortedFindings, result.Findings)
	sortutil.ByLocation(sortedFindings,
		func(f Finding) string { return f.FilePath },
		func(f Finding) int { return f.Line },
		func(f Finding) int { return f.Column },
	)

	// Output findings grouped by file
	var currentFile string
	for _, finding := range sortedFindings {
		// Print file header if it changed
		if finding.FilePath != currentFile {
			if currentFile != "" {
				if _, err := fmt.Fprintln(w); err != nil { // Blank line between files
					return err
				}
			}
			currentFile = finding.FilePath
		}

		if err := r.reportFinding(w, finding); err != nil {
			return err
		}
	}

	// Report errors
	for _, fileErr := range result.Errors {
		if _, err := fmt.Fprintf(w, "Error processing %s: %v\n", fileErr.Path, fileErr.Err); err != nil {
			return err
		}
	}

	// Summary
	if len(sortedFindings) > 0 {
		if _, err := fmt.Fprintf(w, "\n"); err != nil {
			return err
		}
		r.reportSummary(w, result)
	}

	return nil
}

// reportFinding outputs a single finding.
func (r *TextReporter) reportFinding(w io.Writer, f Finding) error {
	// Build the output line
	var parts []string

	// File path and location: file:line:column
	if f.Column > 0 {
		parts = append(parts, fmt.Sprintf("%s:%d:%d:", f.FilePath, f.Line, f.Column))
	} else {
		parts = append(parts, fmt.Sprintf("%s:%d:", f.FilePath, f.Line))
	}

	// Severity
	severity := r.formatSeverity(f.Severity)
	parts = append(parts, severity)

	// Message
	parts = append(parts, f.Message)

	// Rule name
	if r.ShowRule && f.Rule != "" {
		parts = append(parts, fmt.Sprintf("(%s)", f.Rule))
	}

	// Category
	if r.ShowCategory && f.Category != "" {
		parts = append(parts, fmt.Sprintf("[%s]", f.Category))
	}

	if _, err := fmt.Fprintln(w, strings.Join(parts, " ")); err != nil {
		return err
	}

	return nil
}

// formatSeverity formats the severity for display.
func (r *TextReporter) formatSeverity(s Severity) string {
	switch s {
	case SeverityError:
		if r.ColorOutput {
			return "\033[31merror:\033[0m" // Red
		}
		return "error:"
	case SeverityWarning:
		if r.ColorOutput {
			return "\033[33mwarning:\033[0m" // Yellow
		}
		return "warning:"
	case SeverityInfo:
		if r.ColorOutput {
			return "\033[36minfo:\033[0m" // Cyan
		}
		return "info:"
	case SeverityHint:
		if r.ColorOutput {
			return "\033[90mhint:\033[0m" // Gray
		}
		return "hint:"
	default:
		return "unknown:"
	}
}

// reportSummary outputs a summary of the results.
func (r *TextReporter) reportSummary(w io.Writer, result *Result) {
	errors := result.ErrorCount()
	warnings := result.WarningCount()

	var parts []string

	if errors > 0 {
		word := "error"
		if errors > 1 {
			word = "errors"
		}
		parts = append(parts, fmt.Sprintf("%d %s", errors, word))
	}

	if warnings > 0 {
		word := "warning"
		if warnings > 1 {
			word = "warnings"
		}
		parts = append(parts, fmt.Sprintf("%d %s", warnings, word))
	}

	if len(parts) > 0 {
		_, _ = fmt.Fprintf(w, "Found %s in %d file(s)\n", strings.Join(parts, ", "), result.Files)
	}
}

// FileReporter groups findings by file for clearer output.
type FileReporter struct {
	TextReporter
}

// NewFileReporter creates a new file-grouped reporter.
func NewFileReporter() *FileReporter {
	return &FileReporter{
		TextReporter: TextReporter{
			ShowRule:     true,
			ShowCategory: false,
			ColorOutput:  false,
		},
	}
}

// Report implements the Reporter interface with file grouping.
// For MVP, we use the TextReporter as-is since the functionality is similar.

// CompactReporter outputs findings in a compact, single-line format.
// Format: file:line:column: severity: message (rule)
type CompactReporter struct {
	ColorOutput bool
}

// NewCompactReporter creates a new compact reporter.
func NewCompactReporter() *CompactReporter {
	return &CompactReporter{
		ColorOutput: false,
	}
}

// Report implements the Reporter interface for compact output.
func (r *CompactReporter) Report(w io.Writer, result *Result) error {
	if len(result.Findings) == 0 && len(result.Errors) == 0 {
		return nil
	}

	// Sort findings by file, then line, then column
	sortedFindings := make([]Finding, len(result.Findings))
	copy(sortedFindings, result.Findings)
	sortutil.ByLocation(sortedFindings,
		func(f Finding) string { return f.FilePath },
		func(f Finding) int { return f.Line },
		func(f Finding) int { return f.Column },
	)

	// Output each finding on a single line
	for _, finding := range sortedFindings {
		// Format: file:line:column: severity: message (rule)
		location := fmt.Sprintf("%s:%d:%d:", finding.FilePath, finding.Line, finding.Column)
		severity := r.formatSeverity(finding.Severity)
		var line string
		if finding.Rule != "" {
			line = fmt.Sprintf("%s %s %s (%s)\n", location, severity, finding.Message, finding.Rule)
		} else {
			line = fmt.Sprintf("%s %s %s\n", location, severity, finding.Message)
		}
		if _, err := w.Write([]byte(line)); err != nil {
			return err
		}
	}

	// Report errors
	for _, fileErr := range result.Errors {
		if _, err := fmt.Fprintf(w, "%s: error: %v\n", fileErr.Path, fileErr.Err); err != nil {
			return err
		}
	}

	return nil
}

// formatSeverity formats the severity for display.
func (r *CompactReporter) formatSeverity(s Severity) string {
	switch s {
	case SeverityError:
		return "error:"
	case SeverityWarning:
		return "warning:"
	case SeverityInfo:
		return "info:"
	case SeverityHint:
		return "hint:"
	default:
		return "unknown:"
	}
}
