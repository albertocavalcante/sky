package lsp

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bazelbuild/buildtools/build"
	"go.lsp.dev/protocol"
)

// handleSemanticTokensFull handles textDocument/semanticTokens/full requests.
func (s *Server) handleSemanticTokensFull(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.SemanticTokensParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// Copy content while holding lock to avoid data race
	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	var content string
	if ok {
		content = doc.Content
	}
	s.mu.RUnlock()

	if !ok {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}

	log.Printf("semanticTokens/full: %s", p.TextDocument.URI)

	// Tokenize the content
	tokens := tokenizeContent(content)

	// Encode to LSP format
	encoded := encodeTokens(tokens)

	return &protocol.SemanticTokens{
		Data: encoded,
	}, nil
}

// handleSemanticTokensRange handles textDocument/semanticTokens/range requests.
func (s *Server) handleSemanticTokensRange(ctx context.Context, params json.RawMessage) (any, error) {
	var p protocol.SemanticTokensRangeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// Copy content while holding lock to avoid data race
	s.mu.RLock()
	doc, ok := s.documents[p.TextDocument.URI]
	var content string
	if ok {
		content = doc.Content
	}
	s.mu.RUnlock()

	if !ok {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}

	log.Printf("semanticTokens/range: %s [%d:%d - %d:%d]",
		p.TextDocument.URI,
		p.Range.Start.Line, p.Range.Start.Character,
		p.Range.End.Line, p.Range.End.Character)

	// Tokenize the content
	tokens := tokenizeContent(content)

	// Filter to range
	filtered := filterTokensInRange(tokens, p.Range)

	// Encode to LSP format
	encoded := encodeTokens(filtered)

	return &protocol.SemanticTokens{
		Data: encoded,
	}, nil
}

// tokenizeContent parses Starlark content and returns semantic tokens.
func tokenizeContent(content string) []SemanticToken {
	var tokens []SemanticToken

	// Parse the file
	file, err := build.ParseDefault("", []byte(content))
	if err != nil {
		// Even with errors, try to tokenize what we can
		// For now, just return empty
		log.Printf("semantic tokenize parse error: %v", err)
		return tokens
	}

	// Build global scope
	globalScope := NewScope(nil)

	// First pass: collect function definitions and imports (not assignments)
	// We don't pre-define assignments so we can mark first occurrence as declaration
	for _, stmt := range file.Stmt {
		switch s := stmt.(type) {
		case *build.DefStmt:
			globalScope.Define(s.Name, SymbolGlobal)
		case *build.LoadStmt:
			for _, to := range s.To {
				globalScope.Define(to.Name, SymbolImported)
			}
		}
	}

	// Collect comments from all statements
	tokens = append(tokens, collectComments(file.Stmt)...)

	// Second pass: tokenize all statements
	for _, stmt := range file.Stmt {
		tokens = append(tokens, tokenizeStmt(stmt, globalScope, content)...)
	}

	// Sort tokens by position (line, then character)
	sortTokens(tokens)

	return tokens
}

// collectComments recursively collects all comments from statements.
func collectComments(stmts []build.Expr) []SemanticToken {
	var tokens []SemanticToken

	for _, stmt := range stmts {
		tokens = append(tokens, collectCommentsFromExpr(stmt)...)
	}

	return tokens
}

