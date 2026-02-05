# LSP Semantic Tokens Implementation Plan

**Status:** Draft
**Date:** 2026-02-05
**Branch:** `feat/lsp-semantic-tokens`
**Worktree:** `/Users/adsc/dev/ws/sky/semantic-tokens`

## Overview

Semantic tokens provide rich, context-aware syntax highlighting that goes beyond regex-based TextMate grammars. This feature is critical for modern IDE integration and significantly improves developer experience.

### Why This Matters

1. **Context awareness** - Distinguish `native.cc_library` (builtin) from `my_cc_library` (user-defined)
2. **Scope tracking** - Different colors for parameters vs globals vs imports
3. **Error prevention** - Visually identify undefined references before running
4. **Dialect support** - Different highlighting for Bazel vs Buck2 vs Tilt

### Common Pitfalls to Avoid

Based on analysis of other LSP implementations:

1. **Token boundary errors** - Off-by-one in character positions breaks highlighting
2. **Delta encoding bugs** - LSP uses delta encoding; cumulative errors cascade
3. **Multi-line token handling** - String literals spanning lines need special care
4. **Performance on large files** - Must be incremental, not re-tokenize entire file
5. **Modifier combinations** - Bitfield encoding is error-prone
6. **Dialect coupling** - Hard-coding Bazel-specific tokens limits extensibility

## LSP Protocol Details

### Capability Registration

```go
SemanticTokensProvider: &protocol.SemanticTokensOptions{
    Legend: protocol.SemanticTokensLegend{
        TokenTypes: []string{
            "namespace",    // 0: module names in load()
            "type",         // 1: type names (depset, Target, etc.)
            "class",        // 2: rule definitions
            "function",     // 3: function/macro definitions
            "method",       // 4: methods on objects
            "property",     // 5: struct fields, rule attributes
            "variable",     // 6: local variables
            "parameter",    // 7: function parameters
            "keyword",      // 8: def, if, for, load, etc.
            "string",       // 9: string literals
            "number",       // 10: numeric literals
            "operator",     // 11: +, -, *, /, etc.
            "comment",      // 12: # comments
            "macro",        // 13: macro invocations
            "decorator",    // 14: @decorator (if applicable)
            "label",        // 15: Bazel labels "//pkg:target"
        },
        TokenModifiers: []string{
            "declaration",    // 0: where symbol is defined
            "definition",     // 1: def statement
            "readonly",       // 2: constants, immutable
            "static",         // 3: module-level
            "deprecated",     // 4: marked deprecated
            "modification",   // 5: assignment target
            "documentation",  // 6: docstrings
            "defaultLibrary", // 7: builtin functions/types
        },
    },
    Full:  true,  // Support full document tokenization
    Range: true,  // Support range-based tokenization (optimization)
    Delta: true,  // Support incremental updates
},
```

### Token Encoding

LSP semantic tokens use a compact encoding:
- 5 integers per token: `[deltaLine, deltaStartChar, length, tokenType, tokenModifiers]`
- Delta encoding: positions relative to previous token
- Modifiers: bitfield (e.g., `declaration | defaultLibrary` = 0b10000001)

```go
type SemanticToken struct {
    Line       uint32 // 0-based line number
    StartChar  uint32 // 0-based character offset
    Length     uint32 // token length in characters
    Type       uint32 // index into tokenTypes legend
    Modifiers  uint32 // bitfield of modifiers
}

// Encode to LSP format (delta encoding)
func encodeTokens(tokens []SemanticToken) []uint32 {
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
```

## Architecture Design

### Dialect-Aware Token Classification

```
┌─────────────────────────────────────────────────────────────┐
│                    SemanticTokenizer                         │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  TokenClassifier                     │   │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────────────┐  │   │
│  │  │  Starlark │ │   Bazel   │ │   Buck2/Tilt/...  │  │   │
│  │  │   Core    │ │  Dialect  │ │     Dialects      │  │   │
│  │  └───────────┘ └───────────┘ └───────────────────┘  │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              SymbolResolver                          │   │
│  │  - Resolves identifiers to their definitions         │   │
│  │  - Tracks scope (local, parameter, global, builtin)  │   │
│  │  - Uses builtins.Provider for dialect awareness      │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              TokenEncoder                            │   │
│  │  - Converts classified tokens to LSP format          │   │
│  │  - Handles delta encoding                            │   │
│  │  - Supports incremental updates                      │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Key Interfaces

```go
// TokenClassifier classifies AST nodes into semantic token types.
// Implementations are dialect-specific.
type TokenClassifier interface {
    // ClassifyExpr returns the token type and modifiers for an expression.
    ClassifyExpr(expr build.Expr, scope *Scope) (TokenType, TokenModifiers)

    // ClassifyIdent classifies an identifier based on context.
    ClassifyIdent(name string, ctx IdentContext, scope *Scope) (TokenType, TokenModifiers)

    // IsBuiltin returns true if the name is a dialect builtin.
    IsBuiltin(name string) bool

    // IsDeprecated returns true if the symbol is deprecated.
    IsDeprecated(name string) bool
}

