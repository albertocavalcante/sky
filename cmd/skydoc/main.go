package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/albertocavalcante/sky/internal/starlark/docgen"
	"github.com/albertocavalcante/sky/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var (
		outputFlag  string
		formatFlag  string
		privateFlag bool
		titleFlag   string
		tocFlag     bool
		versionFlag bool
	)

	fs := flag.NewFlagSet("skydoc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&outputFlag, "o", "", "output file (default: stdout)")
	fs.StringVar(&formatFlag, "format", "markdown", "output format: markdown, json")
	fs.BoolVar(&privateFlag, "private", false, "include private symbols (starting with _)")
	fs.StringVar(&titleFlag, "title", "", "document title (default: filename)")
	fs.BoolVar(&tocFlag, "toc", true, "include table of contents")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")

	fs.Usage = func() {
		writeln(stderr, "Usage: skydoc [flags] <file.star>")
		writeln(stderr)
		writeln(stderr, "Generate documentation from Starlark files.")
		writeln(stderr)
		writeln(stderr, "Extracts docstrings and generates formatted documentation.")
		writeln(stderr, "Supports Python-style docstrings with Args:, Returns:, etc.")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skydoc lib.star                    # Print markdown to stdout")
		writeln(stderr, "  skydoc -o docs/lib.md lib.star     # Write to file")
		writeln(stderr, "  skydoc -format json lib.star       # JSON output")
		writeln(stderr, "  skydoc -private lib.star           # Include private symbols")
		writeln(stderr)
		writeln(stderr, "Docstring format:")
		writeln(stderr, "  def my_func(name, count=1):")
		writeln(stderr, "      \"\"\"Short description.")
		writeln(stderr)
		writeln(stderr, "      Longer description here.")
		writeln(stderr)
		writeln(stderr, "      Args:")
		writeln(stderr, "          name: The name parameter.")
		writeln(stderr, "          count: How many times (default: 1).")
		writeln(stderr)
		writeln(stderr, "      Returns:")
		writeln(stderr, "          A result string.")
		writeln(stderr, "      \"\"\"")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if versionFlag {
		writef(stdout, "skydoc %s\n", version.String())
		return 0
	}

	if fs.NArg() != 1 {
		writeln(stderr, "skydoc: expected exactly one file argument")
		fs.Usage()
		return 2
	}

	filename := fs.Arg(0)

	// Read source file
	src, err := os.ReadFile(filename)
	if err != nil {
		writef(stderr, "skydoc: %v\n", err)
		return 1
	}

	// Extract documentation
	opts := docgen.Options{
		IncludePrivate: privateFlag,
	}
	doc, err := docgen.ExtractFile(filename, src, opts)
	if err != nil {
		writef(stderr, "skydoc: %v\n", err)
		return 1
	}

	// Determine output writer
	var out io.Writer = stdout
	if outputFlag != "" {
		// Ensure output directory exists
		dir := filepath.Dir(outputFlag)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				writef(stderr, "skydoc: %v\n", err)
				return 1
			}
		}

		f, err := os.Create(outputFlag)
		if err != nil {
			writef(stderr, "skydoc: %v\n", err)
			return 1
		}
		defer func() { _ = f.Close() }()
		out = f
	}

	// Generate output
	switch formatFlag {
	case "markdown", "md":
		mdOpts := docgen.MarkdownOptions{
			Title:                  titleFlag,
			IncludeTableOfContents: tocFlag,
		}
		if err := docgen.RenderMarkdown(out, doc, mdOpts); err != nil {
			writef(stderr, "skydoc: %v\n", err)
			return 1
		}

	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(doc); err != nil {
			writef(stderr, "skydoc: %v\n", err)
			return 1
		}

	default:
		writef(stderr, "skydoc: unknown format %q (use markdown or json)\n", formatFlag)
		return 2
	}

	return 0
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
