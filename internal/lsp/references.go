package lsp

import (
	"context"
	"encoding/json"
	"log"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// handleReferences returns all references to the symbol at the given position.
func (s *Server) handleReferences(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.ReferenceParams
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
	refs := findReferencesIn(doc.Content, file, word, p.TextDocument.Uri, p.Context.IncludeDeclaration)

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

// identRef is one occurrence of an identifier in a file.
type identRef struct {
	Range protocol.Range
	// IsDecl is true when the occurrence introduces the name (def name,
	// parameter, assignment target, loop variable, load alias).
	IsDecl bool
}

// findReferences finds all references to a symbol name in a file.
// If includeDeclaration is true, includes the definition site as well.
// Columns are rune-based (no UTF-16 conversion); prefer findReferencesIn when
// the source text is available.
func findReferences(file *build.File, targetName string, uri string, includeDeclaration bool) []protocol.Location {
	return findReferencesIn("", file, targetName, uri, includeDeclaration)
}

// findReferencesIn is like findReferences but converts columns to UTF-16
// using content.
func findReferencesIn(content string, file *build.File, targetName string, uri string, includeDeclaration bool) []protocol.Location {
	var refs []protocol.Location
	for _, r := range collectIdentRefs(content, file, targetName) {
		if r.IsDecl && !includeDeclaration {
			continue
		}
		refs = append(refs, protocol.Location{Uri: uri, Range: r.Range})
	}
	return refs
}

// collectIdentRefs returns every identifier occurrence named targetName in
// file, at any nesting depth. Keyword-argument names in calls and attribute
// names (x.name) are not references and are excluded. Occurrences are
// identified by position, so a declaration never hides other uses on the
// same line.
func collectIdentRefs(content string, file *build.File, targetName string) []identRef {
	// Pass 1: classify identifiers by node identity.
	decl := map[*build.Ident]bool{}
	skip := map[*build.Ident]bool{}

	var markDecl func(e build.Expr)
	markDecl = func(e build.Expr) {
		switch v := e.(type) {
		case *build.Ident:
			decl[v] = true
		case *build.ListExpr:
			for _, x := range v.List {
				markDecl(x)
			}
		case *build.TupleExpr:
			for _, x := range v.List {
				markDecl(x)
			}
		case *build.ParenExpr:
			markDecl(v.X)
		case *build.UnaryExpr: // *args / **kwargs
			markDecl(v.X)
		case *build.AssignExpr: // param with default
			markDecl(v.LHS)
		}
	}

	build.Walk(file, func(expr build.Expr, stack []build.Expr) {
		var parent build.Expr
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		switch e := expr.(type) {
		case *build.AssignExpr:
			switch parent.(type) {
			case *build.CallExpr:
				if id, ok := e.LHS.(*build.Ident); ok {
					skip[id] = true // keyword argument name
				}
				return
			case *build.DefStmt, *build.LambdaExpr:
				return // handled with the parameters below
			}
			if e.Op == "=" {
				markDecl(e.LHS)
			}
		case *build.DefStmt:
			for _, p := range e.Params {
				markDecl(p)
			}
		case *build.LambdaExpr:
			for _, p := range e.Params {
				markDecl(p)
			}
		case *build.ForStmt:
			markDecl(e.Vars)
		case *build.ForClause:
			markDecl(e.Vars)
		case *build.LoadStmt:
			for _, id := range e.To {
				decl[id] = true
			}
		}
	})

	// Pass 2: emit occurrences.
	var refs []identRef
	seen := map[[2]int]bool{}
	add := func(start, end build.Position, isDecl bool) {
		key := [2]int{start.Line, start.LineRune}
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, identRef{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(start.Line - 1),
					Character: uint32(runeColToUTF16(lineOf(content, start.Line-1), start.LineRune-1)),
				},
				End: protocol.Position{
					Line:      uint32(end.Line - 1),
					Character: uint32(runeColToUTF16(lineOf(content, end.Line-1), end.LineRune-1)),
				},
			},
			IsDecl: isDecl,
		})
	}

	build.Walk(file, func(expr build.Expr, stack []build.Expr) {
		switch e := expr.(type) {
		case *build.DefStmt:
			if e.Name == targetName {
				start, end := defNamePosition(content, e)
				add(start, end, true)
			}
		case *build.Ident:
			if e.Name != targetName || skip[e] {
				return
			}
			start, end := e.Span()
			add(start, end, decl[e])
		}
	})

	return refs
}

// defNamePosition locates the name of a def statement. DefStmt.Name is a
// plain string, so the position is derived from the "def" keyword position.
func defNamePosition(content string, def *build.DefStmt) (start, end build.Position) {
	kw, _ := def.Span()
	i := kw.LineRune - 1 + len("def") // 0-based rune index just after "def"
	if content == "" {
		i++ // no text available: assume a single space
	} else {
		runes := []rune(lineOf(content, kw.Line-1))
		for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
	}
	nameLen := len([]rune(def.Name))
	start = build.Position{Line: kw.Line, LineRune: i + 1}
	end = build.Position{Line: kw.Line, LineRune: i + 1 + nameLen}
	return start, end
}
