package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
)

// TestCodeAction_NoFixes tests that no code actions are returned when there are no fixable issues.
func TestCodeAction_NoFixes(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Open a document with no lint issues
	content := `def hello(name):
    """Greet someone."""
    return "Hello, " + name
`
	openDocument(t, server, "file:///test.star", content)

	// Request code actions
	result := requestCodeActions(t, server, "file:///test.star", protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 2, Character: 0},
	})

	// Should return empty list (no fixable issues)
	if len(result) != 0 {
		t.Errorf("expected 0 code actions, got %d", len(result))
	}
}

// TestCodeAction_SingleFix tests that a code action is returned for a single fixable issue.
func TestCodeAction_SingleFix(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Create a mock finding with a replacement
	finding := linter.Finding{
		FilePath:  "/test.star",
		Line:      1,
		Column:    1,
		EndLine:   1,
		EndColumn: 10,
		Rule:      "test-rule",
		Message:   "Test issue",
		Severity:  linter.SeverityWarning,
		Replacement: &linter.Replacement{
			Content: "fixed_code",
			Start:   0,
			End:     9,
		},
	}

	// Test the conversion function directly
	content := "old_code\nmore content"
	actions := findingsToCodeActions("file:///test.star", []linter.Finding{finding}, content)

	if len(actions) != 1 {
		t.Fatalf("expected 1 code action, got %d", len(actions))
	}

	action := actions[0]
	if action.Title != "Fix: test-rule" {
		t.Errorf("Title = %q, want %q", action.Title, "Fix: test-rule")
	}
	if action.Kind != protocol.CodeActionKindQuickFix {
		t.Errorf("Kind = %v, want QuickFix", action.Kind)
	}
	if action.Edit.Changes == nil && action.Edit.DocumentChanges == nil {
		t.Fatal("Edit should not be nil")
	}

	changes := action.Edit.Changes
	if changes == nil {
		t.Fatal("Changes should not be nil")
	}

	edits, ok := changes["file:///test.star"]
	if !ok || len(edits) != 1 {
		t.Fatalf("expected 1 edit for URI, got %d", len(edits))
	}

	edit := edits[0]
	if edit.NewText != "fixed_code" {
		t.Errorf("NewText = %q, want %q", edit.NewText, "fixed_code")
	}
}

// TestCodeAction_MultipleFixes tests that multiple code actions are returned for multiple fixable issues.
func TestCodeAction_MultipleFixes(t *testing.T) {
	server := NewServer(nil)
	initializeServer(t, server)

	// Create mock findings with replacements
	findings := []linter.Finding{
		{
			FilePath:  "/test.star",
			Line:      1,
			Column:    1,
			EndLine:   1,
			EndColumn: 5,
			Rule:      "rule-one",
			Message:   "First issue",
			Severity:  linter.SeverityWarning,
			Replacement: &linter.Replacement{
				Content: "FIX1",
				Start:   0,
				End:     4,
			},
		},
		{
			FilePath:  "/test.star",
			Line:      2,
			Column:    1,
			EndLine:   2,
			EndColumn: 5,
			Rule:      "rule-two",
			Message:   "Second issue",
			Severity:  linter.SeverityWarning,
			Replacement: &linter.Replacement{
				Content: "FIX2",
				Start:   10,
				End:     14,
			},
		},
		{
			// Finding without replacement - should not produce code action
			FilePath: "/test.star",
			Line:     3,
			Column:   1,
			Rule:     "rule-three",
			Message:  "Third issue (no fix)",
			Severity: linter.SeverityWarning,
		},
	}

	content := "test\nline\nmore"
	actions := findingsToCodeActions("file:///test.star", findings, content)

	// Should have 2 code actions (only for findings with replacements)
	if len(actions) != 2 {
		t.Fatalf("expected 2 code actions, got %d", len(actions))
	}

	// Verify titles
	titles := make(map[string]bool)
	for _, action := range actions {
		titles[action.Title] = true
	}

	if !titles["Fix: rule-one"] {
		t.Error("missing code action for rule-one")
	}
	if !titles["Fix: rule-two"] {
		t.Error("missing code action for rule-two")
	}
}

