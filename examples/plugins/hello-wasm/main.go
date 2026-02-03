//go:build wasip1

// Package main implements a minimal WASM Sky plugin.
//
// This example demonstrates how to build a plugin for WebAssembly.
// WASM plugins run in a sandboxed environment with limited capabilities.
//
// Build with:
//
//	GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm
//
// Or with TinyGo for smaller binaries:
//
//	tinygo build -o plugin.wasm -target=wasip1 .
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	pluginName    = "hello-wasm"
	pluginVersion = "1.0.0"
	pluginSummary = "A minimal WASM Sky plugin example"
)

func main() {
	// Verify we're running as a Sky plugin
	if os.Getenv("SKY_PLUGIN") != "1" {
		fmt.Fprintf(os.Stderr, "This is a Sky plugin. Run it with: sky %s\n", pluginName)
		os.Exit(1)
	}

	// Handle metadata request
	if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
		outputMetadata()
		return
	}

	// Run the plugin
	run()
}

func outputMetadata() {
	metadata := map[string]any{
		"api_version": 1,
		"name":        pluginName,
		"version":     pluginVersion,
		"summary":     pluginSummary,
		"commands": []map[string]string{
			{
				"name":    pluginName,
				"summary": pluginSummary,
			},
		},
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(metadata); err != nil {
		os.Exit(1)
	}
}

func run() {
	args := os.Args[1:]

	// Simple argument parsing (no flag package for TinyGo compatibility)
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printHelp()
			return
		case "-v", "--version":
			fmt.Printf("%s %s\n", pluginName, pluginVersion)
			return
		case "--env":
			printEnv()
			return
		}
	}

	// Default: greet
	name := "World"
	for i, arg := range args {
		if arg == "-name" || arg == "--name" {
			if i+1 < len(args) {
				name = args[i+1]
			}
		}
	}

	fmt.Printf("Hello from WASM, %s!\n", name)

	// Show workspace info if available
	if root := os.Getenv("SKY_WORKSPACE_ROOT"); root != "" {
		fmt.Printf("Workspace: %s\n", root)
	}

	// Note: WASM plugins cannot access the filesystem directly
	fmt.Println("(Running in WASM sandbox)")
}

func printHelp() {
	fmt.Printf("Usage: %s [options]\n\n", pluginName)
	fmt.Println("A minimal WASM plugin example.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help      Show this help message")
	fmt.Println("  -v, --version   Show version")
	fmt.Println("  --env           Show plugin environment variables")
	fmt.Println("  -name NAME      Name to greet (default: World)")
}

func printEnv() {
	envVars := []string{
		"SKY_PLUGIN",
		"SKY_PLUGIN_MODE",
		"SKY_PLUGIN_NAME",
		"SKY_WORKSPACE_ROOT",
		"SKY_CONFIG_DIR",
		"SKY_OUTPUT_FORMAT",
		"SKY_NO_COLOR",
		"SKY_VERBOSE",
	}

	for _, key := range envVars {
		value := os.Getenv(key)
		if value == "" {
			value = "(not set)"
		}
		fmt.Printf("%s=%s\n", key, value)
	}
}
