package lsp

import (
	"context"
	"encoding/json"
	"log"

	"github.com/albertocavalcante/sky/internal/protocol"
)

// --- Text document sync ---

func (s *Server) handleDidOpen(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.documents[p.TextDocument.Uri] = &Document{
		URI:     p.TextDocument.Uri,
		Version: p.TextDocument.Version,
		Content: p.TextDocument.Text,
	}
	s.mu.Unlock()

	log.Printf("didOpen: %s", p.TextDocument.Uri)

	// Publish initial diagnostics
	s.publishDiagnostics(ctx, p.TextDocument.Uri, p.TextDocument.Text)

	return nil, nil
}

// contentChange is a single textDocument/didChange content change. It is
// decoded locally (rather than via the generated protocol union) so that a
// missing "range" (full-document replacement) can be told apart from an
// explicit zero range.
type contentChange struct {
	Range *protocol.Range `json:"range"`
	Text  string          `json:"text"`
}

type didChangeParams struct {
	TextDocument   protocol.VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange                          `json:"contentChanges"`
}

// applyContentChange applies one change to content. A change without a range
// replaces the whole document; otherwise the range (UTF-16 positions) is
// replaced with the change text.
func applyContentChange(content string, c contentChange) string {
	if c.Range == nil {
		return c.Text
	}
	start := positionToOffset(content, int(c.Range.Start.Line), int(c.Range.Start.Character))
	end := positionToOffset(content, int(c.Range.End.Line), int(c.Range.End.Character))
	if end < start {
		start, end = end, start
	}
	return content[:start] + c.Text + content[end:]
}

func (s *Server) handleDidChange(ctx context.Context, params json.RawMessage) (any, error) {
	var p didChangeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	uri := p.TextDocument.Uri
	var content string
	var found bool

	s.mu.Lock()
	if doc, ok := s.documents[uri]; ok {
		content = doc.Content
		for _, c := range p.ContentChanges {
			content = applyContentChange(content, c)
		}
		// Documents are immutable snapshots: readers may hold the old
		// pointer outside the lock, so publish a fresh value.
		s.documents[uri] = &Document{
			URI:     doc.URI,
			Version: p.TextDocument.Version,
			Content: content,
		}
		found = true
	}
	s.mu.Unlock()

	log.Printf("didChange: %s v%d", uri, p.TextDocument.Version)

	if found {
		s.publishDiagnostics(ctx, uri, content)
	}
	return nil, nil
}

func (s *Server) handleDidClose(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.Lock()
	delete(s.documents, p.TextDocument.Uri)
	s.mu.Unlock()

	log.Printf("didClose: %s", p.TextDocument.Uri)

	// Clear diagnostics for closed document
	if conn := s.getConn(); conn != nil {
		if err := conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
			Uri:         p.TextDocument.Uri,
			Diagnostics: []protocol.Diagnostic{},
		}); err != nil {
			log.Printf("failed to clear diagnostics: %v", err)
		}
	}

	return nil, nil
}

func (s *Server) handleDidSave(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	log.Printf("didSave: %s", p.TextDocument.Uri)

	// Get document content (either from save params or our cache)
	var content string
	var known bool
	if p.Text != "" {
		content = p.Text
		known = true
	} else {
		s.mu.RLock()
		if doc, ok := s.documents[p.TextDocument.Uri]; ok {
			content = doc.Content
			known = true
		}
		s.mu.RUnlock()
	}

	// Always publish for a known document, even when empty, so stale
	// diagnostics are replaced (an emptied file must clear them).
	if known {
		s.publishDiagnostics(ctx, p.TextDocument.Uri, content)
	}

	return nil, nil
}
