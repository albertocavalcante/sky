package lsp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"go.lsp.dev/protocol"
)

// =============================================================================
// Delta Encoding Tests
// =============================================================================

func TestEncodeTokens_Empty(t *testing.T) {
	tokens := []SemanticToken{}
	encoded := encodeTokens(tokens)

	if len(encoded) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(encoded))
	}
}

func TestEncodeTokens_SingleToken(t *testing.T) {
	tokens := []SemanticToken{
		{Line: 0, StartChar: 4, Length: 3, Type: TokenKeyword, Modifiers: 0},
	}
	encoded := encodeTokens(tokens)

	// Expected: [deltaLine=0, deltaChar=4, length=3, type=8, mods=0]
	expected := []uint32{0, 4, 3, TokenKeyword, 0}
	if !reflect.DeepEqual(encoded, expected) {
		t.Errorf("expected %v, got %v", expected, encoded)
	}
}

func TestEncodeTokens_SameLine(t *testing.T) {
	// Two tokens on the same line: "def foo"
	tokens := []SemanticToken{
		{Line: 0, StartChar: 0, Length: 3, Type: TokenKeyword, Modifiers: 0},              // def
		{Line: 0, StartChar: 4, Length: 3, Type: TokenFunction, Modifiers: ModDefinition}, // foo
	}
	encoded := encodeTokens(tokens)

	// Token 1: deltaLine=0, deltaChar=0, len=3, type=keyword
	// Token 2: deltaLine=0, deltaChar=4 (relative to prev), len=3, type=function
	expected := []uint32{
		0, 0, 3, TokenKeyword, 0,
		0, 4, 3, TokenFunction, ModDefinition,
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Errorf("expected %v, got %v", expected, encoded)
	}
}

func TestEncodeTokens_DifferentLines(t *testing.T) {
	tokens := []SemanticToken{
		{Line: 0, StartChar: 0, Length: 3, Type: TokenKeyword, Modifiers: 0},  // line 0
		{Line: 2, StartChar: 4, Length: 5, Type: TokenVariable, Modifiers: 0}, // line 2
	}
	encoded := encodeTokens(tokens)

	// Token 1: deltaLine=0, deltaChar=0
	// Token 2: deltaLine=2, deltaChar=4 (absolute, since new line)
	expected := []uint32{
		0, 0, 3, TokenKeyword, 0,
		2, 4, 5, TokenVariable, 0,
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Errorf("expected %v, got %v", expected, encoded)
	}
}

func TestEncodeTokens_Modifiers(t *testing.T) {
	tokens := []SemanticToken{
		{Line: 0, StartChar: 0, Length: 3, Type: TokenFunction, Modifiers: ModDeclaration | ModDefinition},
	}
	encoded := encodeTokens(tokens)

	expected := []uint32{0, 0, 3, TokenFunction, ModDeclaration | ModDefinition}
	if !reflect.DeepEqual(encoded, expected) {
		t.Errorf("expected %v, got %v", expected, encoded)
	}
}

// =============================================================================
// Tokenizer Tests - Keywords
// =============================================================================

func TestSemanticTokens_Keywords(t *testing.T) {
	content := `def foo():
    if True:
        return 1
    for x in range(10):
        pass`

	tokens := tokenizeContent(content)

	// Find keyword tokens
	keywords := filterByType(tokens, TokenKeyword)

	// Should have: def, if, return, for, in, pass
	expectedKeywords := []string{"def", "if", "return", "for", "in", "pass"}
	if len(keywords) < len(expectedKeywords) {
		t.Errorf("expected at least %d keywords, got %d", len(expectedKeywords), len(keywords))
	}

	// Verify "def" is at position 0,0
	if len(keywords) > 0 {
		if keywords[0].Line != 0 || keywords[0].StartChar != 0 || keywords[0].Length != 3 {
			t.Errorf("expected 'def' at (0,0) with length 3, got line=%d char=%d len=%d",
				keywords[0].Line, keywords[0].StartChar, keywords[0].Length)
		}
	}
}

func TestSemanticTokens_FunctionDefinition(t *testing.T) {
	content := `def my_function(arg1, arg2):
    return arg1 + arg2`

	tokens := tokenizeContent(content)

	// Find function token
	funcs := filterByType(tokens, TokenFunction)

	if len(funcs) == 0 {
		t.Fatal("expected at least one function token")
	}

	// Function name should have declaration modifier
	if funcs[0].Modifiers&ModDeclaration == 0 {
		t.Error("function definition should have declaration modifier")
	}

	// Should be at the right position (after "def ")
	if funcs[0].StartChar != 4 {
		t.Errorf("expected function at char 4, got %d", funcs[0].StartChar)
	}

	if funcs[0].Length != 11 { // "my_function"
		t.Errorf("expected function length 11, got %d", funcs[0].Length)
	}
}

