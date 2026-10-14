package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

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
			// From names are the exported symbols of another file, not
			// occurrences of a local name (and share a span with To when
			// there is no alias).
			for _, id := range e.From {
				skip[id] = true
			}
		}
	})

	// Pass 2: emit occurrences.
	li := newLineIndex(content)
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
					Character: uint32(li.utf16Col(start.Line-1, start.LineRune-1)),
				},
				End: protocol.Position{
					Line:      uint32(end.Line - 1),
					Character: uint32(li.utf16Col(end.Line-1, end.LineRune-1)),
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

// lineIndex splits content into lines once and caches, per line, the
// rune-column to UTF-16 mapping, so position conversion is linear in the
// file size even when a single line holds many positions.
type lineIndex struct {
	lines []string
	cols  map[int][]int // line -> UTF-16 offset of each rune index; nil entry = ASCII line
}

func newLineIndex(content string) *lineIndex {
	return &lineIndex{lines: strings.Split(content, "\n"), cols: map[int][]int{}}
}

// line returns the 0-based line without its "\r", or "" when out of range.
func (li *lineIndex) line(n int) string {
	if n < 0 || n >= len(li.lines) {
		return ""
	}
	return strings.TrimSuffix(li.lines[n], "\r")
}

// utf16Col converts a 0-based rune column on the 0-based line n to a UTF-16
// column. Columns past the end of the line count one unit per rune.
func (li *lineIndex) utf16Col(n, runeCol int) int {
	if runeCol <= 0 {
		return 0
	}
	offs, ok := li.cols[n]
	if !ok {
		text := li.line(n)
		ascii := true
		for i := 0; i < len(text); i++ {
			if text[i] >= utf8.RuneSelf {
				ascii = false
				break
			}
		}
		if !ascii {
			offs = make([]int, 0, len(text)+1)
			units := 0
			for _, r := range text {
				offs = append(offs, units)
				units += utf16Width(r)
			}
			offs = append(offs, units)
		}
		li.cols[n] = offs
	}
	if offs == nil {
		return runeCol
	}
	if runeCol < len(offs) {
		return offs[runeCol]
	}
	return offs[len(offs)-1] + (runeCol - (len(offs) - 1))
}

// bindingInfo describes every binding of a name in a file.
type bindingInfo struct {
	defs   []*build.DefStmt // def statements binding the name
	others int              // params, assignments, loop variables
	loads  int              // load() bindings
}

// collectBindings records, file-wide, how each name is bound. It is
// deliberately scope-insensitive: any second binding of a name makes the
// name ambiguous for callee resolution.
func collectBindings(file *build.File) map[string]*bindingInfo {
	info := map[string]*bindingInfo{}
	get := func(n string) *bindingInfo {
		if info[n] == nil {
			info[n] = &bindingInfo{}
		}
		return info[n]
	}
	var bind func(e build.Expr)
	bind = func(e build.Expr) {
		switch v := e.(type) {
		case *build.Ident:
			get(v.Name).others++
		case *build.ListExpr:
			for _, x := range v.List {
				bind(x)
			}
		case *build.TupleExpr:
			for _, x := range v.List {
				bind(x)
			}
		case *build.ParenExpr:
			bind(v.X)
		case *build.UnaryExpr:
			bind(v.X)
		case *build.AssignExpr:
			bind(v.LHS)
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
			case *build.CallExpr, *build.DefStmt, *build.LambdaExpr:
				return
			}
			if strings.HasSuffix(e.Op, "=") {
				bind(e.LHS)
			}
		case *build.DefStmt:
			b := get(e.Name)
			b.defs = append(b.defs, e)
			for _, p := range e.Params {
				bind(p)
			}
		case *build.LambdaExpr:
			for _, p := range e.Params {
				bind(p)
			}
		case *build.ForStmt:
			bind(e.Vars)
		case *build.ForClause:
			bind(e.Vars)
		case *build.LoadStmt:
			for _, id := range e.To {
				get(id.Name).loads++
			}
		}
	})
	return info
}

