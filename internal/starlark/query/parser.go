package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Parse parses a query string into an AST.
//
// Examples:
//
//	"//..."                           -> LiteralExpr
//	"defs(//...)"                     -> CallExpr{Func:"defs", Args:[LiteralExpr]}
//	"filter(\"pat\", defs(//...))"    -> CallExpr{Func:"filter", Args:[StringExpr, CallExpr]}
//	"//a/... + //b/..."               -> BinaryExpr{Op:"+", Left:Literal, Right:Literal}
func Parse(query string) (Expr, error) {
	p := &parser{
		input: query,
		pos:   0,
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected character at position %d: %q", p.pos, p.input[p.pos])
	}
	return expr, nil
}

// parser is a simple recursive descent parser for query expressions.
type parser struct {
	input string
	pos   int
}

// parseExpr parses an expression, which may be a binary expression.
func (p *parser) parseExpr() (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return p.parseBinaryRHS(left)
}

// parseBinaryRHS parses the right-hand side of a binary expression.
func (p *parser) parseBinaryRHS(left Expr) (Expr, error) {
	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return left, nil
		}

		op := p.peekChar()
		if op != '+' && op != '-' && op != '^' {
			return left, nil
		}
		p.pos++ // consume operator

		p.skipWhitespace()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpr{
			Op:    string(op),
			Left:  left,
			Right: right,
		}
	}
}

// parsePrimary parses a primary expression (literal, call, or string).
func (p *parser) parsePrimary() (Expr, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of input")
	}

	// Check for string literal
	if p.peekChar() == '"' {
		return p.parseString()
	}

	// Check for parenthesized expression
	if p.peekChar() == '(' {
		p.pos++ // consume '('
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.peekChar() != ')' {
			return nil, fmt.Errorf("expected ')' at position %d", p.pos)
		}
		p.pos++ // consume ')'
		return expr, nil
	}

	// Parse identifier or pattern
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		// Allow alphanumeric, underscore, and pattern characters
		if isIdentChar(ch) || isPatternChar(ch) {
			p.pos++
		} else {
			break
		}
	}

	if p.pos == start {
		return nil, fmt.Errorf("expected identifier or pattern at position %d", p.pos)
	}

	token := p.input[start:p.pos]
	p.skipWhitespace()

	// Check if this is a function call
	if p.pos < len(p.input) && p.peekChar() == '(' {
		p.pos++ // consume '('
		args, err := p.parseArgs()
		if err != nil {
			return nil, err
		}
		return &CallExpr{
			Func: token,
			Args: args,
		}, nil
	}

	// It's a literal pattern
	return &LiteralExpr{Pattern: token}, nil
}

// parseArgs parses function arguments.
func (p *parser) parseArgs() ([]Expr, error) {
	var args []Expr

	p.skipWhitespace()
	if p.pos < len(p.input) && p.peekChar() == ')' {
		p.pos++ // consume ')'
		return args, nil
	}

	for {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("expected ')' or ',' at position %d", p.pos)
		}

		ch := p.peekChar()
		if ch == ')' {
			p.pos++ // consume ')'
			return args, nil
		}
		if ch == ',' {
			p.pos++ // consume ','
			continue
		}
		return nil, fmt.Errorf("expected ')' or ',' at position %d, got %q", p.pos, ch)
	}
}

// parseString parses a double-quoted string literal.
func (p *parser) parseString() (Expr, error) {
	if p.peekChar() != '"' {
		return nil, fmt.Errorf("expected '\"' at position %d", p.pos)
	}
	p.pos++ // consume opening quote

	var sb strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '"' {
			p.pos++ // consume closing quote
			return &StringExpr{Value: sb.String()}, nil
		}
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos++ // consume backslash
			next := p.input[p.pos]
			switch next {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte(next)
			}
			p.pos++
			continue
		}
		sb.WriteByte(ch)
		p.pos++
	}
	return nil, fmt.Errorf("unterminated string literal")
}

// peekChar returns the current character without advancing.
func (p *parser) peekChar() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

// skipWhitespace advances past any whitespace characters.
func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

// isIdentChar returns true if ch is a valid identifier character.
func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_'
}

// isPatternChar returns true if ch is a valid pattern character.
func isPatternChar(ch byte) bool {
	return ch == '/' || ch == ':' || ch == '.' || ch == '*' || ch == '@' || ch == '-'
}