// collectCommentsFromExpr collects comments from an expression and its children.
func collectCommentsFromExpr(expr build.Expr) []SemanticToken {
	var tokens []SemanticToken
	if expr == nil {
		return tokens
	}

	// Get comments attached to this expression
	comments := expr.Comment()

	// Before comments
	for _, c := range comments.Before {
		tokens = append(tokens, tokenizeComment(c)...)
	}

	// Suffix comments (inline comments)
	for _, c := range comments.Suffix {
		tokens = append(tokens, tokenizeComment(c)...)
	}

	// After comments
	for _, c := range comments.After {
		tokens = append(tokens, tokenizeComment(c)...)
	}

	// Recursively collect from children based on expression type
	switch e := expr.(type) {
	case *build.DefStmt:
		for _, param := range e.Params {
			tokens = append(tokens, collectCommentsFromExpr(param)...)
		}
		for _, stmt := range e.Body {
			tokens = append(tokens, collectCommentsFromExpr(stmt)...)
		}
	case *build.ForStmt:
		tokens = append(tokens, collectCommentsFromExpr(e.Vars)...)
		tokens = append(tokens, collectCommentsFromExpr(e.X)...)
		for _, stmt := range e.Body {
			tokens = append(tokens, collectCommentsFromExpr(stmt)...)
		}
	case *build.IfStmt:
		tokens = append(tokens, collectCommentsFromExpr(e.Cond)...)
		for _, stmt := range e.True {
			tokens = append(tokens, collectCommentsFromExpr(stmt)...)
		}
		for _, stmt := range e.False {
			tokens = append(tokens, collectCommentsFromExpr(stmt)...)
		}
	case *build.CallExpr:
		tokens = append(tokens, collectCommentsFromExpr(e.X)...)
		for _, arg := range e.List {
			tokens = append(tokens, collectCommentsFromExpr(arg)...)
		}
	case *build.AssignExpr:
		tokens = append(tokens, collectCommentsFromExpr(e.LHS)...)
		tokens = append(tokens, collectCommentsFromExpr(e.RHS)...)
	case *build.BinaryExpr:
		tokens = append(tokens, collectCommentsFromExpr(e.X)...)
		tokens = append(tokens, collectCommentsFromExpr(e.Y)...)
	case *build.ListExpr:
		for _, elem := range e.List {
			tokens = append(tokens, collectCommentsFromExpr(elem)...)
		}
	case *build.DictExpr:
		for _, entry := range e.List {
			tokens = append(tokens, collectCommentsFromExpr(entry.Key)...)
			tokens = append(tokens, collectCommentsFromExpr(entry.Value)...)
		}
	case *build.ReturnStmt:
		if e.Result != nil {
			tokens = append(tokens, collectCommentsFromExpr(e.Result)...)
		}
	case *build.LoadStmt:
		tokens = append(tokens, collectCommentsFromExpr(e.Module)...)
		for _, from := range e.From {
			tokens = append(tokens, collectCommentsFromExpr(from)...)
		}
		for _, to := range e.To {
			tokens = append(tokens, collectCommentsFromExpr(to)...)
		}
	}

	return tokens
}

// tokenizeComment creates tokens for a comment.
func tokenizeComment(comment build.Comment) []SemanticToken {
	var tokens []SemanticToken

	start := comment.Start
	text := comment.Token

	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1), // 0-based
		StartChar: uint32(start.LineRune - 1),
		Length:    uint32(len(text)),
		Type:      TokenComment,
		Modifiers: 0,
	})

	return tokens
}

// tokenizeStmt tokenizes a statement.
func tokenizeStmt(stmt build.Expr, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	switch s := stmt.(type) {
	case *build.DefStmt:
		tokens = append(tokens, tokenizeDefStmt(s, scope, content)...)
	case *build.LoadStmt:
		tokens = append(tokens, tokenizeLoadStmt(s, content)...)
	case *build.ForStmt:
		tokens = append(tokens, tokenizeForStmt(s, scope, content)...)
	case *build.IfStmt:
		tokens = append(tokens, tokenizeIfStmt(s, scope, content)...)
	case *build.ReturnStmt:
		tokens = append(tokens, tokenizeReturnStmt(s, scope, content)...)
	case *build.AssignExpr:
		tokens = append(tokens, tokenizeAssignExpr(s, scope, content)...)
	case *build.CallExpr:
		tokens = append(tokens, tokenizeCallExpr(s, scope, content)...)
	case *build.BranchStmt:
		tokens = append(tokens, tokenizeBranchStmt(s)...)
	default:
		// For other expressions, tokenize recursively
		tokens = append(tokens, tokenizeExpr(stmt, scope, content)...)
	}

	return tokens
}

