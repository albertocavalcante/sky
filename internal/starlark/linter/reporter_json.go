package linter

import (
	"encoding/json"
	"io"
)

// JSONReporter outputs findings in JSON format for CI integration.
type JSONReporter struct{}

// NewJSONReporter creates a new JSON reporter.
func NewJSONReporter() *JSONReporter {
	return &JSONReporter{}
}

// jsonOutput represents the root JSON structure.
type jsonOutput struct {
	Files   []jsonFile  `json:"files"`
	Summary jsonSummary `json:"summary"`
}

// jsonFile represents a file and its findings.
type jsonFile struct {
	Path     string        `json:"path"`
	Findings []jsonFinding `json:"findings"`
}

// jsonFinding represents a single lint finding.
type jsonFinding struct {
	Rule      string `json:"rule"`
	Category  string `json:"category"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
}

// jsonSummary represents summary statistics.
type jsonSummary struct {
	TotalFiles    int             `json:"total_files"`
	TotalFindings int             `json:"total_findings"`
	Errors        int             `json:"errors"`
	Warnings      int             `json:"warnings"`
	Infos         int             `json:"infos"`
	Hints         int             `json:"hints"`
	FileErrors    []jsonFileError `json:"file_errors,omitempty"`
	BySeverity    map[string]int  `json:"by_severity"`
	ByRule        map[string]int  `json:"by_rule"`
	ByCategory    map[string]int  `json:"by_category"`
}

// jsonFileError represents an error that occurred while processing a file.
type jsonFileError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Report implements the Reporter interface for JSON output.
func (r *JSONReporter) Report(w io.Writer, result *Result) error {
	output := r.buildOutput(result)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// buildOutput constructs the JSON output structure from the result.
func (r *JSONReporter) buildOutput(result *Result) jsonOutput {
	// Group findings by file
	fileMap := make(map[string][]Finding)
	for _, finding := range result.Findings {
		fileMap[finding.FilePath] = append(fileMap[finding.FilePath], finding)
	}

	// Build file list
	var files []jsonFile
	for path, findings := range fileMap {
		jf := jsonFile{
			Path:     path,
			Findings: make([]jsonFinding, 0, len(findings)),
		}
		for _, f := range findings {
			jf.Findings = append(jf.Findings, jsonFinding{
				Rule:      f.Rule,
				Category:  f.Category,
				Severity:  severityToString(f.Severity),
				Message:   f.Message,
				Line:      f.Line,
				Column:    f.Column,
				EndLine:   f.EndLine,
				EndColumn: f.EndColumn,
			})
		}
		files = append(files, jf)
	}

	// Build summary
	summary := jsonSummary{
		TotalFiles:    result.Files,
		TotalFindings: len(result.Findings),
		Errors:        0,
		Warnings:      0,
		Infos:         0,
		Hints:         0,
		BySeverity:    make(map[string]int),
		ByRule:        make(map[string]int),
		ByCategory:    make(map[string]int),
	}

	// Count by severity
	for _, finding := range result.Findings {
		sevStr := severityToString(finding.Severity)
		summary.BySeverity[sevStr]++

		switch finding.Severity {
		case SeverityError:
			summary.Errors++
		case SeverityWarning:
			summary.Warnings++
		case SeverityInfo:
			summary.Infos++
		case SeverityHint:
			summary.Hints++
		}

		// Count by rule and category
		if finding.Rule != "" {
			summary.ByRule[finding.Rule]++
		}
		if finding.Category != "" {
			summary.ByCategory[finding.Category]++
		}
	}

	// Add file errors
	if len(result.Errors) > 0 {
		summary.FileErrors = make([]jsonFileError, len(result.Errors))
		for i, err := range result.Errors {
			summary.FileErrors[i] = jsonFileError{
				Path:    err.Path,
				Message: err.Err.Error(),
			}
		}
	}

	return jsonOutput{
		Files:   files,
		Summary: summary,
	}
}

// severityToString converts a Severity to its string representation.
func severityToString(s Severity) string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}
