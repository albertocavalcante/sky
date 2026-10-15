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

// A plain reassignment writes the variable; it must not count as a read.
func TestChecker_ReassignedButNeverReadIsUnused(t *testing.T) {
	srcs := map[string]string{
		"simple":   "def f():\n    x = 1\n    x = 2\n",
		"tuple":    "def f():\n    x = 1\n    x, y = 2, 3\n    return y\n",
		"loop var": "def f(xs):\n    x = 1\n    for x in xs:\n        pass\n",
	}
	for name, src := range srcs {
		t.Run(name, func(t *testing.T) {
			var found bool
			for _, d := range checkSrc(t, src) {
				if d.Code == "unused" && d.Message == `local variable "x" is assigned but never used` {
					found = true
				}
			}
			if !found {
				t.Errorf("expected unused diagnostic for x in:\n%s", src)
			}
		})
	}
}

// Any read of the name still counts, including augmented assignment and
// subscript stores, which read the variable first.
func TestChecker_ReassignedAndReadNotUnused(t *testing.T) {
	srcs := []string{
		"def f():\n    x = 1\n    x = 2\n    return x\n",
		"def f():\n    x = 1\n    x += 1\n",
		"def f():\n    x = []\n    x[0] = 1\n",
		"def f():\n    x = 1\n    x = x + 1\n    return x\n",
	}
	for _, src := range srcs {
		for _, d := range checkSrc(t, src) {
			if d.Code == "unused" {
				t.Errorf("unexpected unused %v in:\n%s", d, src)
			}
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
