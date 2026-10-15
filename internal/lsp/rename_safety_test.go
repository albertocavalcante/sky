package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
)

func renameLines(t *testing.T, code string, line, char uint32, name string) (map[uint32]bool, error) {
	t.Helper()
	edit, err := doRename(t, code, line, char, name)
	if err != nil {
		return nil, err
	}
	lines := map[uint32]bool{}
	if edit != nil {
		for _, e := range edit.Changes["file:///rename.star"] {
			lines[e.Range.Start.Line] = true
		}
	}
	return lines, nil
}

func requireRequestFailed(t *testing.T, err error, lines map[uint32]bool) {
	t.Helper()
	if err == nil {
		t.Fatalf("rename must be refused with RequestFailed; got edits on lines %v", lines)
	}
	if re, ok := err.(*ResponseError); !ok || re.Code != CodeRequestFailed {
		t.Errorf("err = %v, want RequestFailed", err)
	}
}

func TestRename_ShadowedCalleeIsRefused(t *testing.T) {
	// f(name=1) inside invoke(f) calls the parameter f, which may be any
	// function, so neither editing nor skipping the label is safe.
	code := "def f(name):\n    return name\n\ndef invoke(f):\n    return f(name = 1)\n"
	lines, err := renameLines(t, code, 0, 7, "label")
	requireRequestFailed(t, err, lines)
	// The refusal must name the reason the call cannot be resolved.
	if !strings.Contains(err.Error(), "the callee is shadowed") {
		t.Errorf("refusal must explain that the callee is shadowed, got: %v", err)
	}
}

func TestRename_ParenthesizedCalleeIsResolved(t *testing.T) {
	code := "def f(name):\n    return name\n\n(f)(name = 1)\n((f))(name = 2)\n"
	lines, err := renameLines(t, code, 0, 7, "label")
	if err != nil {
		t.Fatal(err)
	}
	if !lines[3] || !lines[4] {
		t.Errorf("labels of parenthesized calls to f must be renamed, edited lines: %v", lines)
	}
}

func TestRename_ParenthesizedUnresolvedCalleeIsRefused(t *testing.T) {
	for _, code := range []string{
		"def f(name):\n    return name\n\ng = f\n(g)(name = 1)\n",
		"(lambda name: name)(name = 1)\n",
		"def f(name):\n    return name\n\nfs = [f]\nfs[0](name = 1)\n",
		"def f(name):\n    return name\n\ndef mk():\n    return f\n\nmk()(name = 1)\n",
	} {
		char := uint32(7)
		if code[0] == '(' {
			char = 9
		}
		lines, err := renameLines(t, code, 0, char, "label")
		requireRequestFailed(t, err, lines)
	}
}

func TestRename_KeywordUnpackingIsRefused(t *testing.T) {
	for _, code := range []string{
		"def f(name):\n    return name\n\nf(**{\"name\": 1})\n",
		"def f(name):\n    return name\n\nkw = {\"name\": 1}\nf(**kw)\n",
		"def f(name):\n    return name\n\n(f)(**{\"name\": 1})\n",
	} {
		lines, err := renameLines(t, code, 0, 7, "label")
		requireRequestFailed(t, err, lines)
	}
}

func TestRename_KeywordUnpackingToForeignCalleesDoesNotBlock(t *testing.T) {
	code := "def macro(name, **kwargs):\n    native.cc_library(name = name, **kwargs)\n    dict(**kwargs)\n\ndef g(a):\n    return a\n\ng(**{\"a\": 1})\n"
	// Guard against over-refusal: the rename is allowed, and it edits exactly
	// the declaration and the value use, never the keyword label of the
	// foreign call (line 1, column 22).
	edit, err := doRename(t, code, 0, 11, "label")
	if err != nil {
		t.Fatalf("unpacking into foreign or unrelated callees must not block: %v", err)
	}
	var got []string
	for _, e := range edit.Changes["file:///rename.star"] {
		got = append(got, fmt.Sprintf("%d:%d-%d", e.Range.Start.Line, e.Range.Start.Character, e.Range.End.Character))
	}
	if want := []string{"0:10-14", "1:29-33"}; !slices.Equal(got, want) {
		t.Errorf("edits = %v, want %v", got, want)
	}
}

func TestRename_VariadicParamHasNoKeywordLabel(t *testing.T) {
	code := "def f(**name):\n    return name[\"name\"]\n\nf(name = 1)\n"
	lines, err := renameLines(t, code, 0, 8, "label")
	if err != nil {
		t.Fatal(err)
	}
	if lines[3] {
		t.Error("f(name=1) populates **name; its label must not be edited")
	}
}

