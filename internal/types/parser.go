package types

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseTypeComment parses a "# type: T" comment into a TypeRef.
// Returns an error if the comment is not a valid type comment.
//
// Supported syntax:
//   - Simple types: int, str, bool, None, Any
//   - Generic types: list[str], dict[str, int]
//   - Union types: int | None, str | int | bool
//   - Optional sugar: Optional[int] -> int | None
//   - Function types: (int, str) -> bool
//   - Nested types: list[dict[str, int]]
func ParseTypeComment(comment string) (TypeRef, error) {
	// Strip comment prefix
	comment = strings.TrimSpace(comment)

	// Handle various comment formats
	typeStr := ""
	switch {
	case strings.HasPrefix(comment, "# type:"):
		typeStr = strings.TrimPrefix(comment, "# type:")
	case strings.HasPrefix(comment, "#type:"):
		typeStr = strings.TrimPrefix(comment, "#type:")
	case strings.HasPrefix(comment, "# type :"):
		typeStr = strings.TrimPrefix(comment, "# type :")
	default:
		return nil, fmt.Errorf("not a type comment: %q", comment)
	}

	typeStr = strings.TrimSpace(typeStr)
	if typeStr == "" {
		return nil, fmt.Errorf("empty type in comment")
	}

	p := &typeParser{input: typeStr}
	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}

	// Ensure we consumed all input
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected characters after type: %q", p.input[p.pos:])
	}

	return typ, nil
}

// ParseTypeString parses a type expression string (without the # type: prefix).
func ParseTypeString(typeStr string) (TypeRef, error) {
	typeStr = strings.TrimSpace(typeStr)
	if typeStr == "" {
		return nil, fmt.Errorf("empty type string")
	}

	p := &typeParser{input: typeStr}
	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected characters after type: %q", p.input[p.pos:])
	}

	return typ, nil
}

// ParseFunctionTypeComment parses a function type comment of the form:
// # type: (T1, T2, ...) -> R
func ParseFunctionTypeComment(comment string) (*FunctionType, error) {
	typ, err := ParseTypeComment(comment)
	if err != nil {
		return nil, err
	}

	ft, ok := typ.(*FunctionType)
	if !ok {
		return nil, fmt.Errorf("expected function type, got %s", typ.String())
	}
	return ft, nil
}

// typeParser is a simple recursive descent parser for type expressions.
type typeParser struct {
	input string
	pos   int
}

// parseType is the entry point - parses a complete type expression.
func (p *typeParser) parseType() (TypeRef, error) {
	return p.parseUnion()
}

// parseUnion parses union types: T1 | T2 | ...
func (p *typeParser) parseUnion() (TypeRef, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	types := []TypeRef{left}
	for {
		p.skipWhitespace()
		if !p.match('|') {
			break
		}
		p.skipWhitespace()

		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		types = append(types, right)
	}

	if len(types) == 1 {
		return types[0], nil
	}
	return Union(types...), nil
}

// parsePrimary parses a single type (not a union).
func (p *typeParser) parsePrimary() (TypeRef, error) {
	p.skipWhitespace()

	// Check for function type: (T1, T2) -> R or Callable[[T1, T2], R]
	if p.peek() == '(' {
		return p.parseFunctionType()
	}

	// Parse identifier (type name)
	name := p.parseIdent()
	if name == "" {
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unexpected end of type expression")
		}
		return nil, fmt.Errorf("expected type name at position %d, got %q", p.pos, string(p.peek()))
	}

	// Handle special types
	switch name {
	case "None":
		return None(), nil
	case "Any":
		return Any(), nil
	case "Optional":
		// Optional[T] is sugar for T | None
		if !p.match('[') {
			return nil, fmt.Errorf("expected '[' after Optional")
		}
		inner, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if !p.match(']') {
			return nil, fmt.Errorf("expected ']' after Optional type argument")
		}
		return Optional(inner), nil
	case "Callable":
		return p.parseCallable()
	}

	// Check for generic arguments: name[T1, T2, ...]
	p.skipWhitespace()
	if p.match('[') {
		args, err := p.parseTypeArgs()
		if err != nil {
			return nil, err
		}
		return &NamedType{Name: name, Args: args}, nil
	}

	return &NamedType{Name: name}, nil
}

