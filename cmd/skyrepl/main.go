package main

import (
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
	cli.Writeln(stdout, "skyrepl: Starlark REPL not implemented yet")
	cli.Writeln(stdout, "")
	cli.Writeln(stdout, "This will provide an interactive Starlark environment with:")
	cli.Writeln(stdout, "  - Dialect-aware evaluation (bazel, buck2, starlark)")
	cli.Writeln(stdout, "  - Tab completion for builtins and loaded symbols")
	cli.Writeln(stdout, "  - Type hover information (when --type-mode=enabled)")
	cli.Writeln(stdout, "  - Preload files with --preload=<file>")
	cli.Writeln(stdout, "")
	cli.Writeln(stdout, "Usage: skyrepl [flags] [<file>...]")
	cli.Writeln(stdout, "")
	cli.Writeln(stdout, "Options:")
	cli.Writeln(stdout, "  --dialect=...    Dialect: bazel, buck2, starlark (default: starlark)")
	cli.Writeln(stdout, "  --preload=...    Preload files before starting REPL")
	cli.Writeln(stdout, "  -e <expr>        Evaluate expression and exit")
	return nil
}
