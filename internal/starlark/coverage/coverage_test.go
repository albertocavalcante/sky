package coverage

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLineCoverage(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		lc := NewLineCoverage()
		lc.Compute()

		if lc.TotalLines != 0 {
			t.Errorf("TotalLines = %d, want 0", lc.TotalLines)
		}
		if lc.CoveredLines != 0 {
			t.Errorf("CoveredLines = %d, want 0", lc.CoveredLines)
		}
		if lc.Percentage() != 100.0 {
			t.Errorf("Percentage = %f, want 100.0", lc.Percentage())
		}
	})

	t.Run("with hits", func(t *testing.T) {
		lc := NewLineCoverage()
		lc.RecordHit(1)
		lc.RecordHit(1)
		lc.RecordHit(2)
		lc.Hits[3] = 0 // Line exists but not covered
		lc.Compute()

		if lc.TotalLines != 3 {
			t.Errorf("TotalLines = %d, want 3", lc.TotalLines)
		}
		if lc.CoveredLines != 2 {
			t.Errorf("CoveredLines = %d, want 2", lc.CoveredLines)
		}
		if lc.Hits[1] != 2 {
			t.Errorf("Hits[1] = %d, want 2", lc.Hits[1])
		}
	})

	t.Run("lines sorted", func(t *testing.T) {
		lc := NewLineCoverage()
		lc.RecordHit(10)
		lc.RecordHit(5)
		lc.RecordHit(15)

		lines := lc.Lines()
		if len(lines) != 3 {
			t.Fatalf("len(Lines) = %d, want 3", len(lines))
		}
		if lines[0] != 5 || lines[1] != 10 || lines[2] != 15 {
			t.Errorf("Lines = %v, want [5 10 15]", lines)
		}
	})
}

func TestFileCoverage(t *testing.T) {
	fc := NewFileCoverage("/path/to/file.star")

	if fc.Path != "/path/to/file.star" {
		t.Errorf("Path = %q, want /path/to/file.star", fc.Path)
	}
	if fc.Lines == nil {
		t.Error("Lines is nil")
	}
	if fc.Functions == nil {
		t.Error("Functions is nil")
	}
}

func TestReport(t *testing.T) {
	t.Run("empty report", func(t *testing.T) {
		r := NewReport()
		r.Compute()

		if r.TotalLines != 0 {
			t.Errorf("TotalLines = %d, want 0", r.TotalLines)
		}
		if r.Percentage() != 100.0 {
			t.Errorf("Percentage = %f, want 100.0", r.Percentage())
		}
	})

	t.Run("add files", func(t *testing.T) {
		r := NewReport()

		fc1 := r.AddFile("a.star")
		fc1.Lines.RecordHit(1)
		fc1.Lines.RecordHit(2)

		fc2 := r.AddFile("b.star")
		fc2.Lines.RecordHit(1)
		fc2.Lines.Hits[2] = 0

		r.Compute()

		if r.TotalLines != 4 {
			t.Errorf("TotalLines = %d, want 4", r.TotalLines)
		}
		if r.CoveredLines != 3 {
			t.Errorf("CoveredLines = %d, want 3", r.CoveredLines)
		}
	})

	t.Run("get file", func(t *testing.T) {
		r := NewReport()
		r.AddFile("exists.star")

		if r.GetFile("exists.star") == nil {
			t.Error("GetFile returned nil for existing file")
		}
		if r.GetFile("missing.star") != nil {
			t.Error("GetFile returned non-nil for missing file")
		}
	})

	t.Run("file paths sorted", func(t *testing.T) {
		r := NewReport()
		r.AddFile("z.star")
		r.AddFile("a.star")
		r.AddFile("m.star")

		paths := r.FilePaths()
		if len(paths) != 3 {
			t.Fatalf("len(FilePaths) = %d, want 3", len(paths))
		}
		if paths[0] != "a.star" || paths[1] != "m.star" || paths[2] != "z.star" {
			t.Errorf("FilePaths = %v, want [a.star m.star z.star]", paths)
		}
	})

	t.Run("merge", func(t *testing.T) {
		r1 := NewReport()
		fc1 := r1.AddFile("shared.star")
		fc1.Lines.RecordHit(1)
		fc1.Lines.RecordHit(2)
		r1.AddFile("only1.star")

		r2 := NewReport()
		fc2 := r2.AddFile("shared.star")
		fc2.Lines.RecordHit(2)
		fc2.Lines.RecordHit(3)
		r2.AddFile("only2.star")

		r1.Merge(r2)

		if len(r1.Files) != 3 {
			t.Errorf("len(Files) = %d, want 3", len(r1.Files))
		}

		shared := r1.GetFile("shared.star")
		if shared.Lines.Hits[1] != 1 {
			t.Errorf("shared Hits[1] = %d, want 1", shared.Lines.Hits[1])
		}
		if shared.Lines.Hits[2] != 2 {
			t.Errorf("shared Hits[2] = %d, want 2", shared.Lines.Hits[2])
		}
		if shared.Lines.Hits[3] != 1 {
			t.Errorf("shared Hits[3] = %d, want 1", shared.Lines.Hits[3])
		}
	})
}

