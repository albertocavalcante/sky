package tester

import (
	"strings"
	"testing"
)

func TestRunnerBasic(t *testing.T) {
	src := []byte(`
def test_addition():
    assert.eq(1 + 1, 2)

def test_string():
    assert.eq("hello" + " world", "hello world")

def helper_not_a_test():
    pass
`)

	runner := New(DefaultOptions())
	result, err := runner.RunFile("test.star", src)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	if len(result.Tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(result.Tests))
	}

	passed, failed := result.Summary()
	if passed != 2 {
		t.Errorf("expected 2 passed, got %d", passed)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
}

func TestRunnerFailingTest(t *testing.T) {
	src := []byte(`
def test_will_fail():
    assert.eq(1, 2, "numbers should match")
`)

	runner := New(DefaultOptions())
	result, err := runner.RunFile("test.star", src)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	if len(result.Tests) != 1 {
		t.Errorf("expected 1 test, got %d", len(result.Tests))
	}

	if result.Tests[0].Passed {
		t.Error("expected test to fail")
	}

	if result.Tests[0].Error == nil {
		t.Error("expected error message")
	}
}

func TestRunnerSetupTeardown(t *testing.T) {
	// Note: Setup/teardown functions run but cannot modify frozen globals.
	// They're useful for actions that don't require mutable state,
	// or for future enhancements where we might support mutable test context.
	src := []byte(`
def setup():
    # Setup runs before each test
    pass

def teardown():
    # Teardown runs after each test
    pass

def test_first():
    assert.eq(1 + 1, 2)

def test_second():
    assert.eq("a" + "b", "ab")
`)

	runner := New(DefaultOptions())
	result, err := runner.RunFile("test.star", src)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	if len(result.Tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(result.Tests))
	}

	// Both tests should pass
	passed, failed := result.Summary()
	if passed != 2 {
		t.Errorf("expected 2 passed, got %d (failed: %d)", passed, failed)
		for _, test := range result.Tests {
			if !test.Passed {
				t.Logf("  %s failed: %v", test.Name, test.Error)
			}
		}
	}
}

func TestRunnerCustomPrefix(t *testing.T) {
	src := []byte(`
def Test_uppercase():
    assert.true(True)

def test_lowercase():
    assert.true(True)
`)

	// Test with Go-style prefix
	opts := DefaultOptions()
	opts.TestPrefix = "Test_"
	runner := New(opts)

	result, err := runner.RunFile("test.star", src)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	if len(result.Tests) != 1 {
		t.Errorf("expected 1 test with Test_ prefix, got %d", len(result.Tests))
	}

	if result.Tests[0].Name != "Test_uppercase" {
		t.Errorf("expected Test_uppercase, got %s", result.Tests[0].Name)
	}
}

func TestAssertEq(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "equal ints",
			src:  `assert.eq(1, 1)`,
		},
		{
			name: "equal strings",
			src:  `assert.eq("a", "a")`,
		},
		{
			name: "equal lists",
			src:  `assert.eq([1, 2], [1, 2])`,
		},
		{
			name:    "unequal ints",
			src:     `assert.eq(1, 2)`,
			wantErr: true,
		},
		{
			name:    "custom message",
			src:     `assert.eq(1, 2, "custom error")`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fullSrc := []byte("def test_it():\n    " + tc.src)
			runner := New(DefaultOptions())
			result, err := runner.RunFile("test.star", fullSrc)
			if err != nil {
				t.Fatalf("RunFile failed: %v", err)
			}

			if tc.wantErr && result.Tests[0].Passed {
				t.Error("expected test to fail")
			}
			if !tc.wantErr && !result.Tests[0].Passed {
				t.Errorf("expected test to pass: %v", result.Tests[0].Error)
			}
		})
	}
}

