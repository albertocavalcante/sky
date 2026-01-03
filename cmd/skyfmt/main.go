package main

import (
	"fmt"
	"io"
	"os"

	"github.com/albertocavalcante/sky/internal/cli"
)

func main() {
	cmd := cli.Command{
		Name:    "skyfmt",
		Summary: "Format Starlark files (placeholder).",
		Run:     run,
	}
	os.Exit(cli.Execute(cmd, os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, _ io.Writer) error {
	if len(args) == 0 {
		writeln(stdout, "skyfmt: formatter not implemented yet")
		return nil
	}

	writef(stdout, "skyfmt: formatting %d file(s) is not implemented yet\n", len(args))
	return nil
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
