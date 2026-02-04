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
# Release Management
# ============================================================================

# Show current version and latest tags
version:
    @echo "Latest tags:"
    @git tag --sort=-v:refname | head -5 || echo "  (no tags yet)"
    @echo ""
    @echo "To release:"
    @echo "  just release-rc 0.1.0    # Create v0.1.0-rc.0 (or increment rc)"
    @echo "  just release 0.1.0       # Create v0.1.0 final release"

# Create a release candidate (v0.1.0-rc.0, v0.1.0-rc.1, etc.)
release-rc version:
    #!/usr/bin/env bash
    set -euo pipefail

    # Find the latest RC for this version
    LATEST_RC=$(git tag --sort=-v:refname | grep "^v{{version}}-rc\." | head -1 || true)

    if [ -z "$LATEST_RC" ]; then
        NEW_TAG="v{{version}}-rc.0"
    else
        # Extract RC number and increment
        RC_NUM=$(echo "$LATEST_RC" | sed 's/.*-rc\.\([0-9]*\)/\1/')
        NEW_RC=$((RC_NUM + 1))
        NEW_TAG="v{{version}}-rc.${NEW_RC}"
    fi

    echo "Creating release candidate: $NEW_TAG"
    echo ""

    # Show what will be released
    if [ -n "$LATEST_RC" ]; then
        echo "Changes since $LATEST_RC:"
        git log --oneline "$LATEST_RC"..HEAD | head -20
    else
        LATEST_TAG=$(git tag --sort=-v:refname | head -1 || true)
        if [ -n "$LATEST_TAG" ]; then
            echo "Changes since $LATEST_TAG:"
            git log --oneline "$LATEST_TAG"..HEAD | head -20
        else
            echo "Changes (first release):"
            git log --oneline | head -20
        fi
    fi

    echo ""
    read -p "Create and push $NEW_TAG? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -a "$NEW_TAG" -m "Release $NEW_TAG"
        git push origin "$NEW_TAG"
        echo ""
        echo "Tag $NEW_TAG pushed! GitHub Actions will create the release."
        echo "View at: https://github.com/albertocavalcante/sky/releases"
    else
        echo "Aborted."
    fi

# Create a final release (v0.1.0)
release version:
    #!/usr/bin/env bash
    set -euo pipefail

    NEW_TAG="v{{version}}"

    # Check if tag already exists
    if git tag | grep -q "^${NEW_TAG}$"; then
        echo "Error: Tag $NEW_TAG already exists!"
        exit 1
    fi

    echo "Creating release: $NEW_TAG"
    echo ""

    # Show what will be released
    LATEST_TAG=$(git tag --sort=-v:refname | head -1 || true)
    if [ -n "$LATEST_TAG" ]; then
        echo "Changes since $LATEST_TAG:"
        git log --oneline "$LATEST_TAG"..HEAD | head -20
    else
        echo "Changes (first release):"
        git log --oneline | head -20
    fi

    echo ""
    read -p "Create and push $NEW_TAG? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -a "$NEW_TAG" -m "Release $NEW_TAG"
        git push origin "$NEW_TAG"
        echo ""
        echo "Tag $NEW_TAG pushed! GitHub Actions will create the release."
        echo "View at: https://github.com/albertocavalcante/sky/releases"
    else
        echo "Aborted."
    fi

# Delete a tag (local and remote) - use with caution!
release-delete tag:
    #!/usr/bin/env bash
    set -euo pipefail

    echo "This will delete tag: {{tag}}"
    read -p "Are you sure? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -d "{{tag}}" || true
        git push origin --delete "{{tag}}" || true
        echo "Tag {{tag}} deleted."
    else
        echo "Aborted."
    fi
