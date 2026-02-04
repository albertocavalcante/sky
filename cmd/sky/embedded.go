package main

import (
	"context"
	"io"
)

// EmbeddedTool is a function that runs a tool with the given arguments.
// Returns the exit code.
type EmbeddedTool func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int

// embeddedTools is populated by embedded_full.go or embedded_minimal.go
// based on build tags.
var embeddedTools map[string]EmbeddedTool

// getEmbeddedTool returns an embedded tool by name, or nil if not found.
func getEmbeddedTool(name string) EmbeddedTool {
	if embeddedTools == nil {
		return nil
	}
	return embeddedTools[name]
}
