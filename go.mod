module github.com/albertocavalcante/sky

go 1.24.6

require (
	github.com/BurntSushi/toml v1.6.0
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
	golang.org/x/term v0.40.0
	golang.org/x/tools v0.42.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/gostaticanalysis/analysisutil v0.7.1 // indirect
	github.com/gostaticanalysis/comment v1.4.2 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
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
