package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/checker"
	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/docgen"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/formatter"
	"github.com/albertocavalcante/sky/internal/starlark/linter"
	"github.com/albertocavalcante/sky/internal/starlark/linter/buildtools"
	"github.com/albertocavalcante/sky/internal/starlark/query/index"
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

	// Diagnostics
	lintDriver *linter.Driver
	checker    *checker.Checker

	// Builtins provider for completion and hover
	builtins builtins.Provider

	// Callbacks
	onExit func()
}

// Document represents an open text document.
type Document struct {
	URI     protocol.DocumentURI
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
		documents:  make(map[protocol.DocumentURI]*Document),
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
			SignatureHelpProvider: &protocol.SignatureHelpOptions{
				TriggerCharacters:   []string{"(", ","},
				RetriggerCharacters: []string{","},
			},
			FoldingRangeProvider: true,
			DocumentLinkProvider: &protocol.DocumentLinkOptions{},
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

	// Publish initial diagnostics
	s.publishDiagnostics(ctx, p.TextDocument.URI, p.TextDocument.Text)

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

	// Clear diagnostics for closed document
	if s.conn != nil {
		if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
			URI:         p.TextDocument.URI,
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

	log.Printf("didSave: %s", p.TextDocument.URI)

	// Get document content (either from save params or our cache)
	var content string
	if p.Text != "" {
		content = p.Text
	} else {
		s.mu.RLock()
		if doc, ok := s.documents[p.TextDocument.URI]; ok {
			content = doc.Content
		}
		s.mu.RUnlock()
	}

	// Run diagnostics
	if content != "" {
		s.publishDiagnostics(ctx, p.TextDocument.URI, content)
	}

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

	path := uriToPath(p.TextDocument.URI)

	// Find the word at the cursor position
	word := getWordAtPosition(doc.Content, int(p.Position.Line), int(p.Position.Character))
	if word == "" {
		return nil, nil
	}

	log.Printf("hover: %s @ %d:%d -> %q", path, p.Position.Line, p.Position.Character, word)

	var markdown string

	// First, check builtins from provider
	if s.builtins != nil {
		markdown = s.getBuiltinHover(word, p.TextDocument.URI)
	}

	// Fall back to document-defined symbols if not a builtin
	if markdown == "" {
		// Extract documentation
		moduleDoc, err := docgen.ExtractFile(path, []byte(doc.Content), docgen.Options{IncludePrivate: true})
		if err != nil {
			log.Printf("hover: docgen error: %v", err)
			return nil, nil
		}

		// Check functions
		for _, fn := range moduleDoc.Functions {
			if fn.Name == word {
				markdown = formatFunctionHover(fn)
				break
			}
		}

		// Check globals if not found in functions
		if markdown == "" {
			for _, g := range moduleDoc.Globals {
				if g.Name == word {
					markdown = formatGlobalHover(g)
					break
				}
			}
		}
	}

	if markdown == "" {
		return nil, nil // No documentation found
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: markdown,
		},
	}, nil
}

func (s *Server) handleDefinition(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DefinitionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	path := uriToPath(p.TextDocument.URI)

	// Find the word at the cursor position
	word := getWordAtPosition(doc.Content, int(p.Position.Line), int(p.Position.Character))
	if word == "" {
		return nil, nil
	}

	log.Printf("definition: %s @ %d:%d -> %q", path, p.Position.Line, p.Position.Character, word)

	// Classify and parse the file
	cls := classifier.NewDefaultClassifier()
	classification, err := cls.Classify(path)
	if err != nil {
		classification.FileKind = filekind.KindStarlark
	}

	file, err := parseStarlarkFile([]byte(doc.Content), path, classification.FileKind)
	if err != nil {
		log.Printf("definition: parse error: %v", err)
		return nil, nil
	}

	// Extract symbols
	indexed := index.ExtractFile(file, path, classification.FileKind)

	// Look for definition
	var defLine int

	// Check function definitions
	for _, def := range indexed.Defs {
		if def.Name == word {
			defLine = def.Line
			break
		}
	}

	// Check assignments if not found
	if defLine == 0 {
		for _, assign := range indexed.Assigns {
			if assign.Name == word {
				defLine = assign.Line
				break
			}
		}
	}

	// Check load statements for imported symbols
	if defLine == 0 {
		for _, load := range indexed.Loads {
			for localName := range load.Symbols {
				if localName == word {
					defLine = load.Line
					break
				}
			}
			if defLine > 0 {
				break
			}
		}
	}

	if defLine == 0 {
		return nil, nil // Not found
	}

	// Return location (same file for now)
	return []protocol.Location{
		{
			URI:   p.TextDocument.URI,
			Range: lineToRange(defLine),
		},
	}, nil
}

