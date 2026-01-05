// Package validator provides interfaces for semantic validation of Starlark files.
package validator

import (
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Severity represents the severity of a diagnostic message.
type Severity int

const (
	// SeverityError indicates a blocking issue that prevents further processing.
	SeverityError Severity = iota
	// SeverityWarning indicates a non-blocking issue that should be addressed.
	SeverityWarning
	// SeverityInfo indicates informational messages.
	SeverityInfo
	// SeverityHint indicates suggestions for improvement.
	SeverityHint
)

// String returns the string representation of the Severity.
func (s Severity) String() string {
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

// Diagnostic represents a validation finding.
type Diagnostic struct {
	// Severity indicates how serious this issue is.
	Severity Severity `json:"severity"`

	// Message is a human-readable description of the issue.
	Message string `json:"message"`

	// File is the path to the file containing the issue.
	File string `json:"file"`

	// Line is the 1-based line number where the issue starts.
	Line int `json:"line"`

	// Column is the 1-based column number where the issue starts.
	Column int `json:"column"`

	// EndLine is the 1-based line number where the issue ends (optional).
	EndLine int `json:"end_line,omitempty"`

	// EndColumn is the 1-based column number where the issue ends (optional).
	EndColumn int `json:"end_column,omitempty"`

	// Code is a machine-readable code for this diagnostic (e.g., "E001", "W042").
	Code string `json:"code"`

	// Source identifies the validator that produced this diagnostic.
	Source string `json:"source"`
}

// IsError returns true if this diagnostic is an error.
func (d Diagnostic) IsError() bool {
	return d.Severity == SeverityError
}

// Context provides context for semantic validation.
type Context struct {
	// Dialect is the name of the dialect being validated.
	Dialect string

	// FileKind is the kind of file being validated.
	FileKind filekind.Kind

	// FilePath is the path to the file being validated.
	FilePath string

	// FileContent is the content of the file (if available).
	FileContent []byte

	// TODO: Add AST, resolved symbols, type info, etc.
}

// Validator performs semantic analysis on Starlark files.
type Validator interface {
	// Name returns the unique identifier for this validator.
	Name() string

	// Validate performs validation and returns diagnostics.
	// Returns an error only if validation itself fails (not for validation issues).
	Validate(ctx Context) ([]Diagnostic, error)

	// SupportedKinds returns the file kinds this validator applies to.
	// An empty slice means all kinds are supported.
	SupportedKinds() []filekind.Kind
}

// ValidatorFunc is a function type that implements Validator.
type ValidatorFunc struct {
	NameVal    string
	Kinds      []filekind.Kind
	ValidateFn func(ctx Context) ([]Diagnostic, error)
}

// Name implements the Validator interface.
func (v ValidatorFunc) Name() string {
	return v.NameVal
}

// Validate implements the Validator interface.
func (v ValidatorFunc) Validate(ctx Context) ([]Diagnostic, error) {
	if v.ValidateFn != nil {
		return v.ValidateFn(ctx)
	}
	return nil, nil
}

// SupportedKinds implements the Validator interface.
func (v ValidatorFunc) SupportedKinds() []filekind.Kind {
	return v.Kinds
}

// Runner runs multiple validators and collects diagnostics.
type Runner struct {
	validators []Validator
}

// NewRunner creates a new validator runner.
func NewRunner(validators ...Validator) *Runner {
	return &Runner{validators: validators}
}

// Run runs all applicable validators and returns all diagnostics.
func (r *Runner) Run(ctx Context) ([]Diagnostic, error) {
	var all []Diagnostic
	for _, v := range r.validators {
		kinds := v.SupportedKinds()
		if len(kinds) > 0 && !containsKind(kinds, ctx.FileKind) {
			continue
		}
		diags, err := v.Validate(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, diags...)
	}
	return all, nil
}

func containsKind(kinds []filekind.Kind, kind filekind.Kind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}
