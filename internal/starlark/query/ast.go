// Package query provides a query language for Starlark source files.
// It enables searching and introspecting Starlark code structures such as
// function definitions, load statements, function calls, and assignments.
package query

import "fmt"

// Expr is a query expression.
// All query expressions implement this interface.
type Expr interface {
	expr()
	String() string
}

// LiteralExpr is a file pattern like "//..." or "//pkg:file.bzl".
type LiteralExpr struct {
	Pattern string
}

func (*LiteralExpr) expr() {}

// String returns the string representation of the literal expression.
func (e *LiteralExpr) String() string {
	return e.Pattern
}

// CallExpr is a function call like defs(//...) or filter("pat", expr).
type CallExpr struct {
	Func string
	Args []Expr
}

func (*CallExpr) expr() {}

// String returns the string representation of the call expression.
func (e *CallExpr) String() string {
	s := e.Func + "("
	for i, arg := range e.Args {
		if i > 0 {
			s += ", "
		}
		s += arg.String()
	}
	s += ")"
	return s
}

// StringExpr is a string literal like "pattern".
type StringExpr struct {
	Value string
}

func (*StringExpr) expr() {}

// String returns the string representation of the string expression.
func (e *StringExpr) String() string {
	return fmt.Sprintf("%q", e.Value)
}

// BinaryExpr is a set operation like a + b, a - b, a ^ b.
type BinaryExpr struct {
	Op    string // "+", "-", "^"
	Left  Expr
	Right Expr
}

func (*BinaryExpr) expr() {}

// String returns the string representation of the binary expression.
func (e *BinaryExpr) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Op, e.Right.String())
}
