package ci

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

// 57 of 100 lines is exactly 57%; 57/100*100 is 56.99999999999999 in float64.
func TestCoverageThresholdExactEqualityPasses(t *testing.T) {
	cov := filepath.Join(t.TempDir(), "cov.json")
	if err := os.WriteFile(cov, []byte(`{"files":{},"total_lines":100,"covered_lines":57,"percentage": 57}`), 0o644); err != nil {
		t.Fatal(err)
	}
	const pass = `{"passed":1,"failed":0,"total":1,"files":1,"duration_ms":1,"results":[{"file":"a","duration_ms":1,"tests":[{"name":"t","passed":true,"duration_ms":1}]}]}`
	var o, e bytes.Buffer
	args := []string{"--system=generic", "--coverage-file=" + cov, "--coverage-threshold=57"}
	if c := Run(args, strings.NewReader(pass), &o, &e); c != exitOK {
		t.Errorf("57/100 at threshold 57: code=%d, stderr=%s", c, e.String())
	}
}

// 64.4*250 is 16100.000000000002 in float64: 161 of 250 lines is exactly 64.4%
// and must pass, 160 of 250 must still fail.
func TestCoverageThresholdDecimalEquality(t *testing.T) {
	const pass = `{"passed":1,"failed":0,"total":1,"files":1,"duration_ms":1,"results":[{"file":"a","duration_ms":1,"tests":[{"name":"t","passed":true,"duration_ms":1}]}]}`
	tests := []struct {
		covered int
		pct     string
		want    int
	}{
		{161, "64.4", exitOK},
		{160, "64", exitFailed},
	}
	for _, tt := range tests {
		cov := filepath.Join(t.TempDir(), "cov.json")
		body := `{"files":{},"total_lines":250,"covered_lines":` + strconv.Itoa(tt.covered) + `,"percentage": ` + tt.pct + `}`
		if err := os.WriteFile(cov, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		var o, e bytes.Buffer
		args := []string{"--system=generic", "--coverage-file=" + cov, "--coverage-threshold=64.4"}
		if c := Run(args, strings.NewReader(pass), &o, &e); c != tt.want {
			t.Errorf("%d/250 at threshold 64.4: code=%d, want %d, stderr=%s", tt.covered, c, tt.want, e.String())
		}
	}
}

func TestCoverageThreshold(t *testing.T) {
	dir := t.TempDir()
	cov := filepath.Join(dir, "cov.json")
	if err := os.WriteFile(cov, []byte(`{"files":{"a.star":{"lines":{"1":1,"2":0}}},"total_lines":2,"covered_lines":1,"percentage": 50}`), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "coverage.json"), []byte(`{"files":{"a.star":{"lines":{"1":1,"2":0}}},"total_lines":2,"covered_lines":1,"percentage": 50}`), 0o644); err != nil {
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

const passJSON = `{"passed":1,"failed":0,"total":1,"files":1,"duration_ms":1,"results":[{"file":"a","duration_ms":1,"tests":[{"name":"t","passed":true,"duration_ms":1}]}]}`

func TestCoverageThresholdRejectsEmptyCoverage(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"zero lines (fabricated 100%)": `{"files":{},"total_lines":0,"covered_lines":0,"percentage":100}`,
		"no total_lines":               `{"percentage":100}`,
		"empty object":                 `{}`,
		"covered exceeds total":        `{"total_lines":2,"covered_lines":3,"percentage":150}`,
	} {
		cov := filepath.Join(dir, "cov.json")
		if err := os.WriteFile(cov, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		var o, e bytes.Buffer
		c := Run([]string{"--system=generic", "--coverage-file=" + cov, "--coverage-threshold=80"}, strings.NewReader(passJSON), &o, &e)
		if c != exitError {
			t.Errorf("%s: code=%d want %d (stderr=%s)", name, c, exitError, e.String())
		}
	}
}

func TestCoverageThresholdRejectsInvalidValues(t *testing.T) {
	cov := filepath.Join(t.TempDir(), "cov.json")
	if err := os.WriteFile(cov, []byte(`{"total_lines":2,"covered_lines":1,"percentage":50}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"NaN", "Inf", "-Inf", "-1", "101"} {
		var o, e bytes.Buffer
		c := Run([]string{"--system=generic", "--coverage-file=" + cov, "--coverage-threshold=" + v}, strings.NewReader(passJSON), &o, &e)
		if c != exitError {
			t.Errorf("threshold %s: code=%d want %d", v, c, exitError)
		}
	}
}

func TestSkippedTestsUseAuthoritativeCounts(t *testing.T) {
	// skytest reports a skipped test as passed:true in the per-test list but
	// excludes it from the aggregate passed/failed/total counts.
	const in = `{"passed":1,"failed":0,"total":1,"files":1,"duration_ms":1,"results":[{"file":"a","duration_ms":1,"tests":[` +
		`{"name":"ok","passed":true,"duration_ms":1},{"name":"skipped","passed":true,"duration_ms":0}]}]}`
	var r TestResults
	if err := json.Unmarshal([]byte(in), &r); err != nil {
		t.Fatal(err)
	}
	passed, failed, skipped, total := r.Summary()
	if passed != 1 || failed != 0 || total != 1 || skipped != 1 {
		t.Errorf("Summary() = passed %d failed %d skipped %d total %d; want 1 0 1 1", passed, failed, skipped, total)
	}
}

func TestPartialOrInconsistentAggregateCannotHideFailures(t *testing.T) {
	const tests = `"results":[{"file":"a","tests":[{"name":"t","passed":false}]}]`
	for name, in := range map[string]string{
		"total only":          `{"total":1,` + tests + `}`,
		"null failed":         `{"passed":0,"failed":null,"total":1,` + tests + `}`,
		"zero failed":         `{"passed":0,"failed":0,"total":1,` + tests + `}`,
		"inconsistent counts": `{"passed":5,"failed":0,"total":9,` + tests + `}`,
	} {
		var r TestResults
		if err := json.Unmarshal([]byte(in), &r); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !r.HasFailures() {
			t.Errorf("%s: failure hidden", name)
		}
		if _, failed, _, _ := r.Summary(); failed != 1 {
			t.Errorf("%s: failed = %d, want 1", name, failed)
		}
	}
}

// firstWriteChecker runs check just before the first byte is written.
type firstWriteChecker struct {
	bytes.Buffer
	check func()
	done  bool
}

func (w *firstWriteChecker) Write(p []byte) (int, error) {
	if !w.done {
		w.done = true
		w.check()
	}
	return w.Buffer.Write(p)
}

func TestSkytestWritesCoverageBeforeJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x_test.star"), []byte("def test_ok():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cov := filepath.Join(dir, "cov.json")
	// A stale report from a previous run must never survive.
	if err := os.WriteFile(cov, []byte(`{"total_lines":10,"covered_lines":10,"percentage":100}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &firstWriteChecker{}
	var existedAtJSON bool
	var contentAtJSON []byte
	out.check = func() {
		contentAtJSON, _ = os.ReadFile(cov)
		_, err := os.Stat(cov)
		existedAtJSON = err == nil
	}
	var errb bytes.Buffer
	skytest.RunWithIO(context.Background(), []string{"-json", "-coverage", "-coverprofile=" + cov, filepath.Join(dir, "x_test.star")}, nil, out, &errb)
	if !existedAtJSON {
		t.Fatalf("coverage file not written before JSON was emitted; stderr=%s", errb.String())
	}
	var got struct {
		TotalLines int `json:"total_lines"`
	}
	_ = json.Unmarshal(contentAtJSON, &got)
	if got.TotalLines == 10 {
		t.Errorf("stale coverage survived: %s", contentAtJSON)
	}

	// The shared collector produced real data, so sky-ci accepts it.
	var o, e bytes.Buffer
	c := Run([]string{"--system=generic", "--coverage-file=" + cov, "--coverage-threshold=80"}, &out.Buffer, &o, &e)
	if c != exitOK {
		t.Errorf("sky-ci with fresh coverage: code=%d want %d; stderr=%s", c, exitOK, e.String())
	}
}
