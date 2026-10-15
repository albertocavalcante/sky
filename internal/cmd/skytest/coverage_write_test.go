//go:build unix

package skytest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/tester"
)

func covRunner(t *testing.T) *tester.Runner {
	t.Helper()
	opts := tester.DefaultOptions()
	opts.Coverage = true
	r := tester.New(opts)
	if _, err := r.RunFile("c.star", []byte("def test_a():\n    assert.eq(1, 1)\n")); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWriteCoverageReportDoesNotUsePredictableTmp(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "coverage.json")
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, out+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := writeCoverageReport(covRunner(t), out, 0o666, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "keep" {
		t.Errorf("symlink target clobbered: %q", b)
	}
}

func TestWriteCoverageReportPreservesMode(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	out := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(out, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCoverageReport(covRunner(t), out, 0o600, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(out)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
	if b, _ := os.ReadFile(out); !strings.Contains(string(b), "total_lines") {
		t.Errorf("content not replaced: %q", b)
	}
}

func TestRun_StaleReportKeepsExactModeUnderUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	dir := t.TempDir()
	file := filepath.Join(dir, "test_a.star")
	if err := os.WriteFile(file, []byte("def test_a():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cov.json")
	if err := os.WriteFile(out, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(out, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := RunWithIO(context.Background(), []string{"--coverage", "--coverprofile=" + out, file}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit = %d\nstderr: %s", code, stderr.String())
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644 (the stale report's mode)", fi.Mode().Perm())
	}
}
