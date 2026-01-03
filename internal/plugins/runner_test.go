package plugins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParsePluginType(t *testing.T) {
	cases := []struct {
		input    string
		expected PluginType
		ok       bool
	}{
		{input: "", expected: TypeExecutable, ok: true},
		{input: "exe", expected: TypeExecutable, ok: true},
		{input: "wasm", expected: TypeWasm, ok: true},
		{input: "bad", expected: "", ok: false},
	}

	for _, tc := range cases {
		got, err := ParsePluginType(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("expected %q to parse: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("expected %q to fail", tc.input)
		}
		if tc.ok && got != tc.expected {
			t.Fatalf("expected %q to parse as %q, got %q", tc.input, tc.expected, got)
		}
	}
}

func TestDetectPluginType(t *testing.T) {
	cases := []struct {
		input    string
		expected PluginType
	}{
		{input: "plugin.wasm", expected: TypeWasm},
		{input: "plugin", expected: TypeExecutable},
		{input: "https://example.com/plug.wasm", expected: TypeWasm},
		{input: "https://example.com/plug", expected: TypeExecutable},
	}

	for _, tc := range cases {
		got := DetectPluginType(tc.input)
		if got != tc.expected {
			t.Fatalf("expected %q to detect %q, got %q", tc.input, tc.expected, got)
		}
	}
}

func TestExecRunnerMetadataAndRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script plugins are not supported on windows")
	}

	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "demo-plugin")
	script := strings.Join([]string{
		"#!/bin/sh",
		"if [ \"$SKY_PLUGIN_MODE\" = \"metadata\" ]; then",
		"  echo '{\"api_version\":1,\"name\":\"demo\",\"version\":\"0.1.0\",\"summary\":\"Demo plugin\",\"commands\":[{\"name\":\"hello\",\"summary\":\"Say hi\"}]}'",
		"  exit 0",
		"fi",
		"echo \"args:$@\"",
		"exit 0",
	}, "\n")

	if err := os.WriteFile(pluginPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner := Runner{}
	plugin := Plugin{
		Name: "demo",
		Path: pluginPath,
		Type: TypeExecutable,
	}

	metadata, err := runner.Metadata(context.Background(), plugin)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if metadata.Name != "demo" {
		t.Fatalf("expected metadata name demo, got %q", metadata.Name)
	}
	if metadata.APIVersion != 1 {
		t.Fatalf("expected api version 1, got %d", metadata.APIVersion)
	}
	if metadata.Summary != "Demo plugin" {
		t.Fatalf("expected summary, got %q", metadata.Summary)
	}
	if len(metadata.Commands) != 1 || metadata.Commands[0].Name != "hello" {
		t.Fatalf("expected command metadata")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode, err := runner.Run(context.Background(), plugin, []string{"alpha", "beta"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "args:alpha beta") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
}
