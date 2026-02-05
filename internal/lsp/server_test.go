package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/checker"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
	"go.lsp.dev/protocol"
	"go.starlark.net/syntax"
)

func TestServerInitialize(t *testing.T) {
	server := NewServer(nil)

	params, _ := json.Marshal(protocol.InitializeParams{
		ProcessID: 1234,
		RootURI:   "file:///test",
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

	serverInfo, ok := initResult["serverInfo"].(map[string]string)
	if !ok {
		t.Fatal("expected serverInfo map")
	}

	if serverInfo["name"] != "skyls" {
		t.Errorf("ServerInfo.Name = %q, want %q", serverInfo["name"], "skyls")
	}

	capabilities, ok := initResult["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities map")
	}

	if capabilities["hoverProvider"] != true {
		t.Error("HoverProvider should be true")
	}
}

func TestServerNotInitialized(t *testing.T) {
	server := NewServer(nil)

	// Try to call a method before initialization
	params, _ := json.Marshal(protocol.HoverParams{})
	_, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/hover",
		Params:  params,
	})

	if err == nil {
		t.Fatal("expected error for uninitialized server")
	}

	rpcErr, ok := err.(*ResponseError)
	if !ok {
		t.Fatalf("expected ResponseError, got %T", err)
	}

	if rpcErr.Code != CodeInvalidRequest {
		t.Errorf("Code = %d, want %d", rpcErr.Code, CodeInvalidRequest)
	}
}

func TestServerLifecycle(t *testing.T) {
	exitCalled := false
	server := NewServer(func() { exitCalled = true })

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, err := server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Initialized notification (no ID)
	_, err = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})
	if err != nil {
		t.Fatalf("initialized failed: %v", err)
	}

	// Shutdown
	_, err = server.Handle(context.Background(), &Request{
		Method: "shutdown",
		ID:     rawID(2),
	})
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// After shutdown, only exit is allowed
	_, err = server.Handle(context.Background(), &Request{
		Method: "textDocument/hover",
		ID:     rawID(3),
		Params: json.RawMessage("{}"),
	})
	if err == nil {
		t.Error("expected error after shutdown")
	}

	// Exit
	_, err = server.Handle(context.Background(), &Request{
		Method: "exit",
	})
	if err != nil {
		t.Fatalf("exit failed: %v", err)
	}

	if !exitCalled {
		t.Error("exit callback was not called")
	}
}

func TestServerDocumentSync(t *testing.T) {
	server := NewServer(nil)

	// Initialize first
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Open document
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.star",
			LanguageID: "starlark",
			Version:    1,
			Text:       "def hello():\n    pass\n",
		},
	})
	_, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})
	if err != nil {
		t.Fatalf("didOpen failed: %v", err)
	}

	// Verify document is tracked
	server.mu.RLock()
	doc, ok := server.documents["file:///test.star"]
	server.mu.RUnlock()

	if !ok {
		t.Fatal("document not found after didOpen")
	}
	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1", doc.Version)
	}

	// Close document
	closeParams, _ := json.Marshal(protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///test.star",
		},
	})
	_, err = server.Handle(context.Background(), &Request{
		Method: "textDocument/didClose",
		Params: closeParams,
	})
	if err != nil {
		t.Fatalf("didClose failed: %v", err)
	}

	// Verify document is removed
	server.mu.RLock()
	_, ok = server.documents["file:///test.star"]
	server.mu.RUnlock()

	if ok {
		t.Error("document should be removed after didClose")
	}
}

func TestServerFormatting(t *testing.T) {
	server := NewServer(nil)

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Open document with badly formatted code
	unformatted := "def   foo(  x,y ):\n  return x+y\n"
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.star",
			LanguageID: "starlark",
			Version:    1,
			Text:       unformatted,
		},
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})

	// Request formatting
	fmtParams, _ := json.Marshal(protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///test.star",
		},
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/formatting",
		ID:     rawID(2),
		Params: fmtParams,
	})

	if err != nil {
		t.Fatalf("formatting failed: %v", err)
	}

	edits, ok := result.([]protocol.TextEdit)
	if !ok {
		t.Fatalf("result is not []TextEdit: %T", result)
	}

	// Should have exactly one edit (whole document replacement)
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}

	// The edit should produce formatted code
	formatted := edits[0].NewText
	if formatted == unformatted {
		t.Error("formatted text should differ from original")
	}

	// Check that the formatted code is cleaner
	if !containsSubstring(formatted, "def foo(x, y):") {
		t.Errorf("formatted code doesn't look right: %q", formatted)
	}
}

