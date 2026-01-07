// Package buildtools provides adapters for buildtools/warn lint rules.
package buildtools

import (
	"os"

	"github.com/bazelbuild/buildtools/build"
	"github.com/bazelbuild/buildtools/warn"

	"github.com/albertocavalcante/sky/internal/starlark/linter"
)

// AllRules returns all buildtools lint rules wrapped as linter.Rule instances.
func AllRules() []*linter.Rule {
	var rules []*linter.Rule

	// Wrap file-level warnings (single file analysis)
	for name := range warn.FileWarningMap {
		rules = append(rules, wrapFileWarning(name))
	}

	// Wrap multi-file warnings (require cross-file context)
	for name := range warn.MultiFileWarningMap {
		rules = append(rules, wrapMultiFileWarning(name))
	}

	// Wrap rule-level warnings (analyze individual rule calls)
	for name := range warn.RuleWarningMap {
		rules = append(rules, wrapRuleWarning(name))
	}

	return rules
}

// DefaultRules returns the default set of buildtools rules (those enabled by default).
func DefaultRules() []*linter.Rule {
	var rules []*linter.Rule
	defaultSet := make(map[string]bool)
	for _, name := range warn.DefaultWarnings {
		defaultSet[name] = true
	}

	// Wrap file-level warnings
	for name := range warn.FileWarningMap {
		if defaultSet[name] {
			rules = append(rules, wrapFileWarning(name))
		}
	}

	// Wrap multi-file warnings
	for name := range warn.MultiFileWarningMap {
		if defaultSet[name] {
			rules = append(rules, wrapMultiFileWarning(name))
		}
	}

	// Wrap rule-level warnings
	for name := range warn.RuleWarningMap {
		if defaultSet[name] {
			rules = append(rules, wrapRuleWarning(name))
		}
	}

	return rules
}

// wrapFileWarning wraps a single-file buildtools warning as a linter.Rule.
func wrapFileWarning(name string) *linter.Rule {
	fn := warn.FileWarningMap[name]

	return &linter.Rule{
		Name:     name,
		Doc:      getWarningDoc(name),
		URL:      getWarningURL(name),
		Category: categorizeWarning(name),
		Severity: linter.SeverityWarning,
		AutoFix:  false, // Buildtools rules can have fixes, but we'll handle that later
		Run: func(pass *linter.Pass) (any, error) {
			findings := fn(pass.File)
			for _, f := range findings {
				pass.Report(convertFinding(f, name, pass.FilePath))
			}
			return nil, nil
		},
	}
}

// wrapMultiFileWarning wraps a multi-file buildtools warning as a linter.Rule.
func wrapMultiFileWarning(name string) *linter.Rule {
	fn := warn.MultiFileWarningMap[name]

	return &linter.Rule{
		Name:     name,
		Doc:      getWarningDoc(name),
		URL:      getWarningURL(name),
		Category: categorizeWarning(name),
		Severity: linter.SeverityWarning,
		AutoFix:  false,
		Run: func(pass *linter.Pass) (any, error) {
			// Create a FileReader that reads from the filesystem
			fileReader := warn.NewFileReader(func(path string) ([]byte, error) {
				return os.ReadFile(path)
			})

			findings := fn(pass.File, fileReader)
			for _, f := range findings {
				pass.Report(convertFinding(f, name, pass.FilePath))
			}
			return nil, nil
		},
	}
}

// wrapRuleWarning wraps a rule-level buildtools warning as a linter.Rule.
func wrapRuleWarning(name string) *linter.Rule {
	fn := warn.RuleWarningMap[name]

	return &linter.Rule{
		Name:     name,
		Doc:      getWarningDoc(name),
		URL:      getWarningURL(name),
		Category: categorizeWarning(name),
		Severity: linter.SeverityWarning,
		AutoFix:  false,
		Run: func(pass *linter.Pass) (any, error) {
			// Walk through all rule calls in the file
			build.Walk(pass.File, func(expr build.Expr, stack []build.Expr) {
				call, ok := expr.(*build.CallExpr)
				if !ok {
					return
				}

				// Determine package context (empty for now)
				pkg := ""

				if finding := fn(call, pkg); finding != nil {
					pass.Report(convertFinding(finding, name, pass.FilePath))
				}
			})
			return nil, nil
		},
	}
}

// convertFinding converts a buildtools LinterFinding to a linter.Finding.
func convertFinding(f *warn.LinterFinding, ruleName, filePath string) linter.Finding {
	finding := linter.Finding{
		Severity:  linter.SeverityWarning,
		Message:   f.Message,
		Line:      f.Start.Line,
		Column:    f.Start.LineRune,
		EndLine:   f.End.Line,
		EndColumn: f.End.LineRune,
		Rule:      ruleName,
		Category:  categorizeWarning(ruleName),
	}

	// Convert replacement if present
	if len(f.Replacement) > 0 {
		// For now, we only handle single replacements
		// TODO: Handle multiple replacements
		// Buildtools replacements are complex - skip for MVP
		// We would need to format the New Expr and extract byte positions
	}

	return finding
}

