package skyrepl

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-version"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-version) returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "skyrepl") {
		t.Errorf("version output = %q, want to contain 'skyrepl'", stdout.String())
	}
}

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-help"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Errorf("RunWithIO(-help) returned %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("help output = %q, want to contain 'Usage:'", stderr.String())
	}
}

func TestRun_EvalExpression(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		wantOut  string
		wantCode int
	}{
		{
			name:     "simple addition",
			expr:     "1 + 1",
			wantOut:  "2\n",
			wantCode: 0,
		},
		{
			name:     "string concat",
			expr:     `"hello" + " world"`,
			wantOut:  `"hello world"` + "\n",
			wantCode: 0,
		},
		{
			name:     "list comprehension",
			expr:     "[x*2 for x in [1,2,3]]",
			wantOut:  "[2, 4, 6]\n",
			wantCode: 0,
		},
		{
			name:     "None returns nothing",
			expr:     "None",
			wantOut:  "",
			wantCode: 0,
		},
		{
			name:     "print returns None",
			expr:     "print('hello')",
			wantOut:  "", // print goes to thread.Print, not stdout for Eval
			wantCode: 0,
		},
		{
			name:     "undefined variable",
			expr:     "undefined_var",
			wantOut:  "",
			wantCode: 1,
		},
		{
			name:     "syntax error",
			expr:     "1 +",
			wantOut:  "",
			wantCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-e", tc.expr}, nil, &stdout, &stderr)

			if code != tc.wantCode {
				t.Errorf("RunWithIO(-e %q) returned %d, want %d\nstderr: %s", tc.expr, code, tc.wantCode, stderr.String())
			}
			if stdout.String() != tc.wantOut {
				t.Errorf("RunWithIO(-e %q) output = %q, want %q", tc.expr, stdout.String(), tc.wantOut)
			}
		})
	}
}

func TestRun_BuiltinModules(t *testing.T) {
	// Test that json, math, time modules are available
	tests := []struct {
		name string
		expr string
	}{
		{"json module", `json.encode({"a": 1})`},
		{"math module", "math.sqrt(4)"},
		{"time module", "time.now()"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithIO(context.Background(), []string{"-e", tc.expr}, nil, &stdout, &stderr)

			if code != 0 {
				t.Errorf("RunWithIO(-e %q) returned %d, want 0\nstderr: %s", tc.expr, code, stderr.String())
			}
		})
	}
}

func TestRun_TooManyArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"file1.star", "file2.star"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Errorf("RunWithIO(file1, file2) returned %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "at most one file") {
		t.Errorf("error = %q, want to contain 'at most one file'", stderr.String())
	}
}

func TestRun_Recursion(t *testing.T) {
	// Without -recursion flag, recursion should fail
	var stdout, stderr bytes.Buffer
	code := RunWithIO(context.Background(), []string{"-e", "def f(): return f()\nf()"}, nil, &stdout, &stderr)

	// Should fail due to recursion not being allowed by default
	if code != 1 {
		t.Errorf("recursion without flag returned %d, want 1", code)
	}
}
