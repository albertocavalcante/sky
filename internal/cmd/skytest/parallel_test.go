package skytest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Unit Tests for parseParallelism
// ============================================================================

func TestParseParallelism(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "empty string defaults to sequential",
			input: "",
			want:  1,
		},
		{
			name:  "explicit 1 is sequential",
			input: "1",
			want:  1,
		},
		{
			name:  "auto uses CPU count",
			input: "auto",
			want:  runtime.NumCPU(),
		},
		{
			name:  "AUTO uppercase works",
			input: "AUTO",
			want:  runtime.NumCPU(),
		},
		{
			name:  "explicit number",
			input: "4",
			want:  4,
		},
		{
			name:  "large number",
			input: "16",
			want:  16,
		},
		{
			name:  "zero defaults to sequential",
			input: "0",
			want:  1,
		},
		{
			name:  "negative defaults to sequential",
			input: "-1",
			want:  1,
		},
		{
			name:  "invalid string defaults to sequential",
			input: "invalid",
			want:  1,
		},
		{
			name:  "float string defaults to sequential",
			input: "2.5",
			want:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseParallelism(tc.input)
			if got != tc.want {
				t.Errorf("parseParallelism(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// ============================================================================
// Integration Tests for Parallel Execution
// ============================================================================

func TestRun_ParallelBasic(t *testing.T) {
	dir := t.TempDir()

	// Create 3 test files
	for i := 1; i <= 3; i++ {
		file := filepath.Join(dir, "test_file"+string(rune('a'+i-1))+".star")
		content := `def test_pass():
    assert.eq(1, 1)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "2", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 2) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	// Verify all tests ran
	output := stdout.String()
	if !strings.Contains(output, "3 passed") {
		t.Errorf("expected '3 passed' in output, got:\n%s", output)
	}
}

func TestRun_ParallelAuto(t *testing.T) {
	dir := t.TempDir()

	// Create 2 test files
	for i := 1; i <= 2; i++ {
		file := filepath.Join(dir, "test_auto"+string(rune('a'+i-1))+".star")
		content := `def test_pass():
    assert.eq(1, 1)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "auto", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j auto) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	// Verify all tests ran
	output := stdout.String()
	if !strings.Contains(output, "2 passed") {
		t.Errorf("expected '2 passed' in output, got:\n%s", output)
	}
}

func TestRun_ParallelSequentialFallback(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "test_seq.star")
	content := `def test_pass():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// -j 1 should behave same as no -j flag (sequential)
	var stdout1, stderr1 bytes.Buffer
	code1 := RunWithIO(context.Background(), []string{"-j", "1", file}, nil, &stdout1, &stderr1)

	var stdout2, stderr2 bytes.Buffer
	code2 := RunWithIO(context.Background(), []string{file}, nil, &stdout2, &stderr2)

	if code1 != code2 {
		t.Errorf("exit codes differ: -j 1 = %d, no flag = %d", code1, code2)
	}

	if code1 != 0 {
		t.Errorf("expected exit code 0, got %d", code1)
	}
}

func TestRun_ParallelWithFailures(t *testing.T) {
	dir := t.TempDir()

	// File 1: passes
	file1 := filepath.Join(dir, "test_a_passes.star")
	if err := os.WriteFile(file1, []byte("def test_pass():\n    assert.eq(1, 1)"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// File 2: fails
	file2 := filepath.Join(dir, "test_b_fails.star")
	if err := os.WriteFile(file2, []byte("def test_fail():\n    assert.eq(1, 2)"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// File 3: passes
	file3 := filepath.Join(dir, "test_c_passes.star")
	if err := os.WriteFile(file3, []byte("def test_pass2():\n    assert.eq(2, 2)"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "3", dir}, nil, &stdout, &stderr)

	// Should fail because one test fails
	if code == 0 {
		t.Error("expected non-zero exit code when test fails")
	}

	// Verify results reflect both passes and failure
	output := stdout.String()
	if !strings.Contains(output, "2 passed") || !strings.Contains(output, "1 failed") {
		t.Errorf("expected '2 passed, 1 failed' in output, got:\n%s", output)
	}
}

func TestRun_ParallelFailFast(t *testing.T) {
	dir := t.TempDir()

	// Create many test files - the first one fails
	file1 := filepath.Join(dir, "test_aaa_fails.star")
	// Use a sleep to ensure this file starts first (lower alphabetical order)
	if err := os.WriteFile(file1, []byte("def test_fail():\n    assert.eq(1, 2)"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create other files that would pass but take time
	for i := 2; i <= 4; i++ {
		file := filepath.Join(dir, "test_"+string(rune('a'+i))+"_passes.star")
		// These tests do some work to simulate longer running tests
		content := `def test_pass():
    x = 0
    for i in range(100):
        x = x + 1
    assert.eq(1, 1)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "2", "--bail", dir}, nil, &stdout, &stderr)

	// Should fail
	if code == 0 {
		t.Error("expected non-zero exit code with --bail")
	}

	// With fail-fast, we should see FAIL status
	output := stdout.String()
	if !strings.Contains(output, "FAIL") {
		t.Errorf("expected FAIL in output, got:\n%s", output)
	}
}

