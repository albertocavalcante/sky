package ci

import (
	"bytes"
	"strings"
	"testing"
)

func TestInvalidReportShapesExitWithError(t *testing.T) {
	failing := `{"passed":0,"failed":1,"total":1,"files":1,"duration_ms":0,"results":[{"file":"a.star","tests":[{"name":"bad","passed":false}]}]}`
	tests := []struct{ name, in string }{
		{"empty object", `{}`},
		{"null", `null`},
		{"null results", `{"passed":0,"failed":0,"total":0,"results":null}`},
		{"missing results", `{"passed":0,"failed":0,"total":0}`},
		{"array", `[]`},
		{"empty input", ``},
		{"second report after empty object", `{} ` + failing},
		{"second report after valid one", `{"passed":1,"failed":0,"total":1,"results":[]}` + "\n" + failing},
		{"trailing garbage", failing + ` x`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o, e bytes.Buffer
			code := Run([]string{"--system=generic"}, strings.NewReader(tt.in), &o, &e)
			if code != exitError {
				t.Fatalf("code=%d want %d; stderr=%s", code, exitError, e.String())
			}
			if !strings.Contains(e.String(), "sky-ci: reading input:") {
				t.Errorf("stderr lacks clear message: %q", e.String())
			}
		})
	}
}

func TestEmptyResultsArrayStaysValid(t *testing.T) {
	var o, e bytes.Buffer
	in := `{"passed":0,"failed":0,"total":0,"files":0,"duration_ms":0,"results":[]}` + "\n\n"
	if code := Run([]string{"--system=generic"}, strings.NewReader(in), &o, &e); code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, e.String())
	}
}
