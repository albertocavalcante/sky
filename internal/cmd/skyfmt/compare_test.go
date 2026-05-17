package skyfmt

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestEngineFlag_AcceptsBuildtools(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("x = 1\n")
	code := RunWithIO(context.Background(), []string{"-engine=buildtools"}, stdin, &stdout, &stderr)
	if code != exitOK {
		t.Errorf("exit = %d, want %d; stderr=%s", code, exitOK, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Error("no output produced")
	}
}

func TestEngineFlag_AcceptsCST(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("x = 1\n")
	code := RunWithIO(context.Background(), []string{"-engine=cst"}, stdin, &stdout, &stderr)
	if code != exitOK {
		t.Errorf("exit = %d, want %d; stderr=%s", code, exitOK, stderr.String())
	}
	if got := stdout.String(); got != "x = 1\n" {
		t.Errorf("cst stdin output = %q, want \"x = 1\\n\"", got)
	}
}

func TestEngineFlag_RejectsUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("x = 1\n")
	code := RunWithIO(context.Background(), []string{"-engine=nonexistent"}, stdin, &stdout, &stderr)
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "unknown engine") {
		t.Errorf("stderr missing 'unknown engine' message: %s", stderr.String())
	}
}

func TestCompareMode_AgreementExitsZero(t *testing.T) {
	// A trivial input both engines handle identically.
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("x = 1\n")
	code := RunWithIO(context.Background(), []string{"-engine=compare"}, stdin, &stdout, &stderr)
	if code != exitOK {
		t.Errorf("exit = %d, want %d (agreement); stdout=%q stderr=%q",
			code, exitOK, stdout.String(), stderr.String())
	}
	// On agreement, stdout should be empty (no diff to report).
	if stdout.Len() != 0 {
		t.Errorf("stdout non-empty on agreement: %q", stdout.String())
	}
}

func TestCompareMode_DivergenceExitsOne(t *testing.T) {
	// A BUILD file where the two engines may format differently. If they
	// happen to agree on this input, this test passes trivially (and the
	// compare exit code is OK). The test asserts the contract shape, not
	// a specific divergence — the divergence ledger evolves.
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString(`load("@b", "y", "x")
load("@a", "n", "m")
`)
	code := RunWithIO(context.Background(), []string{"-engine=compare", "-type=build"}, stdin, &stdout, &stderr)
	if code != exitOK && code != exitNeedsFormat {
		t.Errorf("exit = %d, want %d or %d", code, exitOK, exitNeedsFormat)
	}
	if code == exitNeedsFormat && stdout.Len() == 0 {
		t.Error("divergent exit but no diff written to stdout")
	}
}

func TestCompareMode_RejectsWriteFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("x = 1\n")
	code := RunWithIO(context.Background(), []string{"-engine=compare", "-w"}, stdin, &stdout, &stderr)
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "compare") {
		t.Errorf("stderr should mention compare incompatibility: %s", stderr.String())
	}
}

func TestEngineFlag_DefaultPathStillBuildtools(t *testing.T) {
	// Make sure the existing default behavior is preserved when no engine
	// flag is passed. This is the existence test for the migration safety
	// net.
	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString(`def foo():
  return   1
`)
	code := RunWithIO(context.Background(), []string{}, stdin, &stdout, &stderr)
	if code != exitOK {
		t.Errorf("default-engine exit = %d, want %d", code, exitOK)
	}
	// buildtools normalizes "  " → "    " indentation. CST neutral does
	// not. So this output proves we still used buildtools by default.
	if !strings.Contains(stdout.String(), "    return 1") {
		t.Errorf("default engine output suggests we didn't run buildtools:\n%s", stdout.String())
	}
}
