// Package dialect defines the Starlark dialect configuration used by the SKY toolchain.
package dialect

import (
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/typemode"
)

// Dialect represents a Starlark dialect configuration.
// Dialects define which language features are enabled and how files are processed.
type Dialect struct {
	// Name is the unique identifier for this dialect (e.g., "bazel", "buck2", "starlark").
	Name string `json:"name"`

	// Version is an optional version string for the dialect.
	Version string `json:"version,omitempty"`

	// Description provides a human-readable description of the dialect.
	Description string `json:"description,omitempty"`

	// Features controls which Starlark language features are enabled.
	Features Features `json:"features"`

	// FileKinds lists the file kinds this dialect handles.
	FileKinds []filekind.Kind `json:"file_kinds"`

	// TypeMode controls how type annotations are processed.
	TypeMode typemode.Mode `json:"type_mode"`
}

// Features controls which Starlark language features are enabled.
// Modeled after starlark-rust's Dialect struct.
type Features struct {
	// Statement features.

	// EnableDef allows def statements for function definitions.
	EnableDef bool `json:"enable_def"`
	// EnableLambda allows lambda expressions.
	EnableLambda bool `json:"enable_lambda"`
	// EnableLoad allows load statements for importing symbols.
	EnableLoad bool `json:"enable_load"`
	// EnableLoadAssign allows assigned loads: x = load(...).
	EnableLoadAssign bool `json:"enable_load_assign"`
	// EnableIf allows if statements (not just conditional expressions).
	EnableIf bool `json:"enable_if"`
	// EnableFor allows for statements.
	EnableFor bool `json:"enable_for"`
	// EnableWhile allows while statements (rare in Starlark).
	EnableWhile bool `json:"enable_while"`

	// Expression features.

	// EnableSetLiteral allows {1, 2, 3} set literals.
	EnableSetLiteral bool `json:"enable_set_literal"`
	// EnableFString allows f"..." f-strings.
	EnableFString bool `json:"enable_f_string"`
	// EnableRecursion allows recursive function calls.
	EnableRecursion bool `json:"enable_recursion"`

	// Type annotation features.

	// EnableTypeComments enables parsing of # type: comments.
	EnableTypeComments bool `json:"enable_type_comments"`
	// EnableAnnotations enables parsing of PEP-484 style annotations.
	EnableAnnotations bool `json:"enable_annotations"`

	// Strictness features.

	// RequireTopLevel requires all defs to be at the top level.
	RequireTopLevel bool `json:"require_top_level"`
	// StrictString enables strict string comparisons.
	StrictString bool `json:"strict_string"`
}

// Standard returns a dialect with standard Starlark features enabled.
// This is suitable for generic .star files.
func Standard() Dialect {
	return Dialect{
		Name:        "starlark",
		Description: "Standard Starlark dialect",
		Features: Features{
			EnableDef:    true,
			EnableLambda: true,
			EnableLoad:   true,
			EnableIf:     true,
			EnableFor:    true,
		},
		FileKinds: []filekind.Kind{filekind.KindStarlark},
		TypeMode:  typemode.Disabled,
	}
}

// Bazel returns a dialect configured for Bazel BUILD and .bzl files.
func Bazel() Dialect {
	return Dialect{
		Name:        "bazel",
		Description: "Bazel Starlark dialect",
		Features: Features{
			EnableDef:          true,
			EnableLambda:       true,
			EnableLoad:         true,
			EnableIf:           true,
			EnableFor:          true,
			EnableTypeComments: true,
			RequireTopLevel:    true,
		},
		FileKinds: []filekind.Kind{
			filekind.KindBUILD,
			filekind.KindBzl,
			filekind.KindWORKSPACE,
			filekind.KindMODULE,
			filekind.KindBzlmod,
		},
		TypeMode: typemode.ParseOnly,
	}
}

// Buck2 returns a dialect configured for Buck2 BUCK and .bzl files.
func Buck2() Dialect {
	return Dialect{
		Name:        "buck2",
		Description: "Buck2 Starlark dialect",
		Features: Features{
			EnableDef:          true,
			EnableLambda:       true,
			EnableLoad:         true,
			EnableIf:           true,
			EnableFor:          true,
			EnableTypeComments: true,
		},
		FileKinds: []filekind.Kind{
			filekind.KindBUCK,
			filekind.KindBzlBuck,
		},
		TypeMode: typemode.ParseOnly,
	}
}
