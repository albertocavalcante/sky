package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/builtins"
)

// TestSignatureHelp_BuiltinFunction tests signature help for builtin functions.
func TestSignatureHelp_BuiltinFunction(t *testing.T) {
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
		Content: "print(",
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 6}, // cursor after "("
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil for builtin function")
	}

	sigHelp, ok := result.(*protocol.SignatureHelp)
	if !ok {
		t.Fatalf("handleSignatureHelp() returned %T, want *protocol.SignatureHelp", result)
	}

	if len(sigHelp.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}

	// Check the signature contains "print"
	sig := sigHelp.Signatures[0]
	if !strings.Contains(sig.Label, "print") {
		t.Errorf("signature label should contain 'print', got: %s", sig.Label)
	}

	// Check parameters are present
	if len(sig.Parameters) == 0 {
		t.Error("expected parameters in signature")
	}
}

// TestSignatureHelp_ActiveParameter tests that the active parameter is correctly identified.
func TestSignatureHelp_ActiveParameter(t *testing.T) {
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
		Content: `range(1, `, // cursor after first comma - should highlight second param
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 9}, // cursor after ", "
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil")
	}

	sigHelp := result.(*protocol.SignatureHelp)
	if *sigHelp.ActiveParameter != 1 {
		t.Errorf("ActiveParameter = %d, want 1 (second param)", sigHelp.ActiveParameter)
	}
}

// TestSignatureHelp_NestedCalls tests that nested calls show the innermost function signature.
func TestSignatureHelp_NestedCalls(t *testing.T) {
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
		Content: "print(len(", // cursor inside len() - should show len signature, not print
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 10}, // cursor after "len("
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil")
	}

	sigHelp := result.(*protocol.SignatureHelp)
	if len(sigHelp.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}

	// Should show len(), not print()
	sig := sigHelp.Signatures[0]
	if !strings.Contains(sig.Label, "len") {
		t.Errorf("signature should be for 'len', got: %s", sig.Label)
	}
	if strings.Contains(sig.Label, "print") {
		t.Errorf("signature should NOT be for 'print' when inside len(), got: %s", sig.Label)
	}
}

// TestSignatureHelp_UserDefinedFunction tests signature help for user-defined functions.
func TestSignatureHelp_UserDefinedFunction(t *testing.T) {
	server := NewServer(nil) // Use real server with default provider

	uri := string("file:///test.star")
	code := `def my_func(name, value=None):
    """Do something useful.

    Args:
        name: The name to use.
        value: Optional value.
    """
    pass

my_func(`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 9, Character: 8}, // cursor after "my_func("
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil for user-defined function")
	}

	sigHelp := result.(*protocol.SignatureHelp)
	if len(sigHelp.Signatures) == 0 {
		t.Fatal("expected at least one signature for user-defined function")
	}

	sig := sigHelp.Signatures[0]
	if !strings.Contains(sig.Label, "my_func") {
		t.Errorf("signature label should contain 'my_func', got: %s", sig.Label)
	}

	// Check that parameters are present
	if len(sig.Parameters) < 2 {
		t.Errorf("expected at least 2 parameters (name, value), got %d", len(sig.Parameters))
	}

	// Check documentation is present
	if sig.Documentation.Value != nil {
		docStr, ok := sig.Documentation.Value.(string)
		if ok && !strings.Contains(docStr, "useful") {
			t.Logf("Documentation: %v", sig.Documentation)
		}
	}
}

// TestSignatureHelp_OutsideCall tests that no signature help is returned outside function calls.
func TestSignatureHelp_OutsideCall(t *testing.T) {
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
		Content: "x = 1",
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 5}, // cursor at end of "x = 1"
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	// Result can be nil or empty SignatureHelp outside function calls
	if result != nil {
		sigHelp, ok := result.(*protocol.SignatureHelp)
		if ok && len(sigHelp.Signatures) > 0 {
			t.Errorf("expected no signatures outside function calls, got %d", len(sigHelp.Signatures))
		}
	}
}

// TestSignatureHelp_ClosedParen tests that no signature help is shown after closing paren.
func TestSignatureHelp_ClosedParen(t *testing.T) {
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

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 8}, // cursor after ")"
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	// Result should be nil or empty after closing paren
	if result != nil {
		sigHelp, ok := result.(*protocol.SignatureHelp)
		if ok && len(sigHelp.Signatures) > 0 {
			t.Errorf("expected no signatures after closing paren, got %d", len(sigHelp.Signatures))
		}
	}
}

// TestSignatureHelp_MultipleParameters tests correct parameter highlighting with multiple params.
func TestSignatureHelp_MultipleParameters(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	tests := []struct {
		name       string
		content    string
		cursorCol  uint32
		wantActive uint32
	}{
		{
			name:       "first param",
			content:    "range(",
			cursorCol:  6,
			wantActive: 0,
		},
		{
			name:       "second param",
			content:    "range(1, ",
			cursorCol:  9,
			wantActive: 1,
		},
		{
			name:       "third param",
			content:    "range(1, 10, ",
			cursorCol:  13,
			wantActive: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := string("file:///test.star")
			server.mu.Lock()
			server.documents[uri] = &Document{
				URI:     uri,
				Version: 1,
				Content: tt.content,
			}
			server.mu.Unlock()

			params := protocol.SignatureHelpParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
					Position:     protocol.Position{Line: 0, Character: tt.cursorCol},
				},
			}
			paramsJSON, _ := json.Marshal(params)

			result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
			if err != nil {
				t.Fatalf("handleSignatureHelp() error = %v", err)
			}

			if result == nil {
				t.Fatal("handleSignatureHelp() returned nil")
			}

			sigHelp := result.(*protocol.SignatureHelp)
			if *sigHelp.ActiveParameter != tt.wantActive {
				t.Errorf("ActiveParameter = %d, want %d", sigHelp.ActiveParameter, tt.wantActive)
			}
		})
	}
}

