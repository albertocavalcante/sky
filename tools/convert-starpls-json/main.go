// convert-starpls-json converts starpls JSON format to our builtins JSON format.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
)

// StarplsBuiltins represents the starpls JSON schema.
type StarplsBuiltins struct {
	Builtins []StarplsBuiltin `json:"builtins"`
}

// StarplsBuiltin represents a single builtin in starpls format.
type StarplsBuiltin struct {
	Name     string           `json:"name"`
	Doc      string           `json:"doc,omitempty"`
	Type     string           `json:"type,omitempty"`
	Callable *StarplsCallable `json:"callable,omitempty"`
	Fields   []StarplsField   `json:"fields,omitempty"`
}

// StarplsCallable represents a callable in starpls format.
type StarplsCallable struct {
	Params     []StarplsParam `json:"params,omitempty"`
	ReturnType string         `json:"return_type,omitempty"`
	Doc        string         `json:"doc,omitempty"`
}

// StarplsParam represents a parameter in starpls format.
type StarplsParam struct {
	Name          string `json:"name"`
	Type          string `json:"type,omitempty"`
	Doc           string `json:"doc,omitempty"`
	DefaultValue  string `json:"default_value,omitempty"`
	IsMandatory   bool   `json:"is_mandatory"`
	IsStarArg     bool   `json:"is_star_arg"`
	IsStarStarArg bool   `json:"is_star_star_arg"`
}

// StarplsField represents a field in starpls format.
type StarplsField struct {
	Name     string           `json:"name"`
	Type     string           `json:"type,omitempty"`
	Doc      string           `json:"doc,omitempty"`
	Callable *StarplsCallable `json:"callable,omitempty"`
}

func main() {
	inputDir := flag.String("input", "", "Input directory containing starpls JSON files")
	outputDir := flag.String("output", "", "Output directory for converted JSON files")
	flag.Parse()

	if *inputDir == "" || *outputDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -input=<dir> -output=<dir>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Map of starpls filenames to our output filenames
	conversions := map[string]string{
		"build.builtins.json":        "bazel-build.json",
		"bzl.builtins.json":          "bazel-bzl.json",
		"workspace.builtins.json":    "bazel-workspace.json",
		"module-bazel.builtins.json": "bazel-module.json",
	}

	for starplsFile, outputFile := range conversions {
		inputPath := filepath.Join(*inputDir, starplsFile)
		outputPath := filepath.Join(*outputDir, outputFile)

		if err := convertFile(inputPath, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to convert %s: %v\n", starplsFile, err)
			continue
		}

		fmt.Printf("Converted %s -> %s\n", starplsFile, outputFile)
	}
}

func convertFile(inputPath, outputPath string) error {
	// Read starpls JSON
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var starplsBuiltins StarplsBuiltins
	if err := json.Unmarshal(data, &starplsBuiltins); err != nil {
		return fmt.Errorf("failed to parse starpls JSON: %w", err)
	}

	// Convert to our format
	ourBuiltins := convertBuiltins(starplsBuiltins)

	// Write our JSON
	output, err := json.MarshalIndent(ourBuiltins, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

func convertBuiltins(starpls StarplsBuiltins) builtins.Builtins {
	result := builtins.Builtins{
		Functions: []builtins.Signature{},
		Types:     []builtins.TypeDef{},
		Globals:   []builtins.Field{},
	}

	for _, sb := range starpls.Builtins {
		// Determine if this is a type, function, or global
		if len(sb.Fields) > 0 {
			// It's a type definition
			result.Types = append(result.Types, convertType(sb))
		} else if sb.Callable != nil {
			// It's a function
			result.Functions = append(result.Functions, convertFunction(sb))
		} else {
			// It's a global constant/variable
			result.Globals = append(result.Globals, builtins.Field{
				Name: sb.Name,
				Type: sb.Type,
				Doc:  sb.Doc,
			})
		}
	}

	return result
}

func convertType(sb StarplsBuiltin) builtins.TypeDef {
	typeDef := builtins.TypeDef{
		Name:    sb.Name,
		Doc:     sb.Doc,
		Fields:  []builtins.Field{},
		Methods: []builtins.Signature{},
	}

	for _, field := range sb.Fields {
		if field.Callable != nil {
			// It's a method
			method := builtins.Signature{
				Name:       field.Name,
				Doc:        combineDoc(field.Doc, field.Callable.Doc),
				Params:     convertParams(field.Callable.Params),
				ReturnType: field.Callable.ReturnType,
			}
			typeDef.Methods = append(typeDef.Methods, method)
		} else {
			// It's a field
			typeDef.Fields = append(typeDef.Fields, builtins.Field{
				Name: field.Name,
				Type: field.Type,
				Doc:  field.Doc,
			})
		}
	}

	return typeDef
}

func convertFunction(sb StarplsBuiltin) builtins.Signature {
	doc := sb.Doc
	if sb.Callable != nil && sb.Callable.Doc != "" {
		doc = combineDoc(doc, sb.Callable.Doc)
	}

	var params []builtins.Param
	var returnType string

	if sb.Callable != nil {
		params = convertParams(sb.Callable.Params)
		returnType = sb.Callable.ReturnType
	}

	return builtins.Signature{
		Name:       sb.Name,
		Doc:        doc,
		Params:     params,
		ReturnType: returnType,
	}
}

func convertParams(starplsParams []StarplsParam) []builtins.Param {
	params := make([]builtins.Param, len(starplsParams))
	for i, sp := range starplsParams {
		params[i] = builtins.Param{
			Name:     sp.Name,
			Type:     sp.Type,
			Default:  sp.DefaultValue,
			Required: sp.IsMandatory,
			Variadic: sp.IsStarArg,
			KWArgs:   sp.IsStarStarArg,
		}
	}
	return params
}

func combineDoc(doc1, doc2 string) string {
	doc1 = strings.TrimSpace(doc1)
	doc2 = strings.TrimSpace(doc2)

	if doc1 == "" {
		return doc2
	}
	if doc2 == "" {
		return doc1
	}
	if doc1 == doc2 {
		return doc1
	}

	return doc1 + "\n\n" + doc2
}
