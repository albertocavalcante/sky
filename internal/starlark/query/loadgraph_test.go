package query

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

// setupLoadGraphTestIndex creates a test index with files that have load relationships.
func setupLoadGraphTestIndex(t *testing.T) (*index.Index, string) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create a file structure with load dependencies:
	// lib/base.bzl - no loads
	// lib/utils.bzl - loads lib/base.bzl
	// lib/advanced.bzl - loads lib/utils.bzl (indirect dep on base.bzl)
	// pkg/BUILD.bazel - loads lib/utils.bzl
	// pkg/macros.bzl - loads lib/advanced.bzl
	files := map[string]string{
		"lib/base.bzl": `
def base_function():
    pass

BASE_CONSTANT = "base"
`,
		"lib/utils.bzl": `
load("//lib:base.bzl", "base_function")

def my_function(name):
    base_function()

def _private_helper():
    pass
`,
		"lib/advanced.bzl": `
load("//lib:utils.bzl", "my_function")

def advanced_function():
    my_function("test")
`,
		"pkg/BUILD.bazel": `
load("//lib:utils.bzl", "my_function")

my_function(
    name = "target1",
)
`,
		"pkg/macros.bzl": `
load("//lib:advanced.bzl", "advanced_function")

def package_macro():
    advanced_function()
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

	idx := index.New(tmpDir)
	for path := range files {
		if err := idx.Add(filepath.Join(tmpDir, path)); err != nil {
			t.Fatalf("failed to add %s: %v", path, err)
		}
	}

	return idx, tmpDir
}

func TestLoadedBy(t *testing.T) {
	idx, _ := setupLoadGraphTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name       string
		query      string
		wantCount  int
		wantFiles  []string
		wantErr    bool
	}{
		{
			name:      "loadedby base.bzl",
			query:     `loadedby("//lib:base.bzl")`,
			wantCount: 1,
			wantFiles: []string{"lib/utils.bzl"},
		},
		{
			name:      "loadedby utils.bzl",
			query:     `loadedby("//lib:utils.bzl")`,
			wantCount: 2,
			wantFiles: []string{"lib/advanced.bzl", "pkg/BUILD.bazel"},
		},
		{
			name:      "loadedby advanced.bzl",
			query:     `loadedby("//lib:advanced.bzl")`,
			wantCount: 1,
			wantFiles: []string{"pkg/macros.bzl"},
		},
		{
			name:      "loadedby non-existent module",
			query:     `loadedby("//nonexistent:file.bzl")`,
			wantCount: 0,
		},
		{
			name:    "loadedby no args",
			query:   `loadedby()`,
			wantErr: true,
		},
		{
			name:    "loadedby too many args",
			query:   `loadedby("//a:b.bzl", "//c:d.bzl")`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvalString(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d items, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  item: %s", item.Name)
				}
			}

			// Check that all items are files
			for _, item := range result.Items {
				if item.Type != "file" {
					t.Errorf("item type = %q, want 'file'", item.Type)
				}
			}

			// Check expected files are present
			for _, wantFile := range tt.wantFiles {
				found := false
				for _, item := range result.Items {
					if item.Name == wantFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected file %q not found in results", wantFile)
				}
			}
		})
	}
}

func TestAllLoads(t *testing.T) {
	idx, _ := setupLoadGraphTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantLoads []string
		wantErr   bool
	}{
		{
			name:      "allloads pkg/macros.bzl - transitive loads",
			query:     `allloads(//pkg/macros.bzl)`,
			wantCount: 3, // advanced.bzl -> utils.bzl -> base.bzl
			wantLoads: []string{"//lib:advanced.bzl", "//lib:utils.bzl", "//lib:base.bzl"},
		},
		{
			name:      "allloads lib/utils.bzl - single load",
			query:     `allloads(//lib/utils.bzl)`,
			wantCount: 1, // only base.bzl
			wantLoads: []string{"//lib:base.bzl"},
		},
		{
			name:      "allloads lib/base.bzl - no loads",
			query:     `allloads(//lib/base.bzl)`,
			wantCount: 0,
		},
		{
			name:    "allloads no args",
			query:   `allloads()`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvalString(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d items, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  load: %s", item.Name)
				}
			}

			// Check that all items are loads
			for _, item := range result.Items {
				if item.Type != "load" {
					t.Errorf("item type = %q, want 'load'", item.Type)
				}
			}

			// Check expected loads are present
			for _, wantLoad := range tt.wantLoads {
				found := false
				for _, item := range result.Items {
					if item.Name == wantLoad {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected load %q not found in results", wantLoad)
				}
			}
		})
	}
}

func TestAllLoadsMultipleFiles(t *testing.T) {
	idx, _ := setupLoadGraphTestIndex(t)
	engine := NewEngine(idx)

	// Test allloads with multiple files
	result, err := engine.EvalString(`allloads(//pkg/...)`)
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}

	// Should have loads from both BUILD.bazel and macros.bzl
	// BUILD.bazel loads utils.bzl which loads base.bzl
	// macros.bzl loads advanced.bzl which loads utils.bzl which loads base.bzl
	// So we expect: utils.bzl, base.bzl, advanced.bzl (deduplicated)
	if len(result.Items) < 3 {
		t.Errorf("expected at least 3 unique loads, got %d", len(result.Items))
		for _, item := range result.Items {
			t.Logf("  load: %s", item.Name)
		}
	}
}

