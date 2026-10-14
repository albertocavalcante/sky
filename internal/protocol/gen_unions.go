//go:build ignore

// gen_unions post-processes protocol.go (produced by lspls, which cannot be
// re-run hermetically) and inserts required-key (and optional-key tie-breaking) discrimination into every
// Or_* UnmarshalJSON that has two or more struct variants. Idempotent and exactly reproducible.
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
	aliases := map[string]string{} // alias name -> target Or_ type
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, sp := range gd.Specs {
			if ts, ok := sp.(*ast.TypeSpec); ok && ts.Assign.IsValid() {
				if id, ok := ts.Type.(*ast.Ident); ok && strings.HasPrefix(id.Name, "Or_") {
					aliases[ts.Name.Name] = id.Name
				}
			}
		}
	}
	// keys collects the required and optional JSON keys of a struct,
	// following embedded structs.
	var keys func(name string, seen map[string]bool) (req, opt []string)
	keys = func(name string, seen map[string]bool) (req, opt []string) {
		st := structs[name]
		if st == nil || seen[name] {
			return nil, nil
		}
		seen[name] = true
		for _, fld := range st.Fields.List {
			tag := ""
			if fld.Tag != nil {
				tag = reflect.StructTag(strings.Trim(fld.Tag.Value, "`")).Get("json")
			}
			if len(fld.Names) == 0 {
				if id, ok := fld.Type.(*ast.Ident); ok && tag == "" {
					r, o := keys(id.Name, seen)
					req, opt = append(req, r...), append(opt, o...)
				}
				continue
			}
			parts := strings.Split(tag, ",")
			if parts[0] == "" || parts[0] == "-" {
				continue
			}
			if strings.Contains(tag, "omitempty") {
				opt = append(opt, parts[0])
			} else {
				req = append(req, parts[0])
			}
		}
		sort.Strings(req)
		sort.Strings(opt)
		return req, opt
	}

	text := string(src)
	// Strip blocks from a previous run so output is a pure function of the
	// lspls-generated code (regeneration is exact and picks up generator fixes).
	text = regexp.MustCompile(`(?s)\t`+marker+`:.*?\n\tvar h0 `).ReplaceAllString(text, "\tvar h0 ")
	re := regexp.MustCompile(`(?m)^func \(t \*(Or_\w+)\) UnmarshalJSON\(x \[\]byte\) error \{\n\tif string\(x\) == "null" \{\n\t\tt\.Value = nil\n\t\treturn nil\n\t\}\n`)
	vre := regexp.MustCompile(`var h\d+ (\w+)\n`)

	// variants lists the declared variant types of every Or_ union.
	variants := map[string][]string{}
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		end := strings.Index(text[m[1]:], "\n}\n") + m[1]
		for _, vm := range vre.FindAllStringSubmatch(text[m[1]:end], -1) {
			variants[text[m[2]:m[3]]] = append(variants[text[m[2]:m[3]]], vm[1])
		}
	}
	// keySets resolves a variant to the key sets of its struct leaves,
	// expanding union aliases. ok is false if any leaf is not a struct.
	var keySets func(v string, depth int) (sets []string, ok bool)
	keySets = func(v string, depth int) ([]string, bool) {
		if depth > 8 {
			return nil, false
		}
		if structs[v] != nil {
			req, opt := keys(v, map[string]bool{})
			return []string{fmt.Sprintf("keySet{%s, %s}", strSlice(req), strSlice(opt))}, true
		}
		target, isAlias := aliases[v]
		if !isAlias {
			return nil, false
		}
		var out []string
		for _, w := range variants[target] {
			sub, ok := keySets(w, depth+1)
			if !ok {
				return nil, false
			}
			out = append(out, sub...)
		}
		return out, len(out) > 0
	}

	var sb strings.Builder
	last := 0
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		if snip, ok := special[name]; ok {
			sb.WriteString(text[last:m[1]])
			sb.WriteString(snip)
			last = m[1]
			continue
		}
		type cand struct {
			typ  string
			sets []string
		}
		var cands []cand
		for _, v := range variants[name] {
			if sets, ok := keySets(v, 0); ok {
				cands = append(cands, cand{v, sets})
			}
		}
		if len(cands) < 2 {
			continue
		}
		var b strings.Builder
		b.WriteString("\t" + marker + ": prefer the struct variant whose required keys are present (most specific wins; optional keys break ties).\n")
		b.WriteString("\t{\n\t\tbestScore := -1\n\t\tvar best any\n")
		for _, c := range cands {
			fmt.Fprintf(&b, "\t\tif s := unionRank(x, %s); s > bestScore {\n\t\t\tvar v %s\n\t\t\tif json.Unmarshal(x, &v) == nil {\n\t\t\t\tbest, bestScore = v, s\n\t\t\t}\n\t\t}\n", strings.Join(c.sets, ", "), c.typ)
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

func strSlice(ss []string) string {
	if len(ss) == 0 {
		return "nil"
	}
	q := make([]string, len(ss))
	for i, k := range ss {
		q[i] = fmt.Sprintf("%q", k)
	}
	return "[]string{" + strings.Join(q, ", ") + "}"
}
