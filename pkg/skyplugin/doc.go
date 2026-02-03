// Package skyplugin provides a developer SDK for building Sky plugins.
//
// This package eliminates boilerplate code by providing:
//   - Environment variable helpers for reading plugin context
//   - Metadata handling for plugin discovery
//   - Output formatting for consistent CLI output
//   - A simple Serve() entrypoint that handles the plugin protocol
//
// # Quick Start
//
// A minimal plugin using the SDK:
//
//	package main
//
//	import (
//		"context"
//		"fmt"
//		"os"
//
//		"github.com/albertocavalcante/sky/pkg/skyplugin"
//	)
//
//	func main() {
//		skyplugin.Serve(skyplugin.Plugin{
//			Metadata: skyplugin.Metadata{
//				APIVersion: 1,
//				Name:       "hello",
//				Version:    "0.1.0",
//				Summary:    "A hello world plugin",
//			},
//			Run: func(ctx context.Context, args []string) error {
//				fmt.Println("Hello from Sky!")
//				return nil
//			},
//		})
//	}
//
// # Environment Variables
//
// The SDK provides helper functions to read plugin environment variables:
//
//	skyplugin.IsPlugin()        // Returns true if running as a Sky plugin
//	skyplugin.PluginName()      // Returns the plugin name
//	skyplugin.WorkspaceRoot()   // Returns the workspace root directory
//	skyplugin.ConfigDir()       // Returns the Sky config directory
//	skyplugin.OutputFormat()    // Returns "text" or "json"
//	skyplugin.NoColor()         // Returns true if color output is disabled
//	skyplugin.Verbose()         // Returns verbosity level (0-3)
//
// # Output Formatting
//
// The Output type provides consistent output formatting:
//
//	out := skyplugin.DefaultOutput()
//	out.WriteResult(data, func() string { return "Human readable output" })
//
// This automatically handles JSON vs text output based on SKY_OUTPUT_FORMAT.
//
// # Testing
//
// The skyplugin/testing package provides utilities for testing plugins:
//
//	import "github.com/albertocavalcante/sky/pkg/skyplugin/testing"
//
//	func TestPlugin(t *testing.T) {
//		cleanup := testing.MockEnv("exec", "my-plugin")
//		defer cleanup()
//
//		result := testing.CaptureOutput(func() {
//			myPluginMain()
//		})
//
//		if result.ExitCode != 0 {
//			t.Errorf("unexpected exit code: %d", result.ExitCode)
//		}
//	}
package skyplugin
