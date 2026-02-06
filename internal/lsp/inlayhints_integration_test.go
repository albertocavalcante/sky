package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
)

// TestInlayHints_InitializeCapability tests that InlayHintProvider is advertised.
func TestInlayHints_InitializeCapability(t *testing.T) {
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

	// InlayHintProvider should be true
	if capabilities["inlayHintProvider"] != true {
		t.Error("InlayHintProvider should be true")
	}
}

// TestInlayHints_Request tests the full inlay hints request flow.
func TestInlayHints_Request(t *testing.T) {
	server := NewServer(nil)

	// Initialize
	initParams, _ := json.Marshal(protocol.InitializeParams{
		XInitializeParams: protocol.XInitializeParams{ProcessId: ptrInt32(1234), RootUri: ptrString("file:///test")},
	})
	_, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "initialize",
		Params:  initParams,
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Mark initialized
	_, err = server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  json.RawMessage("{}"),
	})
	if err != nil {
		t.Fatalf("initialized failed: %v", err)
	}

	// Open document
	content := `count = 42
name = "hello"
items = [1, 2, 3]
`
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			Uri:        "file:///test.star",
			LanguageId: "starlark",
			Version:    1,
			Text:       content,
		},
	})
	_, err = server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  openParams,
	})
	if err != nil {
		t.Fatalf("didOpen failed: %v", err)
	}

	// Request inlay hints
	hintParams, _ := json.Marshal(protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: "file:///test.star",
		},
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 10, Character: 0},
		},
	})

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(2),
		Method:  "textDocument/inlayHint",
		Params:  hintParams,
	})
	if err != nil {
		t.Fatalf("inlayHint failed: %v", err)
	}

	hints, ok := result.([]protocol.InlayHint)
	if !ok {
		t.Fatalf("expected []protocol.InlayHint, got %T", result)
	}

	// Should have 3 hints for count, name, items
	if len(hints) != 3 {
		t.Fatalf("got %d hints, want 3", len(hints))
	}

	// Verify the hints
	expected := []struct {
		line  uint32
		label string
	}{
		{0, ": int"},
		{1, ": str"},
		{2, ": list[int]"},
	}

	for i, want := range expected {
		if hints[i].Position.Line != want.line {
			t.Errorf("hint[%d] line = %d, want %d", i, hints[i].Position.Line, want.line)
		}
		if hints[i].Label.Value.(string) != want.label {
			t.Errorf("hint[%d] label = %q, want %q", i, hints[i].Label.Value, want.label)
		}
	}
}
