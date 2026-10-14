package protocol

import "encoding/json"

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
type keySet struct {
	required, optional []string
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
