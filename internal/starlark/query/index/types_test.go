package index

import (
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestFileTypes(t *testing.T) {
	// Test that File struct can be properly initialized
	f := File{
		Path: "lib/utils.bzl",
		Kind: filekind.KindBzl,
		Defs: []Def{
			{
				Name:      "my_rule",
				File:      "lib/utils.bzl",
				Line:      10,
				Params:    []string{"ctx", "deps"},
				Docstring: "My rule implementation.",
			},
		},
		Loads: []Load{
			{
				Module:  "//lib:common.bzl",
				Symbols: map[string]string{"helper": "helper", "alias": "original"},
				File:    "lib/utils.bzl",
				Line:    1,
			},
		},
		Calls: []Call{
			{
				Function: "register_toolchains",
				Args: []Arg{
					{Name: "", Value: "//toolchains:all"},
				},
				File: "lib/utils.bzl",
				Line: 5,
			},
		},
		Assigns: []Assign{
			{
				Name: "DEFAULT_VISIBILITY",
				File: "lib/utils.bzl",
				Line: 3,
			},
		},
	}

	// Verify all fields are accessible
	if f.Path != "lib/utils.bzl" {
		t.Errorf("File.Path = %q, want %q", f.Path, "lib/utils.bzl")
	}
	if f.Kind != filekind.KindBzl {
		t.Errorf("File.Kind = %v, want %v", f.Kind, filekind.KindBzl)
	}
	if len(f.Defs) != 1 {
		t.Errorf("len(File.Defs) = %d, want 1", len(f.Defs))
	}
	if len(f.Loads) != 1 {
		t.Errorf("len(File.Loads) = %d, want 1", len(f.Loads))
	}
	if len(f.Calls) != 1 {
		t.Errorf("len(File.Calls) = %d, want 1", len(f.Calls))
	}
	if len(f.Assigns) != 1 {
		t.Errorf("len(File.Assigns) = %d, want 1", len(f.Assigns))
	}
}

func TestDefType(t *testing.T) {
	def := Def{
		Name:      "my_function",
		File:      "test.bzl",
		Line:      42,
		Params:    []string{"ctx", "name", "*args", "**kwargs"},
		Docstring: "This is a docstring.",
	}

	if def.Name != "my_function" {
		t.Errorf("Def.Name = %q, want %q", def.Name, "my_function")
	}
	if def.File != "test.bzl" {
		t.Errorf("Def.File = %q, want %q", def.File, "test.bzl")
	}
	if def.Line != 42 {
		t.Errorf("Def.Line = %d, want %d", def.Line, 42)
	}
	if len(def.Params) != 4 {
		t.Errorf("len(Def.Params) = %d, want 4", len(def.Params))
	}
	if def.Docstring != "This is a docstring." {
		t.Errorf("Def.Docstring = %q, want %q", def.Docstring, "This is a docstring.")
	}
}

func TestLoadType(t *testing.T) {
	load := Load{
		Module: "@rules_go//go:def.bzl",
		Symbols: map[string]string{
			"go_library": "go_library",
			"lib":        "go_library",
		},
		File: "BUILD.bazel",
		Line: 1,
	}

	if load.Module != "@rules_go//go:def.bzl" {
		t.Errorf("Load.Module = %q, want %q", load.Module, "@rules_go//go:def.bzl")
	}
	if len(load.Symbols) != 2 {
		t.Errorf("len(Load.Symbols) = %d, want 2", len(load.Symbols))
	}
	if load.Symbols["go_library"] != "go_library" {
		t.Errorf("Load.Symbols[go_library] = %q, want %q", load.Symbols["go_library"], "go_library")
	}
	if load.Symbols["lib"] != "go_library" {
		t.Errorf("Load.Symbols[lib] = %q, want %q", load.Symbols["lib"], "go_library")
	}
	if load.File != "BUILD.bazel" {
		t.Errorf("Load.File = %q, want %q", load.File, "BUILD.bazel")
	}
	if load.Line != 1 {
		t.Errorf("Load.Line = %d, want %d", load.Line, 1)
	}
}

func TestCallType(t *testing.T) {
	call := Call{
		Function: "go_library",
		Args: []Arg{
			{Name: "name", Value: "mylib"},
			{Name: "srcs", Value: "[main.go]"},
			{Name: "", Value: "positional_arg"},
		},
		File: "BUILD.bazel",
		Line: 5,
	}

	if call.Function != "go_library" {
		t.Errorf("Call.Function = %q, want %q", call.Function, "go_library")
	}
	if len(call.Args) != 3 {
		t.Errorf("len(Call.Args) = %d, want 3", len(call.Args))
	}

	// Check keyword argument
	if call.Args[0].Name != "name" {
		t.Errorf("Call.Args[0].Name = %q, want %q", call.Args[0].Name, "name")
	}
	if call.Args[0].Value != "mylib" {
		t.Errorf("Call.Args[0].Value = %q, want %q", call.Args[0].Value, "mylib")
	}

	// Check positional argument
	if call.Args[2].Name != "" {
		t.Errorf("Call.Args[2].Name = %q, want empty string", call.Args[2].Name)
	}

	// Check File and Line
	if call.File != "BUILD.bazel" {
		t.Errorf("Call.File = %q, want %q", call.File, "BUILD.bazel")
	}
	if call.Line != 5 {
		t.Errorf("Call.Line = %d, want %d", call.Line, 5)
	}
}

func TestAssignType(t *testing.T) {
	assign := Assign{
		Name: "MY_CONSTANT",
		File: "defs.bzl",
		Line: 7,
	}

	if assign.Name != "MY_CONSTANT" {
		t.Errorf("Assign.Name = %q, want %q", assign.Name, "MY_CONSTANT")
	}
	if assign.File != "defs.bzl" {
		t.Errorf("Assign.File = %q, want %q", assign.File, "defs.bzl")
	}
	if assign.Line != 7 {
		t.Errorf("Assign.Line = %d, want %d", assign.Line, 7)
	}
}

func TestArgType(t *testing.T) {
	tests := []struct {
		name      string
		arg       Arg
		wantName  string
		wantValue string
	}{
		{
			name:      "keyword argument",
			arg:       Arg{Name: "deps", Value: "[:common]"},
			wantName:  "deps",
			wantValue: "[:common]",
		},
		{
			name:      "positional argument",
			arg:       Arg{Name: "", Value: "hello"},
			wantName:  "",
			wantValue: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.arg.Name != tt.wantName {
				t.Errorf("Arg.Name = %q, want %q", tt.arg.Name, tt.wantName)
			}
			if tt.arg.Value != tt.wantValue {
				t.Errorf("Arg.Value = %q, want %q", tt.arg.Value, tt.wantValue)
			}
		})
	}
}
