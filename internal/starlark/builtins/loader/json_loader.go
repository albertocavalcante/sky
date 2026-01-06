// Package loader provides loaders for Starlark builtin definitions.
package loader

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// JSONProvider loads builtin definitions from JSON files.
type JSONProvider struct {
	// mu protects the cache
	mu sync.RWMutex

	// cache stores parsed builtins by dialect and file kind
	cache map[string]map[filekind.Kind]builtins.Builtins

	// dataFS holds the JSON data files (filesystem or mock in tests)
	dataFS fsReader
}

// NewJSONProvider creates a new JSON-based builtin provider.
// For now, uses runtime file reading. Will add embed support later.
func NewJSONProvider() *JSONProvider {
	// TODO: Add embed.FS support once JSON files are generated
	// For now, this will fail gracefully if files don't exist yet
	return &JSONProvider{
		cache:  make(map[string]map[filekind.Kind]builtins.Builtins),
		dataFS: &diskFS{baseDir: "internal/starlark/builtins/loader"},
	}
}

// newTestJSONProvider creates a JSON provider for testing with injectable data.
func newTestJSONProvider() *JSONProvider {
	return &JSONProvider{
		cache:  make(map[string]map[filekind.Kind]builtins.Builtins),
		dataFS: &embedFS{files: make(map[string][]byte)},
	}
}

// injectTestData injects test data into the provider (for testing only).
func (p *JSONProvider) injectTestData(filename string, data []byte) {
	if fs, ok := p.dataFS.(*embedFS); ok {
		fs.files[filename] = data
	}
}

// Builtins implements the Provider interface.
// Returns builtin definitions for the specified dialect and file kind.
func (p *JSONProvider) Builtins(dialect string, kind filekind.Kind) (builtins.Builtins, error) {
	// Check cache first
	p.mu.RLock()
	if dialectCache, ok := p.cache[dialect]; ok {
		if cached, ok := dialectCache[kind]; ok {
			p.mu.RUnlock()
			return cached, nil
		}
	}
	p.mu.RUnlock()

	// Determine the JSON filename
	filename := p.jsonFilename(dialect, kind)
	if filename == "" {
		return builtins.Builtins{}, fmt.Errorf("unsupported dialect %q or file kind %q", dialect, kind)
	}

	// Load the JSON file
	data, err := p.loadJSONData(filename)
	if err != nil {
		return builtins.Builtins{}, fmt.Errorf("failed to load JSON data for %s/%s: %w", dialect, kind, err)
	}

	// Parse the JSON file (direct unmarshal to our struct)
	var result builtins.Builtins
	if err := json.Unmarshal(data, &result); err != nil {
		return builtins.Builtins{}, fmt.Errorf("failed to parse JSON file %s: %w", filename, err)
	}

	// Cache the result
	p.mu.Lock()
	if p.cache[dialect] == nil {
		p.cache[dialect] = make(map[filekind.Kind]builtins.Builtins)
	}
	p.cache[dialect][kind] = result
	p.mu.Unlock()

	return result, nil
}

// SupportedDialects implements the Provider interface.
// Returns the list of dialects this provider supports.
func (p *JSONProvider) SupportedDialects() []string {
	return []string{"bazel", "buck2", "starlark"}
}

// jsonFilename maps a dialect and file kind to a JSON filename.
// Returns an empty string if the combination is not supported.
func (p *JSONProvider) jsonFilename(dialect string, kind filekind.Kind) string {
	// Normalize dialect name
	dialect = strings.ToLower(dialect)

	// Build the filename based on dialect and kind
	var basename string
	switch dialect {
	case "bazel":
		switch kind {
		case filekind.KindBUILD:
			basename = "bazel-build"
		case filekind.KindBzl:
			basename = "bazel-bzl"
		case filekind.KindWORKSPACE:
			basename = "bazel-workspace"
		case filekind.KindMODULE:
			basename = "bazel-module"
		case filekind.KindBzlmod:
			basename = "bazel-bzlmod"
		default:
			return ""
		}
	case "buck2":
		switch kind {
		case filekind.KindBUCK:
			basename = "buck2-buck"
		case filekind.KindBzlBuck:
			basename = "buck2-bzl"
		case filekind.KindBuckconfig:
			basename = "buck2-buckconfig"
		default:
			return ""
		}
	case "starlark":
		switch kind {
		case filekind.KindStarlark:
			basename = "starlark-core"
		case filekind.KindSkyI:
			basename = "starlark-skyi"
		default:
			return ""
		}
	default:
		return ""
	}

	return path.Join("data", "json", basename+".json")
}

// loadJSONData loads JSON data from the filesystem.
func (p *JSONProvider) loadJSONData(filename string) ([]byte, error) {
	data, err := p.dataFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("JSON data file not found: %s", filename)
	}
	return data, nil
}
