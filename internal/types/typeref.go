// Package types provides type representation and inference for Starlark code.
//
// This package implements a gradual type system inspired by starlark-rust/buck2,
// supporting type comments, docstring extraction, and basic inference.
//
// Type representation uses a union-based approach where types can have multiple
// alternatives. Type checking succeeds if ANY alternative matches.
package types

import (
	"fmt"
	"slices"
	"strings"
)

// TypeRef represents a type reference in Starlark code.
// Types are immutable and can be safely shared.
type TypeRef interface {
	// typeRef is a marker method to seal the interface.
	typeRef()

	// String returns the display representation of the type.
	String() string

	// Equal returns true if the types are structurally equal.
	Equal(other TypeRef) bool

	// IsUnknown returns true if this type is unknown/unresolved.
	IsUnknown() bool
}

// Ensure all type variants implement TypeRef.
var (
	_ TypeRef = (*NamedType)(nil)
	_ TypeRef = (*UnionType)(nil)
	_ TypeRef = (*FunctionType)(nil)
	_ TypeRef = (*NoneType)(nil)
	_ TypeRef = (*AnyType)(nil)
	_ TypeRef = (*UnknownType)(nil)
)

// NamedType represents a simple or generic type like "int" or "list[str]".
// This covers most Starlark types including builtins and user-defined types.
type NamedType struct {
	Name string    // Type name: "int", "str", "list", "dict", etc.
	Args []TypeRef // Generic arguments: list[str] -> Args: [str]
}

func (*NamedType) typeRef() {}

func (t *NamedType) String() string {
	if len(t.Args) == 0 {
		return t.Name
	}
	args := make([]string, len(t.Args))
	for i, arg := range t.Args {
		args[i] = arg.String()
	}
	return fmt.Sprintf("%s[%s]", t.Name, strings.Join(args, ", "))
}

func (t *NamedType) Equal(other TypeRef) bool {
	o, ok := other.(*NamedType)
	if !ok {
		return false
	}
	if t.Name != o.Name || len(t.Args) != len(o.Args) {
		return false
	}
	for i, arg := range t.Args {
		if !arg.Equal(o.Args[i]) {
			return false
		}
	}
	return true
}

func (t *NamedType) IsUnknown() bool { return false }

// UnionType represents a union of types like "int | str | None".
// In Starlark type comments, unions are written with | operator.
//
// Following starlark-rust semantics: operations succeed if ANY alternative matches.
type UnionType struct {
	Types []TypeRef // At least 2 types
}

func (*UnionType) typeRef() {}

func (t *UnionType) String() string {
	parts := make([]string, len(t.Types))
	for i, typ := range t.Types {
		parts[i] = typ.String()
	}
	return strings.Join(parts, " | ")
}

func (t *UnionType) Equal(other TypeRef) bool {
	o, ok := other.(*UnionType)
	if !ok {
		return false
	}
	if len(t.Types) != len(o.Types) {
		return false
	}
	// Order matters for equality (could normalize later)
	for i, typ := range t.Types {
		if !typ.Equal(o.Types[i]) {
			return false
		}
	}
	return true
}

func (t *UnionType) IsUnknown() bool { return false }

// FunctionType represents a callable type like "(int, str) -> bool".
// Used for function signatures from type comments.
type FunctionType struct {
	Params []ParamType // Parameter types
	Return TypeRef     // Return type (nil means None)
}

// ParamType represents a function parameter with optional type and metadata.
type ParamType struct {
	Name     string  // Parameter name (may be empty for positional-only)
	Type     TypeRef // Parameter type (may be nil if unknown)
	Optional bool    // Has default value
	IsArgs   bool    // *args parameter
	IsKwargs bool    // **kwargs parameter
}

func (*FunctionType) typeRef() {}

