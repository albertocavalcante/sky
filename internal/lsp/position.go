package lsp

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// LSP positions use UTF-16 code units for the character offset. The helpers
// below convert between that encoding and Go byte offsets / rune columns.

// utf16Width returns the number of UTF-16 code units needed for r.
func utf16Width(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// utf16Len returns the length of s in UTF-16 code units.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16Width(r)
	}
	return n
}

// utf16ColToByte converts a UTF-16 column within line to a byte offset.
// Columns past the end of the line clamp to len(line); a column that falls
// inside a surrogate pair rounds down to the start of the character.
func utf16ColToByte(line string, col int) int {
	if col <= 0 {
		return 0
	}
	units := 0
	for i, r := range line {
		w := utf16Width(r)
		if units+w > col {
			return i
		}
		units += w
		if units == col {
			return i + utf8.RuneLen(r)
		}
	}
	return len(line)
}

// byteToUTF16Col converts a byte offset within line to a UTF-16 column.
func byteToUTF16Col(line string, off int) int {
	if off > len(line) {
		off = len(line)
	}
	if off <= 0 {
		return 0
	}
	return utf16Len(line[:off])
}

// runeColToUTF16 converts a 0-based rune column within line (as used by
// buildtools positions, once decremented) to a UTF-16 column.
func runeColToUTF16(line string, runeCol int) int {
	if runeCol <= 0 {
		return 0
	}
	units := 0
	n := 0
	for _, r := range line {
		if n >= runeCol {
			break
		}
		units += utf16Width(r)
		n++
	}
	// Columns past the end of the line keep counting one unit per rune.
	return units + (runeCol - n)
}

// lineText returns the text of the given 0-based line (without the newline).
func lineText(content string, line int) (string, bool) {
	if line < 0 {
		return "", false
	}
	start := 0
	for i := 0; i < line; i++ {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			return "", false
		}
		start += idx + 1
	}
	end := strings.IndexByte(content[start:], '\n')
	if end < 0 {
		return content[start:], true
	}
	return content[start : start+end], true
}

// lineOf returns the text of the given 0-based line, or "" if it does not exist.
func lineOf(content string, line int) string {
	t, _ := lineText(content, line)
	return strings.TrimSuffix(t, "\r")
}

// positionToOffset converts an LSP position to a byte offset in content.
// Out-of-range lines clamp to len(content); characters clamp to the line end.
func positionToOffset(content string, line, char int) int {
	if line < 0 {
		return 0
	}
	start := 0
	for i := 0; i < line; i++ {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			return len(content)
		}
		start += idx + 1
	}
	end := strings.IndexByte(content[start:], '\n')
	if end < 0 {
		end = len(content) - start
	}
	l := content[start : start+end]
	return start + utf16ColToByte(strings.TrimSuffix(l, "\r"), char)
}

// offsetToPosition converts a byte offset in content to a 0-based line and
// UTF-16 column.
func offsetToPosition(content string, offset int) (line, col uint32) {
	if offset < 0 {
		return 0, 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	lineStart := 0
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return line, uint32(utf16Len(content[lineStart:offset]))
}

// --- URI <-> path ---

// uriToPath converts a document URI to a file path. file:// URIs are
// percent-decoded; a leading "/C:" drive prefix is normalized to "C:".
// Non-file inputs are returned unchanged.
func uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "file://")
	}
	p := u.Path
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' && isASCIILetter(p[1]) {
		p = p[1:]
	} else if u.Host != "" && u.Host != "localhost" {
		p = "//" + u.Host + p // UNC path
	}
	return filepath.FromSlash(p)
}

// pathToURI converts a file path to a file:// URI, percent-encoding as needed.
func pathToURI(path string) string {
	p := path
	// Backslashes are separators only for Windows-style paths (drive letter
	// or UNC); on Unix they are literal filename characters and get
	// percent-encoded by url.URL.
	if runtime.GOOS == "windows" || isWindowsStylePath(path) {
		p = strings.ReplaceAll(path, "\\", "/")
	}
	if len(p) >= 2 && p[1] == ':' && isASCIILetter(p[0]) {
		p = "/" + p
	}
	if strings.HasPrefix(p, "//") {
		// UNC: //host/share/...
		host, rest, _ := strings.Cut(p[2:], "/")
		return (&url.URL{Scheme: "file", Host: host, Path: "/" + rest}).String()
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func isWindowsStylePath(p string) bool {
	return (len(p) >= 2 && p[1] == ':' && isASCIILetter(p[0])) || strings.HasPrefix(p, `\\`)
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
