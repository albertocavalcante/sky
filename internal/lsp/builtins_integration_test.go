package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// mockProvider is a test provider that returns predefined builtins.
type mockProvider struct {
	builtins builtins.Builtins
	dialects []string
}

func (m *mockProvider) Builtins(dialect string, kind filekind.Kind) (builtins.Builtins, error) {
	return m.builtins, nil
}

func (m *mockProvider) SupportedDialects() []string {
	return m.dialects
}

// testBuiltins returns a mock builtins set for testing.
func testBuiltins() builtins.Builtins {
	return builtins.Builtins{
		Functions: []builtins.Signature{
			{
				Name:       "print",
				Doc:        "Prints values to standard output",
				ReturnType: "None",
				Params: []builtins.Param{
					{Name: "args", Type: "any", Variadic: true},
					{Name: "sep", Type: "string", Default: "\" \""},
				},
			},
			{
				Name:       "len",
				Doc:        "Returns the length of a sequence or collection",
				ReturnType: "int",
				Params: []builtins.Param{
					{Name: "x", Type: "sequence", Required: true},
				},
			},
			{
				Name:       "range",
				Doc:        "Returns a range of integers",
				ReturnType: "range",
				Params: []builtins.Param{
					{Name: "start_or_stop", Type: "int", Required: true},
					{Name: "stop", Type: "int"},
					{Name: "step", Type: "int", Default: "1"},
				},
			},
		},
		Types: []builtins.TypeDef{
			{
				Name: "dict",
				Doc:  "Dictionary type for key-value storage",
				Methods: []builtins.Signature{
					{
						Name:       "get",
						Doc:        "Returns the value for key if present",
						ReturnType: "any",
						Params: []builtins.Param{
							{Name: "key", Type: "any", Required: true},
							{Name: "default", Type: "any", Default: "None"},
						},
					},
					{
						Name:       "keys",
						Doc:        "Returns a list of dictionary keys",
						ReturnType: "list",
					},
				},
			},
			{
				Name: "list",
				Doc:  "List type for ordered sequences",
				Methods: []builtins.Signature{
					{
						Name:       "append",
						Doc:        "Appends an element to the list",
						ReturnType: "None",
						Params: []builtins.Param{
							{Name: "item", Type: "any", Required: true},
						},
					},
				},
			},
		},
		Globals: []builtins.Field{
			{Name: "True", Type: "bool", Doc: "Boolean true constant"},
			{Name: "False", Type: "bool", Doc: "Boolean false constant"},
		},
	}
}

// --- Test 1.1: Server can be initialized with Provider ---

func TestServer_InitializeWithProvider(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}

	server := NewServerWithProvider(nil, provider)
	if server == nil {
		t.Fatal("NewServerWithProvider returned nil")
	}

	if server.builtins == nil {
		t.Error("Server.builtins should not be nil when provider is given")
	}
}

func TestServer_InitializeWithNilProvider(t *testing.T) {
	// Should still work with nil provider (backward compatibility)
	server := NewServerWithProvider(nil, nil)
	if server == nil {
		t.Fatal("NewServerWithProvider returned nil with nil provider")
	}
}

func TestServer_NewServerBackwardCompatibility(t *testing.T) {
	// Existing NewServer should still work
	server := NewServer(nil)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
}

// --- Test 1.2: Completion includes builtins from provider ---

func TestCompletion_IncludesBuiltinFunctions(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "pri", // user typing "pri" expecting "print" completion
	}
	server.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list, ok := result.(*protocol.CompletionList)
	if !ok {
		t.Fatalf("handleCompletion() returned %T, want *protocol.CompletionList", result)
	}

	// Check for "print" in completions
	found := false
	for _, item := range list.Items {
		if item.Label == "print" {
			found = true
			if item.Kind != protocol.CompletionItemKindFunction {
				t.Errorf("print completion kind = %v, want Function", item.Kind)
			}
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'print' builtin completion from provider")
	}
}

func TestCompletion_IncludesBuiltinTypes(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "dic", // user typing "dic" expecting "dict" type
	}
	server.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Check for "dict" type in completions
	found := false
	for _, item := range list.Items {
		if item.Label == "dict" {
			found = true
			// Types should appear as Class or Struct kind
			if item.Kind != protocol.CompletionItemKindClass && item.Kind != protocol.CompletionItemKindStruct {
				t.Errorf("dict completion kind = %v, want Class or Struct", item.Kind)
			}
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'dict' builtin type from provider")
	}
}

