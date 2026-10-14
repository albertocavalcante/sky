package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/albertocavalcante/sky/internal/protocol"
)

func doRefs(t *testing.T, code string, line, char uint32, includeDecl bool) []protocol.Location {
	t.Helper()
	s := NewServer(nil)
	uri := "file:///refs.star"
	openDoc(s, uri, code)
	params, _ := json.Marshal(protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: line, Character: char},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: includeDecl},
	})
	res, err := s.Handle(context.Background(), &Request{Method: "textDocument/references", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	locs, _ := res.([]protocol.Location)
	return locs
}

func TestReferences_ExcludeDeclarationKeepsSameLineUses(t *testing.T) {
	// The declaration shares a line with a use on the right-hand side.
	code := "x = 1\nx = x + 1\nprint(x)\n"
	locs := doRefs(t, code, 0, 0, false)
	// Declarations (line 0, line 1 LHS) are dropped; the RHS use on line 1
	// and print(x) remain.
	if len(locs) != 2 {
		t.Fatalf("got %d refs: %+v", len(locs), locs)
	}
	if locs[0].Range.Start.Line != 1 || locs[0].Range.Start.Character != 4 {
		t.Errorf("first ref = %+v, want 1:4", locs[0].Range)
	}
}

func TestReferences_NestedBodiesAndKwargs(t *testing.T) {
	code := `def f(items):
    for i in items:
        if i:
            total = i
            g(total = total)
    return [total for total in items]
`
	// total: assignment (decl), kwarg value use, comprehension body use and var (decl).
	incl := doRefs(t, code, 3, 14, true)
	excl := doRefs(t, code, 3, 14, false)
	if len(incl) != 4 {
		t.Errorf("include decl: got %d, want 4: %+v", len(incl), incl)
	}
	for _, l := range excl {
		if l.Range.Start.Line == 4 && l.Range.Start.Character == 14 {
			t.Errorf("keyword-argument name reported as reference: %+v", l)
		}
	}
	if len(excl) != 2 {
		t.Errorf("exclude decl: got %d, want 2: %+v", len(excl), excl)
	}
}

func TestReferences_FunctionDeclaration(t *testing.T) {
	code := "s = '😀'; x = 1\ndef foo():\n    return foo\n"
	locs := doRefs(t, code, 1, 5, true)
	if len(locs) != 2 {
		t.Fatalf("got %+v", locs)
	}
	if locs[0].Range.Start.Character != 4 || locs[0].Range.End.Character != 7 {
		t.Errorf("def name range = %+v", locs[0].Range)
	}
	locs = doRefs(t, code, 1, 5, false)
	if len(locs) != 1 {
		t.Errorf("exclude decl got %+v", locs)
	}
}

func doRename(t *testing.T, code string, line, char uint32, newName string) (*protocol.WorkspaceEdit, error) {
	t.Helper()
	s := NewServer(nil)
	uri := "file:///rename.star"
	openDoc(s, uri, code)
	params, _ := json.Marshal(protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position:     protocol.Position{Line: line, Character: char},
		NewName:      newName,
	})
	res, err := s.Handle(context.Background(), &Request{Method: "textDocument/rename", Params: params})
	if err != nil {
		return nil, err
	}
	edit, _ := res.(*protocol.WorkspaceEdit)
	return edit, nil
}

func TestRename_SkipsStringsCommentsKwargsAttributes(t *testing.T) {
	code := `name = 1
s = "name"
# name in a comment
f(name = name)
y = obj.name
z = name
`
	edit, err := doRename(t, code, 0, 1, "label")
	if err != nil {
		t.Fatal(err)
	}
	if edit == nil {
		t.Fatal("no edit")
	}
	var lines []uint32
	for _, e := range edit.Changes["file:///rename.star"] {
		lines = append(lines, e.Range.Start.Line)
		if e.Range.Start.Line == 3 && e.Range.Start.Character != 9 {
			t.Errorf("kwarg name edited: %+v", e.Range)
		}
	}
	want := []uint32{0, 3, 5}
	if len(lines) != len(want) {
		t.Fatalf("edited lines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("edited lines = %v, want %v", lines, want)
		}
	}
}

func TestRename_ValidatesNewName(t *testing.T) {
	for _, bad := range []string{"", "1abc", "a-b", "a b", "def", "None", "x.y"} {
		_, err := doRename(t, "x = 1\n", 0, 0, bad)
		var re *ResponseError
		if !errors.As(err, &re) || re.Code != CodeInvalidParams {
			t.Errorf("NewName %q: err = %v, want InvalidParams", bad, err)
		}
	}
	if edit, err := doRename(t, "x = 1\n", 0, 0, "_ok9"); err != nil || edit == nil {
		t.Errorf("valid name rejected: %v", err)
	}
}

func TestRename_UTF16Positions(t *testing.T) {
	code := "s = '😀'; foo = 1\nprint(foo)\n"
	edit, err := doRename(t, code, 0, 11, "bar") // 'foo' starts at UTF-16 col 10
	if err != nil || edit == nil {
		t.Fatalf("edit=%v err=%v", edit, err)
	}
	es := edit.Changes["file:///rename.star"]
	if len(es) != 2 || es[0].Range.Start.Character != 10 || es[0].Range.End.Character != 13 {
		t.Errorf("edits = %+v", es)
	}
}