// parseFunctionType parses (T1, T2, ...) -> R
func (p *typeParser) parseFunctionType() (TypeRef, error) {
	if !p.match('(') {
		return nil, fmt.Errorf("expected '(' for function type")
	}

	// Parse parameter types
	var params []ParamType
	p.skipWhitespace()

	if p.peek() != ')' {
		for {
			p.skipWhitespace()
			paramType, err := p.parseType()
			if err != nil {
				return nil, fmt.Errorf("error parsing function parameter type: %w", err)
			}
			params = append(params, ParamType{Type: paramType})

			p.skipWhitespace()
			if !p.match(',') {
				break
			}
		}
	}

	if !p.match(')') {
		return nil, fmt.Errorf("expected ')' in function type")
	}

	p.skipWhitespace()

	// Parse arrow
	if !p.matchString("->") {
		return nil, fmt.Errorf("expected '->' in function type")
	}

	p.skipWhitespace()

	// Parse return type
	retType, err := p.parseType()
	if err != nil {
		return nil, fmt.Errorf("error parsing function return type: %w", err)
	}

	return &FunctionType{Params: params, Return: retType}, nil
}

// parseCallable parses Callable[[T1, T2], R] syntax (typing module style).
func (p *typeParser) parseCallable() (TypeRef, error) {
	if !p.match('[') {
		return nil, fmt.Errorf("expected '[' after Callable")
	}

	p.skipWhitespace()

	// Parse parameter types list: [T1, T2, ...]
	var params []ParamType
	if p.match('[') {
		p.skipWhitespace()
		if p.peek() != ']' {
			for {
				p.skipWhitespace()
				paramType, err := p.parseType()
				if err != nil {
					return nil, fmt.Errorf("error parsing Callable parameter type: %w", err)
				}
				params = append(params, ParamType{Type: paramType})

				p.skipWhitespace()
				if !p.match(',') {
					break
				}
			}
		}
		if !p.match(']') {
			return nil, fmt.Errorf("expected ']' after Callable parameter types")
		}
	} else if p.matchString("...") {
		// Callable[..., R] - variadic callable
		// Represented as empty params with a marker
		// For now, just treat as empty params
	} else {
		return nil, fmt.Errorf("expected '[' or '...' for Callable parameters")
	}

	p.skipWhitespace()
	if !p.match(',') {
		return nil, fmt.Errorf("expected ',' after Callable parameters")
	}

	p.skipWhitespace()

	// Parse return type
	retType, err := p.parseType()
	if err != nil {
		return nil, fmt.Errorf("error parsing Callable return type: %w", err)
	}

	if !p.match(']') {
		return nil, fmt.Errorf("expected ']' after Callable")
	}

	return &FunctionType{Params: params, Return: retType}, nil
}

// parseTypeArgs parses comma-separated type arguments within brackets.
// The opening '[' should already be consumed.
func (p *typeParser) parseTypeArgs() ([]TypeRef, error) {
	var args []TypeRef

	for {
		p.skipWhitespace()

		// Handle empty brackets
		if p.peek() == ']' {
			break
		}

		arg, err := p.parseType()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		p.skipWhitespace()
		if !p.match(',') {
			break
		}
	}

	if !p.match(']') {
		return nil, fmt.Errorf("expected ']' in generic type")
	}

	return args, nil
}

// parseIdent parses an identifier (type name).
// Supports dotted names like module.Type.
func (p *typeParser) parseIdent() string {
	start := p.pos

	// First character must be letter or underscore
	if p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	} else {
		return ""
	}

	// Subsequent characters can be letters, digits, underscores, or dots (for qualified names)
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' || ch == '.' {
			p.pos++
		} else {
			break
		}
	}

	return p.input[start:p.pos]
}

// Helper methods for parsing

func (p *typeParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *typeParser) match(c byte) bool {
	p.skipWhitespace()
	if p.pos < len(p.input) && p.input[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *typeParser) matchString(s string) bool {
	p.skipWhitespace()
	if p.pos+len(s) <= len(p.input) && p.input[p.pos:p.pos+len(s)] == s {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *typeParser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}
