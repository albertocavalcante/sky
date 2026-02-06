package lsp

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/albertocavalcante/sky/internal/protocol"
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

	// Find all references to this word
	refs := findAllReferences(doc.Content, word)
	if len(refs) == 0 {
		return nil, nil
	}

	// Create text edits for each reference
	edits := make([]protocol.TextEdit, 0, len(refs))
	for _, ref := range refs {
		edits = append(edits, protocol.TextEdit{
			Range:   ref,
			NewText: p.NewName,
		})
	}

	return &protocol.WorkspaceEdit{
		Changes: map[string][]protocol.TextEdit{
			p.TextDocument.Uri: edits,
		},
	}, nil
}

// getWordAndRangeAtPosition extracts the word and its range at a given line and character position.
func getWordAndRangeAtPosition(content string, line, char int) (string, protocol.Range) {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return "", protocol.Range{}
	}

	lineContent := lines[line]
	if char < 0 || char >= len(lineContent) {
		// Allow char == len for end of line
		if char != len(lineContent) {
			return "", protocol.Range{}
		}
	}

	// Find start of word
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
		return "", protocol.Range{}
	}

	word := lineContent[start:end]
	wordRange := protocol.Range{
		Start: protocol.Position{Line: uint32(line), Character: uint32(start)},
		End:   protocol.Position{Line: uint32(line), Character: uint32(end)},
	}

	return word, wordRange
}

// findAllReferences finds all occurrences of a word in the content.
// It returns ranges for each occurrence, ensuring only whole-word matches.
func findAllReferences(content, word string) []protocol.Range {
	var refs []protocol.Range
	lines := strings.Split(content, "\n")

	for lineNum, lineContent := range lines {
		// Find all occurrences in this line
		offset := 0
		for {
			idx := strings.Index(lineContent[offset:], word)
			if idx == -1 {
				break
			}

			absoluteIdx := offset + idx
			wordEnd := absoluteIdx + len(word)

			// Check if this is a whole word match (not part of a larger identifier)
			isWholeWord := true

			// Check character before
			if absoluteIdx > 0 && isIdentChar(lineContent[absoluteIdx-1]) {
				isWholeWord = false
			}

			// Check character after
			if wordEnd < len(lineContent) && isIdentChar(lineContent[wordEnd]) {
				isWholeWord = false
			}

			if isWholeWord {
				refs = append(refs, protocol.Range{
					Start: protocol.Position{Line: uint32(lineNum), Character: uint32(absoluteIdx)},
					End:   protocol.Position{Line: uint32(lineNum), Character: uint32(wordEnd)},
				})
			}

			offset = wordEnd
		}
	}

	return refs
}

// isKeyword returns true if the word is a Starlark keyword.
func isKeyword(word string) bool {
	return starlarkKeywordSet[word]
}

// isBuiltin returns true if the word is a Starlark builtin function.
func isBuiltin(word string) bool {
	return starlarkBuiltinSet[word]
}
