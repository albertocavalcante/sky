package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/protocol"
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

	initResult, ok := result.(*protocol.InitializeResult)
	if !ok {
		t.Fatalf("result is not InitializeResult: %T", result)
	}

	if initResult.ServerInfo.Name != "skyls" {
		t.Errorf("ServerInfo.Name = %q, want %q", initResult.ServerInfo.Name, "skyls")
	}

	if !initResult.Capabilities.HoverProvider.(bool) {
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
	server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	server.Handle(context.Background(), &Request{
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
	server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	server.Handle(context.Background(), &Request{
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
	server.Handle(context.Background(), &Request{
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
	server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	server.Handle(context.Background(), &Request{
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
	server.Handle(context.Background(), &Request{
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
