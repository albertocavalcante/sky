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
	"go.starlark.net/syntax"
)

// covFuncKey identifies a compiled function (position plus name).
type covFuncKey struct {
	pos  syntax.Position
	name string
}

// maxProbePC bounds the pc scan used to enumerate a function's line table.
//
// LIMITS: starlark-go exposes neither a function's code length nor its nested
// Funcode list (both are in internal/compile), so the scan is bounded and
// functions that are defined but never instantiated or called (nested defs,
// or lines beyond this bound) are only counted once they execute.
const maxProbePC = 1 << 13

// registerFunctionCoverage registers every line the compiler attributes to
// fn's instructions, so lines never executed count as missing. It uses the
// same pc-to-line table the OnExec hook reports from, keeping registration
// and observation consistent (the compiler attaches positions to calls,
// indexing, identifiers etc., not to statement starts).
func (r *Runner) registerFunctionCoverage(fn *starlark.Function) {
	if r.coverage == nil {
		return
	}
	// Position alone is not an identity: a module whose first statement is a
	// def shares its initializer's position with that function.
	key := covFuncKey{pos: fn.Position(), name: fn.Name()}
	r.covMu.Lock()
	if r.covSeen == nil {
		r.covSeen = make(map[covFuncKey]bool)
	}
	seen := r.covSeen[key]
	r.covSeen[key] = true
	r.covMu.Unlock()
	if seen {
		return
	}
	var last int32 = -1
	for pc := uint32(0); pc < maxProbePC; pc++ {
		pos := fn.PositionAt(pc)
		if pos.Line > 0 && pos.Line != last {
			last = pos.Line
			r.coverage.RegisterLine(pos.Filename(), int(pos.Line))
		}
	}
}

// registerGlobalsCoverage registers the lines of all functions defined in globals.
func (r *Runner) registerGlobalsCoverage(globals starlark.StringDict) {
	if r.coverage == nil {
		return
	}
	for _, v := range globals {
		if fn, ok := v.(*starlark.Function); ok {
			r.registerFunctionCoverage(fn)
		}
	}
}

// setupCoverageHook configures the OnExec callback on the thread for coverage collection.
//
// EXPERIMENTAL: This only works with starlark-go-x fork.
// With upstream starlark-go, this is a no-op (OnExec field doesn't exist).
func (r *Runner) setupCoverageHook(thread *starlark.Thread) {
	if r.coverage == nil {
		return
	}

	// The OnExec callback is invoked before each bytecode instruction.
	// We use PositionAt(pc) to map the program counter to source location.
	// TODO(upstream): Remove once OnExec is merged to go.starlark.net
	var lastFn *starlark.Function
	thread.OnExec = func(fn *starlark.Function, pc uint32) {
		if fn != lastFn {
			lastFn = fn
			r.registerFunctionCoverage(fn)
		}
		pos := fn.PositionAt(pc)
		r.coverage.BeforeExec(pos.Filename(), int(pos.Line))
	}
}