func TestCompletion_BuiltinDocumentation(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "le", // user typing "le" expecting "len" completion
	}
	server.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Check that "len" has documentation
	for _, item := range list.Items {
		if item.Label == "len" {
			// Check Detail field contains documentation or signature info
			if item.Detail == "" {
				t.Error("len completion should have documentation in Detail field")
			}
			// The detail should mention it's a builtin or include type info
			if !strings.Contains(item.Detail, "builtin") && !strings.Contains(item.Detail, "int") {
				t.Logf("len completion Detail: %q", item.Detail)
			}
			return
		}
	}
	t.Error("handleCompletion() did not return 'len' completion")
}

func TestCompletion_BuiltinGlobals(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "Tr", // user typing "Tr" expecting "True" global
	}
	server.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Check for "True" global in completions
	found := false
	for _, item := range list.Items {
		if item.Label == "True" {
			found = true
			// Globals should be constants or variables
			if item.Kind != protocol.CompletionItemKindConstant && item.Kind != protocol.CompletionItemKindVariable {
				t.Errorf("True completion kind = %v, want Constant or Variable", item.Kind)
			}
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'True' builtin global from provider")
	}
}

// --- Test 1.3: Hover shows builtin documentation ---

func TestHover_BuiltinFunction(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "print(x)",
	}
	server.mu.Unlock()

	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 2}, // hovering over "print"
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleHover(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleHover() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleHover() returned nil for builtin function")
	}

	hover, ok := result.(*protocol.Hover)
	if !ok {
		t.Fatalf("handleHover() returned %T, want *protocol.Hover", result)
	}

	// Check content contains function signature
	content := hover.Contents.Value.(protocol.MarkupContent).Value
	if !strings.Contains(content, "print") {
		t.Errorf("hover content should contain 'print', got: %s", content)
	}

	// Check content contains documentation
	if !strings.Contains(content, "Prints values") && !strings.Contains(content, "standard output") {
		t.Errorf("hover content should contain documentation, got: %s", content)
	}
}

func TestHover_BuiltinType(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "x = dict()",
	}
	server.mu.Unlock()

	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 5}, // hovering over "dict"
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleHover(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleHover() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleHover() returned nil for builtin type")
	}

	hover, ok := result.(*protocol.Hover)
	if !ok {
		t.Fatalf("handleHover() returned %T, want *protocol.Hover", result)
	}

	// Check content contains type name
	content := hover.Contents.Value.(protocol.MarkupContent).Value
	if !strings.Contains(content, "dict") {
		t.Errorf("hover content should contain 'dict', got: %s", content)
	}

	// Check content contains documentation
	if !strings.Contains(content, "Dictionary") && !strings.Contains(content, "key-value") {
		t.Errorf("hover content should contain type documentation, got: %s", content)
	}
}

func TestHover_BuiltinGlobal(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "x = True",
	}
	server.mu.Unlock()

	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 5}, // hovering over "True"
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleHover(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleHover() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleHover() returned nil for builtin global")
	}

	hover, ok := result.(*protocol.Hover)
	if !ok {
		t.Fatalf("handleHover() returned %T, want *protocol.Hover", result)
	}

	// Check content contains global name
	content := hover.Contents.Value.(protocol.MarkupContent).Value
	if !strings.Contains(content, "True") {
		t.Errorf("hover content should contain 'True', got: %s", content)
	}

	// Check content contains type info
	if !strings.Contains(content, "bool") {
		t.Errorf("hover content should contain type 'bool', got: %s", content)
	}
}

func TestHover_FallbackToDocumentSymbols(t *testing.T) {
	// When hovering over a symbol that's not a builtin,
	// it should fall back to document-defined symbols
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	code := `def my_func():
    """My custom function."""
    pass
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 5}, // hovering over "my_func"
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleHover(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleHover() error = %v", err)
	}

	// Should return hover for the document-defined function
	if result == nil {
		t.Fatal("handleHover() should return hover for document-defined function")
	}

	hover := result.(*protocol.Hover)
	if !strings.Contains(hover.Contents.Value.(protocol.MarkupContent).Value, "my_func") {
		t.Errorf("hover should contain 'my_func', got: %s", hover.Contents.Value.(protocol.MarkupContent).Value)
	}
}

// --- Integration test: Provider used with real JSON data ---

func TestCompletion_WithEmptyProvider(t *testing.T) {
	// Test with provider that returns empty builtins
	provider := &mockProvider{
		builtins: builtins.Builtins{},
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "de", // user typing "de" expecting "def" keyword
	}
	server.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Should still return keywords even with empty provider
	found := false
	for _, item := range list.Items {
		if item.Label == "def" {
			found = true
			break
		}
	}
	if !found {
		t.Error("handleCompletion() should still return keywords even with empty provider")
	}
}
