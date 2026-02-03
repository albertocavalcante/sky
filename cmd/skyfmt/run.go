package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/formatter"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK          = 0
	exitNeedsFormat = 1 // --check mode: files need formatting
	exitError       = 2 // error occurred
)

// Run executes skyfmt with the given arguments.
// Returns exit code.
func Run(args []string) int {
	return RunWithIO(context.Background(), args, os.Stdin, os.Stdout, os.Stderr)
}

// RunWithIO allows custom IO for embedding/testing.
func RunWithIO(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var (
		writeFlag   bool
		diffFlag    bool
		checkFlag   bool
		typeFlag    string
		versionFlag bool
	)

	fs := flag.NewFlagSet("skyfmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&writeFlag, "w", false, "write result to file instead of stdout")
	fs.BoolVar(&diffFlag, "d", false, "display diff instead of formatted output")
	fs.BoolVar(&checkFlag, "check", false, "exit with non-zero status if files need formatting")
	fs.StringVar(&typeFlag, "type", "", "file type: build, bzl, workspace, module, default")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")

	fs.Usage = func() {
		writeln(stderr, "Usage: skyfmt [flags] [path ...]")
		writeln(stderr)
		writeln(stderr, "Formats Starlark files. With no paths, reads from stdin and writes to stdout.")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "File types:")
		writeln(stderr, "  build      BUILD, BUILD.bazel files")
		writeln(stderr, "  bzl        .bzl extension files")
		writeln(stderr, "  workspace  WORKSPACE files")
		writeln(stderr, "  module     MODULE.bazel files")
		writeln(stderr, "  default    Generic Starlark files")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skyfmt %s\n", version.String())
		return exitOK
	}

	// Validate flag combinations
	if writeFlag && diffFlag {
		writeln(stderr, "skyfmt: cannot use -w and -d together")
		return exitError
	}
	if writeFlag && checkFlag {
		writeln(stderr, "skyfmt: cannot use -w and --check together")
		return exitError
	}

	kind := parseTypeFlag(typeFlag)
	paths := fs.Args()

	// No paths: read from stdin
	if len(paths) == 0 {
		return formatStdin(stdin, stdout, stderr, kind, checkFlag, diffFlag)
	}

	// Format files
	return formatPaths(paths, stdout, stderr, kind, writeFlag, diffFlag, checkFlag)
}

func parseTypeFlag(t string) filekind.Kind {
	switch strings.ToLower(t) {
	case "build":
		return filekind.KindBUILD
	case "bzl":
		return filekind.KindBzl
	case "workspace":
		return filekind.KindWORKSPACE
	case "module":
		return filekind.KindMODULE
	case "default", "starlark":
		return filekind.KindStarlark
	default:
		return "" // auto-detect
	}
}

func formatStdin(stdin io.Reader, stdout, stderr io.Writer, kind filekind.Kind, checkFlag, diffFlag bool) int {
	src, err := io.ReadAll(stdin)
	if err != nil {
		writef(stderr, "skyfmt: reading stdin: %v\n", err)
		return exitError
	}

	// Use default kind if not specified
	if kind == "" {
		kind = filekind.KindStarlark
	}

	formatted, err := formatter.Format(src, "<stdin>", kind)
	if err != nil {
		writef(stderr, "skyfmt: %v\n", err)
		return exitError
	}

	if checkFlag {
		if !bytes.Equal(src, formatted) {
			writeln(stderr, "<stdin>")
			return exitNeedsFormat
		}
		return exitOK
	}

	if diffFlag {
		diff := computeDiff("<stdin>", src, formatted)
		if diff != "" {
			write(stdout, diff)
		}
		return exitOK
	}

	writeBytes(stdout, formatted)
	return exitOK
}

func formatPaths(paths []string, stdout, stderr io.Writer, kind filekind.Kind, writeFlag, diffFlag, checkFlag bool) int {
	var files []string

	// Expand paths (including directories)
	for _, path := range paths {
		expanded, err := expandPath(path)
		if err != nil {
			writef(stderr, "skyfmt: %v\n", err)
			return exitError
		}
		files = append(files, expanded...)
	}

	if len(files) == 0 {
		writeln(stderr, "skyfmt: no files to format")
		return exitOK
	}

	needsFormat := false
	hasError := false

	for _, path := range files {
		var result *formatter.Result
		if kind != "" {
			result = formatter.FormatFileWithKind(path, kind)
		} else {
			result = formatter.FormatFile(path)
		}

		if result.Err != nil {
			writef(stderr, "skyfmt: %s: %v\n", path, result.Err)
			hasError = true
			continue
		}

		if !result.Changed() {
			continue
		}

		needsFormat = true

		if checkFlag {
			writeln(stdout, path)
			continue
		}

		if writeFlag {
			if err := os.WriteFile(path, result.Formatted, 0644); err != nil {
				writef(stderr, "skyfmt: %s: %v\n", path, err)
				hasError = true
				continue
			}
			continue
		}

		if diffFlag {
			diff := computeDiff(path, result.Original, result.Formatted)
			if diff != "" {
				write(stdout, diff)
			}
			continue
		}

		// Default: print formatted output
		writef(stdout, "==> %s <==\n", path)
		writeBytes(stdout, result.Formatted)
		writeln(stdout)
	}

	if hasError {
		return exitError
	}
	if checkFlag && needsFormat {
		return exitNeedsFormat
	}
	return exitOK
}

// expandPath expands a path to a list of files to format.
// If path is a directory, it recursively finds all Starlark files.
func expandPath(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if filekind.IsStarlarkFile(d.Name()) {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

// computeDiff returns a unified diff between original and formatted content.
// This is a simple line-by-line diff.
func computeDiff(path string, original, formatted []byte) string {
	if bytes.Equal(original, formatted) {
		return ""
	}

	var buf strings.Builder
	_, _ = buf.WriteString(fmt.Sprintf("--- %s\n", path))
	_, _ = buf.WriteString(fmt.Sprintf("+++ %s\n", path))

	origLines := strings.Split(string(original), "\n")
	fmtLines := strings.Split(string(formatted), "\n")

	// Simple diff: show all changes
	// For a production tool, we'd use a proper diff algorithm
	_, _ = buf.WriteString("@@ -1 +1 @@\n")
	for _, line := range origLines {
		_, _ = buf.WriteString("-" + line + "\n")
	}
	for _, line := range fmtLines {
		_, _ = buf.WriteString("+" + line + "\n")
	}

	return buf.String()
}

// Helper functions for writing output.
// Write errors are intentionally ignored because:
//  1. These functions write to stdout/stderr where there's no reasonable recovery
//     if the terminal/pipe is broken (EPIPE, etc.)
//  2. If we can't write error messages, we can't report the write failure either
//  3. The exit code still reflects the actual operation status
//
// Note: File output (-w flag) uses os.WriteFile which handles errors properly.
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func write(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

func writeBytes(w io.Writer, b []byte) {
	_, _ = w.Write(b)
}
