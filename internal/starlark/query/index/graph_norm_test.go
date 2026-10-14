package index

import (
	"sort"
	"testing"
)

func TestLoadGraph_RelativeLabelAndPathLookup(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "lib/base.bzl", "base.bzl")
	writeFile := func(name, content string) {
		t.Helper()
		writeTree(t, root, name)
		if err := writeContent(root, name, content); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("lib/utils.bzl", `load(":base.bzl", "x")`+"\n")
	writeFile("top.bzl", `load("//lib/utils.bzl", "y")`+"\n")
	writeFile("a.bzl", `load("//lib:utils.bzl", "y")`+"\n")

	idx := New(root)
	if _, err := idx.AddPattern("//..."); err != nil {
		t.Fatal(err)
	}
	g := idx.BuildLoadGraph()

	// ":base.bzl" in lib/utils.bzl is lib/base.bzl, not the root base.bzl.
	if got := g.LoadedBy("//lib:base.bzl"); len(got) != 1 || got[0] != "lib/utils.bzl" {
		t.Errorf("LoadedBy(//lib:base.bzl) = %v", got)
	}
	if got := g.LoadedBy("//:base.bzl"); len(got) != 0 {
		t.Errorf("LoadedBy(//:base.bzl) = %v, want none", got)
	}

	// Path and label spellings all find the same loaders.
	for _, q := range []string{"lib/utils.bzl", "//lib/utils.bzl", "//lib:utils.bzl"} {
		got := append([]string(nil), g.LoadedBy(q)...)
		sort.Strings(got)
		if len(got) != 2 || got[0] != "a.bzl" || got[1] != "top.bzl" {
			t.Errorf("LoadedBy(%q) = %v, want [a.bzl top.bzl]", q, got)
		}
	}

	// Transitive loads follow the relative label.
	all := g.AllLoads("top.bzl")
	sort.Strings(all)
	if len(all) != 2 {
		t.Errorf("AllLoads(top.bzl) = %v, want utils and lib/base", all)
	}
}
