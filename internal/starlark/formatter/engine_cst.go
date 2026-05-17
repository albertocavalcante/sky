package formatter

import (
	"fmt"

	bzlbuildifier "github.com/albertocavalcante/bazel-cst-go/format/buildifier"
	"github.com/albertocavalcante/starlark-cst-go/parser"
	neutral "github.com/albertocavalcante/starlark-format-go"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// CST is the formatter built on the native Roslyn-style starlark-cst-go +
// bazel-cst-go + starlark-format-go stack.
//
// HEURISTIC — file-kind dispatch table.
//
//	What it does: maps sky's filekind.Kind to one of three handlers:
//	  - bazel-cst-go/format/buildifier (Bazel-flavored)
//	  - starlark-format-go/Neutral (spec-only)
//	  - ErrEngineDoesNotSupport (Buck2, until buck2-cst-go ships)
//
//	Why it is a heuristic: sky's filekind.Kind enum mixes flavors
//	("BUILD" is Bazel's BUILD; "BUCK" is Buck2's; "Bzl" could be
//	either depending on context). The mapping from filekind to
//	"which formatter dialect" is a CHOICE; there's no spec answer
//	for files like *.bzl that both Bazel and Buck2 use.
//
//	Why this particular dispatch: in sky's usage, *.bzl files in a
//	Bazel-dominant codebase get Bazel formatting. Buck2 .bzl files
//	(KindBzlBuck) are explicitly classified by sky and we route those
//	to ErrEngineDoesNotSupport until a buck2-cst-go formatter exists.
//
//	Why deferred (no per-path inference): we could try harder to
//	detect Buck2-ness from file content (presence of `prelude_rules`
//	calls, etc.). Not worth the complexity for v0; sky's classifier
//	already does the Bazel-vs-Buck2 distinction by file location.
//
//	Why acceptable: ErrEngineDoesNotSupport is a clean signal; the
//	caller (compare mode tracks it separately, the format path could
//	add fallback). Misclassification produces "wrong formatter"
//	output, not corrupt code — Buck2 .bzl is still valid Starlark.
//
//	When to revisit: when buck2-cst-go ships; or when sky adds
//	file-content-based dialect detection.
//
//	Source-of-truth pointer: engine_test.go pins each dispatch case
//	(BUILD → buildifier, .star → neutral, BUCK → unsupported).
//
// See bazel-cst-go/format/buildifier/DIFFERENCES.md for what the Bazel
// path does NOT yet do relative to upstream buildifier.
var CST Engine = cstEngine{}

type cstEngine struct{}

func (cstEngine) Name() string { return "cst" }

func (cstEngine) Format(src []byte, path string, kind filekind.Kind) ([]byte, error) {
	switch kind {
	case filekind.KindBUILD,
		filekind.KindWORKSPACE,
		filekind.KindMODULE,
		filekind.KindBzl,
		filekind.KindBzlmod:
		out, err := bzlbuildifier.FormatBytes(src)
		if err != nil {
			return nil, fmt.Errorf("cst/buildifier: %w", err)
		}
		return out, nil

	case filekind.KindBUCK, filekind.KindBzlBuck:
		// Buck2 dialect not implemented yet; callers fall back to Buildtools.
		return nil, fmt.Errorf("cst: BUCK files: %w", ErrEngineDoesNotSupport)

	default:
		// KindStarlark, KindSkyI, KindUnknown — use neutral mode.
		parsed := parser.ParseFile(src)
		out, err := (neutral.Neutral{}).Format(parsed.SyntaxTree(), src)
		if err != nil {
			return nil, fmt.Errorf("cst/neutral: %w", err)
		}
		return out, nil
	}
}