func (t *FunctionType) String() string {
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		if p.Name != "" && p.Type != nil {
			params[i] = fmt.Sprintf("%s: %s", p.Name, p.Type.String())
		} else if p.Type != nil {
			params[i] = p.Type.String()
		} else if p.Name != "" {
			params[i] = p.Name
		} else {
			params[i] = "?"
		}
		if p.IsArgs {
			params[i] = "*" + params[i]
		}
		if p.IsKwargs {
			params[i] = "**" + params[i]
		}
	}

	ret := "None"
	if t.Return != nil {
		ret = t.Return.String()
	}
	return fmt.Sprintf("(%s) -> %s", strings.Join(params, ", "), ret)
}

func (t *FunctionType) Equal(other TypeRef) bool {
	o, ok := other.(*FunctionType)
	if !ok {
		return false
	}
	if len(t.Params) != len(o.Params) {
		return false
	}
	for i, p := range t.Params {
		op := o.Params[i]
		if p.Name != op.Name || p.Optional != op.Optional ||
			p.IsArgs != op.IsArgs || p.IsKwargs != op.IsKwargs {
			return false
		}
		if (p.Type == nil) != (op.Type == nil) {
			return false
		}
		if p.Type != nil && !p.Type.Equal(op.Type) {
			return false
		}
	}
	if (t.Return == nil) != (o.Return == nil) {
		return false
	}
	if t.Return != nil && !t.Return.Equal(o.Return) {
		return false
	}
	return true
}

func (t *FunctionType) IsUnknown() bool { return false }

// NoneType represents the None/null type.
type NoneType struct{}

func (*NoneType) typeRef()        {}
func (*NoneType) String() string  { return "None" }
func (*NoneType) IsUnknown() bool { return false }
func (*NoneType) Equal(other TypeRef) bool {
	_, ok := other.(*NoneType)
	return ok
}

// AnyType represents a wildcard type that matches anything.
// Used for gradual typing - operations on Any always succeed.
type AnyType struct{}

func (*AnyType) typeRef()        {}
func (*AnyType) String() string  { return "Any" }
func (*AnyType) IsUnknown() bool { return false }
func (*AnyType) Equal(other TypeRef) bool {
	_, ok := other.(*AnyType)
	return ok
}

// UnknownType represents a type that could not be determined.
// Different from Any: Unknown indicates missing information,
// while Any is an explicit wildcard. Unknown types should be
// refined through inference when possible.
type UnknownType struct {
	// Reason optionally explains why the type is unknown.
	Reason string
}

func (*UnknownType) typeRef()        {}
func (*UnknownType) String() string  { return "Unknown" }
func (*UnknownType) IsUnknown() bool { return true }
func (*UnknownType) Equal(other TypeRef) bool {
	_, ok := other.(*UnknownType)
	return ok
}

// ----------------------------------------------------------------------------
// Constructors - convenience functions for creating common types
// ----------------------------------------------------------------------------

// Primitive type constructors.
func Int() TypeRef     { return &NamedType{Name: "int"} }
func Str() TypeRef     { return &NamedType{Name: "str"} }
func Bool() TypeRef    { return &NamedType{Name: "bool"} }
func Float() TypeRef   { return &NamedType{Name: "float"} }
func Bytes() TypeRef   { return &NamedType{Name: "bytes"} }
func None() TypeRef    { return &NoneType{} }
func Any() TypeRef     { return &AnyType{} }
func Unknown() TypeRef { return &UnknownType{} }

// UnknownWithReason creates an Unknown type with an explanation.
func UnknownWithReason(reason string) TypeRef {
	return &UnknownType{Reason: reason}
}

// Collection type constructors.

// List creates a list[T] type.
func List(elem TypeRef) TypeRef {
	return &NamedType{Name: "list", Args: []TypeRef{elem}}
}

// Dict creates a dict[K, V] type.
func Dict(key, value TypeRef) TypeRef {
	return &NamedType{Name: "dict", Args: []TypeRef{key, value}}
}

// Set creates a set[T] type.
func Set(elem TypeRef) TypeRef {
	return &NamedType{Name: "set", Args: []TypeRef{elem}}
}

// Tuple creates a tuple[T...] type with specific element types.
func Tuple(elems ...TypeRef) TypeRef {
	return &NamedType{Name: "tuple", Args: elems}
}

// Union type constructors.

