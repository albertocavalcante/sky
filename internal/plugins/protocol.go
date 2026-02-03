package plugins

import (
	"fmt"
	"strings"
)

const (
	// Core protocol environment variables (v1.0)
	EnvPlugin     = "SKY_PLUGIN"
	EnvPluginMode = "SKY_PLUGIN_MODE"
	EnvPluginName = "SKY_PLUGIN_NAME"

	// Extended environment variables (v1.1)
	EnvWorkspaceRoot = "SKY_WORKSPACE_ROOT"
	EnvConfigDir     = "SKY_CONFIG_DIR"
	EnvOutputFormat  = "SKY_OUTPUT_FORMAT"
	EnvNoColor       = "SKY_NO_COLOR"
	EnvVerbose       = "SKY_VERBOSE"
)

const (
	ModeExec     = "exec"
	ModeMetadata = "metadata"
)

const MetadataAPIVersion = 1

// PluginType describes how a plugin is executed.
type PluginType string

const (
	TypeExecutable PluginType = "exe"
	TypeWasm       PluginType = "wasm"
)

// Metadata describes a plugin's capabilities.
type Metadata struct {
	APIVersion int               `json:"api_version"`
	Name       string            `json:"name"`
	Version    string            `json:"version,omitempty"`
	Summary    string            `json:"summary,omitempty"`
	Commands   []CommandMetadata `json:"commands,omitempty"`
}

// CommandMetadata describes a single plugin command.
type CommandMetadata struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
}

// ParsePluginType normalizes user input into a PluginType.
func ParsePluginType(input string) (PluginType, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "", string(TypeExecutable), "bin", "binary":
		return TypeExecutable, nil
	case string(TypeWasm):
		return TypeWasm, nil
	default:
		return "", fmt.Errorf("unknown plugin type %q", input)
	}
}

// DetectPluginType infers the plugin type from a path or URL.
func DetectPluginType(source string) PluginType {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(source)), ".wasm") {
		return TypeWasm
	}
	return TypeExecutable
}

// EffectiveType returns the default plugin type when missing.
func (p Plugin) EffectiveType() PluginType {
	if p.Type == "" {
		return TypeExecutable
	}
	return p.Type
}
