package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/checker"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
	"github.com/albertocavalcante/sky/internal/starlark/linter/buildtools"
)

// Server handles LSP requests for Starlark files.
type Server struct {
	conn *Conn

	// State
	mu          sync.RWMutex
	initialized bool
	shutdown    bool
	documents   map[string]*Document
	rootURI     string

	// Diagnostics
	lintDriver *linter.Driver
	checker    *checker.Checker

	// Builtins provider for completion and hover
	builtins builtins.Provider

	// Workspace index for cross-file features
	workspace *WorkspaceIndex

	// Callbacks
	onExit func()
}

// Document represents an open text document.
type Document struct {
	URI     string
	Version int32
	Content string
}

// NewServer creates a new LSP server with default configuration.
// It initializes a default builtins provider from proto/JSON data
// to provide completion and hover for Bazel and Starlark builtins.
func NewServer(onExit func()) *Server {
	return NewServerWithProvider(onExit, NewDefaultProvider())
}

// NewServerWithProvider creates a new LSP server with a custom builtins provider.
// If provider is nil, the server will use hardcoded fallback builtins.
func NewServerWithProvider(onExit func(), provider builtins.Provider) *Server {
	// Set up linter with buildtools rules
	registry := linter.NewRegistry()
	_ = registry.Register(buildtools.AllRules()...)
	lintDriver := linter.NewDriver(registry)

	// Set up semantic checker
	chk := checker.New(checker.DefaultOptions())

	return &Server{
		documents:  make(map[string]*Document),
		lintDriver: lintDriver,
		checker:    chk,
		builtins:   provider,
		onExit:     onExit,
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
	case "textDocument/foldingRange":
		return s.handleFoldingRange(ctx, req.Params)
	case "textDocument/documentLink":
		return s.handleDocumentLink(ctx, req.Params)
	case "textDocument/signatureHelp":
		return s.handleSignatureHelp(ctx, req.Params)
	case "textDocument/codeAction":
		return s.handleCodeAction(ctx, req.Params)
	case "textDocument/references":
		return s.handleReferences(ctx, req.Params)
	case "textDocument/rename":
		return s.handleRename(ctx, req.Params)
	case "textDocument/prepareRename":
		return s.handlePrepareRename(ctx, req.Params)

	// Workspace features
	case "workspace/symbol":
		return s.handleWorkspaceSymbol(ctx, req.Params)

	// Semantic tokens
	case "textDocument/semanticTokens/full":
		return s.handleSemanticTokensFull(ctx, req.Params)
	case "textDocument/semanticTokens/range":
		return s.handleSemanticTokensRange(ctx, req.Params)

	// Inlay hints
	case "textDocument/inlayHint":
		return s.handleInlayHint(ctx, req.Params)

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
	if p.WorkspaceFolders != nil && len(*p.WorkspaceFolders) > 0 {
		s.rootURI = string((*p.WorkspaceFolders)[0].Uri)
	} else if p.RootUri != nil && *p.RootUri != "" {
		s.rootURI = *p.RootUri
	}
	s.mu.Unlock()

	log.Printf("initialize: root=%s", s.rootURI)

	// Build capabilities using a map to include fields not in protocol v0.12.0
	capabilities := map[string]interface{}{
		"textDocumentSync": protocol.TextDocumentSyncOptions{
			OpenClose: true,
			Change:    protocol.TextDocumentSyncKindFull,
			Save: protocol.Or_SaveOptions_bool{Value: protocol.SaveOptions{
				IncludeText: true}},
		},
		"hoverProvider":              true,
		"definitionProvider":         true,
		"documentSymbolProvider":     true,
		"documentFormattingProvider": true,
		"foldingRangeProvider":       true,
		"referencesProvider":         true,
		"workspaceSymbolProvider":    true,
		"completionProvider": &protocol.CompletionOptions{
			TriggerCharacters: []string{".", "("},
		},
		"signatureHelpProvider": &protocol.SignatureHelpOptions{
			TriggerCharacters:   []string{"(", ","},
			RetriggerCharacters: []string{","},
		},
		"documentLinkProvider": &protocol.DocumentLinkOptions{},
		"codeActionProvider": &protocol.CodeActionOptions{
			CodeActionKinds: []protocol.CodeActionKind{protocol.CodeActionKindQuickFix},
		},
		"renameProvider": &protocol.RenameOptions{
			PrepareProvider: true,
		},
		"semanticTokensProvider": map[string]interface{}{
			"legend": protocol.SemanticTokensLegend{
				TokenTypes:     TokenTypeNames,
				TokenModifiers: TokenModifierNames,
			},
			"full":  true,
			"range": true,
		},
		// InlayHintProvider is not in protocol v0.12.0, but we include it here
		"inlayHintProvider": true,
	}

	return map[string]interface{}{
		"capabilities": capabilities,
		"serverInfo": map[string]string{
			"name":    "skyls",
			"version": "0.1.0",
		},
	}, nil
}

func (s *Server) handleInitialized(ctx context.Context, params json.RawMessage) (any, error) {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	log.Printf("initialized")

	// Build workspace index in background
	go s.buildWorkspaceIndex()

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

// --- Shared utilities ---

// getWordAtPosition extracts the word at a given line and character position.
func getWordAtPosition(content string, line, char int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}

	lineContent := lines[line]
	if char < 0 || char >= len(lineContent) {
		return ""
	}

	// Find start of word (use package-level isIdentChar)
	start := char
	for start > 0 && isIdentChar(lineContent[start-1]) {
		start--
	}

	// Find end of word
	end := char
	for end < len(lineContent) && isIdentChar(lineContent[end]) {
		end++
	}

	if start == end {
		return ""
	}

	return lineContent[start:end]
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// parseStarlarkFile parses content into a build.File based on file kind.
func parseStarlarkFile(content []byte, path string, kind filekind.Kind) (*build.File, error) {
	switch kind {
	case filekind.KindBUILD, filekind.KindBUCK:
		return build.ParseBuild(path, content)
	case filekind.KindWORKSPACE:
		return build.ParseWorkspace(path, content)
	case filekind.KindMODULE:
		return build.ParseModule(path, content)
	case filekind.KindBzl, filekind.KindBzlmod, filekind.KindBzlBuck:
		return build.ParseBzl(path, content)
	default:
		return build.ParseDefault(path, content)
	}
}

// lineToRange creates a Range for a line number (1-based input, 0-based output).
func lineToRange(line int) protocol.Range {
	l := uint32(0)
	if line > 0 {
		l = uint32(line - 1)
	}
	return protocol.Range{
		Start: protocol.Position{Line: l, Character: 0},
		End:   protocol.Position{Line: l, Character: 1000}, // End of line approximation
	}
}

// uriToPath converts a document URI to a file path.
// Handles file:// URIs and returns just the path component.
func uriToPath(uri string) string {
	s := string(uri)
	if strings.HasPrefix(s, "file://") {
		return s[7:] // Remove "file://"
	}
	return s
}

// ptrInt32 returns a pointer to the given int32 value.
func ptrInt32(v int32) *int32 {
	return &v
}

// ptrString returns a pointer to the given string value.
func ptrString(v string) *string {
	return &v
}