func TestAssertContains(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "list contains",
			src:  `assert.contains([1, 2, 3], 2)`,
		},
		{
			name: "string contains",
			src:  `assert.contains("hello world", "world")`,
		},
		{
			name: "dict contains key",
			src:  `assert.contains({"a": 1}, "a")`,
		},
		{
			name:    "list not contains",
			src:     `assert.contains([1, 2, 3], 4)`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fullSrc := []byte("def test_it():\n    " + tc.src)
			runner := New(DefaultOptions())
			result, err := runner.RunFile("test.star", fullSrc)
			if err != nil {
				t.Fatalf("RunFile failed: %v", err)
			}

			if tc.wantErr && result.Tests[0].Passed {
				t.Error("expected test to fail")
			}
			if !tc.wantErr && !result.Tests[0].Passed {
				t.Errorf("expected test to pass: %v", result.Tests[0].Error)
			}
		})
	}
}

func TestAssertFails(t *testing.T) {
	src := []byte(`
def failing_func():
    fail("expected error")

def test_fails():
    assert.fails(failing_func)

def test_fails_with_pattern():
    assert.fails(failing_func, "expected")
`)

	runner := New(DefaultOptions())
	result, err := runner.RunFile("test.star", src)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	passed, _ := result.Summary()
	if passed != 2 {
		t.Errorf("expected 2 passed, got %d", passed)
		for _, test := range result.Tests {
			if !test.Passed {
				t.Logf("  %s: %v", test.Name, test.Error)
			}
		}
	}
}

func TestDiscoverTests(t *testing.T) {
	src := []byte(`
def test_a():
    pass

def test_b():
    pass

def helper():
    pass

def Test_c():
    pass
`)

	tests, err := DiscoverTests("test.star", src, "test_")
	if err != nil {
		t.Fatalf("DiscoverTests failed: %v", err)
	}

	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d: %v", len(tests), tests)
	}

	// Should be sorted
	if tests[0] != "test_a" || tests[1] != "test_b" {
		t.Errorf("unexpected tests: %v", tests)
	}
}

func TestDiscoveryFunctions(t *testing.T) {
	t.Run("IsTestFile", func(t *testing.T) {
		tests := []struct {
			filename string
			want     bool
		}{
			{"foo_test.star", true},
			{"test_foo.star", true},
			{"foo.star", false},
			{"test.star", false},
		}

		for _, tc := range tests {
			got := IsTestFile(tc.filename, nil)
			if got != tc.want {
				t.Errorf("IsTestFile(%q) = %v, want %v", tc.filename, got, tc.want)
			}
		}
	})

	t.Run("ClassifyPath", func(t *testing.T) {
		if ClassifyPath("*.star") != "glob" {
			t.Error("expected glob classification")
		}
		if ClassifyPath("file.star") != "file" {
			t.Error("expected file classification")
		}
	})
}

func TestTextReporter(t *testing.T) {
	result := &RunResult{
		Files: []FileResult{
			{
				File: "test.star",
				Tests: []TestResult{
					{Name: "test_pass", Passed: true},
					{Name: "test_fail", Passed: false, Error: &testError{msg: "assertion failed"}},
				},
			},
		},
	}

	reporter := &TextReporter{Verbose: false}
	var buf strings.Builder

	reporter.ReportFile(&buf, &result.Files[0])
	output := buf.String()

	if !strings.Contains(output, "PASS") {
		t.Error("expected PASS in output")
	}
	if !strings.Contains(output, "FAIL") {
		t.Error("expected FAIL in output")
	}
}