func TestCollector(t *testing.T) {
	c := NewCollector()

	c.BeforeExec("file.star", 1)
	c.BeforeExec("file.star", 1)
	c.BeforeExec("file.star", 2)
	c.EnterFunction("file.star", "test_foo", 5)
	c.EnterFunction("file.star", "test_foo", 5)

	report := c.Report()

	fc := report.GetFile("file.star")
	if fc == nil {
		t.Fatal("file.star not in report")
	}

	if fc.Lines.Hits[1] != 2 {
		t.Errorf("Hits[1] = %d, want 2", fc.Lines.Hits[1])
	}
	if fc.Lines.Hits[2] != 1 {
		t.Errorf("Hits[2] = %d, want 1", fc.Lines.Hits[2])
	}

	fn := fc.Functions["test_foo"]
	if fn == nil {
		t.Fatal("test_foo not in functions")
	}
	if fn.Hits != 2 {
		t.Errorf("test_foo.Hits = %d, want 2", fn.Hits)
	}
}

func TestTextReporter(t *testing.T) {
	report := NewReport()
	fc := report.AddFile("test.star")
	fc.Lines.RecordHit(1)
	fc.Lines.RecordHit(2)
	fc.Lines.Hits[3] = 0

	var buf bytes.Buffer
	r := &TextReporter{Verbose: true, ShowMissing: true}
	if err := r.Write(&buf, report); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test.star") {
		t.Error("output should contain filename")
	}
	if !strings.Contains(output, "66.7%") {
		t.Errorf("output should contain coverage percentage, got: %s", output)
	}
	if !strings.Contains(output, "Missing: 3") {
		t.Error("output should contain missing lines")
	}
}

func TestJSONReporter(t *testing.T) {
	report := NewReport()
	fc := report.AddFile("test.star")
	fc.Lines.RecordHit(1)
	fc.Lines.Hits[2] = 0

	var buf bytes.Buffer
	r := &JSONReporter{Pretty: true}
	if err := r.Write(&buf, report); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var jr JSONReport
	if err := json.Unmarshal(buf.Bytes(), &jr); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if jr.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", jr.TotalLines)
	}
	if jr.CoveredLines != 1 {
		t.Errorf("CoveredLines = %d, want 1", jr.CoveredLines)
	}
	if len(jr.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(jr.Files))
	}
	if jr.Files[0].Path != "test.star" {
		t.Errorf("Files[0].Path = %q, want test.star", jr.Files[0].Path)
	}
}

func TestCoberturaReporter(t *testing.T) {
	report := NewReport()
	fc := report.AddFile("src/test.star")
	fc.Lines.RecordHit(1)
	fc.Lines.RecordHit(2)

	var buf bytes.Buffer
	r := &CoberturaReporter{SourceDir: "/workspace"}
	if err := r.Write(&buf, report); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "<?xml version=") {
		t.Error("output should contain XML header")
	}
	if !strings.Contains(output, "<coverage") {
		t.Error("output should contain coverage element")
	}
	if !strings.Contains(output, "src/test.star") {
		t.Error("output should contain filename")
	}
}

func TestLCOVReporter(t *testing.T) {
	report := NewReport()
	fc := report.AddFile("test.star")
	fc.Lines.RecordHit(1)
	fc.Lines.RecordHit(2)
	fc.Functions["test_foo"] = &FunctionCoverage{
		Name:      "test_foo",
		StartLine: 1,
		Hits:      3,
	}

	var buf bytes.Buffer
	r := &LCOVReporter{}
	if err := r.Write(&buf, report); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SF:test.star") {
		t.Error("output should contain SF line")
	}
	if !strings.Contains(output, "DA:1,1") {
		t.Error("output should contain DA line")
	}
	if !strings.Contains(output, "FN:1,test_foo") {
		t.Error("output should contain FN line")
	}
	if !strings.Contains(output, "end_of_record") {
		t.Error("output should contain end_of_record")
	}
}

func TestFormatLineRanges(t *testing.T) {
	tests := []struct {
		lines []int
		want  string
	}{
		{nil, ""},
		{[]int{1}, "1"},
		{[]int{1, 2, 3}, "1-3"},
		{[]int{1, 3, 5}, "1, 3, 5"},
		{[]int{1, 2, 3, 10, 11, 20}, "1-3, 10-11, 20"},
	}

	for _, tt := range tests {
		got := formatLineRanges(tt.lines)
		if got != tt.want {
			t.Errorf("formatLineRanges(%v) = %q, want %q", tt.lines, got, tt.want)
		}
	}
}