func TestRun_ParallelOutputNotInterleaved(t *testing.T) {
	dir := t.TempDir()

	// Create test files with unique test names
	for i := 1; i <= 3; i++ {
		file := filepath.Join(dir, "test_output"+string(rune('a'+i-1))+".star")
		content := `def test_file_` + string(rune('a'+i-1)) + `():
    assert.eq(1, 1)

def test_file_` + string(rune('a'+i-1)) + `_second():
    assert.eq(2, 2)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "3", "-v", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 3 -v) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Verify output is not interleaved - each PASS should be on its own line
	// and test names should appear correctly
	output := stdout.String()
	lines := strings.Split(output, "\n")

	passCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "PASS") {
			passCount++
			// Each PASS line should contain a complete test name
			if !strings.Contains(line, "test_file_") {
				t.Errorf("malformed PASS line (possibly interleaved): %s", line)
			}
		}
	}

	if passCount != 6 {
		t.Errorf("expected 6 PASS lines, got %d", passCount)
	}
}

func TestRun_ParallelResultsAggregated(t *testing.T) {
	dir := t.TempDir()

	// Create 3 files with 2 tests each
	for i := 1; i <= 3; i++ {
		file := filepath.Join(dir, "test_agg"+string(rune('a'+i-1))+".star")
		content := `def test_one():
    assert.eq(1, 1)

def test_two():
    assert.eq(2, 2)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "auto", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j auto) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Verify summary shows correct totals
	output := stdout.String()
	// 3 files * 2 tests = 6 tests total
	if !strings.Contains(output, "6 passed") {
		t.Errorf("expected '6 passed' in summary, got:\n%s", output)
	}
	if !strings.Contains(output, "3 file") {
		t.Errorf("expected '3 file' in summary, got:\n%s", output)
	}
}

func TestRun_ParallelWithJUnit(t *testing.T) {
	dir := t.TempDir()

	for i := 1; i <= 2; i++ {
		file := filepath.Join(dir, "test_junit"+string(rune('a'+i-1))+".star")
		content := `def test_pass():
    assert.eq(1, 1)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "2", "-junit", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 2 -junit) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Verify JUnit XML is well-formed
	output := stdout.String()
	if !strings.Contains(output, "<?xml") {
		t.Error("expected XML declaration in JUnit output")
	}
	if !strings.Contains(output, "<testsuites") {
		t.Error("expected testsuites element in JUnit output")
	}
	if !strings.Contains(output, "tests=\"2\"") {
		t.Errorf("expected tests=\"2\" in JUnit output, got:\n%s", output)
	}
}

func TestRun_ParallelWithJSON(t *testing.T) {
	dir := t.TempDir()

	for i := 1; i <= 2; i++ {
		file := filepath.Join(dir, "test_json"+string(rune('a'+i-1))+".star")
		content := `def test_pass():
    assert.eq(1, 1)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "2", "-json", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 2 -json) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Verify JSON structure
	output := stdout.String()
	if !strings.Contains(output, `"passed": 2`) {
		t.Errorf("expected '\"passed\": 2' in JSON output, got:\n%s", output)
	}
	if !strings.Contains(output, `"files": 2`) {
		t.Errorf("expected '\"files\": 2' in JSON output, got:\n%s", output)
	}
}

func TestRun_ParallelSingleFile(t *testing.T) {
	// Parallel mode with single file should work correctly
	dir := t.TempDir()
	file := filepath.Join(dir, "test_single.star")
	content := `def test_one():
    assert.eq(1, 1)

def test_two():
    assert.eq(2, 2)
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "4", file}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 4 single file) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_ParallelWithColonSyntax(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "test_colon.star")
	content := `def test_one():
    assert.eq(1, 1)

def test_two():
    assert.eq(2, 2)

def test_three():
    assert.eq(3, 3)
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Use :: syntax to select specific test, with parallel mode
	code := RunWithIO(context.Background(), []string{"-j", "2", "-v", file + "::test_two"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 2 with :: syntax) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "test_two") {
		t.Error("expected test_two to run")
	}
	// test_one and test_three should NOT appear as passed
	if strings.Contains(output, "PASS") && strings.Contains(output, "test_one") {
		t.Error("test_one should not run with :: syntax")
	}
}

