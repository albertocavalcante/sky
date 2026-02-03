package query

import (
	"testing"
)

func TestUnion(t *testing.T) {
	tests := []struct {
		name      string
		a         *Result
		b         *Result
		wantCount int
	}{
		{
			name:      "both nil",
			a:         nil,
			b:         nil,
			wantCount: 0,
		},
		{
			name: "a nil",
			a:    nil,
			b: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			wantCount: 1,
		},
		{
			name: "b nil",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b:         nil,
			wantCount: 1,
		},
		{
			name: "no overlap",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
			}},
			wantCount: 2,
		},
		{
			name: "with overlap",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
				{Type: "file", Name: "c.bzl", File: "c.bzl", Line: 1},
			}},
			wantCount: 3,
		},
		{
			name: "identical",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Union(tt.a, tt.b)
			if result == nil {
				if tt.wantCount != 0 {
					t.Errorf("Union() = nil, want %d items", tt.wantCount)
				}
				return
			}
			if len(result.Items) != tt.wantCount {
				t.Errorf("Union() got %d items, want %d", len(result.Items), tt.wantCount)
			}
		})
	}
}

func TestDifference(t *testing.T) {
	tests := []struct {
		name      string
		a         *Result
		b         *Result
		wantCount int
		wantNames []string
	}{
		{
			name:      "a nil",
			a:         nil,
			b:         &Result{Items: []Item{{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1}}},
			wantCount: 0,
		},
		{
			name:      "b nil",
			a:         &Result{Items: []Item{{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1}}},
			b:         nil,
			wantCount: 1,
		},
		{
			name: "no overlap",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
			}},
			wantCount: 1,
			wantNames: []string{"a.bzl"},
		},
		{
			name: "with overlap",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
				{Type: "file", Name: "c.bzl", File: "c.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
			}},
			wantCount: 2,
			wantNames: []string{"a.bzl", "c.bzl"},
		},
		{
			name: "identical",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Difference(tt.a, tt.b)
			if len(result.Items) != tt.wantCount {
				t.Errorf("Difference() got %d items, want %d", len(result.Items), tt.wantCount)
			}

			// Check expected names
			for _, wantName := range tt.wantNames {
				found := false
				for _, item := range result.Items {
					if item.Name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Difference() missing expected item %q", wantName)
				}
			}
		})
	}
}

func TestIntersection(t *testing.T) {
	tests := []struct {
		name      string
		a         *Result
		b         *Result
		wantCount int
		wantNames []string
	}{
		{
			name:      "a nil",
			a:         nil,
			b:         &Result{Items: []Item{{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1}}},
			wantCount: 0,
		},
		{
			name:      "b nil",
			a:         &Result{Items: []Item{{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1}}},
			b:         nil,
			wantCount: 0,
		},
		{
			name: "no overlap",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
			}},
			wantCount: 0,
		},
		{
			name: "with overlap",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "b.bzl", File: "b.bzl", Line: 1},
				{Type: "file", Name: "c.bzl", File: "c.bzl", Line: 1},
			}},
			wantCount: 1,
			wantNames: []string{"b.bzl"},
		},
		{
			name: "identical",
			a: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			b: &Result{Items: []Item{
				{Type: "file", Name: "a.bzl", File: "a.bzl", Line: 1},
			}},
			wantCount: 1,
			wantNames: []string{"a.bzl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Intersection(tt.a, tt.b)
			if len(result.Items) != tt.wantCount {
				t.Errorf("Intersection() got %d items, want %d", len(result.Items), tt.wantCount)
			}

			// Check expected names
			for _, wantName := range tt.wantNames {
				found := false
				for _, item := range result.Items {
					if item.Name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Intersection() missing expected item %q", wantName)
				}
			}
		})
	}
}

func TestUnion_Deduplication(t *testing.T) {
	// Test that duplicate items are properly deduplicated
	a := &Result{Items: []Item{
		{Type: "def", Name: "func1", File: "a.bzl", Line: 1},
		{Type: "def", Name: "func1", File: "a.bzl", Line: 1}, // duplicate
	}}
	b := &Result{Items: []Item{
		{Type: "def", Name: "func1", File: "a.bzl", Line: 1}, // same as in a
		{Type: "def", Name: "func2", File: "b.bzl", Line: 5},
	}}

	result := Union(a, b)
	if len(result.Items) != 2 {
		t.Errorf("Union() got %d items, want 2 (func1, func2)", len(result.Items))
		for _, item := range result.Items {
			t.Logf("  item: %s in %s:%d", item.Name, item.File, item.Line)
		}
	}
}

func TestIntersection_Deduplication(t *testing.T) {
	// Test that duplicate items in intersection are properly handled
	a := &Result{Items: []Item{
		{Type: "def", Name: "func1", File: "a.bzl", Line: 1},
		{Type: "def", Name: "func1", File: "a.bzl", Line: 1}, // duplicate
	}}
	b := &Result{Items: []Item{
		{Type: "def", Name: "func1", File: "a.bzl", Line: 1},
	}}

	result := Intersection(a, b)
	if len(result.Items) != 1 {
		t.Errorf("Intersection() got %d items, want 1", len(result.Items))
	}
}
