package index

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func relPaths(root string, paths []string) []string {
	var out []string
	for _, p := range paths {
		r, _ := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(r))
	}
	sort.Strings(out)
	return out
}

func TestDiscoverEmbeddedDoubleStarIsNotRecursive(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, ".config/a.bzl", ".config/ab.bzl", ".config/sub/a.bzl")
	got, err := Discover(".config/a**.bzl", root)
	if err != nil {
		t.Fatal(err)
	}
	rel := relPaths(root, got)
	if strings.Join(rel, ",") != ".config/a.bzl,.config/ab.bzl" {
		t.Errorf("Discover(.config/a**.bzl) = %v, want [.config/a.bzl .config/ab.bzl]", rel)
	}
}

func TestHasRecursiveSegment(t *testing.T) {
	for pattern, want := range map[string]bool{
		"**":          true,
		"**/*.bzl":    true,
		"lib/**/x":    true,
		"lib/**":      true,
		"a**.bzl":     false,
		"lib/a**/x":   false,
		"**a/x.bzl":   false,
		"*.bzl":       false,
		"lib/***/x":   false,
		"plain/x.bzl": false,
	} {
		if got := hasRecursiveSegment(pattern); got != want {
			t.Errorf("hasRecursiveSegment(%q) = %v, want %v", pattern, got, want)
		}
	}
}

func TestMatchGlobAlternatingDoubleStarIsPolynomial(t *testing.T) {
	pattern := strings.Repeat("**/a/", 28) + "z"
	done := make(chan bool, 2)
	go func() { done <- matchGlob(pattern, strings.Repeat("a/", 28)+"x") }()
	go func() { done <- matchGlob(pattern, strings.Repeat("a/", 28)+"z") }()
	var results []bool
	timeout := time.After(5 * time.Second)
	for range 2 {
		select {
		case r := <-done:
			results = append(results, r)
		case <-timeout:
			t.Fatal("matchGlob did not finish in 5s: exponential backtracking")
		}
	}
	trues := 0
	for _, r := range results {
		if r {
			trues++
		}
	}
	if trues != 1 {
		t.Errorf("expected exactly the z-ending name to match, got results %v", results)
	}
}

func TestMatchFilesAgreesWithDiscoverOnMetacharacters(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "lib/a1.bzl", "lib/a2.bzl", "lib/ab.bzl")
	idx := New(root)
	if _, err := idx.AddPattern("//..."); err != nil {
		t.Fatal(err)
	}
	for pattern, want := range map[string]int{
		"lib/a?.bzl":    3,
		"lib/a[12].bzl": 2,
	} {
		if got := idx.MatchFiles(pattern); len(got) != want {
			t.Errorf("MatchFiles(%q) = %d files, want %d", pattern, len(got), want)
		}
		disc, err := Discover(pattern, root)
		if err != nil || len(disc) != want {
			t.Errorf("Discover(%q) = %v, %v; want %d files", pattern, disc, err, want)
		}
	}
}

func TestDiscoverSymlinks(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "real/x.bzl", "lib/real.bzl")
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	link(filepath.Join(root, "real"), "lib/cache.bzl")            // directory named like a file
	link(filepath.Join(root, "lib", "real.bzl"), "lib/alias.bzl") // link to a file: kept
	link(filepath.Join(root, "missing"), "lib/dangling.bzl")      // dangling: skipped

	all := "lib/alias.bzl,lib/real.bzl,real/x.bzl"
	lib := "lib/alias.bzl,lib/real.bzl"
	for pattern, want := range map[string]string{
		"//...":       all,
		"**/*.bzl":    all,
		"//lib/...":   lib,
		"//lib":       lib,
		"lib/*.bzl":   lib,
		"lib/**/*.b*": lib,
	} {
		got, err := Discover(pattern, root)
		if err != nil {
			t.Fatalf("Discover(%q): %v", pattern, err)
		}
		if rel := strings.Join(relPaths(root, got), ","); rel != want {
			t.Errorf("Discover(%q) = %s, want %s", pattern, rel, want)
		}
	}
}
