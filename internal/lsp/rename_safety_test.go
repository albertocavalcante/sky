package lsp

import (
	"testing"
)

func renameLines(t *testing.T, code string, line, char uint32, name string) (map[uint32]bool, error) {
	t.Helper()
	edit, err := doRename(t, code, line, char, name)
	if err != nil {
		return nil, err
	}
	lines := map[uint32]bool{}
	if edit != nil {
		for _, e := range edit.Changes["file:///rename.star"] {
			lines[e.Range.Start.Line] = true
		}
	}
	return lines, nil
}

func TestRename_ShadowedCalleeIsNotEdited(t *testing.T) {
	code := "def f(name):\n    return name\n\ndef invoke(f):\n    return f(name = 1)\n"
	lines, err := renameLines(t, code, 0, 7, "label")
	if err == nil && lines[4] {
		t.Fatal("f(name=1) inside invoke(f) resolves to the parameter f and must not be edited")
	}
	if err == nil {
		return
	}
	if re, ok := err.(*ResponseError); !ok || re.Code != CodeRequestFailed {
		t.Errorf("err = %v, want RequestFailed", err)
	}
}

func TestRename_VariadicParamHasNoKeywordLabel(t *testing.T) {
	code := "def f(**name):\n    return name[\"name\"]\n\nf(name = 1)\n"
	lines, err := renameLines(t, code, 0, 8, "label")
	if err != nil {
		t.Fatal(err)
	}
	if lines[3] {
		t.Error("f(name=1) populates **name; its label must not be edited")
	}
}

func TestRename_LambdaAndAliasAreRefused(t *testing.T) {
	for _, code := range []string{
		"f = lambda name: name\nf(name = 1)\n",
		"def f(name):\n    return name\n\ng = f\ng(name = 1)\n",
	} {
		char := uint32(11)
		if code[0] == 'd' {
			char = 6
		}
		lines, err := renameLines(t, code, 0, char, "label")
		if err == nil {
			t.Errorf("rename must be refused (or the alias call edited); got edits on %v for %q", lines, code)
			continue
		}
		if re, ok := err.(*ResponseError); !ok || re.Code != CodeRequestFailed {
			t.Errorf("err = %v, want RequestFailed", err)
		}
	}
}

func TestRename_ForeignKeywordCallsDoNotBlockParamRename(t *testing.T) {
	code := "def macro(name):\n    native.cc_library(name = name)\n    other(name = name)\n"
	lines, err := renameLines(t, code, 0, 11, "label")
	if err != nil {
		t.Fatalf("calls to unrelated callees must not block: %v", err)
	}
	if lines[1] != true || lines[2] != true {
		t.Errorf("value uses should be renamed: %v", lines)
	}
}

func TestRename_SuccessiveUnicodeRenames(t *testing.T) {
	// After renaming x to π the buffer holds π; it must be selectable again.
	code := "π = 1\nprint(π)\n"
	edit, err := doRename(t, code, 1, 7, "ω")
	if err != nil {
		t.Fatal(err)
	}
	if edit == nil || len(edit.Changes["file:///rename.star"]) != 2 {
		t.Fatalf("want 2 edits for π, got %+v", edit)
	}
	w, r := getWordAndRangeAtPosition("a = élan + 𝒙1\n", 0, 12)
	if w != "𝒙1" || r.Start.Character != 11 || r.End.Character != 14 {
		t.Errorf("astral word = %q %+v", w, r)
	}
}
