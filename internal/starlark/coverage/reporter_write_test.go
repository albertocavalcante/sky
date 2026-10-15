package coverage

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

// shortWriter accepts one byte fewer than offered and reports no error.
type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

// failAfter succeeds for the first n bytes and then reports a full disk.
type failAfter struct {
	n   int
	err error
}

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, f.err
	}
	if len(p) > f.n {
		n := f.n
		f.n = 0
		return n, f.err
	}
	f.n -= len(p)
	return len(p), nil
}

func writeTestReport() *Report {
	c := NewCollector()
	c.RegisterLine("a.star", 1)
	c.RegisterLine("a.star", 2)
	c.BeforeExec("a.star", 1)
	return c.Report()
}

func TestReportersReturnWriteErrors(t *testing.T) {
	reporters := map[string]Reporter{
		"text":      &TextReporter{Verbose: true, ShowMissing: true},
		"json":      &JSONReporter{},
		"json-pret": &JSONReporter{Pretty: true},
		"cobertura": &CoberturaReporter{},
		"html":      &HTMLReporter{},
		"lcov":      &LCOVReporter{},
	}
	diskFull := errors.New("disk full")
	for name, r := range reporters {
		t.Run(name, func(t *testing.T) {
			if err := r.Report(failingWriter{err: diskFull}, writeTestReport()); !errors.Is(err, diskFull) {
				t.Errorf("failing writer: error = %v, want %v", err, diskFull)
			}
			if err := r.Report(shortWriter{}, writeTestReport()); !errors.Is(err, io.ErrShortWrite) {
				t.Errorf("short writer: error = %v, want io.ErrShortWrite", err)
			}
			// A failure after some output was already written (the truncated
			// file case) must still surface.
			if err := r.Report(&failAfter{n: 20, err: diskFull}, writeTestReport()); !errors.Is(err, diskFull) {
				t.Errorf("late failure: error = %v, want %v", err, diskFull)
			}
			var sb strings.Builder
			if err := r.Report(&sb, writeTestReport()); err != nil {
				t.Errorf("healthy writer: %v", err)
			}
		})
	}
}
