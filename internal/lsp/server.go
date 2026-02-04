package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/formatter"
	"go.lsp.dev/protocol"
)

// Server handles LSP requests for Starlark files.
type Server struct {
	conn *Conn

	// State
	mu          sync.RWMutex
	initialized bool
	shutdown    bool
	documents   map[protocol.DocumentURI]*Document
	rootURI     protocol.DocumentURI

	// Callbacks
	onExit func()
}

// Document represents an open text document.
type Document struct {
	URI     protocol.DocumentURI
	Version int32
	Content string
}

// NewServer creates a new LSP server.
func NewServer(onExit func()) *Server {
	return &Server{
		documents: make(map[protocol.DocumentURI]*Document),
		onExit:    onExit,
	}
}

// SetConn sets the connection for sending notifications.
func (s *Server) SetConn(conn *Conn) {
	s.conn = conn
}

// Handle implements Handler interface - routes requests to methods.
func (s *Server) Handle(ctx context.Context, req *Request) (any, error) {
	s.mu.RLock()
	shutdown := s.shutdown
	initialized := s.initialized
	s.mu.RUnlock()

	// Check shutdown state - only allow exit after shutdown
	if shutdown && req.Method != "exit" {
		return nil, &ResponseError{
			Code:    CodeInvalidRequest,
			Message: "server is shutting down",
		}
	}

	// Check initialization - only lifecycle methods allowed before initialize
	if !initialized {
		switch req.Method {
		case "initialize", "initialized", "shutdown", "exit":
			// Allowed before initialization
		default:
			return nil, &ResponseError{
				Code:    CodeInvalidRequest,
				Message: "server not initialized",
			}
		}
	}

	// Route to method handlers
	switch req.Method {
	// Lifecycle
	case "initialize":
		return s.handleInitialize(ctx, req.Params)
	case "initialized":
		return s.handleInitialized(ctx, req.Params)
	case "shutdown":
		return s.handleShutdown(ctx)
	case "exit":
		return s.handleExit(ctx)

	// Text document sync
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, req.Params)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, req.Params)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, req.Params)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, req.Params)

	// Language features
	case "textDocument/hover":
		return s.handleHover(ctx, req.Params)
	case "textDocument/definition":
		return s.handleDefinition(ctx, req.Params)
	case "textDocument/completion":
		return s.handleCompletion(ctx, req.Params)
	case "textDocument/formatting":
		return s.handleFormatting(ctx, req.Params)
	case "textDocument/documentSymbol":
		return s.handleDocumentSymbol(ctx, req.Params)

	default:
		log.Printf("unhandled method: %s", req.Method)
		return nil, ErrMethodNotFound
	}
}

// --- Lifecycle methods ---

func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parsing initialize params: %w", err)
	}

	s.mu.Lock()
	if len(p.WorkspaceFolders) > 0 {
		s.rootURI = protocol.DocumentURI(p.WorkspaceFolders[0].URI)
	} else if p.RootURI != "" {
		s.rootURI = p.RootURI
	}
	s.mu.Unlock()

	log.Printf("initialize: root=%s", s.rootURI)

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save: &protocol.SaveOptions{
					IncludeText: true,
				},
			},
			HoverProvider:          true,
			DefinitionProvider:     true,
			DocumentSymbolProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", "("},
			},
			DocumentFormattingProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "skyls",
			Version: "0.1.0",
		},
	}, nil
}

func (s *Server) handleInitialized(ctx context.Context, params json.RawMessage) (any, error) {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	log.Printf("initialized")
	return nil, nil
}

func (s *Server) handleShutdown(ctx context.Context) (any, error) {
	s.mu.Lock()
	s.shutdown = true
	s.mu.Unlock()

	log.Printf("shutdown")
	return nil, nil
}

func (s *Server) handleExit(ctx context.Context) (any, error) {
	log.Printf("exit")
	if s.onExit != nil {
		s.onExit()
	}
	return nil, nil
}

// --- Text document sync ---

func (s *Server) handleDidOpen(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.documents[p.TextDocument.URI] = &Document{
		URI:     p.TextDocument.URI,
		Version: p.TextDocument.Version,
		Content: p.TextDocument.Text,
	}
	s.mu.Unlock()

	log.Printf("didOpen: %s", p.TextDocument.URI)

	// TODO: Publish diagnostics
	return nil, nil
}

func (s *Server) handleDidChange(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.Lock()
	if doc, ok := s.documents[p.TextDocument.URI]; ok {
		doc.Version = p.TextDocument.Version
		// Full sync - take the last change
		if len(p.ContentChanges) > 0 {
			doc.Content = p.ContentChanges[len(p.ContentChanges)-1].Text
		}
	}
	s.mu.Unlock()

	log.Printf("didChange: %s v%d", p.TextDocument.URI, p.TextDocument.Version)
	return nil, nil
}

func (s *Server) handleDidClose(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.Lock()
	delete(s.documents, p.TextDocument.URI)
	s.mu.Unlock()

	log.Printf("didClose: %s", p.TextDocument.URI)
	return nil, nil
}

func (s *Server) handleDidSave(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	log.Printf("didSave: %s", p.TextDocument.URI)

	// TODO: Run diagnostics (skylint, skycheck)
	return nil, nil
}

// --- Language features ---

func (s *Server) handleHover(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.HoverParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	// TODO: Integrate with skydoc for actual hover info
	_ = doc

	// For now, return a placeholder
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**skyls** - hover not yet implemented",
		},
	}, nil
}

func (s *Server) handleDefinition(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DefinitionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// TODO: Integrate with skyquery for go-to-definition
	log.Printf("definition: %s @ %d:%d", p.TextDocument.URI, p.Position.Line, p.Position.Character)

	return nil, nil // No result yet
}

func (s *Server) handleCompletion(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// TODO: Implement completion using builtins + loaded symbols
	log.Printf("completion: %s @ %d:%d", p.TextDocument.URI, p.Position.Line, p.Position.Character)

	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        []protocol.CompletionItem{},
	}, nil
}

func (s *Server) handleFormatting(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DocumentFormattingParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	// Extract filename from URI for kind detection
	path := uriToPath(p.TextDocument.URI)
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

func (s *Server) handleDocumentSymbol(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DocumentSymbolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// TODO: Integrate with skyquery for symbols
	log.Printf("documentSymbol: %s", p.TextDocument.URI)

	return []protocol.DocumentSymbol{}, nil
}

// uriToPath converts a document URI to a file path.
// Handles file:// URIs and returns just the path component.
func uriToPath(uri protocol.DocumentURI) string {
	s := string(uri)
	if strings.HasPrefix(s, "file://") {
		return s[7:] // Remove "file://"
	}
	return s
}
