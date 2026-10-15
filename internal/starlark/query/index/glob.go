package index

import (
	"path"
	"path/filepath"
	"strings"
)

// matchGlob reports whether the slash-separated, root-relative name matches
// pattern. Each segment is matched with path.Match semantics; a "**" segment
// matches zero or more whole path segments.
func matchGlob(pattern, name string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	name = strings.TrimPrefix(name, "./")
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

// hasRecursiveSegment reports whether pattern has a whole "**" path segment.
// A "**" embedded in a larger segment (a**.bzl) is not recursive; it behaves
// like "*" under path.Match.
func hasRecursiveSegment(pattern string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(pattern), "/") {
		if seg == "**" {
			return true
		}
	}
	return false
}

// hasGlobMeta reports whether s contains a path.Match metacharacter.
func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// matchSegments matches pattern segments against name segments. It is a DP
// over (pattern index, name index) so alternating "**" and literal segments
// cost O(len(pat)*len(name)) segment matches rather than exponential time.
func matchSegments(pat, name []string) bool {
	// ok[i][j] caches whether pat[i:] matches name[j:]; 0 unknown, 1 yes, 2 no.
	memo := make([][]uint8, len(pat)+1)
	for i := range memo {
		memo[i] = make([]uint8, len(name)+1)
	}
	var match func(i, j int) bool
	match = func(i, j int) bool {
		if m := memo[i][j]; m != 0 {
			return m == 1
		}
		var res bool
		switch {
		case i == len(pat):
			res = j == len(name)
		case pat[i] == "**":
			// Zero segments, or consume one name segment and stay on "**".
			res = match(i+1, j) || (j < len(name) && match(i, j+1))
		case j == len(name):
			res = false
		default:
			ok, err := path.Match(pat[i], name[j])
			res = err == nil && ok && match(i+1, j+1)
		}
		if res {
			memo[i][j] = 1
		} else {
			memo[i][j] = 2
		}
		return res
	}
	return match(0, 0)
}
