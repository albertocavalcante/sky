// Package tooldeps keeps Bazel-only analyzer deps in go.mod.
package tooldeps

import (
	_ "github.com/kisielk/errcheck/errcheck"
	_ "github.com/timakin/bodyclose/passes/bodyclose"
	_ "golang.org/x/tools/go/analysis/passes/nilness"
	_ "golang.org/x/tools/go/analysis/passes/unusedwrite"
)