func TestServerFormattingNoChange(t *testing.T) {
	server := NewServer(nil)

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Open document with already formatted code
	formatted := "def foo(x, y):\n    return x + y\n"
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.star",
			LanguageID: "starlark",
			Version:    1,
			Text:       formatted,
		},
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})

	// Request formatting
	fmtParams, _ := json.Marshal(protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///test.star",
		},
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/formatting",
		ID:     rawID(2),
		Params: fmtParams,
	})

	if err != nil {
		t.Fatalf("formatting failed: %v", err)
	}

	edits, ok := result.([]protocol.TextEdit)
	if !ok {
		t.Fatalf("result is not []TextEdit: %T", result)
	}

	// Should have no edits since code is already formatted
	if len(edits) != 0 {
		t.Errorf("expected 0 edits for already formatted code, got %d", len(edits))
	}
}

func TestServerDefinition(t *testing.T) {
	server := NewServer(nil)

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Open document with function definitions and usage
	code := `VERSION = "1.0"

def helper():
    return "help"

def main():
    x = helper()
    return x
`
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.star",
			LanguageID: "starlark",
			Version:    1,
			Text:       code,
		},
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})

	// Go to definition of "helper" on line 6 (0-indexed)
	// Line 6 is: "    x = helper()"
	defParams, _ := json.Marshal(protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 6, Character: 10}, // "helper" in "    x = helper()"
		},
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/definition",
		ID:     rawID(2),
		Params: defParams,
	})

	if err != nil {
		t.Fatalf("definition failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	if len(locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locations))
	}

	// Should point to line 2 (0-indexed) where "def helper():" is defined
	if locations[0].Range.Start.Line != 2 {
		t.Errorf("expected definition at line 2, got %d", locations[0].Range.Start.Line)
	}
}

func TestServerHover(t *testing.T) {
	server := NewServer(nil)

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Open document with a documented function
	code := `"""Module docstring."""

def greet(name, greeting="Hello"):
    """Greet someone with a message.

    Args:
        name: The person's name.
        greeting: The greeting to use.

    Returns:
        A greeting string.
    """
    return greeting + ", " + name
`
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.star",
			LanguageID: "starlark",
			Version:    1,
			Text:       code,
		},
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})

	// Hover over "greet" function name (line 2, character 4)
	hoverParams, _ := json.Marshal(protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 2, Character: 5}, // "greet"
		},
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/hover",
		ID:     rawID(2),
		Params: hoverParams,
	})

	if err != nil {
		t.Fatalf("hover failed: %v", err)
	}

	hover, ok := result.(*protocol.Hover)
	if !ok {
		t.Fatalf("result is not *Hover: %T", result)
	}

	// Check that we got markdown content
	if hover.Contents.Kind != protocol.Markdown {
		t.Errorf("expected Markdown, got %v", hover.Contents.Kind)
	}

	// Check that the content includes the function signature
	if !containsSubstring(hover.Contents.Value, "def greet(") {
		t.Errorf("hover content missing function signature: %s", hover.Contents.Value)
	}

	// Check that docstring summary is included
	if !containsSubstring(hover.Contents.Value, "Greet someone") {
		t.Errorf("hover content missing docstring: %s", hover.Contents.Value)
	}
}

