package formatter

import (
	"fmt"

	bzlbuildifier "github.com/albertocavalcante/bazel-cst-go/format/buildifier"
	buck2buildifier "github.com/albertocavalcante/buck2-cst-go/format/buildifier"
	"github.com/albertocavalcante/starlark-cst-go/parser"
	neutral "github.com/albertocavalcante/starlark-format-go"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// CST is the formatter built on the native Roslyn-style starlark-cst-go +
// bazel-cst-go + buck2-cst-go + starlark-format-go + starlark-refactor-go
// stack.
//
// HEURISTIC — file-kind dispatch table.
//
//	What it does: maps sky's filekind.Kind to one of three formatter
//	handlers:
//	  - bazel-cst-go/format/buildifier (Bazel-flavored opinionated
//	    pipeline matching buildifier's output for BUILD/MODULE/.bzl)
//	  - buck2-cst-go/format/buildifier (Buck2-flavored opinionated
//	    pipeline — same 7 dialect-agnostic passes as Bazel, sans the
//	    MODULE.bazel-specific ones)
//	  - starlark-format-go/Neutral (spec-only — trivia normalisation,
//	    no opinionated reformats)
//
//	Why this particular dispatch: both Bazel and Buck2 files now get
//	full buildifier-style formatting. The two dialects share the same
//	dialect-agnostic refactor passes (load sorting, quote
//	normalization, attribute ordering, trailing-comma insertion,
//	line reflow at 79 cols, comment-block spacing) via
//	starlark-refactor-go; only MODULE.bazel-specific passes live in
//	bazel-cst-go. Files outside both dialects (generic .star,
//	skylark-internal, unknown) get Neutral.
//
//	Why deferred (no per-path inference for *.bzl ambiguity): we
//	could try harder to detect Buck2-ness from .bzl content
//	(presence of `prelude_rules` calls, `bxl_main`, etc.). Not worth
//	the complexity for v0; sky's classifier already does the Bazel-
//	vs-Buck2 distinction by file location.
//
//	Source-of-truth pointer: engine_test.go pins each dispatch case
//	(BUILD → buildifier, BUCK → buildifier (Buck2), .star → neutral).
//
// See bazel-cst-go/format/buildifier/DIFFERENCES.md for what the Bazel
// path does NOT yet do relative to upstream buildifier; Buck2 inherits
// the same gaps via the shared starlark-refactor-go pipeline.
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
			return nil, fmt.Errorf("cst/bazel-buildifier: %w", err)
		}
		return out, nil

	case filekind.KindBUCK, filekind.KindBzlBuck:
		// Buck2 files: full buildifier-style pipeline via buck2-cst-go.
		// Same 7 dialect-agnostic passes as the Bazel path (sourced
		// from starlark-refactor-go), minus the MODULE.bazel-specific
		// passes (use_repo sorting, bazel_dep separators, MODULE-
		// specific top-level blank insertion, MODULE call expansion).
		out, err := buck2buildifier.FormatBytes(src)
		if err != nil {
			return nil, fmt.Errorf("cst/buck2-buildifier: %w", err)
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
