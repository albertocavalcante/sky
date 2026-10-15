package lsp

import (
	"sort"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/albertocavalcante/sky/internal/protocol"
)

// runStar executes src and returns the printed value of the global r, or the
// error text, so two programs can be compared for equal behavior.
func runStar(src string) string {
	thread := &starlark.Thread{Name: "rename-exec"}
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, "p.star", src, nil)
	if err != nil {
		return "error: " + err.Error()
	}
	r, ok := globals["r"]
	if !ok {
		return "no r"
	}
	return r.String()
}

// applyEdits applies single-line, ASCII-only edits to src.
func applyEdits(t *testing.T, src string, edits []protocol.TextEdit) string {
	t.Helper()
	lines := strings.Split(src, "\n")
	sorted := append([]protocol.TextEdit(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i].Range.Start, sorted[j].Range.Start
		if a.Line != b.Line {
			return a.Line > b.Line
		}
		return a.Character > b.Character
	})
	for _, e := range sorted {
		l := lines[e.Range.Start.Line]
		lines[e.Range.Start.Line] = l[:e.Range.Start.Character] + e.NewText + l[e.Range.End.Character:]
	}
	return strings.Join(lines, "\n")
}

// checkRenameByExecution renames the first occurrence of oldName in code and
// asserts the verdict (refused or accepted). An accepted rename must leave the
// program's observable result (global r) unchanged.
func checkRenameByExecution(t *testing.T, code, oldName, newName string, wantRefused bool) {
	t.Helper()
	idx := strings.Index(code, oldName)
	if idx < 0 {
		t.Fatalf("%q not in %q", oldName, code)
	}
	line := strings.Count(code[:idx], "\n")
	char := idx - (strings.LastIndex(code[:idx], "\n") + 1)
	edit, err := doRename(t, code, uint32(line), uint32(char), newName)
	if wantRefused {
		if err == nil {
			after := applyEdits(t, code, edit.Changes["file:///rename.star"])
			t.Fatalf("rename %s -> %s must be refused; it would turn\n%s\ninto\n%s\nbefore: %s\nafter:  %s",
				oldName, newName, code, after, runStar(code), runStar(after))
		}
		if re, ok := err.(*ResponseError); !ok || re.Code != CodeRequestFailed {
			t.Errorf("err = %v, want RequestFailed", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("rename %s -> %s is safe and must be accepted: %v", oldName, newName, err)
	}
	if edit == nil {
		t.Fatal("no edits")
	}
	after := applyEdits(t, code, edit.Changes["file:///rename.star"])
	if before, got := runStar(code), runStar(after); before != got {
		t.Errorf("accepted rename changed behavior\n%s\n->\n%s\nbefore: %s\nafter:  %s", code, after, before, got)
	}
}

func TestRename_DefaultsResolveInEnclosingScope(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		old     string
		newName string
		refused bool
	}{
		{"global renamed onto builtin used as default", "x = 1\ndef f(len=len):\n    return len([])\nr = f()\n", "x", "len", true},
		{"default expression sees the global being renamed", "x = 1\ndef f(y=x):\n    return y\nr = f()\n", "x", "y", false},
		{"lambda default sees the global being renamed", "x = 1\nf = lambda y=x: y\nr = f()\n", "x", "y", false},
		{"lambda default capture", "x = 1\nf = lambda len=len: len([])\nr = f()\n", "x", "len", true},
		{"body use of param still captured", "x = 1\ndef f(y=2):\n    return x + y\nr = f()\n", "x", "y", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRenameByExecution(t, tt.code, tt.old, tt.newName, tt.refused)
		})
	}
}