// Union creates a union of multiple types.
// If only one type is provided, returns that type directly.
// Flattens nested unions.
func Union(types ...TypeRef) TypeRef {
	if len(types) == 0 {
		return Unknown()
	}
	if len(types) == 1 {
		return types[0]
	}

	// Flatten nested unions
	flat := make([]TypeRef, 0, len(types))
	for _, t := range types {
		if u, ok := t.(*UnionType); ok {
			flat = append(flat, u.Types...)
		} else {
			flat = append(flat, t)
		}
	}

	// Remove duplicates (simple equality check)
	seen := make(map[string]bool)
	deduped := make([]TypeRef, 0, len(flat))
	for _, t := range flat {
		key := t.String()
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, t)
		}
	}

	if len(deduped) == 1 {
		return deduped[0]
	}
	return &UnionType{Types: deduped}
}

// Optional creates a T | None union type.
func Optional(t TypeRef) TypeRef {
	return Union(t, None())
}

// Function type constructors.

// Func creates a function type with the given parameter and return types.
func Func(params []ParamType, ret TypeRef) TypeRef {
	return &FunctionType{Params: params, Return: ret}
}

// SimpleFunc creates a function type from just the parameter types and return type.
func SimpleFunc(paramTypes []TypeRef, ret TypeRef) TypeRef {
	params := make([]ParamType, len(paramTypes))
	for i, pt := range paramTypes {
		params[i] = ParamType{Type: pt}
	}
	return &FunctionType{Params: params, Return: ret}
}

// ----------------------------------------------------------------------------
// Type predicates and utilities
// ----------------------------------------------------------------------------

// IsNone returns true if the type is None.
func IsNone(t TypeRef) bool {
	_, ok := t.(*NoneType)
	return ok
}

// IsAny returns true if the type is Any.
func IsAny(t TypeRef) bool {
	_, ok := t.(*AnyType)
	return ok
}

// IsFunction returns true if the type is a function type.
func IsFunction(t TypeRef) bool {
	_, ok := t.(*FunctionType)
	return ok
}

// IsUnion returns true if the type is a union type.
func IsUnion(t TypeRef) bool {
	_, ok := t.(*UnionType)
	return ok
}

// IsCollection returns true if the type is list, dict, set, or tuple.
func IsCollection(t TypeRef) bool {
	n, ok := t.(*NamedType)
	if !ok {
		return false
	}
	switch n.Name {
	case "list", "dict", "set", "tuple":
		return true
	}
	return false
}

// ElementType returns the element type for collections.
// For list[T] and set[T], returns T.
// For dict[K,V], returns V (the value type).
// For tuple, returns Unknown (heterogeneous).
// For non-collections, returns nil.
func ElementType(t TypeRef) TypeRef {
	n, ok := t.(*NamedType)
	if !ok {
		return nil
	}
	switch n.Name {
	case "list", "set":
		if len(n.Args) > 0 {
			return n.Args[0]
		}
		return Unknown()
	case "dict":
		if len(n.Args) > 1 {
			return n.Args[1]
		}
		return Unknown()
	case "tuple":
		// Tuples are heterogeneous, can't give single element type
		return Unknown()
	}
	return nil
}

// ContainsUnknown returns true if the type or any nested type is Unknown.
func ContainsUnknown(t TypeRef) bool {
	if t.IsUnknown() {
		return true
	}
	switch typ := t.(type) {
	case *NamedType:
		return slices.ContainsFunc(typ.Args, ContainsUnknown)
	case *UnionType:
		return slices.ContainsFunc(typ.Types, ContainsUnknown)
	case *FunctionType:
		for _, p := range typ.Params {
			if p.Type != nil && ContainsUnknown(p.Type) {
				return true
			}
		}
		if typ.Return != nil && ContainsUnknown(typ.Return) {
			return true
		}
	}
	return false
}

// Simplify reduces a type to its canonical form.
// - Flattens nested unions
// - Removes duplicate union members
// - Unwraps single-element unions
func Simplify(t TypeRef) TypeRef {
	switch typ := t.(type) {
	case *UnionType:
		return Union(typ.Types...)
	default:
		return t
	}
}
