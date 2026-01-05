package main

import (
	"fmt"
	"io"
	"os"

	"github.com/albertocavalcante/sky/internal/cli"
)

func main() {
	cmd := cli.Command{
		Name:    "skyrepl",
		Summary: "Interactive Starlark REPL (placeholder).",
		Run:     run,
	}
	os.Exit(cli.Execute(cmd, os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, _ io.Writer) error {
	writeln(stdout, "skyrepl: Starlark REPL not implemented yet")
	writeln(stdout, "")
	writeln(stdout, "This will provide an interactive Starlark environment with:")
	writeln(stdout, "  - Dialect-aware evaluation (bazel, buck2, starlark)")
	writeln(stdout, "  - Tab completion for builtins and loaded symbols")
	writeln(stdout, "  - Type hover information (when --type-mode=enabled)")
	writeln(stdout, "  - Preload files with --preload=<file>")
	writeln(stdout, "")
	writeln(stdout, "Usage: skyrepl [flags] [<file>...]")
	writeln(stdout, "")
	writeln(stdout, "Options:")
	writeln(stdout, "  --dialect=...    Dialect: bazel, buck2, starlark (default: starlark)")
	writeln(stdout, "  --preload=...    Preload files before starting REPL")
	writeln(stdout, "  -e <expr>        Evaluate expression and exit")
	return nil
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
