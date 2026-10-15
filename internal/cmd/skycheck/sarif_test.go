package skycheck

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSARIFCommand(t *testing.T) {
	for _, clean := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "space #.star")
		source := "print(missing)\n"
		want := exitError
		if clean {
			source = "x = 1\n"
			want = exitOK
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		code := RunWithIO(context.Background(), []string{"--format=sarif", "--sarif-root=" + filepath.Dir(path), path}, nil, &out, &stderr)
		if code != want {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		var report struct {
			Version string
			Runs    []struct{ Results []json.RawMessage }
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatalf("not SARIF: %v: %s", err, out.String())
		}
		if report.Version != "2.1.0" || len(report.Runs) != 1 {
			t.Fatal("invalid SARIF envelope")
		}
		if clean && report.Runs[0].Results == nil {
			t.Fatal("clean scan must contain results array")
		}
		if !clean && len(report.Runs[0].Results) == 0 {
			t.Fatal("lost finding")
		}
	}
}

func TestSARIFRootAndPartialFailure(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.star"), []byte("print(missing)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	var out, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"--format=sarif", "--sarif-root=" + root, "main.star", "missing.star"}, nil, &out, &stderr)
	if code != exitError {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"uri": "sub/main.star"`)) || !bytes.Contains(out.Bytes(), []byte(`"ruleId": "undefined"`)) || !bytes.Contains(out.Bytes(), []byte(`"executionSuccessful": false`)) {
		t.Fatalf("lost partial result or root: %s", out.String())
	}
}
