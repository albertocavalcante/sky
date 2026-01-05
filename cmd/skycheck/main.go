package main

import (
	"fmt"
	"io"
	"os"

	"github.com/albertocavalcante/sky/internal/cli"
)

func main() {
	cmd := cli.Command{
		Name:    "skycheck",
		Summary: "Type check Starlark files (placeholder).",
		Run:     run,
	}
	os.Exit(cli.Execute(cmd, os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, _ io.Writer) error {
	if len(args) == 0 {
		writeln(stdout, "skycheck: type checker not implemented yet")
		writeln(stdout, "")
		writeln(stdout, "Usage: skycheck [flags] <files...>")
		writeln(stdout, "")
		writeln(stdout, "Options:")
		writeln(stdout, "  --mode=ide|ci    Analysis mode (default: ci)")
		writeln(stdout, "  --type-mode=...  Type checking mode: disabled, parse_only, enabled")
		writeln(stdout, "  --json           Output diagnostics as JSON")
		return nil
	}

	writef(stdout, "skycheck: type checking %d file(s) is not implemented yet\n", len(args))
	return nil
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
