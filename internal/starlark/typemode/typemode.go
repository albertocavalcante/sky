// Package typemode defines how type information is processed by the SKY toolchain.
package typemode

import "fmt"

// Mode controls how type information is processed.
type Mode string

const (
	// Disabled ignores all type information.
	// Type annotations are treated as syntax errors if the parser is strict,
	// or simply ignored if tolerant.
	Disabled Mode = "disabled"

	// ParseOnly parses type annotations and comments but does not check them.
	// Useful for IDE features (completion, hover) without full type checking overhead.
	ParseOnly Mode = "parse_only"

	// Enabled performs full static type checking.
	// Type mismatches are reported as errors.
	Enabled Mode = "enabled"
)

// String returns the string representation of the Mode.
func (m Mode) String() string {
	return string(m)
}

// Parse parses a string into a Mode.
// An empty string defaults to Disabled.
func Parse(s string) (Mode, error) {
	switch s {
	case "", "disabled":
		return Disabled, nil
	case "parse_only", "parse-only", "parseonly":
		return ParseOnly, nil
	case "enabled":
		return Enabled, nil
	default:
		return "", fmt.Errorf("unknown type mode: %q (valid: disabled, parse_only, enabled)", s)
	}
}

// IsEnabled returns true if this mode enables any type processing.
func (m Mode) IsEnabled() bool {
	return m == ParseOnly || m == Enabled
}

// ShouldCheck returns true if this mode enables full type checking.
func (m Mode) ShouldCheck() bool {
	return m == Enabled
}

// AllModes returns all defined type modes.
func AllModes() []Mode {
	return []Mode{Disabled, ParseOnly, Enabled}
}
