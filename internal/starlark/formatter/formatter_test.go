package formatter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		path    string
		kind    filekind.Kind
		want    string
		wantErr bool
	}{
		// BUILD file formatting
		{
			name: "format BUILD file with bad spacing",
			src:  `cc_library(name="foo",srcs=["foo.cc"])`,
			path: "BUILD",
			kind: filekind.KindBUILD,
			want: `cc_library(
    name = "foo",
    srcs = ["foo.cc"],
)
`,
		},
		{
			name: "format BUILD file already formatted",
			src: `cc_library(
    name = "foo",
    srcs = ["foo.cc"],
)
`,
			path: "BUILD",
			kind: filekind.KindBUILD,
			want: `cc_library(
    name = "foo",
    srcs = ["foo.cc"],
)
`,
		},

		// .bzl file formatting
		{
			name: "format bzl file",
			src:  `def foo(x,y): return x+y`,
			path: "defs.bzl",
			kind: filekind.KindBzl,
			want: `def foo(x, y):
    return x + y
`,
		},

		// WORKSPACE formatting
		{
			name: "format WORKSPACE file",
			src:  `workspace(name="my_workspace")`,
			path: "WORKSPACE",
			kind: filekind.KindWORKSPACE,
			want: `workspace(name = "my_workspace")
`,
		},

		// MODULE.bazel formatting
		{
			name: "format MODULE.bazel file",
			src:  `module(name="my_module",version="1.0")`,
			path: "MODULE.bazel",
			kind: filekind.KindMODULE,
			want: `module(
    name = "my_module",
    version = "1.0",
)
`,
		},

		// Generic Starlark formatting
		{
			name: "format generic starlark file",
			src:  `x=1+2`,
			path: "script.star",
			kind: filekind.KindStarlark,
			want: `x = 1 + 2
`,
		},

		// BUCK file formatting (uses BUILD parser)
		{
			name: "format BUCK file",
			src:  `cxx_library(name="foo",srcs=["foo.cpp"])`,
			path: "BUCK",
			kind: filekind.KindBUCK,
			want: `cxx_library(
    name = "foo",
    srcs = ["foo.cpp"],
)
`,
		},

		// Auto-detect with KindUnknown
		{
			name: "auto-detect kind when unknown",
			src:  `x=1`,
			path: "unknown.txt",
			kind: filekind.KindUnknown,
			want: `x = 1
`,
		},

		// Empty kind string
		{
			name: "empty kind string uses default",
			src:  `y=2`,
			path: "foo.star",
			kind: "",
			want: `y = 2
`,
		},

		// Parse error
		{
			name:    "parse error returns error",
			src:     `def foo(: return`,
			path:    "bad.bzl",
			kind:    filekind.KindBzl,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format([]byte(tt.src), tt.path, tt.kind)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("Format() =\n%q\nwant:\n%q", string(got), tt.want)
			}
		})
	}
}

func TestResult_Changed(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   bool
	}{
		{
			name: "changed when content differs",
			result: Result{
				Original:  []byte("x=1"),
				Formatted: []byte("x = 1\n"),
			},
			want: true,
		},
		{
			name: "not changed when content same",
			result: Result{
				Original:  []byte("x = 1\n"),
				Formatted: []byte("x = 1\n"),
			},
			want: false,
		},
		{
			name: "not changed when error",
			result: Result{
				Original:  []byte("x=1"),
				Formatted: nil,
				Err:       os.ErrNotExist,
			},
			want: false,
		},
		{
			name: "changed when length differs",
			result: Result{
				Original:  []byte("x"),
				Formatted: []byte("xy"),
			},
			want: true,
		},
		{
			name: "not changed when both empty",
			result: Result{
				Original:  []byte{},
				Formatted: []byte{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Changed(); got != tt.want {
				t.Errorf("Changed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "skyfmt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	tests := []struct {
		name       string
		filename   string
		content    string
		wantChange bool
		wantErr    bool
	}{
		{
			name:       "format BUILD file",
			filename:   "BUILD",
			content:    `cc_library(name="foo",srcs=["foo.cc"])`,
			wantChange: true,
		},
		{
			name:     "already formatted BUILD",
			filename: "BUILD.bazel",
			content: `cc_library(
    name = "foo",
    srcs = ["foo.cc"],
)
`,
			wantChange: false,
		},
		{
			name:       "format bzl file",
			filename:   "rules.bzl",
			content:    `def foo(): return 1`,
			wantChange: true,
		},
		{
			name:       "format star file",
			filename:   "config.star",
			content:    `x=1`,
			wantChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			result := FormatFile(path)

			if (result.Err != nil) != tt.wantErr {
				t.Errorf("FormatFile() error = %v, wantErr %v", result.Err, tt.wantErr)
				return
			}

			if result.Changed() != tt.wantChange {
				t.Errorf("FormatFile() changed = %v, want %v", result.Changed(), tt.wantChange)
			}

			if result.Path != path {
				t.Errorf("FormatFile() path = %v, want %v", result.Path, path)
			}
		})
	}
}

func TestFormatFile_NotFound(t *testing.T) {
	result := FormatFile("/nonexistent/path/BUILD")
	if result.Err == nil {
		t.Error("FormatFile() expected error for nonexistent file")
	}
	if result.Changed() {
		t.Error("FormatFile() Changed() should be false when error")
	}
}

func TestFormatFileWithKind(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "skyfmt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write a file with non-standard name
	path := filepath.Join(tmpDir, "myfile.txt")
	content := `cc_library(name="foo",srcs=["foo.cc"])`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Format with explicit BUILD kind
	result := FormatFileWithKind(path, filekind.KindBUILD)
	if result.Err != nil {
		t.Fatalf("FormatFileWithKind() error = %v", result.Err)
	}

	if !result.Changed() {
		t.Error("FormatFileWithKind() should have detected changes")
	}

	// Verify BUILD-style formatting was applied
	want := `cc_library(
    name = "foo",
    srcs = ["foo.cc"],
)
`
	if string(result.Formatted) != want {
		t.Errorf("FormatFileWithKind() =\n%q\nwant:\n%q", string(result.Formatted), want)
	}
}

func TestDetectKind(t *testing.T) {
	tests := []struct {
		path string
		want filekind.Kind
	}{
		{"BUILD", filekind.KindBUILD},
		{"BUILD.bazel", filekind.KindBUILD},
		{"WORKSPACE", filekind.KindWORKSPACE},
		{"MODULE.bazel", filekind.KindMODULE},
		{"BUCK", filekind.KindBUCK},
		{"defs.bzl", filekind.KindBzl},
		{"script.star", filekind.KindStarlark},
		{"types.skyi", filekind.KindSkyI},
		{"unknown.txt", filekind.KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := detectKind(tt.path); got != tt.want {
				t.Errorf("detectKind(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func BenchmarkFormat(b *testing.B) {
	src := []byte(`cc_library(name="foo",srcs=["foo.cc"],deps=[":bar",":baz"])`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Format(src, "BUILD", filekind.KindBUILD)
		if err != nil {
			b.Fatal(err)
		}
	}
}
