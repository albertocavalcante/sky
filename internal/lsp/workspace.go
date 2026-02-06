package lsp

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

// WorkspaceIndex holds indexed symbols across the workspace for fast lookup.
type WorkspaceIndex struct {
	mu sync.RWMutex

	// root is the workspace root directory
	root string

	// symbols maps symbol name -> list of definitions
	// This allows quick lookup by name for workspace/symbol
	symbols map[string][]SymbolDef

	// exports maps file path -> list of exported symbol names
	// This is used to resolve cross-file references
	exports map[string][]string

	// loadCache maps load module path -> resolved absolute file path
	// This caches resolved load paths for faster lookup
	loadCache map[string]string
}

// SymbolDef represents a symbol definition in the workspace.
type SymbolDef struct {
	Name     string
	Kind     protocol.SymbolKind
	Location protocol.Location
	File     string // Absolute file path
}

// NewWorkspaceIndex creates a new workspace index.
func NewWorkspaceIndex(root string) *WorkspaceIndex {
	return &WorkspaceIndex{
		root:      root,
		symbols:   make(map[string][]SymbolDef),
		exports:   make(map[string][]string),
		loadCache: make(map[string]string),
	}
}

// Clear removes all indexed data.
func (w *WorkspaceIndex) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.symbols = make(map[string][]SymbolDef)
	w.exports = make(map[string][]string)
	w.loadCache = make(map[string]string)
}

// AddFile indexes a single file and adds its symbols to the index.
func (w *WorkspaceIndex) AddFile(indexedFile *index.File, absPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var exportedNames []string

	// Index function definitions
	for _, def := range indexedFile.Defs {
		symbolDef := SymbolDef{
			Name: def.Name,
			Kind: protocol.SymbolKindFunction,
			Location: protocol.Location{
				Uri:   string("file://" + absPath),
				Range: lineToRange(def.Line),
			},
			File: absPath,
		}
		w.symbols[def.Name] = append(w.symbols[def.Name], symbolDef)
		exportedNames = append(exportedNames, def.Name)
	}

	// Index top-level assignments (variables/constants)
	for _, assign := range indexedFile.Assigns {
		symbolDef := SymbolDef{
			Name: assign.Name,
			Kind: protocol.SymbolKindVariable,
			Location: protocol.Location{
				Uri:   string("file://" + absPath),
				Range: lineToRange(assign.Line),
			},
			File: absPath,
		}
		w.symbols[assign.Name] = append(w.symbols[assign.Name], symbolDef)
		exportedNames = append(exportedNames, assign.Name)
	}

	w.exports[absPath] = exportedNames
}

// Search searches for symbols matching the query.
// The search is case-insensitive and matches symbols that contain the query.
func (w *WorkspaceIndex) Search(query string) []SymbolDef {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if query == "" {
		return nil
	}

	queryLower := strings.ToLower(query)
	var results []SymbolDef

	for name, defs := range w.symbols {
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, queryLower) {
			results = append(results, defs...)
		}
	}

	return results
}

// FindDefinition searches for a symbol definition by name.
// Returns the first matching definition or nil if not found.
func (w *WorkspaceIndex) FindDefinition(name string) *protocol.Location {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if defs, ok := w.symbols[name]; ok && len(defs) > 0 {
		return &defs[0].Location
	}
	return nil
}

// FindDefinitionInFile searches for a symbol that is exported from a specific file.
func (w *WorkspaceIndex) FindDefinitionInFile(name string, filePath string) *protocol.Location {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if defs, ok := w.symbols[name]; ok {
		for _, def := range defs {
			if def.File == filePath {
				return &def.Location
			}
		}
	}
	return nil
}