// tokenizeDefStmt tokenizes a function definition.
func tokenizeDefStmt(def *build.DefStmt, parentScope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	// "def" keyword
	start := def.Function.StartPos
	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    3, // "def"
		Type:      TokenKeyword,
		Modifiers: 0,
	})

	// Function name
	nameStart, _ := findIdentPosition(content, def.Name, start.Line, start.LineRune+4)
	if nameStart.Line > 0 {
		tokens = append(tokens, SemanticToken{
			Line:      uint32(nameStart.Line - 1),
			StartChar: uint32(nameStart.LineRune - 1),
			Length:    uint32(len(def.Name)),
			Type:      TokenFunction,
			Modifiers: ModDeclaration | ModDefinition,
		})
	}

	// Create function scope
	funcScope := NewScope(parentScope)

	// Parameters
	for _, param := range def.Params {
		tokens = append(tokens, tokenizeParameter(param, funcScope, content)...)
	}

	// Function body
	for _, stmt := range def.Body {
		tokens = append(tokens, tokenizeStmt(stmt, funcScope, content)...)
	}

	return tokens
}

// tokenizeParameter tokenizes a function parameter.
func tokenizeParameter(param build.Expr, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	switch p := param.(type) {
	case *build.Ident:
		scope.Define(p.Name, SymbolParameter)
		start, _ := p.Span()
		tokens = append(tokens, SemanticToken{
			Line:      uint32(start.Line - 1),
			StartChar: uint32(start.LineRune - 1),
			Length:    uint32(len(p.Name)),
			Type:      TokenParameter,
			Modifiers: ModDeclaration,
		})

	case *build.AssignExpr:
		// param = default
		if ident, ok := p.LHS.(*build.Ident); ok {
			scope.Define(ident.Name, SymbolParameter)
			start, _ := ident.Span()
			tokens = append(tokens, SemanticToken{
				Line:      uint32(start.Line - 1),
				StartChar: uint32(start.LineRune - 1),
				Length:    uint32(len(ident.Name)),
				Type:      TokenParameter,
				Modifiers: ModDeclaration,
			})
		}
		// Tokenize default value
		tokens = append(tokens, tokenizeExpr(p.RHS, scope, content)...)

	case *build.UnaryExpr:
		// *args or **kwargs
		if ident, ok := p.X.(*build.Ident); ok {
			scope.Define(ident.Name, SymbolParameter)
			start, _ := ident.Span()
			tokens = append(tokens, SemanticToken{
				Line:      uint32(start.Line - 1),
				StartChar: uint32(start.LineRune - 1),
				Length:    uint32(len(ident.Name)),
				Type:      TokenParameter,
				Modifiers: ModDeclaration,
			})
		}
	}

	return tokens
}

// tokenizeLoadStmt tokenizes a load statement.
func tokenizeLoadStmt(load *build.LoadStmt, content string) []SemanticToken {
	var tokens []SemanticToken

	// "load" keyword
	start := load.Load
	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    4, // "load"
		Type:      TokenKeyword,
		Modifiers: 0,
	})

	// Module path string
	modStart, modEnd := load.Module.Span()
	tokens = append(tokens, SemanticToken{
		Line:      uint32(modStart.Line - 1),
		StartChar: uint32(modStart.LineRune - 1),
		Length:    uint32(modEnd.LineRune - modStart.LineRune),
		Type:      TokenString,
		Modifiers: 0,
	})

	// Imported symbols
	for i, from := range load.From {
		fStart, fEnd := from.Span()
		tokens = append(tokens, SemanticToken{
			Line:      uint32(fStart.Line - 1),
			StartChar: uint32(fStart.LineRune - 1),
			Length:    uint32(fEnd.LineRune - fStart.LineRune),
			Type:      TokenString,
			Modifiers: 0,
		})

		// Local name (if aliased or same)
		to := load.To[i]
		tStart, _ := to.Span()
		tokens = append(tokens, SemanticToken{
			Line:      uint32(tStart.Line - 1),
			StartChar: uint32(tStart.LineRune - 1),
			Length:    uint32(len(to.Name)),
			Type:      TokenVariable,
			Modifiers: ModDeclaration,
		})
	}

	return tokens
}

