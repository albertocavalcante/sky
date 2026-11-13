package skyquery

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Not parallel: the test changes the working directory.
func TestRun_RelativeWorkspace(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "ws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "ws", "a.star"), []byte("def f():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-workspace", "ws", "f"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "no such file") {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}