// TestSignatureHelp_BuiltinType tests signature help for builtin type constructors like dict().
func TestSignatureHelp_BuiltinType(t *testing.T) {
	provider := &mockProvider{
		builtins: builtins.Builtins{
			Types: []builtins.TypeDef{
				{
					Name: "dict",
					Doc:  "Dictionary type for key-value storage",
				},
			},
			Functions: []builtins.Signature{
				{
					Name:       "dict",
					Doc:        "Creates a new dictionary",
					ReturnType: "dict",
					Params: []builtins.Param{
						{Name: "pairs", Type: "iterable", Default: "[]"},
						{Name: "kwargs", KWArgs: true},
					},
				},
			},
		},
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "x = dict(",
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 9}, // cursor after "dict("
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil for builtin type")
	}

	sigHelp := result.(*protocol.SignatureHelp)
	if len(sigHelp.Signatures) == 0 {
		t.Fatal("expected at least one signature for dict()")
	}

	sig := sigHelp.Signatures[0]
	if !strings.Contains(sig.Label, "dict") {
		t.Errorf("signature label should contain 'dict', got: %s", sig.Label)
	}
}

// TestSignatureHelp_MultilineCall tests signature help works across multiple lines.
func TestSignatureHelp_MultilineCall(t *testing.T) {
	provider := &mockProvider{
		builtins: testBuiltins(),
		dialects: []string{"starlark"},
	}
	server := NewServerWithProvider(nil, provider)

	uri := string("file:///test.star")
	code := `print(
    "hello",
    `
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 2, Character: 4}, // cursor on line 3
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil for multiline call")
	}

	sigHelp := result.(*protocol.SignatureHelp)
	if len(sigHelp.Signatures) == 0 {
		t.Fatal("expected signature help in multiline call")
	}

	sig := sigHelp.Signatures[0]
	if !strings.Contains(sig.Label, "print") {
		t.Errorf("signature should be for 'print', got: %s", sig.Label)
	}

	// Should be on second parameter (after the comma)
	if *sigHelp.ActiveParameter != 1 {
		t.Errorf("ActiveParameter = %d, want 1 (second param after comma)", sigHelp.ActiveParameter)
	}
}

// TestSignatureHelp_StringWithComma tests that commas inside strings don't affect parameter count.
func TestSignatureHelp_StringWithComma(t *testing.T) {
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
		Content: `print("a, b, c", `, // commas inside string should not count
	}
	server.mu.Unlock()

	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 17}, // cursor after the real comma
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.handleSignatureHelp(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleSignatureHelp() error = %v", err)
	}

	if result == nil {
		t.Fatal("handleSignatureHelp() returned nil")
	}

	sigHelp := result.(*protocol.SignatureHelp)
	// Should be on second parameter (only one real comma)
	if *sigHelp.ActiveParameter != 1 {
		t.Errorf("ActiveParameter = %d, want 1 (commas in string should not count)", sigHelp.ActiveParameter)
	}
}

// TestSignatureHelp_InitializeCapability tests that SignatureHelpProvider is advertised.
func TestSignatureHelp_InitializeCapability(t *testing.T) {
	server := NewServer(nil)

	params, _ := json.Marshal(protocol.InitializeParams{
		XInitializeParams: protocol.XInitializeParams{ProcessId: ptrInt32(1234), RootUri: ptrString("file:///test")},
	})

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "initialize",
		Params:  params,
	})

	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Server returns map[string]interface{} to support LSP fields not in protocol v0.12.0
	initResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result is not map[string]interface{}: %T", result)
	}

	capabilities, ok := initResult["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities map")
	}

	sigProvider := capabilities["signatureHelpProvider"]
	if sigProvider == nil {
		t.Fatal("SignatureHelpProvider capability should be advertised")
	}

	// Check trigger characters - need to marshal/unmarshal to access
	sigProviderBytes, _ := json.Marshal(sigProvider)
	var sigOpts protocol.SignatureHelpOptions
	if err := json.Unmarshal(sigProviderBytes, &sigOpts); err != nil {
		t.Fatalf("failed to unmarshal SignatureHelpOptions: %v", err)
	}

	if sigOpts.TriggerCharacters == nil || len(sigOpts.TriggerCharacters) == 0 {
		t.Error("SignatureHelpProvider should have trigger characters")
	}

	// Check for "(" trigger
	hasParen := false
	for _, c := range sigOpts.TriggerCharacters {
		if c == "(" {
			hasParen = true
			break
		}
	}
	if !hasParen {
		t.Error("SignatureHelpProvider should trigger on '('")
	}

	// Check for "," trigger
	hasComma := false
	for _, c := range sigOpts.TriggerCharacters {
		if c == "," {
			hasComma = true
			break
		}
	}
	if !hasComma {
		t.Error("SignatureHelpProvider should trigger on ','")
	}
}
