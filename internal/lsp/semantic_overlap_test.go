package lsp

import (
	"context"
	"encoding/json"
	"testing"
)

func sameRangeCount(tokens []SemanticToken) int {
	n := 0
	for i, a := range tokens {
		for _, b := range tokens[i+1:] {
			if a.Line == b.Line && a.StartChar == b.StartChar && a.Length == b.Length {
				n++
			}
		}
	}
	return n
}

func TestSemanticTokens_LoadWithoutAliasDoesNotOverlap(t *testing.T) {
	content := "load(\":f.bzl\", \"foo\")\n"
	if n := sameRangeCount(tokenizeContent(content)); n != 0 {
		t.Errorf("%d token pairs share a range without overlappingTokenSupport", n)
	}
	if n := sameRangeCount(tokenizeContentWith(content, true)); n != 1 {
		t.Errorf("with overlappingTokenSupport the string and variable tokens are both kept, got %d overlapping pairs", n)
	}
	// An alias gives the two names different ranges: nothing to merge.
	aliased := "load(\":f.bzl\", a = \"foo\")\n"
	if len(tokenizeContent(aliased)) != len(tokenizeContentWith(aliased, true)) {
		t.Error("aliased load tokens must not be dropped")
	}
}

func TestSemanticTokens_OverlapFollowsClientCapability(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		server := NewServer(nil)
		flag := "false"
		if overlap {
			flag = "true"
		}
		params := `{"processId":null,"rootUri":"file:///test","capabilities":{"textDocument":{"semanticTokens":{"requests":{},"tokenTypes":[],"tokenModifiers":[],"formats":["relative"],"overlappingTokenSupport":` + flag + `}}}}`
		if _, err := server.Handle(context.Background(), &Request{Method: "initialize", ID: rawID(1), Params: json.RawMessage(params)}); err != nil {
			t.Fatal(err)
		}
		if _, err := server.Handle(context.Background(), &Request{Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
			t.Fatal(err)
		}
		openDocumentForSemantic(t, server, "file:///l.bzl", "load(\":f.bzl\", \"foo\")\n")
		data := requestSemanticTokensFull(t, server, "file:///l.bzl").Data
		// 5 values per token: the load keyword, the module string, the
		// exported name (only when overlapping) and the local name.
		want := 3
		if overlap {
			want = 4
		}
		if len(data)/5 != want {
			t.Errorf("overlap=%v: %d tokens, want %d", overlap, len(data)/5, want)
		}
	}
}
