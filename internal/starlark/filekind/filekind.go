// Package filekind defines the types of Starlark files recognized by the SKY toolchain.
package filekind

import "path/filepath"

// Kind represents the type of Starlark file.
type Kind string

const (
	// Generic Starlark file kinds.

	// KindStarlark is a generic .star file.
	KindStarlark Kind = "starlark"
	// KindSkyI is a type stub file (.skyi).
	KindSkyI Kind = "skyi"

	// Bazel file kinds.

	// KindBUILD represents BUILD and BUILD.bazel files.
	KindBUILD Kind = "BUILD"
	// KindBzl represents .bzl extension files.
	KindBzl Kind = "bzl"
	// KindWORKSPACE represents WORKSPACE and WORKSPACE.bazel files.
	KindWORKSPACE Kind = "WORKSPACE"
	// KindMODULE represents MODULE.bazel files (bzlmod).
	KindMODULE Kind = "MODULE"
	// KindBzlmod represents .bzl files used in bzlmod extensions.
	KindBzlmod Kind = "bzlmod"

	// Buck2 file kinds.

	// KindBUCK represents BUCK files.
	KindBUCK Kind = "BUCK"
	// KindBzlBuck represents .bzl files for Buck2.
	KindBzlBuck Kind = "bzl_buck"
	// KindBuckconfig represents .buckconfig files.
	KindBuckconfig Kind = "buckconfig"

	// KindUnknown indicates an unrecognized file type.
	KindUnknown Kind = "unknown"
)

// String returns the string representation of the Kind.
func (k Kind) String() string {
	return string(k)
}

// IsTopLevel returns true if this kind represents a top-level build file
// (e.g., BUILD, WORKSPACE, MODULE.bazel, BUCK).
func (k Kind) IsTopLevel() bool {
	switch k {
	case KindBUILD, KindWORKSPACE, KindMODULE, KindBUCK:
		return true
	}
	return false
}

// IsExtension returns true if this kind represents an extension/library file
// (e.g., .bzl, .star files).
func (k Kind) IsExtension() bool {
	switch k {
	case KindBzl, KindBzlBuck, KindBzlmod, KindStarlark:
		return true
	}
	return false
}

// IsBazel returns true if this kind is a Bazel-specific file type.
func (k Kind) IsBazel() bool {
	switch k {
	case KindBUILD, KindBzl, KindWORKSPACE, KindMODULE, KindBzlmod:
		return true
	}
	return false
}

// IsBuck returns true if this kind is a Buck2-specific file type.
func (k Kind) IsBuck() bool {
	switch k {
	case KindBUCK, KindBzlBuck, KindBuckconfig:
		return true
	}
	return false
}

// AllKinds returns all defined file kinds.
func AllKinds() []Kind {
	return []Kind{
		KindStarlark,
		KindSkyI,
		KindBUILD,
		KindBzl,
		KindWORKSPACE,
		KindMODULE,
		KindBzlmod,
		KindBUCK,
		KindBzlBuck,
		KindBuckconfig,
		KindUnknown,
	}
}

// IsStarlarkFile returns true if the filename is a recognized Starlark file.
// Supports files from: Bazel, Buck2, Pants, Please, Tilt, Copybara, Skycfg,
// Kurtosis, Drone CI, Isopod, Cirrus CI, and generic Starlark.
func IsStarlarkFile(name string) bool {
	// Exact filename matches (no extension)
	switch name {
	case "BUILD", "BUILD.bazel", "WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel", // Bazel
		"BUCK",     // Buck2
		"Tiltfile": // Tilt (Kubernetes dev)
		return true
	}
	// Extension-based matches
	ext := filepath.Ext(name)
	switch ext {
	case ".bzl", // Bazel/Buck2 extensions
		".bxl",      // Buck2 BXL (Buck2 Extension Language)
		".star",     // Generic Starlark (Kurtosis, Drone CI, Cirrus CI, Qri, etc.)
		".starlark", // Full extension variant
		".sky",      // Skycfg, Copybara (.bara.sky)
		".skyi",     // Type stubs
		".axl",      // Starlark config files
		".ipd",      // Isopod (Kubernetes)
		".plz",      // Please Build
		".pconf",    // Protoconf config
		".pinc",     // Protoconf include
		".mpconf":   // Protoconf mutable config
		return true
	}
	return false
}
