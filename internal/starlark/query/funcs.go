package query

import (
	"fmt"
	"regexp"
)

// evalFiles evaluates files(pattern) - returns files matching pattern.
func (e *Engine) evalFiles(args []Expr) (*Result, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("files() requires exactly 1 argument, got %d", len(args))
	}

	// Get the pattern from the argument
	pattern, err := e.getPattern(args[0])
	if err != nil {
		return nil, fmt.Errorf("files(): %w", err)
	}

	files := e.index.MatchFiles(pattern)
	items := make([]Item, len(files))
	for i, f := range files {
		items[i] = Item{
			Type: "file",
			Name: f.Path,
			File: f.Path,
			Line: 1,
			Data: f,
		}
	}
	return &Result{Items: items}, nil
}

// evalDefs evaluates defs(expr) - returns function definitions in files.
func (e *Engine) evalDefs(args []Expr) (*Result, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("defs() requires exactly 1 argument, got %d", len(args))
	}

	files, err := e.getFilesFromExpr(args[0])
	if err != nil {
		return nil, fmt.Errorf("defs(): %w", err)
	}

	var items []Item
	for _, f := range files {
		for _, def := range f.Defs {
			items = append(items, Item{
				Type: "def",
				Name: def.Name,
				File: def.File,
				Line: def.Line,
				Data: def,
			})
		}
	}
	return &Result{Items: items}, nil
}

// evalLoads evaluates loads(expr) - returns load statements in files.
func (e *Engine) evalLoads(args []Expr) (*Result, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("loads() requires exactly 1 argument, got %d", len(args))
	}

	files, err := e.getFilesFromExpr(args[0])
	if err != nil {
		return nil, fmt.Errorf("loads(): %w", err)
	}

	var items []Item
	for _, f := range files {
		for _, load := range f.Loads {
			items = append(items, Item{
				Type: "load",
				Name: load.Module,
				File: load.File,
				Line: load.Line,
				Data: load,
			})
		}
	}
	return &Result{Items: items}, nil
}

// evalCalls evaluates calls(fn, expr) - returns calls to function fn in files.
// If fn is "*", returns all calls.
func (e *Engine) evalCalls(args []Expr) (*Result, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("calls() requires exactly 2 arguments, got %d", len(args))
	}

	// Get function name pattern from first argument
	fnPattern, err := e.getFunctionPattern(args[0])
	if err != nil {
		return nil, fmt.Errorf("calls(): %w", err)
	}

	files, err := e.getFilesFromExpr(args[1])
	if err != nil {
		return nil, fmt.Errorf("calls(): %w", err)
	}

	var items []Item
	for _, f := range files {
		for _, call := range f.Calls {
			if fnPattern == "*" || call.Function == fnPattern {
				items = append(items, Item{
					Type: "call",
					Name: call.Function,
					File: call.File,
					Line: call.Line,
					Data: call,
				})
			}
		}
	}
	return &Result{Items: items}, nil
}

// evalAssigns evaluates assigns(expr) - returns top-level assignments in files.
func (e *Engine) evalAssigns(args []Expr) (*Result, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("assigns() requires exactly 1 argument, got %d", len(args))
	}

	files, err := e.getFilesFromExpr(args[0])
	if err != nil {
		return nil, fmt.Errorf("assigns(): %w", err)
	}

	var items []Item
	for _, f := range files {
		for _, assign := range f.Assigns {
			items = append(items, Item{
				Type: "assign",
				Name: assign.Name,
				File: assign.File,
				Line: assign.Line,
				Data: assign,
			})
		}
	}
	return &Result{Items: items}, nil
}

// evalFilter evaluates filter(pattern, expr) - filters results by regex pattern on name.
func (e *Engine) evalFilter(args []Expr) (*Result, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("filter() requires exactly 2 arguments, got %d", len(args))
	}

	// Get pattern from first argument (must be a string)
	patternStr, ok := args[0].(*StringExpr)
	if !ok {
		return nil, fmt.Errorf("filter() first argument must be a string pattern")
	}

	re, err := regexp.Compile(patternStr.Value)
	if err != nil {
		return nil, fmt.Errorf("filter(): invalid regex pattern: %w", err)
	}

	// Evaluate the second argument
	result, err := e.Eval(args[1])
	if err != nil {
		return nil, fmt.Errorf("filter(): %w", err)
	}

	// Filter items by name
	var items []Item
	for _, item := range result.Items {
		if re.MatchString(item.Name) {
			items = append(items, item)
		}
	}
	return &Result{Items: items}, nil
}

// getPattern extracts a pattern string from an expression.
func (e *Engine) getPattern(expr Expr) (string, error) {
	switch ex := expr.(type) {
	case *LiteralExpr:
		return ex.Pattern, nil
	case *StringExpr:
		return ex.Value, nil
	default:
		return "", fmt.Errorf("expected pattern or string, got %T", expr)
	}
}

// getFunctionPattern extracts a function name pattern from an expression.
func (e *Engine) getFunctionPattern(expr Expr) (string, error) {
	switch ex := expr.(type) {
	case *LiteralExpr:
		return ex.Pattern, nil
	case *StringExpr:
		return ex.Value, nil
	default:
		return "", fmt.Errorf("expected function name or pattern, got %T", expr)
	}
}
