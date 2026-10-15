package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Starlark keywords that cannot be renamed
var starlarkKeywordSet = map[string]bool{
	"and": true, "break": true, "continue": true, "def": true, "elif": true,
	"else": true, "for": true, "if": true, "in": true, "lambda": true,
	"load": true, "not": true, "or": true, "pass": true, "return": true,
	"while": true, "True": true, "False": true, "None": true,
}

// Starlark builtins that cannot be renamed
var starlarkBuiltinSet = map[string]bool{
	"abs": true, "all": true, "any": true, "bool": true, "bytes": true,
	"dict": true, "dir": true, "enumerate": true, "fail": true, "float": true,
	"getattr": true, "hasattr": true, "hash": true, "int": true, "len": true,
	"list": true, "max": true, "min": true, "print": true, "range": true,
	"repr": true, "reversed": true, "sorted": true, "str": true, "tuple": true,
	"type": true, "zip": true,
}

// handlePrepareRename validates that a symbol at the given position can be renamed.
// Returns a Range if the symbol can be renamed, or nil if not.
func (s *Server) handlePrepareRename(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.PrepareRenameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}

	// Find the word at the cursor position
	word, wordRange := getWordAndRangeAtPosition(doc.Content, int(p.Position.Line), int(p.Position.Character))
	if word == "" {
		return nil, nil
	}

	log.Printf("prepareRename: word=%q at %d:%d", word, p.Position.Line, p.Position.Character)

	// Can't rename keywords
	if isKeyword(word) {
		log.Printf("prepareRename: %q is a keyword, cannot rename", word)
		return nil, nil
	}

	// Can't rename builtins
	if isBuiltin(word) {
		log.Printf("prepareRename: %q is a builtin, cannot rename", word)
		return nil, nil
	}

	return &wordRange, nil
}

// handleRename finds all references to a symbol and returns a WorkspaceEdit to rename them.
func (s *Server) handleRename(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.RenameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}

	// Find the word at the cursor position
	word, _ := getWordAndRangeAtPosition(doc.Content, int(p.Position.Line), int(p.Position.Character))
	if word == "" {
		return nil, nil
	}

	log.Printf("rename: word=%q to %q", word, p.NewName)

	// Can't rename keywords or builtins
	if isKeyword(word) || isBuiltin(word) {
		return nil, nil
	}

	if !isValidIdentifier(p.NewName) || isKeyword(p.NewName) {
		return nil, &ResponseError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("%q is not a valid identifier", p.NewName),
		}
	}

	// Find all references using the syntax tree, so occurrences inside
	// strings and comments, keyword-argument names and attribute names are
	// never touched.
	path := uriToPath(p.TextDocument.Uri)
	file, err := parseStarlarkFile([]byte(doc.Content), path, filekind.KindStarlark)
	if err != nil {
		return nil, &ResponseError{
			Code:    CodeRequestFailed,
			Message: "cannot rename: file has syntax errors",
		}
	}
	refs, err := collectRenameRefs(doc.Content, file, word)
	if err != nil {
		return nil, &ResponseError{Code: CodeRequestFailed, Message: err.Error()}
	}
	if len(refs) == 0 {
		return nil, nil
	}
	if p.NewName != word {
		if msg := renameCapture(file, word, p.NewName); msg != "" {
			return nil, &ResponseError{Code: CodeRequestFailed, Message: msg}
		}
	}

	// Create text edits for each reference
	edits := make([]protocol.TextEdit, 0, len(refs))
	for _, ref := range refs {
		edits = append(edits, protocol.TextEdit{
			Range:   ref.Range,
			NewText: p.NewName,
		})
	}

	return &protocol.WorkspaceEdit{
		Changes: map[string][]protocol.TextEdit{
			p.TextDocument.Uri: edits,
		},
	}, nil
}

// getWordAndRangeAtPosition extracts the word and its range at a given line
// and character (UTF-16) position. A cursor at the end of a word selects it.
func getWordAndRangeAtPosition(content string, line, char int) (string, protocol.Range) {
	lineContent, ok := lineText(content, line)
	if !ok || char < 0 {
		return "", protocol.Range{}
	}
	lineContent = strings.TrimSuffix(lineContent, "\r")
	if char > utf16Len(lineContent) {
		return "", protocol.Range{}
	}
	pos := utf16ColToByte(lineContent, char)

	// Find start of word
	start := pos
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(lineContent[:start])
		if !isIdentRune(r) {
			break
		}
		start -= size
	}

	// Find end of word
	end := pos
	for end < len(lineContent) {
		r, size := utf8.DecodeRuneInString(lineContent[end:])
		if !isIdentRune(r) {
			break
		}
		end += size
	}

	if start == end {
		return "", protocol.Range{}
	}

	word := lineContent[start:end]
	wordRange := protocol.Range{
		Start: protocol.Position{Line: uint32(line), Character: uint32(byteToUTF16Col(lineContent, start))},
		End:   protocol.Position{Line: uint32(line), Character: uint32(byteToUTF16Col(lineContent, end))},
	}

	return word, wordRange
}

// isIdentRune reports whether r can appear in an identifier, using the same
// rules as isValidIdentifier.
func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || (r >= '0' && r <= '9')
}

