//go:build ignore

// gen_unions post-processes protocol.go (produced by lspls, which cannot be
// re-run hermetically) and inserts required-key discrimination into every
// Or_* UnmarshalJSON that has two or more struct variants. Idempotent.
//
// Usage: go run gen_unions.go  (from internal/protocol)
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// special holds hand-written discriminators for unions whose variants are not
// plain structs (MarkedString is itself a union alias).
var special = map[string]string{
	"Or_ArrMarkedString_MarkedString_MarkupContent": "\t" + marker + ": MarkupContent has kind+value, MarkedStringWithLanguage has language+value.\n" +
		"\tif unionScore(x, \"kind\", \"value\") >= 0 {\n\t\tvar mc MarkupContent\n\t\tif err := json.Unmarshal(x, &mc); err == nil {\n\t\t\tt.Value = mc\n\t\t\treturn nil\n\t\t}\n\t}\n",
	"Or_CreateFile_DeleteFile_RenameFile_TextDocumentEdit": "\t" + marker + ": resource operations are tagged by their kind field.\n" +
		"\tvar probe struct {\n\t\tKind string `json:\"kind\"`\n\t}\n\tif err := json.Unmarshal(x, &probe); err == nil {\n\t\tswitch probe.Kind {\n" +
		"\t\tcase \"create\":\n\t\t\tvar v CreateFile\n\t\t\tif err := json.Unmarshal(x, &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\tt.Value = v\n\t\t\treturn nil\n" +
		"\t\tcase \"delete\":\n\t\t\tvar v DeleteFile\n\t\t\tif err := json.Unmarshal(x, &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\tt.Value = v\n\t\t\treturn nil\n" +
		"\t\tcase \"rename\":\n\t\t\tvar v RenameFile\n\t\t\tif err := json.Unmarshal(x, &v); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\tt.Value = v\n\t\t\treturn nil\n\t\t}\n\t}\n" +
		"\tif unionScore(x, \"edits\", \"textDocument\") >= 0 {\n\t\tvar v TextDocumentEdit\n\t\tif err := json.Unmarshal(x, &v); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tt.Value = v\n\t\treturn nil\n\t}\n",
}

const marker = "// union-discriminate"

func main() {
	src, err := os.ReadFile("protocol.go")
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "protocol.go", src, 0)
	if err != nil {
		panic(err)
	}
	structs := map[string]*ast.StructType{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range gd.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok {
				if st, ok := ts.Type.(*ast.StructType); ok {
					structs[ts.Name.Name] = st
				}
			}
		}
	}
	var required func(name string, seen map[string]bool) []string
	required = func(name string, seen map[string]bool) []string {
		st := structs[name]
		if st == nil || seen[name] {
			return nil
		}
		seen[name] = true
		var out []string
		for _, fld := range st.Fields.List {
			tag := ""
			if fld.Tag != nil {
				tag = reflect.StructTag(strings.Trim(fld.Tag.Value, "`")).Get("json")
			}
			if len(fld.Names) == 0 {
				if id, ok := fld.Type.(*ast.Ident); ok && tag == "" {
					out = append(out, required(id.Name, seen)...)
				}
				continue
			}
			parts := strings.Split(tag, ",")
			if parts[0] == "" || parts[0] == "-" || strings.Contains(tag, "omitempty") {
				continue
			}
			out = append(out, parts[0])
		}
		sort.Strings(out)
		return out
	}

	text := string(src)
	re := regexp.MustCompile(`(?m)^func \(t \*(Or_\w+)\) UnmarshalJSON\(x \[\]byte\) error \{\n\tif string\(x\) == "null" \{\n\t\tt\.Value = nil\n\t\treturn nil\n\t\}\n`)
	vre := regexp.MustCompile(`var h\d+ (\w+)\n`)
	var sb strings.Builder
	last := 0
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		end := strings.Index(text[m[1]:], "\n}\n") + m[1]
		body := text[m[1]:end]
		if strings.Contains(body, marker) {
			continue
		}
		if snip, ok := special[name]; ok && !strings.Contains(body, marker) {
			sb.WriteString(text[last:m[1]])
			sb.WriteString(snip)
			last = m[1]
			continue
		}
		var vars []string
		for _, vm := range vre.FindAllStringSubmatch(body, -1) {
			if structs[vm[1]] != nil {
				vars = append(vars, vm[1])
			}
		}
		if len(vars) < 2 {
			continue
		}
		var b strings.Builder
		b.WriteString("\t" + marker + ": prefer the struct variant whose required keys are present (most specific wins).\n")
		b.WriteString("\t{\n\t\tbestScore := -1\n\t\tvar best any\n")
		for _, v := range vars {
			keys := required(v, map[string]bool{})
			q := make([]string, len(keys))
			for i, k := range keys {
				q[i] = fmt.Sprintf("%q", k)
			}
			args := ""
			if len(q) > 0 {
				args = ", " + strings.Join(q, ", ")
			}
			fmt.Fprintf(&b, "\t\tif s := unionScore(x%s); s > bestScore {\n\t\t\tvar v %s\n\t\t\tif json.Unmarshal(x, &v) == nil {\n\t\t\t\tbest, bestScore = v, s\n\t\t\t}\n\t\t}\n", args, v)
		}
		b.WriteString("\t\tif bestScore >= 0 {\n\t\t\tt.Value = best\n\t\t\treturn nil\n\t\t}\n\t}\n")
		sb.WriteString(text[last:m[1]])
		sb.WriteString(b.String())
		last = m[1]
	}
	sb.WriteString(text[last:])
	if err := os.WriteFile("protocol.go", []byte(sb.String()), 0o644); err != nil {
		panic(err)
	}
}
