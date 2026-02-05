package lsp

import "go.lsp.dev/protocol"

// Semantic token types - indices into the legend.
// Order matters! These are encoded as integers in LSP responses.
const (
	TokenNamespace  uint32 = iota // module names in load()
	TokenType                     // type names (depset, Target, etc.)
	TokenClass                    // rule definitions
	TokenFunction                 // function/macro definitions
	TokenMethod                   // methods on objects
	TokenProperty                 // struct fields, rule attributes
	TokenVariable                 // local variables
	TokenParameter                // function parameters
	TokenKeyword                  // def, if, for, load, etc.
	TokenString                   // string literals
	TokenNumber                   // numeric literals
	TokenOperator                 // +, -, *, /, etc.
	TokenComment                  // # comments
	TokenMacro                    // macro invocations (Bazel rules)
	TokenDecorator                // @decorator (if applicable)
	TokenLabel                    // Bazel labels "//pkg:target"
)

// TokenTypeNames maps token types to their LSP legend names (protocol types).
var TokenTypeNames = []protocol.SemanticTokenTypes{
	protocol.SemanticTokenNamespace,
	protocol.SemanticTokenType,
	protocol.SemanticTokenClass,
	protocol.SemanticTokenFunction,
	protocol.SemanticTokenMethod,
	protocol.SemanticTokenProperty,
	protocol.SemanticTokenVariable,
	protocol.SemanticTokenParameter,
	protocol.SemanticTokenKeyword,
	protocol.SemanticTokenString,
	protocol.SemanticTokenNumber,
	protocol.SemanticTokenOperator,
	protocol.SemanticTokenComment,
	protocol.SemanticTokenMacro,
	"decorator", // Custom - not in protocol
	"label",     // Custom type for Bazel labels
}

// Semantic token modifiers - bit flags that can be combined.
const (
	ModDeclaration    uint32 = 1 << iota // where symbol is defined
	ModDefinition                        // def statement
	ModReadonly                          // constants, immutable
	ModStatic                            // module-level
	ModDeprecated                        // marked deprecated
	ModModification                      // assignment target
	ModDocumentation                     // docstrings
	ModDefaultLibrary                    // builtin functions/types
)

// TokenModifierNames maps modifier bits to their LSP legend names (protocol types).
var TokenModifierNames = []protocol.SemanticTokenModifiers{
	protocol.SemanticTokenModifierDeclaration,
	protocol.SemanticTokenModifierDefinition,
	protocol.SemanticTokenModifierReadonly,
	protocol.SemanticTokenModifierStatic,
	protocol.SemanticTokenModifierDeprecated,
	protocol.SemanticTokenModifierModification,
	protocol.SemanticTokenModifierDocumentation,
	protocol.SemanticTokenModifierDefaultLibrary,
}

// SemanticToken represents a single semantic token before encoding.
type SemanticToken struct {
	Line      uint32 // 0-based line number
	StartChar uint32 // 0-based character offset (in UTF-16 code units for LSP)
	Length    uint32 // token length in characters
	Type      uint32 // token type (index into TokenTypeNames)
	Modifiers uint32 // bitfield of modifiers
}

// SymbolKind represents the kind of a symbol in scope.
type SymbolKind int

const (
	SymbolLocal     SymbolKind = iota // local variable
	SymbolParameter                   // function parameter
	SymbolGlobal                      // module-level variable
	SymbolImported                    // imported via load()
	SymbolBuiltin                     // builtin function/type
)

// Scope tracks symbol definitions within a lexical scope.
type Scope struct {
	parent  *Scope
	symbols map[string]SymbolKind
}

// NewScope creates a new scope, optionally with a parent.
func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]SymbolKind),
	}
}

// Define adds a symbol to the current scope.
func (s *Scope) Define(name string, kind SymbolKind) {
	s.symbols[name] = kind
}

// Lookup searches for a symbol in this scope and parent scopes.
func (s *Scope) Lookup(name string) (SymbolKind, bool) {
	if kind, ok := s.symbols[name]; ok {
		return kind, true
	}
	if s.parent != nil {
		return s.parent.Lookup(name)
	}
	return 0, false
}

// Starlark keywords that should be highlighted as TokenKeyword.
var starlarkKeywordsSet = map[string]bool{
	"and":      true,
	"break":    true,
	"continue": true,
	"def":      true,
	"elif":     true,
	"else":     true,
	"for":      true,
	"if":       true,
	"in":       true,
	"lambda":   true,
	"load":     true,
	"not":      true,
	"or":       true,
	"pass":     true,
	"return":   true,
	"while":    true,
}

// IsStarlarkKeyword returns true if the name is a Starlark keyword.
func IsStarlarkKeyword(name string) bool {
	return starlarkKeywordsSet[name]
}

// Starlark builtin constants.
var starlarkConstantsSet = map[string]bool{
	"True":  true,
	"False": true,
	"None":  true,
}

// IsStarlarkConstant returns true if the name is a Starlark constant.
func IsStarlarkConstant(name string) bool {
	return starlarkConstantsSet[name]
}

// Starlark core builtin functions.
var starlarkBuiltinFuncs = map[string]bool{
	"abs":       true,
	"all":       true,
	"any":       true,
	"bool":      true,
	"bytes":     true,
	"dict":      true,
	"dir":       true,
	"enumerate": true,
	"fail":      true,
	"float":     true,
	"getattr":   true,
	"hasattr":   true,
	"hash":      true,
	"int":       true,
	"len":       true,
	"list":      true,
	"max":       true,
	"min":       true,
	"print":     true,
	"range":     true,
	"repr":      true,
	"reversed":  true,
	"sorted":    true,
	"str":       true,
	"tuple":     true,
	"type":      true,
	"zip":       true,
}

// IsStarlarkBuiltinFunc returns true if the name is a Starlark builtin function.
func IsStarlarkBuiltinFunc(name string) bool {
	return starlarkBuiltinFuncs[name]
}
