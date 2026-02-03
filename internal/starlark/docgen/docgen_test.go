package docgen

import (
	"bytes"
	"strings"
	"testing"
)

func TestExtractFile(t *testing.T) {
	src := []byte(`"""Module docstring.

This is the module description.
"""

def public_func(name, count=1):
    """A public function.

    Does something useful.

    Args:
        name: The name parameter.
        count: How many times. Defaults to 1.

    Returns:
        A string result.

    Example:
        result = public_func("test", 3)
    """
    pass

def _private_func():
    """A private function."""
    pass

MY_CONSTANT = 42
_PRIVATE_VAR = "hidden"
`)

	doc, err := ExtractFile("test.star", src, DefaultOptions())
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}

	// Check module docstring
	if !strings.Contains(doc.Docstring, "Module docstring") {
		t.Errorf("expected module docstring, got %q", doc.Docstring)
	}

	// Check functions (should only have public)
	if len(doc.Functions) != 1 {
		t.Errorf("expected 1 public function, got %d", len(doc.Functions))
	}

	if doc.Functions[0].Name != "public_func" {
		t.Errorf("expected public_func, got %s", doc.Functions[0].Name)
	}

	// Check function params
	fn := doc.Functions[0]
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	}

	if fn.Params[0].Name != "name" || fn.Params[0].HasDefault {
		t.Errorf("unexpected first param: %+v", fn.Params[0])
	}

	if fn.Params[1].Name != "count" || !fn.Params[1].HasDefault {
		t.Errorf("unexpected second param: %+v", fn.Params[1])
	}

	// Check globals (should only have public)
	if len(doc.Globals) != 1 {
		t.Errorf("expected 1 public global, got %d", len(doc.Globals))
	}

	if doc.Globals[0].Name != "MY_CONSTANT" {
		t.Errorf("expected MY_CONSTANT, got %s", doc.Globals[0].Name)
	}
}

func TestExtractFileWithPrivate(t *testing.T) {
	src := []byte(`
def public():
    pass

def _private():
    pass
`)

	opts := Options{IncludePrivate: true}
	doc, err := ExtractFile("test.star", src, opts)
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}

	if len(doc.Functions) != 2 {
		t.Errorf("expected 2 functions with IncludePrivate, got %d", len(doc.Functions))
	}
}

func TestParseDocstring(t *testing.T) {
	docstring := `Short summary.

This is a longer description that spans
multiple lines.

Args:
    name: The name to use.
    count: How many times to repeat.
        Can be any positive integer.

Returns:
    A formatted string.

Raises:
    ValueError: If count is negative.

Example:
    result = my_func("test", 3)
    print(result)

Note:
    This is a note about the function.
`

	parsed := ParseDocstring(docstring)

	if parsed.Summary != "Short summary." {
		t.Errorf("unexpected summary: %q", parsed.Summary)
	}

	if !strings.Contains(parsed.Description, "longer description") {
		t.Errorf("unexpected description: %q", parsed.Description)
	}

	if len(parsed.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(parsed.Args))
	}

	if parsed.Args["name"] != "The name to use." {
		t.Errorf("unexpected name arg: %q", parsed.Args["name"])
	}

	if !strings.Contains(parsed.Args["count"], "positive integer") {
		t.Errorf("unexpected count arg: %q", parsed.Args["count"])
	}

	if !strings.Contains(parsed.Returns, "formatted string") {
		t.Errorf("unexpected returns: %q", parsed.Returns)
	}

	if parsed.Raises["ValueError"] != "If count is negative." {
		t.Errorf("unexpected raises: %q", parsed.Raises["ValueError"])
	}

	if !strings.Contains(parsed.Example, "my_func") {
		t.Errorf("unexpected example: %q", parsed.Example)
	}

	if !strings.Contains(parsed.Note, "note about") {
		t.Errorf("unexpected note: %q", parsed.Note)
	}
}

func TestParseArgsDirectly(t *testing.T) {
	// Test parseArgsSection directly with known content
	content := `    name: The name to use.
    count: How many times to repeat.
        Can be any positive integer.`

	args := parseArgsSection(content)

	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d: %v", len(args), args)
	}

	if args["name"] != "The name to use." {
		t.Errorf("name = %q", args["name"])
	}

	if !strings.Contains(args["count"], "positive integer") {
		t.Errorf("count = %q", args["count"])
	}
}

func TestParseDocstringNoSections(t *testing.T) {
	docstring := "Just a simple description with no sections."

	parsed := ParseDocstring(docstring)

	if parsed.Summary != docstring {
		t.Errorf("unexpected summary: %q", parsed.Summary)
	}

	if len(parsed.Args) != 0 {
		t.Errorf("expected no args, got %d", len(parsed.Args))
	}
}

func TestRenderMarkdown(t *testing.T) {
	doc := &ModuleDoc{
		File:      "example.star",
		Docstring: "Example module.",
		Functions: []FunctionDoc{
			{
				Name:      "greet",
				Docstring: "Greet someone.\n\nArgs:\n    name: Who to greet.",
				Params: []ParamDoc{
					{Name: "name"},
				},
				Line: 5,
			},
		},
		Globals: []GlobalDoc{
			{Name: "VERSION", Value: `"1.0.0"`, Line: 1},
		},
	}

	// Parse the docstring
	doc.Functions[0].Parsed = ParseDocstring(doc.Functions[0].Docstring)

	var buf bytes.Buffer
	err := RenderMarkdown(&buf, doc, DefaultMarkdownOptions())
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}

	output := buf.String()

	// Check for expected content
	checks := []string{
		"# example.star",
		"Example module.",
		"## Functions",
		"### greet",
		"def greet(name)",
		"Greet someone.",
		"**Arguments:**",
		"| `name` |",
		"## Variables",
		"### `VERSION`",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}
}

func TestHasDocumentation(t *testing.T) {
	tests := []struct {
		name   string
		parsed *ParsedDocstring
		want   bool
	}{
		{"nil", nil, false},
		{"empty", &ParsedDocstring{}, false},
		{"summary only", &ParsedDocstring{Summary: "test"}, true},
		{"args only", &ParsedDocstring{Args: map[string]string{"a": "b"}}, true},
		{"returns only", &ParsedDocstring{Returns: "something"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.parsed.HasDocumentation()
			if got != tc.want {
				t.Errorf("HasDocumentation() = %v, want %v", got, tc.want)
			}
		})
	}
}
