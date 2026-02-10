package lsp

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/docgen"
)

func (s *Server) handleHover(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.HoverParams
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

	log.Printf("hover: %s @ %d:%d -> %q", path, p.Position.Line, p.Position.Character, word)

	var markdown string

	// First, check builtins from provider
	if s.builtins != nil {
		markdown = s.getBuiltinHover(word, p.TextDocument.Uri)
	}

	// Fall back to document-defined symbols if not a builtin
	if markdown == "" {
		// Extract documentation
		moduleDoc, err := docgen.ExtractFile(path, []byte(doc.Content), docgen.Options{IncludePrivate: true})
		if err != nil {
			log.Printf("hover: docgen error: %v", err)
			return nil, nil
		}

		// Check functions
		for _, fn := range moduleDoc.Functions {
			if fn.Name == word {
				markdown = formatFunctionHover(fn)
				break
			}
		}

		// Check globals if not found in functions
		if markdown == "" {
			for _, g := range moduleDoc.Globals {
				if g.Name == word {
					markdown = formatGlobalHover(g)
					break
				}
			}
		}
	}

	if markdown == "" {
		return nil, nil // No documentation found
	}

	return &protocol.Hover{
		Contents: protocol.Or_ArrMarkedString_MarkedString_MarkupContent{Value: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: markdown},
		},
	}, nil
}

// --- Hover helpers ---

// formatFunctionHover formats a FunctionDoc as Markdown for hover display.
func formatFunctionHover(fn docgen.FunctionDoc) string {
	var b strings.Builder

	// Signature
	b.WriteString("```python\n")
	b.WriteString("def ")
	b.WriteString(fn.Name)
	b.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		if p.HasDefault {
			b.WriteString("=")
			b.WriteString(p.Default)
		}
	}
	b.WriteString(")\n```\n\n")

	// Documentation
	if fn.Parsed != nil && fn.Parsed.Summary != "" {
		b.WriteString(fn.Parsed.Summary)
		b.WriteString("\n")

		if fn.Parsed.Description != "" {
			b.WriteString("\n")
			b.WriteString(fn.Parsed.Description)
			b.WriteString("\n")
		}

		// Args
		if len(fn.Parsed.Args) > 0 {
			b.WriteString("\n**Args:**\n")
			for name, desc := range fn.Parsed.Args {
				b.WriteString("- `")
				b.WriteString(name)
				b.WriteString("`: ")
				b.WriteString(desc)
				b.WriteString("\n")
			}
		}

		// Returns
		if fn.Parsed.Returns != "" {
			b.WriteString("\n**Returns:** ")
			b.WriteString(fn.Parsed.Returns)
			b.WriteString("\n")
		}

		// Example
		if fn.Parsed.Example != "" {
			b.WriteString("\n**Example:**\n```python\n")
			b.WriteString(fn.Parsed.Example)
			b.WriteString("\n```\n")
		}
	} else if fn.Docstring != "" {
		// Raw docstring if not parsed
		b.WriteString(fn.Docstring)
		b.WriteString("\n")
	}

	return b.String()
}

// formatGlobalHover formats a GlobalDoc as Markdown for hover display.
func formatGlobalHover(g docgen.GlobalDoc) string {
	var b strings.Builder
	b.WriteString("```python\n")
	b.WriteString(g.Name)
	b.WriteString(" = ")
	b.WriteString(g.Value)
	b.WriteString("\n```\n")
	return b.String()
}

// getBuiltinHover returns hover markdown for a builtin symbol from the provider.
// Uses dialect/kind detection based on the document URI to return appropriate builtins.
func (s *Server) getBuiltinHover(word string, uri string) string {
	if s.builtins == nil {
		return ""
	}

	// Get dialect and file kind from URI
	dialect, kind := s.getDialectAndKind(uri)

	b, err := s.builtins.Builtins(dialect, kind)
	if err != nil {
		return ""
	}

	// Check builtin functions
	for _, fn := range b.Functions {
		if fn.Name == word {
			return formatBuiltinFunctionHover(fn)
		}
	}

	// Check builtin types
	for _, typ := range b.Types {
		if typ.Name == word {
			return formatBuiltinTypeHover(typ)
		}
	}

	// Check builtin globals
	for _, g := range b.Globals {
		if g.Name == word {
			return formatBuiltinGlobalHover(g)
		}
	}

	return ""
}

// formatBuiltinFunctionHover formats a builtin function signature for hover.
func formatBuiltinFunctionHover(fn builtins.Signature) string {
	var b strings.Builder

	// Signature
	b.WriteString("```python\n")
	b.WriteString(fn.Name)
	b.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		if p.Variadic {
			b.WriteString("*")
		}
		if p.KWArgs {
			b.WriteString("**")
		}
		b.WriteString(p.Name)
		if p.Type != "" {
			b.WriteString(": ")
			b.WriteString(p.Type)
		}
		if p.Default != "" {
			b.WriteString(" = ")
			b.WriteString(p.Default)
		}
	}
	b.WriteString(")")
	if fn.ReturnType != "" {
		b.WriteString(" -> ")
		b.WriteString(fn.ReturnType)
	}
	b.WriteString("\n```\n\n")

	// Documentation
	if fn.Doc != "" {
		b.WriteString(fn.Doc)
		b.WriteString("\n")
	}

	return b.String()
}

// formatBuiltinTypeHover formats a builtin type definition for hover.
func formatBuiltinTypeHover(typ builtins.TypeDef) string {
	var b strings.Builder

	// Type header
	b.WriteString("```python\n")
	b.WriteString("type ")
	b.WriteString(typ.Name)
	b.WriteString("\n```\n\n")

	// Documentation
	if typ.Doc != "" {
		b.WriteString(typ.Doc)
		b.WriteString("\n")
	}

	// Fields
	if len(typ.Fields) > 0 {
		b.WriteString("\n**Fields:**\n")
		for _, f := range typ.Fields {
			b.WriteString("- `")
			b.WriteString(f.Name)
			b.WriteString("`")
			if f.Type != "" {
				b.WriteString(": ")
				b.WriteString(f.Type)
			}
			if f.Doc != "" {
				b.WriteString(" - ")
				b.WriteString(f.Doc)
			}
			b.WriteString("\n")
		}
	}

	// Methods
	if len(typ.Methods) > 0 {
		b.WriteString("\n**Methods:**\n")
		for _, m := range typ.Methods {
			b.WriteString("- `")
			b.WriteString(m.Name)
			b.WriteString("()`")
			if m.Doc != "" {
				b.WriteString(" - ")
				b.WriteString(m.Doc)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// formatBuiltinGlobalHover formats a builtin global/constant for hover.
func formatBuiltinGlobalHover(g builtins.Field) string {
	var b strings.Builder

	// Global
	b.WriteString("```python\n")
	b.WriteString(g.Name)
	if g.Type != "" {
		b.WriteString(": ")
		b.WriteString(g.Type)
	}
	b.WriteString("\n```\n\n")

	// Documentation
	if g.Doc != "" {
		b.WriteString(g.Doc)
		b.WriteString("\n")
	}

	return b.String()
}
