package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"go.lsp.dev/protocol"
)

// handleSignatureHelp returns signature information for the function call at the cursor position.
func (s *Server) handleSignatureHelp(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.SignatureHelpParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parsing signature help params: %w", err)
	}

	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	// Find the call context at the cursor position
	callCtx := findCallContext(doc.Content, int(p.Position.Line), int(p.Position.Character))
	if callCtx == nil {
		return nil, nil // Not inside a function call
	}

	log.Printf("signatureHelp: function=%s, argIndex=%d", callCtx.FunctionName, callCtx.ArgumentIndex)

	// Try to find signature from builtins first
	sig := s.getBuiltinSignature(callCtx.FunctionName, p.TextDocument.URI)
	if sig != nil {
		return buildSignatureHelp(sig, callCtx.ArgumentIndex), nil
	}

	// Fall back to user-defined functions in the document
	sig = s.getDocumentFunctionSignature(doc.Content, callCtx.FunctionName, p.TextDocument.URI)
	if sig != nil {
		return buildSignatureHelp(sig, callCtx.ArgumentIndex), nil
	}

	return nil, nil // No signature found
}

// callContext represents the function call context at the cursor position.
type callContext struct {
	FunctionName  string
	ArgumentIndex int
}

// findCallContext analyzes the content to find the function call context at the given position.
// It returns nil if the cursor is not inside a function call.
func findCallContext(content string, line, char int) *callContext {
	// Convert line/char to absolute offset
	offset := lineCharToOffset(content, line, char)
	if offset < 0 || offset > len(content) {
		return nil
	}

	// Parse backwards from cursor to find the function call
	// We need to track parentheses depth and find the opening paren of the current call
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := false
	stringChar := byte(0)
	argCount := 0

	// Start from cursor position and go backwards
	for i := offset - 1; i >= 0; i-- {
		ch := content[i]

		// Handle string boundaries
		if !inString && (ch == '"' || ch == '\'') {
			// Check for triple quotes (go back 2 more chars)
			if i >= 2 && content[i-1] == ch && content[i-2] == ch {
				inString = true
				stringChar = ch
				i -= 2 // Skip past the triple quote
				continue
			}
			inString = true
			stringChar = ch
			continue
		}
		if inString {
			if ch == stringChar {
				// Check for triple quote ending
				if i >= 2 && content[i-1] == ch && content[i-2] == ch {
					inString = false
					i -= 2
					continue
				}
				// Check if escaped
				escapeCount := 0
				for j := i - 1; j >= 0 && content[j] == '\\'; j-- {
					escapeCount++
				}
				if escapeCount%2 == 0 {
					inString = false
				}
			}
			continue
		}

		// Track brackets and braces (for nested structures)
		switch ch {
		case ']':
			bracketDepth++
		case '[':
			bracketDepth--
			if bracketDepth < 0 {
				return nil // Unbalanced
			}
		case '}':
			braceDepth++
		case '{':
			braceDepth--
			if braceDepth < 0 {
				return nil // Unbalanced
			}
		case ')':
			parenDepth++
		case '(':
			if bracketDepth == 0 && braceDepth == 0 {
				parenDepth--
				if parenDepth < 0 {
					// Found the opening paren of our call
					funcName := extractFunctionName(content, i)
					if funcName == "" {
						return nil
					}
					return &callContext{
						FunctionName:  funcName,
						ArgumentIndex: argCount,
					}
				}
			}
		case ',':
			// Count commas at depth 0 to determine argument index
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				argCount++
			}
		}
	}

	return nil // Not inside a function call
}

// extractFunctionName extracts the function name before the opening paren at position i.
func extractFunctionName(content string, parenPos int) string {
	// Go backwards from paren to find the identifier
	start := parenPos - 1

	// Skip whitespace before paren
	for start >= 0 && (content[start] == ' ' || content[start] == '\t' || content[start] == '\n') {
		start--
	}
	end := start + 1

	// Find start of identifier (letters, digits, underscores)
	for start >= 0 && isIdentChar(content[start]) {
		start--
	}
	start++ // Move back to first char of identifier

	if start >= end {
		return ""
	}

	return content[start:end]
}

// lineCharToOffset converts line and character position to byte offset.
func lineCharToOffset(content string, line, char int) int {
	currentLine := 0
	offset := 0

	for i, ch := range content {
		if currentLine == line {
			// Found the target line, now count characters
			lineStart := i
			for j := 0; j < char && i+j < len(content); j++ {
				if content[i+j] == '\n' {
					return i + j // Clamp to end of line
				}
			}
			return lineStart + char
		}
		if ch == '\n' {
			currentLine++
		}
		offset = i + 1
	}

	// If we're on the last line
	if currentLine == line {
		return offset + char
	}

	return -1 // Line not found
}

// getBuiltinSignature returns the signature for a builtin function.
func (s *Server) getBuiltinSignature(name string, uri protocol.DocumentURI) *builtins.Signature {
	if s.builtins == nil {
		return nil
	}

	dialect, kind := s.getDialectAndKind(uri)
	b, err := s.builtins.Builtins(dialect, kind)
	if err != nil {
		return nil
	}

	// Check builtin functions
	for i := range b.Functions {
		if b.Functions[i].Name == name {
			return &b.Functions[i]
		}
	}

	// Also check types (they can be called as constructors)
	for i := range b.Types {
		if b.Types[i].Name == name {
			// Type constructors don't have explicit params, but some types have them
			// in the Functions list (like dict()). For now, return nil for pure types.
			continue
		}
	}

	return nil
}

