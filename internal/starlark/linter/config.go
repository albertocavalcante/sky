package linter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the skylint configuration file structure.
type Config struct {
	// Enable is a list of rules or categories to enable (e.g., ["all"], ["correctness"])
	Enable []string `json:"enable,omitempty"`

	// Disable is a list of rules or patterns to disable (e.g., ["native-*"])
	Disable []string `json:"disable,omitempty"`

	// WarningsAsErrors treats all warnings as errors
	WarningsAsErrors bool `json:"warnings_as_errors,omitempty"`

	// Rules contains per-rule configuration overrides
	Rules map[string]RuleConfigOverride `json:"rules,omitempty"`
}

// RuleConfigOverride allows overriding rule-specific settings.
type RuleConfigOverride struct {
	// Severity overrides the default severity for this rule
	Severity string `json:"severity,omitempty"`

	// Options contains rule-specific configuration options
	Options map[string]any `json:"options,omitempty"`
}

// LoadConfig loads the configuration file from the specified path.
// If path is empty, it searches for .skylint.json in the current directory and parent directories.
func LoadConfig(path string) (*Config, error) {
	configPath := path

	// If no path specified, search for .skylint.json
	if configPath == "" {
		found, err := findConfigFile()
		if err != nil {
			return nil, err
		}
		if found == "" {
			// No config file found, return default config
			return &Config{}, nil
		}
		configPath = found
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Parse the JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &config, nil
}

// findConfigFile searches for .skylint.json in the current directory and parent directories.
// Returns an empty string if no config file is found.
func findConfigFile() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	// Search up the directory tree
	for {
		configPath := filepath.Join(dir, ".skylint.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", nil
}

// ApplyToRegistry applies the configuration to a registry.
func (c *Config) ApplyToRegistry(registry *Registry) error {
	// Apply enable/disable settings
	if len(c.Enable) > 0 {
		registry.Enable(c.Enable...)
	}
	if len(c.Disable) > 0 {
		registry.Disable(c.Disable...)
	}

	// Apply per-rule configuration
	for ruleName, override := range c.Rules {
		// Check if rule exists
		if _, exists := registry.Rule(ruleName); !exists {
			return fmt.Errorf("unknown rule in config: %s", ruleName)
		}

		// Parse severity override
		var severity Severity
		if override.Severity != "" {
			sev, err := parseSeverity(override.Severity)
			if err != nil {
				return fmt.Errorf("invalid severity for rule %s: %w", ruleName, err)
			}
			severity = sev
		}

		// Set the config
		if err := registry.SetConfig(ruleName, RuleConfig{
			Severity: severity,
			Options:  override.Options,
		}); err != nil {
			return err
		}
	}

	return nil
}

// parseSeverity converts a string to a Severity value.
func parseSeverity(s string) (Severity, error) {
	switch s {
	case "error":
		return SeverityError, nil
	case "warning":
		return SeverityWarning, nil
	case "info":
		return SeverityInfo, nil
	case "hint":
		return SeverityHint, nil
	default:
		return 0, fmt.Errorf("unknown severity: %s (must be one of: error, warning, info, hint)", s)
	}
}
