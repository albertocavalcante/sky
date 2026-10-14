package checker

import "testing"

func checkSrc(t *testing.T, src string) []Diagnostic {
	t.Helper()
	diags, err := New(DefaultOptions()).CheckFile("test.star", []byte(src))
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}
	return diags
}

func TestChecker_ChrOrdSetDefined(t *testing.T) {
	for _, d := range checkSrc(t, "x = chr(65)\ny = ord(\"A\")\nz = set([1])\n") {
		if d.Code == "undefined" {
			t.Errorf("unexpected undefined: %v", d)
		}
	}
}

func TestChecker_ClosureCaptureNotUnused(t *testing.T) {
	src := "def f(p):\n    x = 1\n    def g():\n        return x\n    return g\n"
	for _, d := range checkSrc(t, src) {
		if d.Code == "unused" {
			t.Errorf("unexpected unused: %v", d)
		}
	}
}

func TestChecker_ParamsAndLoopVarsNotUnused(t *testing.T) {
	src := "def f(a, b=1, *args, **kw):\n    for i in range(3):\n        pass\n    return [1 for j in range(2)]\n"
	for _, d := range checkSrc(t, src) {
		if d.Code == "unused" {
			t.Errorf("unexpected unused: %v", d)
		}
	}
}
