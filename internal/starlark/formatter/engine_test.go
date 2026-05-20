package formatter_test

import (
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
	"github.com/albertocavalcante/sky/internal/starlark/formatter"
)

func TestEngines_RegistersBothEngines(t *testing.T) {
	engines := formatter.Engines()
	if _, ok := engines["buildtools"]; !ok {
		t.Errorf("Engines() missing buildtools")
	}
	if _, ok := engines["cst"]; !ok {
		t.Errorf("Engines() missing cst")
	}
}

func TestDefault_IsBuildtools(t *testing.T) {
	// Default must remain buildtools during the migration period. Flip
	// this assertion deliberately when you flip Default.
	if formatter.Default.Name() != "buildtools" {
		t.Errorf("Default.Name() = %q, want buildtools (migration not yet complete)", formatter.Default.Name())
	}
}

func TestBuildtoolsEngine_FormatsBUILDFile(t *testing.T) {
	src := []byte(`cc_library(name="foo",srcs=["a.c"])
`)
	out, err := formatter.Buildtools.Format(src, "BUILD", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(out), "name = \"foo\"") {
		t.Errorf("buildtools output missing canonical spacing:\n%s", out)
	}
}

func TestCSTEngine_FormatsBUILDFile(t *testing.T) {
	src := []byte(`cc_library(name = "foo", srcs = ["a.c"])
`)
	out, err := formatter.CST.Format(src, "BUILD", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("cst returned empty output for valid BUILD")
	}
}

func TestCSTEngine_FormatsStarlarkFile(t *testing.T) {
	src := []byte(`x = 1
`)
	out, err := formatter.CST.Format(src, "foo.star", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "x = 1\n" {
		t.Errorf("cst neutral output changed simple file: %q", out)
	}
}

func TestCSTEngine_FormatsBUCKThroughBuildifier(t *testing.T) {
	// Buck2 files route through buck2-cst-go/format/buildifier — the
	// same 7 dialect-agnostic passes as the Bazel path, sourced from
	// starlark-refactor-go.
	//
	// Canonical short call must round-trip unchanged.
	src := []byte(`cxx_library(name = "foo")
`)
	out, err := formatter.CST.Format(src, "BUCK", filekind.KindBUCK)
	if err != nil {
		t.Fatalf("CST.Format(BUCK) err = %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("buck2 buildifier mutated canonical BUCK file: got %q, want %q", out, src)
	}
}

func TestCSTEngine_FormatsBUCKAppliesDialectAgnosticPasses(t *testing.T) {
	// A BUCK file that has multiple buildifier-detectable issues
	// should be cleaned up: load symbols sorted, attribute kwargs
	// reordered (name first), trailing comma added on the multi-line
	// list. Proves the pipeline (not just Neutral) ran.
	src := []byte(`load("@prelude//:rules.bzl", "cxx_library", "cxx_binary")

cxx_binary(
    deps = [
        ":bar",
        ":foo"
    ],
    name = "main",
    srcs = ["main.cpp"],
)
`)
	want := []byte(`load("@prelude//:rules.bzl", "cxx_binary", "cxx_library")

cxx_binary(
    name = "main",
    srcs = ["main.cpp"],
    deps = [
        ":bar",
        ":foo",
    ],
)
`)
	out, err := formatter.CST.Format(src, "BUCK", filekind.KindBUCK)
	if err != nil {
		t.Fatalf("CST.Format(BUCK) err = %v", err)
	}
	if string(out) != string(want) {
		t.Errorf("buck2 buildifier didn't apply expected refactors.\n--- got:\n%s\n--- want:\n%s", out, want)
	}
}

func TestEngine_NamesAreDistinct(t *testing.T) {
	if formatter.Buildtools.Name() == formatter.CST.Name() {
		t.Errorf("engine Names collide: both report %q", formatter.Buildtools.Name())
	}
}

// TestCSTEngine_SortsLoadsAndAttrs is the headline "the new engine is
// doing real work" test. After porting buildifier's NamePriority table
// verbatim, CST now reorders rule kwargs per upstream's per-rule + global
// priorities. Expected ordering for cc_library kwargs:
//   - name (priority -99) first
//   - srcs (priority -90)
//   - deps (priority +4) last among the three
func TestCSTEngine_SortsLoadsAndAttrs(t *testing.T) {
	src := []byte(`load("@b", "y", "x")
load("@a", "n", "m")

cc_library(deps = [], name = "foo", srcs = ["a.c"])
`)
	out, err := formatter.CST.Format(src, "BUILD.bazel", filekind.KindBUILD)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := `load("@a", "m", "n")
load("@b", "x", "y")

cc_library(name = "foo", srcs = ["a.c"], deps = [])
`
	if string(out) != want {
		t.Errorf("CST output:\n%s\nwant:\n%s", out, want)
	}
}

func TestFormatWith_DelegatesToEngine(t *testing.T) {
	src := []byte("x = 1\n")
	out, err := formatter.FormatWith(formatter.CST, src, "x.star", filekind.KindStarlark)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "x = 1\n" {
		t.Errorf("FormatWith(CST, …) = %q", out)
	}
}
