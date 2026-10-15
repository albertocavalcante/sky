package types

import (
	"strings"
	"testing"
	"time"
)

func TestInferExprType_Regressions(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"0xDEADBEEF", "int"},
		{"0x1e", "int"},
		{"0b101", "int"},
		{"1e3", "float"},
		{"1.5", "float"},
		{"-x", "Unknown"},
		{"+\"s\"", "Unknown"},
		{"-1.5", "float"},
		{"abs(-1.5)", "float"},
		{"abs(-1)", "int"},
		{"list([1, 2])", "list[int]"},
		{"set([\"a\"])", "set[str]"},
		{"x.pop()", "Unknown"},
		{"[1, 2].pop()", "int"},
	}
	for _, tt := range tests {
		got := InferExprType(parseExpr(t, tt.code)).String()
		if got != tt.want {
			t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestInferDictKeywordCharacterization pins dict() calls whose result is the
// same with and without the all-keyword-values union (#89). They do NOT prove
// the fix; TestInferDictKeywordUnion does.
func TestInferDictKeywordCharacterization(t *testing.T) {
	tests := []struct{ code, want string }{
		{"dict(a=1)", "dict[str, int]"},
		{"dict(a=1, b=2)", "dict[str, int]"},
		{"dict(a=x, b=1)", "dict[str, Unknown]"},
	}
	for _, tt := range tests {
		if got := InferExprType(parseExpr(t, tt.code)).String(); got != tt.want {
			t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestInferDictKeywordUnion holds the cases that fail when dict() inference
// only looks at the first keyword argument: a later keyword changes the result.
func TestInferDictKeywordUnion(t *testing.T) {
	tests := []struct{ code, want string }{
		{"dict(a=1, b=\"s\")", "dict[str, int | str]"},
		{"dict(a=1, b=\"s\")[\"b\"]", "int | str"},
		{"dict(a=1, b=2, c=\"s\")", "dict[str, int | str]"},
		{"dict(a=1, b=None)", "dict[str, int | None]"},
		{"dict(a=1, b=x)", "dict[str, Unknown]"},
		{"dict(a=1, b=2, c=x)", "dict[str, Unknown]"},
		{"dict(a=1, b=x, c=\"s\")", "dict[str, Unknown]"},
		{"dict(a=[1], b=[2], c=\"s\")", "dict[str, list[int] | str]"},
	}
	for _, tt := range tests {
		if got := InferExprType(parseExpr(t, tt.code)).String(); got != tt.want {
			t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestInferNestedDictLatency guards the cost of Union over deeply nested dict
// types: deduplication must not re-render the whole subtree at every level.
// Before the fix 1,600 levels took about 7 s (9.5 ms before #89).
func TestInferNestedDictLatency(t *testing.T) {
	const depth = 1600
	code := strings.Repeat("dict(a=1, b=", depth) + "2" + strings.Repeat(")", depth)
	expr := parseExpr(t, code)

	done := make(chan TypeRef, 1)
	start := time.Now()
	go func() { done <- InferExprType(expr) }()
	select {
	case got := <-done:
		t.Logf("inferred %d nested dicts in %v", depth, time.Since(start))
		d, ok := got.(*NamedType)
		if !ok || d.Name != "dict" {
			t.Fatalf("outermost type = %v, want a dict", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("InferExprType on %d nested dict() calls did not finish within 3s", depth)
	}
}

func TestInferSliceTupleUnspecialized(t *testing.T) {
	for _, code := range []string{"(1, 2)[:]", "tuple((1, 2)[:])"} {
		got := InferExprType(parseExpr(t, code)).String()
		if got == "tuple[Unknown]" {
			t.Errorf("InferExprType(%q) = %q, invented singleton tuple", code, got)
		}
	}
}
