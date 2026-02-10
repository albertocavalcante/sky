package lsp

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/formatter"
)

func (s *Server) handleFormatting(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DocumentFormattingParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	// Extract filename from URI for kind detection
	path := uriToPath(p.TextDocument.Uri)
	log.Printf("formatting: %s", path)

	// Format the document content
	formatted, err := formatter.Format([]byte(doc.Content), path, filekind.KindUnknown)
	if err != nil {
		log.Printf("formatting error: %v", err)
		// Return empty edits on error - don't break the editor
		return []protocol.TextEdit{}, nil
	}

	// If no changes, return empty edits
	formattedStr := string(formatted)
	if formattedStr == doc.Content {
		return []protocol.TextEdit{}, nil
	}

	// Return a single edit that replaces the entire document
	lines := strings.Count(doc.Content, "\n")
	lastLineLen := len(doc.Content)
	if idx := strings.LastIndex(doc.Content, "\n"); idx >= 0 {
		lastLineLen = len(doc.Content) - idx - 1
	}

	return []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: uint32(lines), Character: uint32(lastLineLen)},
			},
			NewText: formattedStr,
		},
	}, nil
}
