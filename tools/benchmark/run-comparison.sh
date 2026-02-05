#!/bin/bash
# run-comparison.sh
#
# Runs a full A/B benchmark comparison between upstream starlark-go
# and starlark-go-x, then generates a markdown report.
#
# Usage:
#   ./tools/benchmark/run-comparison.sh [count]
#
# Arguments:
#   count - Number of benchmark iterations (default: 5, recommended: 10)

set -e

COUNT="${1:-5}"
RESULTS_DIR="${RESULTS_DIR:-/tmp/benchmark-results}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Benchmark Comparison: upstream vs starlark-go-x ==="
echo "Iterations per benchmark: $COUNT"
echo "Results directory: $RESULTS_DIR"
echo ""

mkdir -p "$RESULTS_DIR"

# Install benchstat if needed
BENCHSTAT="$(go env GOPATH)/bin/benchstat"
if [[ ! -x "$BENCHSTAT" ]]; then
  echo "Installing benchstat..."
  go install golang.org/x/perf/cmd/benchstat@latest
fi

# Clone or update upstream
UPSTREAM_DIR="/tmp/upstream-starlark-go"
if [[ -d "$UPSTREAM_DIR" ]]; then
  echo "Updating upstream starlark-go..."
  (cd "$UPSTREAM_DIR" && git pull --quiet)
else
  echo "Cloning upstream starlark-go..."
  git clone --depth=1 https://github.com/google/starlark-go.git "$UPSTREAM_DIR"
fi

# Find starlark-go-x
STARLARK_GO_X="${STARLARK_GO_X:-}"
if [[ -z "$STARLARK_GO_X" ]]; then
  # Try common locations
  for dir in \
    "/Users/adsc/dev/ws/starlark-go-x/trunk" \
    "../starlark-go-x/trunk" \
    "../../starlark-go-x/trunk" \
    "$HOME/starlark-go-x/trunk"; do
    if [[ -d "$dir" ]]; then
      STARLARK_GO_X="$dir"
      break
    fi
  done
fi

if [[ -z "$STARLARK_GO_X" ]] || [[ ! -d "$STARLARK_GO_X" ]]; then
  echo "Error: Cannot find starlark-go-x/trunk directory"
  echo "Set STARLARK_GO_X environment variable to the trunk directory path"
  exit 1
fi

echo "Using starlark-go-x: $STARLARK_GO_X"
echo ""

# Benchmark upstream
echo "=== Benchmarking upstream google/starlark-go ==="
(cd "$UPSTREAM_DIR" && go test -bench=. -benchmem -count="$COUNT" ./starlark/... 2>&1) | tee "$RESULTS_DIR/upstream.txt"
echo ""

# Benchmark starlark-go-x
echo "=== Benchmarking starlark-go-x (hooks=nil) ==="
(cd "$STARLARK_GO_X" && go test -bench=. -benchmem -count="$COUNT" ./starlark/... 2>&1) | tee "$RESULTS_DIR/starlark-go-x.txt"
echo ""

# Compare
echo "=== Running benchstat comparison ==="
"$BENCHSTAT" "$RESULTS_DIR/upstream.txt" "$RESULTS_DIR/starlark-go-x.txt" | tee "$RESULTS_DIR/comparison.txt"
echo ""

# Generate report
echo "=== Generating markdown report ==="
"$SCRIPT_DIR/generate-benchmark-report.sh" \
  "$RESULTS_DIR/upstream.txt" \
  "$RESULTS_DIR/starlark-go-x.txt" \
  "$RESULTS_DIR/benchmark-report.md"

echo ""
echo "=== Done ==="
echo "Results saved to: $RESULTS_DIR/"
echo "  - upstream.txt"
echo "  - starlark-go-x.txt"
echo "  - comparison.txt"
echo "  - benchmark-report.md"
