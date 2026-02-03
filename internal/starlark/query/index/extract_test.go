package index

import (
	"testing"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestExtractFile(t *testing.T) {
	content := []byte(`
load("//lib:common.bzl", "helper")

DEFAULT_VALUE = 42

def my_rule(ctx, name = "default"):
    """My rule docstring."""
    pass

cc_library(
    name = "mylib",
    srcs = ["main.cc"],
)
`)

	f, err := build.ParseBzl("test.bzl", content)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	result := ExtractFile(f, "test.bzl", filekind.KindBzl)

	if result.Path != "test.bzl" {
		t.Errorf("Path = %q, want %q", result.Path, "test.bzl")
	}
	if result.Kind != filekind.KindBzl {
		t.Errorf("Kind = %v, want %v", result.Kind, filekind.KindBzl)
	}
	if len(result.Loads) != 1 {
		t.Errorf("len(Loads) = %d, want 1", len(result.Loads))
	}
	if len(result.Defs) != 1 {
		t.Errorf("len(Defs) = %d, want 1", len(result.Defs))
	}
	if len(result.Calls) != 1 {
		t.Errorf("len(Calls) = %d, want 1", len(result.Calls))
	}
	if len(result.Assigns) != 1 {
		t.Errorf("len(Assigns) = %d, want 1", len(result.Assigns))
	}
}

func TestExtractDefs(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantDefs   int
		wantName   string
		wantParams []string
		wantDoc    string
	}{
		{
			name: "simple function",
			content: `def foo():
    pass`,
			wantDefs:   1,
			wantName:   "foo",
			wantParams: nil,
			wantDoc:    "",
		},
		{
			name: "function with params",
			content: `def bar(a, b, c):
    pass`,
			wantDefs:   1,
			wantName:   "bar",
			wantParams: []string{"a", "b", "c"},
			wantDoc:    "",
		},
		{
			name: "function with default params",
			content: `def baz(x, y = 10):
    pass`,
			wantDefs:   1,
			wantName:   "baz",
			wantParams: []string{"x", "y"},
			wantDoc:    "",
		},
		{
			name: "function with *args and **kwargs",
			content: `def variadic(*args, **kwargs):
    pass`,
			wantDefs:   1,
			wantName:   "variadic",
			wantParams: []string{"*args", "**kwargs"},
			wantDoc:    "",
		},
		{
			name: "function with docstring",
			content: `def documented(ctx):
    """This is the docstring."""
    pass`,
			wantDefs:   1,
			wantName:   "documented",
			wantParams: []string{"ctx"},
			wantDoc:    "This is the docstring.",
		},
		{
			name: "multiple functions",
			content: `def func1():
    pass

def func2():
    pass

def func3():
    pass`,
			wantDefs:   3,
			wantName:   "func1",
			wantParams: nil,
			wantDoc:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(tt.content))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			defs := extractDefs(f, "test.bzl")

			if len(defs) != tt.wantDefs {
				t.Errorf("len(defs) = %d, want %d", len(defs), tt.wantDefs)
				return
			}

			if len(defs) > 0 {
				if defs[0].Name != tt.wantName {
					t.Errorf("defs[0].Name = %q, want %q", defs[0].Name, tt.wantName)
				}
				if len(defs[0].Params) != len(tt.wantParams) {
					t.Errorf("len(defs[0].Params) = %d, want %d", len(defs[0].Params), len(tt.wantParams))
				} else {
					for i, p := range defs[0].Params {
						if p != tt.wantParams[i] {
							t.Errorf("defs[0].Params[%d] = %q, want %q", i, p, tt.wantParams[i])
						}
					}
				}
				if defs[0].Docstring != tt.wantDoc {
					t.Errorf("defs[0].Docstring = %q, want %q", defs[0].Docstring, tt.wantDoc)
				}
			}
		})
	}
}

func TestExtractLoads(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantLoads   int
		wantModule  string
		wantSymbols map[string]string
	}{
		{
			name:        "single load",
			content:     `load("//lib:utils.bzl", "helper")`,
			wantLoads:   1,
			wantModule:  "//lib:utils.bzl",
			wantSymbols: map[string]string{"helper": "helper"},
		},
		{
			name:        "load with alias",
			content:     `load("//lib:utils.bzl", my_helper = "helper")`,
			wantLoads:   1,
			wantModule:  "//lib:utils.bzl",
			wantSymbols: map[string]string{"my_helper": "helper"},
		},
		{
			name:        "load with multiple symbols",
			content:     `load("@rules_go//go:def.bzl", "go_library", "go_test", lib = "go_library")`,
			wantLoads:   1,
			wantModule:  "@rules_go//go:def.bzl",
			wantSymbols: map[string]string{"go_library": "go_library", "go_test": "go_test", "lib": "go_library"},
		},
		{
			name: "multiple loads",
			content: `load("//lib:a.bzl", "a")
load("//lib:b.bzl", "b")`,
			wantLoads:   2,
			wantModule:  "//lib:a.bzl",
			wantSymbols: map[string]string{"a": "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(tt.content))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			loads := extractLoads(f, "test.bzl")

			if len(loads) != tt.wantLoads {
				t.Errorf("len(loads) = %d, want %d", len(loads), tt.wantLoads)
				return
			}

			if len(loads) > 0 {
				if loads[0].Module != tt.wantModule {
					t.Errorf("loads[0].Module = %q, want %q", loads[0].Module, tt.wantModule)
				}
				for k, v := range tt.wantSymbols {
					if loads[0].Symbols[k] != v {
						t.Errorf("loads[0].Symbols[%q] = %q, want %q", k, loads[0].Symbols[k], v)
					}
				}
			}
		})
	}
}