// tokenizeForStmt tokenizes a for loop.
func tokenizeForStmt(f *build.ForStmt, parentScope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	// "for" keyword
	start := f.For
	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    3, // "for"
		Type:      TokenKeyword,
		Modifiers: 0,
	})

	// Loop scope
	loopScope := NewScope(parentScope)

	// Loop variable(s)
	tokens = append(tokens, tokenizeLoopVars(f.Vars, loopScope, content)...)

	// "in" keyword - find it between vars and iterable
	_, varsEnd := f.Vars.Span()
	iterStart, _ := f.X.Span()
	inPos := findKeywordPosition(content, "in", varsEnd.Line, varsEnd.LineRune, iterStart.Line, iterStart.LineRune)
	if inPos.Line > 0 {
		tokens = append(tokens, SemanticToken{
			Line:      uint32(inPos.Line - 1),
			StartChar: uint32(inPos.LineRune - 1),
			Length:    2, // "in"
			Type:      TokenKeyword,
			Modifiers: 0,
		})
	}

	tokens = append(tokens, tokenizeExpr(f.X, parentScope, content)...)

	// Body
	for _, stmt := range f.Body {
		tokens = append(tokens, tokenizeStmt(stmt, loopScope, content)...)
	}

	return tokens
}

// tokenizeLoopVars tokenizes loop variables (handles tuple unpacking).
func tokenizeLoopVars(vars build.Expr, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	switch v := vars.(type) {
	case *build.Ident:
		scope.Define(v.Name, SymbolLocal)
		start, _ := v.Span()
		tokens = append(tokens, SemanticToken{
			Line:      uint32(start.Line - 1),
			StartChar: uint32(start.LineRune - 1),
			Length:    uint32(len(v.Name)),
			Type:      TokenVariable,
			Modifiers: ModDeclaration,
		})

	case *build.TupleExpr:
		for _, elem := range v.List {
			tokens = append(tokens, tokenizeLoopVars(elem, scope, content)...)
		}

	case *build.ListExpr:
		for _, elem := range v.List {
			tokens = append(tokens, tokenizeLoopVars(elem, scope, content)...)
		}
	}

	return tokens
}

// tokenizeIfStmt tokenizes an if statement.
func tokenizeIfStmt(i *build.IfStmt, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	// "if" keyword
	start := i.If
	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    2, // "if"
		Type:      TokenKeyword,
		Modifiers: 0,
	})

	// Condition
	tokens = append(tokens, tokenizeExpr(i.Cond, scope, content)...)

	// True branch
	for _, stmt := range i.True {
		tokens = append(tokens, tokenizeStmt(stmt, scope, content)...)
	}

	// False branch (elif/else)
	for _, stmt := range i.False {
		tokens = append(tokens, tokenizeStmt(stmt, scope, content)...)
	}

	return tokens
}

// tokenizeBranchStmt tokenizes pass, break, continue statements.
func tokenizeBranchStmt(b *build.BranchStmt) []SemanticToken {
	var tokens []SemanticToken

	pos := b.TokenPos
	keyword := b.Token // "pass", "break", or "continue"

	tokens = append(tokens, SemanticToken{
		Line:      uint32(pos.Line - 1),
		StartChar: uint32(pos.LineRune - 1),
		Length:    uint32(len(keyword)),
		Type:      TokenKeyword,
		Modifiers: 0,
	})

	return tokens
}

