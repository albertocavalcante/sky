package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/protocol"
)

func TestRename_Variable(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Code with a local variable used multiple times
	code := `x = 10
y = x + 5
z = x * 2
`
	openDocument(t, server, "file:///test.star", code)

	// Rename 'x' to 'value' at line 0, char 0
	renameParams, _ := json.Marshal(protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 0},
		},
		NewName: "value",
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/rename",
		ID:     rawID(1),
		Params: renameParams,
	})

	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	workspaceEdit, ok := result.(*protocol.WorkspaceEdit)
	if !ok {
		t.Fatalf("result is not *WorkspaceEdit: %T", result)
	}

	edits := workspaceEdit.Changes["file:///test.star"]
	if len(edits) != 3 {
		t.Errorf("expected 3 edits, got %d", len(edits))
		for i, e := range edits {
			t.Logf("edit %d: line %d, char %d-%d, text=%q", i, e.Range.Start.Line, e.Range.Start.Character, e.Range.End.Character, e.NewText)
		}
	}

	// All edits should change to 'value'
	for _, e := range edits {
		if e.NewText != "value" {
			t.Errorf("expected NewText=%q, got %q", "value", e.NewText)
		}
	}
}

func TestRename_Function(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Code with a function definition and multiple calls
	code := `def my_func(arg1, arg2):
    result = arg1 + arg2
    return result

x = my_func(1, 2)
y = my_func(3, 4)
`
	openDocument(t, server, "file:///test.star", code)

	// Rename 'my_func' to 'add_numbers' at function definition line 0
	renameParams, _ := json.Marshal(protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 5}, // on 'my_func'
		},
		NewName: "add_numbers",
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/rename",
		ID:     rawID(1),
		Params: renameParams,
	})

	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	workspaceEdit, ok := result.(*protocol.WorkspaceEdit)
	if !ok {
		t.Fatalf("result is not *WorkspaceEdit: %T", result)
	}

	edits := workspaceEdit.Changes["file:///test.star"]
	// Should rename: def line + 2 call sites = 3 edits
	if len(edits) != 3 {
		t.Errorf("expected 3 edits (1 def + 2 calls), got %d", len(edits))
		for i, e := range edits {
			t.Logf("edit %d: line %d, char %d-%d, text=%q", i, e.Range.Start.Line, e.Range.Start.Character, e.Range.End.Character, e.NewText)
		}
	}

	// All edits should change to 'add_numbers'
	for _, e := range edits {
		if e.NewText != "add_numbers" {
			t.Errorf("expected NewText=%q, got %q", "add_numbers", e.NewText)
		}
	}
}

func TestRename_Parameter(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Code with a function parameter used in the body
	code := `def greet(name):
    message = "Hello, " + name
    return message
`
	openDocument(t, server, "file:///test.star", code)

	// Rename 'name' parameter to 'person' at line 0
	renameParams, _ := json.Marshal(protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 11}, // on 'name' parameter
		},
		NewName: "person",
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/rename",
		ID:     rawID(1),
		Params: renameParams,
	})

	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	workspaceEdit, ok := result.(*protocol.WorkspaceEdit)
	if !ok {
		t.Fatalf("result is not *WorkspaceEdit: %T", result)
	}

	edits := workspaceEdit.Changes["file:///test.star"]
	// Should rename: parameter def + 1 usage in body = 2 edits
	if len(edits) != 2 {
		t.Errorf("expected 2 edits (param + usage), got %d", len(edits))
		for i, e := range edits {
			t.Logf("edit %d: line %d, char %d-%d, text=%q", i, e.Range.Start.Line, e.Range.Start.Character, e.Range.End.Character, e.NewText)
		}
	}

	// All edits should change to 'person'
	for _, e := range edits {
		if e.NewText != "person" {
			t.Errorf("expected NewText=%q, got %q", "person", e.NewText)
		}
	}
}

