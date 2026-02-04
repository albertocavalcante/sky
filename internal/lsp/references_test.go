package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/protocol"
)

// TestReferences_LocalVariable tests finding references to a local variable.
func TestReferences_LocalVariable(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	// Code with a variable used multiple times
	code := `def main():
    result = 42
    print(result)
    return result
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references for "result" at line 1 (the assignment)
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 1, Character: 4}, // "result" in "result = 42"
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	// Should find 3 references: assignment, print(result), return result
	if len(locations) != 3 {
		t.Errorf("expected 3 references, got %d", len(locations))
		for i, loc := range locations {
			t.Logf("  ref %d: line %d, char %d-%d", i, loc.Range.Start.Line, loc.Range.Start.Character, loc.Range.End.Character)
		}
	}

	// Verify all locations are in the same file
	for _, loc := range locations {
		if loc.URI != uri {
			t.Errorf("reference URI = %v, want %v", loc.URI, uri)
		}
	}
}

// TestReferences_FunctionCalls tests finding references to a function definition.
func TestReferences_FunctionCalls(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	// Code with a function defined and called multiple times
	code := `def helper(x):
    return x + 1

def main():
    a = helper(1)
    b = helper(2)
    return a + b
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references for "helper" at line 0 (the function definition)
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 4}, // "helper" in "def helper(x):"
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	// Should find 3 references: def helper, helper(1), helper(2)
	if len(locations) != 3 {
		t.Errorf("expected 3 references, got %d", len(locations))
		for i, loc := range locations {
			t.Logf("  ref %d: line %d, char %d-%d", i, loc.Range.Start.Line, loc.Range.Start.Character, loc.Range.End.Character)
		}
	}
}

// TestReferences_Parameter tests finding references to a function parameter.
func TestReferences_Parameter(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	// Code with a parameter used in the function body
	code := `def process(data):
    print(data)
    result = data + data
    return result
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references for "data" at line 0 (the parameter)
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 12}, // "data" in "def process(data):"
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	// Should find 4 references: parameter def, print(data), data + data (2x)
	if len(locations) != 4 {
		t.Errorf("expected 4 references, got %d", len(locations))
		for i, loc := range locations {
			t.Logf("  ref %d: line %d, char %d-%d", i, loc.Range.Start.Line, loc.Range.Start.Character, loc.Range.End.Character)
		}
	}
}

// TestReferences_NoReferences tests a symbol with no other references.
func TestReferences_NoReferences(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	// Code with a variable that's only defined but never used
	code := `def main():
    unused = 42
    return 0
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references for "unused" at line 1
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 1, Character: 4}, // "unused" in "unused = 42"
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	// Should find exactly 1 reference (the declaration itself, since IncludeDeclaration is true)
	if len(locations) != 1 {
		t.Errorf("expected 1 reference (declaration only), got %d", len(locations))
	}
}

// TestReferences_ExcludeDeclaration tests that IncludeDeclaration=false excludes the definition.
func TestReferences_ExcludeDeclaration(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	code := `def helper():
    return 1

def main():
    return helper()
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references for "helper" with IncludeDeclaration=false
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 4}, // "helper" in "def helper():"
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: false, // Don't include the definition
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	// Should find 1 reference (only the call, not the definition)
	if len(locations) != 1 {
		t.Errorf("expected 1 reference (call only), got %d", len(locations))
	}

	// The reference should be on line 4 (the call site)
	if len(locations) > 0 && locations[0].Range.Start.Line != 4 {
		t.Errorf("expected reference at line 4, got line %d", locations[0].Range.Start.Line)
	}
}

// TestReferences_InitializeCapability tests that ReferencesProvider capability is advertised.
func TestReferences_InitializeCapability(t *testing.T) {
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

	// ReferencesProvider should be true
	refsProvider, ok := initResult.Capabilities.ReferencesProvider.(bool)
	if !ok {
		t.Fatalf("ReferencesProvider is not bool: %T", initResult.Capabilities.ReferencesProvider)
	}

	if !refsProvider {
		t.Error("ReferencesProvider should be true")
	}
}

// TestReferences_UnknownSymbol tests behavior when cursor is not on a symbol.
func TestReferences_UnknownSymbol(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	code := `def main():
    return 42
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references at a position with no identifier (whitespace)
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 0}, // at "def" keyword
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	// Should return nil or empty array for non-identifiers
	if result != nil {
		locations, ok := result.([]protocol.Location)
		if ok && len(locations) > 0 {
			t.Errorf("expected nil or empty result for keyword, got %d locations", len(locations))
		}
	}
}

// TestReferences_MultipleAssignments tests references across multiple assignments.
func TestReferences_MultipleAssignments(t *testing.T) {
	server := NewServer(nil)

	uri := protocol.DocumentURI("file:///test.star")
	// Code where a variable is reassigned
	code := `def main():
    x = 1
    print(x)
    x = 2
    print(x)
    return x
`
	server.mu.Lock()
	server.initialized = true
	server.documents[uri] = &Document{
		URI:     uri,
		Version: 1,
		Content: code,
	}
	server.mu.Unlock()

	// Request references for "x"
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 1, Character: 4}, // first "x"
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := server.Handle(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      rawID(1),
		Method:  "textDocument/references",
		Params:  paramsJSON,
	})

	if err != nil {
		t.Fatalf("references failed: %v", err)
	}

	locations, ok := result.([]protocol.Location)
	if !ok {
		t.Fatalf("result is not []Location: %T", result)
	}

	// Should find all 6 references: x=1, print(x), x=2, print(x), return x (5 actual uses + 1 reassignment)
	// Note: We should find 6 references: line 1 (x=1), line 2 (print x), line 3 (x=2), line 4 (print x), line 5 (return x)
	if len(locations) < 5 {
		t.Errorf("expected at least 5 references, got %d", len(locations))
		for i, loc := range locations {
			t.Logf("  ref %d: line %d, char %d-%d", i, loc.Range.Start.Line, loc.Range.Start.Character, loc.Range.End.Character)
		}
	}
}
