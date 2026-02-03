//go:build !sky_full

package main

func init() {
	// No embedded tools in minimal build.
	// Tools are resolved via external binaries or plugin system.
	embeddedTools = nil
}
