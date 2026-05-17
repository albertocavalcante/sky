// Package formatter provides Starlark file formatting.
//
// Formatting backends are pluggable via the Engine interface. The package
// ships two engines:
//
//   - Buildtools — the stable upstream bazelbuild/buildtools-based engine
//     (currently Default; what skyfmt has shipped to date)
//   - CST — the native Roslyn-style stack built on starlark-cst-go +
//     bazel-cst-go + starlark-format-go (opt-in via --engine=cst)
//
// Package-level Format/FormatFile/FormatFileWithKind delegate to Default.
// Callers that need a specific engine call its Format directly:
//
//	out, err := formatter.CST.Format(src, path, kind)
//
// or use FormatWith to thread an explicit engine through the file APIs:
//
//	res := formatter.FormatFileWith(formatter.CST, path)
//
// See engine.go for the contract and DIFFERENCES between engines.
package formatter

import (
	"os"

	"github.com/albertocavalcante/sky/internal/starlark/classifier"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Result represents the outcome of formatting a file.
type Result struct {
	// Path is the file path (empty for stdin).
	Path string
	// Original is the original content.
	Original []byte
	// Formatted is the formatted content.
	Formatted []byte
	// Err is any error that occurred during formatting.
	Err error
	// Engine is the name of the engine that produced Formatted. Empty when
	// Err is set before the engine ran.
	Engine string
}

// Changed returns true if the file content was changed by formatting.
func (r *Result) Changed() bool {
	if r.Err != nil {
		return false
	}
	if len(r.Original) != len(r.Formatted) {
		return true
	}
	for i := range r.Original {
		if r.Original[i] != r.Formatted[i] {
			return true
		}
	}
	return false
}

// Format formats src using the Default engine.
//
// Equivalent to formatter.Default.Format(src, path, kind). Provided for
// backward compatibility with existing callers.
func Format(src []byte, path string, kind filekind.Kind) ([]byte, error) {
	return Default.Format(src, path, kind)
}

// FormatWith formats src using the supplied engine.
func FormatWith(engine Engine, src []byte, path string, kind filekind.Kind) ([]byte, error) {
	return engine.Format(src, path, kind)
}

// FormatFile reads a file, formats it with Default, and returns the result.
// The file kind is auto-detected from the filename using the default
// classifier.
func FormatFile(path string) *Result {
	return FormatFileWithKind(path, "")
}

// FormatFileWithKind reads a file, formats it with Default using the
// specified kind, and returns the result. If kind is empty, it is
// auto-detected from the filename.
func FormatFileWithKind(path string, kind filekind.Kind) *Result {
	return FormatFileWith(Default, path, kind)
}

// FormatFileWith reads a file and formats it with the supplied engine.
// If kind is empty or KindUnknown, it is auto-detected from path.
func FormatFileWith(engine Engine, path string, kind filekind.Kind) *Result {
	result := &Result{Path: path, Engine: engine.Name()}

	src, err := os.ReadFile(path)
	if err != nil {
		result.Err = err
		return result
	}
	result.Original = src

	if kind == "" || kind == filekind.KindUnknown {
		kind = detectKind(path)
	}

	formatted, err := engine.Format(src, path, kind)
	if err != nil {
		result.Err = err
		return result
	}
	result.Formatted = formatted
	return result
}

// DetectKind uses the default classifier to detect the file kind from a
// path. Returns KindUnknown on any classification error.
//
// Exported so CLIs and tools can do kind-aware dispatch without
// duplicating the classifier wiring.
func DetectKind(path string) filekind.Kind {
	c := classifier.NewDefaultClassifier()
	classification, err := c.Classify(path)
	if err != nil {
		return filekind.KindUnknown
	}
	return classification.FileKind
}

// detectKind is the unexported alias kept for the package-internal call
// sites that pre-date DetectKind. New code should use DetectKind.
func detectKind(path string) filekind.Kind {
	return DetectKind(path)
}
