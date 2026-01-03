package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Runner executes plugins based on their type.
type Runner struct{}

// Metadata fetches plugin metadata using the metadata mode.
func (Runner) Metadata(ctx context.Context, plugin Plugin) (Metadata, error) {
	if plugin.Path == "" {
		return Metadata{}, fmt.Errorf("plugin %q has no path", plugin.Name)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode, err := runWithMode(ctx, plugin, ModeMetadata, nil, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		return Metadata{}, err
	}
	if exitCode != 0 {
		return Metadata{}, fmt.Errorf("plugin %q exited with %d", plugin.Name, exitCode)
	}

	var metadata Metadata
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &metadata); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return Metadata{}, fmt.Errorf("plugin %q metadata parse failed: %v", plugin.Name, message)
	}

	if metadata.APIVersion != MetadataAPIVersion {
		return Metadata{}, fmt.Errorf("plugin %q metadata api_version %d is unsupported", plugin.Name, metadata.APIVersion)
	}
	if metadata.Name != "" && metadata.Name != plugin.Name {
		return Metadata{}, fmt.Errorf("plugin %q metadata name mismatch (%s)", plugin.Name, metadata.Name)
	}
	if metadata.Name == "" {
		metadata.Name = plugin.Name
	}

	return metadata, nil
}

// Run executes a plugin with the provided args.
func (Runner) Run(ctx context.Context, plugin Plugin, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if plugin.Path == "" {
		return 1, fmt.Errorf("plugin %q has no path", plugin.Name)
	}
	return runWithMode(ctx, plugin, ModeExec, args, stdin, stdout, stderr)
}

func runWithMode(ctx context.Context, plugin Plugin, mode string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	switch plugin.EffectiveType() {
	case TypeExecutable:
		return runExec(ctx, plugin, mode, args, stdin, stdout, stderr)
	case TypeWasm:
		return runWasm(ctx, plugin, mode, args, stdin, stdout, stderr)
	default:
		return 1, fmt.Errorf("unsupported plugin type %q", plugin.Type)
	}
}
