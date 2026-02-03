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