// tokenizeReturnStmt tokenizes a return statement.
func tokenizeReturnStmt(r *build.ReturnStmt, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	// "return" keyword
	start := r.Return
	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    6, // "return"
		Type:      TokenKeyword,
		Modifiers: 0,
	})

	// Return value
	if r.Result != nil {
		tokens = append(tokens, tokenizeExpr(r.Result, scope, content)...)
	}

	return tokens
}

// tokenizeAssignExpr tokenizes an assignment.
func tokenizeAssignExpr(a *build.AssignExpr, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	// LHS
	switch lhs := a.LHS.(type) {
	case *build.Ident:
		// Check if this is a new declaration
		_, exists := scope.Lookup(lhs.Name)
		mods := uint32(0)
		if !exists {
			scope.Define(lhs.Name, SymbolLocal)
			mods = ModDeclaration
		}

		start, _ := lhs.Span()
		tokens = append(tokens, SemanticToken{
			Line:      uint32(start.Line - 1),
			StartChar: uint32(start.LineRune - 1),
			Length:    uint32(len(lhs.Name)),
			Type:      TokenVariable,
			Modifiers: mods,
		})

	default:
		tokens = append(tokens, tokenizeExpr(a.LHS, scope, content)...)
	}

	// RHS
	tokens = append(tokens, tokenizeExpr(a.RHS, scope, content)...)

	return tokens
}

// tokenizeCallExpr tokenizes a function call.
func tokenizeCallExpr(c *build.CallExpr, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	// Function being called
	switch fn := c.X.(type) {
	case *build.Ident:
		tokens = append(tokens, tokenizeIdentInCallContext(fn, scope)...)

	case *build.DotExpr:
		// x.method()
		tokens = append(tokens, tokenizeExpr(fn.X, scope, content)...)

		// Method name
		nameStart, _ := fn.NamePos, fn.NamePos
		tokens = append(tokens, SemanticToken{
			Line:      uint32(nameStart.Line - 1),
			StartChar: uint32(nameStart.LineRune - 1),
			Length:    uint32(len(fn.Name)),
			Type:      TokenMethod,
			Modifiers: 0,
		})

	default:
		tokens = append(tokens, tokenizeExpr(c.X, scope, content)...)
	}

	// Arguments
	for _, arg := range c.List {
		switch a := arg.(type) {
		case *build.AssignExpr:
			// keyword=value
			if ident, ok := a.LHS.(*build.Ident); ok {
				start, _ := ident.Span()
				tokens = append(tokens, SemanticToken{
					Line:      uint32(start.Line - 1),
					StartChar: uint32(start.LineRune - 1),
					Length:    uint32(len(ident.Name)),
					Type:      TokenProperty,
					Modifiers: 0,
				})
			}
			tokens = append(tokens, tokenizeExpr(a.RHS, scope, content)...)
		default:
			tokens = append(tokens, tokenizeExpr(arg, scope, content)...)
		}
	}

	return tokens
}

// tokenizeIdentInCallContext tokenizes an identifier in function call position.
func tokenizeIdentInCallContext(ident *build.Ident, scope *Scope) []SemanticToken {
	var tokens []SemanticToken

	start, _ := ident.Span()
	name := ident.Name

	typ := TokenFunction
	mods := uint32(0)

	// Check if it's a builtin
	if IsStarlarkBuiltinFunc(name) {
		mods |= ModDefaultLibrary
	} else if name == "native" {
		typ = TokenNamespace
		mods |= ModDefaultLibrary | ModReadonly
	}

	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    uint32(len(name)),
		Type:      typ,
		Modifiers: mods,
	})

	return tokens
}

