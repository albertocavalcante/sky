package tester

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageRegistersExecutableLines(t *testing.T) {
	opts := DefaultOptions()
	opts.Coverage = true
	runner := New(opts)
	src := []byte("def test_a():\n    assert.eq(1, 1)\n\ndef never_called():\n    return len(\"a\")\n")
	if _, err := runner.RunFile("cov.star", src); err != nil {
		t.Fatal(err)
	}
	rep := runner.CoverageReport()
	fc := rep.GetFile("cov.star")
	if fc == nil {
		t.Fatal("no coverage for file")
	}
	// The compiler attributes the call in never_called's body to line 5.
	if n, ok := fc.Lines.Hits[5]; !ok || n != 0 {
		t.Errorf("unexecuted function line 5 not registered with 0 hits: %v", fc.Lines.Hits)
	}
	if rep.TotalLines < 3 || rep.CoveredLines == rep.TotalLines {
		t.Errorf("total=%d covered=%d, want missing lines", rep.TotalLines, rep.CoveredLines)
	}
}

func covRun(t *testing.T, dir, file, src string, opts Options) *Runner {
	t.Helper()
	opts.Coverage = true
	r := New(opts)
	if _, err := r.RunFile(file, []byte(src)); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCoverageFullyExecutedFunctionHasNoMissingLines(t *testing.T) {
	src := "def test_a():\n    x = 1\n    y = [\n        1,\n        2,\n    ]\n    return\n"
	r := covRun(t, "", "full.star", src, DefaultOptions())
	rep := r.CoverageReport()
	fc := rep.GetFile("full.star")
	if rep.TotalLines != rep.CoveredLines || rep.TotalLines == 0 {
		t.Errorf("total=%d covered=%d hits=%v; fully executed code must be 100%%", rep.TotalLines, rep.CoveredLines, fc.Lines.Hits)
	}
}

func TestCoverageUntakenBranchInConftestIsMissing(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "conftest.star")
	src := "def fixture_f():\n    if False:\n        return len(\"a\")\n    return 1\n"
	if err := os.WriteFile(conf, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	r := covRun(t, dir, filepath.Join(dir, "t.star"), "def test_a(f):\n    assert.eq(f, 1)\n", DefaultOptions())
	rep := r.CoverageReport()
	fc := rep.GetFile(conf)
	if fc == nil {
		t.Fatalf("conftest not tracked: %v", rep.FilePaths())
	}
	if n, ok := fc.Lines.Hits[3]; !ok || n != 0 {
		t.Errorf("conftest untaken line 3 not registered with 0 hits: %v", fc.Lines.Hits)
	}
}

func TestCoverageFirstFunctionUntakenBranchIsMissing(t *testing.T) {
	// The file starts with a def, so its position equals the module initializer's.
	src := "def test_a():\n    if False:\n        return len(\"a\")\n    assert.eq(1, 1)\n"
	r := covRun(t, "", "first.star", src, DefaultOptions())
	fc := r.CoverageReport().GetFile("first.star")
	if n, ok := fc.Lines.Hits[3]; !ok || n != 0 {
		t.Errorf("untaken branch line 3 not registered with 0 hits: %v", fc.Lines.Hits)
	}
}
