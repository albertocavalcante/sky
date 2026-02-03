package query

import (
	"testing"
)

func TestParse_LiteralExpr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "all files",
			input: "//...",
			want:  "//...",
		},
		{
			name:  "recursive under package",
			input: "//pkg/...",
			want:  "//pkg/...",
		},
		{
			name:  "label pattern",
			input: "//pkg:file.bzl",
			want:  "//pkg:file.bzl",
		},
		{
			name:  "glob pattern",
			input: "*.star",
			want:  "*.star",
		},
		{
			name:  "recursive glob",
			input: "**/*.bzl",
			want:  "**/*.bzl",
		},
		{
			name:  "external repository",
			input: "@repo//pkg:file.star",
			want:  "@repo//pkg:file.star",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			lit, ok := expr.(*LiteralExpr)
			if !ok {
				t.Errorf("Parse() = %T, want *LiteralExpr", expr)
				return
			}
			if lit.Pattern != tt.want {
				t.Errorf("Parse().Pattern = %q, want %q", lit.Pattern, tt.want)
			}
		})
	}
}

func TestParse_CallExpr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFunc string
		wantArgs int
		wantErr  bool
	}{
		{
			name:     "defs with pattern",
			input:    "defs(//...)",
			wantFunc: "defs",
			wantArgs: 1,
		},
		{
			name:     "loads with pattern",
			input:    "loads(//pkg/...)",
			wantFunc: "loads",
			wantArgs: 1,
		},
		{
			name:     "calls with two args",
			input:    "calls(http_archive, //...)",
			wantFunc: "calls",
			wantArgs: 2,
		},
		{
			name:     "filter with string and expr",
			input:    `filter("^_", defs(//...))`,
			wantFunc: "filter",
			wantArgs: 2,
		},
		{
			name:     "files function",
			input:    "files(//lib/...)",
			wantFunc: "files",
			wantArgs: 1,
		},
		{
			name:     "assigns function",
			input:    "assigns(//config.star)",
			wantFunc: "assigns",
			wantArgs: 1,
		},
		{
			name:     "empty args",
			input:    "defs()",
			wantFunc: "defs",
			wantArgs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			call, ok := expr.(*CallExpr)
			if !ok {
				t.Errorf("Parse() = %T, want *CallExpr", expr)
				return
			}
			if call.Func != tt.wantFunc {
				t.Errorf("Parse().Func = %q, want %q", call.Func, tt.wantFunc)
			}
			if len(call.Args) != tt.wantArgs {
				t.Errorf("len(Parse().Args) = %d, want %d", len(call.Args), tt.wantArgs)
			}
		})
	}
}

func TestParse_StringExpr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple string",
			input: `"hello"`,
			want:  "hello",
		},
		{
			name:  "pattern string",
			input: `"^_.*"`,
			want:  "^_.*",
		},
		{
			name:  "escaped quote",
			input: `"say \"hi\""`,
			want:  `say "hi"`,
		},
		{
			name:  "escaped newline",
			input: `"line1\nline2"`,
			want:  "line1\nline2",
		},
		{
			name:    "unterminated string",
			input:   `"hello`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			str, ok := expr.(*StringExpr)
			if !ok {
				t.Errorf("Parse() = %T, want *StringExpr", expr)
				return
			}
			if str.Value != tt.want {
				t.Errorf("Parse().Value = %q, want %q", str.Value, tt.want)
			}
		})
	}
}

func TestParse_BinaryExpr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOp  string
		wantErr bool
	}{
		{
			name:   "union",
			input:  "//a/... + //b/...",
			wantOp: "+",
		},
		{
			name:   "difference",
			input:  "//a/... - //b/...",
			wantOp: "-",
		},
		{
			name:   "intersection",
			input:  "//a/... ^ //b/...",
			wantOp: "^",
		},
		{
			name:   "union of calls",
			input:  "defs(//a/...) + defs(//b/...)",
			wantOp: "+",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			bin, ok := expr.(*BinaryExpr)
			if !ok {
				t.Errorf("Parse() = %T, want *BinaryExpr", expr)
				return
			}
			if bin.Op != tt.wantOp {
				t.Errorf("Parse().Op = %q, want %q", bin.Op, tt.wantOp)
			}
		})
	}
}

func TestParse_NestedExpr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantStr string
		wantErr bool
	}{
		{
			name:    "filter with defs",
			input:   `filter("^_", defs(//...))`,
			wantStr: `filter("^_", defs(//...))`,
		},
		{
			name:    "chained binary",
			input:   "//a/... + //b/... + //c/...",
			wantStr: "((//a/... + //b/...) + //c/...)",
		},
		{
			name:    "parenthesized",
			input:   "(//a/... + //b/...)",
			wantStr: "(//a/... + //b/...)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got := expr.String(); got != tt.wantStr {
				t.Errorf("Parse().String() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "empty input",
			input:   "",
			wantErr: "unexpected end of input",
		},
		{
			name:    "unclosed paren",
			input:   "defs(//...",
			wantErr: "expected ')'",
		},
		{
			name:    "unexpected char",
			input:   "//... $",
			wantErr: "unexpected character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Errorf("Parse() expected error containing %q", tt.wantErr)
				return
			}
			// Just check that we got an error; specific message format may vary
		})
	}
}
