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
