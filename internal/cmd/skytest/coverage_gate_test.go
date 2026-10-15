package skytest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/coverage"
)

// gateFixture writes a test file with an uncovered branch and a config that
// sets coverage.fail_under, returning the arguments for RunWithIO.
func gateFixture(t *testing.T, failUnder string) []string {
	t.Helper()
	dir := t.TempDir()
	// The compiler only attributes lines to calls, and a function that is
	// never called registers its lines with zero hits, so unused() leaves
	// missing lines.
	src := "def unused():\n    a = len(\"a\")\n    b = len(\"b\")\n    return a + b\n\ndef test_a():\n    assert.eq(len(\"a\"), 1)\n"
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

// The fixture measures 3 of 6 lines (50%): the gate must decide the exit code
// of a real coverage run, not just of checkCoverageThreshold.
func TestRun_FailUnderBelowThresholdExits1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), gateFixture(t, "80"), nil, &stdout, &stderr)
	if strings.Contains(stderr.String(), "coverage unavailable") {
		t.Skip("coverage hook not enabled in this build")
	}
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, exitFailed, stderr.String())
	}
	if !strings.Contains(stderr.String(), "below threshold") {
		t.Errorf("stderr = %q, want a below-threshold diagnostic", stderr.String())
	}
}

// sizedGateFixture is like gateFixture but measures exactly total lines of
// which covered are hit: unused() holds the uncovered call lines, test_a the
// covered ones (plus the two def lines and one assert).
func sizedGateFixture(t *testing.T, failUnder string, covered, total int) []string {
	t.Helper()
	var src strings.Builder
	src.WriteString("def unused():\n")
	for i := 0; i < total-covered; i++ {
		src.WriteString("    a" + strconv.Itoa(i) + " = len(\"a\")\n")
	}
	src.WriteString("    return 1\n\ndef test_a():\n")
	for i := 0; i < covered-3; i++ {
		src.WriteString("    b" + strconv.Itoa(i) + " = len(\"b\")\n")
	}
	src.WriteString("    assert.eq(len(\"a\"), 1)\n")
	dir := t.TempDir()
	file := filepath.Join(dir, "test_a.star")
	if err := os.WriteFile(file, []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "sky.toml")
	if err := os.WriteFile(cfg, []byte("[test.coverage]\nfail_under = "+failUnder+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{"--config", cfg, "--coverage", "--coverprofile=" + filepath.Join(dir, "cov.json"), file}
}

// 64.4*250 is 16100.000000000002 in float64, so a float product comparison
// fails 161 of 250 lines (exactly 64.4%) at fail_under = 64.4.
func TestRun_FailUnderEqualToCoveragePasses(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), sizedGateFixture(t, "64.4", 161, 250), nil, &stdout, &stderr)
	if strings.Contains(stderr.String(), "coverage unavailable") {
		t.Skip("coverage hook not enabled in this build")
	}
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (161 of 250 lines = 64.4%%, threshold 64.4)\nstderr: %s", code, stderr.String())
	}
}

// Guard, not proof: 160 of 250 (64%) also failed under the old float
// comparison. TestRun_FailUnderEqualToCoveragePasses (161 of 250) is the case
// that proves the exact comparison.
func TestRun_FailUnderOneLineShortStillFails_Guard(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), sizedGateFixture(t, "64.4", 160, 250), nil, &stdout, &stderr)
	if strings.Contains(stderr.String(), "coverage unavailable") {
		t.Skip("coverage hook not enabled in this build")
	}
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d (160 of 250 lines = 64%%, threshold 64.4)\nstderr: %s", code, exitFailed, stderr.String())
	}
}

// --watch never reaches the gate, so combining it with an enforced threshold
// must be refused instead of silently exiting 0.
func TestRun_FailUnderWithWatchIsRejected(t *testing.T) {
	args := append([]string{"--watch"}, gateFixture(t, "80")...)
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), args, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--watch") || !strings.Contains(stderr.String(), "fail_under") {
		t.Errorf("stderr = %q, want a message naming --watch and fail_under", stderr.String())
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
		// Proving cases: the float comparison wrongly failed these equalities.
		// 57/100*100 is 56.99999999999999 in float64; exact equality passes.
		{"exact equality 57 of 100", &coverage.Report{TotalLines: 100, CoveredLines: 57}, 57, exitOK, ""},
		{"exact equality 161 of 250 at 64.4", &coverage.Report{TotalLines: 250, CoveredLines: 161}, 64.4, exitOK, ""},
		{"guard: 160 of 250 at 64.4 (old code also failed)", &coverage.Report{TotalLines: 250, CoveredLines: 160}, 64.4, exitFailed, "below threshold"},
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
