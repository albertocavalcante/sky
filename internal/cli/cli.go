package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/albertocavalcante/sky/internal/version"
)

// ExitCodeError is an error that specifies a particular exit code.
type ExitCodeError int

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e)
}

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
		Writef(stderr, "usage: %s [flags]\n\n%s\n\nflags:\n", cmd.Name, cmd.Summary)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ExitOK
		}
		Writeln(stderr, err)
		return ExitWarning
	}

	if *showVersion {
		Writef(stdout, "%s %s\n", cmd.Name, version.String())
		return ExitOK
	}

	if cmd.Run == nil {
		Writef(stderr, "%s: no command configured\n", cmd.Name)
		return ExitError
	}

	if err := cmd.Run(fs.Args(), stdout, stderr); err != nil {
		// Check for explicit exit code
		if exitErr, ok := err.(ExitCodeError); ok {
			return int(exitErr)
		}
		Writef(stderr, "%s: %v\n", cmd.Name, err)
		return ExitError
	}

	return ExitOK
}
