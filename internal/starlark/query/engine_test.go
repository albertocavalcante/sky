package query

import (
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

// createTestIndex creates an index with test data.
func createTestIndex(t *testing.T) *index.Index {
	t.Helper()

	// Create index with a temp directory
	idx := index.New(t.TempDir())

	// Manually add files with test data using reflection/direct manipulation
	// Since Index doesn't expose direct file addition, we'll test with Add
	// For unit tests, we use a mock approach

	return idx
}

// mockIndex creates a mock index for testing.
// Since we can't easily mock the index, we create a helper that returns
// items directly for testing the query functions.
type mockIndex struct {
	files []*index.File
}

func (m *mockIndex) newEngine() *Engine {
	idx := index.New(".")
	// We'll use pattern matching through the public API
	return &Engine{index: idx}
}

// TestEngine_EvalString tests the EvalString method.
func TestEngine_EvalString(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	// Test parsing and evaluation
	_, err := engine.EvalString("//...")
	if err != nil {
		t.Errorf("EvalString(\"//...\") error = %v", err)
	}

	// Test parse error
	_, err = engine.EvalString("defs(")
	if err == nil {
		t.Error("EvalString(\"defs(\") expected parse error")
	}
}

// TestEngine_UnknownFunction tests handling of unknown functions.
func TestEngine_UnknownFunction(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	_, err := engine.EvalString("unknown_func(//...)")
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

// TestEngine_BinaryOperations tests set operations.
func TestEngine_BinaryOperations(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:  "union",
			query: "//a/... + //b/...",
		},
		{
			name:  "difference",
			query: "//a/... - //b/...",
		},
		{
			name:  "intersection",
			query: "//a/... ^ //b/...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.EvalString(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvalString(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}

// TestEngine_Filter tests the filter function.
func TestEngine_Filter(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:  "filter defs",
			query: `filter("^_", defs(//...))`,
		},
		{
			name:  "filter files",
			query: `filter("test", //...)`,
		},
		{
			name:    "filter wrong arg type",
			query:   `filter(//..., //...)`,
			wantErr: true,
		},
		{
			name:    "filter wrong arg count",
			query:   `filter("pat")`,
			wantErr: true,
		},
		{
			name:    "filter invalid regex",
			query:   `filter("[invalid", //...)`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.EvalString(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvalString(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}

// TestEngine_FunctionArgValidation tests argument validation for query functions.
func TestEngine_FunctionArgValidation(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "files no args",
			query:   "files()",
			wantErr: true,
		},
		{
			name:    "files too many args",
			query:   "files(//a, //b)",
			wantErr: true,
		},
		{
			name:    "defs no args",
			query:   "defs()",
			wantErr: true,
		},
		{
			name:    "loads no args",
			query:   "loads()",
			wantErr: true,
		},
		{
			name:    "calls one arg",
			query:   "calls(foo)",
			wantErr: true,
		},
		{
			name:    "calls three args",
			query:   "calls(foo, //..., bar)",
			wantErr: true,
		},
		{
			name:    "assigns no args",
			query:   "assigns()",
			wantErr: true,
		},
		{
			name:  "valid files",
			query: "files(//...)",
		},
		{
			name:  "valid defs",
			query: "defs(//...)",
		},
		{
			name:  "valid loads",
			query: "loads(//...)",
		},
		{
			name:  "valid calls",
			query: "calls(foo, //...)",
		},
		{
			name:  "valid assigns",
			query: "assigns(//...)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.EvalString(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvalString(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}

// TestItem_Key tests the key function for deduplication.
func TestItem_Key(t *testing.T) {
	item1 := Item{Type: "def", File: "lib/utils.bzl", Name: "my_func", Line: 10}
	item2 := Item{Type: "def", File: "lib/utils.bzl", Name: "my_func", Line: 10}
	item3 := Item{Type: "def", File: "lib/utils.bzl", Name: "other_func", Line: 20}

	if item1.key() != item2.key() {
		t.Error("identical items should have same key")
	}
	if item1.key() == item3.key() {
		t.Error("different items should have different keys")
	}
}

// TestResult_Empty tests handling of empty results.
func TestResult_Empty(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	result, err := engine.EvalString("//...")
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected empty result, got %d items", len(result.Items))
	}
}

// TestNewEngine tests engine creation.
func TestNewEngine(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)
	if engine == nil {
		t.Error("NewEngine() returned nil")
	}
	if engine.index != idx {
		t.Error("NewEngine() did not set index correctly")
	}
}

// TestEngine_StringExpr tests evaluation of bare string expressions.
func TestEngine_StringExpr(t *testing.T) {
	idx := index.New(t.TempDir())
	engine := NewEngine(idx)

	result, err := engine.EvalString(`"just a string"`)
	if err != nil {
		t.Fatalf("EvalString error = %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("bare string should produce empty result, got %d items", len(result.Items))
	}
}

// Helper to check that results contain expected item types.
func hasItemType(result *Result, itemType string) bool {
	for _, item := range result.Items {
		if item.Type == itemType {
			return true
		}
	}
	return false
}

// Placeholder for filekind usage to prevent import error
var _ = filekind.KindBzl
