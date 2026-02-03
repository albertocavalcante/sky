module github.com/albertocavalcante/sky

go 1.24.6

require (
	github.com/bazelbuild/buildtools v0.0.0-20251231073631-eb7356da6895
	github.com/gofrs/flock v0.13.0
	github.com/google/go-cmp v0.7.0
	github.com/kisielk/errcheck v1.9.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/tetratelabs/wazero v1.11.0
	github.com/timakin/bodyclose v0.0.0-20241222091800-1db5c5ca4d67
	go.starlark.net v0.0.0-20260102030733-3fee463870c9
	golang.org/x/term v0.39.0
	golang.org/x/tools v0.40.0
	google.golang.org/protobuf v1.36.11
)

// EXPERIMENTAL: Uncomment to enable starlark-go-x features (OnExec coverage, type hints).
// This replaces upstream starlark-go with our fork (trunk branch).
// Once uncommented, also uncomment the hook in internal/starlark/tester/coverage_hook.go
// TODO(upstream): Remove once features are merged to go.starlark.net
replace go.starlark.net => ../../starlark-go-x/trunk

require (
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/gostaticanalysis/analysisutil v0.7.1 // indirect
	github.com/gostaticanalysis/comment v1.4.2 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated // indirect
)
