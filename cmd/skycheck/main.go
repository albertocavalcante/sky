package main

import (
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
		cli.Writeln(stdout, "skycheck: type checker not implemented yet")
		cli.Writeln(stdout, "")
		cli.Writeln(stdout, "Usage: skycheck [flags] <files...>")
		cli.Writeln(stdout, "")
		cli.Writeln(stdout, "Options:")
		cli.Writeln(stdout, "  --mode=ide|ci    Analysis mode (default: ci)")
		cli.Writeln(stdout, "  --type-mode=...  Type checking mode: disabled, parse_only, enabled")
		cli.Writeln(stdout, "  --json           Output diagnostics as JSON")
		return nil
	}

	cli.Writef(stdout, "skycheck: type checking %d file(s) is not implemented yet\n", len(args))
	return nil
}
