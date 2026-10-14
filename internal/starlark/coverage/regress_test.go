package coverage

import (
	"bytes"
	"strings"
	"testing"
)

func TestRegisterLineCountsMissingLines(t *testing.T) {
	c := NewCollector()
	for _, l := range []int{2, 5, 6, 7, 8} {
		c.RegisterLine("a.star", l)
	}
	c.RegisterLine("a.star", 2) // idempotent
	c.BeforeExec("a.star", 2)
	c.RegisterLine("a.star", 2) // must not reset hits
	c.BeforeExec("a.star", 5)
	rep := c.Report()
	if rep.TotalLines != 5 || rep.CoveredLines != 2 {
		t.Fatalf("total=%d covered=%d, want 5/2", rep.TotalLines, rep.CoveredLines)
	}
	if rep.Percentage() >= 100 {
		t.Fatalf("percentage = %v, want < 100", rep.Percentage())
	}
	var buf bytes.Buffer
	if err := (&TextReporter{Verbose: true, ShowMissing: true}).Report(&buf, rep); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "6-8") {
		t.Errorf("missing lines not reported: %s", buf.String())
	}
}

func TestReportersDeterministic(t *testing.T) {
	c := NewCollector()
	for _, d := range []string{"z", "a", "m", "b"} {
		c.EnterFunction(d+"/f.star", "fn", 1)
		c.EnterFunction(d+"/f.star", "b", 2)
		c.EnterFunction(d+"/f.star", "a", 3)
		c.BeforeExec(d+"/f.star", 1)
	}
	rep := c.Report()
	var first [2]string
	for i := 0; i < 20; i++ {
		var lcov, cob bytes.Buffer
		if err := (&LCOVReporter{}).Report(&lcov, rep); err != nil {
			t.Fatal(err)
		}
		if err := (&CoberturaReporter{}).Report(&cob, rep); err != nil {
			t.Fatal(err)
		}
		// Cobertura embeds a timestamp; strip it.
		s := cob.String()
		if j := strings.Index(s, "timestamp="); j >= 0 {
			k := strings.Index(s[j+11:], "\"")
			s = s[:j] + s[j+11+k+1:]
		}
		if i == 0 {
			first = [2]string{lcov.String(), s}
			continue
		}
		if lcov.String() != first[0] || s != first[1] {
			t.Fatal("reporter output is not deterministic")
		}
	}
}
