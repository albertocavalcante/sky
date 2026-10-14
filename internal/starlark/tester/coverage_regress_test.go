package tester

import "testing"

func TestCoverageRegistersExecutableLines(t *testing.T) {
	opts := DefaultOptions()
	opts.Coverage = true
	runner := New(opts)
	src := []byte("def test_a():\n    assert.eq(1, 1)\n\ndef never_called():\n    return 1\n")
	if _, err := runner.RunFile("cov.star", src); err != nil {
		t.Fatal(err)
	}
	rep := runner.CoverageReport()
	fc := rep.GetFile("cov.star")
	if fc == nil {
		t.Fatal("no coverage for file")
	}
	if _, ok := fc.Lines.Hits[5]; !ok {
		t.Errorf("unexecuted line 5 not registered: %v", fc.Lines.Hits)
	}
	if rep.TotalLines < 4 {
		t.Errorf("TotalLines = %d, want >= 4", rep.TotalLines)
	}
}