// setupCycleTestIndex creates an index with files that have circular load dependencies.
func setupCycleTestIndex(t *testing.T) (*index.Index, string) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create a cycle: a.bzl -> b.bzl -> c.bzl -> a.bzl
	files := map[string]string{
		"cycle/a.bzl": `
load("//cycle:b.bzl", "func_b")

def func_a():
    func_b()
`,
		"cycle/b.bzl": `
load("//cycle:c.bzl", "func_c")

def func_b():
    func_c()
`,
		"cycle/c.bzl": `
load("//cycle:a.bzl", "func_a")

def func_c():
    func_a()
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

	idx := index.New(tmpDir)
	for path := range files {
		if err := idx.Add(filepath.Join(tmpDir, path)); err != nil {
			t.Fatalf("failed to add %s: %v", path, err)
		}
	}

	return idx, tmpDir
}

func TestCycleDetection(t *testing.T) {
	idx, _ := setupCycleTestIndex(t)

	// Build the load graph
	graph := idx.BuildLoadGraph()

	// Detect cycles
	cycles := graph.DetectCycles()

	if len(cycles) == 0 {
		t.Error("expected to detect at least one cycle")
	}

	// The cycle should involve all three files
	t.Logf("detected %d cycle(s)", len(cycles))
	for i, cycle := range cycles {
		t.Logf("  cycle %d: %v", i+1, cycle)
	}
}

func TestCycleDetection_NoCycles(t *testing.T) {
	idx, _ := setupLoadGraphTestIndex(t)

	// Build the load graph
	graph := idx.BuildLoadGraph()

	// Detect cycles - should be none
	cycles := graph.DetectCycles()

	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %d", len(cycles))
		for i, cycle := range cycles {
			t.Logf("  cycle %d: %v", i+1, cycle)
		}
	}
}

func TestAllLoads_WithCycles(t *testing.T) {
	idx, _ := setupCycleTestIndex(t)
	engine := NewEngine(idx)

	// allloads should handle cycles gracefully without infinite loop
	result, err := engine.EvalString(`allloads(//cycle/a.bzl)`)
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}

	// Should return all loads without duplicates
	// a.bzl -> b.bzl -> c.bzl -> a.bzl (cycle stops here)
	// So: b.bzl, c.bzl, a.bzl
	if len(result.Items) != 3 {
		t.Errorf("expected 3 loads, got %d", len(result.Items))
	}

	// Check all are load items
	for _, item := range result.Items {
		if item.Type != "load" {
			t.Errorf("item type = %q, want 'load'", item.Type)
		}
		t.Logf("  load: %s", item.Name)
	}
}

func TestLoadGraph_NilSafety(t *testing.T) {
	var graph *index.LoadGraph

	// These should not panic
	result := graph.LoadedBy("//lib:utils.bzl")
	if result != nil {
		t.Errorf("LoadedBy on nil graph should return nil, got %v", result)
	}

	allLoads := graph.AllLoads("lib/utils.bzl")
	if allLoads != nil {
		t.Errorf("AllLoads on nil graph should return nil, got %v", allLoads)
	}

	cycles := graph.DetectCycles()
	if cycles != nil {
		t.Errorf("DetectCycles on nil graph should return nil, got %v", cycles)
	}
}

func TestLoadGraph_BuildFromIndex(t *testing.T) {
	idx, _ := setupLoadGraphTestIndex(t)

	graph := idx.BuildLoadGraph()

	// Verify forward graph has correct entries
	if len(graph.Forward) != 4 {
		t.Errorf("expected 4 files with loads, got %d", len(graph.Forward))
	}

	// lib/utils.bzl should load lib/base.bzl
	if loads, ok := graph.Forward["lib/utils.bzl"]; ok {
		if len(loads) != 1 || loads[0] != "//lib:base.bzl" {
			t.Errorf("lib/utils.bzl loads = %v, want [//lib:base.bzl]", loads)
		}
	} else {
		t.Error("lib/utils.bzl not in forward graph")
	}

	// Verify reverse graph
	if loaders := graph.Reverse["//lib:utils.bzl"]; len(loaders) != 2 {
		t.Errorf("//lib:utils.bzl loaded by %d files, want 2", len(loaders))
	}
}

func TestModuleToPath(t *testing.T) {
	// This tests the internal moduleToPath function indirectly through AllLoads
	idx, _ := setupLoadGraphTestIndex(t)
	engine := NewEngine(idx)

	// When we call allloads, it needs to resolve module labels to file paths
	// for transitive lookup
	result, err := engine.EvalString(`allloads(//pkg/macros.bzl)`)
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}

	// Should have transitive loads: advanced.bzl -> utils.bzl -> base.bzl
	expectedModules := []string{"//lib:advanced.bzl", "//lib:utils.bzl", "//lib:base.bzl"}

	gotModules := make([]string, len(result.Items))
	for i, item := range result.Items {
		gotModules[i] = item.Name
	}

	sort.Strings(gotModules)
	sort.Strings(expectedModules)

	if len(gotModules) != len(expectedModules) {
		t.Errorf("got %d modules, want %d: got %v, want %v",
			len(gotModules), len(expectedModules), gotModules, expectedModules)
	}
}
