// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by the license in this repository.

package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bazelbuild/buildtools/build"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/albertocavalcante/sky/internal/protocol"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

func renameCaptureFixture(t testing.TB, n int) *build.File {
	t.Helper()
	var b strings.Builder
	b.WriteString("x = 1\ndef f(")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("p" + strconv.Itoa(i))
	}
	b.WriteString("):\n    return [")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("x")
	}
	b.WriteString("]\n")
	file, err := parseStarlarkFile([]byte(b.String()), "p.star", filekind.KindStarlark)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRenameCaptureManyParameters(t *testing.T) {
	file := renameCaptureFixture(t, 100000)
	if msg := renameCapture(file, "x", "y"); msg != "" {
		t.Fatalf("unexpected refusal: %s", msg)
	}
}

// This empirical guard targets the repeated parameter scan fixed in #87.
// It measures child-process CPU rather than wall time, so host descheduling and
// the race detector's exit delay do not count. It does not bound all rename work.
func TestRenameCaptureParameterScanCost(t *testing.T) {
	const workerEnv = "SKY_RENAME_SCAN_WORKER"
	if worker := os.Getenv(workerEnv); worker != "" {
		var n, repetitions int
		if _, err := fmt.Sscanf(worker, "%d,%d", &n, &repetitions); err != nil || n < 1 || repetitions < 1 {
			t.Fatalf("invalid worker input %q", worker)
		}
		file := renameCaptureFixture(t, n)
		for range repetitions {
			if msg := renameCapture(file, "x", "y"); msg != "" {
				t.Fatalf("unexpected refusal: %s", msg)
			}
		}
		return
	}

	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	measure := func(n, repetitions int) time.Duration {
		t.Helper()
		timeout := 2 * time.Minute
		if deadline, ok := t.Deadline(); ok && time.Until(deadline)-time.Second < timeout {
			timeout = time.Until(deadline) - time.Second
		}
		if timeout <= 0 {
			t.Fatal("parent deadline exhausted before CPU worker")
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestRenameCaptureParameterScanCost$", "-test.count=1", "-test.cpu=1")
		cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%d,%d", workerEnv, n, repetitions))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("worker n=%d repetitions=%d: %v (context: %v)\n%s", n, repetitions, err, ctx.Err(), out)
		}
		cpu := cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
		if cpu <= 0 {
			t.Fatal("child CPU accounting unavailable")
		}
		return cpu
	}

	const small, large = 5000, 50000
	repetitions := 1
	// Calibrate only the small case, then hold repetition count fixed. Calibrating
	// both cases to the same CPU budget would hide the quadratic regression.
	for measure(small, repetitions) < 100*time.Millisecond {
		if repetitions >= 1<<14 {
			t.Fatal("could not calibrate a measurable parameter-scan workload")
		}
		repetitions *= 2
	}
	baseline, doubled := measure(small, repetitions), measure(small, 2*repetitions)
	t.Logf("amortization: repetitions %d->%d, CPU %v->%v", repetitions, 2*repetitions, baseline, doubled)
	if doubled < baseline+baseline/4 {
		t.Fatal("doubling repeated rename work did not materially increase CPU; startup cost may dominate")
	}
	var ratios []float64
	for range 3 {
		a, b := measure(small, repetitions), measure(large, repetitions)
		ratio := float64(b) / float64(a)
		ratios = append(ratios, ratio)
		t.Logf("parameters %d->%d, repetitions=%d: CPU %v->%v, ratio=%.2f", small, large, repetitions, a, b, ratio)
	}
	sort.Float64s(ratios)
	// A tenfold input increase permits fourfold headroom over proportional cost,
	// while still rejecting the original per-reference parameter rescan (~100x).
	if ratios[1] > 4*float64(large)/float64(small) {
		t.Fatalf("median CPU growth %.2fx for %dx parameters suggests repeated parameter scans", ratios[1], large/small)
	}
}

func BenchmarkRenameCaptureParameters(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			file := renameCaptureFixture(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if msg := renameCapture(file, "x", "y"); msg != "" {
					b.Fatalf("unexpected refusal: %s", msg)
				}
			}
		})
	}
}

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

func TestRename_FirstComprehensionIterableResolvesOutside(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		old     string
		newName string
		refused bool
	}{
		{"first iterable captured by loop var", "x = 1\nr = [len([]) for len in [len]]\n", "x", "len", true},
		{"first iterable sees renamed global", "x = [1, 2]\nr = [y for y in x]\n", "x", "y", false},
		{"dict comprehension first iterable", "x = [1, 2]\nr = {y: 1 for y in x}\n", "x", "y", false},
		{"nested comprehension in first iterable", "x = [1, 2]\nr = [z for z in [y for y in x]]\n", "x", "y", false},
		{"later iterable is inside", "x = [1, 2]\nr = [y for w in [[1]] for y in x]\n", "x", "y", true},
		{"condition is inside", "x = 1\nr = [y for y in [1, 2] if y != x]\n", "x", "y", true},
		{"body is inside", "x = 1\nr = [y + x for y in [1, 2]]\n", "x", "y", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRenameByExecution(t, tt.code, tt.old, tt.newName, tt.refused)
		})
	}
}

func TestRename_KeywordLabelCollisions(t *testing.T) {
	const def = "def f(name, **kwargs):\n    return (name, kwargs)\n"
	tests := []struct {
		name    string
		code    string
		refused bool
	}{
		{"extra label equals the new name", def + "r = f(1, label=2)\n", true},
		{"call passes both old and new label", def + "r = f(name=1, label=2)\n", true},
		{"unrelated extra label is fine", def + "r = f(name=1, other=2)\n", false},
		{"plain call without kwargs conflict", def + "r = f(1, other=2)\n", false},
		{"same label on a foreign callee is fine", def + "d = {}\nd.update(label=2)\nr = (f(name=1), d)\n", false},
		{"lambda with **kwargs receives the new label", "f = lambda name=1, **kwargs: (name, kwargs)\nr = f(label=2)\n", true},
		{"alias of a def receives the new label", def + "g = f\nr = g(label=2)\n", true},
		{"reassigned callee receives the new label", def + "f = lambda name=1, **kw: (name, kw)\nr = f(label=2)\n", true},
		{"computed callee receives the new label", def + "r = (lambda **kw: kw)(label=2)\n", true},
		{"def without **kwargs", "def f(name, other=0):\n    return (name, other)\nr = f(name=1, other=2)\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRenameByExecution(t, tt.code, "name", "label", tt.refused)
		})
	}
}
