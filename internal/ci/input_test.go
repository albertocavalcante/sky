package ci

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestInvalidReportShapesExitWithError(t *testing.T) {
	failing := `{"passed":0,"failed":1,"total":1,"files":1,"duration_ms":0,"results":[{"file":"a.star","tests":[{"name":"bad","passed":false}]}]}`
	tests := []struct{ name, in string }{
		{"empty object", `{}`},
		{"null", `null`},
		{"null results", `{"passed":0,"failed":0,"total":0,"results":null}`},
		{"missing results", `{"passed":0,"failed":0,"total":0}`},
		// Compatibility guards: the base parser already rejected these two.
		{"compat guard: array", `[]`},
		{"compat guard: empty input", ``},
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

// chunkReader returns at most n bytes per Read, like a pipe with short reads.
type chunkReader struct {
	r io.Reader
	n int
}

func (c chunkReader) Read(p []byte) (int, error) {
	if len(p) > c.n {
		p = p[:c.n]
	}
	return c.r.Read(p)
}

func TestTrailingWhitespaceIsLinear(t *testing.T) {
	const report = `{"passed":0,"failed":0,"total":0,"results":[]}`
	type res struct {
		r   *TestResults
		err error
	}
	done := make(chan res, 1)
	go func() {
		in := io.MultiReader(strings.NewReader(report), bytes.NewReader(bytes.Repeat([]byte(" "), 8<<20)))
		r, err := readResults(chunkReader{in, 512})
		done <- res{r, err}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("valid report followed by whitespace rejected: %v", got.err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("readResults did not return: trailing whitespace scan is not linear")
	}
}

// Guard (passes on the previous implementation too): finite tails with a
// non-whitespace byte stay rejected. The fail-first test is
// TestStrayByteDoesNotDrainStream.
func TestGuardStrayByteAfterReportRejected(t *testing.T) {
	const report = `{"passed":0,"failed":0,"total":0,"results":[]}`
	for _, tail := range []string{"x", "   \n\tx", " {}"} {
		in := io.MultiReader(strings.NewReader(report), strings.NewReader(tail))
		if _, err := readResults(chunkReader{in, 7}); err == nil {
			t.Errorf("tail %q accepted", tail)
		}
	}
}

// Compatibility guard (not a proof of the fix): the base parser already
// accepted a report with an empty results array and a trailing newline, and
// already rejected arrays and empty input. These must keep working/failing.
func TestCompatGuardEmptyResultsArrayStaysValid(t *testing.T) {
	var o, e bytes.Buffer
	in := `{"passed":0,"failed":0,"total":0,"files":0,"duration_ms":0,"results":[]}` + "\n\n"
	if code := Run([]string{"--system=generic"}, strings.NewReader(in), &o, &e); code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, e.String())
	}
}

// endlessAfter yields head in its first Read, then an endless stream of 'y'
// bytes, failing with errOverread once more than limit bytes were served past
// head.
type endlessAfter struct {
	head  []byte
	limit int
	past  int
}

var errOverread = errors.New("test: reader consumed past the stray byte")

func (e *endlessAfter) Read(p []byte) (int, error) {
	if len(e.head) > 0 {
		n := copy(p, e.head)
		e.head = e.head[n:]
		return n, nil
	}
	if e.past >= e.limit {
		return 0, errOverread
	}
	n := min(len(p), e.limit-e.past)
	for i := range p[:n] {
		p[i] = 'y'
	}
	e.past += n
	return n, nil
}

// Fail-first: the previous code drained the stream with io.ReadAll, so an
// open-ended stream after a stray byte was consumed (and would block forever).
func TestStrayByteDoesNotDrainStream(t *testing.T) {
	const report = `{"passed":0,"failed":0,"total":0,"results":[]}`
	src := &endlessAfter{head: []byte(report + "x"), limit: 1 << 20}
	_, err := readResults(src)
	if err == nil || !strings.Contains(err.Error(), "unexpected data") {
		t.Fatalf("err = %v, want unexpected data error", err)
	}
	if src.past > 4096 {
		t.Fatalf("consumed %d bytes past the stray byte, want <= 4096", src.past)
	}
}
