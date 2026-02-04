package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/protocol"
)

// These tests document expected LSP completion behavior.
// Currently FAILING because completion is not implemented (returns empty list).

func TestCompletion_BuiltinFunctions(t *testing.T) {
	s := NewServer(nil)

	// Add a document to the server
	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "pri", // user typing "pri" expecting "print" completion
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list, ok := result.(*protocol.CompletionList)
	if !ok {
		t.Fatalf("handleCompletion() returned %T, want *protocol.CompletionList", result)
	}

	// Should return completions for builtin functions starting with "pri"
	if len(list.Items) == 0 {
		t.Error("handleCompletion() returned 0 items, want completions for builtins like 'print'")
	}

	// Check for "print" in completions
	found := false
	for _, item := range list.Items {
		if item.Label == "print" {
			found = true
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'print' builtin completion")
	}
}

func TestCompletion_BuiltinModules(t *testing.T) {
	s := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "js", // user typing "js" expecting "json" module
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Should suggest "json" module
	found := false
	for _, item := range list.Items {
		if item.Label == "json" {
			found = true
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'json' module completion")
	}
}

func TestCompletion_ModuleMembers(t *testing.T) {
	s := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "json.enc", // user typing "json.enc" expecting "json.encode"
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Should suggest "encode" after "json."
	found := false
	for _, item := range list.Items {
		if item.Label == "encode" {
			found = true
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'encode' for json module member")
	}
}

func TestCompletion_LocalVariables(t *testing.T) {
	s := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "my_variable = 42\nmy_", // user typing "my_" expecting "my_variable"
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 1, Character: 3},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Should suggest "my_variable" from local scope
	found := false
	for _, item := range list.Items {
		if item.Label == "my_variable" {
			found = true
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'my_variable' local variable completion")
	}
}

func TestCompletion_FunctionParameters(t *testing.T) {
	s := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "def foo(bar, baz):\n    ba", // user typing "ba" inside function
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 1, Character: 6},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Should suggest "bar" and "baz" parameters
	foundBar := false
	foundBaz := false
	for _, item := range list.Items {
		if item.Label == "bar" {
			foundBar = true
		}
		if item.Label == "baz" {
			foundBaz = true
		}
	}
	if !foundBar {
		t.Error("handleCompletion() did not return 'bar' parameter completion")
	}
	if !foundBaz {
		t.Error("handleCompletion() did not return 'baz' parameter completion")
	}
}

func TestCompletion_Keywords(t *testing.T) {
	s := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "de", // user typing "de" expecting "def"
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Should suggest "def" keyword
	found := false
	for _, item := range list.Items {
		if item.Label == "def" {
			found = true
			break
		}
	}
	if !found {
		t.Error("handleCompletion() did not return 'def' keyword completion")
	}
}

func TestCompletion_EmptyFile(t *testing.T) {
	s := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	s.mu.Lock()
	s.initialized = true
	s.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: "", // empty file
	}
	s.mu.Unlock()

	params := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := s.handleCompletion(context.Background(), paramsJSON)
	if err != nil {
		t.Fatalf("handleCompletion() error = %v", err)
	}

	list := result.(*protocol.CompletionList)

	// Empty file should still get builtins and keywords
	if len(list.Items) == 0 {
		t.Error("handleCompletion() returned 0 items for empty file, want builtins/keywords")
	}
}
