package sortutil_test

import (
	"testing"

	"github.com/albertocavalcante/sky/internal/starlark/sortutil"
)

type testItem struct {
	name   string
	file   string
	line   int
	column int
}

func TestByName(t *testing.T) {
	items := []testItem{
		{name: "charlie"},
		{name: "alpha"},
		{name: "bravo"},
	}

	sortutil.ByName(items, func(i testItem) string { return i.name })

	want := []string{"alpha", "bravo", "charlie"}
	for i, item := range items {
		if item.name != want[i] {
			t.Errorf("items[%d].name = %q, want %q", i, item.name, want[i])
		}
	}
}

func TestByLocation(t *testing.T) {
	items := []testItem{
		{file: "b.star", line: 10, column: 5},
		{file: "a.star", line: 5, column: 10},
		{file: "a.star", line: 5, column: 3},
		{file: "a.star", line: 3, column: 1},
	}

	sortutil.ByLocation(items,
		func(i testItem) string { return i.file },
		func(i testItem) int { return i.line },
		func(i testItem) int { return i.column },
	)

	// Should be sorted: a.star:3:1, a.star:5:3, a.star:5:10, b.star:10:5
	expected := []struct {
		file   string
		line   int
		column int
	}{
		{"a.star", 3, 1},
		{"a.star", 5, 3},
		{"a.star", 5, 10},
		{"b.star", 10, 5},
	}

	for i, item := range items {
		if item.file != expected[i].file || item.line != expected[i].line || item.column != expected[i].column {
			t.Errorf("items[%d] = {%s, %d, %d}, want {%s, %d, %d}",
				i, item.file, item.line, item.column,
				expected[i].file, expected[i].line, expected[i].column)
		}
	}
}

func TestByLineColumn(t *testing.T) {
	items := []testItem{
		{line: 10, column: 5},
		{line: 5, column: 10},
		{line: 5, column: 3},
	}

	sortutil.ByLineColumn(items,
		func(i testItem) int { return i.line },
		func(i testItem) int { return i.column },
	)

	// Should be sorted: 5:3, 5:10, 10:5
	expected := []struct {
		line   int
		column int
	}{
		{5, 3},
		{5, 10},
		{10, 5},
	}

	for i, item := range items {
		if item.line != expected[i].line || item.column != expected[i].column {
			t.Errorf("items[%d] = {%d, %d}, want {%d, %d}",
				i, item.line, item.column, expected[i].line, expected[i].column)
		}
	}
}

func TestByFileLineName(t *testing.T) {
	items := []testItem{
		{file: "a.star", line: 5, name: "zeta"},
		{file: "a.star", line: 5, name: "alpha"},
		{file: "a.star", line: 3, name: "beta"},
	}

	sortutil.ByFileLineName(items,
		func(i testItem) string { return i.file },
		func(i testItem) int { return i.line },
		func(i testItem) string { return i.name },
	)

	// Should be sorted: a.star:3:beta, a.star:5:alpha, a.star:5:zeta
	expected := []struct {
		line int
		name string
	}{
		{3, "beta"},
		{5, "alpha"},
		{5, "zeta"},
	}

	for i, item := range items {
		if item.line != expected[i].line || item.name != expected[i].name {
			t.Errorf("items[%d] = {%d, %s}, want {%d, %s}",
				i, item.line, item.name, expected[i].line, expected[i].name)
		}
	}
}

func TestAsc(t *testing.T) {
	items := []testItem{
		{line: 30},
		{line: 10},
		{line: 20},
	}

	sortutil.Asc(items, func(i testItem) int { return i.line })

	want := []int{10, 20, 30}
	for i, item := range items {
		if item.line != want[i] {
			t.Errorf("items[%d].line = %d, want %d", i, item.line, want[i])
		}
	}
}

func TestDesc(t *testing.T) {
	items := []testItem{
		{line: 10},
		{line: 30},
		{line: 20},
	}

	sortutil.Desc(items, func(i testItem) int { return i.line })

	want := []int{30, 20, 10}
	for i, item := range items {
		if item.line != want[i] {
			t.Errorf("items[%d].line = %d, want %d", i, item.line, want[i])
		}
	}
}

func TestByFileLine(t *testing.T) {
	items := []testItem{
		{file: "b.star", line: 5},
		{file: "a.star", line: 10},
		{file: "a.star", line: 5},
	}

	sortutil.ByFileLine(items,
		func(i testItem) string { return i.file },
		func(i testItem) int { return i.line },
	)

	expected := []struct {
		file string
		line int
	}{
		{"a.star", 5},
		{"a.star", 10},
		{"b.star", 5},
	}

	for i, item := range items {
		if item.file != expected[i].file || item.line != expected[i].line {
			t.Errorf("items[%d] = {%s, %d}, want {%s, %d}",
				i, item.file, item.line, expected[i].file, expected[i].line)
		}
	}
}

// TestWithPointers verifies sorting works with pointer slices
func TestWithPointers(t *testing.T) {
	items := []*testItem{
		{name: "charlie"},
		{name: "alpha"},
		{name: "bravo"},
	}

	sortutil.ByName(items, func(i *testItem) string { return i.name })

	want := []string{"alpha", "bravo", "charlie"}
	for i, item := range items {
		if item.name != want[i] {
			t.Errorf("items[%d].name = %q, want %q", i, item.name, want[i])
		}
	}
}
