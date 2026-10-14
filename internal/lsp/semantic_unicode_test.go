package lsp

import (
	"strings"
	"testing"
)

// Every identifier-like token must cover exactly one identifier: a byte
// length would overrun into the following punctuation for non-ASCII names.
func TestSemanticTokens_UnicodeIdentifierLengths(t *testing.T) {
	content := "π = 1\ndef é(ж, *ф, э=2):\n    return π.ñ + é.ü(ж=1) + ж\nfor ю in [π]:\n    pass\nload(\":a.bzl\", щ = \"x\")\n"
	lines := strings.Split(content, "\n")
	for _, tok := range tokenizeContent(content) {
		if tok.Type == TokenKeyword || tok.Type == TokenComment || tok.Type == TokenString || tok.Type == TokenNumber {
			continue
		}
		u := utf16Units(lines[tok.Line])
		end := int(tok.StartChar + tok.Length)
		if end > len(u) {
			t.Errorf("token %+v overruns line %q", tok, lines[tok.Line])
			continue
		}
		text := string(u[tok.StartChar:end])
		for _, r := range text {
			if !isIdentRune(r) {
				t.Errorf("token %+v covers %q in line %q", tok, text, lines[tok.Line])
				break
			}
		}
	}
}

func utf16Units(s string) []rune {
	var out []rune
	for _, r := range s {
		out = append(out, r)
		if utf16Width(r) == 2 {
			out = append(out, 0xFFFD)
		}
	}
	return out
}