func TestMarkdownReporter(t *testing.T) {
	result := &RunResult{
		Files: []FileResult{
			{
				File: "math_test.star",
				Tests: []TestResult{
					{Name: "test_addition", Passed: true},
					{Name: "test_subtraction", Passed: true},
					{Name: "test_divide", Passed: false, Error: &testError{msg: "assertion failed: eq(None, 5)\n  at math_test.star:15"}},
					{Name: "test_skipped", Passed: true, Skipped: true, SkipReason: "Not implemented yet"},
				},
			},
		},
	}

	reporter := &MarkdownReporter{}
	var buf strings.Builder

	// ReportFile accumulates results
	reporter.ReportFile(&buf, &result.Files[0])

	// ReportSummary generates the markdown
	reporter.ReportSummary(&buf, result)
	output := buf.String()

	// Check header
	if !strings.Contains(output, "## \U0001F9EA Test Results") {
		t.Error("expected markdown header")
	}

	// Check summary counts
	if !strings.Contains(output, "| \u2705 Passed | 2 |") {
		t.Error("expected 2 passed tests in summary")
	}
	if !strings.Contains(output, "| \u274C Failed | 1 |") {
		t.Error("expected 1 failed test in summary")
	}
	if !strings.Contains(output, "| \u23ED\uFE0F Skipped | 1 |") {
		t.Error("expected 1 skipped test in summary")
	}

	// Check failed test details section
	if !strings.Contains(output, "### \u274C Failed Tests") {
		t.Error("expected Failed Tests section")
	}
	if !strings.Contains(output, "<details>") {
		t.Error("expected collapsible details")
	}
	if !strings.Contains(output, "math_test.star::test_divide") {
		t.Error("expected failed test name in details")
	}

	// Check skipped test section
	if !strings.Contains(output, "### \u23ED\uFE0F Skipped Tests") {
		t.Error("expected Skipped Tests section")
	}
	if !strings.Contains(output, "Not implemented yet") {
		t.Error("expected skip reason")
	}
}