func TestServerDocumentSymbol(t *testing.T) {
	server := NewServer(nil)

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Open document with functions and variables
	code := `"""Module docstring."""

VERSION = "1.0.0"

def hello(name):
    """Say hello."""
    return "Hello, " + name

def add(a, b):
    return a + b

CONFIG = {"key": "value"}
`
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.star",
			LanguageID: "starlark",
			Version:    1,
			Text:       code,
		},
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})

	// Request document symbols
	symbolParams, _ := json.Marshal(protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: "file:///test.star",
		},
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/documentSymbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("documentSymbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.DocumentSymbol)
	if !ok {
		t.Fatalf("result is not []DocumentSymbol: %T", result)
	}

	// Should have 4 symbols: VERSION, hello, add, CONFIG
	if len(symbols) != 4 {
		t.Errorf("expected 4 symbols, got %d", len(symbols))
		for _, s := range symbols {
			t.Logf("  symbol: %s (%v)", s.Name, s.Kind)
		}
	}

	// Check we have the expected symbols
	names := make(map[string]protocol.SymbolKind)
	for _, s := range symbols {
		names[s.Name] = s.Kind
	}

	if kind, ok := names["hello"]; !ok || kind != protocol.SymbolKindFunction {
		t.Errorf("expected hello as Function, got %v", kind)
	}
	if kind, ok := names["add"]; !ok || kind != protocol.SymbolKindFunction {
		t.Errorf("expected add as Function, got %v", kind)
	}
	if kind, ok := names["VERSION"]; !ok || kind != protocol.SymbolKindVariable {
		t.Errorf("expected VERSION as Variable, got %v", kind)
	}
	if kind, ok := names["CONFIG"]; !ok || kind != protocol.SymbolKindVariable {
		t.Errorf("expected CONFIG as Variable, got %v", kind)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func rawID(n int) *json.RawMessage {
	raw := json.RawMessage([]byte{byte('0' + n)})
	return &raw
}

func TestLintFindingToDiagnostic(t *testing.T) {
	tests := []struct {
		name     string
		finding  linter.Finding
		wantLine uint32
		wantChar uint32
		wantSev  protocol.DiagnosticSeverity
	}{
		{
			name: "error at line 5 col 10",
			finding: linter.Finding{
				Line:     5,
				Column:   10,
				Severity: linter.SeverityError,
				Message:  "test error",
				Rule:     "test-rule",
			},
			wantLine: 4, // 0-based
			wantChar: 9, // 0-based
			wantSev:  protocol.DiagnosticSeverityError,
		},
		{
			name: "warning at line 1 col 1",
			finding: linter.Finding{
				Line:     1,
				Column:   1,
				Severity: linter.SeverityWarning,
				Message:  "test warning",
				Rule:     "warn-rule",
			},
			wantLine: 0,
			wantChar: 0,
			wantSev:  protocol.DiagnosticSeverityWarning,
		},
		{
			name: "info severity",
			finding: linter.Finding{
				Line:     10,
				Column:   5,
				Severity: linter.SeverityInfo,
				Message:  "info",
				Rule:     "info-rule",
			},
			wantLine: 9,
			wantChar: 4,
			wantSev:  protocol.DiagnosticSeverityInformation,
		},
		{
			name: "hint severity",
			finding: linter.Finding{
				Line:     2,
				Column:   3,
				Severity: linter.SeverityHint,
				Message:  "hint",
				Rule:     "hint-rule",
			},
			wantLine: 1,
			wantChar: 2,
			wantSev:  protocol.DiagnosticSeverityHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := lintFindingToDiagnostic(tt.finding)

			if diag.Range.Start.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", diag.Range.Start.Line, tt.wantLine)
			}
			if diag.Range.Start.Character != tt.wantChar {
				t.Errorf("Character = %d, want %d", diag.Range.Start.Character, tt.wantChar)
			}
			if diag.Severity != tt.wantSev {
				t.Errorf("Severity = %v, want %v", diag.Severity, tt.wantSev)
			}
			if diag.Source != "skylint" {
				t.Errorf("Source = %q, want %q", diag.Source, "skylint")
			}
			if diag.Message != tt.finding.Message {
				t.Errorf("Message = %q, want %q", diag.Message, tt.finding.Message)
			}
		})
	}
}

func TestCheckerDiagnosticToLSP(t *testing.T) {
	tests := []struct {
		name     string
		diag     checker.Diagnostic
		wantLine uint32
		wantChar uint32
		wantSev  protocol.DiagnosticSeverity
	}{
		{
			name: "error at line 3 col 5",
			diag: checker.Diagnostic{
				Pos:      posAt(3, 5),
				Severity: checker.SeverityError,
				Code:     "undefined",
				Message:  "undefined: foo",
			},
			wantLine: 2,
			wantChar: 4,
			wantSev:  protocol.DiagnosticSeverityError,
		},
		{
			name: "warning",
			diag: checker.Diagnostic{
				Pos:      posAt(1, 1),
				Severity: checker.SeverityWarning,
				Code:     "unused",
				Message:  "unused variable",
			},
			wantLine: 0,
			wantChar: 0,
			wantSev:  protocol.DiagnosticSeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lspDiag := checkerDiagnosticToLSP(tt.diag)

			if lspDiag.Range.Start.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", lspDiag.Range.Start.Line, tt.wantLine)
			}
			if lspDiag.Range.Start.Character != tt.wantChar {
				t.Errorf("Character = %d, want %d", lspDiag.Range.Start.Character, tt.wantChar)
			}
			if lspDiag.Severity != tt.wantSev {
				t.Errorf("Severity = %v, want %v", lspDiag.Severity, tt.wantSev)
			}
			if lspDiag.Source != "skycheck" {
				t.Errorf("Source = %q, want %q", lspDiag.Source, "skycheck")
			}
		})
	}
}

// posAt creates a syntax.Position for testing.
// Note: syntax.Position uses 1-based line and column.
func posAt(line, col int) syntax.Position {
	return syntax.MakePosition(nil, int32(line), int32(col))
}
