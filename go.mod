module github.com/albertocavalcante/sky

go 1.26.2

require (
	github.com/BurntSushi/toml v1.3.2
	github.com/albertocavalcante/bazel-cst-go v0.0.0-20260518153803-61b1d053687a
	github.com/albertocavalcante/buck2-cst-go v0.0.0-20260518220630-ad34465dfc97
	github.com/albertocavalcante/starlark-cst-go v0.0.0-20260518172600-ea0a92ecbf49
	github.com/albertocavalcante/starlark-format-go v0.0.0-20260518172719-67d5fc74d85c
	github.com/bazelbuild/buildtools v0.0.0-20251231073631-eb7356da6895
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gofrs/flock v0.13.0
	github.com/google/go-cmp v0.7.0
	github.com/kisielk/errcheck v1.9.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/rogpeppe/go-internal v1.14.1
	github.com/tetratelabs/wazero v1.11.0
	github.com/timakin/bodyclose v0.0.0-20241222091800-1db5c5ca4d67
	go.starlark.net v0.0.0-20260102030733-3fee463870c9
	golang.org/x/term v0.39.0
	golang.org/x/tools v0.41.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/albertocavalcante/starlark-refactor-go v0.0.0-20260518220042-c8c6100e6f0f // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/gostaticanalysis/analysisutil v0.7.1 // indirect
	github.com/gostaticanalysis/comment v1.4.2 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated // indirect
)

// EXPERIMENTAL: Coverage instrumentation via starlark-go-x hooks.
// This replaces upstream starlark-go with our fork that has coverage hooks:
// - OnExec: line coverage
// - OnBranch: branch coverage
// - OnFunctionEnter/Exit: function coverage
// - OnIteration: loop coverage
// TODO(upstream): Remove once hooks are merged to go.starlark.net
replace go.starlark.net => github.com/albertocavalcante/starlark-go-x v0.0.0-20260203191202-da5a35fe16a6

// CST library stack — versioned via Go pseudo-versions (v0.0.0-{ts}-{hash}).
// Libraries are intentionally pre-1.0 and untagged; we pin commits via the
// pseudo-version mechanism so we can iterate without managing semver. Bump
// by running `go get github.com/albertocavalcante/<repo>@<commit-or-main>`.
//
// Local dev: gitignored go.work (see docs/PLAN-cst-library-versioning.md)
// overrides these requires with relative paths into sibling clones, so you
// can iterate across repos without publishing a new pseudo-version on every
// change.
//
// CI auth: libraries are private, so `go mod download` needs GitHub creds.
// .github/actions/setup-cst-private-auth mints a short-lived octo-sts token
// and configures GOPRIVATE + git insteadOf — no PATs in sky's secrets.
