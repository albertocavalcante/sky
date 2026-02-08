// Command sync-protocol extracts LSP protocol types from gopls.
//
// gopls (the official Go language server) generates its protocol types from
// the LSP specification's metaModel.json. Since gopls uses an internal package,
// we can't import it directly. This tool extracts the types we need.
//
// Usage:
//
//	go run ./tools/sync-protocol [options]
//
// Options:
//
//	-gopls-dir    Path to golang/tools repo (default: clones to temp dir)
//	-output       Output file (default: internal/lsp/protocol_types.go)
//	-types        Comma-separated types to extract (default: InlayHint,InlayHintKind,...)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	goplsDir   = flag.String("gopls-dir", "", "Path to golang/tools repo (clones if empty)")
	outputFile = flag.String("output", "internal/lsp/protocol_types.go", "Output file path")
	typesFlag  = flag.String("types", defaultTypes, "Comma-separated types to extract")
	dryRun     = flag.Bool("dry-run", false, "Print output instead of writing file")
	verbose    = flag.Bool("verbose", false, "Verbose output")
)

const defaultTypes = "InlayHint,InlayHintKind,InlayHintParams,InlayHintLabelPart,InlayHintOptions"

const goplsRepo = "https://github.com/golang/tools.git"
const protocolPath = "gopls/internal/protocol"