func (s *Server) handleCompletion(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parsing completion params: %w", err)
	}

	// Copy document content while holding lock to avoid race conditions
	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	var content string
	var docURI protocol.DocumentURI
	if ok {
		content = doc.Content
		docURI = doc.URI
	}
	s.mu.RUnlock()

	if !ok {
		return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
	}

	// Create a local document snapshot for completion
	docSnapshot := &Document{URI: docURI, Content: content}

	// Get the prefix being typed
	prefix := getCompletionPrefix(content, int(p.Position.Line), int(p.Position.Character))

	var items []protocol.CompletionItem

	// Check if we're completing a module member (e.g., "json.")
	if dotIdx := strings.LastIndex(prefix, "."); dotIdx >= 0 {
		moduleName := prefix[:dotIdx]
		memberPrefix := prefix[dotIdx+1:]
		items = getModuleMemberCompletions(moduleName, memberPrefix)
	} else {
		// Complete keywords, builtins, and document symbols
		// Use provider-aware keyword completions to avoid duplicates
		items = slices.Concat(
			s.getKeywordCompletionsFiltered(prefix),
			s.getProviderBuiltinCompletions(prefix, p.TextDocument.URI),
			getModuleCompletions(prefix),
			s.getDocumentSymbolCompletions(docSnapshot, prefix, int(p.Position.Line)),
		)
	}

	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

// getCompletionPrefix extracts the identifier prefix at the cursor position.
func getCompletionPrefix(content string, line, character int) string {
	// Find the target line without allocating a slice for all lines
	lineStart := 0
	currentLine := 0
	for i, ch := range content {
		if currentLine == line {
			lineStart = i
			break
		}
		if ch == '\n' {
			currentLine++
		}
	}
	if currentLine < line {
		return "" // line number exceeds content
	}

	// Find line end
	lineEnd := lineStart
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}

	lineContent := content[lineStart:lineEnd]
	if character > len(lineContent) {
		character = len(lineContent)
	}

	// Walk backwards to find the start of the identifier
	start := character
	for start > 0 {
		ch := lineContent[start-1]
		if !isIdentChar(ch) && ch != '.' {
			break
		}
		start--
	}

	return lineContent[start:character]
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// Starlark keywords
var starlarkKeywords = []string{
	"and", "break", "continue", "def", "elif", "else", "for", "if", "in",
	"lambda", "load", "not", "or", "pass", "return", "while",
	"True", "False", "None",
}

// Starlark builtin functions
var starlarkBuiltins = []string{
	"abs", "all", "any", "bool", "bytes", "dict", "dir", "enumerate",
	"fail", "float", "getattr", "hasattr", "hash", "int", "len", "list",
	"max", "min", "print", "range", "repr", "reversed", "sorted", "str",
	"tuple", "type", "zip",
}

// Common Starlark modules
var starlarkModules = map[string][]string{
	"json": {"decode", "encode", "indent"},
	"math": {"ceil", "floor", "round", "sqrt", "pow", "log", "exp", "pi", "e", "inf"},
	"time": {"now", "parse_time", "parse_duration", "time", "from_timestamp"},
}

// completionItem creates a completion item with optional snippet support.
func completionItem(label string, kind protocol.CompletionItemKind, detail string, isFunc bool) protocol.CompletionItem {
	item := protocol.CompletionItem{
		Label:  label,
		Kind:   kind,
		Detail: detail,
	}
	if isFunc {
		item.InsertText = label + "($0)"
		item.InsertTextFormat = protocol.InsertTextFormatSnippet
	}
	return item
}

func getBuiltinCompletions(prefix string) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(starlarkBuiltins))
	for _, fn := range starlarkBuiltins {
		if strings.HasPrefix(fn, prefix) {
			items = append(items, completionItem(fn, protocol.CompletionItemKindFunction, "builtin function", true))
		}
	}
	return items
}

