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

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
