package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	builtinspb "github.com/albertocavalcante/sky/internal/starlark/builtins/proto"
	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"google.golang.org/protobuf/proto"
)

var (
	inputPath = flag.String("input", "", "Path to input build-language.pb file")
	outputDir = flag.String("output", "", "Output directory for generated .pb files")
)

func main() {
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *outputDir == "" {
		fmt.Fprintln(os.Stderr, "Error: -output flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Read the input proto file
	data, err := os.ReadFile(*inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Parse the BuildLanguage proto
	var buildLang buildpb.BuildLanguage
	if err := proto.Unmarshal(data, &buildLang); err != nil {
		return fmt.Errorf("failed to unmarshal BuildLanguage proto: %w", err)
	}

	fmt.Printf("Loaded %d rules from %s\n", len(buildLang.GetRule()), *inputPath)

	// Group rules by context
	contexts := groupRulesByContext(buildLang.GetRule())

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output files for each context
	for context, rules := range contexts {
		if err := generateContextFile(context, rules); err != nil {
			return fmt.Errorf("failed to generate %s file: %w", context, err)
		}
		fmt.Printf("Generated %s with %d rules\n", contextToFilename(context), len(rules))
	}

	return nil
}

// groupRulesByContext categorizes rules based on naming patterns and documentation
func groupRulesByContext(rules []*buildpb.RuleDefinition) map[string][]*buildpb.RuleDefinition {
	contexts := map[string][]*buildpb.RuleDefinition{
		"BUILD":     make([]*buildpb.RuleDefinition, 0),
		"BZL":       make([]*buildpb.RuleDefinition, 0),
		"WORKSPACE": make([]*buildpb.RuleDefinition, 0),
		"MODULE":    make([]*buildpb.RuleDefinition, 0),
	}

	for _, rule := range rules {
		name := rule.GetName()
		label := rule.GetLabel()
		doc := strings.ToLower(rule.GetDocumentation())

		// Determine context based on naming patterns and documentation
		context := inferContext(name, label, doc)
		contexts[context] = append(contexts[context], rule)
	}

	return contexts
}

// inferContext determines which context a rule belongs to
func inferContext(name, label, doc string) string {
	// MODULE.bazel specific functions
	if strings.HasPrefix(name, "module.") ||
		strings.HasPrefix(name, "bazel_dep") ||
		strings.HasPrefix(name, "archive_override") ||
		strings.HasPrefix(name, "git_override") ||
		strings.HasPrefix(name, "local_path_override") ||
		strings.HasPrefix(name, "single_version_override") ||
		strings.HasPrefix(name, "multiple_version_override") ||
		strings.HasPrefix(name, "use_extension") ||
		strings.HasPrefix(name, "use_repo") ||
		strings.Contains(doc, "module.bazel") ||
		strings.Contains(doc, "bzlmod") {
		return "MODULE"
	}

	// WORKSPACE specific functions
	if strings.HasPrefix(name, "workspace") ||
		strings.HasPrefix(name, "bind") ||
		strings.HasPrefix(name, "register_") ||
		strings.Contains(name, "_repository") ||
		strings.Contains(name, "_workspace") ||
		strings.HasSuffix(name, "_repository") ||
		strings.Contains(doc, "workspace") && !strings.Contains(doc, "bzl file") {
		return "WORKSPACE"
	}

	// .bzl file specific (Starlark API)
	if strings.HasPrefix(name, "provider") ||
		strings.HasPrefix(name, "rule") ||
		strings.HasPrefix(name, "aspect") ||
		strings.HasPrefix(name, "repository_rule") ||
		strings.HasPrefix(name, "tag_class") ||
		strings.HasPrefix(name, "module_extension") ||
		strings.Contains(name, "Label") ||
		strings.Contains(name, "analysis") ||
		strings.Contains(doc, ".bzl file") ||
		strings.Contains(doc, "starlark") {
		return "BZL"
	}

	// Default to BUILD if no specific markers found
	return "BUILD"
}

func contextToFilename(context string) string {
	return fmt.Sprintf("bazel_%s.pb", strings.ToLower(context))
}

func generateContextFile(context string, rules []*buildpb.RuleDefinition) error {
	// Convert to our Builtins schema
	builtins := convertToBuiltins(rules)

	// Marshal to proto binary
	data, err := proto.Marshal(builtins)
	if err != nil {
		return fmt.Errorf("failed to marshal builtins proto: %w", err)
	}

	// Write to output file
	filename := contextToFilename(context)
	outputPath := filepath.Join(*outputDir, filename)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// convertToBuiltins converts bazel-buildtools RuleDefinitions to our Builtins schema
func convertToBuiltins(rules []*buildpb.RuleDefinition) *builtinspb.Builtins {
	builtins := &builtinspb.Builtins{
		Types:  make([]*builtinspb.Type, 0),
		Values: make([]*builtinspb.Value, 0),
	}

	for _, rule := range rules {
		name := rule.GetName()
		doc := rule.GetDocumentation()

		// Check if this is a callable (function) or a type
		// Most Bazel rules are callable functions
		callable := convertAttributesToCallable(rule.GetAttribute())

		value := &builtinspb.Value{
			Name:     name,
			Doc:      doc,
			Callable: callable,
		}

		builtins.Values = append(builtins.Values, value)
	}

	return builtins
}

// convertAttributesToCallable converts AttributeDefinitions to our Callable schema
func convertAttributesToCallable(attrs []*buildpb.AttributeDefinition) *builtinspb.Callable {
	if len(attrs) == 0 {
		return nil
	}

	callable := &builtinspb.Callable{
		Params: make([]*builtinspb.Param, 0, len(attrs)),
	}

	for _, attr := range attrs {
		param := convertAttributeToParam(attr)
		callable.Params = append(callable.Params, param)
	}

	return callable
}

// convertAttributeToParam converts an AttributeDefinition to a Param
func convertAttributeToParam(attr *buildpb.AttributeDefinition) *builtinspb.Param {
	param := &builtinspb.Param{
		Name:        attr.GetName(),
		Type:        attributeTypeToString(attr.GetType()),
		Doc:         attr.GetDocumentation(),
		IsMandatory: attr.GetMandatory(),
	}

	// Try to extract default value if present
	if defaultVal := attr.GetDefault(); defaultVal != nil {
		param.DefaultValue = defaultValueToString(defaultVal)
	}

	return param
}

// attributeTypeToString converts Bazel attribute type to string representation
func attributeTypeToString(t buildpb.Attribute_Discriminator) string {
	switch t {
	case buildpb.Attribute_INTEGER:
		return "int"
	case buildpb.Attribute_STRING:
		return "str"
	case buildpb.Attribute_LABEL:
		return "Label"
	case buildpb.Attribute_OUTPUT:
		return "str"
	case buildpb.Attribute_STRING_LIST:
		return "list[str]"
	case buildpb.Attribute_LABEL_LIST:
		return "list[Label]"
	case buildpb.Attribute_OUTPUT_LIST:
		return "list[str]"
	case buildpb.Attribute_DISTRIBUTION_SET:
		return "list[str]"
	case buildpb.Attribute_LICENSE:
		return "License"
	case buildpb.Attribute_STRING_DICT:
		return "dict[str, str]"
	case buildpb.Attribute_FILESET_ENTRY_LIST:
		return "list[FilesetEntry]"
	case buildpb.Attribute_LABEL_LIST_DICT:
		return "dict[str, list[Label]]"
	case buildpb.Attribute_STRING_LIST_DICT:
		return "dict[str, list[str]]"
	case buildpb.Attribute_BOOLEAN:
		return "bool"
	case buildpb.Attribute_TRISTATE:
		return "int"
	case buildpb.Attribute_INTEGER_LIST:
		return "list[int]"
	case buildpb.Attribute_LABEL_DICT_UNARY:
		return "dict[str, Label]"
	case buildpb.Attribute_SELECTOR_LIST:
		return "select"
	case buildpb.Attribute_LABEL_KEYED_STRING_DICT:
		return "dict[Label, str]"
	default:
		return "unknown"
	}
}

// defaultValueToString converts an AttributeValue to a string representation
func defaultValueToString(val *buildpb.AttributeValue) string {
	if val.Int != nil {
		return fmt.Sprintf("%d", val.GetInt())
	}
	if val.String_ != nil {
		return fmt.Sprintf("%q", val.GetString_())
	}
	if val.Bool != nil {
		return fmt.Sprintf("%t", val.GetBool())
	}
	if len(val.List) > 0 {
		var parts []string
		for _, item := range val.List {
			parts = append(parts, defaultValueToString(item))
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	}
	if len(val.Dict) > 0 {
		var parts []string
		for _, kv := range val.Dict {
			parts = append(parts, fmt.Sprintf("%q: %s", kv.GetKey(), defaultValueToString(kv.GetValue())))
		}
		return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
	}
	return ""
}
