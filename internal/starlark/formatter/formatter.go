// Package formatter provides Starlark file formatting using buildtools.
package formatter

import (
	"os"

	"github.com/bazelbuild/buildtools/build"

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

// Format formats Starlark source code according to the specified file kind.
// If kind is empty or KindUnknown, TypeDefault formatting is used.
func Format(src []byte, path string, kind filekind.Kind) ([]byte, error) {
	f, err := parse(src, path, kind)
	if err != nil {
		return nil, err
	}
	return build.Format(f), nil
}

// FormatFile reads a file, formats it, and returns the result.
// The file kind is auto-detected from the filename using the default classifier.
func FormatFile(path string) *Result {
	return FormatFileWithKind(path, "")
}

// FormatFileWithKind reads a file, formats it with the specified kind, and returns the result.
// If kind is empty, it is auto-detected from the filename.
func FormatFileWithKind(path string, kind filekind.Kind) *Result {
	result := &Result{Path: path}

	src, err := os.ReadFile(path)
	if err != nil {
		result.Err = err
		return result
	}
	result.Original = src

	// Auto-detect kind if not specified
	if kind == "" || kind == filekind.KindUnknown {
		kind = detectKind(path)
	}

	formatted, err := Format(src, path, kind)
	if err != nil {
		result.Err = err
		return result
	}
	result.Formatted = formatted

	return result
}

// parse parses source code using the appropriate parser based on file kind.
func parse(src []byte, path string, kind filekind.Kind) (*build.File, error) {
	switch kind {
	case filekind.KindBUILD, filekind.KindBUCK:
		return build.ParseBuild(path, src)
	case filekind.KindWORKSPACE:
		return build.ParseWorkspace(path, src)
	case filekind.KindMODULE:
		return build.ParseModule(path, src)
	case filekind.KindBzl, filekind.KindBzlmod, filekind.KindBzlBuck:
		return build.ParseBzl(path, src)
	default:
		// KindStarlark, KindSkyI, KindUnknown, or any other
		return build.ParseDefault(path, src)
	}
}

// detectKind uses the default classifier to detect the file kind from a path.
func detectKind(path string) filekind.Kind {
	c := classifier.NewDefaultClassifier()
	classification, err := c.Classify(path)
	if err != nil {
		return filekind.KindUnknown
	}
	return classification.FileKind
}
