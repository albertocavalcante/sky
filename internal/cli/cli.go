package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/albertocavalcante/sky/internal/version"
)

// Command defines a single CLI entrypoint.
type Command struct {
	Name    string
	Summary string
	Run     func(args []string, stdout, stderr io.Writer) error
}

// Execute runs the command and returns a process exit code.
func Execute(cmd Command, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		writef(stderr, "usage: %s [flags]\n\n%s\n\nflags:\n", cmd.Name, cmd.Summary)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		writeln(stderr, err)
		return 2
	}

	if *showVersion {
		writef(stdout, "%s %s\n", cmd.Name, version.String())
		return 0
	}

	if cmd.Run == nil {
		writef(stderr, "%s: no command configured\n", cmd.Name)
		return 1
	}

	if err := cmd.Run(fs.Args(), stdout, stderr); err != nil {
		writef(stderr, "%s: %v\n", cmd.Name, err)
		return 1
	}

	return 0
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
