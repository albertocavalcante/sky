package skyquery

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_InvalidWorkspaceFailsRegardlessOfKeepGoing(t *testing.T) {
	dir := t.TempDir()
	notDir := filepath.Join(dir, "file.star")
	if err := os.WriteFile(notDir, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")

	for _, ws := range []string{missing, notDir} {
		for _, extra := range [][]string{nil, {"--keep_going"}} {
			args := append([]string{"--workspace", ws}, extra...)
			args = append(args, "files(//...)")
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), args, nil, &stdout, &stderr)
			if code == 0 {
				t.Errorf("RunWithIO(%v) = 0, want non-zero; stderr=%q", args, stderr.String())
			}
			if !strings.Contains(stderr.String(), "workspace") {
				t.Errorf("RunWithIO(%v): stderr lacks a clear workspace message: %q", args, stderr.String())
			}
		}
	}
}

func TestRun_KeepGoingStillContinuesPastParseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.star"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.star"), []byte("x = (\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--workspace", dir, "--keep_going", "files(//...)"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok.star") {
		t.Errorf("stdout lacks ok.star: %q", stdout.String())
	}
}
