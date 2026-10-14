package main

import (
	"go/parser"
	"strings"
	"testing"
)

func TestExprToStringNoUnknown(t *testing.T) {
	for _, src := range []string{
		"-x",
		"f(a, b)",
		"a[1]",
		"func(int) string",
		"(a + b)",
		"interface{ Foo() string }",
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		got := exprToString(e)
		if strings.Contains(got, "unknown") || strings.Contains(got, "unprintable") {
			t.Errorf("exprToString(%q) = %q", src, got)
		}
	}
	e, _ := parser.ParseExpr("interface{ Foo() string }")
	if got := exprToString(e); !strings.Contains(got, "Foo") {
		t.Errorf("interface methods lost: %q", got)
	}
}