// Scope tracks symbol definitions and their kinds.
type Scope struct {
    parent    *Scope
    symbols   map[string]SymbolKind
    // Track where each symbol was defined for "declaration" modifier
    defs      map[string]Position
}

type SymbolKind int
const (
    SymbolLocal SymbolKind = iota
    SymbolParameter
    SymbolGlobal
    SymbolImported
    SymbolBuiltin
)

// IdentContext provides context for identifier classification.
type IdentContext int
const (
    CtxLoad        IdentContext = iota // In load() statement
    CtxCall                            // Function call position
    CtxAttribute                       // Attribute access (x.attr)
    CtxAssignLHS                       // Left side of assignment
    CtxAssignRHS                       // Right side of assignment
    CtxParameter                       // Function parameter
    CtxDefault                         // Default value
)
```

### Starlark Core Classifier

```go
// StarlarkClassifier handles core Starlark tokens.
// Dialect classifiers embed this and override specific behaviors.
type StarlarkClassifier struct {
    builtins builtins.Provider
    dialect  string
    kind     filekind.Kind
}

func (c *StarlarkClassifier) ClassifyIdent(name string, ctx IdentContext, scope *Scope) (TokenType, TokenModifiers) {
    var typ TokenType
    var mods TokenModifiers

    // Check scope first
    if kind, ok := scope.Lookup(name); ok {
        switch kind {
        case SymbolParameter:
            typ = TokenParameter
        case SymbolLocal:
            typ = TokenVariable
        case SymbolGlobal:
            typ = TokenVariable
            mods |= ModStatic
        case SymbolImported:
            typ = TokenNamespace
        }

        // Check if this is the declaration site
        if defPos, ok := scope.defs[name]; ok && defPos == currentPos {
            mods |= ModDeclaration
        }
        return typ, mods
    }

    // Check builtins
    if c.builtins != nil {
        b, _ := c.builtins.Builtins(c.dialect, c.kind)
        for _, fn := range b.Functions {
            if fn.Name == name {
                return TokenFunction, ModDefaultLibrary
            }
        }
        for _, t := range b.Types {
            if t.Name == name {
                return TokenType, ModDefaultLibrary
            }
        }
    }

    // Fallback based on context
    switch ctx {
    case CtxCall:
        return TokenFunction, 0
    case CtxAttribute:
        return TokenProperty, 0
    default:
        return TokenVariable, 0
    }
}
```

### Bazel Dialect Extensions

```go
// BazelClassifier extends StarlarkClassifier with Bazel-specific tokens.
type BazelClassifier struct {
    *StarlarkClassifier
}

func (c *BazelClassifier) ClassifyIdent(name string, ctx IdentContext, scope *Scope) (TokenType, TokenModifiers) {
    // Bazel-specific: native module
    if name == "native" {
        return TokenNamespace, ModDefaultLibrary | ModReadonly
    }

    // Bazel-specific: rule names in BUILD files
    if c.kind == filekind.KindBUILD && ctx == CtxCall {
        if c.isNativeRule(name) {
            return TokenMacro, ModDefaultLibrary
        }
    }

    // Bazel-specific: labels
    // Handled separately in ClassifyString

    return c.StarlarkClassifier.ClassifyIdent(name, ctx, scope)
}

func (c *BazelClassifier) ClassifyString(value string, ctx StringContext) (TokenType, TokenModifiers) {
    // Detect Bazel labels: "//pkg:target", ":target", "@repo//pkg:target"
    if isLabel(value) {
        return TokenLabel, 0
    }
    return TokenString, 0
}

