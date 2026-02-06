package lsp

import (
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/bazelbuild/buildtools/build"
)

func TestInlayHints_VariableTypes(t *testing.T) {
	content := `x = 42
y = "hello"
z = [1, 2, 3]
config = {"timeout": 30}
flag = True
data = (1, "a")
`
	// Use config with HideForSingleChar=false to show hints for x, y, z
	config := InlayHintConfig{
		ShowVariableTypes:  true,
		ShowParameterTypes: true,
		HideForSingleChar:  false,
		MaxHintLength:      50,
	}
	hints := getInlayHintsWithConfig(t, content, config)

	expected := []struct {
		line  uint32
		label string
	}{
		{0, ": int"},
		{1, ": str"},
		{2, ": list[int]"},
		{3, ": dict[str, int]"},
		{4, ": bool"},
		{5, ": tuple[int, str]"},
	}

	if len(hints) != len(expected) {
		t.Fatalf("got %d hints, want %d", len(hints), len(expected))
	}

	for i, want := range expected {
		got := hints[i]
		if got.Position.Line != want.line {
			t.Errorf("hint[%d] line = %d, want %d", i, got.Position.Line, want.line)
		}
		if got.Label.Value.(string) != want.label {
			t.Errorf("hint[%d] label = %q, want %q", i, got.Label.Value, want.label)
		}
		if got.Kind != protocol.InlayHintKindType {
			t.Errorf("hint[%d] kind = %d, want %d (Type)", i, got.Kind, protocol.InlayHintKindType)
		}
	}
}

func TestInlayHints_TypeComments(t *testing.T) {
	content := `data = get_data()  # type: list[str]
config = load_config()  # type: dict[str, Any]
`
	hints := getInlayHints(t, content)

	expected := []struct {
		line  uint32
		label string
	}{
		{0, ": list[str]"},
		{1, ": dict[str, Any]"},
	}

	if len(hints) != len(expected) {
		t.Fatalf("got %d hints, want %d\nhints: %+v", len(hints), len(expected), hints)
	}

	for i, want := range expected {
		got := hints[i]
		if got.Position.Line != want.line {
			t.Errorf("hint[%d] line = %d, want %d", i, got.Position.Line, want.line)
		}
		if got.Label.Value.(string) != want.label {
			t.Errorf("hint[%d] label = %q, want %q", i, got.Label.Value, want.label)
		}
	}
}

func TestInlayHints_HideShortNames(t *testing.T) {
	content := `x = 1
i = 2
name = "test"
`
	config := InlayHintConfig{
		ShowVariableTypes:  true,
		ShowParameterTypes: true,
		HideForSingleChar:  true,
		MaxHintLength:      50,
	}

	hints := getInlayHintsWithConfig(t, content, config)

	// Should only have hint for "name", not "x" or "i"
	if len(hints) != 1 {
		t.Fatalf("got %d hints, want 1 (only 'name')", len(hints))
	}
	if hints[0].Position.Line != 2 {
		t.Errorf("hint line = %d, want 2", hints[0].Position.Line)
	}
}

func TestInlayHints_ShowShortNames(t *testing.T) {
	content := `x = 1
i = 2
`
	config := InlayHintConfig{
		ShowVariableTypes:  true,
		ShowParameterTypes: true,
		HideForSingleChar:  false, // Show all
		MaxHintLength:      50,
	}

	hints := getInlayHintsWithConfig(t, content, config)

	// Should have hints for both x and i
	if len(hints) != 2 {
		t.Fatalf("got %d hints, want 2", len(hints))
	}
}

func TestInlayHints_NoReassignmentHints(t *testing.T) {
	content := `count = 1
count = 2
count = 3
`
	hints := getInlayHints(t, content)

	// Should only have hint for first assignment
	if len(hints) != 1 {
		t.Fatalf("got %d hints, want 1", len(hints))
	}
	if hints[0].Position.Line != 0 {
		t.Errorf("hint line = %d, want 0", hints[0].Position.Line)
	}
}

func TestInlayHints_UnknownTypeSkipped(t *testing.T) {
	content := `unknown_result = unknown_function()
value = 42
`
	hints := getInlayHints(t, content)

	// Should only have hint for value (unknown function returns Unknown)
	if len(hints) != 1 {
		t.Fatalf("got %d hints, want 1", len(hints))
	}
	if hints[0].Label.Value.(string) != ": int" {
		t.Errorf("hint label = %q, want ': int'", hints[0].Label)
	}
}

func TestInlayHints_BuiltinCalls(t *testing.T) {
	content := `length = len([1, 2, 3])
text = str(42)
items = range(10)
`
	hints := getInlayHints(t, content)

	expected := []struct {
		line  uint32
		label string
	}{
		{0, ": int"},
		{1, ": str"},
		{2, ": list[int]"},
	}

	if len(hints) != len(expected) {
		t.Fatalf("got %d hints, want %d", len(hints), len(expected))
	}

	for i, want := range expected {
		got := hints[i]
		if got.Label.Value.(string) != want.label {
			t.Errorf("hint[%d] label = %q, want %q", i, got.Label.Value, want.label)
		}
	}
}

func TestInlayHints_FunctionBody(t *testing.T) {
	content := `def my_func():
    count = 0
    name = "test"
    return count
`
	hints := getInlayHints(t, content)

	expected := []struct {
		line  uint32
		label string
	}{
		{1, ": int"},
		{2, ": str"},
	}

	if len(hints) != len(expected) {
		t.Fatalf("got %d hints, want %d\nhints: %+v", len(hints), len(expected), hints)
	}

	for i, want := range expected {
		got := hints[i]
		if got.Position.Line != want.line {
			t.Errorf("hint[%d] line = %d, want %d", i, got.Position.Line, want.line)
		}
		if got.Label.Value.(string) != want.label {
			t.Errorf("hint[%d] label = %q, want %q", i, got.Label.Value, want.label)
		}
	}
}

