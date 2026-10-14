package docgen

import (
	"bytes"
	"strings"
	"testing"
)

func TestToAnchorKeepsUnderscores(t *testing.T) {
	if got := toAnchor("My_Func"); got != "my_func" {
		t.Errorf("toAnchor = %q, want my_func", got)
	}
}

func TestParseDocstring_TypedArgs(t *testing.T) {
	p := ParseDocstring("Summary.\n\nArgs:\n    name (str): the name\n    count (int, optional): how many\n")
	if p.Args["name"] != "the name" {
		t.Errorf("Args = %v", p.Args)
	}
	if p.Args["count"] != "how many" {
		t.Errorf("Args = %v", p.Args)
	}
}

func TestParseDocstring_ExtraSections(t *testing.T) {
	p := ParseDocstring("Summary.\n\nWarning:\n    careful\n\nTodo:\n    fix it\n\nSee Also:\n    other_fn\n\nDeprecated:\n    use new_fn\n")
	if p.Warning != "careful" || p.Todo != "fix it" || p.SeeAlso != "other_fn" || p.Deprecated != "use new_fn" {
		t.Errorf("sections dropped: %+v", p)
	}
}

func TestExtractFile_ReassignedGlobalsNotDuplicated(t *testing.T) {
	src := "X = 1\nX += 2\nX = 3\nY = []\nY += [1]\n"
	doc, err := ExtractFile("a.bzl", []byte(src), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Globals) != 2 {
		t.Fatalf("got %d globals, want 2: %+v", len(doc.Globals), doc.Globals)
	}
}

func TestRenderMarkdown_UndeclaredArgsDeterministic(t *testing.T) {
	src := "def f():\n    \"\"\"Sum.\n\n    Args:\n        d: d\n        c: c\n        b: b\n        a: a\n    \"\"\"\n    pass\n"
	doc, err := ExtractFile("a.bzl", []byte(src), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var first string
	for i := 0; i < 20; i++ {
		var buf bytes.Buffer
		if err := RenderMarkdown(&buf, doc, DefaultMarkdownOptions()); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = buf.String()
			if strings.Index(first, "`a`") > strings.Index(first, "`b`") {
				t.Errorf("args not sorted:\n%s", first)
			}
		} else if buf.String() != first {
			t.Fatal("nondeterministic output")
		}
	}
}
