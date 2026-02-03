package cli

import (
	"fmt"
	"io"
)

// Writef writes formatted output to the writer.
//
// This is a convenience wrapper around fmt.Fprintf that ignores write errors.
// Use this for CLI output where there's no reasonable recovery from write failures
// to stdout/stderr.
//
// Example:
//
//	cli.Writef(stdout, "Processing %d files...\n", count)
func Writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// Writeln writes a line to the writer.
//
// This is a convenience wrapper around fmt.Fprintln that ignores write errors.
// Use this for CLI output where there's no reasonable recovery from write failures
// to stdout/stderr.
//
// Example:
//
//	cli.Writeln(stderr, "error:", err)
//	cli.Writeln(stdout) // blank line
func Writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

// Write writes a string to the writer.
//
// This is a convenience wrapper around io.WriteString that ignores write errors.
// Use this for CLI output where there's no reasonable recovery from write failures.
//
// Example:
//
//	cli.Write(stdout, diff)
func Write(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

// WriteBytes writes bytes to the writer.
//
// This is a convenience wrapper around w.Write that ignores write errors.
// Use this for CLI output where there's no reasonable recovery from write failures.
//
// Example:
//
//	cli.WriteBytes(stdout, formattedContent)
func WriteBytes(w io.Writer, b []byte) {
	_, _ = w.Write(b)
}
