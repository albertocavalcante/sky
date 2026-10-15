// Package tester provides snapshot testing support.
package tester

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

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

// snapshotNameReplacer maps characters that are invalid in file names on common
// operating systems to underscores. It defines the legacy snapshot file name.
var snapshotNameReplacer = strings.NewReplacer(
	"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
)

// snapshotPath returns the full path for a named snapshot.
//
// New baselines encode both the test name and the snapshot name so that
// distinct names get distinct files: "a/b" becomes "a%2Fb" and "a_b" becomes
// "a%5Fb". (The legacy scheme mapped every unsafe character to "_", so those
// two names shared one file.) The name is also length-bounded.
//
// A baseline written by an earlier scheme is still found and used, and wins
// over the new path: ignoring it would make Compare create a fresh baseline and
// pass a changed value without update mode. Limit: a legacy file shared by two
// colliding names keeps being shared until it is deleted.
func (sm *SnapshotManager) snapshotPath(name string) string {
	safeName := snapshotNameReplacer.Replace(name)
	current := filepath.Join(sm.snapshotDir(), boundSnapshotFileName(encodeFileComponent(sm.testName)+"__"+encodeSnapshotName(name)+".snap"))
	earlier := []string{
		// Raw test name, lossy snapshot name, unbounded.
		sm.legacySnapshotPath(safeName),
		// Encoded test name, lossy snapshot name, bounded.
		filepath.Join(sm.snapshotDir(), boundSnapshotFileName(encodeFileComponent(sm.testName)+"__"+safeName+".snap")),
	}
	for _, p := range earlier {
		if p == "" || p == current {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return current
}

// encodeSnapshotName encodes a snapshot name for a new baseline file. On top of
// encodeFileComponent it encodes "_": a legacy file name is built by replacing
// unsafe characters with "_", so a new file named "a_b" would be mistaken for
// the legacy baseline of "a/b" (which must win) and the two names would share a
// file again. Never emitting "_" keeps new files off every legacy path.
func encodeSnapshotName(name string) string {
	return strings.ReplaceAll(encodeFileComponent(name), "_", "%5F")
}

// legacySnapshotPath is where a baseline lived before names were encoded. It
// returns "" when the raw test name would resolve outside the snapshot
// directory: encoding exists to prevent that, so such a path is never used.
func (sm *SnapshotManager) legacySnapshotPath(safeName string) string {
	dir := sm.snapshotDir()
	path := filepath.Join(dir, sm.testName+"__"+safeName+".snap")
	if rel, err := filepath.Rel(dir, path); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return path
}

// Compare compares a value against its snapshot.
// Returns nil if they match, or an error describing the mismatch.
// If UpdateMode is true, updates the snapshot instead of comparing.
func (sm *SnapshotManager) Compare(value starlark.Value, name string) error {
	serialized, err := serializeChecked(value)
	if err != nil {
		return fmt.Errorf("cannot snapshot %q: %w", name, err)
	}
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
//
// A value that cannot be serialized (a reference cycle, or nesting deeper than
// maxSnapshotDepth) yields a placeholder describing the problem; Compare reports
// it as an error instead.
func SerializeStarlarkValue(v starlark.Value) string {
	out, err := serializeChecked(v)
	if err != nil {
		return "<unserializable: " + err.Error() + ">"
	}
	return out
}

// maxSnapshotDepth bounds container nesting so that a pathologically deep (not
// cyclic) value cannot exhaust the Go stack, which no Starlark timeout can
// interrupt.
const maxSnapshotDepth = 1000

// serializeChecked serializes v, failing on reference cycles and excessive depth.
func serializeChecked(v starlark.Value) (string, error) {
	s := &serializer{active: make(map[starlark.Value]struct{})}
	out := s.serialize(v, 0)
	if s.err != nil {
		return "", s.err
	}
	return out, nil
}

// serializer tracks the containers on the current path to detect cycles.
type serializer struct {
	active map[starlark.Value]struct{}
	depth  int
	err    error
}

func (s *serializer) serialize(v starlark.Value, indent int) string {
	if s.err != nil {
		return ""
	}
	// depth, not indent: a one-element tuple nests without indenting.
	if s.depth >= maxSnapshotDepth {
		s.err = fmt.Errorf("value is nested deeper than %d levels", maxSnapshotDepth)
		return ""
	}
	s.depth++
	defer func() { s.depth-- }()
	switch v.(type) {
	case *starlark.List, *starlark.Dict, *starlark.Set, *starlarkstruct.Struct:
		if _, ok := s.active[v]; ok {
			s.err = fmt.Errorf("value contains a reference cycle (%s)", v.Type())
			return ""
		}
		s.active[v] = struct{}{}
		defer delete(s.active, v)
	}
	return s.serializeValue(v, indent)
}

func (s *serializer) serializeValue(v starlark.Value, indent int) string {
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
			sb.WriteString(s.serialize(val.Index(i), indent+1))
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
			return "(" + s.serialize(val.Index(0), indent) + ",)"
		}
		var sb strings.Builder
		sb.WriteString("(\n")
		for i := 0; i < val.Len(); i++ {
			sb.WriteString(ind)
			sb.WriteString("  ")
			sb.WriteString(s.serialize(val.Index(i), indent+1))
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
			sb.WriteString(s.serialize(k, indent+1))
			sb.WriteString(": ")
			sb.WriteString(s.serialize(v, indent+1))
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
			sb.WriteString(s.serialize(item, indent+1))
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
				sb.WriteString(s.serialize(attrVal, indent+1))
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

// maxSnapshotFileName bounds the snapshot file name length in bytes.
const maxSnapshotFileName = 200

// encodeFileComponent percent-encodes only bytes that are unsafe in a file
// name: path separators, '%' itself, control characters and the Windows-invalid
// set. Everything else (spaces, brackets, unicode, ...) is kept as-is so
// existing baselines keep their names. A ".." component cannot escape because
// separators are encoded, but a name that is only dots is encoded too.
func encodeFileComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == 0x7f || c == '%' || c == '/' || c == '\\' ||
			c == ':' || c == '*' || c == '?' || c == '"' || c == '<' || c == '>' || c == '|' {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	out := b.String()
	if strings.Trim(out, ".") == "" {
		return strings.ReplaceAll(out, ".", "%2E")
	}
	return out
}

// boundSnapshotFileName keeps name within maxSnapshotFileName bytes by
// truncating to a readable prefix and appending a digest of the full name.
func boundSnapshotFileName(name string) string {
	if len(name) <= maxSnapshotFileName {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	suffix := "-" + hex.EncodeToString(sum[:8]) + ".snap"
	cut := maxSnapshotFileName - len(suffix)
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return name[:cut] + suffix
}
