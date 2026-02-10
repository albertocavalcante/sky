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

func (s *Server) handleDidChange(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.Lock()
	if doc, ok := s.documents[p.TextDocument.Uri]; ok {
		doc.Version = p.TextDocument.Version
		// Full sync - take the last change
		if len(p.ContentChanges) > 0 {
			doc.Content = p.ContentChanges[len(p.ContentChanges)-1].Value.(protocol.TextDocumentContentChangeWholeDocument).Text
		}
	}
	s.mu.Unlock()

	log.Printf("didChange: %s v%d", p.TextDocument.Uri, p.TextDocument.Version)
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
	if s.conn != nil {
		if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
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
	if p.Text != "" {
		content = p.Text
	} else {
		s.mu.RLock()
		if doc, ok := s.documents[p.TextDocument.Uri]; ok {
			content = doc.Content
		}
		s.mu.RUnlock()
	}

	// Run diagnostics
	if content != "" {
		s.publishDiagnostics(ctx, p.TextDocument.Uri, content)
	}

	return nil, nil
}
