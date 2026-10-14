package tester

import (
	"path/filepath"
	"strings"
	"testing"
)

func runSrc(t *testing.T, filename, src string, opts Options) *FileResult {
	t.Helper()
	res, err := New(opts).RunFile(filename, []byte(src))
	if err != nil {
		t.Fatalf("RunFile: %v", err)
	}
	return res
}

func TestTestScopedFixtureSharedWithinTest(t *testing.T) {
	res := runSrc(t, "f.star", `
def fixture_db():
    return {}

def fixture_user(db):
    db["user"] = 1
    return db

def test_shared(db, user):
    assert.eq(db.get("user"), 1)
`, DefaultOptions())
	if len(res.Tests) != 1 || !res.Tests[0].Passed {
		t.Fatalf("expected pass, got %+v", res.Tests)
	}
}

func TestFixtureCycleReportsError(t *testing.T) {
	res := runSrc(t, "f.star", `
def fixture_a(b):
    return 1

def fixture_b(a):
    return 2

def test_x(a):
    pass
`, DefaultOptions())
	if len(res.Tests) != 1 || res.Tests[0].Passed || res.Tests[0].Error == nil {
		t.Fatalf("expected failure, got %+v", res.Tests)
	}
	if !strings.Contains(res.Tests[0].Error.Error(), "cycle") {
		t.Errorf("error = %v, want cycle message", res.Tests[0].Error)
	}
}

func TestMockResetBetweenRegularTests(t *testing.T) {
	res := runSrc(t, "m.star", `
def test_a(mock):
    m = mock.wrap(len)
    m([1])
    assert.eq(mock.call_count(m), 1)

def test_b(mock):
    m = mock.wrap(len)
    m([1])
    assert.eq(mock.call_count(m), 1)
`, DefaultOptions())
	for _, tr := range res.Tests {
		if !tr.Passed {
			t.Errorf("%s failed: %v", tr.Name, tr.Error)
		}
	}
}

func TestSnapshotInParametrizedTest(t *testing.T) {
	file := filepath.Join(t.TempDir(), "p.star")
	res := runSrc(t, file, `
__test_params__ = {"test_p": [{"name": "c1"}]}

def test_p(case):
    assert.snapshot("hello", "p_snap")
`, DefaultOptions())
	if len(res.Tests) != 1 || !res.Tests[0].Passed {
		t.Fatalf("expected pass, got %+v (err %v)", res.Tests, res.Tests[0].Error)
	}
}
