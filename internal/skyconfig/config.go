// Package skyconfig provides unified configuration loading for sky tools.
//
// It supports two configuration formats:
//   - config.sky / sky.star: Dynamic Starlark configuration (dogfooding!)
//   - sky.toml: Simple, declarative TOML configuration
//
// The package provides automatic discovery of configuration files,
// walking up the directory tree from the current directory.
//
// Configuration can also be specified via:
//   - SKY_CONFIG environment variable
//   - --config flag on individual tools
package skyconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config file names in priority order.
const (
	// ConfigSky is the canonical Starlark config filename.
	ConfigSky = "config.sky"
	// ConfigStarLegacy is the legacy Starlark config filename (also supported).
	ConfigStarLegacy = "sky.star"
	// ConfigTOML is the TOML config filename.
	ConfigTOML = "sky.toml"
)

// EnvConfig is the environment variable for specifying config file path.
const EnvConfig = "SKY_CONFIG"

// ErrConflict is returned when multiple config files exist in the same directory.
var ErrConflict = errors.New("multiple config files found in the same directory; use only one")

// Config represents the unified sky configuration.
type Config struct {
	// Test contains test runner configuration.
	Test TestConfig `json:"test" toml:"test"`

	// Lint contains linter configuration (future use).
	Lint LintConfig `json:"lint" toml:"lint"`
}

// TestConfig contains test runner configuration.
type TestConfig struct {
	// Timeout is the per-test timeout (e.g., "30s", "1m").
	Timeout Duration `json:"timeout" toml:"timeout"`

	// Parallel controls parallelism: "auto", "1", or a specific number.
	Parallel string `json:"parallel" toml:"parallel"`

	// Prelude is a list of prelude files to load before tests.
	Prelude []string `json:"prelude" toml:"prelude"`

	// Prefix is the test function prefix (default: "test_").
	Prefix string `json:"prefix" toml:"prefix"`

	// FailFast stops on first test failure.
	FailFast bool `json:"fail_fast" toml:"fail_fast"`

	// Verbose enables verbose output.
	Verbose bool `json:"verbose" toml:"verbose"`

	// Coverage contains coverage configuration.
	Coverage CoverageConfig `json:"coverage" toml:"coverage"`
}

// CoverageConfig contains coverage-specific settings.
type CoverageConfig struct {
	// Enabled enables coverage collection.
	Enabled bool `json:"enabled" toml:"enabled"`

	// FailUnder fails if coverage is below this percentage.
	FailUnder float64 `json:"fail_under" toml:"fail_under"`

	// Output is the coverage output file path.
	Output string `json:"output" toml:"output"`
}

// LintConfig contains linter configuration (for future use).
type LintConfig struct {
	// Enable is a list of rules or categories to enable.
	Enable []string `json:"enable" toml:"enable"`

	// Disable is a list of rules or patterns to disable.
	Disable []string `json:"disable" toml:"disable"`

	// WarningsAsErrors treats warnings as errors.
	WarningsAsErrors bool `json:"warnings_as_errors" toml:"warnings_as_errors"`
}

// Duration wraps time.Duration for TOML/JSON string parsing.
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for Duration.
func (d *Duration) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		d.Duration = 0
		return nil
	}
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}
	d.Duration = dur
	return nil
}

// MarshalText implements encoding.TextMarshaler for Duration.
func (d Duration) MarshalText() ([]byte, error) {
	if d.Duration == 0 {
		return nil, nil
	}
	return []byte(d.Duration.String()), nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Test: TestConfig{
			Timeout:  Duration{30 * time.Second},
			Parallel: "",
			Prefix:   "test_",
		},
	}
}

// LoadConfig loads configuration from the specified path.
// The format is auto-detected based on file extension.
// Returns an error if the file doesn't exist or cannot be parsed.
func LoadConfig(path string) (*Config, error) {
	ext := filepath.Ext(path)
	switch ext {
	case ".toml":
		return LoadTOMLConfig(path)
	case ".sky", ".star":
		return LoadStarlarkConfig(path, DefaultStarlarkTimeout)
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s (expected .sky, .star, or .toml)", ext)
	}
}