// getKeywordCompletionsFiltered returns keyword completions, excluding items
// that are provided by the builtins provider (True, False, None are globals, not keywords).
func (s *Server) getKeywordCompletionsFiltered(prefix string) []protocol.CompletionItem {
	// When using a provider, exclude True/False/None from keywords
	// since they're provided as globals with proper type info
	excludeFromKeywords := map[string]bool{}
	if s.builtins != nil {
		excludeFromKeywords["True"] = true
		excludeFromKeywords["False"] = true
		excludeFromKeywords["None"] = true
	}

	items := make([]protocol.CompletionItem, 0, len(starlarkKeywords))
	for _, kw := range starlarkKeywords {
		if excludeFromKeywords[kw] {
			continue
		}
		if strings.HasPrefix(kw, prefix) {
			items = append(items, completionItem(kw, protocol.CompletionItemKindKeyword, "keyword", false))
		}
	}
	return items
}

// getProviderBuiltinCompletions returns completions from the builtins provider.
// Falls back to hardcoded builtins if no provider is configured.
// Uses dialect/kind detection based on the document URI to return appropriate builtins.
func (s *Server) getProviderBuiltinCompletions(prefix string, uri protocol.DocumentURI) []protocol.CompletionItem {
	// Fall back to hardcoded builtins if no provider
	if s.builtins == nil {
		return getBuiltinCompletions(prefix)
	}

	// Get dialect and file kind from URI
	dialect, kind := s.getDialectAndKind(uri)

	// Get builtins from provider for this dialect/kind
	b, err := s.builtins.Builtins(dialect, kind)
	if err != nil {
		// Fall back to hardcoded on error
		return getBuiltinCompletions(prefix)
	}

	var items []protocol.CompletionItem

	// Add builtin functions
	for _, fn := range b.Functions {
		if strings.HasPrefix(fn.Name, prefix) {
			detail := formatFunctionDetail(fn)
			items = append(items, completionItem(fn.Name, protocol.CompletionItemKindFunction, detail, true))
		}
	}

	// Add builtin types
	for _, typ := range b.Types {
		if strings.HasPrefix(typ.Name, prefix) {
			detail := typ.Doc
			if detail == "" {
				detail = "builtin type"
			}
			items = append(items, completionItem(typ.Name, protocol.CompletionItemKindClass, detail, true))
		}
	}

	// Add builtin globals
	for _, g := range b.Globals {
		if strings.HasPrefix(g.Name, prefix) {
			detail := g.Type
			if g.Doc != "" {
				detail = g.Doc
			}
			items = append(items, completionItem(g.Name, protocol.CompletionItemKindConstant, detail, false))
		}
	}

	return items
}

// formatFunctionDetail creates a detail string for function completion.
func formatFunctionDetail(fn builtins.Signature) string {
	if fn.Doc != "" {
		return fn.Doc
	}
	// Build signature as fallback
	var params []string
	for _, p := range fn.Params {
		params = append(params, p.Name)
	}
	sig := fn.Name + "(" + strings.Join(params, ", ") + ")"
	if fn.ReturnType != "" {
		sig += " -> " + fn.ReturnType
	}
	return sig
}

func getModuleCompletions(prefix string) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(starlarkModules))
	for mod := range starlarkModules {
		if strings.HasPrefix(mod, prefix) {
			items = append(items, completionItem(mod, protocol.CompletionItemKindModule, "module", false))
		}
	}
	return items
}

func getModuleMemberCompletions(moduleName, prefix string) []protocol.CompletionItem {
	members, ok := starlarkModules[moduleName]
	if !ok {
		return nil
	}
	items := make([]protocol.CompletionItem, 0, len(members))
	for _, member := range members {
		if strings.HasPrefix(member, prefix) {
			items = append(items, completionItem(member, protocol.CompletionItemKindFunction, moduleName+"."+member, true))
		}
	}
	return items
}

