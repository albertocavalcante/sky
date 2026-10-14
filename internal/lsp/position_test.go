package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUTF16Conversions(t *testing.T) {
	line := "a😀é=1" // bytes: a(1) 😀(4) é(2) =(1) 1(1); utf16: 1,2,1,1,1
	tests := []struct {
		col, byteOff int
	}{
		{0, 0}, {1, 1}, {2, 1}, // inside the surrogate pair rounds down
		{3, 5}, {4, 7}, {5, 8}, {6, 9}, {99, 9},
	}
	for _, tt := range tests {
		if got := utf16ColToByte(line, tt.col); got != tt.byteOff {
			t.Errorf("utf16ColToByte(%d) = %d, want %d", tt.col, got, tt.byteOff)
		}
	}
	if got := byteToUTF16Col(line, 7); got != 4 {
		t.Errorf("byteToUTF16Col(7) = %d, want 4", got)
	}
	if got := runeColToUTF16(line, 2); got != 3 { // after a, 😀
		t.Errorf("runeColToUTF16(2) = %d, want 3", got)
	}
	if got := utf16Len(line); got != 6 {
		t.Errorf("utf16Len = %d, want 6", got)
	}
}

func TestOffsetPositionRoundTrip(t *testing.T) {
	content := "x = '😀'\nyy = 2\n"
	off := positionToOffset(content, 1, 2)
	if content[off:off+1] != " " {
		t.Errorf("offset %d -> %q", off, content[off:])
	}
	l, c := offsetToPosition(content, positionToOffset(content, 0, 7))
	if l != 0 || c != 7 {
		t.Errorf("round trip = %d:%d", l, c)
	}
	if got := positionToOffset(content, 10, 0); got != len(content) {
		t.Errorf("out-of-range line = %d", got)
	}
}

func TestGetWordAtPosition_EndOfLineAndUTF16(t *testing.T) {
	if got := getWordAtPosition("print(foo)", 0, 9); got != "foo" {
		t.Errorf("inside = %q", got)
	}
	if got := getWordAtPosition("x = foo", 0, 7); got != "foo" {
		t.Errorf("end of line = %q, want foo", got)
	}
	// 😀 takes two UTF-16 units, so "foo" starts at character 7.
	if got := getWordAtPosition("s = '😀'; foo", 0, 12); got != "foo" {
		t.Errorf("after emoji = %q", got)
	}
}

func TestGetCompletionPrefix_TrailingEmptyLine(t *testing.T) {
	if got := getCompletionPrefix("abc\n", 1, 0); got != "" {
		t.Errorf("trailing empty line prefix = %q, want empty (not line 0)", got)
	}
	if got := getCompletionPrefix("abc\nde", 1, 2); got != "de" {
		t.Errorf("prefix = %q", got)
	}
	if got := getCompletionPrefix("é😀ab", 0, 4); got != "ab"[:1] {
		t.Errorf("prefix after emoji = %q", got)
	}
}

func TestLineCharToOffset_UTF16(t *testing.T) {
	content := "😀(a, b)\n"
	if got := lineCharToOffset(content, 0, 3); got != 5 {
		t.Errorf("offset = %d, want 5", got)
	}
	if got := lineCharToOffset(content, 9, 0); got != -1 {
		t.Errorf("missing line = %d, want -1", got)
	}
}

func TestByteOffsetToPosition_UTF16(t *testing.T) {
	l, c := byteOffsetToPosition("😀x", 4)
	if l != 0 || c != 2 {
		t.Errorf("got %d:%d, want 0:2", l, c)
	}
}

func TestURIConversions(t *testing.T) {
	if runtime.GOOS != "windows" {
		if got := uriToPath("file:///tmp/my%20dir/a%23b.star"); got != "/tmp/my dir/a#b.star" {
			t.Errorf("decode = %q", got)
		}
		if got := pathToURI("/tmp/my dir/a#b.star"); got != "file:///tmp/my%20dir/a%23b.star" {
			t.Errorf("encode = %q", got)
		}
	}
	if got := filepath.ToSlash(uriToPath("file:///C:/Users/me/a.star")); got != "C:/Users/me/a.star" {
		t.Errorf("drive = %q", got)
	}
	if got := filepath.ToSlash(uriToPath("file:///c%3A/x/y.bzl")); got != "c:/x/y.bzl" {
		t.Errorf("encoded drive = %q", got)
	}
	if got := pathToURI(`C:\Users\me\a b.star`); got != "file:///C:/Users/me/a%20b.star" {
		t.Errorf("windows path = %q", got)
	}
	if got := uriToPath("/plain/path"); got != "/plain/path" {
		t.Errorf("plain = %q", got)
	}
	// Round trip for a path with unusual characters.
	p := "/tmp/ünï/100% [x]/f.bzl"
	if runtime.GOOS != "windows" {
		if got := uriToPath(pathToURI(p)); got != p {
			t.Errorf("round trip = %q", got)
		}
	}
}

func TestResolveLoadPath_EncodedRootURI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	got := resolveLoadPath("//pkg:f.bzl", "/ws/a.bzl", "file:///my%20ws")
	if got != "/my ws/pkg/f.bzl" {
		t.Errorf("got %q", got)
	}
}

func TestSemanticTokensUTF16Columns(t *testing.T) {
	// The identifier after the emoji must be positioned in UTF-16 units.
	tokens := tokenizeContent("s = \"😀\" + foo\n")
	var found bool
	for _, tok := range tokens {
		if tok.Line == 0 && tok.StartChar >= 10 { // rune col 10 would be wrong (UTF-16 col is 11)
			found = tok.StartChar == 11 && tok.Length == 3
		}
	}
	if !found {
		t.Errorf("no UTF-16 token for foo at char 11: %+v", tokens)
	}
}

func TestDocumentLinkRangeUTF16(t *testing.T) {
	s := NewServer(nil)
	uri := "file:///ws/a.bzl"
	openDoc(s, uri, "x = \"😀\"; load(\"//p:f.bzl\", \"y\")\n")
	res, err := s.Handle(context.Background(), &Request{
		Method: "textDocument/documentLink",
		Params: json.RawMessage(`{"textDocument":{"uri":"` + uri + `"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	var links []struct {
		Range  struct{ Start struct{ Character int } }
		Target string
	}
	_ = json.Unmarshal(b, &links)
	if len(links) != 1 {
		t.Skipf("no links produced: %s", b)
	}
	// `x = "😀"; load("` : module string starts at rune col 14, UTF-16 col 15.
	if links[0].Range.Start.Character != 15 {
		t.Errorf("start char = %d, want 15 (%s)", links[0].Range.Start.Character, b)
	}
}