func TestPrepareRename_Valid(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	code := `my_variable = 42
`
	openDocument(t, server, "file:///test.star", code)

	// Prepare rename on valid identifier
	prepareParams, _ := json.Marshal(protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 5}, // on 'my_variable'
		},
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/prepareRename",
		ID:     rawID(1),
		Params: prepareParams,
	})

	if err != nil {
		t.Fatalf("prepareRename failed: %v", err)
	}

	// Should return a Range indicating the symbol can be renamed
	rangeResult, ok := result.(*protocol.Range)
	if !ok {
		t.Fatalf("result is not *Range: %T", result)
	}

	// The range should cover 'my_variable' (chars 0-11)
	if rangeResult.Start.Line != 0 {
		t.Errorf("expected start line 0, got %d", rangeResult.Start.Line)
	}
	if rangeResult.Start.Character != 0 {
		t.Errorf("expected start char 0, got %d", rangeResult.Start.Character)
	}
	if rangeResult.End.Character != 11 {
		t.Errorf("expected end char 11, got %d", rangeResult.End.Character)
	}
}

func TestPrepareRename_Invalid_Keyword(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	code := `def my_func():
    return None
`
	openDocument(t, server, "file:///test.star", code)

	// Prepare rename on 'def' keyword - should return nil
	prepareParams, _ := json.Marshal(protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 1}, // on 'def'
		},
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/prepareRename",
		ID:     rawID(1),
		Params: prepareParams,
	})

	if err != nil {
		t.Fatalf("prepareRename failed: %v", err)
	}

	// Should return nil for keywords (can't rename)
	if result != nil {
		t.Errorf("expected nil for keyword, got %T", result)
	}
}

func TestPrepareRename_Invalid_Builtin(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	code := `x = len([1, 2, 3])
`
	openDocument(t, server, "file:///test.star", code)

	// Prepare rename on 'len' builtin - should return nil
	prepareParams, _ := json.Marshal(protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 5}, // on 'len'
		},
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/prepareRename",
		ID:     rawID(1),
		Params: prepareParams,
	})

	if err != nil {
		t.Fatalf("prepareRename failed: %v", err)
	}

	// Should return nil for builtins (can't rename)
	if result != nil {
		t.Errorf("expected nil for builtin, got %T", result)
	}
}

func TestRename_InitializeCapability(t *testing.T) {
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

	capabilities, ok := initResult["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities map")
	}

	// Check that RenameProvider is advertised
	renameProvider := capabilities["renameProvider"]
	if renameProvider == nil {
		t.Fatal("RenameProvider capability should be set")
	}

	// It should be a *protocol.RenameOptions with PrepareProvider=true
	// Marshal and unmarshal to check
	renameBytes, _ := json.Marshal(renameProvider)
	var renameOpts protocol.RenameOptions
	if err := json.Unmarshal(renameBytes, &renameOpts); err != nil {
		t.Fatalf("failed to unmarshal RenameOptions: %v", err)
	}

	if !renameOpts.PrepareProvider {
		t.Error("RenameOptions.PrepareProvider should be true")
	}
}

func TestRename_NoDocument(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Rename on non-existent document
	renameParams, _ := json.Marshal(protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///nonexistent.star",
			},
			Position: protocol.Position{Line: 0, Character: 0},
		},
		NewName: "newname",
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/rename",
		ID:     rawID(1),
		Params: renameParams,
	})

	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	// Should return nil for non-existent document
	if result != nil {
		t.Errorf("expected nil for non-existent document, got %T", result)
	}
}

func TestRename_EmptyPosition(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	code := `x = 10
`
	openDocument(t, server, "file:///test.star", code)

	// Rename at a position with no word (e.g., on '=')
	renameParams, _ := json.Marshal(protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: "file:///test.star",
			},
			Position: protocol.Position{Line: 0, Character: 2}, // on '='
		},
		NewName: "newname",
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/rename",
		ID:     rawID(1),
		Params: renameParams,
	})

	if err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	// Should return nil when cursor is not on a word
	if result != nil {
		t.Errorf("expected nil for empty position, got %T", result)
	}
}

// Helper functions

func initializeServer(t *testing.T, server *Server) {
	t.Helper()

	initParams, _ := json.Marshal(protocol.InitializeParams{})
	_, err := server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	_, err = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})
	if err != nil {
		t.Fatalf("initialized failed: %v", err)
	}
}

func openDocument(t *testing.T, server *Server, uri, content string) {
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
