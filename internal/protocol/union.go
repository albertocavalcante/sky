package protocol

import (
	"bytes"
	"encoding/json"
)

//go:generate go run gen_unions.go

// unionScore reports how well x matches a struct variant with the given
// required JSON keys. It returns -1 if x is not a JSON object or lacks any
// required key, otherwise the number of required keys (higher is more
// specific).
func unionScore(x []byte, required ...string) int {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(x, &m); err != nil || m == nil {
		return -1
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return -1
		}
	}
	return len(required)
}

// keySet describes the required and optional JSON keys of one struct variant.
//
// nonNull lists required keys whose type is not nullable: a JSON null there
// (which encoding/json would silently decode to the zero value) means the
// variant does not match.
type keySet struct {
	required, optional, nonNull []string
}

// unionNullRequired reports whether x is an object that has every required
// key of some set but carries JSON null for one of that set's non-nullable
// keys.
func unionNullRequired(x []byte, sets ...keySet) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(x, &m); err != nil || m == nil {
		return false
	}
sets:
	for _, s := range sets {
		for _, k := range s.required {
			if _, ok := m[k]; !ok {
				continue sets
			}
		}
		for _, k := range s.nonNull {
			if string(bytes.TrimSpace(m[k])) == "null" {
				return true
			}
		}
	}
	return false
}

// unionRank scores x against the best matching key set. It returns -1 if no
// set matches (x is not an object or lacks a required key). Otherwise the
// rank is dominated by the number of required keys, with the number of
// optional keys present breaking ties between equally specific variants.
func unionRank(x []byte, sets ...keySet) int {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(x, &m); err != nil || m == nil {
		return -1
	}
	best := -1
sets:
	for _, s := range sets {
		for _, k := range s.required {
			if _, ok := m[k]; !ok {
				continue sets
			}
		}
		for _, k := range s.nonNull {
			if string(bytes.TrimSpace(m[k])) == "null" {
				continue sets
			}
		}
		r := len(s.required) * 1024
		for _, k := range s.optional {
			if _, ok := m[k]; ok {
				r++
			}
		}
		best = max(best, r)
	}
	return best
}
