package lsp

import (
	"context"
	"encoding/json"
	"log"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"go.lsp.dev/protocol"
)

// handleReferences returns all references to the symbol at the given position.
func (s *Server) handleReferences(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.ReferenceParams
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

	// Don't find references to keywords
	if isStarlarkKeyword(word) {
		return nil, nil
	}

	log.Printf("references: %s @ %d:%d -> %q", path, p.Position.Line, p.Position.Character, word)

	// Classify and parse the file
	cls := classifier.NewDefaultClassifier()
	classification, err := cls.Classify(path)
	if err != nil {
		classification.FileKind = filekind.KindStarlark
	}

	file, err := parseStarlarkFile([]byte(doc.Content), path, classification.FileKind)
	if err != nil {
		log.Printf("references: parse error: %v", err)
		return nil, nil
	}

	// Find all references to the symbol
	refs := findReferences(file, word, p.TextDocument.URI, p.Context.IncludeDeclaration)

	log.Printf("references: found %d references to %q", len(refs), word)

	return refs, nil
}

// isStarlarkKeyword returns true if the word is a Starlark keyword.
func isStarlarkKeyword(word string) bool {
	switch word {
	case "and", "break", "continue", "def", "elif", "else", "for", "if", "in",
		"lambda", "load", "not", "or", "pass", "return", "while":
		return true
	}
	return false
}

// findReferences finds all references to a symbol name in a file.
// If includeDeclaration is true, includes the definition site as well.
func findReferences(file *build.File, targetName string, uri protocol.DocumentURI, includeDeclaration bool) []protocol.Location {
	var refs []protocol.Location

	// Track declaration positions to optionally exclude them
	declarationLines := make(map[int]bool)

	// First, handle function definitions (DefStmt.Name is stored as string, not as Ident)
	for _, stmt := range file.Stmt {
		if def, ok := stmt.(*build.DefStmt); ok && def.Name == targetName {
			start, _ := def.Span()
			if includeDeclaration {
				// Add the function name as a reference
				// DefStmt.Span() gives us the "def" keyword position
				// We need to calculate where the function name is: after "def "
				// The name starts 4 characters after the "def" keyword
				nameStart := start.LineRune + 3 // "def" is 3 chars, plus 1 for space = position after space
				nameEnd := nameStart + len(targetName)
				refs = append(refs, protocol.Location{
					URI: uri,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(start.Line - 1),
							Character: uint32(nameStart),
						},
						End: protocol.Position{
							Line:      uint32(start.Line - 1),
							Character: uint32(nameEnd),
						},
					},
				})
			} else {
				declarationLines[start.Line] = true
			}
		}
	}

	// Find declaration positions for other declarations if we need to exclude them
	if !includeDeclaration {
		// Look for top-level assignments
		for _, stmt := range file.Stmt {
			if assign, ok := stmt.(*build.AssignExpr); ok {
				if ident, ok := assign.LHS.(*build.Ident); ok && ident.Name == targetName {
					start, _ := ident.Span()
					declarationLines[start.Line] = true
				}
			}
		}

		// Look for parameter definitions in functions
		for _, stmt := range file.Stmt {
			if def, ok := stmt.(*build.DefStmt); ok {
				for _, param := range def.Params {
					switch p := param.(type) {
					case *build.Ident:
						if p.Name == targetName {
							start, _ := p.Span()
							declarationLines[start.Line] = true
						}
					case *build.AssignExpr:
						if ident, ok := p.LHS.(*build.Ident); ok && ident.Name == targetName {
							start, _ := ident.Span()
							declarationLines[start.Line] = true
						}
					case *build.UnaryExpr:
						if ident, ok := p.X.(*build.Ident); ok && ident.Name == targetName {
							start, _ := ident.Span()
							declarationLines[start.Line] = true
						}
					}
				}
			}
		}
	}

	// Walk all expressions in the file to find identifier references
	build.Walk(file, func(expr build.Expr, stack []build.Expr) {
		if ident, ok := expr.(*build.Ident); ok {
			if ident.Name == targetName {
				start, end := ident.Span()

				// Skip declarations if requested
				if !includeDeclaration && declarationLines[start.Line] {
					return
				}

				refs = append(refs, protocol.Location{
					URI: uri,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(start.Line - 1),
							Character: uint32(start.LineRune - 1),
						},
						End: protocol.Position{
							Line:      uint32(end.Line - 1),
							Character: uint32(end.LineRune - 1),
						},
					},
				})
			}
		}
	})

	return refs
}
