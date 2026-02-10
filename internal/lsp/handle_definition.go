package lsp

import (
	"context"
	"encoding/json"
	"log"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/query/index"
)

func (s *Server) handleDefinition(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.DefinitionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	path := uriToPath(p.TextDocument.Uri)

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

	// Check load statements for imported symbols - try cross-file resolution
	if defLine == 0 {
		for _, load := range indexed.Loads {
			for localName, exportedName := range load.Symbols {
				if localName == word {
					// Try to resolve to actual definition via workspace index
					if loc := s.resolveLoadedSymbol(word, p.TextDocument.Uri); loc != nil {
						return []protocol.Location{*loc}, nil
					}
					// Fall back to the load statement line if we can't resolve
					// Try to resolve the module path
					s.mu.RLock()
					wsIndex := s.workspace
					s.mu.RUnlock()
					if wsIndex != nil {
						resolvedPath := wsIndex.ResolveLoadPath(load.Module, path)
						if resolvedPath != "" {
							// Look up the exported symbol in that file
							if loc := wsIndex.FindDefinitionInFile(exportedName, resolvedPath); loc != nil {
								return []protocol.Location{*loc}, nil
							}
						}
					}
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
			Uri:   p.TextDocument.Uri,
			Range: lineToRange(defLine),
		},
	}, nil
}