func TestMarkdownReporterNoFailures(t *testing.T) {
	result := &RunResult{
		Files: []FileResult{
			{
				File: "test.star",
				Tests: []TestResult{
					{Name: "test_pass", Passed: true},
				},
			},
		},
	}

	reporter := &MarkdownReporter{}
	var buf strings.Builder

	reporter.ReportFile(&buf, &result.Files[0])
	reporter.ReportSummary(&buf, result)
	output := buf.String()

	// Should not have failed tests section
	if strings.Contains(output, "### \u274C Failed Tests") {
		t.Error("should not have Failed Tests section when no failures")
	}

	// Should not have skipped tests section
	if strings.Contains(output, "### \u23ED\uFE0F Skipped Tests") {
		t.Error("should not have Skipped Tests section when no skips")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestGitHubReporter(t *testing.T) {
	result := &RunResult{
		Files: []FileResult{
			{
				File: "math_test.star",
				Tests: []TestResult{
					{Name: "test_addition", Passed: true},
					{Name: "test_subtraction", Passed: true},
					{Name: "test_divide", Passed: false, Error: &testError{msg: "math_test.star:15:5: assertion failed: eq(None, 5)"}},
					{Name: "test_skipped", Passed: true, Skipped: true, SkipReason: "Not implemented yet"},
				},
			},
		},
	}

	reporter := &GitHubReporter{}
	var buf strings.Builder

	// ReportFile outputs workflow commands
	reporter.ReportFile(&buf, &result.Files[0])
	output := buf.String()

	// Check group commands
	if !strings.Contains(output, "::group::📁 math_test.star") {
		t.Error("expected ::group:: command for file")
	}
	if !strings.Contains(output, "::endgroup::") {
		t.Error("expected ::endgroup:: command")
	}

	// Check error annotation for failed test
	if !strings.Contains(output, "::error file=math_test.star,line=15,title=Test Failed::") {
		t.Error("expected ::error annotation with file and line")
	}

	// Check notice for skipped test
	if !strings.Contains(output, "::notice file=math_test.star,title=Skipped::test_skipped") {
		t.Error("expected ::notice annotation for skipped test")
	}

	// Check standard output lines
	if !strings.Contains(output, "PASS  test_addition") {
		t.Error("expected PASS output")
	}
	if !strings.Contains(output, "FAIL  test_divide") {
		t.Error("expected FAIL output")
	}
	if !strings.Contains(output, "SKIP  test_skipped") {
		t.Error("expected SKIP output")
	}

	// Test summary
	buf.Reset()
	reporter.ReportSummary(&buf, result)
	summary := buf.String()

	if !strings.Contains(summary, "❌") {
		t.Error("expected failure indicator in summary")
	}
	if !strings.Contains(summary, "1/3 tests failed") {
		t.Error("expected failure count in summary")
	}
}

func TestGitHubReporterAllPassed(t *testing.T) {
	result := &RunResult{
		Files: []FileResult{
			{
				File: "test.star",
				Tests: []TestResult{
					{Name: "test_pass", Passed: true},
					{Name: "test_pass2", Passed: true},
				},
			},
		},
	}

	reporter := &GitHubReporter{}
	var buf strings.Builder

	reporter.ReportSummary(&buf, result)
	summary := buf.String()

	if !strings.Contains(summary, "✅") {
		t.Error("expected success indicator in summary")
	}
	if !strings.Contains(summary, "2 tests passed") {
		t.Error("expected pass count in summary")
	}
}

func TestGitHubReporterXPass(t *testing.T) {
	result := &RunResult{
		Files: []FileResult{
			{
				File: "test.star",
				Tests: []TestResult{
					{Name: "test_xpass", Passed: false, XFail: true, XPass: true},
				},
			},
		},
	}

	reporter := &GitHubReporter{}
	var buf strings.Builder

	reporter.ReportFile(&buf, &result.Files[0])
	output := buf.String()

	// XPASS should generate an error annotation
	if !strings.Contains(output, "::error") {
		t.Error("expected ::error annotation for XPASS")
	}
	if !strings.Contains(output, "Unexpected Pass") {
		t.Error("expected 'Unexpected Pass' in XPASS error")
	}
	if !strings.Contains(output, "XPASS") {
		t.Error("expected XPASS output")
	}
}

func TestParseErrorLocation(t *testing.T) {
	tests := []struct {
		name         string
		defaultFile  string
		errMsg       string
		expectedFile string
		expectedLine int
	}{
		{
			name:         "starlark format with col",
			defaultFile:  "test.star",
			errMsg:       "test.star:15:5: assertion failed",
			expectedFile: "test.star",
			expectedLine: 15,
		},
		{
			name:         "starlark format without col",
			defaultFile:  "test.star",
			errMsg:       "test.star:42: in test_foo",
			expectedFile: "test.star",
			expectedLine: 42,
		},
		{
			name:         "at suffix format",
			defaultFile:  "default.star",
			errMsg:       "assertion failed: eq(1, 2)\n  at other.star:10",
			expectedFile: "other.star",
			expectedLine: 10,
		},
		{
			name:         "no location info",
			defaultFile:  "test.star",
			errMsg:       "some error without location",
			expectedFile: "test.star",
			expectedLine: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, line := parseErrorLocation(tc.defaultFile, &testError{msg: tc.errMsg})
			if file != tc.expectedFile {
				t.Errorf("expected file %q, got %q", tc.expectedFile, file)
			}
			if line != tc.expectedLine {
				t.Errorf("expected line %d, got %d", tc.expectedLine, line)
			}
		})
	}
}

func TestEscapeGitHubMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"line1\nline2", "line1%0Aline2"},
		{"with%percent", "with%25percent"},
		{"with\r\n", "with%0D%0A"},
	}

	for _, tc := range tests {
		got := escapeGitHubMessage(tc.input)
		if got != tc.expected {
			t.Errorf("escapeGitHubMessage(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestEscapeGitHubValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"file:line", "file%3Aline"},
		{"a,b,c", "a%2Cb%2Cc"},
		{"path/to/file.star", "path/to/file.star"},
	}

	for _, tc := range tests {
		got := escapeGitHubValue(tc.input)
		if got != tc.expected {
			t.Errorf("escapeGitHubValue(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