// TestCodeAction_InitializeCapability tests that the server advertises code action capability.
func TestCodeAction_InitializeCapability(t *testing.T) {
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

	// Check CodeActionProvider capability
	if capabilities["codeActionProvider"] == nil {
		t.Error("CodeActionProvider should not be nil")
	}
}

// TestCodeAction_ByteOffsetToPosition tests the byte offset to line/column conversion.
func TestCodeAction_ByteOffsetToPosition(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		offset     int
		wantLine   uint32
		wantColumn uint32
	}{
		{
			name:       "start of content",
			content:    "hello\nworld",
			offset:     0,
			wantLine:   0,
			wantColumn: 0,
		},
		{
			name:       "middle of first line",
			content:    "hello\nworld",
			offset:     3,
			wantLine:   0,
			wantColumn: 3,
		},
		{
			name:       "start of second line",
			content:    "hello\nworld",
			offset:     6,
			wantLine:   1,
			wantColumn: 0,
		},
		{
			name:       "middle of second line",
			content:    "hello\nworld",
			offset:     8,
			wantLine:   1,
			wantColumn: 2,
		},
		{
			name:       "end of content",
			content:    "hello\nworld",
			offset:     11,
			wantLine:   1,
			wantColumn: 5,
		},
		{
			name:       "three lines",
			content:    "abc\ndef\nghi",
			offset:     8, // 'g'
			wantLine:   2,
			wantColumn: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := byteOffsetToPosition(tt.content, tt.offset)
			if line != tt.wantLine {
				t.Errorf("line = %d, want %d", line, tt.wantLine)
			}
			if col != tt.wantColumn {
				t.Errorf("column = %d, want %d", col, tt.wantColumn)
			}
		})
	}
}

// TestCodeAction_FilterByRange tests that code actions are filtered by the requested range.
func TestCodeAction_FilterByRange(t *testing.T) {
	// Findings at different lines
	findings := []linter.Finding{
		{
			FilePath:    "/test.star",
			Line:        2,
			Column:      1,
			EndLine:     2,
			EndColumn:   5,
			Rule:        "rule-in-range",
			Message:     "In range",
			Severity:    linter.SeverityWarning,
			Replacement: &linter.Replacement{Content: "FIX", Start: 5, End: 9},
		},
		{
			FilePath:    "/test.star",
			Line:        10,
			Column:      1,
			EndLine:     10,
			EndColumn:   5,
			Rule:        "rule-out-of-range",
			Message:     "Out of range",
			Severity:    linter.SeverityWarning,
			Replacement: &linter.Replacement{Content: "FIX", Start: 50, End: 54},
		},
	}

	content := "line1\ntest\nline3"

	// Request range covering only line 2 (0-indexed: line 1)
	requestRange := protocol.Range{
		Start: protocol.Position{Line: 1, Character: 0},
		End:   protocol.Position{Line: 1, Character: 10},
	}

	actions := filterCodeActionsByRange(
		findingsToCodeActions("file:///test.star", findings, content),
		requestRange,
	)

	// Should only have 1 code action (the one in range)
	if len(actions) != 1 {
		t.Fatalf("expected 1 code action in range, got %d", len(actions))
	}

	if actions[0].Title != "Fix: rule-in-range" {
		t.Errorf("expected action for rule-in-range, got %q", actions[0].Title)
	}
}

// Helper function for code action tests
// Note: initializeServer and openDocument are defined in rename_test.go

func requestCodeActions(t *testing.T, server *Server, uri string, rng protocol.Range) []protocol.CodeAction {
	t.Helper()

	params, _ := json.Marshal(protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: string(uri)},
		Range:        rng,
		Context:      protocol.CodeActionContext{},
	})

	result, err := server.Handle(context.Background(), &Request{
		Method: "textDocument/codeAction",
		ID:     rawID(2),
		Params: params,
	})
	if err != nil {
		t.Fatalf("codeAction failed: %v", err)
	}

	actions, ok := result.([]protocol.CodeAction)
	if !ok {
		t.Fatalf("result is not []CodeAction: %T", result)
	}

	return actions
}
