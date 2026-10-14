package index

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"*.bzl", "a.bzl", true},
		{"*.bzl", "lib/a.bzl", false},
		{"lib/*.bzl", "lib/a.bzl", true},
		{"lib/*.bzl", "lib/x/a.bzl", false},
		{"*/*.bzl", "lib/a.bzl", true},
		{"*/*.bzl", "a.bzl", false},
		{"**/*.bzl", "a.bzl", true},
		{"**/*.bzl", "a/b/c.bzl", true},
		{"lib/**/x.bzl", "lib/x.bzl", true},
		{"lib/**/x.bzl", "lib/a/b/x.bzl", true},
		{"lib/**/x.bzl", "other/x.bzl", false},
		{"**/test_*.bzl", "pkg/test_a.bzl", true},
		{"**/test_*.bzl", "pkg/a.bzl", false},
		{"**/BUILD.bazel", "MODULE.bazel", false},
		{"**/BUILD.bazel", "a/BUILD.bazel", true},
		{"lib/**", "lib/a/b.bzl", true},
	}
	for _, tt := range tests {
		if got := matchGlob(tt.pattern, tt.name); got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func writeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		full := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeContent(root, name, content string) error {
	return os.WriteFile(filepath.Join(root, name), []byte(content), 0o644)
}

func TestDiscoverGlobPatterns(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "a.bzl", "lib/a.bzl", "lib/sub/x.bzl", "lib/x.bzl", "MODULE.bazel", "pkg/BUILD.bazel", "pkg/test_a.bzl", "pkg/b.bzl")

	rel := func(pattern string) []string {
		got, err := Discover(pattern, root)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, p := range got {
			r, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(r))
		}
		sort.Strings(out)
		return out
	}
	eq := func(name string, got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", name, got, want)
				return
			}
		}
	}
	eq("lib/*.bzl", rel("lib/*.bzl"), "lib/a.bzl", "lib/x.bzl")
	eq("*/*.bzl", rel("*/*.bzl"), "lib/a.bzl", "lib/x.bzl", "pkg/b.bzl", "pkg/test_a.bzl")
	eq("lib/**/x.bzl", rel("lib/**/x.bzl"), "lib/sub/x.bzl", "lib/x.bzl")
	eq("**/test_*.bzl", rel("**/test_*.bzl"), "pkg/test_a.bzl")
	eq("**/BUILD.bazel", rel("**/BUILD.bazel"), "pkg/BUILD.bazel")
}

func TestDiscoverHiddenRootNotSkipped(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".hiddenroot")
	writeTree(t, root, "a.bzl", ".git/skip.bzl")
	got, err := Discover("//...", root)
	if err != nil || len(got) != 1 {
		t.Fatalf("Discover(//...) = %v, %v; want only a.bzl", got, err)
	}
	got, err = Discover("**/*.bzl", root)
	if err != nil || len(got) != 1 {
		t.Fatalf("Discover(**/*.bzl) = %v, %v; want only a.bzl", got, err)
	}
}

func TestIndexMatchFilesGlobs(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "a.bzl", "lib/a.bzl", "lib/sub/x.bzl", "lib/x.bzl", "MODULE.bazel", "pkg/BUILD.bazel")
	idx := New(root)
	if _, err := idx.AddPattern("//..."); err != nil {
		t.Fatal(err)
	}
	for pattern, want := range map[string]int{
		"lib/*.bzl":      2,
		"*/*.bzl":        2,
		"lib/**/x.bzl":   2,
		"**/BUILD.bazel": 1,
		"*.bzl":          1,
	} {
		if got := idx.MatchFiles(pattern); len(got) != want {
			t.Errorf("MatchFiles(%q) = %d files, want %d", pattern, len(got), want)
		}
	}
}
