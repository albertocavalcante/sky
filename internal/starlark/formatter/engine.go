package formatter

import (
	"errors"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Engine is a pluggable formatting backend. Different implementations let
// callers compare outputs (compare mode) and provide a clean migration
// path when adopting a new formatter.
//
// Engines are stateless and safe for concurrent use. Implementations
// should be value types so consumers can pass them around cheaply.
type Engine interface {
	// Name returns the engine identifier used by --engine flags and logs.
	// Examples: "buildtools", "cst".
	Name() string

	// Format formats src using this engine's rules. path is informational
	// (used in diagnostics); kind selects the file-type-specific parsing
	// and formatting policy.
	Format(src []byte, path string, kind filekind.Kind) ([]byte, error)
}

// ErrEngineDoesNotSupport is returned by Format when an engine has no
// implementation for the given file kind. Callers handling this error
// usually fall back to a different engine.
var ErrEngineDoesNotSupport = errors.New("formatter: engine does not support this file kind")

// Default is the engine selected when no explicit engine is requested.
//
// Currently Buildtools (upstream-stable). Will flip to CST — partially
// or per-kind — once CST gains the canonicalization passes that
// build.Format performs on NON-canonical input. The 98.7% corpus
// validation measures already-canonical files (real-world BUILD files
// that have been buildifier'd in CI); on non-canonical user input,
// build.Format does work CST doesn't yet do:
//
//   - Line reflow (single-line calls → multi-line when long; collapse
//     short multi-lines)
//   - Trailing comma insertion on multi-line collections
//   - Quote-style normalization (' → ")
//   - Some inter-token whitespace normalization
//
// So flipping Default = CST today would be a user-visible regression
// for the common case (running skyfmt on un-formatted code). Tracked in
// DIFFERENCES.md and proposal-07-determinism-roadmap.md.
//
// Override by passing an explicit engine via formatter.FormatWith or the
// skyfmt -engine flag.
var Default Engine = Buildtools

// Engines returns the engines registered in this build, keyed by Name().
// Useful for CLIs that surface --engine help.
func Engines() map[string]Engine {
	return map[string]Engine{
		Buildtools.Name(): Buildtools,
		CST.Name():        CST,
	}
}
