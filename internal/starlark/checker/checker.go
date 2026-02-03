// Package checker provides static analysis for Starlark files.
//
// Unlike the linter (which uses buildtools for style checks), the checker
// uses starlark-go's resolver for proper semantic analysis including:
//   - Undefined name detection
//   - Unused binding detection
//   - Scope analysis
//   - (Future) Type checking
package checker

import (
	"fmt"

	"github.com/albertocavalcante/sky/internal/starlark/sortutil"
	"go.starlark.net/resolve"
	"go.starlark.net/syntax"
)

// Diagnostic represents a single issue found during checking.
type Diagnostic struct {
	// Pos is the position of the issue in the source file.
	Pos syntax.Position

	// End is the end position (if known), otherwise equals Pos.
	End syntax.Position

	// Severity indicates the severity of the issue.
	Severity Severity

	// Code is a unique identifier for this diagnostic type.
	Code string

	// Message is a human-readable description of the issue.
	Message string
}

// Severity indicates the severity of a diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Result holds the results of checking one or more files.
type Result struct {
	// Diagnostics contains all issues found.
	Diagnostics []Diagnostic

	// FileCount is the number of files that were checked.
	FileCount int
}

// HasErrors returns true if any diagnostic is an error.
func (r *Result) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-level diagnostics.
func (r *Result) ErrorCount() int {
	count := 0
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level diagnostics.
func (r *Result) WarningCount() int {
	count := 0
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// Options configures the checker behavior.
type Options struct {
	// Predeclared is a set of names that are predeclared in the module.
	// These are typically builtins provided by the host application.
	Predeclared map[string]bool

	// Universal is a set of universal names available everywhere.
	// These are typically core Starlark builtins like 'len', 'str', etc.
	Universal map[string]bool

	// ReportUnused enables reporting of unused bindings.
	ReportUnused bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Predeclared:  make(map[string]bool),
		Universal:    defaultUniversal(),
		ReportUnused: true,
	}
}

// defaultUniversal returns the set of Starlark universal builtins.
func defaultUniversal() map[string]bool {
	// Core Starlark builtins (from the spec)
	return map[string]bool{
		// Types
		"None":  true,
		"True":  true,
		"False": true,

		// Functions
		"abs":       true,
		"all":       true,
		"any":       true,
		"bool":      true,
		"bytes":     true,
		"dict":      true,
		"dir":       true,
		"enumerate": true,
		"fail":      true,
		"float":     true,
		"getattr":   true,
		"hasattr":   true,
		"hash":      true,
		"int":       true,
		"len":       true,
		"list":      true,
		"max":       true,
		"min":       true,
		"print":     true,
		"range":     true,
		"repr":      true,
		"reversed":  true,
		"sorted":    true,
		"str":       true,
		"tuple":     true,
		"type":      true,
		"zip":       true,
	}
}

// Checker performs static analysis on Starlark files.
type Checker struct {
	opts Options
}

// New creates a new Checker with the given options.
func New(opts Options) *Checker {
	return &Checker{opts: opts}
}

// CheckFile checks a single file and returns diagnostics.
func (c *Checker) CheckFile(filename string, src []byte) ([]Diagnostic, error) {
	// Parse the file
	f, err := syntax.Parse(filename, src, syntax.RetainComments)
	if err != nil {
		// Parse errors are reported as diagnostics
		if serr, ok := err.(syntax.Error); ok {
			return []Diagnostic{{
				Pos:      serr.Pos,
				Severity: SeverityError,
				Code:     "parse-error",
				Message:  serr.Msg,
			}}, nil
		}
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}

	return c.checkParsedFile(f)
}

// checkParsedFile analyzes a parsed file.
func (c *Checker) checkParsedFile(f *syntax.File) ([]Diagnostic, error) {
	var diagnostics []Diagnostic

	// Run name resolution
	isPredeclared := func(name string) bool { return c.opts.Predeclared[name] }
	isUniversal := func(name string) bool { return c.opts.Universal[name] }

	if err := resolve.File(f, isPredeclared, isUniversal); err != nil {
		// Resolution errors indicate undefined names
		if errList, ok := err.(resolve.ErrorList); ok {
			for _, e := range errList {
				diagnostics = append(diagnostics, Diagnostic{
					Pos:      e.Pos,
					Severity: SeverityError,
					Code:     "undefined",
					Message:  e.Msg,
				})
			}
		} else {
			return nil, fmt.Errorf("resolving: %w", err)
		}
	}

	// Check for unused bindings
	if c.opts.ReportUnused {
		unused := c.findUnusedBindings(f)
		diagnostics = append(diagnostics, unused...)
	}

	// Sort diagnostics by position
	sortutil.ByLineColumn(diagnostics,
		func(d Diagnostic) int { return int(d.Pos.Line) },
		func(d Diagnostic) int { return int(d.Pos.Col) },
	)

	return diagnostics, nil
}

// findUnusedBindings walks the AST to find unused variable bindings.
func (c *Checker) findUnusedBindings(f *syntax.File) []Diagnostic {
	var diagnostics []Diagnostic

	// Track all bindings and their usage
	bindings := make(map[*syntax.Ident]bool) // ident -> used
	uses := make(map[string][]*syntax.Ident) // name -> all uses

	// First pass: collect all binding sites and uses
	syntax.Walk(f, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Ident:
			if n.Binding == nil {
				return true
			}
			b, ok := n.Binding.(*resolve.Binding)
			if !ok {
				return true
			}
			// Only track local variables for unused detection
			// Global/module-level bindings may be exported, so don't warn
			switch b.Scope {
			case resolve.Local, resolve.Cell:
				if b.First == n {
					// This is a binding site
					bindings[n] = false
				} else {
					// This is a use
					uses[n.Name] = append(uses[n.Name], n)
				}
			case resolve.Global:
				// Track uses but don't warn on unused globals
				if b.First != n {
					uses[n.Name] = append(uses[n.Name], n)
				}
			}
		}
		return true
	})

	// Mark bindings that are used
	for _, useList := range uses {
		for _, use := range useList {
			if use.Binding == nil {
				continue
			}
			b, ok := use.Binding.(*resolve.Binding)
			if !ok || b.First == nil {
				continue
			}
			bindings[b.First] = true
		}
	}

	// Report unused bindings
	for ident, used := range bindings {
		if !used && !isUnderscore(ident.Name) {
			diagnostics = append(diagnostics, Diagnostic{
				Pos:      ident.NamePos,
				Severity: SeverityWarning,
				Code:     "unused",
				Message:  fmt.Sprintf("local variable %q is assigned but never used", ident.Name),
			})
		}
	}

	return diagnostics
}

// isUnderscore returns true if the name is "_" or starts with "_" (convention for unused).
func isUnderscore(name string) bool {
	return name == "_" || (len(name) > 1 && name[0] == '_')
}