// tokenizeExpr tokenizes a general expression.
func tokenizeExpr(expr build.Expr, scope *Scope, content string) []SemanticToken {
	var tokens []SemanticToken

	switch e := expr.(type) {
	case *build.Ident:
		tokens = append(tokens, tokenizeIdent(e, scope)...)

	case *build.StringExpr:
		tokens = append(tokens, tokenizeString(e)...)

	case *build.LiteralExpr:
		start, end := e.Span()
		tokens = append(tokens, SemanticToken{
			Line:      uint32(start.Line - 1),
			StartChar: uint32(start.LineRune - 1),
			Length:    uint32(end.LineRune - start.LineRune),
			Type:      TokenNumber,
			Modifiers: 0,
		})

	case *build.ListExpr:
		for _, elem := range e.List {
			tokens = append(tokens, tokenizeExpr(elem, scope, content)...)
		}

	case *build.DictExpr:
		for _, entry := range e.List {
			kv := entry
			tokens = append(tokens, tokenizeExpr(kv.Key, scope, content)...)
			tokens = append(tokens, tokenizeExpr(kv.Value, scope, content)...)
		}

	case *build.TupleExpr:
		for _, elem := range e.List {
			tokens = append(tokens, tokenizeExpr(elem, scope, content)...)
		}

	case *build.CallExpr:
		tokens = append(tokens, tokenizeCallExpr(e, scope, content)...)

	case *build.BinaryExpr:
		tokens = append(tokens, tokenizeExpr(e.X, scope, content)...)
		tokens = append(tokens, tokenizeExpr(e.Y, scope, content)...)

	case *build.UnaryExpr:
		tokens = append(tokens, tokenizeExpr(e.X, scope, content)...)

	case *build.DotExpr:
		tokens = append(tokens, tokenizeExpr(e.X, scope, content)...)
		// Attribute
		tokens = append(tokens, SemanticToken{
			Line:      uint32(e.NamePos.Line - 1),
			StartChar: uint32(e.NamePos.LineRune - 1),
			Length:    uint32(len(e.Name)),
			Type:      TokenProperty,
			Modifiers: 0,
		})

	case *build.IndexExpr:
		tokens = append(tokens, tokenizeExpr(e.X, scope, content)...)
		tokens = append(tokens, tokenizeExpr(e.Y, scope, content)...)

	case *build.SliceExpr:
		tokens = append(tokens, tokenizeExpr(e.X, scope, content)...)
		if e.From != nil {
			tokens = append(tokens, tokenizeExpr(e.From, scope, content)...)
		}
		if e.To != nil {
			tokens = append(tokens, tokenizeExpr(e.To, scope, content)...)
		}
		if e.Step != nil {
			tokens = append(tokens, tokenizeExpr(e.Step, scope, content)...)
		}

	case *build.LambdaExpr:
		lambdaScope := NewScope(scope)
		for _, param := range e.Function.Params {
			tokens = append(tokens, tokenizeParameter(param, lambdaScope, content)...)
		}
		for _, bodyExpr := range e.Function.Body {
			tokens = append(tokens, tokenizeExpr(bodyExpr, lambdaScope, content)...)
		}

	case *build.Comprehension:
		compScope := NewScope(scope)
		tokens = append(tokens, tokenizeExpr(e.Body, compScope, content)...)
		for _, clause := range e.Clauses {
			if fc, ok := clause.(*build.ForClause); ok {
				tokens = append(tokens, tokenizeLoopVars(fc.Vars, compScope, content)...)
				tokens = append(tokens, tokenizeExpr(fc.X, scope, content)...)
			}
			if ic, ok := clause.(*build.IfClause); ok {
				tokens = append(tokens, tokenizeExpr(ic.Cond, compScope, content)...)
			}
		}

	case *build.ConditionalExpr:
		tokens = append(tokens, tokenizeExpr(e.Then, scope, content)...)
		tokens = append(tokens, tokenizeExpr(e.Test, scope, content)...)
		tokens = append(tokens, tokenizeExpr(e.Else, scope, content)...)
	}

	return tokens
}

// tokenizeIdent tokenizes an identifier.
func tokenizeIdent(ident *build.Ident, scope *Scope) []SemanticToken {
	var tokens []SemanticToken

	start, _ := ident.Span()
	name := ident.Name

	typ := TokenVariable
	mods := uint32(0)

	// Check constants
	if IsStarlarkConstant(name) {
		mods |= ModReadonly | ModDefaultLibrary
	} else if IsStarlarkBuiltinFunc(name) {
		typ = TokenFunction
		mods |= ModDefaultLibrary
	} else if kind, ok := scope.Lookup(name); ok {
		switch kind {
		case SymbolParameter:
			typ = TokenParameter
		case SymbolLocal, SymbolGlobal:
			typ = TokenVariable
		case SymbolImported:
			typ = TokenVariable
		case SymbolBuiltin:
			typ = TokenFunction
			mods |= ModDefaultLibrary
		}
	} else if name == "native" {
		typ = TokenNamespace
		mods |= ModDefaultLibrary | ModReadonly
	}

	tokens = append(tokens, SemanticToken{
		Line:      uint32(start.Line - 1),
		StartChar: uint32(start.LineRune - 1),
		Length:    uint32(len(name)),
		Type:      typ,
		Modifiers: mods,
	})

	return tokens
}

// tokenizeString tokenizes a string literal.
// For multi-line strings, we create separate tokens per line to avoid LSP issues.
func tokenizeString(s *build.StringExpr) []SemanticToken {
	var tokens []SemanticToken

	start, end := s.Span()
	value := s.Value

	// Check if it's a Bazel label
	typ := TokenString
	if isLabel(value) {
		typ = TokenLabel
	}

	// Handle multi-line strings: LSP expects each token to be on a single line
	if start.Line == end.Line {
		// Single line string - simple case
		length := end.LineRune - start.LineRune
		if length > 0 {
			tokens = append(tokens, SemanticToken{
				Line:      uint32(start.Line - 1),
				StartChar: uint32(start.LineRune - 1),
				Length:    uint32(length),
				Type:      typ,
				Modifiers: 0,
			})
		}
	} else {
		// Multi-line string: tokenize the opening quote on the first line
		// We can't easily determine line lengths without the content,
		// so we use a conservative approach: just mark the start position
		// with a reasonable length for the opening delimiter
		tokens = append(tokens, SemanticToken{
			Line:      uint32(start.Line - 1),
			StartChar: uint32(start.LineRune - 1),
			Length:    3, // Assume """ or ''' for multi-line
			Type:      typ,
			Modifiers: 0,
		})
	}

	return tokens
}

// isLabel checks if a string looks like a Bazel label.
func isLabel(s string) bool {
	return strings.HasPrefix(s, "//") ||
		strings.HasPrefix(s, ":") ||
		strings.HasPrefix(s, "@")
}

// encodeTokens encodes semantic tokens to LSP delta format.
func encodeTokens(tokens []SemanticToken) []uint32 {
	if len(tokens) == 0 {
		return []uint32{}
	}

	result := make([]uint32, 0, len(tokens)*5)
	prevLine, prevChar := uint32(0), uint32(0)

	for _, tok := range tokens {
		deltaLine := tok.Line - prevLine
		deltaChar := tok.StartChar
		if deltaLine == 0 {
			deltaChar = tok.StartChar - prevChar
		}

		result = append(result,
			deltaLine,
			deltaChar,
			tok.Length,
			tok.Type,
			tok.Modifiers,
		)

		prevLine = tok.Line
		prevChar = tok.StartChar
	}

	return result
}

// filterTokensInRange filters tokens to only those within the given range.
func filterTokensInRange(tokens []SemanticToken, r protocol.Range) []SemanticToken {
	var filtered []SemanticToken

	for _, tok := range tokens {
		// Check if token is within range
		if tok.Line >= uint32(r.Start.Line) && tok.Line <= uint32(r.End.Line) {
			filtered = append(filtered, tok)
		}
	}

	return filtered
}

