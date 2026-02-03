package skyplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Output provides consistent output formatting for plugins.
type Output struct {
	stdout io.Writer
	stderr io.Writer
}

// DefaultOutput creates an Output that writes to os.Stdout and os.Stderr.
func DefaultOutput() *Output {
	return &Output{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// NewOutput creates an Output with custom writers.
func NewOutput(stdout, stderr io.Writer) *Output {
	return &Output{
		stdout: stdout,
		stderr: stderr,
	}
}

// WriteJSON writes a value as JSON to stdout.
func (o *Output) WriteJSON(v any) error {
	enc := json.NewEncoder(o.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WriteResult writes output based on the current output format.
// If JSON output is requested, v is encoded as JSON.
// Otherwise, textFn is called to generate human-readable text.
func (o *Output) WriteResult(v any, textFn func() string) error {
	if IsJSONOutput() {
		return o.WriteJSON(v)
	}
	_, err := fmt.Fprintln(o.stdout, textFn())
	return err
}

// Println writes a line to stdout.
func (o *Output) Println(args ...any) {
	_, _ = fmt.Fprintln(o.stdout, args...)
}

// Printf writes formatted output to stdout.
func (o *Output) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(o.stdout, format, args...)
}

// Error writes an error message to stderr.
func (o *Output) Error(args ...any) {
	_, _ = fmt.Fprintln(o.stderr, args...)
}

// Errorf writes a formatted error message to stderr.
func (o *Output) Errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(o.stderr, format, args...)
}

// Verbose prints a message only if verbosity is at least the given level.
func (o *Output) Verbose(level int, args ...any) {
	if Verbose() >= level {
		_, _ = fmt.Fprintln(o.stderr, args...)
	}
}

// Verbosef prints a formatted message only if verbosity is at least the given level.
func (o *Output) Verbosef(level int, format string, args ...any) {
	if Verbose() >= level {
		_, _ = fmt.Fprintf(o.stderr, format, args...)
	}
}
