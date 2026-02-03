package skyplugin

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

// Plugin defines a Sky plugin.
type Plugin struct {
	// Metadata describes the plugin for discovery.
	Metadata Metadata

	// Run is the main entry point for the plugin.
	// It receives the CLI arguments (excluding the program name).
	Run func(ctx context.Context, args []string) error
}

// Serve is the main entrypoint for plugins.
// It handles the plugin protocol:
//   - If running in metadata mode, outputs metadata and exits
//   - Otherwise, calls the Run function with a cancellable context
//
// Usage:
//
//	func main() {
//		skyplugin.Serve(skyplugin.Plugin{
//			Metadata: skyplugin.Metadata{
//				APIVersion: 1,
//				Name:       "my-plugin",
//				Version:    "1.0.0",
//				Summary:    "Does something useful",
//			},
//			Run: func(ctx context.Context, args []string) error {
//				// Plugin logic here
//				return nil
//			},
//		})
//	}
func Serve(p Plugin) {
	// Check if we're running as a plugin
	if !IsPlugin() {
		fmt.Fprintf(os.Stderr, "This is a Sky plugin. Run it with: sky %s\n", p.Metadata.Name)
		os.Exit(1)
	}

	// Handle metadata request
	if IsMetadataMode() {
		HandleMetadata(p.Metadata)
		return // HandleMetadata calls os.Exit
	}

	// Set up context with cancellation on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Run the plugin
	args := os.Args[1:]
	if err := p.Run(ctx, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ServeFunc is a convenience wrapper around Serve for simple plugins.
//
// Usage:
//
//	func main() {
//		skyplugin.ServeFunc(
//			skyplugin.Metadata{Name: "hello", Version: "1.0.0"},
//			func(ctx context.Context, args []string) error {
//				fmt.Println("Hello!")
//				return nil
//			},
//		)
//	}
func ServeFunc(m Metadata, run func(ctx context.Context, args []string) error) {
	Serve(Plugin{Metadata: m, Run: run})
}
