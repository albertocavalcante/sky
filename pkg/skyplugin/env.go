package skyplugin

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Environment variable names used by the Sky plugin protocol.
const (
	EnvPlugin        = "SKY_PLUGIN"
	EnvPluginMode    = "SKY_PLUGIN_MODE"
	EnvPluginName    = "SKY_PLUGIN_NAME"
	EnvWorkspaceRoot = "SKY_WORKSPACE_ROOT"
	EnvConfigDir     = "SKY_CONFIG_DIR"
	EnvOutputFormat  = "SKY_OUTPUT_FORMAT"
	EnvNoColor       = "SKY_NO_COLOR"
	EnvVerbose       = "SKY_VERBOSE"
)

// IsPlugin returns true if the current process is running as a Sky plugin.
func IsPlugin() bool {
	return os.Getenv(EnvPlugin) == "1"
}

// IsMetadataMode returns true if the plugin should output metadata and exit.
func IsMetadataMode() bool {
	return os.Getenv(EnvPluginMode) == "metadata"
}

// PluginName returns the name of the current plugin.
func PluginName() string {
	return os.Getenv(EnvPluginName)
}

// WorkspaceRoot returns the workspace root directory.
// If SKY_WORKSPACE_ROOT is not set, it falls back to the current working directory.
func WorkspaceRoot() string {
	if root := os.Getenv(EnvWorkspaceRoot); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// ConfigDir returns the Sky configuration directory.
// If SKY_CONFIG_DIR is not set, it returns the platform-appropriate default:
//   - Linux/macOS: ~/.config/sky
//   - Windows: %APPDATA%\sky
func ConfigDir() string {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return dir
	}
	return defaultConfigDir()
}

// OutputFormat returns the requested output format.
// Returns "text" by default if SKY_OUTPUT_FORMAT is not set.
func OutputFormat() string {
	if format := os.Getenv(EnvOutputFormat); format != "" {
		return format
	}
	return "text"
}

// IsJSONOutput returns true if JSON output is requested.
func IsJSONOutput() bool {
	return OutputFormat() == "json"
}

// NoColor returns true if color output should be disabled.
// This respects both SKY_NO_COLOR and the NO_COLOR standard (https://no-color.org).
func NoColor() bool {
	if os.Getenv(EnvNoColor) == "1" {
		return true
	}
	// Also respect the NO_COLOR standard
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return true
	}
	return false
}

// Verbose returns the verbosity level (0-3).
// Returns 0 if SKY_VERBOSE is not set or invalid.
func Verbose() int {
	v := os.Getenv(EnvVerbose)
	if v == "" {
		return 0
	}
	level, err := strconv.Atoi(v)
	if err != nil || level < 0 {
		return 0
	}
	if level > 3 {
		return 3
	}
	return level
}

// defaultConfigDir returns the platform-appropriate config directory.
func defaultConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "sky")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "sky")
	default:
		// Linux, macOS, etc.
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "sky")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "sky")
	}
}
