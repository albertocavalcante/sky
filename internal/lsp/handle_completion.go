package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/builtins"
)

func (s *Server) handleCompletion(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parsing completion params: %w", err)
	}

	// Copy document content while holding lock to avoid race conditions
	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	var content string
	var docURI string
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
			s.getProviderBuiltinCompletions(prefix, p.TextDocument.Uri),
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
func (s *Server) getProviderBuiltinCompletions(prefix string, uri string) []protocol.CompletionItem {
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
