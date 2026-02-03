package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/albertocavalcante/sky/internal/starlark/checker"
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
		jsonFlag    bool
		versionFlag bool
		quietFlag   bool
	)

	fs := flag.NewFlagSet("skycheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&jsonFlag, "json", false, "output diagnostics as JSON")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")
	fs.BoolVar(&quietFlag, "quiet", false, "only output errors, suppress warnings")

	fs.Usage = func() {
		writeln(stderr, "Usage: skycheck [flags] <files...>")
		writeln(stderr)
		writeln(stderr, "Static analysis for Starlark files.")
		writeln(stderr)
		writeln(stderr, "Checks for:")
		writeln(stderr, "  - Undefined names")
		writeln(stderr, "  - Unused local variables")
		writeln(stderr, "  - Parse errors")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skycheck file.star              # Check a single file")
		writeln(stderr, "  skycheck *.star                 # Check multiple files")
		writeln(stderr, "  skycheck --json file.star       # Output as JSON")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skycheck %s\n", version.String())
		return exitOK
	}

	paths := fs.Args()
	if len(paths) == 0 {
		writeln(stderr, "skycheck: no files specified")
		fs.Usage()
		return exitError
	}

	// Expand paths (handle globs on systems that don't expand them)
	var files []string
	for _, pattern := range paths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			writef(stderr, "skycheck: invalid pattern %q: %v\n", pattern, err)
			return exitError
		}
		if len(matches) == 0 {
			// No glob match, treat as literal path
			files = append(files, pattern)
		} else {
			files = append(files, matches...)
		}
	}

	// Create checker with default options
	opts := checker.DefaultOptions()
	c := checker.New(opts)

	// Check all files
	result := checker.Result{FileCount: len(files)}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			writef(stderr, "skycheck: %v\n", err)
			return exitError
		}

		diags, err := c.CheckFile(path, src)
		if err != nil {
			writef(stderr, "skycheck: %v\n", err)
			return exitError
		}

		result.Diagnostics = append(result.Diagnostics, diags...)
	}

	// Filter if quiet mode
	if quietFlag {
		var filtered []checker.Diagnostic
		for _, d := range result.Diagnostics {
			if d.Severity == checker.SeverityError {
				filtered = append(filtered, d)
			}
		}
		result.Diagnostics = filtered
	}

	// Output results
	if jsonFlag {
		return outputJSON(stdout, result)
	}
	return outputText(stdout, result)
}

func outputText(w io.Writer, result checker.Result) int {
	// Group by file
	byFile := make(map[string][]checker.Diagnostic)
	for _, d := range result.Diagnostics {
		byFile[d.Pos.Filename()] = append(byFile[d.Pos.Filename()], d)
	}

	for file, diags := range byFile {
		for _, d := range diags {
			severity := strings.ToLower(d.Severity.String())
			writef(w, "%s:%d:%d: %s: %s [%s]\n",
				file, d.Pos.Line, d.Pos.Col,
				severity, d.Message, d.Code)
		}
	}

	// Summary
	if len(result.Diagnostics) > 0 {
		writeln(w)
	}
	errors := result.ErrorCount()
	warnings := result.WarningCount()
	if errors > 0 || warnings > 0 {
		writef(w, "Found %d error(s) and %d warning(s) in %d file(s)\n",
			errors, warnings, result.FileCount)
	} else {
		writef(w, "Checked %d file(s), no issues found\n", result.FileCount)
	}

	// Return code
	if errors > 0 {
		return exitError
	}
	if warnings > 0 {
		return exitWarning
	}
	return exitOK
}

type jsonOutput struct {
	Files       int              `json:"files"`
	Errors      int              `json:"errors"`
	Warnings    int              `json:"warnings"`
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
}

type jsonDiagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func outputJSON(w io.Writer, result checker.Result) int {
	out := jsonOutput{
		Files:       result.FileCount,
		Errors:      result.ErrorCount(),
		Warnings:    result.WarningCount(),
		Diagnostics: make([]jsonDiagnostic, 0, len(result.Diagnostics)),
	}

	for _, d := range result.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, jsonDiagnostic{
			File:     d.Pos.Filename(),
			Line:     int(d.Pos.Line),
			Column:   int(d.Pos.Col),
			Severity: strings.ToLower(d.Severity.String()),
			Code:     d.Code,
			Message:  d.Message,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return exitError
	}

	if out.Errors > 0 {
		return exitError
	}
	if out.Warnings > 0 {
		return exitWarning
	}
	return exitOK
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
