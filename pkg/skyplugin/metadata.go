package skyplugin

import (
	"encoding/json"
	"os"
)

// Metadata describes a plugin's capabilities for discovery.
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

// HandleMetadata writes the metadata as JSON to stdout and exits.
// This should be called when IsMetadataMode() returns true.
func HandleMetadata(m Metadata) {
	if m.APIVersion == 0 {
		m.APIVersion = 1
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(m); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
