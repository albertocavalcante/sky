// Package main implements a Starlark file analyzer plugin.
//
// This example demonstrates a real-world plugin that uses the buildtools
// library to parse and analyze Starlark files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/albertocavalcante/sky/examples/plugins/star-counter/counter"
)

const (
	pluginName    = "star-counter"
	pluginVersion = "1.0.0"
	pluginSummary = "Analyzes Starlark files and counts definitions"
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

	// Get files to analyze
	paths := fs.Args()
	if len(paths) == 0 {
		// Default to current directory
		paths = []string{"."}
	}

	var allStats []counter.FileStats
	var errors []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		if info.IsDir() {
			if *recursive {
				stats, errs := analyzeDirectory(path)
				allStats = append(allStats, stats...)
				errors = append(errors, errs...)
			} else {
				// Non-recursive: just list .star, .bzl, BUILD files in this dir
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
						stats, err := counter.Analyze(filePath)
						if err != nil {
							errors = append(errors, fmt.Sprintf("%s: %v", filePath, err))
						} else {
							allStats = append(allStats, stats)
						}
					}
				}
			}
		} else {
			stats, err := counter.Analyze(path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			} else {
				allStats = append(allStats, stats)
			}
		}
	}

	// Output results
	if *jsonOutput || os.Getenv("SKY_OUTPUT_FORMAT") == "json" {
		result := map[string]any{
			"files":  allStats,
			"errors": errors,
			"totals": computeTotals(allStats),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			return 1
		}
	} else {
		printTextOutput(allStats, errors)
	}

	if len(errors) > 0 {
		return 1
	}
	return 0
}

func analyzeDirectory(root string) ([]counter.FileStats, []string) {
	var stats []counter.FileStats
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
			s, err := counter.Analyze(path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			} else {
				stats = append(stats, s)
			}
		}
		return nil
	})
	if err != nil {
		errors = append(errors, fmt.Sprintf("%s: %v", root, err))
	}

	return stats, errors
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

func computeTotals(stats []counter.FileStats) counter.FileStats {
	totals := counter.FileStats{Path: "TOTAL"}
	for _, s := range stats {
		totals.Defs += s.Defs
		totals.Loads += s.Loads
		totals.Calls += s.Calls
		totals.Assigns += s.Assigns
		totals.Lines += s.Lines
	}
	return totals
}

func printTextOutput(stats []counter.FileStats, errors []string) {
	if len(stats) == 0 && len(errors) == 0 {
		fmt.Println("No Starlark files found")
		return
	}

	// Print per-file stats
	fmt.Printf("%-40s %6s %6s %6s %6s %6s\n", "FILE", "DEFS", "LOADS", "CALLS", "ASSIGN", "LINES")
	fmt.Println(repeatString("-", 78))

	for _, s := range stats {
		fmt.Printf("%-40s %6d %6d %6d %6d %6d\n",
			truncatePath(s.Path, 40), s.Defs, s.Loads, s.Calls, s.Assigns, s.Lines)
	}

	// Print totals
	if len(stats) > 1 {
		fmt.Println(repeatString("-", 78))
		totals := computeTotals(stats)
		fmt.Printf("%-40s %6d %6d %6d %6d %6d\n",
			fmt.Sprintf("TOTAL (%d files)", len(stats)),
			totals.Defs, totals.Loads, totals.Calls, totals.Assigns, totals.Lines)
	}

	// Print errors
	if len(errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range errors {
			fmt.Printf("  %s\n", e)
		}
	}
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
