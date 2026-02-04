# Sky - Starlark Toolchain
# Run `just --list` to see available recipes

# Default recipe - show help
default:
    @just --list

# ============================================================================
# Development
# ============================================================================

# Build all CLI tools (via Bazel)
build:
    bazel build //cmd/...

# Run all tests
test:
    bazel test //...

# Run linter (nogo via bazel build)
lint:
    bazel build //...

# Format all Go files
format:
    gofmt -w $(find . -name '*.go' -not -path './vendor/*' -not -path './bazel-*')

# Update BUILD.bazel files via Gazelle
gazelle:
    bazel run //:gazelle

# Tidy go modules
tidy:
    bazel run @rules_go//go -- mod tidy

# Run format + lint + test (CI check)
check: format lint test

# Install git hooks via lefthook
hooks:
    lefthook install

# Run pre-commit hooks manually
pre-commit:
    lefthook run pre-commit

# Build a specific tool (e.g., just tool skylint)
tool name:
    bazel build //cmd/{{name}}

# Run a specific tool (e.g., just run skylint -- --help)
run name *args:
    bazel run //cmd/{{name}} -- {{args}}

# Clean bazel cache
clean:
    bazel clean

# Deep clean (expunge bazel cache)
clean-all:
    bazel clean --expunge

# ============================================================================
# Distribution - Go Build (fast, no Bazel required)
# ============================================================================

# Output directory for binaries
dist_dir := "dist"

# Build sky (minimal) for current platform using Go
dist-sky:
    @mkdir -p {{dist_dir}}
    go build -o {{dist_dir}}/sky ./cmd/sky

# Build sky_full (embedded) for current platform using Go
dist-sky-full:
    @mkdir -p {{dist_dir}}
    go build -tags=sky_full -o {{dist_dir}}/sky_full ./cmd/sky

# Build skyls (LSP server) for current platform using Go
dist-skyls:
    @mkdir -p {{dist_dir}}
    go build -o {{dist_dir}}/skyls ./cmd/skyls

# Build sky_full for all supported platforms using Go
dist-all: _dist-go-linux-amd64 _dist-go-linux-arm64 _dist-go-darwin-arm64 _dist-go-windows-amd64
    @echo "Built all platforms in {{dist_dir}}/"
    @ls -lh {{dist_dir}}/

# Build minimal sky for all platforms using Go
dist-all-minimal: _dist-go-minimal-linux-amd64 _dist-go-minimal-linux-arm64 _dist-go-minimal-darwin-arm64 _dist-go-minimal-windows-amd64
    @echo "Built all minimal platforms in {{dist_dir}}/"
    @ls -lh {{dist_dir}}/

# Helper recipes for Go cross-compilation (full)
_dist-go-linux-amd64:
    @mkdir -p {{dist_dir}}
    GOOS=linux GOARCH=amd64 go build -tags=sky_full -o {{dist_dir}}/sky-linux-amd64 ./cmd/sky

_dist-go-linux-arm64:
    @mkdir -p {{dist_dir}}
    GOOS=linux GOARCH=arm64 go build -tags=sky_full -o {{dist_dir}}/sky-linux-arm64 ./cmd/sky

_dist-go-darwin-arm64:
    @mkdir -p {{dist_dir}}
    GOOS=darwin GOARCH=arm64 go build -tags=sky_full -o {{dist_dir}}/sky-darwin-arm64 ./cmd/sky

_dist-go-windows-amd64:
    @mkdir -p {{dist_dir}}
    GOOS=windows GOARCH=amd64 go build -tags=sky_full -o {{dist_dir}}/sky-windows-amd64.exe ./cmd/sky

# Helper recipes for Go cross-compilation (minimal)
_dist-go-minimal-linux-amd64:
    @mkdir -p {{dist_dir}}
    GOOS=linux GOARCH=amd64 go build -o {{dist_dir}}/sky-minimal-linux-amd64 ./cmd/sky

_dist-go-minimal-linux-arm64:
    @mkdir -p {{dist_dir}}
    GOOS=linux GOARCH=arm64 go build -o {{dist_dir}}/sky-minimal-linux-arm64 ./cmd/sky

_dist-go-minimal-darwin-arm64:
    @mkdir -p {{dist_dir}}
    GOOS=darwin GOARCH=arm64 go build -o {{dist_dir}}/sky-minimal-darwin-arm64 ./cmd/sky

_dist-go-minimal-windows-amd64:
    @mkdir -p {{dist_dir}}
    GOOS=windows GOARCH=amd64 go build -o {{dist_dir}}/sky-minimal-windows-amd64.exe ./cmd/sky

# ============================================================================
# Distribution - Bazel Build (hermetic, cached)
# ============================================================================

# Build sky (minimal) using Bazel
bazel-sky:
    bazel build //cmd/sky:sky
    @mkdir -p {{dist_dir}}
    cp bazel-bin/cmd/sky/sky_/sky {{dist_dir}}/sky-bazel

# Build sky_full using Bazel
bazel-sky-full:
    bazel build //cmd/sky:sky_full
    @mkdir -p {{dist_dir}}
    cp bazel-bin/cmd/sky/sky_full_/sky_full {{dist_dir}}/sky_full-bazel

# Clean dist directory
clean-dist:
    rm -rf {{dist_dir}}

# ============================================================================
# Example Plugins
# ============================================================================

# Build hello-native example plugin
example-hello-native:
    cd examples/plugins/hello-native && go build -o plugin

# Build hello-wasm example plugin
example-hello-wasm:
    cd examples/plugins/hello-wasm && GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm

# Build star-counter example plugin (requires go mod tidy first)
example-star-counter:
    cd examples/plugins/star-counter && go mod tidy && go build -o plugin

# Build custom-lint example plugin (requires go mod tidy first)
example-custom-lint:
    cd examples/plugins/custom-lint && go mod tidy && go build -o plugin

# Build all example plugins
examples: example-hello-native example-star-counter example-custom-lint
    @echo "Built all native example plugins"

# Test hello-native example
test-example-hello-native:
    cd examples/plugins/hello-native && go test ./...

# Test star-counter example
test-example-star-counter:
    cd examples/plugins/star-counter && go mod tidy && go test ./...

# Test custom-lint example
test-example-custom-lint:
    cd examples/plugins/custom-lint && go mod tidy && go test ./...

# Test all example plugins
test-examples: test-example-hello-native test-example-star-counter test-example-custom-lint
    @echo "All example plugin tests passed"

# ============================================================================
# Release Management (uses tools/release)
# ============================================================================

# Show current version and latest tags
version:
    @go run ./tools/release

# Create a release candidate (e.g., just release-rc 0.1.0)
release-rc version:
    @go run ./tools/release rc {{version}}

# Create a final release (e.g., just release 0.1.0)
release version:
    @go run ./tools/release final {{version}}

# Delete a tag (e.g., just release-delete v0.1.0-rc.0)
release-delete tag:
    @go run ./tools/release delete {{tag}}
