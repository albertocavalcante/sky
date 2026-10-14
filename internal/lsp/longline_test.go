package lsp

import (
	"strings"
	"testing"
	"time"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func TestCollectIdentRefs_LongLineLinear(t *testing.T) {
	const n = 50000
	content := "x = [" + strings.Repeat("x, ", n) + "]\n"
	file, err := parseStarlarkFile([]byte(content), "a.star", filekind.KindStarlark)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	refs := collectIdentRefs(content, file, "x")
	if len(refs) != n+1 {
		t.Fatalf("refs = %d, want %d", len(refs), n+1)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("collectIdentRefs took %v on one long line (quadratic)", d)
	}
}

func TestLineIndex_UTF16Col(t *testing.T) {
	li := newLineIndex("a𝒙é = 1\nplain\n")
	for _, c := range []struct{ line, rc, want int }{
		{0, 0, 0}, {0, 1, 1}, {0, 2, 3}, {0, 3, 4}, {0, 4, 5}, {0, 9, 10}, {1, 3, 3}, {5, 2, 2},
	} {
		if got := li.utf16Col(c.line, c.rc); got != c.want {
			t.Errorf("utf16Col(%d,%d) = %d, want %d", c.line, c.rc, got, c.want)
		}
	}
}