// ResolveLoadPath resolves a load module path to an absolute file path.
// Uses caching for repeated lookups.
func (w *WorkspaceIndex) ResolveLoadPath(module string, fromFile string) string {
	// Try cache first
	cacheKey := fromFile + "|" + module
	w.mu.RLock()
	if cached, ok := w.loadCache[cacheKey]; ok {
		w.mu.RUnlock()
		return cached
	}
	w.mu.RUnlock()

	// Resolve the path
	resolved := resolveLoadPath(module, fromFile, w.root)

	// Cache the result
	if resolved != "" {
		w.mu.Lock()
		w.loadCache[cacheKey] = resolved
		w.mu.Unlock()
	}

	return resolved
}

// handleWorkspaceSymbol handles the workspace/symbol request.
func (s *Server) handleWorkspaceSymbol(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	log.Printf("workspace/symbol: query=%q", p.Query)

	s.mu.RLock()
	wsIndex := s.workspace
	s.mu.RUnlock()

	if wsIndex == nil {
		return []protocol.SymbolInformation{}, nil
	}

	// Search for symbols matching the query
	matches := wsIndex.Search(p.Query)

	// Convert to SymbolInformation
	var symbols []protocol.SymbolInformation
	for _, match := range matches {
		symbols = append(symbols, protocol.SymbolInformation{BaseSymbolInformation: protocol.BaseSymbolInformation{
			Name: match.Name,
			Kind: match.Kind},
			Location: match.Location,
		})
	}

	return symbols, nil
}

// buildWorkspaceIndex scans the workspace and builds the symbol index.
func (s *Server) buildWorkspaceIndex() {
	s.buildWorkspaceIndexSync()
}

// buildWorkspaceIndexSync synchronously builds the workspace index.
// Exposed for testing purposes.
func (s *Server) buildWorkspaceIndexSync() {
	s.mu.RLock()
	rootURI := s.rootURI
	s.mu.RUnlock()

	if rootURI == "" {
		return
	}

	root := uriToPath(rootURI)
	log.Printf("building workspace index for: %s", root)

	// Create the index
	idx := index.New(root)

	// Discover and add all Starlark files
	count, errs := idx.AddPattern("//...")
	if len(errs) > 0 {
		for _, err := range errs {
			log.Printf("workspace index error: %v", err)
		}
	}

	log.Printf("indexed %d files", count)

	// Build workspace index from the file index
	wsIndex := NewWorkspaceIndex(root)

	for _, file := range idx.Files() {
		absPath := filepath.Join(root, file.Path)
		wsIndex.AddFile(file, absPath)
	}

	// Store the workspace index
	s.mu.Lock()
	s.workspace = wsIndex
	s.mu.Unlock()

	// Count total symbols
	wsIndex.mu.RLock()
	totalSymbols := 0
	for _, defs := range wsIndex.symbols {
		totalSymbols += len(defs)
	}
	wsIndex.mu.RUnlock()

	log.Printf("workspace index complete: %d symbols", totalSymbols)
}

// resolveLoadedSymbol attempts to resolve a symbol that was loaded from another file.
// It checks load statements in the current file and looks up the definition in the workspace index.
func (s *Server) resolveLoadedSymbol(word string, docURI string) *protocol.Location {
	s.mu.RLock()
	doc, ok := s.documents[docURI]
	wsIndex := s.workspace
	s.mu.RUnlock()

	if !ok || wsIndex == nil {
		return nil
	}

	path := uriToPath(docURI)

	// Parse the current file to find load statements
	file, err := parseStarlarkFile([]byte(doc.Content), path, filekind.KindStarlark)
	if err != nil {
		return nil
	}

	// Extract load statements using the index
	indexed := index.ExtractFile(file, path, filekind.KindStarlark)

	// Check each load statement for the symbol
	for _, load := range indexed.Loads {
		// Check if this load imports the symbol we're looking for
		for localName, exportedName := range load.Symbols {
			if localName == word {
				// Found the import - resolve the module path
				resolvedPath := wsIndex.ResolveLoadPath(load.Module, path)
				if resolvedPath == "" {
					return nil
				}

				// Look up the exported symbol in that file
				return wsIndex.FindDefinitionInFile(exportedName, resolvedPath)
			}
		}
	}

	return nil
}
