// Package linter provides a configurable Starlark linter with extensible rules.
package linter

import (
	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/validator"
)

// Severity represents the severity level of a lint finding.
// Re-export from validator for convenience.
type Severity = validator.Severity

// Severity constants re-exported from validator.
const (
	SeverityError   = validator.SeverityError
	SeverityWarning = validator.SeverityWarning
	SeverityInfo    = validator.SeverityInfo
	SeverityHint    = validator.SeverityHint
)

// Rule defines a single lint rule.
// Inspired by Go's analysis.Analyzer interface.
type Rule struct {
	// Name is the unique kebab-case identifier (e.g., "unused-load").
	Name string

	// Doc is a one-line description of what this rule checks.
	Doc string

	// URL is an optional link to detailed documentation.
	URL string

	// Category groups related rules (e.g., "correctness", "style", "performance").
	Category string

	// Severity is the default severity for findings from this rule.
	Severity Severity

	// AutoFix indicates whether this rule can automatically fix issues.
	AutoFix bool

	// FileKinds specifies which file kinds this rule applies to.
	// An empty slice means the rule applies to all file kinds.
	FileKinds []filekind.Kind

	// Requires lists rules that must run before this rule.
	// Used for horizontal dependencies (same file, different rules).
	Requires []*Rule

	// Run is the function that executes this rule.
	// It receives a Pass with context and reports findings via Pass.Report.
	// The returned value can be used by dependent rules via Pass.ResultOf.
	Run func(*Pass) (any, error)
}

// Pass provides context to a running rule.
type Pass struct {
	// File is the parsed AST of the file being linted.
	File *build.File

	// FilePath is the path to the file being linted.
	FilePath string

	// FileKind is the detected kind of the file (BUILD, bzl, etc.).
	FileKind filekind.Kind

	// Content is the raw source content of the file.
	Content []byte

	// Config holds per-rule configuration options.
	Config RuleConfig

	// Report is called to report a finding.
	Report func(Finding)

	// ResultOf returns the result of a required rule.
	// Only valid for rules listed in the current rule's Requires.
	ResultOf func(*Rule) any
}

// RuleConfig holds per-rule configuration options.
type RuleConfig struct {
	// Severity overrides the rule's default severity.
	// Zero value means use the rule's default.
	Severity Severity

	// Options holds rule-specific configuration.
	Options map[string]any
}

// Finding represents a lint diagnostic.
type Finding struct {
	// FilePath is the path to the file containing this finding.
	FilePath string

	// Severity is the severity of this finding.
	Severity Severity

	// Message is a human-readable description of the issue.
	Message string

	// Line is the 1-based line number where the issue starts.
	Line int

	// Column is the 1-based column number where the issue starts.
	Column int

	// EndLine is the 1-based line number where the issue ends.
	EndLine int

	// EndColumn is the 1-based column number where the issue ends.
	EndColumn int

	// Rule is the name of the rule that produced this finding.
	Rule string

	// Category is the category of the rule.
	Category string

	// Replacement is an optional suggested fix.
	Replacement *Replacement
}

// Replacement represents a suggested fix for a finding.
type Replacement struct {
	// Content is the replacement text.
	Content string

	// Start is the byte offset where the replacement starts.
	Start int

	// End is the byte offset where the replacement ends.
	End int
}

// ToDiagnostic converts a Finding to a validator.Diagnostic.
func (f Finding) ToDiagnostic(filePath string) validator.Diagnostic {
	return validator.Diagnostic{
		Severity:  f.Severity,
		Message:   f.Message,
		File:      filePath,
		Line:      f.Line,
		Column:    f.Column,
		EndLine:   f.EndLine,
		EndColumn: f.EndColumn,
		Code:      f.Rule,
		Source:    "skylint",
	}
}

// Result represents the outcome of linting one or more files.
type Result struct {
	// Files is the number of files that were linted.
	Files int

	// Findings is the list of all findings.
	Findings []Finding

	// Errors is the list of files that could not be linted.
	Errors []FileError
}

// FileError represents an error that occurred while linting a file.
type FileError struct {
	// Path is the path to the file.
	Path string

	// Err is the error that occurred.
	Err error
}

// HasErrors returns true if any finding has error severity.
func (r *Result) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any finding has warning or error severity.
func (r *Result) HasWarnings() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError || f.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of findings with error severity.
func (r *Result) ErrorCount() int {
	count := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of findings with warning severity.
func (r *Result) WarningCount() int {
	count := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityWarning {
			count++
		}
	}
	return count
}
