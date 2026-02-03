# Sky - Starlark Toolchain
# Run `just --list` to see available recipes

# Default recipe - show help
default:
    @just --list

# Build all CLI tools
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
# Cross-compilation targets
# ============================================================================

# Output directory for binaries
dist_dir := "dist"

# Build sky (minimal) for current platform
build-sky:
    go build -o {{dist_dir}}/sky ./cmd/sky

# Build sky_full (embedded) for current platform
build-sky-full:
    go build -tags=sky_full -o {{dist_dir}}/sky_full ./cmd/sky

# Build sky_full for all supported platforms
build-all: build-linux-amd64 build-linux-arm64 build-darwin-arm64 build-windows-amd64
    @echo "Built all platforms in {{dist_dir}}/"

# Linux AMD64
build-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -tags=sky_full -o {{dist_dir}}/sky-linux-amd64 ./cmd/sky

# Linux ARM64
build-linux-arm64:
    GOOS=linux GOARCH=arm64 go build -tags=sky_full -o {{dist_dir}}/sky-linux-arm64 ./cmd/sky

# macOS ARM64 (Apple Silicon)
build-darwin-arm64:
    GOOS=darwin GOARCH=arm64 go build -tags=sky_full -o {{dist_dir}}/sky-darwin-arm64 ./cmd/sky

# Windows AMD64
build-windows-amd64:
    GOOS=windows GOARCH=amd64 go build -tags=sky_full -o {{dist_dir}}/sky-windows-amd64.exe ./cmd/sky

# Build minimal sky for all platforms
build-all-minimal: build-linux-amd64-minimal build-linux-arm64-minimal build-darwin-arm64-minimal build-windows-amd64-minimal
    @echo "Built all minimal platforms in {{dist_dir}}/"

build-linux-amd64-minimal:
    GOOS=linux GOARCH=amd64 go build -o {{dist_dir}}/sky-minimal-linux-amd64 ./cmd/sky

build-linux-arm64-minimal:
    GOOS=linux GOARCH=arm64 go build -o {{dist_dir}}/sky-minimal-linux-arm64 ./cmd/sky

build-darwin-arm64-minimal:
    GOOS=darwin GOARCH=arm64 go build -o {{dist_dir}}/sky-minimal-darwin-arm64 ./cmd/sky

build-windows-amd64-minimal:
    GOOS=windows GOARCH=amd64 go build -o {{dist_dir}}/sky-minimal-windows-amd64.exe ./cmd/sky

# Clean dist directory
clean-dist:
    rm -rf {{dist_dir}}
