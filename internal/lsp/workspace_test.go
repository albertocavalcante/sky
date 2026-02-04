package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
)

// TestWorkspaceSymbol_FindFunction tests searching for a function by exact name.
func TestWorkspaceSymbol_FindFunction(t *testing.T) {
	// Create temp workspace with test files
	tmpDir := t.TempDir()
	createTestFile(t, tmpDir, "utils.bzl", `
def helper_function(arg1, arg2):
    """A helper function."""
    return arg1 + arg2

def another_func():
    pass
`)
	createTestFile(t, tmpDir, "lib/rules.bzl", `
def my_rule(name, srcs):
    """Custom rule definition."""
    pass
`)

	server := NewServer(nil)

	// Initialize with workspace root
	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// Search for "helper_function"
	symbolParams, _ := json.Marshal(protocol.WorkspaceSymbolParams{
		Query: "helper_function",
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "workspace/symbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("workspace/symbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("result is not []SymbolInformation: %T", result)
	}

	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}

	if symbols[0].Name != "helper_function" {
		t.Errorf("expected name 'helper_function', got %q", symbols[0].Name)
	}
	if symbols[0].Kind != protocol.SymbolKindFunction {
		t.Errorf("expected Function kind, got %v", symbols[0].Kind)
	}
}

// TestWorkspaceSymbol_PartialMatch tests searching with partial name.
func TestWorkspaceSymbol_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFile(t, tmpDir, "utils.bzl", `
def build_binary(name):
    pass

def build_library(name):
    pass

def test_helper():
    pass
`)

	server := NewServer(nil)

	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// Search for "build" - should match both build_binary and build_library
	symbolParams, _ := json.Marshal(protocol.WorkspaceSymbolParams{
		Query: "build",
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "workspace/symbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("workspace/symbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("result is not []SymbolInformation: %T", result)
	}

	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}

	names := make(map[string]bool)
	for _, s := range symbols {
		names[s.Name] = true
	}

	if !names["build_binary"] || !names["build_library"] {
		t.Errorf("expected build_binary and build_library, got %v", names)
	}
}

// TestWorkspaceSymbol_NoMatch tests searching with no results.
func TestWorkspaceSymbol_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFile(t, tmpDir, "utils.bzl", `
def some_function():
    pass
`)

	server := NewServer(nil)

	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// Search for "nonexistent"
	symbolParams, _ := json.Marshal(protocol.WorkspaceSymbolParams{
		Query: "nonexistent",
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "workspace/symbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("workspace/symbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("result is not []SymbolInformation: %T", result)
	}

	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols, got %d", len(symbols))
	}
}

// TestWorkspaceIndex_BuildOnInit verifies the index is built on initialization.
func TestWorkspaceIndex_BuildOnInit(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFile(t, tmpDir, "main.bzl", `
def main_function():
    pass
`)
	createTestFile(t, tmpDir, "pkg/helper.star", `
def pkg_helper():
    pass

CONFIG = "value"
`)

	server := NewServer(nil)

	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// After initialization, the workspace index should be built
	// Search for symbols to verify
	symbolParams, _ := json.Marshal(protocol.WorkspaceSymbolParams{
		Query: "main",
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "workspace/symbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("workspace/symbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("result is not []SymbolInformation: %T", result)
	}

	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol (main_function), got %d", len(symbols))
	}

	if symbols[0].Name != "main_function" {
		t.Errorf("expected 'main_function', got %q", symbols[0].Name)
	}
}

// TestWorkspaceSymbol_Variables tests searching for variables/assignments.
func TestWorkspaceSymbol_Variables(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFile(t, tmpDir, "config.bzl", `
VERSION = "1.0.0"
DEBUG_MODE = True
CONFIG_MAP = {"key": "value"}
`)

	server := NewServer(nil)

	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// Search for "VERSION"
	symbolParams, _ := json.Marshal(protocol.WorkspaceSymbolParams{
		Query: "VERSION",
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "workspace/symbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("workspace/symbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("result is not []SymbolInformation: %T", result)
	}

	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}

	if symbols[0].Name != "VERSION" {
		t.Errorf("expected 'VERSION', got %q", symbols[0].Name)
	}
	if symbols[0].Kind != protocol.SymbolKindVariable {
		t.Errorf("expected Variable kind, got %v", symbols[0].Kind)
	}
}

// TestWorkspaceSymbol_CaseInsensitive tests case-insensitive searching.
func TestWorkspaceSymbol_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFile(t, tmpDir, "utils.bzl", `
def MyFunction():
    pass

def myfunction():
    pass

MYFUNCTION = "value"
`)

	server := NewServer(nil)

	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// Search for "myfunction" (lowercase) - should match all three
	symbolParams, _ := json.Marshal(protocol.WorkspaceSymbolParams{
		Query: "myfunction",
	})
	result, err := server.Handle(context.Background(), &Request{
		Method: "workspace/symbol",
		ID:     rawID(2),
		Params: symbolParams,
	})

	if err != nil {
		t.Fatalf("workspace/symbol failed: %v", err)
	}

	symbols, ok := result.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("result is not []SymbolInformation: %T", result)
	}

	if len(symbols) != 3 {
		t.Errorf("expected 3 symbols (case-insensitive match), got %d", len(symbols))
	}
}

// TestCrossFileDefinition tests go-to-definition for symbols imported via load().
func TestCrossFileDefinition(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that exports a function
	createTestFile(t, tmpDir, "lib/utils.bzl", `
def exported_helper(arg):
    """Helper function to be loaded elsewhere."""
    return arg + 1
`)

	// Create a file that loads and uses the function
	createTestFile(t, tmpDir, "main.bzl", `
load("//lib:utils.bzl", "exported_helper")

def main():
    return exported_helper(42)
`)

	server := NewServer(nil)

	// Initialize with workspace root
	initParams, _ := json.Marshal(protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + tmpDir),
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialize",
		ID:     rawID(1),
		Params: initParams,
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "initialized",
		Params: json.RawMessage("{}"),
	})

	// Wait for workspace index to be built (in tests we build synchronously)
	server.buildWorkspaceIndexSync()

	// Open the main file
	mainContent, _ := os.ReadFile(filepath.Join(tmpDir, "main.bzl"))
	openParams, _ := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI("file://" + filepath.Join(tmpDir, "main.bzl")),
			LanguageID: "starlark",
			Version:    1,
			Text:       string(mainContent),
		},
	})
	_, _ = server.Handle(context.Background(), &Request{
		Method: "textDocument/didOpen",
		Params: openParams,
	})

	// Go to definition of "exported_helper" on line 4 (0-indexed)
	// File content with leading newline:
	// Line 0: (empty)
	// Line 1: load("//lib:utils.bzl", "exported_helper")
	// Line 2: (empty)
	// Line 3: def main():
	// Line 4:     return exported_helper(42)
	defParams, _ := json.Marshal(protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI("file://" + filepath.Join(tmpDir, "main.bzl")),
			},
			Position: protocol.Position{Line: 4, Character: 15}, // on "exported_helper"
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

	// Should point to lib/utils.bzl where exported_helper is defined
	expectedURI := "file://" + filepath.Join(tmpDir, "lib/utils.bzl")
	if string(locations[0].URI) != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, locations[0].URI)
	}

	// Should point to line 1 (0-indexed) where "def exported_helper" is
	if locations[0].Range.Start.Line != 1 {
		t.Errorf("expected line 1, got %d", locations[0].Range.Start.Line)
	}
}

// createTestFile creates a test file in the given directory.
func createTestFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}