// getDocumentFunctionSignature extracts signature for a function defined in the document.
func (s *Server) getDocumentFunctionSignature(content, name string, uri protocol.DocumentURI) *builtins.Signature {
	path := uriToPath(uri)

	// Classify the file
	cls := classifier.NewDefaultClassifier()
	classification, err := cls.Classify(path)
	if err != nil {
		classification.FileKind = filekind.KindStarlark
	}

	// For signature help, we need to handle incomplete code gracefully.
	// If parse fails, try parsing just the portion before the cursor position.
	// Alternatively, use a regex-based approach to find function definitions.
	file, err := parseStarlarkFile([]byte(content), path, classification.FileKind)
	if err != nil {
		// Try to parse a "fixed" version by closing any incomplete function calls
		fixedContent := content + ")"
		file, err = parseStarlarkFile([]byte(fixedContent), path, classification.FileKind)
		if err != nil {
			// Even that failed, try a minimal fix
			fixedContent = content + "None)"
			file, err = parseStarlarkFile([]byte(fixedContent), path, classification.FileKind)
			if err != nil {
				log.Printf("signatureHelp: parse error (even with fix): %v", err)
				return nil
			}
		}
	}

	// Find the function definition
	for _, stmt := range file.Stmt {
		def, ok := stmt.(*build.DefStmt)
		if !ok || def.Name != name {
			continue
		}

		// Build signature from DefStmt
		sig := &builtins.Signature{
			Name: def.Name,
		}

		// Extract parameters
		for _, param := range def.Params {
			p := builtins.Param{}
			switch pt := param.(type) {
			case *build.Ident:
				p.Name = pt.Name
				p.Required = true
			case *build.AssignExpr:
				if ident, ok := pt.LHS.(*build.Ident); ok {
					p.Name = ident.Name
					// Has default value
					p.Default = exprToString(pt.RHS)
				}
			case *build.UnaryExpr:
				if ident, ok := pt.X.(*build.Ident); ok {
					p.Name = ident.Name
					if pt.Op == "*" {
						p.Variadic = true
					} else if pt.Op == "**" {
						p.KWArgs = true
					}
				}
			}
			if p.Name != "" {
				sig.Params = append(sig.Params, p)
			}
		}

		// Extract docstring (first statement may be a StringExpr directly)
		if len(def.Body) > 0 {
			if str, ok := def.Body[0].(*build.StringExpr); ok {
				sig.Doc = str.Value
			}
		}

		return sig
	}

	return nil
}

// exprToString converts a build.Expr to a string representation for defaults.
func exprToString(expr build.Expr) string {
	switch e := expr.(type) {
	case *build.StringExpr:
		return fmt.Sprintf("%q", e.Value)
	case *build.LiteralExpr:
		return e.Token
	case *build.Ident:
		return e.Name
	case *build.ListExpr:
		return "[]"
	case *build.DictExpr:
		return "{}"
	default:
		return "..."
	}
}

// buildSignatureHelp creates a SignatureHelp response from a signature.
func buildSignatureHelp(sig *builtins.Signature, activeParam int) *protocol.SignatureHelp {
	// Build the label with parameters
	var label strings.Builder
	label.WriteString(sig.Name)
	label.WriteString("(")

	params := make([]protocol.ParameterInformation, 0, len(sig.Params))
	for i, p := range sig.Params {
		if i > 0 {
			label.WriteString(", ")
		}

		// Track parameter start position in label
		paramStart := label.Len()

		// Build parameter string
		if p.Variadic {
			label.WriteString("*")
		}
		if p.KWArgs {
			label.WriteString("**")
		}
		label.WriteString(p.Name)
		if p.Type != "" {
			label.WriteString(": ")
			label.WriteString(p.Type)
		}
		if p.Default != "" {
			label.WriteString("=")
			label.WriteString(p.Default)
		}

		// Create parameter info
		paramInfo := protocol.ParameterInformation{
			Label: label.String()[paramStart:],
		}
		params = append(params, paramInfo)
	}
	label.WriteString(")")

	if sig.ReturnType != "" {
		label.WriteString(" -> ")
		label.WriteString(sig.ReturnType)
	}

	// Build documentation
	var doc interface{}
	if sig.Doc != "" {
		doc = protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: sig.Doc,
		}
	}

	// Ensure activeParam is in bounds
	if activeParam >= len(params) && len(params) > 0 {
		// If we have more args than params, check for variadic/kwargs
		for i, p := range sig.Params {
			if p.Variadic || p.KWArgs {
				activeParam = i
				break
			}
		}
		// If still out of bounds, clamp to last param
		if activeParam >= len(params) {
			activeParam = len(params) - 1
		}
	}
	if activeParam < 0 {
		activeParam = 0
	}

	return &protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{
			{
				Label:         label.String(),
				Documentation: doc,
				Parameters:    params,
			},
		},
		ActiveSignature: 0,
		ActiveParameter: uint32(activeParam),
	}
}