// starlarkReservedSet holds the words the Starlark scanner reserves (they
// are not valid identifiers) beyond the language keywords.
var starlarkReservedSet = map[string]bool{
	"as": true, "async": true, "await": true, "class": true, "del": true,
	"except": true, "finally": true, "from": true, "global": true,
	"import": true, "is": true, "nonlocal": true, "raise": true, "try": true,
	"with": true, "yield": true,
}

// isValidIdentifier reports whether name is a valid Starlark identifier:
// a Unicode letter or '_' followed by letters, digits or '_', and not a
// keyword or reserved word.
func isValidIdentifier(name string) bool {
	if name == "" || starlarkKeywordSet[name] || starlarkReservedSet[name] {
		return false
	}
	for i, r := range name {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// isKeyword returns true if the word is a Starlark keyword.
func isKeyword(word string) bool {
	return starlarkKeywordSet[word]
}

// isBuiltin returns true if the word is a Starlark builtin function.
func isBuiltin(word string) bool {
	return starlarkBuiltinSet[word]
}

// renameCapture reports (as a non-empty message) when renaming oldName to
// newName could change what a name refers to: newName is already bound in a
// scope between a use of oldName and the scope that binds oldName (the
// rename would merge two variables or make the inner binding shadow it), or
// newName is used in the scope that binds oldName and would start resolving
// to the renamed variable. The check is name-based and conservative: it never
// resolves which of several same-named variables a use belongs to.
func renameCapture(file *build.File, oldName, newName string) string {
	_, skip := classifyIdents(file)

	isScope := func(e build.Expr) bool {
		switch e.(type) {
		case *build.DefStmt, *build.LambdaExpr, *build.Comprehension:
			return true
		}
		return false
	}
	// chain returns the scopes enclosing a node, innermost first, ending
	// with the module (the file).
	chain := func(stack []build.Expr) []any {
		out := []any{}
		for i := len(stack) - 1; i >= 0; i-- {
			if isScope(stack[i]) {
				out = append(out, stack[i])
			}
		}
		return append(out, file)
	}

	binds := map[any]map[string]bool{}
	bind := func(scope any, name string) {
		if binds[scope] == nil {
			binds[scope] = map[string]bool{}
		}
		binds[scope][name] = true
	}
	var bindPattern func(scope any, e build.Expr)
	bindPattern = func(scope any, e build.Expr) {
		switch v := e.(type) {
		case *build.Ident:
			bind(scope, v.Name)
		case *build.ListExpr:
			for _, x := range v.List {
				bindPattern(scope, x)
			}
		case *build.TupleExpr:
			for _, x := range v.List {
				bindPattern(scope, x)
			}
		case *build.ParenExpr:
			bindPattern(scope, v.X)
		case *build.UnaryExpr:
			bindPattern(scope, v.X)
		case *build.AssignExpr:
			bindPattern(scope, v.LHS)
		}
	}

	type occurrence struct {
		name  string
		scope []any
	}
	var occs []occurrence
	note := func(name string, stack []build.Expr) {
		if name == oldName || name == newName {
			occs = append(occs, occurrence{name, chain(stack)})
		}
	}

	build.Walk(file, func(expr build.Expr, stack []build.Expr) {
		outer := chain(stack)[0]
		var parent build.Expr
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		switch e := expr.(type) {
		case *build.Ident:
			if !skip[e] {
				note(e.Name, stack)
			}
		case *build.DefStmt:
			bind(outer, e.Name)
			note(e.Name, stack)
			for _, p := range e.Params {
				bindPattern(e, p)
			}
		case *build.LambdaExpr:
			for _, p := range e.Params {
				bindPattern(e, p)
			}
		case *build.AssignExpr:
			switch parent.(type) {
			case *build.CallExpr, *build.DefStmt, *build.LambdaExpr:
				return
			}
			if strings.HasSuffix(e.Op, "=") {
				bindPattern(outer, e.LHS)
			}
		case *build.ForStmt:
			bindPattern(outer, e.Vars)
		case *build.ForClause:
			bindPattern(outer, e.Vars)
		case *build.LoadStmt:
			for _, id := range e.To {
				bind(outer, id.Name)
			}
		}
	})

	// Scopes whose binding of oldName the rename moves to newName: the
	// innermost binder of each use, or the module for a free name.
	owners := map[any]bool{}
	for _, o := range occs {
		if o.name != oldName {
			continue
		}
		owner := any(file)
		for _, sc := range o.scope {
			if binds[sc][oldName] {
				owner = sc
				break
			}
		}
		owners[owner] = true
		for _, sc := range o.scope {
			if binds[sc][newName] {
				return fmt.Sprintf("cannot rename %q to %q: %q is already bound in a scope where %q is used, so the rename would change which variable a name refers to", oldName, newName, newName, oldName)
			}
			if sc == owner {
				break
			}
		}
	}
	for _, o := range occs {
		if o.name != newName {
			continue
		}
		for _, sc := range o.scope {
			if binds[sc][newName] {
				break // resolves to its own binding, unaffected
			}
			if owners[sc] {
				return fmt.Sprintf("cannot rename %q to %q: %q is already used where %q is bound, so the renamed variable would capture it", oldName, newName, newName, oldName)
			}
		}
	}
	return ""
}
