package classifier

import (
	"path/filepath"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// DefaultClassifier is a filename-based classifier that determines
// the dialect and file kind from file paths.
type DefaultClassifier struct{}

// NewDefaultClassifier creates a new default classifier.
func NewDefaultClassifier() *DefaultClassifier {
	return &DefaultClassifier{}
}

// Classify returns the classification for a file path based on its filename and extension.
// The path may be absolute or workspace-relative.
func (c *DefaultClassifier) Classify(path string) (Classification, error) {
	// Extract the base filename
	base := filepath.Base(path)

	// Check for exact filename matches first (BUILD, WORKSPACE, BUCK, etc.)
	switch base {
	case "BUILD", "BUILD.bazel":
		return Classification{
			Dialect:  "bazel",
			FileKind: filekind.KindBUILD,
		}, nil
	case "WORKSPACE", "WORKSPACE.bazel":
		return Classification{
			Dialect:  "bazel",
			FileKind: filekind.KindWORKSPACE,
		}, nil
	case "MODULE.bazel":
		return Classification{
			Dialect:  "bazel",
			FileKind: filekind.KindMODULE,
		}, nil
	case "BUCK":
		return Classification{
			Dialect:  "buck2",
			FileKind: filekind.KindBUCK,
		}, nil
	}

	// Check for extension-based matches
	ext := filepath.Ext(base)
	switch ext {
	case ".bzl":
		// For now, treat all .bzl files as Bazel
		// In the future, we could detect Buck2 workspace context
		return Classification{
			Dialect:  "bazel",
			FileKind: filekind.KindBzl,
		}, nil
	case ".star":
		return Classification{
			Dialect:  "starlark",
			FileKind: filekind.KindStarlark,
		}, nil
	case ".skyi":
		return Classification{
			Dialect:  "starlark",
			FileKind: filekind.KindSkyI,
		}, nil
	}

	// Default to unknown file with starlark dialect
	return Classification{
		Dialect:  "starlark",
		FileKind: filekind.KindUnknown,
	}, nil
}

// SupportsDialect returns true if this classifier handles the named dialect.
func (c *DefaultClassifier) SupportsDialect(dialect string) bool {
	switch dialect {
	case "bazel", "buck2", "starlark":
		return true
	default:
		return false
	}
}
