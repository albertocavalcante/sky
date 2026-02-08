// Command vendor-grammar fetches and vendors the Starlark TextMate grammar
// from the official vscode-bazel extension, preserving license headers.
//
// Usage:
//
//	go run ./tools/vendor-grammar [options]
//
// Options:
//
//	-output    Output directory for vendored files (default: editors/code/syntaxes)
//	-source    Source to fetch from: "github" or local path (default: github)
//	-ref       Git ref to fetch (branch, tag, commit) (default: master)
//	-dry-run   Print what would be done without making changes
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// GitHub raw content URLs for vscode-bazel
	baseURL    = "https://raw.githubusercontent.com/bazelbuild/vscode-bazel"
	repoURL    = "https://github.com/bazelbuild/vscode-bazel"
	defaultRef = "master"
)

// Files to vendor from vscode-bazel/syntaxes/
// Note: We only vendor Starlark files. Bazelrc grammar is available upstream
// but not needed for Sky's Starlark-focused extension.
var vendorFiles = []string{
	"starlark.tmLanguage.json",
	"starlark.tmLanguage.license",
	"starlark.configuration.json",
}

type options struct {
	output string
	source string
	ref    string
	dryRun bool
}

func main() {
	opts := parseFlags()

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.output, "output", "editors/code/syntaxes", "Output directory for vendored files")
	flag.StringVar(&opts.source, "source", "github", "Source: 'github' or local path to vscode-bazel clone")
	flag.StringVar(&opts.ref, "ref", defaultRef, "Git ref to fetch (branch, tag, commit)")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "Print what would be done without making changes")
	flag.Parse()
	return opts
}

func run(opts options) error {
	fmt.Printf("Vendoring Starlark grammar from vscode-bazel\n")
	fmt.Printf("  Source: %s (ref: %s)\n", opts.source, opts.ref)
	fmt.Printf("  Output: %s\n", opts.output)
	fmt.Println()

	// Create output directory
	if !opts.dryRun {
		if err := os.MkdirAll(opts.output, 0755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
	}

	// Fetch and write each file
	for _, filename := range vendorFiles {
		fmt.Printf("  Fetching %s...\n", filename)

		content, err := fetchFile(opts.source, opts.ref, filename)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", filename, err)
		}

		outPath := filepath.Join(opts.output, filename)

		if opts.dryRun {
			fmt.Printf("    Would write %d bytes to %s\n", len(content), outPath)
			continue
		}

		if err := os.WriteFile(outPath, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Printf("    Wrote %d bytes to %s\n", len(content), outPath)
	}

	// Write VENDOR.md with attribution and instructions
	vendorMD := generateVendorDoc(opts.ref)
	vendorPath := filepath.Join(opts.output, "VENDOR.md")

	if opts.dryRun {
		fmt.Printf("  Would write VENDOR.md to %s\n", vendorPath)
	} else {
		if err := os.WriteFile(vendorPath, []byte(vendorMD), 0644); err != nil {
			return fmt.Errorf("write VENDOR.md: %w", err)
		}
		fmt.Printf("  Wrote VENDOR.md to %s\n", vendorPath)
	}

	fmt.Println()
	fmt.Println("Done! Remember to:")
	fmt.Println("  1. Review changes for any upstream modifications")
	fmt.Println("  2. Keep starlark.tmLanguage.license alongside the grammar")
	fmt.Println("  3. Update VENDOR.md if you make local modifications")

	return nil
}

func fetchFile(source, ref, filename string) ([]byte, error) {
	if source == "github" {
		return fetchFromGitHub(ref, filename)
	}
	// Local path
	path := filepath.Join(source, "syntaxes", filename)
	return os.ReadFile(path)
}

func fetchFromGitHub(ref, filename string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/syntaxes/%s", baseURL, ref, filename)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func generateVendorDoc(ref string) string {
	return fmt.Sprintf(`# Vendored Starlark Grammar

This directory contains TextMate grammar files vendored from the
[vscode-bazel](https://github.com/bazelbuild/vscode-bazel) extension.

## Source

- **Repository**: %s
- **Ref**: %s
- **Vendored**: %s
- **Tool**: go run ./tools/vendor-grammar

## Files

| File | License | Description |
|------|---------|-------------|
| starlark.tmLanguage.json | MIT (MagicPython) | TextMate grammar for Starlark |
| starlark.tmLanguage.license | - | License for the grammar file |
| starlark.configuration.json | Apache 2.0 | Language configuration (brackets, comments) |

## Licenses

### starlark.tmLanguage.json

The Starlark grammar is derived from MagicPython and is licensed under the
**MIT License**. See starlark.tmLanguage.license for the full license text.

### Other files

Other files are from the vscode-bazel project and are licensed under the
**Apache License 2.0**.

## Updating

To update the vendored files:

`+"```"+`bash
go run ./tools/vendor-grammar -ref master
`+"```"+`

To vendor from a specific tag or commit:

`+"```"+`bash
go run ./tools/vendor-grammar -ref v0.10.0
`+"```"+`

## Local Modifications

If you make local modifications to these files, document them here:

- (none yet)

## Attribution

The Starlark TextMate grammar is derived from:

- [MagicPython](https://github.com/MagicStack/MagicPython) - MIT License
- [vscode-bazel](https://github.com/bazelbuild/vscode-bazel) - Apache 2.0

`, repoURL, ref, time.Now().Format("2006-01-02"))
}

// ValidateGrammar checks that the grammar JSON is valid and has expected structure.
// This can be extended to check for specific patterns we depend on.
func ValidateGrammar(content []byte) error {
	var grammar map[string]any
	if err := json.Unmarshal(content, &grammar); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check required fields
	required := []string{"name", "scopeName", "patterns", "repository"}
	for _, field := range required {
		if _, ok := grammar[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Check scopeName
	if scopeName, ok := grammar["scopeName"].(string); !ok || scopeName != "source.starlark" {
		return fmt.Errorf("unexpected scopeName: %v", grammar["scopeName"])
	}

	return nil
}

// PatchGrammar applies Sky-specific patches to the grammar.
// This allows us to extend the upstream grammar with features like type annotations.
func PatchGrammar(content []byte) ([]byte, error) {
	var grammar map[string]any
	if err := json.Unmarshal(content, &grammar); err != nil {
		return nil, err
	}

	// Add Sky-specific patterns (type annotations, etc.)
	// For now, we don't patch - just validate and pass through
	// Future: Add type annotation highlighting

	// Re-encode with nice formatting
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(grammar); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
