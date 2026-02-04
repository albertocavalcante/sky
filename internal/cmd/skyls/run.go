package skyls

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/albertocavalcante/sky/internal/lsp"
	"github.com/albertocavalcante/sky/internal/version"
)

// Exit codes
const (
	exitOK    = 0
	exitError = 1
)

// Run executes skyls with the given arguments.
func Run(args []string) int {
	return RunWithIO(context.Background(), args, os.Stdin, os.Stdout, os.Stderr)
}

// RunWithIO allows custom IO for testing.
func RunWithIO(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var (
		versionFlag bool
		verboseFlag bool
	)

	fs := flag.NewFlagSet("skyls", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&versionFlag, "version", false, "print version and exit")
	fs.BoolVar(&verboseFlag, "v", false, "verbose logging to stderr")

	fs.Usage = func() {
		writeln(stderr, "Usage: skyls [flags]")
		writeln(stderr)
		writeln(stderr, "Starlark Language Server Protocol (LSP) implementation.")
		writeln(stderr)
		writeln(stderr, "The server communicates over stdio using JSON-RPC 2.0.")
		writeln(stderr, "Configure your editor to launch this binary as an LSP server.")
		writeln(stderr)
		writeln(stderr, "Features:")
		writeln(stderr, "  - Hover documentation")
		writeln(stderr, "  - Go to definition")
		writeln(stderr, "  - Document symbols")
		writeln(stderr, "  - Code completion")
		writeln(stderr, "  - Formatting (via skyfmt)")
		writeln(stderr, "  - Diagnostics (via skylint, skycheck)")
		writeln(stderr)
		writeln(stderr, "Flags:")
		fs.PrintDefaults()
		writeln(stderr)
		writeln(stderr, "Editor Configuration:")
		writeln(stderr, "  VS Code:  Install Starlark extension, set skyls as server")
		writeln(stderr, "  Neovim:   Use nvim-lspconfig with custom server config")
		writeln(stderr, "  Helix:    Add to languages.toml")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if versionFlag {
		writef(stdout, "skyls %s\n", version.String())
		return exitOK
	}

	// Setup logging
	if verboseFlag {
		log.SetOutput(stderr)
		log.SetFlags(log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	// Create context with cancellation for clean shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create server
	server := lsp.NewServer(cancel)

	// Create stdio connection
	rwc := &stdioConn{
		Reader: stdin,
		Writer: stdout,
	}

	conn := lsp.NewConn(rwc, server)
	server.SetConn(conn)

	log.Printf("skyls: starting server")

	// Run the server
	if err := conn.Run(ctx); err != nil && ctx.Err() == nil {
		writef(stderr, "skyls: %v\n", err)
		return exitError
	}

	log.Printf("skyls: server stopped")
	return exitOK
}

// stdioConn wraps stdin/stdout as an io.ReadWriteCloser.
type stdioConn struct {
	io.Reader
	io.Writer
}

func (s *stdioConn) Close() error {
	return nil
}

func writef(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	fmt.Fprintln(w, args...)
}
