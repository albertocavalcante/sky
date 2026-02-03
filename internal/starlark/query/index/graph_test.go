package index

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestModuleToPath(t *testing.T) {
	tests := []struct {
		module string
		want   string
	}{
		{"//lib:utils.bzl", "lib/utils.bzl"},
		{"//pkg/sub:file.star", "pkg/sub/file.star"},
		{"//BUILD", "BUILD"},
		{"@repo//lib:utils.bzl", ""}, // external repos not supported
		{"", ""},
		{"//:utils.bzl", "utils.bzl"},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			got := moduleToPath(tt.module)
			if got != tt.want {
				t.Errorf("moduleToPath(%q) = %q, want %q", tt.module, got, tt.want)
			}
		})
	}
}

func TestLoadGraph_LoadedBy(t *testing.T) {
	g := &LoadGraph{
		Forward: map[string][]string{
			"a.bzl": {"//lib:utils.bzl", "//lib:base.bzl"},
			"b.bzl": {"//lib:utils.bzl"},
		},
		Reverse: map[string][]string{
			"//lib:utils.bzl": {"a.bzl", "b.bzl"},
			"//lib:base.bzl":  {"a.bzl"},
		},
	}

	tests := []struct {
		module string
		want   []string
	}{
		{"//lib:utils.bzl", []string{"a.bzl", "b.bzl"}},
		{"//lib:base.bzl", []string{"a.bzl"}},
		{"//nonexistent:file.bzl", nil},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			got := g.LoadedBy(tt.module)
			if len(got) != len(tt.want) {
				t.Errorf("LoadedBy(%q) = %v, want %v", tt.module, got, tt.want)
			}
		})
	}
}

func TestLoadGraph_AllLoads_Simple(t *testing.T) {
	// Create graph: a.bzl -> b.bzl -> c.bzl
	g := &LoadGraph{
		Forward: map[string][]string{
			"a.bzl":     {"//lib:b.bzl"},
			"lib/b.bzl": {"//lib:c.bzl"},
		},
		Reverse: map[string][]string{
			"//lib:b.bzl": {"a.bzl"},
			"//lib:c.bzl": {"lib/b.bzl"},
		},
	}

	loads := g.AllLoads("a.bzl")

	// Should return b.bzl and c.bzl transitively
	if len(loads) != 2 {
		t.Errorf("AllLoads(\"a.bzl\") returned %d loads, want 2: %v", len(loads), loads)
	}

	expected := map[string]bool{"//lib:b.bzl": true, "//lib:c.bzl": true}
	for _, load := range loads {
		if !expected[load] {
			t.Errorf("unexpected load: %s", load)
		}
	}
}

func TestLoadGraph_DetectCycles_WithCycle(t *testing.T) {
	// Create graph with cycle: a -> b -> c -> a
	// All files must be in the same directory for paths to match
	g := &LoadGraph{
		Forward: map[string][]string{
			"lib/a.bzl": {"//lib:b.bzl"},
			"lib/b.bzl": {"//lib:c.bzl"},
			"lib/c.bzl": {"//lib:a.bzl"},
		},
		Reverse: map[string][]string{
			"//lib:b.bzl": {"lib/a.bzl"},
			"//lib:c.bzl": {"lib/b.bzl"},
			"//lib:a.bzl": {"lib/c.bzl"},
		},
	}

	cycles := g.DetectCycles()
	if len(cycles) == 0 {
		t.Error("expected at least one cycle")
	}
}

func TestLoadGraph_DetectCycles_NoCycle(t *testing.T) {
	// Create graph without cycle: a -> b -> c
	g := &LoadGraph{
		Forward: map[string][]string{
			"a.bzl":     {"//lib:b.bzl"},
			"lib/b.bzl": {"//lib:c.bzl"},
		},
		Reverse: map[string][]string{
			"//lib:b.bzl": {"a.bzl"},
			"//lib:c.bzl": {"lib/b.bzl"},
		},
	}

	cycles := g.DetectCycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %d: %v", len(cycles), cycles)
	}
}

func TestBuildLoadGraph(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"a.bzl": `
load("//lib:utils.bzl", "func")

def a():
    func()
`,
		"lib/utils.bzl": `
load("//lib:base.bzl", "base")

def func():
    base()
`,
		"lib/base.bzl": `
def base():
    pass
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	idx := New(tmpDir)
	for path := range files {
		if err := idx.Add(filepath.Join(tmpDir, path)); err != nil {
			t.Fatalf("failed to add %s: %v", path, err)
		}
	}

	graph := idx.BuildLoadGraph()

	// Verify forward graph
	if len(graph.Forward) != 2 {
		t.Errorf("expected 2 files with loads, got %d", len(graph.Forward))
	}

	// a.bzl should load lib/utils.bzl
	if loads := graph.Forward["a.bzl"]; len(loads) != 1 || loads[0] != "//lib:utils.bzl" {
		t.Errorf("a.bzl loads = %v, want [//lib:utils.bzl]", loads)
	}

	// lib/utils.bzl should load lib/base.bzl
	if loads := graph.Forward["lib/utils.bzl"]; len(loads) != 1 || loads[0] != "//lib:base.bzl" {
		t.Errorf("lib/utils.bzl loads = %v, want [//lib:base.bzl]", loads)
	}

	// Verify reverse graph
	loaders := graph.Reverse["//lib:utils.bzl"]
	sort.Strings(loaders)
	if len(loaders) != 1 || loaders[0] != "a.bzl" {
		t.Errorf("//lib:utils.bzl loaded by %v, want [a.bzl]", loaders)
	}
}
