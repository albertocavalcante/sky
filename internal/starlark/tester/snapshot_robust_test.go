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

// Shared (non-cyclic) substructure and ordinary nesting serialize exactly as before.
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

func TestCollidingSnapshotNamesGetDifferentFiles(t *testing.T) {
	dir := t.TempDir()
	newSM := func(update bool) *SnapshotManager {
		sm := NewSnapshotManager(update)
		sm.SetContext(filepath.Join(dir, "t.star"), "test_p")
		return sm
	}
	sm := newSM(true)
	if sm.snapshotPath("a/b") == sm.snapshotPath("a_b") {
		t.Fatal("a/b and a_b map to the same snapshot file")
	}
	if err := sm.Compare(starlark.String("slash"), "a/b"); err != nil {
		t.Fatal(err)
	}
	if err := sm.Compare(starlark.String("underscore"), "a_b"); err != nil {
		t.Fatal(err)
	}
	if n := countFiles(t, sm.snapshotDir()); n != 2 {
		t.Fatalf("found %d snapshot files, want 2", n)
	}
	// A plain run after -u must pass for both.
	plain := newSM(false)
	if err := plain.Compare(starlark.String("slash"), "a/b"); err != nil {
		t.Errorf("a/b: %v", err)
	}
	if err := plain.Compare(starlark.String("underscore"), "a_b"); err != nil {
		t.Errorf("a_b: %v", err)
	}
}

// Baselines written by the previous scheme (lossy snapshot name) keep winning.
func TestLegacyBaselineForNameWithSeparatorIsStillUsed(t *testing.T) {
	dir := t.TempDir()
	sm := NewSnapshotManager(false)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p")
	legacy := legacySnapshotFile(sm, "a/b")
	writeBaseline(t, legacy, `"OLD"`)

	if err := sm.Compare(starlark.String("OLD"), "a/b"); err != nil {
		t.Fatalf("legacy baseline not used: %v", err)
	}
	if err := sm.Compare(starlark.String("CHANGED"), "a/b"); err == nil {
		t.Fatal("changed value passed against a legacy baseline")
	}
	if n := countFiles(t, sm.snapshotDir()); n != 1 {
		t.Errorf("found %d snapshot files, want only the legacy baseline", n)
	}

	up := NewSnapshotManager(true)
	up.SetContext(filepath.Join(dir, "t.star"), "test_p")
	if err := up.Compare(starlark.String("NEW"), "a/b"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(legacy); !strings.Contains(string(got), "NEW") {
		t.Errorf("legacy baseline not rewritten in place: %q", got)
	}
	if n := countFiles(t, sm.snapshotDir()); n != 1 {
		t.Errorf("found %d snapshot files, want 1", n)
	}
}

// Existing baselines of ordinary snake_case names keep their file name.
func TestExistingUnderscoreNameBaselineIsStillUsed(t *testing.T) {
	sm := NewSnapshotManager(false)
	sm.SetContext(filepath.Join(t.TempDir(), "t.star"), "test_p")
	writeBaseline(t, filepath.Join(sm.snapshotDir(), "test_p__my_snap.snap"), `"OLD"`)
	if err := sm.Compare(starlark.String("OLD"), "my_snap"); err != nil {
		t.Fatal(err)
	}
	if err := sm.Compare(starlark.String("CHANGED"), "my_snap"); err == nil {
		t.Fatal("changed value passed against an existing baseline")
	}
	if n := countFiles(t, sm.snapshotDir()); n != 1 {
		t.Errorf("found %d snapshot files, want 1", n)
	}
}

// The intermediate scheme (encoded test name, lossy snapshot name) is honored too.
func TestIntermediateSchemeBaselineIsStillUsed(t *testing.T) {
	dir := t.TempDir()
	sm := NewSnapshotManager(false)
	sm.SetContext(filepath.Join(dir, "t.star"), "test_p[x/y]")
	old := filepath.Join(sm.snapshotDir(), "test_p[x%2Fy]__a_b.snap")
	writeBaseline(t, old, `"OLD"`+"\n")
	if err := sm.Compare(starlark.String("CHANGED"), "a/b"); err == nil {
		t.Fatal("changed value passed; intermediate-scheme baseline was ignored")
	}
	if n := countFiles(t, sm.snapshotDir()); n != 1 {
		t.Errorf("found %d snapshot files, want 1", n)
	}
}
