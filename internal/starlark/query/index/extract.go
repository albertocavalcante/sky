package index

import (
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// ExtractFile extracts index data from a parsed build.File.
func ExtractFile(f *build.File, path string, kind filekind.Kind) *File {
	return &File{
		Path:    path,
		Kind:    kind,
		Defs:    extractDefs(f, path),
		Loads:   extractLoads(f, path),
		Calls:   extractCalls(f, path),
		Assigns: extractAssigns(f, path),
	}
}

// extractDefs extracts function definitions from a file.
func extractDefs(f *build.File, path string) []Def {
	var defs []Def

	for _, stmt := range f.Stmt {
		defStmt, ok := stmt.(*build.DefStmt)
		if !ok {
			continue
		}

		params := extractParams(defStmt.Params)
		docstring := extractDocstring(defStmt.Body)

		start, _ := defStmt.Span()
		defs = append(defs, Def{
			Name:      defStmt.Name,
			File:      path,
			Line:      start.Line,
			Params:    params,
			Docstring: docstring,
		})
	}

	return defs
}

// extractParams extracts parameter names from a function's parameter list.
func extractParams(params []build.Expr) []string {
	var names []string

	for _, param := range params {
		switch p := param.(type) {
		case *build.Ident:
			names = append(names, p.Name)
		case *build.AssignExpr:
			// Default value parameter: name = default
			if ident, ok := p.LHS.(*build.Ident); ok {
				names = append(names, ident.Name)
			}
		case *build.UnaryExpr:
			// *args or **kwargs
			if ident, ok := p.X.(*build.Ident); ok {
				if p.Op == "*" {
					names = append(names, "*"+ident.Name)
				} else if p.Op == "**" {
					names = append(names, "**"+ident.Name)
				}
			}
		}
	}

	return names
}

// extractDocstring extracts the docstring from a function body.
// Returns empty string if no docstring is present.
func extractDocstring(body []build.Expr) string {
	if len(body) == 0 {
		return ""
	}

	// Check if the first statement is a string literal (docstring)
	firstStmt := body[0]
	strLit, ok := firstStmt.(*build.StringExpr)
	if !ok {
		return ""
	}

	return strLit.Value
}

// extractLoads extracts load statements from a file.
func extractLoads(f *build.File, path string) []Load {
	var loads []Load

	for _, stmt := range f.Stmt {
		loadStmt, ok := stmt.(*build.LoadStmt)
		if !ok {
			continue
		}

		symbols := make(map[string]string)
		for i, from := range loadStmt.From {
			to := loadStmt.To[i]
			// from.Name is the exported name, to.Name is the local alias
			symbols[to.Name] = from.Name
		}

		start, _ := loadStmt.Span()
		loads = append(loads, Load{
			Module:  loadStmt.Module.Value,
			Symbols: symbols,
			File:    path,
			Line:    start.Line,
		})
	}

	return loads
}

// extractCalls extracts top-level function calls from a file.
func extractCalls(f *build.File, path string) []Call {
	var calls []Call

	for _, stmt := range f.Stmt {
		callExpr, ok := stmt.(*build.CallExpr)
		if !ok {
			continue
		}

		funcName := extractFunctionName(callExpr.X)
		if funcName == "" {
			continue
		}

		args := extractArgs(callExpr.List)

		start, _ := callExpr.Span()
		calls = append(calls, Call{
			Function: funcName,
			Args:     args,
			File:     path,
			Line:     start.Line,
		})
	}

	return calls
}

// extractFunctionName extracts the function name from a call expression.
func extractFunctionName(expr build.Expr) string {
	switch e := expr.(type) {
	case *build.Ident:
		return e.Name
	case *build.DotExpr:
		// Handle method calls like native.cc_library
		baseName := extractFunctionName(e.X)
		if baseName != "" {
			return baseName + "." + e.Name
		}
		return e.Name
	}
	return ""
}

// extractArgs extracts arguments from a call expression.
func extractArgs(list []build.Expr) []Arg {
	var args []Arg

	for _, expr := range list {
		switch e := expr.(type) {
		case *build.AssignExpr:
			// Keyword argument: name = value
			if ident, ok := e.LHS.(*build.Ident); ok {
				args = append(args, Arg{
					Name:  ident.Name,
					Value: exprToString(e.RHS),
				})
			}
		default:
			// Positional argument
			args = append(args, Arg{
				Name:  "",
				Value: exprToString(expr),
			})
		}
	}

	return args
}

// exprToString converts an expression to its string representation.
func exprToString(expr build.Expr) string {
	switch e := expr.(type) {
	case *build.StringExpr:
		return e.Value
	case *build.Ident:
		return e.Name
	case *build.LiteralExpr:
		return e.Token
	case *build.ListExpr:
		var items []string
		for _, item := range e.List {
			items = append(items, exprToString(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case *build.DictExpr:
		var items []string
		for _, kv := range e.List {
			items = append(items, exprToString(kv.Key)+": "+exprToString(kv.Value))
		}
		return "{" + strings.Join(items, ", ") + "}"
	case *build.CallExpr:
		funcName := extractFunctionName(e.X)
		return funcName + "(...)"
	case *build.BinaryExpr:
		return exprToString(e.X) + " " + e.Op + " " + exprToString(e.Y)
	case *build.UnaryExpr:
		return e.Op + exprToString(e.X)
	case *build.DotExpr:
		return exprToString(e.X) + "." + e.Name
	case *build.IndexExpr:
		return exprToString(e.X) + "[" + exprToString(e.Y) + "]"
	case *build.SliceExpr:
		return exprToString(e.X) + "[...]"
	case *build.TupleExpr:
		var items []string
		for _, item := range e.List {
			items = append(items, exprToString(item))
		}
		return "(" + strings.Join(items, ", ") + ")"
	case *build.Comprehension:
		return "[...]"
	case *build.ConditionalExpr:
		return exprToString(e.Then) + " if ... else " + exprToString(e.Else)
	case *build.LambdaExpr:
		return "lambda(...)"
	}
	return "<expr>"
}

// extractAssigns extracts top-level assignments from a file.
func extractAssigns(f *build.File, path string) []Assign {
	var assigns []Assign

	for _, stmt := range f.Stmt {
		assignExpr, ok := stmt.(*build.AssignExpr)
		if !ok {
			continue
		}

		// Extract the assigned name(s)
		start, _ := assignExpr.Span()
		names := extractAssignNames(assignExpr.LHS)
		for _, name := range names {
			assigns = append(assigns, Assign{
				Name: name,
				File: path,
				Line: start.Line,
			})
		}
	}

	return assigns
}

// extractAssignNames extracts variable names from the left-hand side of an assignment.
func extractAssignNames(expr build.Expr) []string {
	switch e := expr.(type) {
	case *build.Ident:
		return []string{e.Name}
	case *build.TupleExpr:
		// Tuple unpacking: a, b = ...
		var names []string
		for _, item := range e.List {
			names = append(names, extractAssignNames(item)...)
		}
		return names
	case *build.ListExpr:
		// List unpacking: [a, b] = ...
		var names []string
		for _, item := range e.List {
			names = append(names, extractAssignNames(item)...)
		}
		return names
	}
	return nil
}
