package tester

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// Helpers (legacySnapshotFile, writeBaseline, countFiles) live in
// snapshot_legacy_test.go.

func TestSnapshotRejectsReferenceCycles(t *testing.T) {
	list := starlark.NewList(nil)
	if err := list.Append(list); err != nil {
		t.Fatal(err)
	}
	dict := starlark.NewDict(1)
	if err := dict.SetKey(starlark.String("self"), dict); err != nil {
		t.Fatal(err)
	}
	inner := starlark.NewList(nil)
	viaTuple := starlark.Tuple{inner}
	if err := inner.Append(viaTuple); err != nil {
		t.Fatal(err)
	}

	for name, v := range map[string]starlark.Value{"list": list, "dict": dict, "tuple": viaTuple} {
		t.Run(name, func(t *testing.T) {
			sm := NewSnapshotManager(true)
			sm.SetContext(filepath.Join(t.TempDir(), "t.star"), "test_cycle")
			err := sm.Compare(v, "x")
			if err == nil || !strings.Contains(err.Error(), "cycle") {
				t.Fatalf("Compare() error = %v, want a cycle error", err)
			}
			if n := countFiles(t, sm.snapshotDir()); n != 0 {
				t.Errorf("a baseline was written for an unserializable value")
			}
		})
	}
}

func TestSnapshotRejectsExcessiveDepth(t *testing.T) {
	var v starlark.Value = starlark.MakeInt(1)
	for i := 0; i < 50000; i++ {
		v = starlark.Tuple{v}
	}
	sm := NewSnapshotManager(true)
	sm.SetContext(filepath.Join(t.TempDir(), "t.star"), "test_deep")
	err := sm.Compare(v, "x")
	if err == nil || !strings.Contains(err.Error(), "nested deeper") {
		t.Fatalf("Compare() error = %v, want a depth error", err)
	}
}

// Compatibility guard (passes on main too): shared, non-cyclic substructure and
// ordinary nesting must serialize exactly as before the cycle check.
func TestSerializationOfAcyclicValuesIsUnchanged(t *testing.T) {
	shared := starlark.NewList([]starlark.Value{starlark.MakeInt(1)})
	d := starlark.NewDict(2)
	_ = d.SetKey(starlark.String("a"), shared)
	_ = d.SetKey(starlark.String("b"), starlark.Tuple{shared, starlark.Tuple{starlark.None}})
	want := "{\n" +
		"  \"a\": [\n    1,\n  ],\n" +
		"  \"b\": (\n    [\n      1,\n    ],\n    (None,),\n  ),\n" +
		"}"
	if got := SerializeStarlarkValue(d); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCollidingSnapshotNamesAreRejected(t *testing.T) {
	for _, update := range []bool{true, false} {
		dir := t.TempDir()
		sm := NewSnapshotManager(update)
		sm.SetContext(filepath.Join(dir, "t.star"), "test_p")
		if err := sm.Compare(starlark.String("slash"), "a/b"); err != nil {
			t.Fatalf("update=%v first name: %v", update, err)
		}
		path := sm.snapshotPath("a_b")
		before, _ := os.ReadFile(path)
		err := sm.Compare(starlark.String("underscore"), "a_b")
		if err == nil {
			t.Fatalf("update=%v: colliding name accepted", update)
		}
		for _, want := range []string{`"a/b"`, `"a_b"`, filepath.Base(path)} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("update=%v: error %q does not mention %s", update, err, want)
			}
		}
		if after, _ := os.ReadFile(path); string(after) != string(before) {
			t.Errorf("update=%v: file changed by the rejected name: %q -> %q", update, before, after)
		}
	}
}

func TestSameSnapshotNameTwiceIsFine(t *testing.T) {
	sm := NewSnapshotManager(true)
	sm.SetContext(filepath.Join(t.TempDir(), "t.star"), "test_p")
	for i := 0; i < 2; i++ {
		if err := sm.Compare(starlark.String("v"), "a/b"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	// Re-running the same test in the same run is fine too.
	sm.SetContext(sm.testFile, "test_p")
	if err := sm.Compare(starlark.String("w"), "a/b"); err != nil {
		t.Fatal(err)
	}
}

func TestNonCollidingNamesAreFine(t *testing.T) {
	sm := NewSnapshotManager(true)
	sm.SetContext(filepath.Join(t.TempDir(), "t.star"), "test_p")
	for _, name := range []string{"a/b", "a/c", "x"} {
		if err := sm.Compare(starlark.String(name), name); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
	}
	if n := countFiles(t, sm.snapshotDir()); n != 3 {
		t.Errorf("found %d files, want 3", n)
	}
}

// The same snapshot name in different tests uses different files and is fine.
func TestSameNameInDifferentTestsIsFine(t *testing.T) {
	sm := NewSnapshotManager(true)
	file := filepath.Join(t.TempDir(), "t.star")
	for _, test := range []string{"test_one", "test_two"} {
		sm.SetContext(file, test)
		if err := sm.Compare(starlark.String(test), "a/b"); err != nil {
			t.Fatalf("%s: %v", test, err)
		}
	}
	if n := countFiles(t, sm.snapshotDir()); n != 2 {
		t.Errorf("found %d files, want 2", n)
	}
}
