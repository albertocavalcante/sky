package query

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

// setupTestIndex creates a test index with some sample files.
func setupTestIndex(t *testing.T) (*index.Index, string) {
	t.Helper()

	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"lib/utils.bzl": `
def my_function(name, deps = []):
    """A utility function."""
    pass

def _private_helper():
    pass

MY_CONSTANT = "value"
`,
		"lib/BUILD.bazel": `
load("//lib:utils.bzl", "my_function")

my_function(
    name = "target1",
    deps = [":dep1"],
)

cc_library(
    name = "mylib",
    srcs = ["lib.cc"],
)
`,
		"pkg/defs.bzl": `
load("//lib:utils.bzl", "my_function", helper = "_private_helper")

def another_function():
    pass

SOME_LIST = ["a", "b", "c"]
`,
		"pkg/BUILD.bazel": `
load("//pkg:defs.bzl", "another_function")

another_function()
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

	// Create index and add files
	idx := index.New(tmpDir)
	for path := range files {
		if err := idx.Add(filepath.Join(tmpDir, path)); err != nil {
			t.Fatalf("failed to add %s: %v", path, err)
		}
	}

	return idx, tmpDir
}

func TestEvalFiles(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "all files",
			query:     "files(//...)",
			wantCount: 4,
		},
		{
			name:      "lib package",
			query:     "files(//lib/...)",
			wantCount: 2,
		},
		{
			name:      "pkg package",
			query:     "files(//pkg/...)",
			wantCount: 2,
		},
		{
			name:      "bzl files",
			query:     "files(**/*.bzl)",
			wantCount: 2,
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
				t.Errorf("EvalString(%q) got %d items, want %d", tt.query, len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  item: %s (%s)", item.Name, item.Type)
				}
			}
		})
	}
}

func TestEvalDefs(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantNames []string
	}{
		{
			name:      "all defs",
			query:     "defs(//...)",
			wantCount: 3, // my_function, _private_helper, another_function
			wantNames: []string{"my_function", "_private_helper", "another_function"},
		},
		{
			name:      "lib defs",
			query:     "defs(//lib/...)",
			wantCount: 2,
			wantNames: []string{"my_function", "_private_helper"},
		},
		{
			name:      "pkg defs",
			query:     "defs(//pkg/...)",
			wantCount: 1,
			wantNames: []string{"another_function"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if err != nil {
				t.Fatalf("EvalString(%q) error = %v", tt.query, err)
			}

			// Check count
			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d defs, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  def: %s in %s", item.Name, item.File)
				}
			}

			// Check that all items are defs
			for _, item := range result.Items {
				if item.Type != "def" {
					t.Errorf("item type = %q, want 'def'", item.Type)
				}
			}
		})
	}
}

func TestEvalLoads(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "all loads",
			query:     "loads(//...)",
			wantCount: 3,
		},
		{
			name:      "lib loads",
			query:     "loads(//lib/...)",
			wantCount: 1,
		},
		{
			name:      "pkg loads",
			query:     "loads(//pkg/...)",
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if err != nil {
				t.Fatalf("EvalString(%q) error = %v", tt.query, err)
			}
			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d loads, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  load: %s in %s", item.Name, item.File)
				}
			}
		})
	}
}

func TestEvalCalls(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "all calls",
			query:     "calls(*, //...)",
			wantCount: 3, // my_function, cc_library, another_function
		},
		{
			name:      "my_function calls",
			query:     "calls(my_function, //...)",
			wantCount: 1,
		},
		{
			name:      "cc_library calls",
			query:     "calls(cc_library, //...)",
			wantCount: 1,
		},
		{
			name:      "another_function calls",
			query:     "calls(another_function, //...)",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if err != nil {
				t.Fatalf("EvalString(%q) error = %v", tt.query, err)
			}
			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d calls, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  call: %s in %s:%d", item.Name, item.File, item.Line)
				}
			}
		})
	}
}

func TestEvalAssigns(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantNames []string
	}{
		{
			name:      "all assigns",
			query:     "assigns(//...)",
			wantCount: 2,
			wantNames: []string{"MY_CONSTANT", "SOME_LIST"},
		},
		{
			name:      "lib assigns",
			query:     "assigns(//lib/...)",
			wantCount: 1,
			wantNames: []string{"MY_CONSTANT"},
		},
		{
			name:      "pkg assigns",
			query:     "assigns(//pkg/...)",
			wantCount: 1,
			wantNames: []string{"SOME_LIST"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if err != nil {
				t.Fatalf("EvalString(%q) error = %v", tt.query, err)
			}
			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d assigns, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  assign: %s in %s", item.Name, item.File)
				}
			}
		})
	}
}

func TestEvalFilter(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "filter private defs",
			query:     `filter("^_", defs(//...))`,
			wantCount: 1, // _private_helper
		},
		{
			name:      "filter public defs",
			query:     `filter("^[^_]", defs(//...))`,
			wantCount: 2, // my_function, another_function
		},
		{
			name:      "filter by function suffix",
			query:     `filter("_function$", defs(//...))`,
			wantCount: 2, // my_function, another_function
		},
		{
			name:      "filter loads",
			query:     `filter("utils", loads(//...))`,
			wantCount: 2, // loads from //lib:utils.bzl
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if err != nil {
				t.Fatalf("EvalString(%q) error = %v", tt.query, err)
			}
			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d items, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  item: %s (%s)", item.Name, item.Type)
				}
			}
		})
	}
}

func TestSetOperations(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "union",
			query:     "defs(//lib/...) + defs(//pkg/...)",
			wantCount: 3, // 2 from lib + 1 from pkg
		},
		{
			name:      "difference",
			query:     "defs(//...) - defs(//pkg/...)",
			wantCount: 2, // all defs minus pkg defs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.EvalString(tt.query)
			if err != nil {
				t.Fatalf("EvalString(%q) error = %v", tt.query, err)
			}
			if len(result.Items) != tt.wantCount {
				t.Errorf("got %d items, want %d", len(result.Items), tt.wantCount)
				for _, item := range result.Items {
					t.Logf("  item: %s (%s) in %s", item.Name, item.Type, item.File)
				}
			}
		})
	}
}

func TestEvalCalls_WildcardPattern(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	// Test that "*" returns all calls
	result, err := engine.EvalString("calls(*, //...)")
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}

	if len(result.Items) == 0 {
		t.Error("expected at least one call")
	}

	// All items should be calls
	for _, item := range result.Items {
		if item.Type != "call" {
			t.Errorf("item type = %q, want 'call'", item.Type)
		}
	}
}

func TestEvalCalls_SpecificFunction(t *testing.T) {
	idx, _ := setupTestIndex(t)
	engine := NewEngine(idx)

	result, err := engine.EvalString("calls(cc_library, //...)")
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("got %d calls, want 1", len(result.Items))
	}

	if len(result.Items) > 0 && result.Items[0].Name != "cc_library" {
		t.Errorf("call name = %q, want 'cc_library'", result.Items[0].Name)
	}
}
