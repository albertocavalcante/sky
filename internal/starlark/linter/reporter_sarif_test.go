package linter

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestSARIFSourceSnapshotAndFix(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "space #.star")
	r := &SARIFReporter{Root: root, Rules: []*Rule{{Name: "rule", Doc: "Stable description", URL: "https://example.com/rule"}}}
	result := &Result{Files: 1, Sources: map[string][]byte{path: []byte("é😀x\n")}, Findings: []Finding{{FilePath: path, Rule: "rule", Message: "message", Severity: SeverityWarning, Line: 1, Column: 3, EndLine: 1, EndColumn: 4, Replacement: &Replacement{Start: 2, End: 6, Content: "replacement"}}}}
	var out bytes.Buffer
	if err := r.Report(&out, result); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Runs []struct {
			Results []struct {
				Locations []struct {
					Physical struct {
						Artifact struct{ URI string } `json:"artifactLocation"`
						Region   struct {
							StartColumn int `json:"startColumn"`
							EndColumn   int `json:"endColumn"`
						}
					} `json:"physicalLocation"`
				}
				Fixes []json.RawMessage
			}
		}
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	f := report.Runs[0].Results[0]
	loc := f.Locations[0].Physical
	if loc.Region.StartColumn != 4 || loc.Region.EndColumn != 5 || loc.Artifact.URI != "src/space%20%23.star" || len(f.Fixes) != 1 {
		t.Fatalf("bad source adapter: %s", out.String())
	}
	if err := r.Report(sarifBrokenWriter{}, result); err == nil {
		t.Fatal("discarded writer error")
	}
}

type sarifBrokenWriter struct{}

func (sarifBrokenWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }
