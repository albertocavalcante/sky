package lsp

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/types"
)

// InlayHintConfig controls which inlay hints are displayed.
type InlayHintConfig struct {
	ShowVariableTypes  bool // Show types for variables
	ShowParameterTypes bool // Show types for function parameters
	HideForSingleChar  bool // Hide hints for single-character variables
	MaxHintLength      int  // Truncate long type names (0 = no limit)
}

// DefaultInlayHintConfig returns the default configuration.
var DefaultInlayHintConfig = InlayHintConfig{
	ShowVariableTypes:  true,
	ShowParameterTypes: true,
	HideForSingleChar:  true,
	MaxHintLength:      50,
}

// handleInlayHint handles textDocument/inlayHint requests.
func (s *Server) handleInlayHint(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.InlayHintParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// Copy document content while holding lock
	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.Uri]
	var content string
	if ok {
		content = doc.Content
	}
	s.mu.RUnlock()

	if !ok {
		return []protocol.InlayHint{}, nil
	}

	log.Printf("inlayHint: %s [%d:%d - %d:%d]",
		p.TextDocument.Uri,
		p.Range.Start.Line, p.Range.Start.Character,
		p.Range.End.Line, p.Range.End.Character)

	// Parse the file
	file, err := build.ParseDefault("", []byte(content))
	if err != nil {
		log.Printf("inlayHint parse error: %v", err)
		return []protocol.InlayHint{}, nil
	}

	// Collect inlay hints
	collector := newInlayHintCollector(content, p.Range, DefaultInlayHintConfig)
	hints := collector.collect(file)

	log.Printf("inlayHint: collected %d hints", len(hints))
	return hints, nil
}

// inlayHintCollector collects inlay hints from a parsed file.
type inlayHintCollector struct {
	content string
	lines   []string
	rng     protocol.Range
	config  InlayHintConfig
	hints   []protocol.InlayHint

	// Track defined variables to avoid duplicate hints
	defined map[string]bool
}

func newInlayHintCollector(content string, rng protocol.Range, config InlayHintConfig) *inlayHintCollector {
	return &inlayHintCollector{
		content: content,
		lines:   strings.Split(content, "\n"),
		rng:     rng,
		config:  config,
		hints:   []protocol.InlayHint{},
		defined: make(map[string]bool),
	}
}

func (c *inlayHintCollector) collect(file *build.File) []protocol.InlayHint {
	// Process all statements
	for _, stmt := range file.Stmt {
		c.collectStmt(stmt)
	}
	return c.hints
}

func (c *inlayHintCollector) collectStmt(stmt build.Expr) {
	switch s := stmt.(type) {
	case *build.AssignExpr:
		c.collectAssignment(s)

	case *build.DefStmt:
		c.collectFunction(s)

	case *build.ForStmt:
		c.collectFor(s)

	case *build.IfStmt:
		// Process both branches
		for _, stmt := range s.True {
			c.collectStmt(stmt)
		}
		for _, stmt := range s.False {
			c.collectStmt(stmt)
		}

	case *build.ReturnStmt:
		// No hints for return statements

	case *build.CallExpr:
		// Standalone call expressions don't need type hints

	case *build.BranchStmt:
		// pass, break, continue - no hints
	}
}

// collectAssignment processes an assignment and potentially adds a type hint.
func (c *inlayHintCollector) collectAssignment(assign *build.AssignExpr) {
	// Handle simple variable assignment
	ident, ok := assign.LHS.(*build.Ident)
	if !ok {
		// Could be tuple unpacking or subscript assignment
		return
	}

	// Skip if already defined (only hint first assignment)
	if c.defined[ident.Name] {
		return
	}
	c.defined[ident.Name] = true

	// Get position
	_, endPos := ident.Span()
	if !c.inRange(endPos) {
		return
	}

	// Skip short variable names if configured
	if c.config.HideForSingleChar && len(ident.Name) == 1 {
		return
	}

	// First, check for explicit type comment
	typeRef := c.getTypeComment(assign)

	// If no explicit type, try inference
	if typeRef == nil {
		typeRef = types.InferExprType(assign.RHS)
	}

	// Skip if unknown
	if typeRef.IsUnknown() {
		return
	}

	c.addTypeHint(endPos, typeRef)
}

// collectFunction processes a function definition.
func (c *inlayHintCollector) collectFunction(def *build.DefStmt) {
	if !c.config.ShowParameterTypes {
		// Process body even if not showing parameter hints
		for _, stmt := range def.Body {
			c.collectStmt(stmt)
		}
		return
	}

	// Check for function type comment in the body
	funcType := c.getFunctionTypeComment(def)

	// Get parameter types from docstring
	docInfo := c.getDocstringInfo(def)

	// Process parameters
	for i, param := range def.Params {
		var paramType types.TypeRef

		// Priority: type comment > docstring
		if funcType != nil && i < len(funcType.Params) && funcType.Params[i].Type != nil {
			paramType = funcType.Params[i].Type
		} else if docInfo != nil {
			paramName := getParamName(param)
			if pdoc, ok := docInfo[paramName]; ok {
				paramType = pdoc
			}
		}

		// Add hint for parameter if we have a type
		if paramType != nil && !paramType.IsUnknown() {
			c.addParameterTypeHint(param, paramType)
		}
	}

	// Process function body
	for _, stmt := range def.Body {
		c.collectStmt(stmt)
	}
}

// collectFor processes a for loop.
func (c *inlayHintCollector) collectFor(f *build.ForStmt) {
	// Try to infer loop variable type from iterable
	iterType := types.InferExprType(f.X)
	elemType := types.ElementType(iterType)

	if elemType != nil && !elemType.IsUnknown() {
		// Add hints for loop variables
		c.collectLoopVarsHint(f.Vars, elemType)
	}

	// Process body
	for _, stmt := range f.Body {
		c.collectStmt(stmt)
	}
}

// collectLoopVarsHint adds hints for loop variables.
func (c *inlayHintCollector) collectLoopVarsHint(vars build.Expr, elemType types.TypeRef) {
	switch v := vars.(type) {
	case *build.Ident:
		if c.config.HideForSingleChar && len(v.Name) == 1 {
			return
		}
		if c.defined[v.Name] {
			return
		}
		c.defined[v.Name] = true

		_, endPos := v.Span()
		if c.inRange(endPos) {
			c.addTypeHint(endPos, elemType)
		}

	case *build.TupleExpr:
		// Tuple unpacking - we can't easily infer individual element types
		// unless elemType is a tuple with matching arity
		if named, ok := elemType.(*types.NamedType); ok && named.Name == "tuple" {
			for i, elem := range v.List {
				if i < len(named.Args) {
					c.collectLoopVarsHint(elem, named.Args[i])
				}
			}
		}
	}
}

// addTypeHint adds a type hint at the given position.
func (c *inlayHintCollector) addTypeHint(pos build.Position, typeRef types.TypeRef) {
	label := ": " + c.formatType(typeRef)

	c.hints = append(c.hints, protocol.InlayHint{
		Position: protocol.Position{
			Line:      uint32(pos.Line - 1),
			Character: uint32(pos.LineRune - 1),
		},
		Label:        protocol.Or_ArrInlayHintLabelPart_string{Value: label},
		Kind:         protocol.InlayHintKindType,
		PaddingLeft:  false,
		PaddingRight: true,
	})
}

// addParameterTypeHint adds a type hint for a function parameter.
func (c *inlayHintCollector) addParameterTypeHint(param build.Expr, typeRef types.TypeRef) {
	var endPos build.Position

	switch p := param.(type) {
	case *build.Ident:
		if c.config.HideForSingleChar && len(p.Name) == 1 {
			return
		}
		_, endPos = p.Span()

	case *build.AssignExpr:
		// param = default
		if ident, ok := p.LHS.(*build.Ident); ok {
			if c.config.HideForSingleChar && len(ident.Name) == 1 {
				return
			}
			_, endPos = ident.Span()
		} else {
			return
		}

	case *build.UnaryExpr:
		// *args or **kwargs
		if ident, ok := p.X.(*build.Ident); ok {
			_, endPos = ident.Span()
		} else {
			return
		}

	default:
		return
	}

	if !c.inRange(endPos) {
		return
	}

	c.addTypeHint(endPos, typeRef)
}

// formatType formats a type for display, potentially truncating if too long.
func (c *inlayHintCollector) formatType(typeRef types.TypeRef) string {
	s := typeRef.String()
	if c.config.MaxHintLength > 0 && len(s) > c.config.MaxHintLength {
		return s[:c.config.MaxHintLength-3] + "..."
	}
	return s
}

// inRange checks if a position is within the requested range.
func (c *inlayHintCollector) inRange(pos build.Position) bool {
	line := uint32(pos.Line - 1) // Convert to 0-based
	return line >= c.rng.Start.Line && line <= c.rng.End.Line
}

// getTypeComment looks for a type comment on the line of an expression.
func (c *inlayHintCollector) getTypeComment(expr build.Expr) types.TypeRef {
	comments := expr.Comment()

	// Check suffix comments (inline comments)
	for _, comment := range comments.Suffix {
		if typeRef, err := types.ParseTypeComment(comment.Token); err == nil {
			return typeRef
		}
	}

	return nil
}

// getFunctionTypeComment looks for a function type comment.
// This is typically the first statement in the body as a comment.
func (c *inlayHintCollector) getFunctionTypeComment(def *build.DefStmt) *types.FunctionType {
	// Check if the first statement has a type comment
	if len(def.Body) == 0 {
		return nil
	}

	// Look for a type comment in the function's comments
	comments := def.Comment()
	for _, comment := range comments.After {
		if ft, err := types.ParseFunctionTypeComment(comment.Token); err == nil {
			return ft
		}
	}

	// Also check the first statement's before comments
	if len(def.Body) > 0 {
		bodyComments := def.Body[0].Comment()
		for _, comment := range bodyComments.Before {
			if ft, err := types.ParseFunctionTypeComment(comment.Token); err == nil {
				return ft
			}
		}
	}

	return nil
}

// getDocstringInfo extracts parameter types from a docstring.
// Returns a map of parameter name to type.
func (c *inlayHintCollector) getDocstringInfo(def *build.DefStmt) map[string]types.TypeRef {
	if len(def.Body) == 0 {
		return nil
	}

	// Check if first statement is a string (docstring)
	var docstring string
	switch s := def.Body[0].(type) {
	case *build.StringExpr:
		docstring = s.Value
	default:
		return nil
	}

	// Parse Google-style docstring Args section
	return parseDocstringArgs(docstring)
}

// parseDocstringArgs extracts parameter types from a Google-style docstring.
func parseDocstringArgs(docstring string) map[string]types.TypeRef {
	result := make(map[string]types.TypeRef)

	lines := strings.Split(docstring, "\n")
	inArgs := false
	argsIndent := -1 // Track the indentation level of Args:

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect section headers
		if trimmed == "Args:" {
			inArgs = true
			// Calculate indent of Args:
			argsIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			continue
		}
		if trimmed == "Returns:" || trimmed == "Raises:" || trimmed == "Example:" ||
			trimmed == "Examples:" || trimmed == "Note:" || trimmed == "Yields:" {
			inArgs = false
			continue
		}

		// Parse parameter lines in Args section
		// Parameter lines should be indented more than Args:
		if inArgs && argsIndent >= 0 && len(line) > 0 {
			lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))

			// Parameter lines are typically 4 spaces more than Args:
			// They should have content (not just whitespace)
			if lineIndent > argsIndent && trimmed != "" {
				// Check this is a parameter line (has colon) not a continuation
				if strings.Contains(trimmed, ":") {
					// Extract parameter name and type
					name, typeStr := parseParamLine(trimmed)
					if name != "" && typeStr != "" {
						// Try to parse the type
						if typeRef, err := types.ParseTypeString(typeStr); err == nil {
							result[name] = typeRef
						}
					}
				}
			}
		}
	}

	return result
}

// parseParamLine parses a docstring parameter line.
// Returns the parameter name and type string.
func parseParamLine(line string) (name, typeStr string) {
	// Format 1: "name (type): description"
	// Format 2: "name (type, optional): description"

	colonIdx := strings.Index(line, ":")
	if colonIdx == -1 {
		return "", ""
	}

	nameAndType := strings.TrimSpace(line[:colonIdx])

	// Check for type in parentheses
	parenStart := strings.Index(nameAndType, "(")
	parenEnd := strings.LastIndex(nameAndType, ")")

	if parenStart != -1 && parenEnd != -1 && parenEnd > parenStart {
		name = strings.TrimSpace(nameAndType[:parenStart])
		typeStr = strings.TrimSpace(nameAndType[parenStart+1 : parenEnd])

		// Remove "optional" suffix if present
		typeStr = strings.TrimSuffix(typeStr, ", optional")
		typeStr = strings.TrimSuffix(typeStr, ",optional")
		typeStr = strings.TrimSpace(typeStr)
	} else {
		// No type specified
		name = nameAndType
	}

	return name, typeStr
}

// getParamName extracts the parameter name from a parameter expression.
func getParamName(param build.Expr) string {
	switch p := param.(type) {
	case *build.Ident:
		return p.Name
	case *build.AssignExpr:
		if ident, ok := p.LHS.(*build.Ident); ok {
			return ident.Name
		}
	case *build.UnaryExpr:
		if ident, ok := p.X.(*build.Ident); ok {
			return ident.Name
		}
	}
	return ""
}
