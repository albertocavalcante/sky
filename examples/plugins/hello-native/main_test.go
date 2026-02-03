package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMetadataOutput(t *testing.T) {
	// Save and restore stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	// Set environment for metadata mode
	if err := os.Setenv("SKY_PLUGIN", "1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("SKY_PLUGIN_MODE", "metadata"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Unsetenv("SKY_PLUGIN")
		_ = os.Unsetenv("SKY_PLUGIN_MODE")
	}()

	outputMetadata()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	// Verify JSON structure
	var metadata map[string]any
	if err := json.Unmarshal([]byte(output), &metadata); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if metadata["api_version"].(float64) != 1 {
		t.Errorf("expected api_version 1, got %v", metadata["api_version"])
	}

	if metadata["name"].(string) != pluginName {
		t.Errorf("expected name %q, got %q", pluginName, metadata["name"])
	}

	if metadata["version"].(string) != pluginVersion {
		t.Errorf("expected version %q, got %q", pluginVersion, metadata["version"])
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		wantCode int
		wantOut  string
	}{
		{
			name:     "default greeting",
			args:     []string{},
			wantCode: 0,
			wantOut:  "Hello, World!",
		},
		{
			name:     "custom name",
			args:     []string{"-name", "Sky"},
			wantCode: 0,
			wantOut:  "Hello, Sky!",
		},
		{
			name:     "version flag",
			args:     []string{"-version"},
			wantCode: 0,
			wantOut:  pluginVersion,
		},
		{
			name:     "env flag",
			args:     []string{"-env"},
			wantCode: 0,
			wantOut:  "SKY_PLUGIN=",
		},
		{
			name: "with workspace root",
			args: []string{},
			env: map[string]string{
				"SKY_WORKSPACE_ROOT": "/test/workspace",
			},
			wantCode: 0,
			wantOut:  "Workspace: /test/workspace",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment
			for k, v := range tc.env {
				if err := os.Setenv(k, v); err != nil {
					t.Fatal(err)
				}
				defer func(key string) {
					_ = os.Unsetenv(key)
				}(k)
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = w

			code := run(tc.args)

			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			os.Stdout = oldStdout

			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatal(err)
			}
			output := buf.String()

			if code != tc.wantCode {
				t.Errorf("run() = %d, want %d", code, tc.wantCode)
			}

			if !strings.Contains(output, tc.wantOut) {
				t.Errorf("output %q does not contain %q", output, tc.wantOut)
			}
		})
	}
}
