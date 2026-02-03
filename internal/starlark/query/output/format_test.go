package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    Format
		wantErr bool
	}{
		{input: "", want: FormatName},
		{input: "name", want: FormatName},
		{input: "location", want: FormatLocation},
		{input: "json", want: FormatJSON},
		{input: "count", want: FormatCount},
		{input: "invalid", wantErr: true},
		{input: "JSON", wantErr: true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFormat(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseFormat(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewFormatter(t *testing.T) {
	// Valid format
	f := NewFormatter("json")
	if f.format != FormatJSON {
		t.Errorf("NewFormatter('json') format = %q, want %q", f.format, FormatJSON)
	}

	// Invalid format defaults to name
	f = NewFormatter("invalid")
	if f.format != FormatName {
		t.Errorf("NewFormatter('invalid') format = %q, want %q", f.format, FormatName)
	}
}

func TestFormatName(t *testing.T) {
	result := &SimpleResult{
		QueryStr: "defs(//...)",
		ResultItems: []Item{
			&SimpleDef{
				SimpleItem: SimpleItem{
					ItemType: "def",
					ItemName: "my_function",
					ItemFile: "lib/utils.star",
					ItemLine: 15,
				},
				ParamNames: []string{"ctx", "deps"},
			},
			&SimpleDef{
				SimpleItem: SimpleItem{
					ItemType: "def",
					ItemName: "another_function",
					ItemFile: "lib/utils.star",
					ItemLine: 42,
				},
			},
			&SimpleDef{
				SimpleItem: SimpleItem{
					ItemType: "def",
					ItemName: "_private",
					ItemFile: "lib/internal.star",
					ItemLine: 8,
				},
			},
		},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatName)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Results are sorted by file, then line
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}

	// lib/internal.star comes before lib/utils.star alphabetically
	expected := []string{"_private", "my_function", "another_function"}
	for i, want := range expected {
		if i >= len(lines) {
			break
		}
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}

func TestFormatLocation(t *testing.T) {
	result := &SimpleResult{
		QueryStr: "defs(//...)",
		ResultItems: []Item{
			&SimpleDef{
				SimpleItem: SimpleItem{
					ItemType: "def",
					ItemName: "my_function",
					ItemFile: "lib/utils.star",
					ItemLine: 15,
				},
			},
			&SimpleDef{
				SimpleItem: SimpleItem{
					ItemType: "def",
					ItemName: "another_function",
					ItemFile: "lib/utils.star",
					ItemLine: 42,
				},
			},
		},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatLocation)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Check location format
	if !strings.HasPrefix(lines[0], "//lib/utils.star:15:") {
		t.Errorf("unexpected line 0: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "//lib/utils.star:42:") {
		t.Errorf("unexpected line 1: %q", lines[1])
	}
}

func TestFormatJSON(t *testing.T) {
	result := &SimpleResult{
		QueryStr: "defs(//...)",
		ResultItems: []Item{
			&SimpleDef{
				SimpleItem: SimpleItem{
					ItemType: "def",
					ItemName: "my_function",
					ItemFile: "lib/utils.star",
					ItemLine: 15,
				},
				ParamNames: []string{"ctx", "deps"},
				Doc:        "This is a docstring.",
			},
		},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatJSON)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Parse JSON to verify structure
	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if output.Query != "defs(//...)" {
		t.Errorf("Query = %q, want %q", output.Query, "defs(//...)")
	}
	if output.Count != 1 {
		t.Errorf("Count = %d, want 1", output.Count)
	}
	if len(output.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(output.Results))
	}

	r := output.Results[0]
	if r.Type != "def" {
		t.Errorf("Type = %q, want 'def'", r.Type)
	}
	if r.Name != "my_function" {
		t.Errorf("Name = %q, want 'my_function'", r.Name)
	}
	if r.File != "lib/utils.star" {
		t.Errorf("File = %q, want 'lib/utils.star'", r.File)
	}
	if r.Line != 15 {
		t.Errorf("Line = %d, want 15", r.Line)
	}
	if len(r.Params) != 2 || r.Params[0] != "ctx" || r.Params[1] != "deps" {
		t.Errorf("Params = %v, want [ctx deps]", r.Params)
	}
	if r.Docstring != "This is a docstring." {
		t.Errorf("Docstring = %q, want 'This is a docstring.'", r.Docstring)
	}
}

func TestFormatJSONWithLoad(t *testing.T) {
	result := &SimpleResult{
		QueryStr: "loads(//...)",
		ResultItems: []Item{
			&SimpleLoad{
				SimpleItem: SimpleItem{
					ItemType: "load",
					ItemName: "utils.bzl",
					ItemFile: "lib/BUILD.bazel",
					ItemLine: 3,
				},
				ModulePath: "//lib:utils.bzl",
				ImportedSymbols: map[string]string{
					"my_rule": "my_rule",
					"alias":   "original_name",
				},
			},
		},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatJSON)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if len(output.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(output.Results))
	}

	r := output.Results[0]
	if r.Type != "load" {
		t.Errorf("Type = %q, want 'load'", r.Type)
	}
	if r.Module != "//lib:utils.bzl" {
		t.Errorf("Module = %q, want '//lib:utils.bzl'", r.Module)
	}
	if len(r.Symbols) != 2 {
		t.Errorf("len(Symbols) = %d, want 2", len(r.Symbols))
	}
}

func TestFormatJSONWithCall(t *testing.T) {
	result := &SimpleResult{
		QueryStr: "calls(http_archive, //...)",
		ResultItems: []Item{
			&SimpleCall{
				SimpleItem: SimpleItem{
					ItemType: "call",
					ItemName: "rules_go",
					ItemFile: "WORKSPACE.bazel",
					ItemLine: 10,
				},
				FunctionName: "http_archive",
			},
		},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatJSON)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if len(output.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(output.Results))
	}

	r := output.Results[0]
	if r.Type != "call" {
		t.Errorf("Type = %q, want 'call'", r.Type)
	}
	if r.Function != "http_archive" {
		t.Errorf("Function = %q, want 'http_archive'", r.Function)
	}
}

func TestFormatCount(t *testing.T) {
	result := &SimpleResult{
		QueryStr: "defs(//...)",
		ResultItems: []Item{
			&SimpleItem{ItemType: "def", ItemName: "a"},
			&SimpleItem{ItemType: "def", ItemName: "b"},
			&SimpleItem{ItemType: "def", ItemName: "c"},
		},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatCount)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "3" {
		t.Errorf("count output = %q, want '3'", got)
	}
}

func TestFormatCountEmpty(t *testing.T) {
	result := &SimpleResult{
		QueryStr:    "defs(//empty/...)",
		ResultItems: []Item{},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatCount)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "0" {
		t.Errorf("count output = %q, want '0'", got)
	}
}

func TestFormatNameEmpty(t *testing.T) {
	result := &SimpleResult{
		QueryStr:    "defs(//empty/...)",
		ResultItems: []Item{},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatName)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	result := &SimpleResult{
		QueryStr:    "defs(//empty/...)",
		ResultItems: []Item{},
	}

	var buf bytes.Buffer
	f := NewFormatterWithFormat(FormatJSON)
	if err := f.Write(&buf, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if output.Count != 0 {
		t.Errorf("Count = %d, want 0", output.Count)
	}
	if len(output.Results) != 0 {
		t.Errorf("len(Results) = %d, want 0", len(output.Results))
	}
}

func TestFormatDeterministicOrdering(t *testing.T) {
	// Create items in random order
	result := &SimpleResult{
		QueryStr: "defs(//...)",
		ResultItems: []Item{
			&SimpleItem{ItemType: "def", ItemName: "z_func", ItemFile: "a.star", ItemLine: 5},
			&SimpleItem{ItemType: "def", ItemName: "a_func", ItemFile: "b.star", ItemLine: 1},
			&SimpleItem{ItemType: "def", ItemName: "m_func", ItemFile: "a.star", ItemLine: 1},
		},
	}

	// Run multiple times to ensure deterministic output
	var firstOutput string
	for i := 0; i < 3; i++ {
		var buf bytes.Buffer
		f := NewFormatterWithFormat(FormatName)
		if err := f.Write(&buf, result); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if i == 0 {
			firstOutput = buf.String()
		} else if buf.String() != firstOutput {
			t.Errorf("non-deterministic output on iteration %d:\ngot: %q\nwant: %q", i, buf.String(), firstOutput)
		}
	}

	// Verify ordering: sorted by file, then line
	lines := strings.Split(strings.TrimSpace(firstOutput), "\n")
	expected := []string{"m_func", "z_func", "a_func"} // a.star:1, a.star:5, b.star:1
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}
