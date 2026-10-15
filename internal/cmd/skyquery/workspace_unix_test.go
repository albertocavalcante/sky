//go:build unix

package skyquery

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRun_UnreadableWorkspaceFailsRegardlessOfKeepGoing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode 000 directories")
	}
	dir := t.TempDir()
	ws := filepath.Join(dir, "locked")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "a.star"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ws, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })

	for _, extra := range [][]string{nil, {"--keep_going"}} {
		args := append([]string{"--workspace", ws}, extra...)
		args = append(args, "files(//...)")
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), args, nil, &stdout, &stderr)
		if code == 0 {
			t.Errorf("RunWithIO(%v) = 0, want non-zero; stderr=%q", args, stderr.String())
		}
		if !strings.Contains(stderr.String(), "invalid workspace") {
			t.Errorf("RunWithIO(%v): stderr lacks the invalid-workspace message: %q", args, stderr.String())
		}
	}
}

func TestRun_UntraversableWorkspaceFailsRegardlessOfKeepGoing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	ws := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "a.star"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Listable but not searchable: entries cannot be stat'ed or opened.
	if err := os.Chmod(ws, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })

	for _, extra := range [][]string{nil, {"--keep_going"}} {
		args := append([]string{"--workspace", ws}, extra...)
		args = append(args, "files(//...)")
		var stdout, stderr bytes.Buffer
		code := RunWithIO(context.Background(), args, nil, &stdout, &stderr)
		if code == 0 {
			t.Errorf("RunWithIO(%v) = 0, want non-zero; stderr=%q", args, stderr.String())
		}
		if !strings.Contains(stderr.String(), "invalid workspace") {
			t.Errorf("RunWithIO(%v): stderr lacks the invalid-workspace message: %q", args, stderr.String())
		}
	}
}

func TestRun_WorkspaceWithDanglingSymlinks(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "z.star"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Several dangling links so that one is almost certainly the first
	// directory entry; a dangling link must not make the workspace invalid.
	for i := 0; i < 20; i++ {
		link := filepath.Join(ws, "dangling"+strconv.Itoa(i))
		if err := os.Symlink(filepath.Join(ws, "missing"), link); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--workspace", ws, "files(//...)"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "z.star") {
		t.Errorf("z.star missing: stdout=%q", stdout.String())
	}
}

func TestRun_SymlinkedWorkspace(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "a.star"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--workspace", link, "files(//...)"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "a.star") {
		t.Errorf("symlinked workspace returned no files: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), dir) {
		t.Errorf("paths must stay relative to the workspace: %q", stdout.String())
	}
}