// getDocumentSymbolCompletions extracts symbols defined in the document,
// including function parameters if the cursor is inside a function.
func (s *Server) getDocumentSymbolCompletions(doc *Document, prefix string, line int) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Parse the document to find defined symbols
	f, err := build.ParseDefault(string(doc.URI), []byte(doc.Content))
	if err != nil {
		return items
	}

	// Find all assignments and function definitions
	for _, stmt := range f.Stmt {
		switch st := stmt.(type) {
		case *build.DefStmt:
			// Check if cursor is inside this function (for parameter completions)
			startPos, endPos := st.Span()
			defStart := startPos.Line // 1-based
			defEnd := endPos.Line     // 1-based
			if line+1 >= defStart && line+1 <= defEnd {
				// Add function parameters as completions
				for _, param := range st.Params {
					var paramName string
					switch p := param.(type) {
					case *build.Ident:
						paramName = p.Name
					case *build.AssignExpr:
						if ident, ok := p.LHS.(*build.Ident); ok {
							paramName = ident.Name
						}
					case *build.UnaryExpr: // *args or **kwargs
						if ident, ok := p.X.(*build.Ident); ok {
							paramName = ident.Name
						}
					}
					if paramName != "" && strings.HasPrefix(paramName, prefix) && paramName != prefix {
						items = append(items, protocol.CompletionItem{
							Label:  paramName,
							Kind:   protocol.CompletionItemKindVariable,
							Detail: "parameter",
						})
					}
				}
			}

			// Also add the function name itself as a completion
			name := st.Name
			if strings.HasPrefix(name, prefix) && name != prefix {
				items = append(items, protocol.CompletionItem{
					Label:            name,
					Kind:             protocol.CompletionItemKindFunction,
					Detail:           "function",
					InsertText:       name + "($0)",
					InsertTextFormat: protocol.InsertTextFormatSnippet,
				})
			}
		case *build.AssignExpr:
			if ident, ok := st.LHS.(*build.Ident); ok {
				name := ident.Name
				if strings.HasPrefix(name, prefix) && name != prefix {
					items = append(items, protocol.CompletionItem{
						Label:  name,
						Kind:   protocol.CompletionItemKindVariable,
						Detail: "variable",
					})
				}
			}
		}
	}

	return items
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

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	s.mu.RUnlock()

	if !ok {
		return []protocol.DocumentSymbol{}, nil
	}

	path := uriToPath(p.TextDocument.URI)
	log.Printf("documentSymbol: %s", path)

	// Classify the file to determine its kind
	cls := classifier.NewDefaultClassifier()
	classification, err := cls.Classify(path)
	if err != nil {
		classification.FileKind = filekind.KindStarlark
	}

	// Parse the document
	file, err := parseStarlarkFile([]byte(doc.Content), path, classification.FileKind)
	if err != nil {
		log.Printf("documentSymbol parse error: %v", err)
		return []protocol.DocumentSymbol{}, nil
	}

	// Extract symbols using skyquery index
	indexed := index.ExtractFile(file, path, classification.FileKind)

	var symbols []protocol.DocumentSymbol

	// Add function definitions
	for _, def := range indexed.Defs {
		detail := "def " + def.Name + "(" + strings.Join(def.Params, ", ") + ")"
		symbols = append(symbols, protocol.DocumentSymbol{
			Name:           def.Name,
			Kind:           protocol.SymbolKindFunction,
			Detail:         detail,
			Range:          lineToRange(def.Line),
			SelectionRange: lineToRange(def.Line),
		})
	}

	// Add top-level assignments as variables
	for _, assign := range indexed.Assigns {
		symbols = append(symbols, protocol.DocumentSymbol{
			Name:           assign.Name,
			Kind:           protocol.SymbolKindVariable,
			Range:          lineToRange(assign.Line),
			SelectionRange: lineToRange(assign.Line),
		})
	}

	return symbols, nil
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
func uriToPath(uri protocol.DocumentURI) string {
	s := string(uri)
	if strings.HasPrefix(s, "file://") {
		return s[7:] // Remove "file://"
	}
	return s
}

// --- Diagnostics ---

// publishDiagnostics runs linter and checker on a document and publishes results.
func (s *Server) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI, content string) {
	// Guard against nil connection (e.g., in tests)
	if s.conn == nil {
		return
	}

	path := uriToPath(uri)
	var diagnostics []protocol.Diagnostic

	// Run linter (reads from disk - works for didSave)
	if findings, err := s.lintDriver.RunFile(path); err == nil {
		for _, f := range findings {
			diagnostics = append(diagnostics, lintFindingToDiagnostic(f))
		}
	} else {
		log.Printf("linter error: %v", err)
	}

	// Run semantic checker (uses content from memory)
	if checkerDiags, err := s.checker.CheckFile(path, []byte(content)); err == nil {
		for _, d := range checkerDiags {
			diagnostics = append(diagnostics, checkerDiagnosticToLSP(d))
		}
	} else {
		log.Printf("checker error: %v", err)
	}

	// Publish diagnostics to client
	if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}); err != nil {
		log.Printf("failed to publish diagnostics: %v", err)
	}

	log.Printf("published %d diagnostics for %s", len(diagnostics), path)
}

