# Nogo Linting (Spec)

Sky uses `nogo` from `rules_go` as the primary Go linter. The nogo runner is
configured in `tools/nogo:sky_nogo` and registered with `go_sdk.nogo` so it
executes automatically during Bazel builds.

## Goals

- Run vet-based analyzers during Go compilation under Bazel.
- Keep lint execution aligned with Bazel targets (only files in the build graph).
- Allow opt-out for generated or noisy targets using `tags = ["no-nogo"]`.

## Configuration

- `tools/nogo:sky_nogo` sets `vet = True` to enable safe vet analyzers.
- Additional analyzers are enabled for higher coverage:
  - `nilness`
  - `unusedwrite`
  - `bodyclose`
  - `errcheck`
- The Go module includes `golang.org/x/tools` to provide the analyzers.
- Nogo runs for all Go targets in this repository by default.
  The curated list is documented in `docs/NOGO_ANALYZERS.md`.

## Usage

- `bazel build //...` runs nogo automatically.
- `make lint` wraps the Bazel build to surface nogo findings.
