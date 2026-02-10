package lsp

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

func (s *Server) handleDocumentSymbol(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DocumentSymbolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()

	if !ok {
		return []protocol.DocumentSymbol{}, nil
	}

	path := uriToPath(p.TextDocument.Uri)
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
