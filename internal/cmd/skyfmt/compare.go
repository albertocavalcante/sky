package skyfmt

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/formatter"
)

// compareStdin runs both engines against stdin and writes a divergence
// report to stdout. Returns exitOK when the engines agree (including when
// one returns ErrEngineDoesNotSupport, which is treated as a known
// abstention rather than disagreement). Returns exitNeedsFormat on any
// real divergence.
func compareStdin(stdin io.Reader, stdout, stderr io.Writer, kind filekind.Kind) int {
	src, err := io.ReadAll(stdin)
	if err != nil {
		writef(stderr, "skyfmt: reading stdin: %v\n", err)
		return exitError
	}
	if kind == "" {
		kind = filekind.KindStarlark
	}
	return reportCompare(stdout, stderr, "<stdin>", src, kind)
}

// comparePaths walks each path (file or directory) and runs both engines
// on every Starlark file, accumulating a divergence summary.
//
// Exit code:
//
//	exitOK           — every file agreed
//	exitNeedsFormat  — at least one file diverged
//	exitError        — IO or unexpected error
func comparePaths(paths []string, stdout, stderr io.Writer, kind filekind.Kind) int {
	var files []string
	for _, path := range paths {
		expanded, err := expandPath(path)
		if err != nil {
			writef(stderr, "skyfmt: %v\n", err)
			return exitError
		}
		files = append(files, expanded...)
	}
	if len(files) == 0 {
		writeln(stderr, "skyfmt: no files to compare")
		return exitOK
	}

	var (
		divergent   int
		agreed      int
		unsupported int
		errored     int
	)
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			writef(stderr, "skyfmt: %s: %v\n", path, err)
			errored++
			continue
		}
		fileKind := kind
		if fileKind == "" || fileKind == filekind.KindUnknown {
			fileKind = formatter.DetectKind(path)
		}
		status := reportCompare(stdout, stderr, path, src, fileKind)
		switch status {
		case exitOK:
			agreed++
		case exitNeedsFormat:
			divergent++
		case exitError:
			errored++
		case statusUnsupported:
			unsupported++
		}
	}

	// Summary always goes to stderr so stdout stays a clean diff stream.
	writef(stderr, "\nskyfmt compare: %d agreed, %d diverged, %d unsupported, %d errored\n",
		agreed, divergent, unsupported, errored)
	switch {
	case errored > 0:
		return exitError
	case divergent > 0:
		return exitNeedsFormat
	default:
		return exitOK
	}
}

// statusUnsupported is an internal compareStdin/comparePaths return value
// used only to keep accounting honest in the summary. It is mapped to
// exitOK at the process level.
const statusUnsupported = 99

// reportCompare runs both engines on src and writes a unified diff to
// stdout when they disagree.
func reportCompare(stdout, stderr io.Writer, path string, src []byte, kind filekind.Kind) int {
	btOut, btErr := formatter.Buildtools.Format(src, path, kind)
	cstOut, cstErr := formatter.CST.Format(src, path, kind)

	switch {
	case btErr != nil && cstErr != nil:
		// Both engines refused the input. Symmetric refusal == agreement.
		// Most commonly this is malformed source both engines hate. We
		// don't escalate as divergent.
		return exitOK
	case errors.Is(cstErr, formatter.ErrEngineDoesNotSupport):
		// CST hasn't implemented this file kind yet. Not a divergence;
		// the migration tracker bucket.
		return statusUnsupported
	case btErr != nil:
		writef(stderr, "skyfmt: %s: buildtools failed: %v\n", path, btErr)
		return exitError
	case cstErr != nil:
		writef(stderr, "skyfmt: %s: cst failed: %v\n", path, cstErr)
		return exitError
	}

	if bytes.Equal(btOut, cstOut) {
		return exitOK
	}

	if err := writeUnifiedDiff(stdout, path, string(btOut), string(cstOut)); err != nil {
		writef(stderr, "skyfmt: %s: diff render failed: %v\n", path, err)
		return exitError
	}
	return exitNeedsFormat
}

// writeUnifiedDiff emits a unified diff with 3 lines of context using
// pmezard/go-difflib's Myers implementation. Header lines name the two
// engines so a reader scanning a batch of diffs can immediately tell
// which side is which.
func writeUnifiedDiff(w io.Writer, path, a, b string) error {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(a),
		B:        difflib.SplitLines(b),
		FromFile: path + " [buildtools]",
		ToFile:   path + " [cst]",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return err
	}
	_, _ = io.WriteString(w, text)
	return nil
}

// (former detectKindForCompare removed: it was buggy — always returned
// KindUnknown because it tried to read the kind back from a Result
// that doesn't carry one. Replaced by formatter.DetectKind, now
// exported.)
