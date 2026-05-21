# PLAN — WASM distribution for skyls (experimental, secondary)

**Status:** Draft. Exploratory. Not blocking any other work.
**Owner:** @albertocavalcante
**Tracks:** how skyls binaries reach end users.

## The question

Can `skyls` ship as a single `.wasm` artifact that runs on any
platform with a WASI runtime, instead of N native binaries (one
per OS × arch)? The JVM / Node model — one artifact, per-platform
runtime — is appealing.

## Short answer

**Yes, it's feasible. No, it shouldn't be the _primary_ distribution.**
Ship native binaries via goreleaser as the default; add WASM as an
_optional secondary artifact_ for browser-native editors,
no-install workflows, and forward-compat with WASI Preview 2.

## Why WASM is feasible

Since Go 1.21, `GOOS=wasip1 GOARCH=wasm` produces a portable
`.wasm` runnable under any WASI Preview 1 runtime (`wasmtime`,
`wasmer`, `wasmedge`, `wasmtime-cli` bundled in some editors).

What skyls needs at runtime fits WASI cleanly:

| LSP runtime need                   | WASI Preview 1 status    |
| ---------------------------------- | ------------------------ |
| stdio (JSON-RPC over stdin/stdout) | ✅ native                |
| read source files                  | ✅ via preopened dirs    |
| read workspace tree (recursive)    | ✅ via preopened dirs    |
| concurrency (goroutines)           | ✅ since Go 1.21         |
| process spawn                      | not needed for skyls     |
| TCP/UDP                            | not needed for stdio LSP |

A typical Go LSP compiles cleanly to wasip1 with zero source
changes. Hot-glue: `wasmtime --dir=. skyls.wasm` and you have a
working LSP.

## Why it shouldn't be the _primary_ distribution

### Performance

Go's WASI codegen is roughly **2-3x slower than native** for CPU-
bound work (parsing, tree walks). For an LSP doing keystroke-rate
incremental reparse, this matters:

- Today: native incremental reparse, ~20µs for a 1-byte edit in a
  1000-stmt file (level-1 in `incremental.Driver`).
- Under WASM: ~60µs for the same edit. Still well under the
  perceptible-latency threshold (~100ms), BUT P99 on large files
  (5000+ stmts) starts approaching it.

For sky's typical Bazel use case (BUILD files: 100-300 lines,
MODULE.bazel: <100 lines), neither path is anywhere near
user-perceptible. The penalty is invisible.

For larger files (generated .bzl, `bazel_dep`-heavy MODULE.bazel
in big monorepos), WASM may start showing edge-case latency.
Native is the safe default.

### Binary size

Go's WASM output is large because the Go runtime is bundled:

- skyls native (darwin/arm64): ~25 MB (estimated)
- skyls WASM: probably **50-80 MB**

The native distribution has the advantage that each platform
binary is single-purpose. WASM ships ONE binary but it's
substantially bigger.

This is a wash: total bandwidth-to-distribute is similar, but
the per-platform native binary is what most users actually need.

### Editor support reality

The "one artifact" benefit assumes the _editor_ has a WASM
runtime available:

| Editor                   | WASM-LSP support                                               |
| ------------------------ | -------------------------------------------------------------- |
| VS Code                  | Yes via `vscode-wasm` extension (Microsoft-supported, growing) |
| VS Code Web (vscode.dev) | Native WASM runtime; this is where WASM-LSP shines             |
| Neovim                   | Possible via plugin; not standard                              |
| Emacs                    | No native support; would need wrapper                          |
| Helix                    | No native support                                              |
| Zed                      | No native support                                              |

So WASM-LSP works _great_ for browser-based editors and VS Code
desktop with the wasm extension, and is awkward-to-impossible
elsewhere. Native LSPs work _everywhere_ with the standard
`command:` config.

### Tooling maturity

WASI Preview 1 is the current stable target. WASI Preview 2 (the
component model) is the actual future — better resource isolation,
better interface types, smaller binaries. Go's Preview 2 support
is in progress but not stable as of 2026-05.

Picking Preview 1 today means re-doing work when Preview 2 lands.
Defer until tooling settles.

## Recommended path

### Phase 0 (now) — defer

Don't ship WASM yet. Sky's LSP distribution story isn't broken;
this is a "should we" question with no urgency.

### Phase 1 — native binaries via goreleaser (when needed)

When sky needs a distribution story beyond `go install`, ship via
goreleaser. Standard 6-platform matrix:

```yaml
# .goreleaser.yml (sketch)
builds:
  - id: skyls
    main: ./cmd/skyls
    binary: skyls
    goos: [darwin, linux, windows]
    goarch: [amd64, arm64]
    env: [CGO_ENABLED=0]
```

5 minutes of CI per release, well-tooled. Standard expectation for
LSPs.

### Phase 2 — WASM as optional secondary artifact (experimental)

Once Phase 1 ships, add a single experimental WASM target:

```yaml
- id: skyls-wasm
  main: ./cmd/skyls
  binary: skyls.wasm
  goos: [wasip1]
  goarch: [wasm]
  env: [GOOS=wasip1, GOARCH=wasm]
```

Document the invocation in `docs/wasm.md`:

```bash
wasmtime run --dir=. ./skyls.wasm  # macOS / Linux
```

Plus a VS Code Web integration recipe for editors that bundle a
WASM runtime.

**Don't make this the default.** Keep native primary. Document the
WASM artifact as "experimental, for browser-native dev workflows".

### Phase 3 — re-evaluate when WASI P2 lands

When Go has stable WASI Preview 2 support AND mainstream editors
bundle WASM runtimes by default, revisit whether WASM should be
the default distribution path. That world is probably 2-3 years
out.

## What this plan does NOT do

- Make sky's _currently-bundled_ tools (`skyfmt`, `skylint`, etc.)
  WASM. Same logic applies but no immediate need.
- Replace Go's existing cross-compilation. Native binaries are
  the conventional, well-tooled path.
- Build a wasm-binary-fetcher integration in VS Code extension.
  Out of scope until Phase 2 lands.

## Open questions

1. Does the `vscode-wasm` extension API expose a stdio-style LSP
   handle, or does it require custom protocol shims? (Last I
   checked it had stdio support but verify.)
2. Are there any Go stdlib bits skyls uses that don't compile under
   wasip1? Common gotchas: `os/exec` (not supported), `net`
   (limited), `syscall` (limited). Sky's LSP shouldn't hit these
   but verify with a compile-only test.
3. The `wasmtime --dir=.` permission model is coarse. For a multi-
   workspace LSP setup, would per-workspace `--dir` flags be
   awkward in editor configs?

## Acceptance for "we're done"

- Native binaries shipping via goreleaser (Phase 1).
- An experimental WASM artifact alongside that documents how to
  run + which editors it works with (Phase 2).
- A periodic check-in (yearly) on WASI Preview 2 / editor runtime
  bundling status; flip default if/when the ecosystem catches up.
