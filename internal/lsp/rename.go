package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	refs := collectIdentRefs(doc.Content, file, word)
	if len(refs) == 0 {
		return nil, nil
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
	for start > 0 && isIdentChar(lineContent[start-1]) {
		start--
	}

	// Find end of word
	end := pos
	for end < len(lineContent) && isIdentChar(lineContent[end]) {
		end++
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

// isValidIdentifier reports whether name is a valid Starlark identifier.
func isValidIdentifier(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isIdentChar(name[i]) {
			return false
		}
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