func TestInlayHints_ForLoop(t *testing.T) {
	content := `for item in [1, 2, 3]:
    print(item)
`
	config := InlayHintConfig{
		ShowVariableTypes:  true,
		ShowParameterTypes: true,
		HideForSingleChar:  false, // Show loop vars
		MaxHintLength:      50,
	}

	hints := getInlayHintsWithConfig(t, content, config)

	// Should have hint for loop variable "item"
	if len(hints) != 1 {
		t.Fatalf("got %d hints, want 1", len(hints))
	}
	if hints[0].Label.Value.(string) != ": int" {
		t.Errorf("hint label = %q, want ': int'", hints[0].Label)
	}
}

func TestInlayHints_IfStatement(t *testing.T) {
	content := `if True:
    value = 1
else:
    other = "test"
`
	hints := getInlayHints(t, content)

	expected := []struct {
		line  uint32
		label string
	}{
		{1, ": int"},
		{3, ": str"},
	}

	if len(hints) != len(expected) {
		t.Fatalf("got %d hints, want %d", len(hints), len(expected))
	}

	for i, want := range expected {
		got := hints[i]
		if got.Position.Line != want.line {
			t.Errorf("hint[%d] line = %d, want %d", i, got.Position.Line, want.line)
		}
	}
}

func TestInlayHints_TruncateLongTypes(t *testing.T) {
	content := `data = {"very_long_key_name_here": {"nested": {"deep": [1, 2, 3]}}}
`
	config := InlayHintConfig{
		ShowVariableTypes:  true,
		ShowParameterTypes: true,
		HideForSingleChar:  true,
		MaxHintLength:      20,
	}

	hints := getInlayHintsWithConfig(t, content, config)

	if len(hints) != 1 {
		t.Fatalf("got %d hints, want 1", len(hints))
	}

	// Should be truncated
	if len(hints[0].Label.Value.(string)) > 22 { // ": " + 20 chars
		t.Errorf("hint label too long: %q", hints[0].Label.Value)
	}
	if !strings.HasSuffix(hints[0].Label.Value.(string), "...") {
		t.Errorf("truncated label should end with ...: %q", hints[0].Label.Value)
	}
}

func TestInlayHints_RangeFiltering(t *testing.T) {
	content := `line0 = 1
line1 = 2
line2 = 3
line3 = 4
line4 = 5
`
	// Only request hints for lines 1-3
	rng := protocol.Range{
		Start: protocol.Position{Line: 1, Character: 0},
		End:   protocol.Position{Line: 3, Character: 100},
	}

	file, err := build.ParseDefault("test.star", []byte(content))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	collector := newInlayHintCollector(content, rng, DefaultInlayHintConfig)
	hints := collector.collect(file)

	// Should only have hints for lines 1, 2, 3 (0-based)
	for _, h := range hints {
		if h.Position.Line < 1 || h.Position.Line > 3 {
			t.Errorf("hint at line %d is outside requested range [1, 3]", h.Position.Line)
		}
	}

	if len(hints) != 3 {
		t.Errorf("got %d hints, want 3", len(hints))
	}
}

func TestInlayHints_EmptyFile(t *testing.T) {
	content := ``
	hints := getInlayHints(t, content)

	if len(hints) != 0 {
		t.Errorf("got %d hints, want 0 for empty file", len(hints))
	}
}

func TestInlayHints_CommentOnly(t *testing.T) {
	content := `# This is just a comment
# No actual code
`
	hints := getInlayHints(t, content)

	if len(hints) != 0 {
		t.Errorf("got %d hints, want 0 for comment-only file", len(hints))
	}
}

func TestParseDocstringArgs(t *testing.T) {
	docstring := `Creates a custom target.

    This is a longer description.

    Args:
        name (str): The target name.
        srcs (list[Label]): Source files.
        deps (list[Label], optional): Dependencies.

    Returns:
        Target: The created target.
`

	result := parseDocstringArgs(docstring)

	tests := []struct {
		name     string
		wantType string
	}{
		{"name", "str"},
		{"srcs", "list[Label]"},
		{"deps", "list[Label]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeRef, ok := result[tt.name]
			if !ok {
				t.Fatalf("missing parameter %q", tt.name)
			}
			if typeRef.String() != tt.wantType {
				t.Errorf("type = %q, want %q", typeRef.String(), tt.wantType)
			}
		})
	}
}

func TestParseParamLine(t *testing.T) {
	tests := []struct {
		line     string
		wantName string
		wantType string
	}{
		{"name (str): description", "name", "str"},
		{"srcs (list[Label]): sources", "srcs", "list[Label]"},
		{"deps (list[Label], optional): deps", "deps", "list[Label]"},
		{"foo: no type here", "foo", ""},
		{"bar (int): with type", "bar", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			name, typeStr := parseParamLine(tt.line)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if typeStr != tt.wantType {
				t.Errorf("type = %q, want %q", typeStr, tt.wantType)
			}
		})
	}
}

// Helper functions

func getInlayHints(t *testing.T, content string) []protocol.InlayHint {
	t.Helper()
	return getInlayHintsWithConfig(t, content, DefaultInlayHintConfig)
}

func getInlayHintsWithConfig(t *testing.T, content string, config InlayHintConfig) []protocol.InlayHint {
	t.Helper()

	file, err := build.ParseDefault("test.star", []byte(content))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Request full range
	lines := strings.Count(content, "\n") + 1
	rng := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: uint32(lines), Character: 0},
	}

	collector := newInlayHintCollector(content, rng, config)
	return collector.collect(file)
}
