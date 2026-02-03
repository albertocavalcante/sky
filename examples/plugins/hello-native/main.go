// Package main implements a minimal native Sky plugin.
//
// This example demonstrates the plugin protocol without any external dependencies.
// It shows how to:
//   - Handle the metadata mode
//   - Parse environment variables
//   - Handle command-line arguments
//   - Use the workspace root
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const (
	pluginName    = "hello-native"
	pluginVersion = "1.0.0"
	pluginSummary = "A minimal native Sky plugin example"
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
	os.Exit(run(os.Args[1:]))
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

func run(args []string) int {
	fs := flag.NewFlagSet(pluginName, flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "show version")
	showEnv := fs.Bool("env", false, "show plugin environment variables")
	name := fs.String("name", "World", "name to greet")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Printf("%s %s\n", pluginName, pluginVersion)
		return 0
	}

	if *showEnv {
		printEnv()
		return 0
	}

	// Greet the user
	fmt.Printf("Hello, %s!\n", *name)

	// Show workspace info if available
	if root := os.Getenv("SKY_WORKSPACE_ROOT"); root != "" {
		fmt.Printf("Workspace: %s\n", root)
	}

	return 0
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
