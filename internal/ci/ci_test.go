package ci

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/cmd/skytest"
)

func TestEndToEndSkytestJSON(t *testing.T) {
	dir := t.TempDir()
	src := "def test_ok():\n    pass\n\ndef test_bad():\n    fail(\"boom\\nline2, a:b\")\n"
	if err := os.WriteFile(filepath.Join(dir, "x_test.star"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var js, errb bytes.Buffer
	skytest.RunWithIO(context.Background(), []string{"-json", dir}, nil, &js, &errb)

	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	t.Setenv("GITHUB_OUTPUT", "")
	var out, serr bytes.Buffer
	code := Run(nil, &js, &out, &serr)
	if code != exitFailed {
		t.Fatalf("code=%d want %d; stderr=%s", code, exitFailed, serr.String())
	}
	if !strings.Contains(out.String(), "::error file=") || !strings.Contains(out.String(), "test_bad") {
		t.Errorf("missing annotation: %q", out.String())
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Errorf("annotation must be a single line: %q", out.String())
	}
}

func TestEndToEndPassing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x_test.star"), []byte("def test_ok():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var js, errb bytes.Buffer
	skytest.RunWithIO(context.Background(), []string{"-json", dir}, nil, &js, &errb)
	t.Setenv("GITHUB_ACTIONS", "true")
	var out, serr bytes.Buffer
	if code := Run(nil, &js, &out, &serr); code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, serr.String())
	}
}

func TestAnnotationEscaping(t *testing.T) {
	r := &TestResults{Results: []FileResult{{
		File:  "a:b,c\nd.star",
		Tests: []TestResult{{Name: "t\n::error::pwn", Error: "x%\r\ny"}},
	}}}
	var w bytes.Buffer
	(&GitHubHandler{Config: Config{Annotations: true}}).writeAnnotations(r, &w)
	got := w.String()
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("injection: %q", got)
	}
	for _, want := range []string{"%3A", "%2C", "%0A", "%25", "%0D"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %q", want, got)
		}
	}
}

func TestCoverageThreshold(t *testing.T) {
	dir := t.TempDir()
	cov := filepath.Join(dir, "cov.json")
	if err := os.WriteFile(cov, []byte(`{"percentage": 50}`), 0o644); err != nil {
		t.Fatal(err)
	}
	const pass = `{"passed":1,"failed":0,"total":1,"files":1,"duration_ms":1,"results":[{"file":"a","duration_ms":1,"tests":[{"name":"t","passed":true,"duration_ms":1}]}]}`
	run := func(args ...string) int {
		var o, e bytes.Buffer
		return Run(append([]string{"--system=generic", "--coverage-file=" + cov}, args...), strings.NewReader(pass), &o, &e)
	}
	if c := run("--coverage-threshold=80"); c != exitFailed {
		t.Errorf("below threshold: code=%d", c)
	}
	if c := run("--coverage-threshold=40"); c != exitOK {
		t.Errorf("above threshold: code=%d", c)
	}
	if c := run(); c != exitOK {
		t.Errorf("no threshold: code=%d", c)
	}
	var o, e bytes.Buffer
	c := Run([]string{"--system=generic", "--coverage-file=" + filepath.Join(dir, "nope"), "--coverage-threshold=10"}, strings.NewReader(pass), &o, &e)
	if c != exitError {
		t.Errorf("missing coverage: code=%d", c)
	}
}

func TestCoverageFailUnderFromConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sky.toml"), []byte("[test.coverage]\nfail_under = 80\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "coverage.json"), []byte(`{"percentage": 50}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	const pass = `{"passed":1,"failed":0,"total":1,"files":1,"duration_ms":1,"results":[{"file":"a","duration_ms":1,"tests":[{"name":"t","passed":true,"duration_ms":1}]}]}`
	var o, e bytes.Buffer
	if c := Run([]string{"--system=generic"}, strings.NewReader(pass), &o, &e); c != exitFailed {
		t.Errorf("fail_under from config: code=%d stderr=%s", c, e.String())
	}
	if c := Run([]string{"--system=generic", "--coverage-threshold=0"}, strings.NewReader(pass), &o, &e); c != exitOK {
		t.Errorf("explicit 0 should disable: code=%d", c)
	}
}
