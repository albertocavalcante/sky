// Package builtins provides interfaces and types for Starlark builtin definitions.
package builtins

import (
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Signature represents a function or method signature.
type Signature struct {
	// Name is the function or method name.
	Name string `json:"name"`

	// Doc is the documentation string.
	Doc string `json:"doc,omitempty"`

	// Params lists the function parameters.
	Params []Param `json:"params,omitempty"`

	// ReturnType is the return type as a string (e.g., "str", "list[int]").
	ReturnType string `json:"return_type,omitempty"`

	// Deprecated contains a deprecation message if this function is deprecated.
	Deprecated string `json:"deprecated,omitempty"`
}

// Param represents a function parameter.
type Param struct {
	// Name is the parameter name.
	Name string `json:"name"`

	// Type is the parameter type as a string (e.g., "str", "int | None").
	Type string `json:"type,omitempty"`

	// Default is the default value as a string representation.
	Default string `json:"default,omitempty"`

	// Required is true if this parameter must be provided.
	Required bool `json:"required,omitempty"`

	// Variadic is true if this is a *args parameter.
	Variadic bool `json:"variadic,omitempty"`

	// KWArgs is true if this is a **kwargs parameter.
	KWArgs bool `json:"kwargs,omitempty"`
}

// TypeDef represents a type definition (struct, provider, etc.).
type TypeDef struct {
	// Name is the type name.
	Name string `json:"name"`

	// Doc is the documentation string.
	Doc string `json:"doc,omitempty"`

	// Fields lists the type's fields.
	Fields []Field `json:"fields,omitempty"`

	// Methods lists the type's methods.
	Methods []Signature `json:"methods,omitempty"`
}

// Field represents a struct or provider field.
type Field struct {
	// Name is the field name.
	Name string `json:"name"`

	// Type is the field type as a string.
	Type string `json:"type,omitempty"`

	// Doc is the documentation string.
	Doc string `json:"doc,omitempty"`
}

// Builtins contains all builtin definitions for a dialect and file kind.
type Builtins struct {
	// Functions lists builtin functions.
	Functions []Signature `json:"functions,omitempty"`

	// Types lists builtin type definitions.
	Types []TypeDef `json:"types,omitempty"`

	// Globals lists global variables/constants.
	Globals []Field `json:"globals,omitempty"`
}

// Merge combines another Builtins into this one.
func (b *Builtins) Merge(other Builtins) {
	b.Functions = append(b.Functions, other.Functions...)
	b.Types = append(b.Types, other.Types...)
	b.Globals = append(b.Globals, other.Globals...)
}

// Provider supplies builtin definitions for dialects.
type Provider interface {
	// Builtins returns builtin definitions for a dialect and file kind.
	// Returns an error if the dialect or file kind is not supported.
	Builtins(dialect string, kind filekind.Kind) (Builtins, error)

	// SupportedDialects returns the dialects this provider knows about.
	SupportedDialects() []string
}

// ProviderFunc is a function type that implements Provider.
type ProviderFunc func(dialect string, kind filekind.Kind) (Builtins, error)

// Builtins implements the Provider interface.
func (f ProviderFunc) Builtins(dialect string, kind filekind.Kind) (Builtins, error) {
	return f(dialect, kind)
}

// SupportedDialects returns an empty slice for ProviderFunc.
func (f ProviderFunc) SupportedDialects() []string {
	return nil
}

// ChainProvider chains multiple providers.
type ChainProvider struct {
	providers []Provider
}

// NewChainProvider creates a provider that merges results from all providers.
func NewChainProvider(providers ...Provider) *ChainProvider {
	return &ChainProvider{providers: providers}
}

// Builtins merges builtins from all providers.
func (c *ChainProvider) Builtins(dialect string, kind filekind.Kind) (Builtins, error) {
	var result Builtins
	for _, p := range c.providers {
		b, err := p.Builtins(dialect, kind)
		if err != nil {
			continue // Skip providers that don't support this dialect/kind
		}
		result.Merge(b)
	}
	return result, nil
}

// SupportedDialects returns all dialects from all providers.
func (c *ChainProvider) SupportedDialects() []string {
	seen := make(map[string]bool)
	var result []string
	for _, p := range c.providers {
		for _, d := range p.SupportedDialects() {
			if !seen[d] {
				seen[d] = true
				result = append(result, d)
			}
		}
	}
	return result
}
