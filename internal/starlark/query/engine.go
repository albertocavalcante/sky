package query

import (
	"fmt"

	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

// Result is the result of a query - a set of items.
type Result struct {
	Items []Item
}

// Item is a query result item (file, def, load, call, assign, etc.).
type Item struct {
	// Type is the kind of item: "file", "def", "load", "call", "assign".
	Type string

	// Name is the primary identifier (function name, variable name, file path, etc.).
	Name string

	// File is the file path where this item is located.
	File string

	// Line is the 1-based line number where this item starts.
	Line int

	// Data is the original data (index.Def, index.Load, etc.).
	Data any
}

// key returns a unique key for this item for deduplication.
func (i Item) key() string {
	return fmt.Sprintf("%s:%s:%s:%d", i.Type, i.File, i.Name, i.Line)
}

// Engine evaluates queries against an index.
type Engine struct {
	index *index.Index
}

// NewEngine creates a query engine with the given index.
func NewEngine(idx *index.Index) *Engine {
	return &Engine{index: idx}
}

// Eval evaluates a query expression and returns results.
func (e *Engine) Eval(expr Expr) (*Result, error) {
	switch ex := expr.(type) {
	case *LiteralExpr:
		return e.evalLiteral(ex)
	case *CallExpr:
		return e.evalCall(ex)
	case *StringExpr:
		// A bare string expression evaluates to an empty result
		return &Result{}, nil
	case *BinaryExpr:
		return e.evalBinary(ex)
	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

// EvalString parses and evaluates a query string.
func (e *Engine) EvalString(query string) (*Result, error) {
	expr, err := Parse(query)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return e.Eval(expr)
}

// evalLiteral evaluates a literal pattern expression.
// It returns files matching the pattern.
func (e *Engine) evalLiteral(expr *LiteralExpr) (*Result, error) {
	files := e.index.MatchFiles(expr.Pattern)
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

// evalCall evaluates a function call expression.
func (e *Engine) evalCall(expr *CallExpr) (*Result, error) {
	switch expr.Func {
	case "files":
		return e.evalFiles(expr.Args)
	case "defs":
		return e.evalDefs(expr.Args)
	case "loads":
		return e.evalLoads(expr.Args)
	case "calls":
		return e.evalCalls(expr.Args)
	case "assigns":
		return e.evalAssigns(expr.Args)
	case "filter":
		return e.evalFilter(expr.Args)
	default:
		return nil, fmt.Errorf("unknown function: %s", expr.Func)
	}
}

// evalBinary evaluates a binary expression (set operation).
func (e *Engine) evalBinary(expr *BinaryExpr) (*Result, error) {
	left, err := e.Eval(expr.Left)
	if err != nil {
		return nil, err
	}
	right, err := e.Eval(expr.Right)
	if err != nil {
		return nil, err
	}

	switch expr.Op {
	case "+":
		return Union(left, right), nil
	case "-":
		return Difference(left, right), nil
	case "^":
		return Intersection(left, right), nil
	default:
		return nil, fmt.Errorf("unknown operator: %s", expr.Op)
	}
}

// getFilesFromExpr evaluates an expression and returns the file items.
// If the expression result contains files, those are returned.
// If the expression result contains other items, their files are extracted.
func (e *Engine) getFilesFromExpr(expr Expr) ([]*index.File, error) {
	result, err := e.Eval(expr)
	if err != nil {
		return nil, err
	}

	// Collect unique files
	seen := make(map[string]bool)
	var files []*index.File

	for _, item := range result.Items {
		if item.Type == "file" {
			if f, ok := item.Data.(*index.File); ok && !seen[f.Path] {
				seen[f.Path] = true
				files = append(files, f)
			}
		} else if item.File != "" && !seen[item.File] {
			// Get file from index by path
			if f := e.index.Get(item.File); f != nil {
				seen[item.File] = true
				files = append(files, f)
			}
		}
	}

	return files, nil
}
