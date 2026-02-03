// Package tester coverage hook for starlark-go-x.
//
// EXPERIMENTAL: This file contains coverage instrumentation that requires
// starlark-go-x with the OnExec callback hook. To enable:
//
//  1. Uncomment the replace directive in go.mod:
//     replace go.starlark.net => ../starlark-go-x/coverage-hooks
//
//  2. Rebuild: go build ./...
//
// When the replace is not active, this file compiles but the hook is a no-op
// because Thread.OnExec doesn't exist in upstream starlark-go.
//
// TODO(upstream): Remove this experimental scaffolding once OnExec is merged.
package tester

import (
	"go.starlark.net/starlark"
)

// setupCoverageHook configures the OnExec callback on the thread for coverage collection.
//
// EXPERIMENTAL: This only works with starlark-go-x fork.
// With upstream starlark-go, this is a no-op (OnExec field doesn't exist).
func (r *Runner) setupCoverageHook(thread *starlark.Thread) {
	if r.coverage == nil {
		return
	}

	// EXPERIMENTAL: The OnExec callback is invoked before each bytecode instruction.
	// We use PositionAt(pc) to map the program counter to source location.
	// TODO(upstream): Remove experimental note once OnExec is merged to go.starlark.net
	thread.OnExec = func(fn *starlark.Function, pc uint32) {
		pos := fn.PositionAt(pc)
		r.coverage.BeforeExec(pos.Filename(), int(pos.Line))
	}
}
