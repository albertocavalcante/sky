// Package sortutil provides common sorting utilities for the starlark tooling.
//
// These helpers use Go 1.21+ slices.SortFunc and cmp packages for cleaner,
// more efficient sorting of common types.
package sortutil

import (
	"cmp"
	"slices"
)

// ByName sorts a slice of elements using a function that extracts the name.
func ByName[S ~[]E, E any](s S, getName func(E) string) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Compare(getName(a), getName(b))
	})
}

// ByLocation sorts elements by file path, then line, then column.
// This is the most common sorting pattern for findings and diagnostics.
func ByLocation[S ~[]E, E any](s S, getPath func(E) string, getLine func(E) int, getCol func(E) int) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Or(
			cmp.Compare(getPath(a), getPath(b)),
			cmp.Compare(getLine(a), getLine(b)),
			cmp.Compare(getCol(a), getCol(b)),
		)
	})
}

// ByLineColumn sorts elements by line, then column (for same-file sorting).
func ByLineColumn[S ~[]E, E any](s S, getLine func(E) int, getCol func(E) int) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Or(
			cmp.Compare(getLine(a), getLine(b)),
			cmp.Compare(getCol(a), getCol(b)),
		)
	})
}

// ByFileLineName sorts elements by file, then line, then name.
// Used for query result items.
func ByFileLineName[S ~[]E, E any](s S, getFile func(E) string, getLine func(E) int, getName func(E) string) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Or(
			cmp.Compare(getFile(a), getFile(b)),
			cmp.Compare(getLine(a), getLine(b)),
			cmp.Compare(getName(a), getName(b)),
		)
	})
}

// ByFileLine sorts elements by file, then line.
func ByFileLine[S ~[]E, E any](s S, getFile func(E) string, getLine func(E) int) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Or(
			cmp.Compare(getFile(a), getFile(b)),
			cmp.Compare(getLine(a), getLine(b)),
		)
	})
}

// Asc sorts elements by an integer field in ascending order.
func Asc[S ~[]E, E any](s S, getValue func(E) int) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Compare(getValue(a), getValue(b))
	})
}

// Desc sorts elements by an integer field in descending order.
func Desc[S ~[]E, E any](s S, getValue func(E) int) {
	slices.SortFunc(s, func(a, b E) int {
		return cmp.Compare(getValue(b), getValue(a))
	})
}
