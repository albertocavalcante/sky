package filekind

import (
	"testing"
)

func TestKind_String(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindStarlark, "starlark"},
		{KindBUILD, "BUILD"},
		{KindBzl, "bzl"},
		{KindWORKSPACE, "WORKSPACE"},
		{KindMODULE, "MODULE"},
		{KindBUCK, "BUCK"},
		{KindUnknown, "unknown"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("Kind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKind_IsTopLevel(t *testing.T) {
	tests := []struct {
		kind Kind
		want bool
	}{
		{KindBUILD, true},
		{KindWORKSPACE, true},
		{KindMODULE, true},
		{KindBUCK, true},
		{KindBzl, false},
		{KindStarlark, false},
		{KindUnknown, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := tt.kind.IsTopLevel(); got != tt.want {
				t.Errorf("Kind.IsTopLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKind_IsExtension(t *testing.T) {
	tests := []struct {
		kind Kind
		want bool
	}{
		{KindBzl, true},
		{KindBzlBuck, true},
		{KindBzlmod, true},
		{KindStarlark, true},
		{KindBUILD, false},
		{KindWORKSPACE, false},
		{KindUnknown, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := tt.kind.IsExtension(); got != tt.want {
				t.Errorf("Kind.IsExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKind_IsBazel(t *testing.T) {
	tests := []struct {
		kind Kind
		want bool
	}{
		{KindBUILD, true},
		{KindBzl, true},
		{KindWORKSPACE, true},
		{KindMODULE, true},
		{KindBzlmod, true},
		{KindBUCK, false},
		{KindStarlark, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := tt.kind.IsBazel(); got != tt.want {
				t.Errorf("Kind.IsBazel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKind_IsBuck(t *testing.T) {
	tests := []struct {
		kind Kind
		want bool
	}{
		{KindBUCK, true},
		{KindBzlBuck, true},
		{KindBuckconfig, true},
		{KindBUILD, false},
		{KindStarlark, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := tt.kind.IsBuck(); got != tt.want {
				t.Errorf("Kind.IsBuck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllKinds(t *testing.T) {
	kinds := AllKinds()
	if len(kinds) == 0 {
		t.Error("AllKinds() returned empty slice")
	}

	// Check that KindUnknown is included
	found := false
	for _, k := range kinds {
		if k == KindUnknown {
			found = true
			break
		}
	}
	if !found {
		t.Error("AllKinds() should include KindUnknown")
	}
}

func TestIsStarlarkFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Bazel files (exact matches)
		{"BUILD", true},
		{"BUILD.bazel", true},
		{"WORKSPACE", true},
		{"WORKSPACE.bazel", true},
		{"MODULE.bazel", true},

		// Buck2 files
		{"BUCK", true},

		// Tilt files
		{"Tiltfile", true},

		// Extension-based matches
		{"rules.bzl", true},
		{"defs.bxl", true},
		{"main.star", true},
		{"config.starlark", true},
		{"config.sky", true},
		{"types.skyi", true},
		{"config.axl", true},
		{"deploy.ipd", true},
		{"build.plz", true},
		{"config.pconf", true},
		{"include.pinc", true},
		{"mutable.mpconf", true},

		// Non-Starlark files
		{"main.go", false},
		{"README.md", false},
		{"Makefile", false},
		{"BUILD.txt", false},
		{"config.json", false},
		{"script.py", false},
		{".bzl", true}, // Hidden file with .bzl extension is still a Starlark file
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStarlarkFile(tt.name); got != tt.want {
				t.Errorf("IsStarlarkFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
