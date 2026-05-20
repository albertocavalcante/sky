package ci

import (
	"fmt"
	"io"
)

// printf is a fmt.Fprintf wrapper that discards the (n, err) returns.
// Use only for diagnostic / informational output to stderr or to
// GitHub-Actions output files where the error cannot be meaningfully
// recovered from. For real I/O paths, use fmt.Fprintf directly and
// handle the error.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// println is the fmt.Fprintln equivalent of printf.
func println(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
