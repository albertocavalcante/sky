// Package testing provides utilities for testing Sky plugins.
package testing

import (
	"os"
)

// EnvConfig holds configuration for mocking plugin environment.
type EnvConfig struct {
	Mode          string // "exec" or "metadata"
	Name          string // Plugin name
	WorkspaceRoot string // Workspace root directory
	ConfigDir     string // Config directory
	OutputFormat  string // "text" or "json"
	NoColor       bool   // Disable color output
	Verbose       int    // Verbosity level (0-3)
}

// MockEnv sets up plugin environment variables for testing.
// Returns a cleanup function that restores the original environment.
//
// Usage:
//
//	func TestPlugin(t *testing.T) {
//		cleanup := testing.MockEnv("exec", "my-plugin")
//		defer cleanup()
//
//		// Run plugin code...
//	}
func MockEnv(mode, name string) func() {
	return MockEnvFull(EnvConfig{
		Mode: mode,
		Name: name,
	})
}

// MockEnvFull sets up plugin environment variables with full configuration.
// Returns a cleanup function that restores the original environment.
func MockEnvFull(cfg EnvConfig) func() {
	// Save original values
	origVars := map[string]string{}
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
		if val, ok := os.LookupEnv(key); ok {
			origVars[key] = val
		}
	}

	// Set new values - errors are intentionally ignored for testing utilities
	_ = os.Setenv("SKY_PLUGIN", "1")
	if cfg.Mode != "" {
		_ = os.Setenv("SKY_PLUGIN_MODE", cfg.Mode)
	} else {
		_ = os.Unsetenv("SKY_PLUGIN_MODE")
	}
	if cfg.Name != "" {
		_ = os.Setenv("SKY_PLUGIN_NAME", cfg.Name)
	} else {
		_ = os.Unsetenv("SKY_PLUGIN_NAME")
	}
	if cfg.WorkspaceRoot != "" {
		_ = os.Setenv("SKY_WORKSPACE_ROOT", cfg.WorkspaceRoot)
	} else {
		_ = os.Unsetenv("SKY_WORKSPACE_ROOT")
	}
	if cfg.ConfigDir != "" {
		_ = os.Setenv("SKY_CONFIG_DIR", cfg.ConfigDir)
	} else {
		_ = os.Unsetenv("SKY_CONFIG_DIR")
	}
	if cfg.OutputFormat != "" {
		_ = os.Setenv("SKY_OUTPUT_FORMAT", cfg.OutputFormat)
	} else {
		_ = os.Unsetenv("SKY_OUTPUT_FORMAT")
	}
	if cfg.NoColor {
		_ = os.Setenv("SKY_NO_COLOR", "1")
	} else {
		_ = os.Unsetenv("SKY_NO_COLOR")
	}
	if cfg.Verbose > 0 {
		_ = os.Setenv("SKY_VERBOSE", string(rune('0'+cfg.Verbose)))
	} else {
		_ = os.Unsetenv("SKY_VERBOSE")
	}

	// Return cleanup function
	return func() {
		for _, key := range envVars {
			if orig, ok := origVars[key]; ok {
				_ = os.Setenv(key, orig)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
}

// ClearEnv removes all Sky plugin environment variables.
// Returns a cleanup function that restores the original environment.
func ClearEnv() func() {
	origVars := map[string]string{}
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
		if val, ok := os.LookupEnv(key); ok {
			origVars[key] = val
		}
		_ = os.Unsetenv(key)
	}

	return func() {
		for _, key := range envVars {
			if orig, ok := origVars[key]; ok {
				_ = os.Setenv(key, orig)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
}
