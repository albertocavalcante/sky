package linter

import (
	"testing"
)

func TestApplyFixes(t *testing.T) {
	tests := []struct {
		name    string
		content string
		fixes   []*Replacement
		want    string
		applied int
		skipped int
	}{
		{
			name:    "no fixes",
			content: "hello world",
			fixes:   nil,
			want:    "hello world",
			applied: 0,
			skipped: 0,
		},
		{
			name:    "single fix",
			content: "hello world",
			fixes: []*Replacement{
				{Content: "there", Start: 6, End: 11},
			},
			want:    "hello there",
			applied: 1,
			skipped: 0,
		},
		{
			name:    "multiple non-overlapping fixes",
			content: "aaa bbb ccc",
			fixes: []*Replacement{
				{Content: "AAA", Start: 0, End: 3},
				{Content: "CCC", Start: 8, End: 11},
			},
			want:    "AAA bbb CCC",
			applied: 2,
			skipped: 0,
		},
		{
			name:    "overlapping fixes - second skipped",
			content: "hello world",
			fixes: []*Replacement{
				{Content: "HELLO", Start: 0, End: 5},
				{Content: "lo wo", Start: 3, End: 8}, // overlaps with first
			},
			want:    "HELLO world",
			applied: 1,
			skipped: 1,
		},
		{
			name:    "insert at position",
			content: "hello world",
			fixes: []*Replacement{
				{Content: " beautiful", Start: 5, End: 5}, // insert, not replace
			},
			want:    "hello beautiful world",
			applied: 1,
			skipped: 0,
		},
		{
			name:    "delete text",
			content: "hello beautiful world",
			fixes: []*Replacement{
				{Content: "", Start: 5, End: 15}, // delete " beautiful"
			},
			want:    "hello world",
			applied: 1,
			skipped: 0,
		},
		{
			name:    "nil fix in list",
			content: "hello world",
			fixes: []*Replacement{
				nil,
				{Content: "there", Start: 6, End: 11},
			},
			want:    "hello there",
			applied: 1,
			skipped: 0,
		},
		{
			name:    "invalid fix bounds",
			content: "hello",
			fixes: []*Replacement{
				{Content: "x", Start: -1, End: 2},  // invalid start
				{Content: "y", Start: 0, End: 100}, // end beyond content
			},
			want:    "hello",
			applied: 0,
			skipped: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, applied, skipped := ApplyFixes([]byte(tt.content), tt.fixes)
			if string(got) != tt.want {
				t.Errorf("ApplyFixes() content = %q, want %q", string(got), tt.want)
			}
			if applied != tt.applied {
				t.Errorf("ApplyFixes() applied = %d, want %d", applied, tt.applied)
			}
			if skipped != tt.skipped {
				t.Errorf("ApplyFixes() skipped = %d, want %d", skipped, tt.skipped)
			}
		})
	}
}

func TestFixResult_Diff(t *testing.T) {
	result := FixResult{
		Path:            "test.star",
		OriginalContent: []byte("hello world\n"),
		FixedContent:    []byte("hello there\n"),
		AppliedFixes:    1,
	}

	diff := result.Diff()
	if diff == "" {
		t.Error("Diff() returned empty string for changed content")
	}

	// Check that diff contains expected markers
	if !containsString(diff, "---") || !containsString(diff, "+++") {
		t.Error("Diff() should contain unified diff markers")
	}
	if !containsString(diff, "-hello world") || !containsString(diff, "+hello there") {
		t.Error("Diff() should show the actual changes")
	}
}

func TestFixResult_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		original string
		fixed    string
		want     bool
	}{
		{
			name:     "no changes",
			original: "hello",
			fixed:    "hello",
			want:     false,
		},
		{
			name:     "has changes",
			original: "hello",
			fixed:    "world",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FixResult{
				OriginalContent: []byte(tt.original),
				FixedContent:    []byte(tt.fixed),
			}
			if got := r.HasChanges(); got != tt.want {
				t.Errorf("HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFixableCount(t *testing.T) {
	findings := []Finding{
		{Message: "no fix"},
		{Message: "has fix", Replacement: &Replacement{Content: "x", Start: 0, End: 1}},
		{Message: "also no fix"},
		{Message: "another fix", Replacement: &Replacement{Content: "y", Start: 2, End: 3}},
	}

	if got := FixableCount(findings); got != 2 {
		t.Errorf("FixableCount() = %d, want 2", got)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