// lintFindingToDiagnostic converts a linter finding to an LSP diagnostic.
func lintFindingToDiagnostic(f linter.Finding) protocol.Diagnostic {
	// Convert 1-based to 0-based positions
	startLine := uint32(0)
	if f.Line > 0 {
		startLine = uint32(f.Line - 1)
	}
	startChar := uint32(0)
	if f.Column > 0 {
		startChar = uint32(f.Column - 1)
	}
	endLine := startLine
	if f.EndLine > 0 {
		endLine = uint32(f.EndLine - 1)
	}
	endChar := startChar + 1 // Default to single character
	if f.EndColumn > 0 {
		endChar = uint32(f.EndColumn - 1)
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: startLine, Character: startChar},
			End:   protocol.Position{Line: endLine, Character: endChar},
		},
		Severity: lintSeverityToLSP(f.Severity),
		Code:     f.Rule,
		Source:   "skylint",
		Message:  f.Message,
	}
}

// checkerDiagnosticToLSP converts a checker diagnostic to an LSP diagnostic.
func checkerDiagnosticToLSP(d checker.Diagnostic) protocol.Diagnostic {
	// Convert 1-based to 0-based positions
	startLine := uint32(0)
	if d.Pos.Line > 0 {
		startLine = uint32(d.Pos.Line - 1)
	}
	startChar := uint32(0)
	if d.Pos.Col > 0 {
		startChar = uint32(d.Pos.Col - 1)
	}
	endLine := startLine
	endChar := startChar + 1 // Default to single character
	if d.End.Line > 0 {
		endLine = uint32(d.End.Line - 1)
		if d.End.Col > 0 {
			endChar = uint32(d.End.Col - 1)
		}
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: startLine, Character: startChar},
			End:   protocol.Position{Line: endLine, Character: endChar},
		},
		Severity: checkerSeverityToLSP(d.Severity),
		Code:     d.Code,
		Source:   "skycheck",
		Message:  d.Message,
	}
}

// lintSeverityToLSP converts linter severity to LSP severity.
// Linter: Error=0, Warning=1, Info=2, Hint=3
// LSP: Error=1, Warning=2, Information=3, Hint=4
func lintSeverityToLSP(s linter.Severity) protocol.DiagnosticSeverity {
	switch s {
	case linter.SeverityError:
		return protocol.DiagnosticSeverityError
	case linter.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case linter.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	case linter.SeverityHint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityWarning
	}
}

// checkerSeverityToLSP converts checker severity to LSP severity.
func checkerSeverityToLSP(s checker.Severity) protocol.DiagnosticSeverity {
	switch s {
	case checker.SeverityError:
		return protocol.DiagnosticSeverityError
	case checker.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case checker.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityWarning
	}
}

// --- Hover helpers ---

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

// formatFunctionHover formats a FunctionDoc as Markdown for hover display.
func formatFunctionHover(fn docgen.FunctionDoc) string {
	var b strings.Builder

	// Signature
	b.WriteString("```python\n")
	b.WriteString("def ")
	b.WriteString(fn.Name)
	b.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		if p.HasDefault {
			b.WriteString("=")
			b.WriteString(p.Default)
		}
	}
	b.WriteString(")\n```\n\n")

	// Documentation
	if fn.Parsed != nil && fn.Parsed.Summary != "" {
		b.WriteString(fn.Parsed.Summary)
		b.WriteString("\n")

		if fn.Parsed.Description != "" {
			b.WriteString("\n")
			b.WriteString(fn.Parsed.Description)
			b.WriteString("\n")
		}

		// Args
		if len(fn.Parsed.Args) > 0 {
			b.WriteString("\n**Args:**\n")
			for name, desc := range fn.Parsed.Args {
				b.WriteString("- `")
				b.WriteString(name)
				b.WriteString("`: ")
				b.WriteString(desc)
				b.WriteString("\n")
			}
		}

		// Returns
		if fn.Parsed.Returns != "" {
			b.WriteString("\n**Returns:** ")
			b.WriteString(fn.Parsed.Returns)
			b.WriteString("\n")
		}

		// Example
		if fn.Parsed.Example != "" {
			b.WriteString("\n**Example:**\n```python\n")
			b.WriteString(fn.Parsed.Example)
			b.WriteString("\n```\n")
		}
	} else if fn.Docstring != "" {
		// Raw docstring if not parsed
		b.WriteString(fn.Docstring)
		b.WriteString("\n")
	}

	return b.String()
}