// categorizeWarning assigns a category to a buildtools warning based on its name.
func categorizeWarning(name string) string {
	// Categorize based on warning name patterns
	switch {
	case contains(name, "native-cc", "native-java", "native-py", "native-sh", "native-android", "native-proto"):
		return "native-rules"
	case contains(name, "docstring", "doc"):
		return "documentation"
	case contains(name, "load", "same-origin"):
		return "imports"
	case contains(name, "unused", "unreachable", "no-effect"):
		return "unused"
	case contains(name, "ctx-actions", "depset", "provider"):
		return "bazel-api"
	case contains(name, "return", "control-flow"):
		return "control-flow"
	case contains(name, "deprecated", "obsolete"):
		return "deprecation"
	case contains(name, "print", "unsorted", "confusing", "name-conventions"):
		return "style"
	case contains(name, "integer-division", "string-iteration"):
		return "compatibility"
	default:
		return "correctness"
	}
}

// getWarningDoc returns a brief description for a warning.
// This is a simplified version - ideally we'd extract this from buildtools documentation.
func getWarningDoc(name string) string {
	// Map of known warnings to descriptions
	docs := map[string]string{
		"attr-cfg":                  "Checks for invalid cfg attribute",
		"attr-license":              "Checks for deprecated licenses attribute",
		"attr-non-empty":            "Checks for non-empty attribute requirements",
		"attr-output-default":       "Checks for output attributes with defaults",
		"attr-single-file":          "Checks for single_file attribute usage",
		"build-args-kwargs":         "Checks for **kwargs in build rules",
		"bzl-visibility":            "Checks for deprecated bzl_visibility",
		"confusing-name":            "Checks for confusing variable names",
		"constant-glob":             "Checks for constant glob patterns",
		"ctx-actions":               "Checks for deprecated ctx.action",
		"ctx-args":                  "Checks for deprecated ctx.{outputs,files}",
		"deprecated-function":       "Checks for deprecated function usage",
		"depset-iteration":          "Checks for iteration over depset",
		"depset-union":              "Checks for deprecated depset union",
		"dict-concatenation":        "Checks for dict concatenation with +",
		"duplicated-name":           "Checks for duplicate variable names",
		"filetype":                  "Checks for deprecated FileType usage",
		"function-docstring":        "Checks for missing function docstrings",
		"function-docstring-args":   "Checks function docstring documents args",
		"function-docstring-header": "Checks function docstring header format",
		"function-docstring-return": "Checks function docstring documents return",
		"git-repository":            "Checks for deprecated git_repository",
		"http-archive":              "Checks for deprecated http_archive",
		"integer-division":          "Checks for integer division compatibility",
		"keyword-position-args":     "Checks for positional args after keywords",
		"load":                      "Checks for unused load statements",
		"load-on-top":               "Checks that loads are at top of file",
		"module-docstring":          "Checks for missing module docstrings",
		"name-conventions":          "Checks naming conventions",
		"native-android":            "Checks for native android_* rules",
		"native-build":              "Checks for native.existing_rules usage",
		"native-cc":                 "Checks for native cc_* rules",
		"native-java":               "Checks for native java_* rules",
		"native-package":            "Checks for native.package usage",
		"native-proto":              "Checks for native proto_* rules",
		"native-py":                 "Checks for native py_* rules",
		"no-effect":                 "Checks for statements with no effect",
		"output-group":              "Checks for deprecated OutputGroupInfo",
		"overly-nested-depset":      "Checks for deeply nested depsets",
		"package-name":              "Checks for deprecated PACKAGE_NAME",
		"package-on-top":            "Checks that package() is at top",
		"positional-args":           "Checks for too many positional args",
		"print":                     "Checks for print statements",
		"provider-params":           "Checks provider parameter usage",
		"redefined-variable":        "Checks for variable redefinition",
		"repository-name":           "Checks for deprecated REPOSITORY_NAME",
		"return-value":              "Checks for return value issues",
		"rule-impl-return":          "Checks rule implementation return value",
		"same-origin-load":          "Checks for same-origin load statements",
		"string-iteration":          "Checks for string iteration compatibility",
		"uninitialized":             "Checks for uninitialized variables",
		"unnamed-macro":             "Checks for unnamed macro definitions",
		"unreachable":               "Checks for unreachable code",
		"unsorted-dict-items":       "Checks for unsorted dictionary items",
		"unused-variable":           "Checks for unused variables",
	}

	if doc, ok := docs[name]; ok {
		return doc
	}

	// Generate a generic description
	return "Buildtools warning: " + name
}

// getWarningURL returns the documentation URL for a warning.
func getWarningURL(name string) string {
	// All buildtools warnings are documented in the same place
	return "https://github.com/bazelbuild/buildtools/blob/master/WARNINGS.md#" + name
}

// contains checks if str contains any of the substrings.
func contains(str string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(str) >= len(sub) {
			for i := 0; i <= len(str)-len(sub); i++ {
				if str[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
