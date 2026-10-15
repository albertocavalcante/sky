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

func TestUnionOptionalFieldsOmittedWhenAbsent(t *testing.T) {
	tests := map[string]any{
		"SemanticTokensOptions":              SemanticTokensOptions{},
		"ClientSemanticTokensOptions":        ClientSemanticTokensRequestOptions{},
		"NotebookDocumentFilterNotebookType": NotebookDocumentFilterNotebookType{},
	}
	for name, v := range tests {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for k, raw := range m {
			if string(raw) == "null" {
				t.Errorf("%s: absent field %q re-marshaled as null: %s", name, k, b)
			}
		}
	}
	// A present value must still be emitted.
	b, err := json.Marshal(SemanticTokensOptions{Range: Or_Literal_bool{Value: true}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if string(m["range"]) != "true" {
		t.Errorf("range = %s, want true (%s)", m["range"], b)
	}
}

func TestUnionRoundTripOmitsAbsentFields(t *testing.T) {
	in := `{"legend":{"tokenTypes":[],"tokenModifiers":[]}}`
	var o SemanticTokensOptions
	if err := json.Unmarshal([]byte(in), &o); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != in {
		t.Errorf("round trip = %s, want %s", b, in)
	}
}

func TestUnionContentChangeRejectsMalformed(t *testing.T) {
	for _, in := range []string{
		`{}`,
		`{"text":"x","range":"bad"}`,
		`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}`,
		`"x"`,
		`[]`,
	} {
		var c TextDocumentContentChangeEvent
		if err := json.Unmarshal([]byte(in), &c); err == nil {
			t.Errorf("%s accepted as %#v, want error", in, c.Value)
		}
	}
	// Valid forms stay valid.
	for in, want := range map[string]string{
		`{"text":"x"}`: "whole",
		`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"rangeLength":1,"text":"x"}`: "partial",
		`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"text":""}`:                  "partial",
	} {
		var c TextDocumentContentChangeEvent
		if err := json.Unmarshal([]byte(in), &c); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		got := "whole"
		if _, ok := c.Value.(TextDocumentContentChangePartial); ok {
			got = "partial"
		}
		if got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestUnionContentChangeRequiredNull(t *testing.T) {
	type U = Or_TextDocumentContentChangePartial_TextDocumentContentChangeWholeDocument
	for _, in := range []string{
		`{"text":null}`,
		`{"range":null,"text":"x"}`,
		`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"text":null}`,
	} {
		var u U
		if err := json.Unmarshal([]byte(in), &u); err == nil {
			t.Errorf("%s: decoded as %#v, want error", in, u.Value)
		}
	}
}

func TestUnionContentChangeValidForms(t *testing.T) {
	type U = Or_TextDocumentContentChangePartial_TextDocumentContentChangeWholeDocument
	rng := `{"start":{"line":1,"character":2},"end":{"line":1,"character":3}}`
	var u U
	if err := json.Unmarshal([]byte(`{"text":"x"}`), &u); err != nil {
		t.Fatal(err)
	}
	if v, ok := u.Value.(TextDocumentContentChangeWholeDocument); !ok || v.Text != "x" {
		t.Errorf("whole: got %#v", u.Value)
	}
	u = U{}
	if err := json.Unmarshal([]byte(`{"range":`+rng+`,"text":"y"}`), &u); err != nil {
		t.Fatal(err)
	}
	if v, ok := u.Value.(TextDocumentContentChangePartial); !ok || v.Text != "y" || v.Range.End.Character != 3 {
		t.Errorf("partial: got %#v", u.Value)
	}
	u = U{}
	if err := json.Unmarshal([]byte(`{"range":`+rng+`,"rangeLength":4,"text":"z"}`), &u); err != nil {
		t.Fatal(err)
	}
	if v, ok := u.Value.(TextDocumentContentChangePartial); !ok || v.RangeLength != 4 {
		t.Errorf("rangeLength: got %#v", u.Value)
	}
}

// A nullable required field (lspls spells it as a pointer) must still accept null.
func TestUnionNullableRequiredFieldAccepted(t *testing.T) {
	var v OptionalVersionedTextDocumentIdentifier
	if err := json.Unmarshal([]byte(`{"uri":"file:///a","version":null}`), &v); err != nil {
		t.Fatal(err)
	}
}