func main() {
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Get or clone gopls source
	srcDir, cleanup, err := getGoplsSource()
	if err != nil {
		return fmt.Errorf("getting gopls source: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Parse the protocol package
	protocolDir := filepath.Join(srcDir, protocolPath)
	if *verbose {
		fmt.Printf("Parsing protocol from: %s\n", protocolDir)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, protocolDir, func(fi os.FileInfo) bool {
		// Only parse tsprotocol.go and related files
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing protocol: %w", err)
	}

	pkg, ok := pkgs["protocol"]
	if !ok {
		return fmt.Errorf("protocol package not found in %s", protocolDir)
	}

	// Extract requested types
	wantTypes := parseTypesList(*typesFlag)
	if *verbose {
		fmt.Printf("Extracting types: %v\n", wantTypes)
	}

	extracted := extractTypes(pkg, wantTypes)

	// Generate output
	output, err := generateOutput(extracted, fset)
	if err != nil {
		return fmt.Errorf("generating output: %w", err)
	}

	if *dryRun {
		fmt.Println(output)
		return nil
	}

	// Write output file
	if err := os.MkdirAll(filepath.Dir(*outputFile), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(*outputFile, []byte(output), 0o644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Printf("Wrote %s\n", *outputFile)
	return nil
}

func getGoplsSource() (string, func(), error) {
	if *goplsDir != "" {
		// Use existing directory
		return *goplsDir, nil, nil
	}

	// Clone to temp directory
	tmpDir, err := os.MkdirTemp("", "gopls-protocol-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	fmt.Printf("Cloning %s...\n", goplsRepo)
	cmd := exec.Command("git", "clone", "--depth=1", "--filter=blob:none", "--sparse", goplsRepo, tmpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("cloning repo: %w", err)
	}

	// Sparse checkout just the protocol directory
	cmd = exec.Command("git", "-C", tmpDir, "sparse-checkout", "set", protocolPath)
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("sparse checkout: %w", err)
	}

	return tmpDir, cleanup, nil
}

func parseTypesList(s string) []string {
	var result []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

type extractedType struct {
	Name    string
	Kind    string // "type", "const", "var"
	Code    string
	Comment string
}

func extractTypes(pkg *ast.Package, wantTypes []string) []extractedType {
	want := make(map[string]bool)
	for _, t := range wantTypes {
		want[t] = true
	}

	var result []extractedType

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if want[s.Name.Name] {
							code := extractTypeSpec(s, d)
							result = append(result, extractedType{
								Name:    s.Name.Name,
								Kind:    "type",
								Code:    code,
								Comment: extractComment(d.Doc),
							})
						}
					case *ast.ValueSpec:
						// Check for const values like InlayHintKind constants
						for _, name := range s.Names {
							if matchesTypePrefix(name.Name, wantTypes) {
								code := extractValueSpec(s, d)
								result = append(result, extractedType{
									Name:    name.Name,
									Kind:    tokenString(d.Tok),
									Code:    code,
									Comment: extractComment(d.Doc),
								})
								break // Only add once per spec
							}
						}
					}
				}
			}
		}
	}

	// Sort by name for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

func matchesTypePrefix(name string, types []string) bool {
	for _, t := range types {
		if strings.HasPrefix(name, t) {
			return true
		}
	}
	return false
}

func extractTypeSpec(spec *ast.TypeSpec, decl *ast.GenDecl) string {
	var buf bytes.Buffer
	if decl.Doc != nil {
		for _, c := range decl.Doc.List {
			buf.WriteString(c.Text)
			buf.WriteString("\n")
		}
	}
	buf.WriteString("type ")
	buf.WriteString(spec.Name.Name)
	buf.WriteString(" ")
	buf.WriteString(exprToString(spec.Type))
	return buf.String()
}

func extractValueSpec(spec *ast.ValueSpec, decl *ast.GenDecl) string {
	var buf bytes.Buffer
	buf.WriteString(tokenString(decl.Tok))
	buf.WriteString(" ")

	for i, name := range spec.Names {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(name.Name)
	}

	if spec.Type != nil {
		buf.WriteString(" ")
		buf.WriteString(exprToString(spec.Type))
	}

	if len(spec.Values) > 0 {
		buf.WriteString(" = ")
		for i, v := range spec.Values {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(exprToString(v))
		}
	}

	return buf.String()
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.StructType:
		return structToString(e)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.BasicLit:
		return e.Value
	case *ast.BinaryExpr:
		return exprToString(e.X) + " " + e.Op.String() + " " + exprToString(e.Y)
	default:
		return fmt.Sprintf("/* unknown: %T */", expr)
	}
}

func structToString(s *ast.StructType) string {
	var buf bytes.Buffer
	buf.WriteString("struct {\n")
	for _, field := range s.Fields.List {
		buf.WriteString("\t")
		for i, name := range field.Names {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(name.Name)
		}
		if len(field.Names) > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(exprToString(field.Type))
		if field.Tag != nil {
			buf.WriteString(" ")
			buf.WriteString(field.Tag.Value)
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}")
	return buf.String()
}

func extractComment(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var lines []string
	for _, c := range doc.List {
		lines = append(lines, c.Text)
	}
	return strings.Join(lines, "\n")
}

func tokenString(tok token.Token) string {
	switch tok {
	case token.CONST:
		return "const"
	case token.VAR:
		return "var"
	case token.TYPE:
		return "type"
	default:
		return tok.String()
	}
}

func generateOutput(types []extractedType, fset *token.FileSet) (string, error) {
	var buf bytes.Buffer

	buf.WriteString(`// Code generated by tools/sync-protocol. DO NOT EDIT.
// Source: github.com/golang/tools/gopls/internal/protocol
//
// This file contains LSP protocol types extracted from gopls.
// gopls generates these from the official LSP metaModel.json specification.
//
// To regenerate:
//   go run ./tools/sync-protocol
//
// Or with a local golang/tools checkout:
//   go run ./tools/sync-protocol -gopls-dir=/path/to/golang/tools

package lsp

`)

	// Group by kind
	var typeDecls, constDecls []extractedType
	for _, t := range types {
		switch t.Kind {
		case "type":
			typeDecls = append(typeDecls, t)
		case "const":
			constDecls = append(constDecls, t)
		}
	}

	// Write type declarations
	if len(typeDecls) > 0 {
		buf.WriteString("// LSP 3.17 Protocol Types\n\n")
		for _, t := range typeDecls {
			if t.Comment != "" {
				buf.WriteString(t.Comment)
				buf.WriteString("\n")
			}
			buf.WriteString(t.Code)
			buf.WriteString("\n\n")
		}
	}

	// Write const declarations
	if len(constDecls) > 0 {
		buf.WriteString("// LSP 3.17 Protocol Constants\n")
		buf.WriteString("const (\n")
		for _, t := range constDecls {
			// Indent const declarations
			lines := strings.Split(t.Code, "\n")
			for _, line := range lines {
				// Remove "const " prefix for grouped const
				line = strings.TrimPrefix(line, "const ")
				buf.WriteString("\t")
				buf.WriteString(line)
				buf.WriteString("\n")
			}
		}
		buf.WriteString(")\n")
	}

	// Format the output
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted if formatting fails
		if *verbose {
			fmt.Fprintf(os.Stderr, "warning: formatting failed: %v\n", err)
		}
		return buf.String(), nil
	}

	// Post-process: replace gopls-specific type references
	result := string(formatted)
	result = postProcess(result)

	return result, nil
}

func postProcess(code string) string {
	// Replace protocol package references with local types or go.lsp.dev/protocol
	replacements := map[string]string{
		"protocol.Position":               "protocol.Position",
		"protocol.Range":                  "protocol.Range",
		"protocol.TextDocumentIdentifier": "protocol.TextDocumentIdentifier",
		"protocol.TextEdit":               "protocol.TextEdit",
		"protocol.Command":                "protocol.Command",
		"protocol.Location":               "protocol.Location",
		"protocol.MarkupContent":          "protocol.MarkupContent",
	}

	for old, new := range replacements {
		code = strings.ReplaceAll(code, old, new)
	}

	// Remove gopls-specific types we don't need
	// These are complex union types that we can simplify
	re := regexp.MustCompile(`\*OrPTooltip_\w+`)
	code = re.ReplaceAllString(code, "*string")

	return code
}