func TestRename_LambdaAndAliasAreRefused(t *testing.T) {
	for _, code := range []string{
		"f = lambda name: name\nf(name = 1)\n",
		"def f(name):\n    return name\n\ng = f\ng(name = 1)\n",
	} {
		char := uint32(11)
		if code[0] == 'd' {
			char = 6
		}
		lines, err := renameLines(t, code, 0, char, "label")
		if err == nil {
			t.Errorf("rename must be refused (or the alias call edited); got edits on %v for %q", lines, code)
			continue
		}
		if re, ok := err.(*ResponseError); !ok || re.Code != CodeRequestFailed {
			t.Errorf("err = %v, want RequestFailed", err)
		}
	}
}

func TestRename_ForeignKeywordCallsDoNotBlockParamRename(t *testing.T) {
	code := "def macro(name):\n    native.cc_library(name = name)\n    other(name = name)\n"
	lines, err := renameLines(t, code, 0, 11, "label")
	if err != nil {
		t.Fatalf("calls to unrelated callees must not block: %v", err)
	}
	if lines[1] != true || lines[2] != true {
		t.Errorf("value uses should be renamed: %v", lines)
	}
}

func TestRename_SuccessiveUnicodeRenames(t *testing.T) {
	// After renaming x to π the buffer holds π; it must be selectable again.
	code := "π = 1\nprint(π)\n"
	edit, err := doRename(t, code, 1, 7, "ω")
	if err != nil {
		t.Fatal(err)
	}
	if edit == nil || len(edit.Changes["file:///rename.star"]) != 2 {
		t.Fatalf("want 2 edits for π, got %+v", edit)
	}
	w, r := getWordAndRangeAtPosition("a = élan + 𝒙1\n", 0, 12)
	if w != "𝒙1" || r.Start.Character != 11 || r.End.Character != 14 {
		t.Errorf("astral word = %q %+v", w, r)
	}
}

func TestRename_NameCapture(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		line    uint32
		char    uint32
		newName string
		refused bool
	}{
		{"global into param scope", "x = 1\ndef f(y):\n    return x + y\n", 0, 0, "y", true},
		{"global onto other global", "x = 1\ny = 2\nprint(x + y)\n", 0, 0, "y", true},
		{"local onto free global use", "y = 5\ndef f(x):\n    return x + y\n", 1, 6, "y", true},
		{"local onto its own sibling local", "def f(x):\n    y = 1\n    return x + y\n", 0, 6, "y", true},
		{"global onto builtin used in a def", "x = 1\ndef f(l):\n    return len(l) + x\n", 0, 0, "len", true},
		{"loop var captured by comprehension", "x = 1\nr = [y for y in range(3)]\nprint(x)\n", 0, 0, "y", false},
		{"global onto name only bound elsewhere", "x = 1\ndef g(y):\n    return y\nprint(x)\n", 0, 0, "y", false},
		{"local onto unrelated global", "y = 5\ndef f(x):\n    return x\n", 1, 6, "y", false},
		{"sibling functions", "def f(a):\n    return a\ndef g(b):\n    return b\n", 0, 6, "b", false},
		{"fresh name", "x = 1\ndef f(y):\n    return x + y\n", 0, 0, "z", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := renameLines(t, tt.code, tt.line, tt.char, tt.newName)
			if tt.refused {
				requireRequestFailed(t, err, lines)
				return
			}
			if err != nil {
				t.Fatalf("rename must be allowed: %v", err)
			}
			if len(lines) == 0 {
				t.Fatal("no edits")
			}
		})
	}
}

func TestWordAtPosition_Unicode(t *testing.T) {
	// After renaming x to π, hover/definition/references must still find it.
	if got := getWordAtPosition("π = 1\nprint(π)\n", 1, 7); got != "π" {
		t.Errorf("word at π (end of ident) = %q", got)
	}
	if got := getWordAtPosition("a = élan + 𝒙1\n", 0, 12); got != "𝒙1" {
		t.Errorf("astral word = %q", got)
	}
	s := NewServer(nil)
	uri := "file:///uni.star"
	openDoc(s, uri, "π = 1\nprint(π)\n")
	params := json.RawMessage(`{"textDocument":{"uri":"file:///uni.star"},"position":{"line":1,"character":6},"context":{"includeDeclaration":true}}`)
	res, err := s.Handle(context.Background(), &Request{Method: "textDocument/references", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if locs, _ := res.([]protocol.Location); len(locs) != 2 {
		t.Errorf("references to π = %+v, want 2", res)
	}
}
