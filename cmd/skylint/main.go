package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/albertocavalcante/sky/internal/starlark/linter"
	"github.com/albertocavalcante/sky/internal/starlark/linter/buildtools"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK      = 0
	exitError   = 1
	exitWarning = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var (
		enableFlag         string
		disableFlag        string
		formatFlag         string
		configFlag         string
		warningsAsErrors   bool
		listRulesFlag      bool
		listCategoriesFlag bool
		explainFlag        string
		versionFlag        bool
		fixFlag            bool
		diffFlag           bool
	)

	fs := flag.NewFlagSet("skylint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&enableFlag, "enable", "", "enable rules (comma-separated, supports 'all' and categories)")
	fs.StringVar(&disableFlag, "disable", "", "disable rules (comma-separated, supports patterns like 'native-*')")
	fs.StringVar(&formatFlag, "format", "text", "output format: text, compact, json, github")
	fs.StringVar(&configFlag, "config", "", "config file path (default: search for .skylint.json)")
	fs.BoolVar(&warningsAsErrors, "warnings-as-errors", false, "treat warnings as errors")
	fs.BoolVar(&listRulesFlag, "list-rules", false, "list all available rules")
	fs.BoolVar(&listCategoriesFlag, "list-categories", false, "list all rule categories")
	fs.StringVar(&explainFlag, "explain", "", "show detailed explanation for a rule")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")
	fs.BoolVar(&fixFlag, "fix", false, "automatically fix issues where possible")
	fs.BoolVar(&diffFlag, "diff", false, "show diff of fixes without applying (use with --fix)")

	fs.Usage = func() {
		writeln(stderr, "Usage: skylint [flags] path ...")
		writeln(stderr)
		writeln(stderr, "Lints Starlark files.")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skylint BUILD.bazel              # Lint a single file")
		writeln(stderr, "  skylint ./...                    # Lint all files recursively")
		writeln(stderr, "  skylint --enable=all .           # Enable all rules")
		writeln(stderr, "  skylint --disable=native-* .     # Disable native-* rules")
		writeln(stderr, "  skylint --fix .                  # Fix issues automatically")
		writeln(stderr, "  skylint --fix --diff .           # Preview fixes as diff")
		writeln(stderr, "  skylint --list-rules             # List all available rules")
		writeln(stderr, "  skylint --explain=load           # Explain the 'load' rule")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skylint %s\n", version.String())
		return exitOK
	}

	// Create registry and register all buildtools rules
	registry := linter.NewRegistry()
	if err := registry.Register(buildtools.AllRules()...); err != nil {
		writef(stderr, "skylint: failed to register rules: %v\n", err)
		return exitError
	}

	// Handle --list-rules
	if listRulesFlag {
		return listRules(stdout, registry)
	}

	// Handle --list-categories
	if listCategoriesFlag {
		return listCategories(stdout, registry)
	}

	// Handle --explain
	if explainFlag != "" {
		return explainRule(stdout, stderr, registry, explainFlag)
	}

	// Load configuration file
	config, err := linter.LoadConfig(configFlag)
	if err != nil {
		writef(stderr, "skylint: failed to load config: %v\n", err)
		return exitError
	}

	// Apply config to registry
	if err := config.ApplyToRegistry(registry); err != nil {
		writef(stderr, "skylint: failed to apply config: %v\n", err)
		return exitError
	}

	// Apply enable/disable flags (these override config file)
	if enableFlag != "" {
		rules := parseCommaSeparated(enableFlag)
		registry.Enable(rules...)
	}

	if disableFlag != "" {
		rules := parseCommaSeparated(disableFlag)
		registry.Disable(rules...)
	}

	// Apply warnings-as-errors flag (overrides config file)
	if warningsAsErrors || config.WarningsAsErrors {
		warningsAsErrors = true
	}

	// Validate registry
	if err := registry.Validate(); err != nil {
		writef(stderr, "skylint: configuration error: %v\n", err)
		return exitError
	}

	// Get paths to lint
	paths := fs.Args()
	if len(paths) == 0 {
		writeln(stderr, "skylint: no files specified")
		fs.Usage()
		return exitError
	}

	// Create driver and run linter
	driver := linter.NewDriver(registry)
	result, err := driver.Run(context.Background(), paths)
	if err != nil {
		writef(stderr, "skylint: %v\n", err)
		return exitError
	}

	// Handle --fix mode
	if fixFlag {
		fixableCount := linter.FixableCount(result.Findings)
		if fixableCount == 0 {
			writeln(stderr, "skylint: no fixable issues found")
		} else {
			fixResults, err := linter.FixFiles(result.Findings)
			if err != nil {
				writef(stderr, "skylint: failed to compute fixes: %v\n", err)
				return exitError
			}

			// Count total fixes
			totalApplied := 0
			totalSkipped := 0
			filesChanged := 0
			for _, fr := range fixResults {
				totalApplied += fr.AppliedFixes
				totalSkipped += fr.SkippedFixes
				if fr.HasChanges() {
					filesChanged++
				}
			}

			if diffFlag {
				// Show diff without applying
				for _, fr := range fixResults {
					if fr.HasChanges() {
						writef(stdout, "%s", fr.Diff())
					}
				}
				writef(stderr, "\nWould fix %d issue(s) in %d file(s)", totalApplied, filesChanged)
				if totalSkipped > 0 {
					writef(stderr, " (%d skipped due to conflicts)", totalSkipped)
				}
				writeln(stderr)
			} else {
				// Apply fixes
				if err := linter.WriteFixResults(fixResults); err != nil {
					writef(stderr, "skylint: failed to write fixes: %v\n", err)
					return exitError
				}
				writef(stderr, "Fixed %d issue(s) in %d file(s)", totalApplied, filesChanged)
				if totalSkipped > 0 {
					writef(stderr, " (%d skipped due to conflicts)", totalSkipped)
				}
				writeln(stderr)
			}
		}
		return exitOK
	}

	// Create reporter based on format
	var reporter linter.Reporter
	switch formatFlag {
	case "text":
		reporter = linter.NewTextReporter()
	case "compact":
		reporter = linter.NewCompactReporter()
	case "json":
		reporter = linter.NewJSONReporter()
	case "github":
		reporter = linter.NewGitHubReporter()
	default:
		writef(stderr, "skylint: unknown format: %s\n", formatFlag)
		return exitError
	}

	// Report results
	if err := reporter.Report(stdout, result); err != nil {
		writef(stderr, "skylint: failed to report results: %v\n", err)
		return exitError
	}

	// Determine exit code
	if result.HasErrors() || len(result.Errors) > 0 {
		return exitError
	}
	// If warnings-as-errors is enabled, treat warnings as errors
	if warningsAsErrors && result.HasWarnings() {
		return exitError
	}
	if result.HasWarnings() {
		return exitWarning
	}

	return exitOK
}