func TestSemanticTokens_Parameters(t *testing.T) {
	content := `def greet(name, greeting="Hello"):
    return greeting + " " + name`

	tokens := tokenizeContent(content)

	// Find parameter tokens
	params := filterByType(tokens, TokenParameter)

	// Should have at least 2 parameters: name, greeting
	if len(params) < 2 {
		t.Errorf("expected at least 2 parameter tokens, got %d", len(params))
	}
}

func TestSemanticTokens_Builtins(t *testing.T) {
	content := `x = len([1, 2, 3])
y = str(42)
z = print("hello")`

	tokens := tokenizeContent(content)

	// Find function tokens with defaultLibrary modifier
	builtins := filterByTypeAndMod(tokens, TokenFunction, ModDefaultLibrary)

	// Should have: len, str, print
	if len(builtins) < 3 {
		t.Errorf("expected at least 3 builtin functions, got %d", len(builtins))
	}
}

func TestSemanticTokens_Variables(t *testing.T) {
	content := `x = 1
y = x + 2
z = y * x`

	tokens := tokenizeContent(content)

	// Find variable tokens
	vars := filterByType(tokens, TokenVariable)

	// Should have multiple variable references
	if len(vars) < 3 {
		t.Errorf("expected at least 3 variable tokens, got %d", len(vars))
	}

	// First 'x' should be declaration
	decls := filterByTypeAndMod(tokens, TokenVariable, ModDeclaration)
	if len(decls) < 1 {
		t.Error("expected at least one variable declaration")
	}
}

func TestSemanticTokens_Strings(t *testing.T) {
	content := `name = "hello"
path = 'world'
doc = """multi
line"""`

	tokens := tokenizeContent(content)

	// Find string tokens
	strings := filterByType(tokens, TokenString)

	if len(strings) < 3 {
		t.Errorf("expected at least 3 string tokens, got %d", len(strings))
	}
}

func TestSemanticTokens_Numbers(t *testing.T) {
	content := `x = 42
y = 3.14
z = 0xFF`

	tokens := tokenizeContent(content)

	// Find number tokens
	numbers := filterByType(tokens, TokenNumber)

	if len(numbers) < 3 {
		t.Errorf("expected at least 3 number tokens, got %d", len(numbers))
	}
}

func TestSemanticTokens_Comments(t *testing.T) {
	content := `# This is a comment
x = 1  # inline comment`

	tokens := tokenizeContent(content)

	// Find comment tokens
	comments := filterByType(tokens, TokenComment)

	if len(comments) < 2 {
		t.Errorf("expected at least 2 comment tokens, got %d", len(comments))
	}
}

func TestSemanticTokens_LoadStatement(t *testing.T) {
	content := `load("//pkg:defs.bzl", "my_rule", alias = "other_rule")`

	tokens := tokenizeContent(content)

	// "load" should be a keyword
	keywords := filterByType(tokens, TokenKeyword)
	hasLoad := false
	for _, k := range keywords {
		if k.Length == 4 && k.StartChar == 0 { // "load"
			hasLoad = true
			break
		}
	}
	if !hasLoad {
		t.Error("expected 'load' keyword token")
	}

	// The module path should be a string
	strings := filterByType(tokens, TokenString)
	if len(strings) < 1 {
		t.Error("expected string token for module path")
	}
}

func TestSemanticTokens_BazelLabels(t *testing.T) {
	content := `deps = [
    "//pkg:target",
    ":local_target",
    "@repo//pkg:target",
]`

	tokens := tokenizeContent(content)

	// Labels should be TokenLabel
	labels := filterByType(tokens, TokenLabel)

	if len(labels) < 3 {
		t.Errorf("expected at least 3 label tokens, got %d", len(labels))
	}
}

func TestSemanticTokens_NativeModule(t *testing.T) {
	content := `native.cc_library(
    name = "foo",
)`

	tokens := tokenizeContent(content)

	// "native" should be namespace with defaultLibrary
	namespaces := filterByTypeAndMod(tokens, TokenNamespace, ModDefaultLibrary)

	if len(namespaces) < 1 {
		t.Error("expected 'native' as namespace with defaultLibrary modifier")
	}
}

func TestSemanticTokens_Constants(t *testing.T) {
	content := `x = True
y = False
z = None`

	tokens := tokenizeContent(content)

	// Constants should be variable with readonly and defaultLibrary
	consts := filterByTypeAndMod(tokens, TokenVariable, ModReadonly|ModDefaultLibrary)

	if len(consts) < 3 {
		t.Errorf("expected at least 3 constant tokens, got %d", len(consts))
	}
}