// formatGlobalHover formats a GlobalDoc as Markdown for hover display.
func formatGlobalHover(g docgen.GlobalDoc) string {
	var b strings.Builder
	b.WriteString("```python\n")
	b.WriteString(g.Name)
	b.WriteString(" = ")
	b.WriteString(g.Value)
	b.WriteString("\n```\n")
	return b.String()
}

// getBuiltinHover returns hover markdown for a builtin symbol from the provider.
// Uses dialect/kind detection based on the document URI to return appropriate builtins.
func (s *Server) getBuiltinHover(word string, uri protocol.DocumentURI) string {
	if s.builtins == nil {
		return ""
	}

	// Get dialect and file kind from URI
	dialect, kind := s.getDialectAndKind(uri)

	b, err := s.builtins.Builtins(dialect, kind)
	if err != nil {
		return ""
	}

	// Check builtin functions
	for _, fn := range b.Functions {
		if fn.Name == word {
			return formatBuiltinFunctionHover(fn)
		}
	}

	// Check builtin types
	for _, typ := range b.Types {
		if typ.Name == word {
			return formatBuiltinTypeHover(typ)
		}
	}

	// Check builtin globals
	for _, g := range b.Globals {
		if g.Name == word {
			return formatBuiltinGlobalHover(g)
		}
	}

	return ""
}

// formatBuiltinFunctionHover formats a builtin function signature for hover.
func formatBuiltinFunctionHover(fn builtins.Signature) string {
	var b strings.Builder

	// Signature
	b.WriteString("```python\n")
	b.WriteString(fn.Name)
	b.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		if p.Variadic {
			b.WriteString("*")
		}
		if p.KWArgs {
			b.WriteString("**")
		}
		b.WriteString(p.Name)
		if p.Type != "" {
			b.WriteString(": ")
			b.WriteString(p.Type)
		}
		if p.Default != "" {
			b.WriteString(" = ")
			b.WriteString(p.Default)
		}
	}
	b.WriteString(")")
	if fn.ReturnType != "" {
		b.WriteString(" -> ")
		b.WriteString(fn.ReturnType)
	}
	b.WriteString("\n```\n\n")

	// Documentation
	if fn.Doc != "" {
		b.WriteString(fn.Doc)
		b.WriteString("\n")
	}

	return b.String()
}

// formatBuiltinTypeHover formats a builtin type definition for hover.
func formatBuiltinTypeHover(typ builtins.TypeDef) string {
	var b strings.Builder

	// Type header
	b.WriteString("```python\n")
	b.WriteString("type ")
	b.WriteString(typ.Name)
	b.WriteString("\n```\n\n")

	// Documentation
	if typ.Doc != "" {
		b.WriteString(typ.Doc)
		b.WriteString("\n")
	}

	// Fields
	if len(typ.Fields) > 0 {
		b.WriteString("\n**Fields:**\n")
		for _, f := range typ.Fields {
			b.WriteString("- `")
			b.WriteString(f.Name)
			b.WriteString("`")
			if f.Type != "" {
				b.WriteString(": ")
				b.WriteString(f.Type)
			}
			if f.Doc != "" {
				b.WriteString(" - ")
				b.WriteString(f.Doc)
			}
			b.WriteString("\n")
		}
	}

	// Methods
	if len(typ.Methods) > 0 {
		b.WriteString("\n**Methods:**\n")
		for _, m := range typ.Methods {
			b.WriteString("- `")
			b.WriteString(m.Name)
			b.WriteString("()`")
			if m.Doc != "" {
				b.WriteString(" - ")
				b.WriteString(m.Doc)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// formatBuiltinGlobalHover formats a builtin global/constant for hover.
func formatBuiltinGlobalHover(g builtins.Field) string {
	var b strings.Builder

	// Global
	b.WriteString("```python\n")
	b.WriteString(g.Name)
	if g.Type != "" {
		b.WriteString(": ")
		b.WriteString(g.Type)
	}
	b.WriteString("\n```\n\n")

	// Documentation
	if g.Doc != "" {
		b.WriteString(g.Doc)
		b.WriteString("\n")
	}

	return b.String()
}