func TestRun_ParallelWithFilter(t *testing.T) {
	dir := t.TempDir()

	for i := 1; i <= 3; i++ {
		file := filepath.Join(dir, "test_filter"+string(rune('a'+i-1))+".star")
		content := `def test_parse_something():
    assert.eq(1, 1)

def test_other():
    assert.eq(2, 2)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "2", "-k", "parse", "-v", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 2 -k parse) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Should only run parse tests
	if !strings.Contains(output, "test_parse_something") {
		t.Error("expected test_parse_something to run")
	}
	// 3 files * 1 parse test each = 3 tests
	if !strings.Contains(output, "3 passed") {
		t.Errorf("expected '3 passed' in output, got:\n%s", output)
	}
}

func TestRun_ParallelTimingReasonable(t *testing.T) {
	// Test that parallel execution actually provides speedup
	// by comparing parallel vs sequential on slow tests
	dir := t.TempDir()

	// Create files with tests that take some time
	for i := 1; i <= 4; i++ {
		file := filepath.Join(dir, "test_timing"+string(rune('a'+i-1))+".star")
		// Each test does some work
		content := `def test_work():
    x = 0
    for i in range(10000):
        x = x + 1
    assert.eq(x, 10000)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	// Run sequentially
	var stdout1, stderr1 bytes.Buffer
	start1 := time.Now()
	RunWithIO(context.Background(), []string{"-j", "1", dir}, nil, &stdout1, &stderr1)
	seqDuration := time.Since(start1)

	// Run in parallel with 4 workers
	var stdout2, stderr2 bytes.Buffer
	start2 := time.Now()
	RunWithIO(context.Background(), []string{"-j", "4", dir}, nil, &stdout2, &stderr2)
	parDuration := time.Since(start2)

	// Parallel should be faster (with some tolerance for overhead)
	// We don't require strict speedup, just that it's not significantly slower
	t.Logf("Sequential: %v, Parallel: %v", seqDuration, parDuration)

	// Allow parallel to be up to 2x slower due to test overhead on fast machines
	// The key is that on slow tests, parallel will be faster
	if parDuration > seqDuration*2 {
		t.Errorf("Parallel (%v) significantly slower than sequential (%v)", parDuration, seqDuration)
	}
}

func TestRun_ParallelWithManyFiles(t *testing.T) {
	// This test verifies that running a larger number of files in parallel
	// completes successfully and all results are aggregated correctly.

	dir := t.TempDir()

	// We can't easily measure true concurrency from within a Starlark test,
	// but we can verify that all results are returned correctly when running many files.
	for i := 1; i <= 8; i++ {
		file := filepath.Join(dir, "test_concurrent"+string(rune('a'+i-1))+".star")
		content := `def test_concurrent():
    assert.eq(1, 1)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "4", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 4) returned %d, want 0\nstderr: %s", code, stderr.String())
	}

	// Verify all 8 tests passed
	output := stdout.String()
	if !strings.Contains(output, "8 passed") {
		t.Errorf("expected '8 passed' in output, got:\n%s", output)
	}
}

func TestRun_ParallelInvalidFlag(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_invalid.star")
	content := `def test_pass():
    assert.eq(1, 1)
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Invalid -j value should fall back to sequential
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "invalid", file}, nil, &stdout, &stderr)

	// Should still succeed (falls back to sequential)
	if code != 0 {
		t.Errorf("RunWithIO(-j invalid) returned %d, want 0\nstderr: %s", code, stderr.String())
	}
}

func TestRun_ParallelWithPrelude(t *testing.T) {
	dir := t.TempDir()

	// Create prelude with shared helper
	prelude := filepath.Join(dir, "prelude.star")
	if err := os.WriteFile(prelude, []byte(`SHARED_VALUE = 42`), 0o644); err != nil {
		t.Fatalf("failed to write prelude: %v", err)
	}

	// Create multiple test files using the prelude
	for i := 1; i <= 3; i++ {
		file := filepath.Join(dir, "test_prelude"+string(rune('a'+i-1))+".star")
		content := `def test_uses_shared():
    assert.eq(SHARED_VALUE, 42)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "3", "--prelude", prelude, dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 3 --prelude) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "3 passed") {
		t.Errorf("expected '3 passed' in output, got:\n%s", output)
	}
}

func TestRun_ParallelWithFixtures(t *testing.T) {
	dir := t.TempDir()

	// Create conftest with fixture
	conftest := filepath.Join(dir, "conftest.star")
	if err := os.WriteFile(conftest, []byte(`def fixture_test_data():
    return {"value": 123}
`), 0o644); err != nil {
		t.Fatalf("failed to write conftest: %v", err)
	}

	// Create test files using fixture
	for i := 1; i <= 3; i++ {
		file := filepath.Join(dir, "test_fixture"+string(rune('a'+i-1))+".star")
		content := `def test_uses_fixture(test_data):
    assert.eq(test_data["value"], 123)
`
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file %d: %v", i, err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-j", "3", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-j 3 with fixtures) returned %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "3 passed") {
		t.Errorf("expected '3 passed' in output, got:\n%s", output)
	}
}
