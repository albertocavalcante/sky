package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go.starlark.net/lib/json"
	"go.starlark.net/lib/math"
	"go.starlark.net/lib/time"
	"go.starlark.net/repl"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"golang.org/x/term"

	"github.com/albertocavalcante/sky/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var (
		execExpr    string
		preloadFlag string
		showEnv     bool
		recursion   bool
		versionFlag bool
	)

	fs := flag.NewFlagSet("skyrepl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&execExpr, "e", "", "evaluate `expr` and exit")
	fs.StringVar(&preloadFlag, "preload", "", "comma-separated files to preload")
	fs.BoolVar(&showEnv, "showenv", false, "print final environment on exit")
	fs.BoolVar(&recursion, "recursion", false, "allow recursion and while statements")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")

	fs.Usage = func() {
		writeln(stderr, "Usage: skyrepl [flags] [file]")
		writeln(stderr)
		writeln(stderr, "Interactive Starlark REPL.")
		writeln(stderr)
		writeln(stderr, "With no arguments, starts an interactive read-eval-print loop.")
		writeln(stderr, "With a file argument, executes the file and exits.")
		writeln(stderr)
		writeln(stderr, "Built-in modules: json, math, time")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skyrepl                     # Start interactive REPL")
		writeln(stderr, "  skyrepl script.star         # Execute file")
		writeln(stderr, "  skyrepl -e '1 + 1'          # Evaluate expression")
		writeln(stderr, "  skyrepl -preload lib.star   # Preload file, then start REPL")
		writeln(stderr)
		writeln(stderr, "REPL shortcuts:")
		writeln(stderr, "  _                           # Value of last expression")
		writeln(stderr, "  Ctrl-C                      # Cancel current input")
		writeln(stderr, "  Ctrl-D                      # Exit REPL")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if versionFlag {
		writef(stdout, "skyrepl %s\n", version.String())
		return 0
	}

	// Configure dialect
	if recursion {
		resolve.AllowRecursion = true
	}

	// Set up predeclared modules
	starlark.Universe["json"] = json.Module
	starlark.Universe["time"] = time.Module
	starlark.Universe["math"] = math.Module

	// Create thread and globals
	thread := &starlark.Thread{Load: repl.MakeLoad()}
	globals := make(starlark.StringDict)

	// Preload files
	if preloadFlag != "" {
		for _, file := range strings.Split(preloadFlag, ",") {
			file = strings.TrimSpace(file)
			if file == "" {
				continue
			}
			thread.Name = "exec " + file
			fileGlobals, err := starlark.ExecFile(thread, file, nil, nil)
			if err != nil {
				repl.PrintError(err)
				return 1
			}
			// Merge into globals
			for k, v := range fileGlobals {
				globals[k] = v
			}
		}
	}

	// Mode: execute expression (-e flag)
	if execExpr != "" {
		thread.Name = "eval"
		v, err := starlark.Eval(thread, "<expr>", execExpr, globals)
		if err != nil {
			repl.PrintError(err)
			return 1
		}
		if v != starlark.None {
			writeln(stdout, v.String())
		}
		return 0
	}

	// Mode: execute file
	if fs.NArg() == 1 {
		filename := fs.Arg(0)
		thread.Name = "exec " + filename
		var err error
		globals, err = starlark.ExecFile(thread, filename, nil, globals)
		if err != nil {
			repl.PrintError(err)
			return 1
		}
		if showEnv {
			printEnv(stdout, globals)
		}
		return 0
	}

	// Mode: too many args
	if fs.NArg() > 1 {
		writeln(stderr, "skyrepl: expected at most one file argument")
		return 2
	}

	// Mode: interactive REPL
	stdinIsTerminal := term.IsTerminal(int(os.Stdin.Fd()))
	if stdinIsTerminal {
		writef(stdout, "skyrepl %s (Starlark REPL)\n", version.String())
		writeln(stdout, "Type expressions to evaluate. Use Ctrl-D to exit.")
		writeln(stdout, "Built-in modules: json, math, time")
		writeln(stdout)
	}

	thread.Name = "REPL"
	repl.REPLOptions(syntax.LegacyFileOptions(), thread, globals)

	if stdinIsTerminal {
		writeln(stdout)
	}

	if showEnv {
		printEnv(stdout, globals)
	}

	return 0
}

func printEnv(w io.Writer, globals starlark.StringDict) {
	for _, name := range globals.Keys() {
		if !strings.HasPrefix(name, "_") {
			writef(w, "%s = %s\n", name, globals[name])
		}
	}
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