func TestExtractCalls(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantCalls    int
		wantFunction string
		wantArgs     []Arg
	}{
		{
			name:         "simple call",
			content:      `print("hello")`,
			wantCalls:    1,
			wantFunction: "print",
			wantArgs:     []Arg{{Name: "", Value: "hello"}},
		},
		{
			name: "rule call with keyword args",
			content: `cc_library(
    name = "mylib",
    srcs = ["main.cc"],
)`,
			wantCalls:    1,
			wantFunction: "cc_library",
			wantArgs: []Arg{
				{Name: "name", Value: "mylib"},
				{Name: "srcs", Value: "[main.cc]"},
			},
		},
		{
			name:         "method call",
			content:      `native.cc_library(name = "lib")`,
			wantCalls:    1,
			wantFunction: "native.cc_library",
			wantArgs:     []Arg{{Name: "name", Value: "lib"}},
		},
		{
			name: "multiple calls",
			content: `foo()
bar()
baz()`,
			wantCalls:    3,
			wantFunction: "foo",
			wantArgs:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := build.ParseBuild("BUILD", []byte(tt.content))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			calls := extractCalls(f, "BUILD")

			if len(calls) != tt.wantCalls {
				t.Errorf("len(calls) = %d, want %d", len(calls), tt.wantCalls)
				return
			}

			if len(calls) > 0 {
				if calls[0].Function != tt.wantFunction {
					t.Errorf("calls[0].Function = %q, want %q", calls[0].Function, tt.wantFunction)
				}
				if len(calls[0].Args) != len(tt.wantArgs) {
					t.Errorf("len(calls[0].Args) = %d, want %d", len(calls[0].Args), len(tt.wantArgs))
				} else {
					for i, arg := range calls[0].Args {
						if arg.Name != tt.wantArgs[i].Name {
							t.Errorf("calls[0].Args[%d].Name = %q, want %q", i, arg.Name, tt.wantArgs[i].Name)
						}
						if arg.Value != tt.wantArgs[i].Value {
							t.Errorf("calls[0].Args[%d].Value = %q, want %q", i, arg.Value, tt.wantArgs[i].Value)
						}
					}
				}
			}
		})
	}
}

func TestExtractAssigns(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantAssigns int
		wantNames   []string
	}{
		{
			name:        "simple assignment",
			content:     `X = 42`,
			wantAssigns: 1,
			wantNames:   []string{"X"},
		},
		{
			name:        "string assignment",
			content:     `NAME = "value"`,
			wantAssigns: 1,
			wantNames:   []string{"NAME"},
		},
		{
			name: "multiple assignments",
			content: `A = 1
B = 2
C = 3`,
			wantAssigns: 3,
			wantNames:   []string{"A", "B", "C"},
		},
		{
			name:        "tuple unpacking",
			content:     `X, Y = (1, 2)`,
			wantAssigns: 2,
			wantNames:   []string{"X", "Y"},
		},
		{
			name:        "list unpacking",
			content:     `[A, B] = [1, 2]`,
			wantAssigns: 2,
			wantNames:   []string{"A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(tt.content))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			assigns := extractAssigns(f, "test.bzl")

			if len(assigns) != tt.wantAssigns {
				t.Errorf("len(assigns) = %d, want %d", len(assigns), tt.wantAssigns)
				return
			}

			for i, name := range tt.wantNames {
				if assigns[i].Name != name {
					t.Errorf("assigns[%d].Name = %q, want %q", i, assigns[i].Name, name)
				}
			}
		})
	}
}

func TestExprToString(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "string literal",
			content: `X = "hello"`,
			want:    "hello",
		},
		{
			name:    "number literal",
			content: `X = 42`,
			want:    "42",
		},
		{
			name:    "identifier",
			content: `X = Y`,
			want:    "Y",
		},
		{
			name:    "list",
			content: `X = ["a", "b"]`,
			want:    "[a, b]",
		},
		{
			name:    "binary expression",
			content: `X = A + B`,
			want:    "A + B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(tt.content))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Get the RHS of the assignment
			if len(f.Stmt) == 0 {
				t.Fatal("No statements found")
			}
			assign, ok := f.Stmt[0].(*build.AssignExpr)
			if !ok {
				t.Fatal("First statement is not an assignment")
			}

			got := exprToString(assign.RHS)
			if got != tt.want {
				t.Errorf("exprToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractDocstring(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "with docstring",
			content: `def foo():
    """This is the docstring."""
    pass`,
			want: "This is the docstring.",
		},
		{
			name: "multiline docstring",
			content: `def foo():
    """
    This is a multiline
    docstring.
    """
    pass`,
			want: "\n    This is a multiline\n    docstring.\n    ",
		},
		{
			name: "no docstring",
			content: `def foo():
    pass`,
			want: "",
		},
		{
			name: "non-string first statement",
			content: `def foo():
    x = 1
    pass`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := build.ParseBzl("test.bzl", []byte(tt.content))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			if len(f.Stmt) == 0 {
				t.Fatal("No statements found")
			}
			def, ok := f.Stmt[0].(*build.DefStmt)
			if !ok {
				t.Fatal("First statement is not a function definition")
			}

			got := extractDocstring(def.Body)
			if got != tt.want {
				t.Errorf("extractDocstring() = %q, want %q", got, tt.want)
			}
		})
	}
}