// hasNamedParam reports whether def has an ordinary (non *args/**kwargs)
// parameter called name, i.e. one that can be passed by keyword.
func hasNamedParam(def *build.DefStmt, name string) bool {
	for _, p := range def.Params {
		if _, variadic := p.(*build.UnaryExpr); variadic {
			continue
		}
		if paramName(p) == name {
			return true
		}
	}
	return false
}

// collectRenameRefs returns the references to targetName plus, when
// targetName is an ordinary parameter of a def, the keyword-argument labels
// (f(name=...)) of calls that resolve to that def with certainty.
//
// Policy: a call is resolved only when its callee is a bare identifier whose
// name is bound exactly once in the file, by a def. Anything else is never
// edited. If targetName is a keyword-capable parameter (of a def or a
// lambda) and some call passes targetName= to a bare-identifier callee that
// cannot be resolved this way (shadowed, reassigned, aliased, bound to a
// lambda or a parameter), the edit could silently break that call, so an
// error is returned and the caller must refuse the rename. Calls through
// attributes, loaded names and undefined globals are assumed to be foreign.
// Calls in other files cannot be seen and are not renamed.
func collectRenameRefs(content string, file *build.File, targetName string) ([]identRef, error) {
	refs := collectIdentRefs(content, file, targetName)

	bindings := collectBindings(file)
	paramOwned := false // targetName is a keyword-capable param of a def or lambda
	build.Walk(file, func(expr build.Expr, _ []build.Expr) {
		switch e := expr.(type) {
		case *build.DefStmt:
			if hasNamedParam(e, targetName) {
				paramOwned = true
			}
		case *build.LambdaExpr:
			for _, p := range e.Params {
				if _, variadic := p.(*build.UnaryExpr); !variadic && paramName(p) == targetName {
					paramOwned = true
				}
			}
		}
	})
	if !paramOwned {
		return refs, nil
	}

	li := newLineIndex(content)
	seen := map[[2]int]bool{}
	for _, r := range refs {
		seen[[2]int{int(r.Range.Start.Line), int(r.Range.Start.Character)}] = true
	}
	var unresolved string
	build.Walk(file, func(expr build.Expr, _ []build.Expr) {
		call, ok := expr.(*build.CallExpr)
		if !ok {
			return
		}
		fn, ok := call.X.(*build.Ident)
		if !ok {
			return
		}
		var labels []*build.Ident
		for _, arg := range call.List {
			if as, ok := arg.(*build.AssignExpr); ok {
				if id, ok := as.LHS.(*build.Ident); ok && id.Name == targetName {
					labels = append(labels, id)
				}
			}
		}
		if len(labels) == 0 {
			return
		}
		b := bindings[fn.Name]
		switch {
		case b == nil || (len(b.defs) == 0 && b.others == 0):
			return // builtin, undefined global or loaded name: not a def in this file
		case len(b.defs) == 1 && b.others == 0 && b.loads == 0:
			if !hasNamedParam(b.defs[0], targetName) {
				return // resolved to a def with a different signature
			}
		default:
			if unresolved == "" {
				unresolved = fn.Name
			}
			return
		}
		for _, id := range labels {
			start, end := id.Span()
			sc := uint32(li.utf16Col(start.Line-1, start.LineRune-1))
			ec := uint32(li.utf16Col(end.Line-1, end.LineRune-1))
			key := [2]int{start.Line - 1, int(sc)}
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, identRef{Range: protocol.Range{
				Start: protocol.Position{Line: uint32(start.Line - 1), Character: sc},
				End:   protocol.Position{Line: uint32(end.Line - 1), Character: ec},
			}})
		}
	})
	if unresolved != "" {
		return nil, fmt.Errorf("cannot safely rename parameter %q: the call %s(%s=...) cannot be resolved to a single function definition (it is shadowed, reassigned, aliased or a lambda), so its keyword argument could be left stale or wrongly changed", targetName, unresolved, targetName)
	}
	return refs, nil
}

// paramName returns the name bound by a def parameter expression
// (name, name=default, *name, **name).
func paramName(e build.Expr) string {
	switch v := e.(type) {
	case *build.Ident:
		return v.Name
	case *build.AssignExpr:
		return paramName(v.LHS)
	case *build.UnaryExpr:
		return paramName(v.X)
	}
	return ""
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