// DiscoverConfig searches for a configuration file.
//
// Resolution order:
//  1. If SKY_CONFIG env var is set, use that path
//  2. Walk up from startDir looking for config files
//
// In each directory, config files are checked in this order:
//   - config.sky (canonical Starlark)
//   - sky.star (legacy Starlark)
//   - sky.toml (TOML)
//
// If multiple config files exist in the same directory, an error is returned.
// Returns the loaded config, the path to the config file, and any error.
// If no config is found, returns (DefaultConfig(), "", nil).
func DiscoverConfig(startDir string) (*Config, string, error) {
	// Check environment variable first
	if envPath := os.Getenv(EnvConfig); envPath != "" {
		cfg, err := LoadConfig(envPath)
		if err != nil {
			return nil, "", fmt.Errorf("loading config from %s: %w", EnvConfig, err)
		}
		return cfg, envPath, nil
	}

	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Make path absolute
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolving path: %w", err)
	}

	// Find git root to limit search
	gitRoot := findGitRoot(absDir)

	// Walk up the directory tree
	dir := absDir
	for {
		// Check for config files in this directory
		configPath, err := findConfigInDir(dir)
		if err != nil {
			return nil, "", err
		}

		if configPath != "" {
			cfg, err := LoadConfig(configPath)
			if err != nil {
				return nil, "", err
			}
			return cfg, configPath, nil
		}

		// Stop at git root
		if gitRoot != "" && dir == gitRoot {
			break
		}

		// Move to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// No config found, return defaults
	return DefaultConfig(), "", nil
}

// findConfigInDir looks for config files in a directory.
// Returns the path to the config file if exactly one is found.
// Returns an error if multiple config files exist.
// Returns ("", nil) if no config files exist.
func findConfigInDir(dir string) (string, error) {
	configSkyPath := filepath.Join(dir, ConfigSky)
	skyStarPath := filepath.Join(dir, ConfigStarLegacy)
	skyTomlPath := filepath.Join(dir, ConfigTOML)

	configSkyExists := fileExists(configSkyPath)
	skyStarExists := fileExists(skyStarPath)
	skyTomlExists := fileExists(skyTomlPath)

	// Count how many config files exist
	count := 0
	var found []string
	if configSkyExists {
		count++
		found = append(found, ConfigSky)
	}
	if skyStarExists {
		count++
		found = append(found, ConfigStarLegacy)
	}
	if skyTomlExists {
		count++
		found = append(found, ConfigTOML)
	}

	// Error if multiple exist
	if count > 1 {
		return "", fmt.Errorf("%w: found %s in %s", ErrConflict, strings.Join(found, ", "), dir)
	}

	// Return the one that exists (in priority order)
	if configSkyExists {
		return configSkyPath, nil
	}
	if skyStarExists {
		return skyStarPath, nil
	}
	if skyTomlExists {
		return skyTomlPath, nil
	}

	return "", nil
}

// fileExists returns true if the file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// findGitRoot finds the git repository root from a starting directory.
// Returns empty string if not in a git repository.
func findGitRoot(startDir string) string {
	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		if fileExists(gitPath) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root
		}
		dir = parent
	}
}

// Merge merges the other config into this one.
// Non-zero values from other override values in c.
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	// Merge test config
	if other.Test.Timeout.Duration != 0 {
		c.Test.Timeout = other.Test.Timeout
	}
	if other.Test.Parallel != "" {
		c.Test.Parallel = other.Test.Parallel
	}
	if len(other.Test.Prelude) > 0 {
		c.Test.Prelude = append(c.Test.Prelude, other.Test.Prelude...)
	}
	if other.Test.Prefix != "" {
		c.Test.Prefix = other.Test.Prefix
	}
	if other.Test.FailFast {
		c.Test.FailFast = true
	}
	if other.Test.Verbose {
		c.Test.Verbose = true
	}

	// Merge coverage config
	if other.Test.Coverage.Enabled {
		c.Test.Coverage.Enabled = true
	}
	if other.Test.Coverage.FailUnder != 0 {
		c.Test.Coverage.FailUnder = other.Test.Coverage.FailUnder
	}
	if other.Test.Coverage.Output != "" {
		c.Test.Coverage.Output = other.Test.Coverage.Output
	}

	// Merge lint config
	if len(other.Lint.Enable) > 0 {
		c.Lint.Enable = append(c.Lint.Enable, other.Lint.Enable...)
	}
	if len(other.Lint.Disable) > 0 {
		c.Lint.Disable = append(c.Lint.Disable, other.Lint.Disable...)
	}
	if other.Lint.WarningsAsErrors {
		c.Lint.WarningsAsErrors = true
	}
}