// listRules outputs all available rules.
func listRules(w io.Writer, registry *linter.Registry) int {
	rules := registry.AllRules()
	if len(rules) == 0 {
		writeln(w, "No rules registered")
		return exitOK
	}

	writef(w, "Available rules (%d total):\n\n", len(rules))

	// Group by category
	categories := registry.Categories()
	for _, cat := range categories {
		catRules := registry.RulesByCategory(cat)
		if len(catRules) == 0 {
			continue
		}

		writef(w, "%s (%d rules):\n", cat, len(catRules))
		for _, rule := range catRules {
			writef(w, "  %-30s  %s\n", rule.Name, rule.Doc)
		}
		writeln(w)
	}

	return exitOK
}

// listCategories outputs all rule categories.
func listCategories(w io.Writer, registry *linter.Registry) int {
	categories := registry.Categories()
	if len(categories) == 0 {
		writeln(w, "No categories found")
		return exitOK
	}

	writef(w, "Available categories (%d total):\n\n", len(categories))
	for _, cat := range categories {
		rules := registry.RulesByCategory(cat)
		writef(w, "  %-20s  %d rules\n", cat, len(rules))
	}

	return exitOK
}

// explainRule shows detailed information about a specific rule.
func explainRule(stdout, stderr io.Writer, registry *linter.Registry, ruleName string) int {
	found, ok := registry.Rule(ruleName)
	if !ok {
		writef(stderr, "skylint: unknown rule: %s\n", ruleName)
		writeln(stderr, "\nUse --list-rules to see all available rules")
		return exitError
	}

	writef(stdout, "Rule: %s\n", found.Name)
	writef(stdout, "Category: %s\n", found.Category)
	writef(stdout, "Severity: %s\n", severityToString(found.Severity))
	writef(stdout, "Auto-fix: %v\n", found.AutoFix)
	writeln(stdout)
	writef(stdout, "Description:\n  %s\n", found.Doc)
	if found.URL != "" {
		writeln(stdout)
		writef(stdout, "Documentation:\n  %s\n", found.URL)
	}

	return exitOK
}

// parseCommaSeparated parses a comma-separated string into a slice.
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// severityToString converts a severity to a string.
func severityToString(s linter.Severity) string {
	switch s {
	case linter.SeverityError:
		return "error"
	case linter.SeverityWarning:
		return "warning"
	case linter.SeverityInfo:
		return "info"
	case linter.SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Helper functions for writing output.
// Write errors are intentionally ignored because:
//  1. These functions write to stdout/stderr where there's no reasonable recovery
//     if the terminal/pipe is broken (EPIPE, etc.)
//  2. If we can't write error messages, we can't report the write failure either
//  3. The exit code still reflects the actual operation status
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
