package linter

import "testing"

func TestSuppressionParser_SpaceAfterComma(t *testing.T) {
	src := "# skylint: disable=a, b\nx = 1\n# skylint: disable-next-line=c ,d some reason\ny = 2\n"
	p := NewSuppressionParser([]byte(src))
	for _, rule := range []string{"a", "b"} {
		if !p.IsSuppressed(Finding{Line: 1, Rule: rule}) {
			t.Errorf("rule %q should be suppressed on line 1", rule)
		}
	}
	for _, rule := range []string{"c", "d"} {
		if !p.IsSuppressed(Finding{Line: 4, Rule: rule}) {
			t.Errorf("rule %q should be suppressed on line 4", rule)
		}
	}
	if p.IsSuppressed(Finding{Line: 4, Rule: "some"}) {
		t.Error("trailing prose must not become a rule")
	}
}
