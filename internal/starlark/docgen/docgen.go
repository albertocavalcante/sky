// Package docgen extracts documentation from Starlark files.
//
// It parses Starlark source files and extracts docstrings from
// functions, following Python-style conventions (first string
// literal in function body).
package docgen

import (
	"sort"
	"strings"

	"go.starlark.net/syntax"
)

// ModuleDoc represents documentation for a Starlark module (file).
type ModuleDoc struct {
	// File is the source file path.
	File string

	// Docstring is the module-level docstring (if any).
	Docstring string

	// Functions contains documentation for all functions.
	Functions []FunctionDoc

	// Globals contains documentation for global variables.
	Globals []GlobalDoc
}

// FunctionDoc represents documentation for a single function.
type FunctionDoc struct {
	// Name is the function name.
	Name string

	// Docstring is the raw docstring.
	Docstring string

	// Parsed contains the parsed docstring sections.
	Parsed *ParsedDocstring

	// Params contains the function parameters.
	Params []ParamDoc

	// Line is the line number where the function is defined.
	Line int

	// IsPrivate indicates if the function name starts with _.
	IsPrivate bool
}

// ParamDoc represents a function parameter.
type ParamDoc struct {
	// Name is the parameter name.
	Name string

	// Default is the default value (if any), as source text.
	Default string

	// HasDefault indicates if the parameter has a default value.
	HasDefault bool
}

// GlobalDoc represents a global variable assignment.
type GlobalDoc struct {
	// Name is the variable name.
	Name string

	// Value is the assigned value as source text (truncated if long).
	Value string

	// Line is the line number.
	Line int

	// IsPrivate indicates if the name starts with _.
	IsPrivate bool
}

// Options configures the documentation extraction.
type Options struct {
	// IncludePrivate includes private symbols (starting with _).
	IncludePrivate bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		IncludePrivate: false,
	}
}

// ExtractFile extracts documentation from a Starlark file.
func ExtractFile(filename string, src []byte, opts Options) (*ModuleDoc, error) {
	// Parse the file
	f, err := syntax.Parse(filename, src, syntax.RetainComments)
	if err != nil {
		return nil, err
	}

	doc := &ModuleDoc{
		File: filename,
	}

	// Extract module docstring (first statement if it's a string)
	if len(f.Stmts) > 0 {
		if docstring := extractExprDocstring(f.Stmts[0]); docstring != "" {
			doc.Docstring = docstring
		}
	}

	// Extract functions and globals
	for _, stmt := range f.Stmts {
		switch s := stmt.(type) {
		case *syntax.DefStmt:
			funcDoc := extractFunctionDoc(s)
			if opts.IncludePrivate || !funcDoc.IsPrivate {
				doc.Functions = append(doc.Functions, funcDoc)
			}

		case *syntax.AssignStmt:
			// Only simple assignments (x = value)
			if ident, ok := s.LHS.(*syntax.Ident); ok {
				globalDoc := GlobalDoc{
					Name:      ident.Name,
					Value:     truncateValue(s.RHS),
					Line:      int(s.OpPos.Line),
					IsPrivate: strings.HasPrefix(ident.Name, "_"),
				}
				if opts.IncludePrivate || !globalDoc.IsPrivate {
					doc.Globals = append(doc.Globals, globalDoc)
				}
			}
		}
	}

	// Sort functions by name
	sort.Slice(doc.Functions, func(i, j int) bool {
		return doc.Functions[i].Name < doc.Functions[j].Name
	})

	return doc, nil
}

// extractFunctionDoc extracts documentation from a function definition.
func extractFunctionDoc(def *syntax.DefStmt) FunctionDoc {
	doc := FunctionDoc{
		Name:      def.Name.Name,
		Line:      int(def.Def.Line),
		IsPrivate: strings.HasPrefix(def.Name.Name, "_"),
	}

	// Extract parameters
	for _, param := range def.Params {
		paramDoc := extractParamDoc(param)
		doc.Params = append(doc.Params, paramDoc)
	}

	// Extract docstring (first statement in body if it's a string)
	if len(def.Body) > 0 {
		doc.Docstring = extractExprDocstring(def.Body[0])
	}

	// Parse the docstring
	if doc.Docstring != "" {
		doc.Parsed = ParseDocstring(doc.Docstring)
	}

	return doc
}

// extractParamDoc extracts parameter information.
func extractParamDoc(expr syntax.Expr) ParamDoc {
	switch p := expr.(type) {
	case *syntax.Ident:
		return ParamDoc{Name: p.Name}

	case *syntax.BinaryExpr:
		// name = default
		if p.Op == syntax.EQ {
			if ident, ok := p.X.(*syntax.Ident); ok {
				return ParamDoc{
					Name:       ident.Name,
					Default:    exprToString(p.Y),
					HasDefault: true,
				}
			}
		}

	case *syntax.UnaryExpr:
		// *args or **kwargs
		if ident, ok := p.X.(*syntax.Ident); ok {
			prefix := ""
			if p.Op == syntax.STAR {
				prefix = "*"
			} else if p.Op == syntax.STARSTAR {
				prefix = "**"
			}
			return ParamDoc{Name: prefix + ident.Name}
		}
	}

	return ParamDoc{Name: "?"}
}

// extractExprDocstring extracts a docstring from an expression statement.
func extractExprDocstring(stmt syntax.Stmt) string {
	exprStmt, ok := stmt.(*syntax.ExprStmt)
	if !ok {
		return ""
	}

	lit, ok := exprStmt.X.(*syntax.Literal)
	if !ok || lit.Token != syntax.STRING {
		return ""
	}

	// The value is already unquoted by the parser
	if s, ok := lit.Value.(string); ok {
		return strings.TrimSpace(s)
	}

	return ""
}

// exprToString converts an expression to a string representation.
func exprToString(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.Ident:
		return e.Name
	case *syntax.Literal:
		switch e.Token {
		case syntax.STRING:
			if s, ok := e.Value.(string); ok {
				return `"` + s + `"`
			}
		case syntax.INT:
			return e.Raw
		case syntax.FLOAT:
			return e.Raw
		}
	case *syntax.ListExpr:
		return "[...]"
	case *syntax.DictExpr:
		return "{...}"
	case *syntax.CallExpr:
		if fn, ok := e.Fn.(*syntax.Ident); ok {
			return fn.Name + "(...)"
		}
		return "(...)"
	}
	return "..."
}

// truncateValue returns a string representation of a value, truncated if too long.
func truncateValue(expr syntax.Expr) string {
	s := exprToString(expr)
	if len(s) > 50 {
		return s[:47] + "..."
	}
	return s
}
