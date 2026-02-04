package skycov

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-version"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-version) returned %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Error("RunWithIO(-version) produced no output")
	}
}

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-help"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-help) returned %d, want 0", code)
	}
}

func TestRun_CoverageReport(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON coverage data file (the format skycov expects)
	covFile := filepath.Join(dir, "coverage.json")
	covContent := `{
  "files": {
    "lib.star": {
      "lines": {
        "1": 5,
        "2": 5,
        "3": 3,
        "4": 3,
        "5": 0
      }
    }
  }
}`
	if err := os.WriteFile(covFile, []byte(covContent), 0644); err != nil {
		t.Fatalf("failed to write coverage file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{covFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(coverage) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Should show coverage information
	output := stdout.String()
	if !strings.Contains(output, "coverage") && !strings.Contains(output, "%") {
		t.Errorf("output does not contain coverage info\noutput: %s", output)
	}
}

func TestRun_CoverageOutputFormats(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON coverage data file (the format skycov expects)
	covFile := filepath.Join(dir, "coverage.json")
	covContent := `{
  "files": {
    "lib.star": {
      "lines": {
        "1": 5,
        "2": 5
      }
    }
  }
}`
	if err := os.WriteFile(covFile, []byte(covContent), 0644); err != nil {
		t.Fatalf("failed to write coverage file: %v", err)
	}

	formats := []struct {
		name string
		flag string
	}{
		{"text", "text"},
		{"json", "json"},
		{"html", "html"},
		{"lcov", "lcov"},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-format", tc.flag, covFile}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-format %s) returned %d, want 0\nstderr: %s", tc.flag, code, stderr.String())
			}
		})
	}
}

func TestRun_CoverageOutputToFile(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON coverage data file (the format skycov expects)
	covFile := filepath.Join(dir, "coverage.json")
	covContent := `{
  "files": {
    "lib.star": {
      "lines": {
        "1": 5,
        "2": 5
      }
    }
  }
}`
	if err := os.WriteFile(covFile, []byte(covContent), 0644); err != nil {
		t.Fatalf("failed to write coverage file: %v", err)
	}

	outputFile := filepath.Join(dir, "output.txt")

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-o", outputFile, covFile}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-o file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Check output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("output file was not created")
	}
}

func TestRun_CoverageThreshold(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON coverage data file with partial coverage (50%)
	// 2 lines covered out of 4 total
	covFile := filepath.Join(dir, "coverage.json")
	covContent := `{
  "files": {
    "lib.star": {
      "lines": {
        "1": 5,
        "2": 5,
        "3": 0,
        "4": 0
      }
    }
  }
}`
	if err := os.WriteFile(covFile, []byte(covContent), 0644); err != nil {
		t.Fatalf("failed to write coverage file: %v", err)
	}

	t.Run("threshold met", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		// Use -min flag (not -threshold) and set to 40%, coverage is 50%
		code := RunWithIO(context.Background(), []string{"-min", "40", covFile}, nil, &stdout, &stderr)

		if code != 0 {
			t.Errorf("RunWithIO(-min 40) returned %d, want 0\nstderr: %s", code, stderr.String())
		}
	})

	t.Run("threshold not met", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		// Use -min flag and set to 90%, coverage is 50%
		code := RunWithIO(context.Background(), []string{"-min", "90", covFile}, nil, &stdout, &stderr)

		if code == 0 {
			t.Error("RunWithIO(-min 90) returned 0, want non-zero for low coverage")
		}
	})
}

func TestRun_NoTestFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{}, nil, &stdout, &stderr)

	// Should show usage or error when no files provided
	if code == 0 && stdout.Len() == 0 && stderr.Len() == 0 {
		t.Error("RunWithIO() with no args produced no output")
	}
}

func TestRun_NonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"/nonexistent/coverage.json"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Error("RunWithIO(nonexistent file) returned 0, want non-zero")
	}
}