func isLabel(s string) bool {
    return strings.HasPrefix(s, "//") ||
           strings.HasPrefix(s, ":") ||
           strings.HasPrefix(s, "@")
}
```

## Implementation Plan

### Phase 1: Core Infrastructure (TDD)

**Files to create:**
- `internal/lsp/semantic.go` - Main tokenizer
- `internal/lsp/semantic_test.go` - Tests
- `internal/lsp/semantic_types.go` - Types and constants
- `internal/lsp/semantic_encode.go` - Delta encoding
- `internal/lsp/semantic_scope.go` - Scope tracking

**Tests first:**
```go
func TestSemanticTokens_Keywords(t *testing.T) {
    // def, if, for, return, load should be TokenKeyword
}

func TestSemanticTokens_FunctionDef(t *testing.T) {
    // Function name at definition should be TokenFunction + ModDeclaration
}

func TestSemanticTokens_Parameters(t *testing.T) {
    // Parameters should be TokenParameter, usages too
}

func TestSemanticTokens_Builtins(t *testing.T) {
    // len, str, dict should be TokenFunction + ModDefaultLibrary
}

func TestSemanticTokens_DeltaEncoding(t *testing.T) {
    // Verify encoding matches LSP spec exactly
}

func TestSemanticTokens_MultilineString(t *testing.T) {
    // Triple-quoted strings spanning lines
}
```

### Phase 2: Dialect Awareness

**Extend for Bazel:**
```go
func TestSemanticTokens_BazelLabels(t *testing.T) {
    // "//pkg:target" should be TokenLabel
}

func TestSemanticTokens_NativeModule(t *testing.T) {
    // native.cc_library - native is TokenNamespace
}

func TestSemanticTokens_BuildFileRules(t *testing.T) {
    // cc_library() in BUILD -> TokenMacro + ModDefaultLibrary
}
```

### Phase 3: Performance & Incremental

**Optimizations:**
- Cache tokenization results per document version
- Implement range-based tokenization for large files
- Support delta updates (only re-tokenize changed regions)

### Phase 4: Wire to LSP

**server.go changes:**
```go
case "textDocument/semanticTokens/full":
    return s.handleSemanticTokensFull(ctx, req.Params)
case "textDocument/semanticTokens/range":
    return s.handleSemanticTokensRange(ctx, req.Params)
case "textDocument/semanticTokens/full/delta":
    return s.handleSemanticTokensDelta(ctx, req.Params)
```

## Token Type Mapping

### Starlark Core

| AST Node | Token Type | Modifiers |
|----------|------------|-----------|
| `def name` | function | declaration, definition |
| `name(...)` call | function | (none or defaultLibrary) |
| `param` in def | parameter | declaration |
| `param` usage | parameter | |
| `x = ...` first assign | variable | declaration |
| `x` usage | variable | |
| `load("...", "sym")` | namespace | |
| `for x in` | variable | declaration |
| `if`, `def`, `for`, etc. | keyword | |
| `"string"` | string | |
| `123`, `3.14` | number | |
| `# comment` | comment | |
| `True`, `False`, `None` | variable | readonly, defaultLibrary |

### Bazel Extensions

| AST Node | Token Type | Modifiers |
|----------|------------|-----------|
| `native` | namespace | defaultLibrary, readonly |
| `native.rule()` | macro | defaultLibrary |
| `cc_library()` in BUILD | macro | defaultLibrary |
| `"//pkg:target"` | label | |
| `"@repo//..."` | label | |
| `select({...})` | function | defaultLibrary |
| `depset`, `Target` | type | defaultLibrary |

## Testing Strategy

### Unit Tests
- Token classification for each AST node type
- Delta encoding correctness
- Scope tracking accuracy
- Modifier combinations

### Integration Tests
- Full document tokenization
- Range requests
- Delta updates
- Large file performance

### Golden Tests
- Known Starlark files with expected token output
- Verify against VS Code rendering

### Edge Cases
- Empty files
- Syntax errors (partial tokenization)
- Unicode identifiers
- Very long lines
- Deeply nested structures

## Success Criteria

1. **Correctness**: No off-by-one errors in token positions
2. **Completeness**: All Starlark constructs tokenized
3. **Performance**: <50ms for 1000-line file
4. **Extensibility**: Easy to add Buck2/Tilt dialects
5. **Compatibility**: Works with VS Code, Neovim, Helix

## References

- [LSP Semantic Tokens Spec](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#textDocument_semanticTokens)
- [VS Code Semantic Highlighting Guide](https://code.visualstudio.com/api/language-extensions/semantic-highlight-guide)
- [starpls implementation](https://github.com/withered-magic/starpls)
- [rust-analyzer semantic tokens](https://github.com/rust-lang/rust-analyzer)
