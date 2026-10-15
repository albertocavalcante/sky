package skytest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/coverage"
)

// gateFixture writes a test file with an uncovered branch and a config that
// sets coverage.fail_under, returning the arguments for RunWithIO.
func gateFixture(t *testing.T, failUnder string) []string {
	t.Helper()
	dir := t.TempDir()
	src := "def helper(x):\n    if x:\n        return 1\n    return 2\n\ndef test_a():\n    assert.eq(helper(True), 1)\n"
	file := filepath.Join(dir, "test_a.star")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "sky.toml")
	if err := os.WriteFile(cfg, []byte("[test.coverage]\nfail_under = "+failUnder+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{"--config", cfg, "--coverage", "--coverprofile=" + filepath.Join(dir, "cov.json"), file}
}

func TestRun_FailUnderMetPasses(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), gateFixture(t, "1"), nil, &stdout, &stderr)
	if strings.Contains(stderr.String(), "coverage unavailable") {
		t.Skip("coverage hook not enabled in this build")
	}
	if code != exitOK {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_FailUnderWithoutCoverageWarns(t *testing.T) {
	args := gateFixture(t, "80")
	// Drop "--coverage" and "--coverprofile=..." (indexes 2 and 3).
	args = append(args[:2:2], args[4:]...)
	var stdout, stderr bytes.Buffer
	if code := RunWithIO(context.Background(), args, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not enforced") {
		t.Errorf("expected a warning that fail_under is not enforced, got %q", stderr.String())
	}
}

func TestRun_FailUnderInvalidIsRejected(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_a.star")
	if err := os.WriteFile(file, []byte("def test_a():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "sky.toml")
	if err := os.WriteFile(cfg, []byte("[test.coverage]\nfail_under = nan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := RunWithIO(context.Background(), []string{"--config", cfg, file}, nil, &stdout, &stderr); code != exitError {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, exitError, stderr.String())
	}
}

func TestCheckCoverageThreshold(t *testing.T) {
	tests := []struct {
		name      string
		report    *coverage.Report
		threshold float64
		want      int
		msg       string
	}{
		{"below", &coverage.Report{TotalLines: 10, CoveredLines: 7}, 80, exitFailed, "below threshold"},
		{"equal passes", &coverage.Report{TotalLines: 10, CoveredLines: 8}, 80, exitOK, ""},
		// 57/100*100 is 56.99999999999999 in float64; exact equality passes.
		{"exact equality 57 of 100", &coverage.Report{TotalLines: 100, CoveredLines: 57}, 57, exitOK, ""},
		{"one line short", &coverage.Report{TotalLines: 100, CoveredLines: 56}, 57, exitFailed, "below threshold"},
		{"above passes", &coverage.Report{TotalLines: 10, CoveredLines: 10}, 80, exitOK, ""},
		// Like sky-ci, zero measured lines is "unavailable", never 100%.
		{"zero lines", &coverage.Report{}, 80, exitError, "coverage unavailable"},
		{"nil report", nil, 80, exitError, "coverage unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if got := checkCoverageThreshold(tt.report, tt.threshold, true, &stderr); got != tt.want {
				t.Errorf("exit = %d, want %d (stderr %q)", got, tt.want, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.msg) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.msg)
			}
		})
	}
}

func TestCheckCoverageThresholdDisabledWarns(t *testing.T) {
	var stderr bytes.Buffer
	if code := checkCoverageThreshold(&coverage.Report{TotalLines: 1}, 80, false, &stderr); code != exitOK {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "not enforced") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRun_CoverprofileDirectoryIsRefused(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_a.star")
	if err := os.WriteFile(file, []byte("def test_a():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "covdir")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--coverage", "--coverprofile=" + out, file}, nil, &stdout, &stderr)
	if code != exitError {
		t.Errorf("exit = %d, want %d\nstderr: %s", code, exitError, stderr.String())
	}
	if fi, err := os.Stat(out); err != nil || !fi.IsDir() {
		t.Errorf("directory %s must be kept, got %v (err %v)", out, fi, err)
	}
}
