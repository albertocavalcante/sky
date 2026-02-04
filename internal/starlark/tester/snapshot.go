// Package tester provides snapshot testing support.
package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// SnapshotManager handles reading, writing, and comparing snapshots.
type SnapshotManager struct {
	// UpdateMode when true, updates snapshots instead of comparing.
	UpdateMode bool

	// testFile is the current test file being executed.
	testFile string

	// testName is the current test function name.
	testName string

	// updates tracks snapshots that were created or updated.
	updates []string

	// mismatches tracks snapshots that didn't match.
	mismatches []SnapshotMismatch
}

// SnapshotMismatch records a snapshot comparison failure.
type SnapshotMismatch struct {
	Name     string
	Expected string
	Actual   string
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(updateMode bool) *SnapshotManager {
	return &SnapshotManager{
		UpdateMode: updateMode,
	}
}

// SetContext sets the current test context for snapshot operations.
func (sm *SnapshotManager) SetContext(testFile, testName string) {
	sm.testFile = testFile
	sm.testName = testName
}

// snapshotDir returns the directory for storing snapshots for the current test file.
func (sm *SnapshotManager) snapshotDir() string {
	// Get the directory of the test file
	dir := filepath.Dir(sm.testFile)
	// Get just the filename without extension
	base := filepath.Base(sm.testFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	// Snapshots go in __snapshots__/<testfile>/
	return filepath.Join(dir, "__snapshots__", name)
}

// snapshotPath returns the full path for a named snapshot.
func (sm *SnapshotManager) snapshotPath(name string) string {
	// Sanitize name for filesystem by replacing characters that are invalid on common operating systems.
	// This includes forward/back slashes and Windows-invalid chars: : * ? " < > |
	r := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	safeName := r.Replace(name)
	return filepath.Join(sm.snapshotDir(), sm.testName+"__"+safeName+".snap")
}

// Compare compares a value against its snapshot.
// Returns nil if they match, or an error describing the mismatch.
// If UpdateMode is true, updates the snapshot instead of comparing.
func (sm *SnapshotManager) Compare(value starlark.Value, name string) error {
	serialized := SerializeStarlarkValue(value)
	snapPath := sm.snapshotPath(name)

	// Check if snapshot exists
	existing, err := os.ReadFile(snapPath)
	if os.IsNotExist(err) {
		// No existing snapshot - create it
		if err := sm.writeSnapshot(snapPath, serialized); err != nil {
			return fmt.Errorf("failed to create snapshot %q: %w", name, err)
		}
		sm.updates = append(sm.updates, name)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read snapshot %q: %w", name, err)
	}

	// Compare with existing
	existingStr := string(existing)
	if existingStr == serialized {
		return nil // Match!
	}

	// Mismatch
	if sm.UpdateMode {
		// Update mode - write new snapshot
		if err := sm.writeSnapshot(snapPath, serialized); err != nil {
			return fmt.Errorf("failed to update snapshot %q: %w", name, err)
		}
		sm.updates = append(sm.updates, name)
		return nil
	}

	// Record mismatch
	sm.mismatches = append(sm.mismatches, SnapshotMismatch{
		Name:     name,
		Expected: existingStr,
		Actual:   serialized,
	})

	return fmt.Errorf("snapshot %q does not match:\n%s", name, formatDiff(existingStr, serialized))
}

// writeSnapshot writes a snapshot to disk, creating directories as needed.
func (sm *SnapshotManager) writeSnapshot(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// Updates returns the list of snapshots that were created or updated.
func (sm *SnapshotManager) Updates() []string {
	return sm.updates
}

// Mismatches returns the list of snapshot mismatches.
func (sm *SnapshotManager) Mismatches() []SnapshotMismatch {
	return sm.mismatches
}

// SerializeStarlarkValue converts a Starlark value to a deterministic string representation.
// This is used for snapshot comparison.
func SerializeStarlarkValue(v starlark.Value) string {
	return serializeValue(v, 0)
}

func serializeValue(v starlark.Value, indent int) string {
	ind := strings.Repeat("  ", indent)

	switch val := v.(type) {
	case starlark.NoneType:
		return "None"

	case starlark.Bool:
		if val {
			return "True"
		}
		return "False"

	case starlark.Int:
		return val.String()

	case starlark.Float:
		return fmt.Sprintf("%v", float64(val))

	case starlark.String:
		// Use Go's quoted string format for consistency
		return fmt.Sprintf("%q", string(val))

	case starlark.Bytes:
		return fmt.Sprintf("b%q", string(val))

	case *starlark.List:
		if val.Len() == 0 {
			return "[]"
		}
		var sb strings.Builder
		sb.WriteString("[\n")
		for i := 0; i < val.Len(); i++ {
			sb.WriteString(ind)
			sb.WriteString("  ")
			sb.WriteString(serializeValue(val.Index(i), indent+1))
			sb.WriteString(",\n")
		}
		sb.WriteString(ind)
		sb.WriteString("]")
		return sb.String()

	case starlark.Tuple:
		if val.Len() == 0 {
			return "()"
		}
		if val.Len() == 1 {
			return "(" + serializeValue(val.Index(0), indent) + ",)"
		}
		var sb strings.Builder
		sb.WriteString("(\n")
		for i := 0; i < val.Len(); i++ {
			sb.WriteString(ind)
			sb.WriteString("  ")
			sb.WriteString(serializeValue(val.Index(i), indent+1))
			sb.WriteString(",\n")
		}
		sb.WriteString(ind)
		sb.WriteString(")")
		return sb.String()

	case *starlark.Dict:
		if val.Len() == 0 {
			return "{}"
		}
		// Sort keys for determinism
		keys := val.Keys()
		sortStarlarkValues(keys)

		var sb strings.Builder
		sb.WriteString("{\n")
		for _, k := range keys {
			v, _, _ := val.Get(k)
			sb.WriteString(ind)
			sb.WriteString("  ")
			sb.WriteString(serializeValue(k, indent+1))
			sb.WriteString(": ")
			sb.WriteString(serializeValue(v, indent+1))
			sb.WriteString(",\n")
		}
		sb.WriteString(ind)
		sb.WriteString("}")
		return sb.String()

	case *starlark.Set:
		if val.Len() == 0 {
			return "set()"
		}
		// Convert to sorted list for determinism
		var items []starlark.Value
		iter := val.Iterate()
		defer iter.Done()
		var item starlark.Value
		for iter.Next(&item) {
			items = append(items, item)
		}
		sortStarlarkValues(items)

		var sb strings.Builder
		sb.WriteString("set([\n")
		for _, item := range items {
			sb.WriteString(ind)
			sb.WriteString("  ")
			sb.WriteString(serializeValue(item, indent+1))
			sb.WriteString(",\n")
		}
		sb.WriteString(ind)
		sb.WriteString("])")
		return sb.String()

	case *starlarkstruct.Struct:
		var sb strings.Builder
		sb.WriteString("struct(\n")

		// Get all attribute names
		if an, ok := v.(starlark.HasAttrs); ok {
			fields := an.AttrNames()
			sort.Strings(fields)
			for _, name := range fields {
				attrVal, err := an.Attr(name)
				if err != nil {
					continue
				}
				sb.WriteString(ind)
				sb.WriteString("  ")
				sb.WriteString(name)
				sb.WriteString(" = ")
				sb.WriteString(serializeValue(attrVal, indent+1))
				sb.WriteString(",\n")
			}
		}
		sb.WriteString(ind)
		sb.WriteString(")")
		return sb.String()

	default:
		// For other types, use their string representation
		return fmt.Sprintf("<%s: %s>", v.Type(), v.String())
	}
}

// sortStarlarkValues sorts a slice of Starlark values for deterministic output.
// It uses a stable sorting strategy: first by type, then by canonical comparison
// for comparable types, falling back to string representation.
func sortStarlarkValues(values []starlark.Value) {
	sort.Slice(values, func(i, j int) bool {
		v1, v2 := values[i], values[j]
		t1, t2 := v1.Type(), v2.Type()

		// 1. Different types: sort by type name for determinism
		if t1 != t2 {
			return t1 < t2
		}

		// 2. Same type: try canonical comparison using starlark.Compare
		cmp, err := starlark.Compare(syntax.LT, v1, v2)
		if err == nil {
			return cmp
		}

		// 3. Fallback to string representation for uncomparable types
		return v1.String() < v2.String()
	})
}

// formatDiff creates a unified diff between expected and actual strings.
func formatDiff(expected, actual string) string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		B:        difflib.SplitLines(actual),
		FromFile: "Expected",
		ToFile:   "Actual",
		Context:  3,
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		// Fallback to simple comparison if diff fails
		return fmt.Sprintf("--- Expected\n%s\n+++ Actual\n%s", expected, actual)
	}
	return result
}
