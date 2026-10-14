package protocol

import (
	"encoding/json"
	"testing"
)

func TestUnionContentChangeDiscrimination(t *testing.T) {
	var w Or_TextDocumentContentChangePartial_TextDocumentContentChangeWholeDocument
	if err := json.Unmarshal([]byte(`{"text":"x"}`), &w); err != nil {
		t.Fatal(err)
	}
	if v, ok := w.Value.(TextDocumentContentChangeWholeDocument); !ok || v.Text != "x" {
		t.Fatalf("got %#v, want WholeDocument", w.Value)
	}
	var p Or_TextDocumentContentChangePartial_TextDocumentContentChangeWholeDocument
	if err := json.Unmarshal([]byte(`{"range":{"start":{"line":1,"character":2},"end":{"line":1,"character":3}},"text":"y"}`), &p); err != nil {
		t.Fatal(err)
	}
	if v, ok := p.Value.(TextDocumentContentChangePartial); !ok || v.Text != "y" || v.Range.Start.Line != 1 {
		t.Fatalf("got %#v, want Partial", p.Value)
	}
}

func TestUnionDocumentChange(t *testing.T) {
	var u Or_CreateFile_DeleteFile_RenameFile_TextDocumentEdit
	in := `{"textDocument":{"uri":"file:///a","version":1},"edits":[{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"newText":"z"}]}`
	if err := json.Unmarshal([]byte(in), &u); err != nil {
		t.Fatal(err)
	}
	te, ok := u.Value.(TextDocumentEdit)
	if !ok || len(te.Edits) != 1 {
		t.Fatalf("got %#v, want TextDocumentEdit", u.Value)
	}
	for kind, c := range map[string][2]string{
		"create": {"CreateFile", `"uri":"file:///a"`},
		"delete": {"DeleteFile", `"uri":"file:///a"`},
		"rename": {"RenameFile", `"oldUri":"file:///a","newUri":"file:///b"`},
	} {
		want := c[0]
		var v Or_CreateFile_DeleteFile_RenameFile_TextDocumentEdit
		body := `{"kind":"` + kind + `",` + c[1] + `}`
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Fatal(err)
		}
		got := ""
		switch v.Value.(type) {
		case CreateFile:
			got = "CreateFile"
		case DeleteFile:
			got = "DeleteFile"
		case RenameFile:
			got = "RenameFile"
		}
		if got != want {
			t.Errorf("kind %s: got %T", kind, v.Value)
		}
	}
}

func TestUnionHoverMarkup(t *testing.T) {
	var h Hover
	if err := json.Unmarshal([]byte(`{"contents":{"kind":"markdown","value":"hi"}}`), &h); err != nil {
		t.Fatal(err)
	}
	if v, ok := h.Contents.Value.(MarkupContent); !ok || v.Value != "hi" {
		t.Fatalf("got %#v, want MarkupContent", h.Contents.Value)
	}
	var h2 Hover
	if err := json.Unmarshal([]byte(`{"contents":{"language":"python","value":"x=1"}}`), &h2); err != nil {
		t.Fatal(err)
	}
	if h2.Contents.Value == nil {
		t.Fatal("nil contents")
	}
}

func TestUnionTextEditVariants(t *testing.T) {
	var u Or_AnnotatedTextEdit_SnippetTextEdit_TextEdit
	in := `{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"newText":"a"}`
	if err := json.Unmarshal([]byte(in), &u); err != nil {
		t.Fatal(err)
	}
	if _, ok := u.Value.(TextEdit); !ok {
		t.Fatalf("got %T, want TextEdit", u.Value)
	}
	in = `{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"newText":"a","annotationId":"x"}`
	if err := json.Unmarshal([]byte(in), &u); err != nil {
		t.Fatal(err)
	}
	if _, ok := u.Value.(AnnotatedTextEdit); !ok {
		t.Fatalf("got %T, want AnnotatedTextEdit", u.Value)
	}
}

func TestUnionDocumentFilterAlias(t *testing.T) {
	var f Or_NotebookCellTextDocumentFilter_TextDocumentFilter
	if err := json.Unmarshal([]byte(`{"scheme":"file"}`), &f); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Value.(TextDocumentFilter); !ok {
		t.Fatalf("got %T, want TextDocumentFilter", f.Value)
	}
	if sc, ok := f.Value.(TextDocumentFilter).Value.(TextDocumentFilterScheme); !ok || sc.Scheme != "file" {
		t.Fatalf("lost scheme: %#v", f.Value)
	}
	var n Or_NotebookCellTextDocumentFilter_TextDocumentFilter
	if err := json.Unmarshal([]byte(`{"notebook":"jupyter"}`), &n); err != nil {
		t.Fatal(err)
	}
	if _, ok := n.Value.(NotebookCellTextDocumentFilter); !ok {
		t.Fatalf("got %T, want NotebookCellTextDocumentFilter", n.Value)
	}
}

func TestUnionOptionalKeyTieBreak(t *testing.T) {
	var u Or_NotebookDocumentSyncOptions_NotebookDocumentSyncRegistrationOptions
	if err := json.Unmarshal([]byte(`{"notebookSelector":[],"id":"x"}`), &u); err != nil {
		t.Fatal(err)
	}
	r, ok := u.Value.(NotebookDocumentSyncRegistrationOptions)
	if !ok || r.Id != "x" {
		t.Fatalf("got %#v, want registration options with id", u.Value)
	}
	var o Or_NotebookDocumentSyncOptions_NotebookDocumentSyncRegistrationOptions
	if err := json.Unmarshal([]byte(`{"notebookSelector":[]}`), &o); err != nil {
		t.Fatal(err)
	}
	if _, ok := o.Value.(NotebookDocumentSyncOptions); !ok {
		t.Fatalf("got %T, want plain options", o.Value)
	}
}
