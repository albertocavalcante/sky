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

func TestParseInterspersed_DoubleDash(t *testing.T) {
	cases := []struct {
		args []string
		want []string
		wasm bool
	}{
		{[]string{"--", "--help"}, []string{"--help"}, false},
		{[]string{"foo", "--", "-x", "--wasm"}, []string{"foo", "-x", "--wasm"}, false},
		{[]string{"--wasm", "foo", "--", "--wasm"}, []string{"foo", "--wasm"}, true},
		{[]string{"foo", "--wasm", "bar"}, []string{"foo", "bar"}, true},
	}
	for _, c := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		wasm := fs.Bool("wasm", false, "")
		if err := parseInterspersed(fs, c.args); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if got := fs.Args(); !reflect.DeepEqual(got, c.want) || *wasm != c.wasm {
			t.Fatalf("%v: args=%v wasm=%v, want %v %v", c.args, got, *wasm, c.want, c.wasm)
		}
	}
}

func TestPluginSearch_DoubleDashLiteralQuery(t *testing.T) {
	t.Setenv("SKY_CONFIG_DIR", t.TempDir())
	var out, errb bytes.Buffer
	code := run([]string{"plugin", "search", "--", "--help"}, &out, &errb)
	// With no marketplaces configured the search itself fails (exit 1); a
	// usage error (exit 2) would mean --help was parsed as a flag.
	if code != 1 || !strings.Contains(errb.String(), "no marketplaces") {
		t.Fatalf("exit %d, stderr=%q", code, errb.String())
	}
	if strings.Contains(errb.String(), "Usage") {
		t.Fatalf("--help after -- was treated as a flag: %q", errb.String())
	}
}