// =============================================================================
// Scope Tests
// =============================================================================

func TestScope_Basic(t *testing.T) {
	scope := NewScope(nil)
	scope.Define("x", SymbolLocal)

	kind, ok := scope.Lookup("x")
	if !ok {
		t.Fatal("expected to find 'x' in scope")
	}
	if kind != SymbolLocal {
		t.Errorf("expected SymbolLocal, got %d", kind)
	}
}

func TestScope_Nested(t *testing.T) {
	parent := NewScope(nil)
	parent.Define("global_var", SymbolGlobal)

	child := NewScope(parent)
	child.Define("local_var", SymbolLocal)

	// Child should see both
	if _, ok := child.Lookup("local_var"); !ok {
		t.Error("child should see local_var")
	}
	if _, ok := child.Lookup("global_var"); !ok {
		t.Error("child should see global_var from parent")
	}

	// Parent should not see child's var
	if _, ok := parent.Lookup("local_var"); ok {
		t.Error("parent should not see child's local_var")
	}
}

func TestScope_Shadowing(t *testing.T) {
	parent := NewScope(nil)
	parent.Define("x", SymbolGlobal)

	child := NewScope(parent)
	child.Define("x", SymbolLocal)

	// Child's definition should shadow parent's
	kind, _ := child.Lookup("x")
	if kind != SymbolLocal {
		t.Error("child's 'x' should shadow parent's")
	}
}

// =============================================================================
// LSP Integration Tests
// =============================================================================

func TestSemanticTokens_InitializeCapability(t *testing.T) {
	server := NewServer(nil)
	initResult := initializeServerWithResult(t, server, "file:///test")

	// Check that SemanticTokensProvider is set
	capabilities, ok := initResult["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities map")
	}
	if capabilities["semanticTokensProvider"] == nil {
		t.Fatal("SemanticTokensProvider should be advertised")
	}
}

func TestSemanticTokens_FullRequest(t *testing.T) {
	server := NewServer(nil)
	_ = initializeServerWithResult(t, server, "file:///test")

	// Open a document
	content := `def hello():
    print("world")`
	openDocumentForSemantic(t, server, "file:///test.star", content)

	// Request semantic tokens
	tokens := requestSemanticTokensFull(t, server, "file:///test.star")

	// Should have tokens
	if len(tokens.Data) == 0 {
		t.Error("expected semantic tokens data")
	}

	// Data length should be multiple of 5
	if len(tokens.Data)%5 != 0 {
		t.Errorf("semantic tokens data length should be multiple of 5, got %d", len(tokens.Data))
	}
}

// initializeServerWithResult initializes a server and returns the capabilities as a map.
// The server returns a map to support LSP fields not present in protocol v0.12.0.
func initializeServerWithResult(t *testing.T, server *Server, rootURI string) map[string]interface{} {
	t.Helper()

	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI(rootURI),
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	initResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	_, err = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})
	if err != nil {
		t.Fatalf("initialized failed: %v", err)
	}

	return initResult
}

// openDocumentForSemantic opens a document for semantic token tests.
func openDocumentForSemantic(t *testing.T, server *Server, uri, content string) {
	t.Helper()

	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(uri),
			LanguageID: "starlark",
			Version:    1,
			Text:       content,
		},
	})
	_, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})
	if err != nil {
		t.Fatalf("didOpen failed: %v", err)
	}
}

// requestSemanticTokensFull requests semantic tokens for a document.
func requestSemanticTokensFull(t *testing.T, server *Server, uri string) *protocol.SemanticTokens {
	t.Helper()

	params, _ := json.Marshal(protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(uri),
		},
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/semanticTokens/full",
		ID:     rawID(2),
		Params: params,
	})
	if err != nil {
		t.Fatalf("semanticTokens/full failed: %v", err)
	}

	tokens, ok := result.(*protocol.SemanticTokens)
	if !ok {
		t.Fatalf("expected SemanticTokens, got %T", result)
	}

	return tokens
}

// =============================================================================
// Helper Functions
// =============================================================================

func filterByType(tokens []SemanticToken, typ uint32) []SemanticToken {
	var result []SemanticToken
	for _, t := range tokens {
		if t.Type == typ {
			result = append(result, t)
		}
	}
	return result
}

func filterByTypeAndMod(tokens []SemanticToken, typ uint32, mod uint32) []SemanticToken {
	var result []SemanticToken
	for _, t := range tokens {
		if t.Type == typ && (t.Modifiers&mod) == mod {
			result = append(result, t)
		}
	}
	return result
}
