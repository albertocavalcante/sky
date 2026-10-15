package skycov

import (
	"bytes"
	"context"
	"fmt"
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

func TestRun_NoPathIsError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunWithIO(context.Background(), []string{"-min", "99"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("no path: code = %d, want 2", code)
	}
	if stdout.Len() > 0 {
		t.Error("demo report must not be printed without --demo")
	}
}

func TestRun_Demo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunWithIO(context.Background(), []string{"--demo"}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("--demo: code = %d, stderr: %s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Error("--demo should print the sample report")
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunWithIO(context.Background(), []string{"--demo", "-min", "50"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("--demo with -min: code = %d, want 2", code)
	}
}

func TestRun_BadFormatDoesNotCreateOutput(t *testing.T) {
	dir := t.TempDir()
	covFile := filepath.Join(dir, "coverage.json")
	if err := os.WriteFile(covFile, []byte(`{"files":{"a.star":{"lines":{"1":1,"2":0}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.txt")
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-format", "nope", "-o", out, covFile}, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown format") {
		t.Errorf("stderr should name the bad format, got: %s", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("output file created despite invalid format")
	}
}

func TestRun_InvalidMinIsError(t *testing.T) {
	covFile := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(covFile, []byte(`{"files":{"lib.star":{"lines":{"1":0}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"NaN", "Inf", "-1", "101"} {
		t.Run(v, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-min=" + v, covFile}, nil, &stdout, &stderr)
			if code != 2 {
				t.Errorf("-min=%s: code = %d, want 2\nstderr: %s", v, code, stderr.String())
			}
			if stdout.Len() > 0 {
				t.Errorf("-min=%s: no report should be printed, got %q", v, stdout.String())
			}
		})
	}
}

func TestRun_MinWithNoMeasuredLinesIsError(t *testing.T) {
	covFile := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(covFile, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-min=80", covFile}, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("empty report with -min: code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "coverage unavailable") {
		t.Errorf("stderr = %q, want a coverage unavailable message", stderr.String())
	}

	// Without a requested minimum an empty report is still just reported.
	stdout.Reset()
	stderr.Reset()
	if code := RunWithIO(context.Background(), []string{covFile}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("empty report without -min: code = %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_MalformedLineEntriesAreDataErrors(t *testing.T) {
	tests := []struct {
		name, data, want string
	}{
		{"non-integer key", `{"files":{"lib.star":{"lines":{"1":1,"oops":0}}}}`, `"oops"`},
		{"zero key", `{"files":{"lib.star":{"lines":{"1":1,"0":0}}}}`, `"0"`},
		{"negative key", `{"files":{"lib.star":{"lines":{"1":1,"-2":0}}}}`, `"-2"`},
		{"negative hits", `{"files":{"lib.star":{"lines":{"1":1,"2":-1}}}}`, "negative hit count"},
		{"float hits", `{"files":{"lib.star":{"lines":{"1":1,"2":1.5}}}}`, "lines"},
	}
	for _, tt := range tests {
		for _, m := range []string{"", "-min=100"} {
			t.Run(tt.name+m, func(t *testing.T) {
				covFile := filepath.Join(t.TempDir(), "coverage.json")
				if err := os.WriteFile(covFile, []byte(tt.data), 0o644); err != nil {
					t.Fatal(err)
				}
				args := []string{covFile}
				if m != "" {
					args = []string{m, covFile}
				}
				var stdout, stderr bytes.Buffer
				code := RunWithIO(context.Background(), args, nil, &stdout, &stderr)
				if code != 2 {
					t.Fatalf("code = %d, want 2\nstderr: %s", code, stderr.String())
				}
				if !strings.Contains(stderr.String(), "coverage.json") || !strings.Contains(stderr.String(), tt.want) {
					t.Errorf("stderr = %q, want file name and %q", stderr.String(), tt.want)
				}
				if stdout.Len() > 0 {
					t.Errorf("no report expected, got %q", stdout.String())
				}
			})
		}
	}
}

func TestRun_MinEqualToCoveragePasses(t *testing.T) {
	// 57 of 100 lines: float math gives 56.99999999999999, which must not
	// fail -min=57.
	var b strings.Builder
	b.WriteString(`{"files":{"lib.star":{"lines":{`)
	for i := 1; i <= 100; i++ {
		if i > 1 {
			b.WriteString(",")
		}
		hits := 0
		if i <= 57 {
			hits = 1
		}
		fmt.Fprintf(&b, `"%d":%d`, i, hits)
	}
	b.WriteString(`}}}}`)
	covFile := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(covFile, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := RunWithIO(context.Background(), []string{"-min=57", covFile}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("code = %d, want 0\nstderr: %s", code, stderr.String())
	}
}
