package linter

import (
	"fmt"
	"io"
	"sort"
)

// GitHubReporter outputs findings in GitHub Actions annotation format.
// Format: ::warning file={file},line={line},col={col}::{message}
// See: https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions
type GitHubReporter struct{}

// NewGitHubReporter creates a new GitHub Actions reporter.
func NewGitHubReporter() *GitHubReporter {
	return &GitHubReporter{}
}

// Report implements the Reporter interface for GitHub Actions output.
func (r *GitHubReporter) Report(w io.Writer, result *Result) error {
	// Sort findings by file, then line, then column
	sortedFindings := make([]Finding, len(result.Findings))
	copy(sortedFindings, result.Findings)
	sort.Slice(sortedFindings, func(i, j int) bool {
		fi, fj := sortedFindings[i], sortedFindings[j]
		if fi.FilePath != fj.FilePath {
			return fi.FilePath < fj.FilePath
		}
		if fi.Line != fj.Line {
			return fi.Line < fj.Line
		}
		return fi.Column < fj.Column
	})

	// Output each finding as a GitHub Actions annotation
	for _, finding := range sortedFindings {
		if err := r.reportFinding(w, finding); err != nil {
			return err
		}
	}

	// Report file errors as errors
	for _, fileErr := range result.Errors {
		if _, err := fmt.Fprintf(w, "::error file=%s::Failed to process file: %v\n",
			fileErr.Path, fileErr.Err); err != nil {
			return err
		}
	}

	return nil
}

// reportFinding outputs a single finding in GitHub Actions annotation format.
func (r *GitHubReporter) reportFinding(w io.Writer, f Finding) error {
	// Determine the annotation level based on severity
	level := r.severityToLevel(f.Severity)

	// Build the annotation
	// Format: ::{level} file={file},line={line},col={col},title={title}::{message}
	title := fmt.Sprintf("%s (%s)", f.Rule, f.Category)
	if f.Rule == "" {
		title = f.Category
	}
	if title == "" {
		title = "lint"
	}

	// Build location parameters
	location := fmt.Sprintf("file=%s,line=%d", f.FilePath, f.Line)
	if f.Column > 0 {
		location += fmt.Sprintf(",col=%d", f.Column)
	}
	if f.EndLine > 0 && f.EndLine != f.Line {
		location += fmt.Sprintf(",endLine=%d", f.EndLine)
	}
	if f.EndColumn > 0 && f.EndColumn != f.Column {
		location += fmt.Sprintf(",endColumn=%d", f.EndColumn)
	}

	// Output the annotation
	_, err := fmt.Fprintf(w, "::%s %s,title=%s::%s\n",
		level, location, title, f.Message)
	return err
}

// severityToLevel converts a Severity to a GitHub Actions annotation level.
func (r *GitHubReporter) severityToLevel(s Severity) string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo, SeverityHint:
		// GitHub Actions only supports error, warning, and notice
		return "notice"
	default:
		return "notice"
	}
}
