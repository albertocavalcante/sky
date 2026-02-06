package lsp

import (
	"context"
	"encoding/json"
	"log"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
)

// handleCodeAction returns code actions (quick fixes) for diagnostics in the given range.
func (s *Server) handleCodeAction(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.CodeActionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()

	if !ok {
		return []protocol.CodeAction{}, nil
	}

	path := uriToPath(p.TextDocument.Uri)
	log.Printf("codeAction: %s range=%v", path, p.Range)

	// Run linter to get findings with replacements
	findings, err := s.lintDriver.RunFile(path)
	if err != nil {
		log.Printf("codeAction: linter error: %v", err)
		return []protocol.CodeAction{}, nil
	}

	// Convert findings to code actions
	actions := findingsToCodeActions(string(p.TextDocument.Uri), findings, doc.Content)

	// Filter by requested range
	actions = filterCodeActionsByRange(actions, p.Range)

	log.Printf("codeAction: returning %d actions", len(actions))
	return actions, nil
}

// findingsToCodeActions converts linter findings with replacements to LSP code actions.
func findingsToCodeActions(uri string, findings []linter.Finding, content string) []protocol.CodeAction {
	var actions []protocol.CodeAction

	for _, f := range findings {
		// Skip findings without replacements (not fixable)
		if f.Replacement == nil {
			continue
		}

		// Convert byte offsets to line/column positions
		startLine, startCol := byteOffsetToPosition(content, f.Replacement.Start)
		endLine, endCol := byteOffsetToPosition(content, f.Replacement.End)

		// Create the text edit
		edit := protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: startLine, Character: startCol},
				End:   protocol.Position{Line: endLine, Character: endCol},
			},
			NewText: f.Replacement.Content,
		}

		// Create the diagnostic that this action fixes
		diag := lintFindingToDiagnostic(f)

		// Create the code action
		action := protocol.CodeAction{
			Title: "Fix: " + f.Rule,
			Kind:  protocol.CodeActionKindQuickFix,
			Diagnostics: []protocol.Diagnostic{
				diag,
			},
			Edit: protocol.WorkspaceEdit{
				Changes: map[string][]protocol.TextEdit{
					string(uri): {edit},
				},
			},
		}

		actions = append(actions, action)
	}

	return actions
}

// byteOffsetToPosition converts a byte offset in content to a 0-based line and column.
func byteOffsetToPosition(content string, offset int) (line, col uint32) {
	if offset < 0 {
		return 0, 0
	}
	if offset > len(content) {
		offset = len(content)
	}

	line = 0
	lineStart := 0

	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}

	col = uint32(offset - lineStart)
	return line, col
}

// filterCodeActionsByRange filters code actions to only include those
// whose diagnostic range intersects with the requested range.
func filterCodeActionsByRange(actions []protocol.CodeAction, rng protocol.Range) []protocol.CodeAction {
	var filtered []protocol.CodeAction

	for _, action := range actions {
		// Check if any of the action's diagnostics intersect with the range
		for _, diag := range action.Diagnostics {
			if rangesIntersect(diag.Range, rng) {
				filtered = append(filtered, action)
				break
			}
		}
	}

	return filtered
}

// rangesIntersect returns true if two ranges overlap.
func rangesIntersect(a, b protocol.Range) bool {
	// Range a is completely before range b
	if a.End.Line < b.Start.Line ||
		(a.End.Line == b.Start.Line && a.End.Character <= b.Start.Character) {
		return false
	}

	// Range a is completely after range b
	if a.Start.Line > b.End.Line ||
		(a.Start.Line == b.End.Line && a.Start.Character >= b.End.Character) {
		return false
	}

	return true
}
