package types

import (
	"testing"

	"github.com/bazelbuild/buildtools/build"
)

// parseExpr parses a Starlark expression string into an AST.
func parseExpr(t *testing.T, code string) build.Expr {
	t.Helper()
	// Wrap expression in an assignment to make it a valid statement
	wrapped := "_ = " + code
	file, err := build.ParseDefault("test.star", []byte(wrapped))
	if err != nil {
		t.Fatalf("Failed to parse expression %q: %v", code, err)
	}
	if len(file.Stmt) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(file.Stmt))
	}
	assign, ok := file.Stmt[0].(*build.AssignExpr)
	if !ok {
		t.Fatalf("Expected AssignExpr, got %T", file.Stmt[0])
	}
	return assign.RHS
}

func TestInferExprType_Literals(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		// Integers
		{"42", "int"},
		{"0", "int"},
		{"-1", "int"},
		{"1000000", "int"},
		{"0x10", "int"},
		{"0o17", "int"},
		{"0b1010", "int"},

		// Floats
		{"3.14", "float"},
		{"0.0", "float"},
		{".5", "float"},
		{"1e10", "float"},
		{"1.5e-3", "float"},

		// Booleans
		{"True", "bool"},
		{"False", "bool"},

		// Strings
		{`"hello"`, "str"},
		{`'world'`, "str"},
		{`""`, "str"},
		{`'''multi\nline'''`, "str"},

		// None
		{"None", "None"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Lists(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"[]", "list[Unknown]"},
		{"[1]", "list[int]"},
		{"[1, 2, 3]", "list[int]"},
		{`["a", "b"]`, "list[str]"},
		{"[True, False]", "list[bool]"},
		{"[[1, 2], [3, 4]]", "list[list[int]]"},
		{`[{"a": 1}]`, "list[dict[str, int]]"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Dicts(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"{}", "dict[Unknown, Unknown]"},
		{`{"a": 1}`, "dict[str, int]"},
		{`{"a": 1, "b": 2}`, "dict[str, int]"},
		{`{1: "a"}`, "dict[int, str]"},
		{`{"x": [1, 2]}`, "dict[str, list[int]]"},
		{`{"nested": {"inner": 1}}`, "dict[str, dict[str, int]]"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Tuples(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"()", "tuple"},
		{"(1,)", "tuple[int]"},
		{"(1, 2)", "tuple[int, int]"},
		{`(1, "a")`, "tuple[int, str]"},
		{`(1, "a", True)`, "tuple[int, str, bool]"},
		{"((1, 2), (3, 4))", "tuple[tuple[int, int], tuple[int, int]]"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Builtins(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"len(x)", "int"},
		{"str(42)", "str"},
		{"int(x)", "int"},
		{"bool(x)", "bool"},
		{"float(x)", "float"},
		{"range(10)", "list[int]"},
		{"sorted(x)", "list[Unknown]"},
		{"reversed(x)", "list[Unknown]"},
		{"all(x)", "bool"},
		{"any(x)", "bool"},
		{"hasattr(x, y)", "bool"},
		{"type(x)", "str"},
		{"repr(x)", "str"},
		{"hash(x)", "int"},
		{"dir(x)", "list[str]"},
		{"print(x)", "None"},
		{"fail()", "None"},
		{"list()", "list[Unknown]"},
		{"dict()", "dict[Unknown, Unknown]"},
		{"tuple()", "tuple"},
		{"abs(x)", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_BinaryOps(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		// Arithmetic
		{"1 + 2", "int"},
		{"1 - 2", "int"},
		{"2 * 3", "int"},
		{"10 // 3", "int"},
		{"10 % 3", "int"},
		{"1.0 + 2", "float"},
		{"1 + 2.0", "float"},
		{"10 / 3", "float"},

		// String concatenation
		{`"a" + "b"`, "str"},

		// String repetition
		{`"a" * 3`, "str"},

		// List concatenation
		{"[1] + [2]", "list[int]"},

		// Comparison
		{"1 == 2", "bool"},
		{"1 != 2", "bool"},
		{"1 < 2", "bool"},
		{"1 > 2", "bool"},
		{"1 <= 2", "bool"},
		{"1 >= 2", "bool"},
		{"1 in [1, 2]", "bool"},
		{"1 not in [1, 2]", "bool"},

		// Logical
		{"True and False", "bool"},
		{"True or False", "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_UnaryOps(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"not True", "bool"},
		{"not False", "bool"},
		{"-1", "int"},
		{"+1", "int"},
		{"-1.5", "float"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Indexing(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{`"hello"[0]`, "str"},
		{"[1, 2, 3][0]", "int"},
		{`{"a": 1}["a"]`, "int"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Slicing(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{`"hello"[0:2]`, "str"},
		{`"hello"[:]`, "str"},
		{"[1, 2, 3][0:2]", "list[int]"},
		{"[1, 2, 3][1:]", "list[int]"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Comprehensions(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"[x for x in []]", "list[Unknown]"},
		{"[x for x in [1, 2, 3]]", "list[Unknown]"}, // x is unknown without scope
		{"[1 for _ in [1, 2, 3]]", "list[int]"},     // literal in body
		{`{k: v for k, v in {}}`, "dict[Unknown, Unknown]"},
		{`{k: 1 for k in []}`, "dict[Unknown, int]"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestInferExprType_Conditional(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"1 if True else 2", "int"},
		{`"a" if True else "b"`, "str"},
		{`1 if True else "a"`, "int | str"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			expr := parseExpr(t, tt.code)
			got := InferExprType(expr)
			if got.String() != tt.want {
				t.Errorf("InferExprType(%q) = %q, want %q", tt.code, got.String(), tt.want)
			}
		})
	}
}

func TestBuiltinReturnType(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"len", "int"},
		{"str", "str"},
		{"int", "int"},
		{"bool", "bool"},
		{"float", "float"},
		{"list", "list[Unknown]"},
		{"dict", "dict[Unknown, Unknown]"},
		{"range", "list[int]"},
		{"sorted", "list[Unknown]"},
		{"all", "bool"},
		{"any", "bool"},
		{"print", "None"},
		{"fail", "None"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuiltinReturnType(tt.name)
			if got == nil {
				t.Fatalf("BuiltinReturnType(%q) = nil", tt.name)
			}
			if got.String() != tt.want {
				t.Errorf("BuiltinReturnType(%q) = %q, want %q", tt.name, got.String(), tt.want)
			}
		})
	}
}

func TestBuiltinReturnType_Unknown(t *testing.T) {
	got := BuiltinReturnType("not_a_builtin")
	if got != nil {
		t.Errorf("BuiltinReturnType(\"not_a_builtin\") = %v, want nil", got)
	}
}

func TestGetBuiltinSignature(t *testing.T) {
	sig := GetBuiltinSignature("len")
	if sig == nil {
		t.Fatal("GetBuiltinSignature(\"len\") = nil")
	}
	if sig.Name != "len" {
		t.Errorf("sig.Name = %q, want \"len\"", sig.Name)
	}
	if len(sig.Params) != 1 {
		t.Errorf("len(sig.Params) = %d, want 1", len(sig.Params))
	}
	if sig.ReturnType.String() != "int" {
		t.Errorf("sig.ReturnType = %q, want \"int\"", sig.ReturnType.String())
	}
}

func TestIsBuiltin(t *testing.T) {
	tests := []struct {
		name    string
		builtin bool
	}{
		{"len", true},
		{"str", true},
		{"int", true},
		{"print", true},
		{"not_a_builtin", false},
		{"MyFunc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBuiltin(tt.name); got != tt.builtin {
				t.Errorf("IsBuiltin(%q) = %v, want %v", tt.name, got, tt.builtin)
			}
		})
	}
}
