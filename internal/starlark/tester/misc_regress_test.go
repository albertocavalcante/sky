package tester

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAssertContainsTupleAndBytes(t *testing.T) {
	res := runSrc(t, "c.star", `
def test_tuple():
    assert.contains((1, 2), 1)

def test_bytes():
    assert.contains(b"abc", b"bc")
    assert.contains(b"abc", 97)
`, DefaultOptions())
	if len(res.Tests) != 2 {
		t.Fatalf("got %d tests", len(res.Tests))
	}
	for _, tr := range res.Tests {
		if !tr.Passed {
			t.Errorf("%s failed: %v", tr.Name, tr.Error)
		}
	}
}

func TestJUnitXPassAndSkipped(t *testing.T) {
	rr := &RunResult{Files: []FileResult{{
		File: "f.star",
		Tests: []TestResult{
			{Name: "test_xpass", XFail: true, XPass: true, Passed: false},
			{Name: "test_skip", Skipped: true, Passed: true, SkipReason: "later"},
			{Name: "test_fail", Error: errors.New("boom")},
			{Name: "test_ok", Passed: true},
		},
	}}}
	var buf bytes.Buffer
	(&JUnitReporter{}).ReportSummary(&buf, rr)
	out := buf.String()
	if n := strings.Count(out, "<failure"); n != 2 {
		t.Errorf("failures = %d, want 2\n%s", n, out)
	}
	if !strings.Contains(out, "<skipped") {
		t.Errorf("missing <skipped/>\n%s", out)
	}
	if !strings.Contains(out, `failures="2"`) {
		t.Errorf("suite failure count wrong\n%s", out)
	}
}

func TestFilterNotCaseInsensitive(t *testing.T) {
	for _, f := range []string{"Not slow", "NOT slow", "not slow"} {
		opts := DefaultOptions()
		opts.Filter = f
		r := New(opts)
		if r.matchesFilter("test_slow") {
			t.Errorf("%q should exclude test_slow", f)
		}
		if !r.matchesFilter("test_fast") {
			t.Errorf("%q should include test_fast", f)
		}
	}
}

func TestTestNamesMatchParametrizedBase(t *testing.T) {
	opts := DefaultOptions()
	opts.TestNames = []string{"test_p"}
	res := runSrc(t, "p.star", `
__test_params__ = {"test_p": [{"name": "a"}, {"name": "b"}]}

def test_p(case):
    pass

def test_other():
    pass
`, opts)
	if len(res.Tests) != 2 {
		t.Fatalf("got %d tests, want 2 parametrized cases: %+v", len(res.Tests), res.Tests)
	}
}
