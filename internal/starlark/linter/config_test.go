package linter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestLoadConfig_ValidFile verifies loading a valid config file.
func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	configJSON := `{
  "enable": ["all"],
  "disable": ["test-rule"],
  "warnings_as_errors": true,
  "rules": {
    "some-rule": {
      "severity": "error",
      "options": {
        "key": "value"
      }
    }
  }
}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(config.Enable) != 1 || config.Enable[0] != "all" {
		t.Errorf("Enable: got %v, want [all]", config.Enable)
	}
	if len(config.Disable) != 1 || config.Disable[0] != "test-rule" {
		t.Errorf("Disable: got %v, want [test-rule]", config.Disable)
	}
	if !config.WarningsAsErrors {
		t.Error("WarningsAsErrors should be true")
	}
	if _, exists := config.Rules["some-rule"]; !exists {
		t.Error("Rules should contain some-rule")
	}
}

// TestLoadConfig_MissingFile verifies error handling for missing config file.
func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("Expected error for missing config file, got nil")
	}
}

// TestLoadConfig_EmptyPath verifies default behavior with empty path.
func TestLoadConfig_EmptyPath(t *testing.T) {
	// Change to a temp directory that has no .skylint.json
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig with empty path failed: %v", err)
	}

	// Should return default config
	if config == nil {
		t.Error("Expected default config, got nil")
	}
}

// TestLoadConfig_SearchParentDirs verifies config file search in parent directories.
func TestLoadConfig_SearchParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".skylint.json")
	configJSON := `{"enable": ["all"]}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("Failed to change to sub directory: %v", err)
	}

	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(config.Enable) != 1 || config.Enable[0] != "all" {
		t.Errorf("Config from parent dir not loaded correctly")
	}
}

// TestLoadConfig_InvalidJSON verifies error handling for invalid JSON.
func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestLoadConfig_EmptyFile verifies handling of empty config file.
func TestLoadConfig_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.json")

	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed for empty config: %v", err)
	}

	if config.Enable != nil && len(config.Enable) > 0 {
		t.Errorf("Enable should be empty, got %v", config.Enable)
	}
	if config.WarningsAsErrors {
		t.Error("WarningsAsErrors should be false")
	}
}

// TestLoadConfig_UnknownFields verifies that unknown fields are ignored.
func TestLoadConfig_UnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unknown-fields.json")

	configJSON := `{
  "enable": ["all"],
  "unknown_field": "should be ignored",
  "another_unknown": 123
}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig should not fail with unknown fields: %v", err)
	}

	if len(config.Enable) != 1 || config.Enable[0] != "all" {
		t.Errorf("Known fields should still be parsed correctly")
	}
}

// TestLoadConfig_AllFields verifies all config fields are parsed correctly.
func TestLoadConfig_AllFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "full-config.json")

	configJSON := `{
  "enable": ["correctness", "style"],
  "disable": ["native-*", "deprecated-*"],
  "warnings_as_errors": true,
  "rules": {
    "load-on-top": {
      "severity": "warning",
      "options": {
        "allow_after_docstring": true
      }
    },
    "unused-variable": {
      "severity": "error"
    }
  }
}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify Enable
	expectedEnable := []string{"correctness", "style"}
	if diff := cmp.Diff(expectedEnable, config.Enable); diff != "" {
		t.Errorf("Enable mismatch (-want +got):\n%s", diff)
	}

	// Verify Disable
	expectedDisable := []string{"native-*", "deprecated-*"}
	if diff := cmp.Diff(expectedDisable, config.Disable); diff != "" {
		t.Errorf("Disable mismatch (-want +got):\n%s", diff)
	}

	// Verify WarningsAsErrors
	if !config.WarningsAsErrors {
		t.Error("WarningsAsErrors should be true")
	}

	// Verify Rules
	if len(config.Rules) != 2 {
		t.Errorf("Expected 2 rule configs, got %d", len(config.Rules))
	}

	loadRule, exists := config.Rules["load-on-top"]
	if !exists {
		t.Error("load-on-top rule config not found")
	} else {
		if loadRule.Severity != "warning" {
			t.Errorf("load-on-top severity: got %s, want warning", loadRule.Severity)
		}
		if allow, ok := loadRule.Options["allow_after_docstring"].(bool); !ok || !allow {
			t.Error("load-on-top options not parsed correctly")
		}
	}

	unusedVar, exists := config.Rules["unused-variable"]
	if !exists {
		t.Error("unused-variable rule config not found")
	} else {
		if unusedVar.Severity != "error" {
			t.Errorf("unused-variable severity: got %s, want error", unusedVar.Severity)
		}
	}
}

// TestParseSeverity verifies severity string parsing.
func TestParseSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Severity
		wantErr  bool
	}{
		{"error", "error", SeverityError, false},
		{"warning", "warning", SeverityWarning, false},
		{"info", "info", SeverityInfo, false},
		{"hint", "hint", SeverityHint, false},
		{"unknown", "unknown", 0, true},
		{"empty", "", 0, true},
		{"uppercase", "ERROR", 0, true}, // Case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSeverity(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Got severity %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

// TestConfig_ApplyToRegistry is a placeholder for integration testing with Registry.
// The full test would require a mock Registry or actual Registry instance.
func TestConfig_ApplyToRegistry(t *testing.T) {
	// This would require the Registry type to be available.
	// For now, we test the logic through the main binary's integration tests.
	t.Skip("Requires Registry implementation - tested via integration tests")
}
