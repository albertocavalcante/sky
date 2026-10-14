package types

import "testing"

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

func TestInferSliceTupleUnspecialized(t *testing.T) {
	for _, code := range []string{"(1, 2)[:]", "tuple((1, 2)[:])"} {
		got := InferExprType(parseExpr(t, code)).String()
		if got == "tuple[Unknown]" {
			t.Errorf("InferExprType(%q) = %q, invented singleton tuple", code, got)
		}
	}
}
