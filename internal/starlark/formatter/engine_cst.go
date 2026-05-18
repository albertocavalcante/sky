package formatter

import (
	"fmt"

	bzlbuildifier "github.com/albertocavalcante/bazel-cst-go/format/buildifier"
	"github.com/albertocavalcante/starlark-cst-go/parser"
	neutral "github.com/albertocavalcante/starlark-format-go"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// CST is the formatter built on the native Roslyn-style starlark-cst-go +
// bazel-cst-go + starlark-format-go (+ buck2-cst-go for the dialect side)
// stack.
//
// HEURISTIC — file-kind dispatch table.
//
//	What it does: maps sky's filekind.Kind to one of two formatter handlers:
//	  - bazel-cst-go/format/buildifier (Bazel-flavored — opinionated 11-step
//	    pipeline matching buildifier's output)
//	  - starlark-format-go/Neutral (spec-only — trivia normalisation, no
//	    opinionated reformats)
//
//	Why this particular dispatch: in sky's usage, *.bzl files in a
//	Bazel-dominant codebase get Bazel formatting. Buck2 files
//	(KindBUCK, KindBzlBuck) route to Neutral — Buck2 has no
//	buildifier-equivalent opinionated pipeline and most teams prefer
//	keep-my-formatting semantics. The buck2-cst-go library is loaded
//	by other tools (LSP, hover, refactor) for Buck2-specific
//	annotations; it deliberately doesn't ship a Format pipeline for v0.
//
//	Why deferred (no per-path inference for *.bzl ambiguity): we
//	could try harder to detect Buck2-ness from file content
//	(presence of `prelude_rules` calls, etc.). Not worth the
//	complexity for v0; sky's classifier already does the Bazel-vs-
//	Buck2 distinction by file location.
//
//	Source-of-truth pointer: engine_test.go pins each dispatch case
//	(BUILD → buildifier, .star → neutral, BUCK → neutral).
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
		// Buck2 files: trivia normalisation only. Buck2's ecosystem has
		// no buildifier-equivalent opinionated pipeline; the
		// buck2-cst-go library exists for LSP / refactor annotations
		// (see github.com/albertocavalcante/buck2-cst-go) but
		// deliberately doesn't ship a Format pipeline.
		parsed := parser.ParseFile(src)
		out, err := (neutral.Neutral{}).Format(parsed.SyntaxTree(), src)
		if err != nil {
			return nil, fmt.Errorf("cst/neutral (BUCK): %w", err)
		}
		return out, nil

	default:
		// KindStarlark, KindSkyI, KindUnknown — use neutral mode.
		//
		// parser.ParseFile is error-tolerant by design: it always returns
		// a usable SyntaxTree (with error markers) instead of refusing
		// to produce a tree on lex/parse errors. We deliberately don't
		// surface parse diagnostics here — the formatter's contract is
		// best-effort on partial input, mirroring how IDEs format
		// in-progress code. Hard failures still come from Neutral.Format
		// (e.g. encoding errors).
		parsed := parser.ParseFile(src)
		out, err := (neutral.Neutral{}).Format(parsed.SyntaxTree(), src)
		if err != nil {
			return nil, fmt.Errorf("cst/neutral: %w", err)
		}
		return out, nil
	}
}
