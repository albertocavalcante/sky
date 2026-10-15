package skylint

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSARIFCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.star")
	if err := os.WriteFile(path, []byte("load(\"a.star\", \"unused\")\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--format=sarif", "--sarif-root=" + filepath.Dir(path), path}, nil, &out, &stderr)
	if code != exitWarning {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var report struct {
		Version string
		Runs    []struct{ Results []json.RawMessage }
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Version != "2.1.0" || len(report.Runs) != 1 || len(report.Runs[0].Results) == 0 {
		t.Fatal("lost findings")
	}
}

func TestSARIFEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	var out, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--format=sarif", "--sarif-root=" + root, root}, nil, &out, &stderr)
	if code != exitError || !bytes.Contains(out.Bytes(), []byte(`"executionSuccessful": false`)) {
		t.Fatalf("empty directory reported as success: %d %s %s", code, out.String(), stderr.String())
	}
}