// sortTokens sorts tokens by position (line, then character).
func sortTokens(tokens []SemanticToken) {
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].Line != tokens[j].Line {
			return tokens[i].Line < tokens[j].Line
		}
		return tokens[i].StartChar < tokens[j].StartChar
	})
}

// findIdentPosition finds the position of an identifier in content.
// This is a helper for cases where AST doesn't give us exact positions.
// startCol is 1-based rune offset.
func findIdentPosition(content string, name string, startLine, startCol int) (build.Position, bool) {
	lines := strings.Split(content, "\n")
	if startLine <= 0 || startLine > len(lines) {
		return build.Position{}, false
	}

	line := lines[startLine-1]
	if startCol <= 0 {
		startCol = 1
	}

	// Convert rune offset to byte offset
	byteOffset := 0
	runeCount := 0
	for byteOffset < len(line) && runeCount < startCol-1 {
		_, size := utf8.DecodeRuneInString(line[byteOffset:])
		byteOffset += size
		runeCount++
	}

	if byteOffset >= len(line) {
		return build.Position{}, false
	}

	// Search for the identifier as a whole word
	searchArea := line[byteOffset:]
	idx := strings.Index(searchArea, name)
	for idx != -1 {
		absPos := byteOffset + idx
		// Check if it's a standalone identifier (not part of another word)
		beforeOK := absPos == 0 || !isIdentChar(line[absPos-1])
		afterPos := absPos + len(name)
		afterOK := afterPos >= len(line) || !isIdentChar(line[afterPos])
		if beforeOK && afterOK {
			// Convert byte position back to rune position
			runePos := utf8.RuneCountInString(line[:absPos]) + 1 // 1-based
			return build.Position{
				Line:     startLine,
				LineRune: runePos,
			}, true
		}
		// Continue searching
		nextIdx := strings.Index(searchArea[idx+1:], name)
		if nextIdx == -1 {
			break
		}
		idx = idx + 1 + nextIdx
	}

	return build.Position{}, false
}

// findKeywordPosition finds a keyword between two positions.
// startCol and endCol are 1-based rune offsets.
func findKeywordPosition(content string, keyword string, startLine, startCol, endLine, endCol int) build.Position {
	lines := strings.Split(content, "\n")

	for lineNum := startLine; lineNum <= endLine && lineNum <= len(lines); lineNum++ {
		lineText := lines[lineNum-1]

		// Convert rune offsets to byte offsets
		fromByte := 0
		if lineNum == startLine && startCol > 1 {
			runeCount := 0
			for fromByte < len(lineText) && runeCount < startCol-1 {
				_, size := utf8.DecodeRuneInString(lineText[fromByte:])
				fromByte += size
				runeCount++
			}
		}

		toByte := len(lineText)
		if lineNum == endLine && endCol > 0 {
			runeCount := 0
			bytePos := 0
			for bytePos < len(lineText) && runeCount < endCol-1 {
				_, size := utf8.DecodeRuneInString(lineText[bytePos:])
				bytePos += size
				runeCount++
			}
			if bytePos < toByte {
				toByte = bytePos
			}
		}

		if fromByte >= toByte {
			continue
		}

		searchArea := lineText[fromByte:toByte]
		idx := strings.Index(searchArea, keyword)
		if idx != -1 {
			// Make sure it's a standalone keyword (not part of another word)
			absPos := fromByte + idx
			// Check character before
			if absPos > 0 && isIdentChar(lineText[absPos-1]) {
				continue
			}
			// Check character after
			afterPos := absPos + len(keyword)
			if afterPos < len(lineText) && isIdentChar(lineText[afterPos]) {
				continue
			}

			// Convert byte position back to rune position (1-based)
			runePos := utf8.RuneCountInString(lineText[:absPos]) + 1
			return build.Position{
				Line:     lineNum,
				LineRune: runePos,
			}
		}
	}

	return build.Position{}
}
