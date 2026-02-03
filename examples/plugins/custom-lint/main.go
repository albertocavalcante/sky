// Package main implements a custom lint rule plugin for Starlark.
//
// This example demonstrates how to create custom lint rules that extend
// Sky's built-in linting capabilities.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/albertocavalcante/sky/examples/plugins/custom-lint/rules"
)

const (
	pluginName    = "custom-lint"
	pluginVersion = "1.0.0"
	pluginSummary = "Custom lint rules for Starlark files"
)

func main() {
	// Verify we're running as a Sky plugin
	if os.Getenv("SKY_PLUGIN") != "1" {
		fmt.Fprintf(os.Stderr, "This is a Sky plugin. Run it with: sky %s\n", pluginName)
		os.Exit(1)
	}

	// Handle metadata request
	if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
		outputMetadata()
		return
	}

	// Run the plugin
	os.Exit(run(os.Args[1:]))
}

func outputMetadata() {
	metadata := map[string]any{
		"api_version": 1,
		"name":        pluginName,
		"version":     pluginVersion,
		"summary":     pluginSummary,
		"commands": []map[string]string{
			{
				"name":    pluginName,
				"summary": pluginSummary,
			},
		},
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(metadata); err != nil {
		os.Exit(1)
	}
}

func run(args []string) int {
	fs := flag.NewFlagSet(pluginName, flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	recursive := fs.Bool("r", false, "recursively scan directories")
	showVersion := fs.Bool("version", false, "show version")
	listRules := fs.Bool("list", false, "list available rules")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Printf("%s %s\n", pluginName, pluginVersion)
		return 0
	}

	if *listRules {
		printRules()
		return 0
	}

	// Get files to analyze
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var allFindings []rules.Finding
	var errors []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		if info.IsDir() {
			if *recursive {
				findings, errs := lintDirectory(path)
				allFindings = append(allFindings, findings...)
				errors = append(errors, errs...)
			} else {
				// Non-recursive: just lint files in this dir
				entries, err := os.ReadDir(path)
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", path, err))
					continue
				}
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					if isStarlarkFile(entry.Name()) {
						filePath := filepath.Join(path, entry.Name())
						findings, err := rules.LintFile(filePath)
						if err != nil {
							errors = append(errors, fmt.Sprintf("%s: %v", filePath, err))
						} else {
							allFindings = append(allFindings, findings...)
						}
					}
				}
			}
		} else {
			findings, err := rules.LintFile(path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			} else {
				allFindings = append(allFindings, findings...)
			}
		}
	}

	// Output results
	if *jsonOutput || os.Getenv("SKY_OUTPUT_FORMAT") == "json" {
		result := map[string]any{
			"findings": allFindings,
			"errors":   errors,
			"summary": map[string]int{
				"files":    countUniqueFiles(allFindings),
				"findings": len(allFindings),
				"errors":   len(errors),
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		printTextOutput(allFindings, errors)
	}

	if len(allFindings) > 0 || len(errors) > 0 {
		return 1
	}
	return 0
}

func lintDirectory(root string) ([]rules.Finding, []string) {
	var findings []rules.Finding
	var errors []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if isStarlarkFile(info.Name()) {
			f, err := rules.LintFile(path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			} else {
				findings = append(findings, f...)
			}
		}
		return nil
	})
	if err != nil {
		errors = append(errors, fmt.Sprintf("%s: %v", root, err))
	}

	return findings, errors
}

func isStarlarkFile(name string) bool {
	switch {
	case filepath.Ext(name) == ".star":
		return true
	case filepath.Ext(name) == ".bzl":
		return true
	case name == "BUILD" || name == "BUILD.bazel":
		return true
	case name == "WORKSPACE" || name == "WORKSPACE.bazel":
		return true
	default:
		return false
	}
}

func countUniqueFiles(findings []rules.Finding) int {
	files := make(map[string]bool)
	for _, f := range findings {
		files[f.File] = true
	}
	return len(files)
}

func printRules() {
	fmt.Println("Available lint rules:")
	fmt.Println()
	for _, rule := range rules.AllRules {
		fmt.Printf("  %-20s %s\n", rule.Name, rule.Description)
	}
}

func printTextOutput(findings []rules.Finding, errors []string) {
	if len(findings) == 0 && len(errors) == 0 {
		fmt.Println("No issues found")
		return
	}

	// Print findings
	for _, f := range findings {
		fmt.Printf("%s:%d:%d: %s: %s\n", f.File, f.Line, f.Column, f.Rule, f.Message)
	}

	// Print errors
	if len(errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range errors {
			fmt.Printf("  %s\n", e)
		}
	}

	// Print summary
	fmt.Println()
	fmt.Printf("Found %d issue(s) in %d file(s)\n", len(findings), countUniqueFiles(findings))
}
