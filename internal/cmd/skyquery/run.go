package skyquery

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/albertocavalcante/sky/internal/starlark/query"
	"github.com/albertocavalcante/sky/internal/starlark/query/index"
	"github.com/albertocavalcante/sky/internal/starlark/query/output"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK    = 0
	exitError = 1
)

// Run executes skyquery with the given arguments.
// Returns exit code.
func Run(args []string) int {
	return RunWithIO(context.Background(), args, os.Stdin, os.Stdout, os.Stderr)
}

// RunWithIO allows custom IO for embedding/testing.
func RunWithIO(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		outputFormat string
		workspace    string
		keepGoing    bool
		versionFlag  bool
	)

	fs := flag.NewFlagSet("skyquery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&outputFormat, "output", "name", "output format: name, location, json, count")
	fs.StringVar(&workspace, "workspace", ".", "workspace root directory")
	fs.BoolVar(&keepGoing, "keep_going", false, "continue on parse errors")
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")

	fs.Usage = func() {
		writeln(stderr, "Usage: skyquery [flags] <query>")
		writeln(stderr)
		writeln(stderr, "Queries Starlark sources.")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Output formats:")
		writeln(stderr, "  name      Names only, one per line (default)")
		writeln(stderr, "  location  File:line: name format")
		writeln(stderr, "  json      JSON output with full details")
		writeln(stderr, "  count     Count of results only")
		writeln(stderr)
		writeln(stderr, "Examples:")
		writeln(stderr, "  skyquery 'defs(//...)'                     # List all function definitions")
		writeln(stderr, "  skyquery 'loads(//internal/...)'           # List all loads in internal/")
		writeln(stderr, "  skyquery 'filter(\"^test_\", defs(//...))'   # Filter by pattern")
		writeln(stderr, "  skyquery --output=json 'defs(//...)'       # JSON output")
		writeln(stderr, "  skyquery --output=location 'calls(load, //...)'  # Location format")
		writeln(stderr, "  skyquery --output=count 'files(//...)'     # Count only")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skyquery %s\n", version.String())
		return exitOK
	}

	// Validate output format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		writef(stderr, "skyquery: %v\n", err)
		return exitError
	}

	// Get query string
	queryArgs := fs.Args()
	if len(queryArgs) == 0 {
		writeln(stderr, "skyquery: no query specified")
		fs.Usage()
		return exitError
	}
	if len(queryArgs) > 1 {
		writeln(stderr, "skyquery: only one query argument allowed")
		return exitError
	}

	queryStr := queryArgs[0]

	// Create index
	idx := index.New(workspace)

	// Index files based on query pattern
	// For now, we index all files at workspace root
	// The engine will filter based on the query
	count, errs := idx.AddPattern("//...")
	if len(errs) > 0 {
		for _, e := range errs {
			writef(stderr, "skyquery: warning: %v\n", e)
		}
		if !keepGoing && count == 0 {
			writeln(stderr, "skyquery: no files indexed")
			return exitError
		}
	}

	// Create engine and evaluate query
	engine := query.NewEngine(idx)
	result, err := engine.EvalString(queryStr)
	if err != nil {
		writef(stderr, "skyquery: %v\n", err)
		return exitError
	}

	// Wrap result for output formatting
	wrapped := &queryResultAdapter{
		query:  queryStr,
		result: result,
	}

	// Format and output results
	formatter := output.NewFormatterWithFormat(format)
	if err := formatter.Write(stdout, wrapped); err != nil {
		writef(stderr, "skyquery: %v\n", err)
		return exitError
	}

	return exitOK
}

// queryResultAdapter adapts query.Result to output.Result interface.
type queryResultAdapter struct {
	query  string
	result *query.Result
}

func (a *queryResultAdapter) Query() string {
	return a.query
}

func (a *queryResultAdapter) Items() []output.Item {
	items := make([]output.Item, len(a.result.Items))
	for i, item := range a.result.Items {
		items[i] = &queryItemAdapter{item: item}
	}
	return items
}

// queryItemAdapter adapts query.Item to output.Item interface.
type queryItemAdapter struct {
	item query.Item
}

func (a *queryItemAdapter) Type() string { return a.item.Type }
func (a *queryItemAdapter) Name() string { return a.item.Name }
func (a *queryItemAdapter) File() string { return a.item.File }
func (a *queryItemAdapter) Line() int    { return a.item.Line }

// Implement optional interfaces for type-specific data
func (a *queryItemAdapter) Params() []string {
	if def, ok := a.item.Data.(*index.Def); ok {
		return def.Params
	}
	if def, ok := a.item.Data.(index.Def); ok {
		return def.Params
	}
	return nil
}

func (a *queryItemAdapter) Docstring() string {
	if def, ok := a.item.Data.(*index.Def); ok {
		return def.Docstring
	}
	if def, ok := a.item.Data.(index.Def); ok {
		return def.Docstring
	}
	return ""
}

func (a *queryItemAdapter) Module() string {
	if load, ok := a.item.Data.(*index.Load); ok {
		return load.Module
	}
	if load, ok := a.item.Data.(index.Load); ok {
		return load.Module
	}
	return ""
}

func (a *queryItemAdapter) Symbols() map[string]string {
	if load, ok := a.item.Data.(*index.Load); ok {
		return load.Symbols
	}
	if load, ok := a.item.Data.(index.Load); ok {
		return load.Symbols
	}
	return nil
}

func (a *queryItemAdapter) Function() string {
	if call, ok := a.item.Data.(*index.Call); ok {
		return call.Function
	}
	if call, ok := a.item.Data.(index.Call); ok {
		return call.Function
	}
	return ""
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
