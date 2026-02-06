# Sky - Starlark Toolchain
# Run `just --list` to see available recipes

# Default recipe - show help
default:
    @just --list

# ============================================================================
# Setup (run once after clone)
# ============================================================================

# One-time setup: install git hooks and verify tools
setup:
    @echo "Setting up development environment..."
    @command -v lefthook >/dev/null 2>&1 || { echo "Installing lefthook..."; go install github.com/evilmartians/lefthook@latest; }
    lefthook install
    @echo "Setup complete! Git hooks are now active."

# ============================================================================
# Development
# ============================================================================

# Build all CLI tools (via Bazel)
build:
    bazel build //cmd/...

# Run all tests (Bazel)
test:
    bazel test //...

# Run all tests with Go (faster iteration)
test-go:
    go test ./...

# Run tests with gotestsum (better output)
test-sum:
    go tool -modfile=tools/testsum/go.mod gotestsum --format pkgname-and-test-fails -- ./...

# Run tests with gotestsum and race detector
test-sum-race:
    go tool -modfile=tools/testsum/go.mod gotestsum --format pkgname-and-test-fails -- -race ./...

# Run tests with gotestsum verbose output
test-sum-v:
    go tool -modfile=tools/testsum/go.mod gotestsum --format standard-verbose -- ./...

# Run linter (nogo via bazel build)
lint:
    bazel build //...

# Format all Go files
format:
    gofmt -w $(find . -name '*.go' -not -path './vendor/*' -not -path './bazel-*')

# Format all shell scripts
format-sh:
    shfmt -w -i 2 -ci $(find . -name '*.sh' -not -path './vendor/*' -not -path './bazel-*')

# Lint all shell scripts
lint-sh:
    shellcheck $(find . -name '*.sh' -not -path './vendor/*' -not -path './bazel-*')
    shfmt -d -i 2 -ci $(find . -name '*.sh' -not -path './vendor/*' -not -path './bazel-*')

# Update BUILD.bazel files via Gazelle
gazelle:
    bazel run //:gazelle

# Tidy go modules
tidy:
    go mod tidy
    cd tools/testsum && go mod tidy
    @command -v gomodfmt >/dev/null 2>&1 && gomodfmt -w go.mod || true

# Format go.mod file (install: go install github.com/albertocavalcante/gomodfmt/cmd/gomodfmt@latest)
format-mod:
    @command -v gomodfmt >/dev/null 2>&1 || { echo "Installing gomodfmt..."; go install github.com/albertocavalcante/gomodfmt/cmd/gomodfmt@latest; }
    gomodfmt -w go.mod

# Run format + lint + test (CI check)
check: format format-sh lint lint-sh test

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
# Install (platform-specific)
# ============================================================================

# Default install directory per platform
install_dir_unix := env_var_or_default("SKY_INSTALL_DIR", env_var("HOME") / ".local" / "bin")
install_dir_windows := env_var_or_default("SKY_INSTALL_DIR", env_var_or_default("LOCALAPPDATA", "C:\\Users\\Default\\AppData\\Local") / "bin")

# Binary name: "sk" to avoid conflicts with common shell aliases (e.g., `alias sky='cd ~/dev/ws/sky'`)
bin_name := "sk"

# Install sky (full) to ~/.local/bin on macOS
[macos]
install:
    @echo "Building sky (full) for macOS..."
    @mkdir -p {{install_dir_unix}}
    go build -tags=sky_full -o {{install_dir_unix}}/{{bin_name}} ./cmd/sky
    @echo "Installed to {{install_dir_unix}}/{{bin_name}}"
    @echo "Make sure {{install_dir_unix}} is in your PATH"

# Install sky (full) to ~/.local/bin on Linux
[linux]
install:
    @echo "Building sky (full) for Linux..."
    @mkdir -p {{install_dir_unix}}
    go build -tags=sky_full -o {{install_dir_unix}}/{{bin_name}} ./cmd/sky
    @echo "Installed to {{install_dir_unix}}/{{bin_name}}"
    @echo "Make sure {{install_dir_unix}} is in your PATH"

# Install sky (full) to %LOCALAPPDATA%\bin on Windows
[windows]
install:
    @echo "Building sky (full) for Windows..."
    @if not exist "{{install_dir_windows}}" mkdir "{{install_dir_windows}}"
    go build -tags=sky_full -o {{install_dir_windows}}\{{bin_name}}.exe ./cmd/sky
    @echo "Installed to {{install_dir_windows}}\{{bin_name}}.exe"
    @echo "Make sure {{install_dir_windows}} is in your PATH"

# ============================================================================
# Code Generation
# ============================================================================

# Sync LSP protocol types from gopls (for LSP 3.17+ types like InlayHint)
sync-protocol:
    go run ./tools/sync-protocol

# Sync protocol with verbose output
sync-protocol-verbose:
    go run ./tools/sync-protocol -verbose

# Preview protocol sync without writing
sync-protocol-dry:
    go run ./tools/sync-protocol -dry-run

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
