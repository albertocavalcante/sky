package main

import (
	"bytes"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestRunVersionFlags(t *testing.T) {
	for _, arg := range []string{"--version", "-v", "version"} {
		var out bytes.Buffer
		if code := run([]string{arg}, &out, io.Discard); code != 0 {
			t.Fatalf("%s: exit %d", arg, code)
		}
		if !strings.HasPrefix(out.String(), "sky ") {
			t.Fatalf("%s: output %q", arg, out.String())
		}
	}
}

func TestParseInterspersed(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("path", "", "")
	wasm := fs.Bool("wasm", false, "")
	if err := parseInterspersed(fs, []string{"foo", "--path", "X", "--wasm"}); err != nil {
		t.Fatal(err)
	}
	if *path != "X" || !*wasm {
		t.Fatalf("flags not parsed: path=%q wasm=%v", *path, *wasm)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"foo"}) {
		t.Fatalf("args = %v", got)
	}
}

func TestParseInterspersed_Error(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseInterspersed(fs, []string{"foo", "--nope"}); err == nil {
		t.Fatal("expected error")
	}
}
