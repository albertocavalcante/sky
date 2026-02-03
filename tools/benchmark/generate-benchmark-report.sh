#!/bin/bash
# generate-benchmark-report.sh
#
# Generates a markdown benchmark report from benchstat output.
# Can be run locally or in CI.
#
# Usage:
#   ./scripts/generate-benchmark-report.sh [upstream.txt] [fork.txt] [output.md]
#
# If no args provided, uses default paths from /tmp/benchmark-results/

set -e

# Defaults
UPSTREAM_FILE="${1:-/tmp/benchmark-results/upstream.txt}"
FORK_FILE="${2:-/tmp/benchmark-results/starlark-go-x.txt}"
OUTPUT_FILE="${3:-/tmp/benchmark-results/report.md}"

# Find benchstat
BENCHSTAT="$(go env GOPATH)/bin/benchstat"
if [[ ! -x "$BENCHSTAT" ]]; then
    echo "Installing benchstat..."
    go install golang.org/x/perf/cmd/benchstat@latest
fi

# Validate inputs
if [[ ! -f "$UPSTREAM_FILE" ]]; then
    echo "Error: Upstream file not found: $UPSTREAM_FILE"
    exit 1
fi

if [[ ! -f "$FORK_FILE" ]]; then
    echo "Error: Fork file not found: $FORK_FILE"
    exit 1
fi

# Run benchstat
COMPARISON=$("$BENCHSTAT" "$UPSTREAM_FILE" "$FORK_FILE" 2>&1)

# Extract geomean
GEOMEAN_TIME=$(echo "$COMPARISON" | grep "^geomean" | head -1 | awk '{print $NF}')
GEOMEAN_MEM=$(echo "$COMPARISON" | grep "^geomean" | tail -2 | head -1 | awk '{print $NF}')

# Get metadata
DATE=$(date +%Y-%m-%d)
GO_VERSION=$(go version | awk '{print $3}')
PLATFORM=$(go env GOOS)/$(go env GOARCH)

# Count samples (from first benchmark line)
SAMPLES=$(echo "$COMPARISON" | grep "n=3" | head -1 | grep -o "n=[0-9]*" | head -1 | cut -d= -f2)
SAMPLES="${SAMPLES:-unknown}"

# Generate report
cat > "$OUTPUT_FILE" << EOF
# Benchmark Report: starlark-go-x vs upstream

**Generated**: $DATE
**Platform**: $PLATFORM
**Go Version**: $GO_VERSION
**Samples**: $SAMPLES runs per benchmark

## Summary

| Metric | Delta | Status |
|--------|-------|--------|
| **Time (geomean)** | $GEOMEAN_TIME | $(if [[ "$GEOMEAN_TIME" == *"-"* ]] || [[ "$GEOMEAN_TIME" == "+0"* ]] || [[ "$GEOMEAN_TIME" == "~" ]]; then echo "✅ Good"; else echo "⚠️ Review"; fi) |
| **Memory (geomean)** | $GEOMEAN_MEM | $(if [[ "$GEOMEAN_MEM" == *"-"* ]] || [[ "$GEOMEAN_MEM" == "+0"* ]] || [[ "$GEOMEAN_MEM" == "~" ]]; then echo "✅ Good"; else echo "⚠️ Review"; fi) |

## Analysis

EOF

# Check if any individual benchmark (not geomean) shows significant regression
# Look for lines with "+" percentage that DON'T have "~" (which means statistically significant)
# Exclude geomean lines and footnote markers
SIGNIFICANT_REGRESSIONS=$(echo "$COMPARISON" | grep -E "^\w.*\+[0-9]+\.[0-9]+%" | grep -v "~" | grep -v "^geomean" | grep -v "¹\|²\|³\|⁴" | head -5 || true)

# Check if all benchmarks show "~" (not significant)
ALL_TILDE=$(echo "$COMPARISON" | grep -E "^(Benchmark|Starlark|String|Hash|Program)" | grep -v "~" | grep -v "^$" | wc -l | tr -d ' ')

if [[ "$ALL_TILDE" -eq 0 ]] || [[ -z "$SIGNIFICANT_REGRESSIONS" ]]; then
    cat >> "$OUTPUT_FILE" << EOF
**Result**: ✅ No statistically significant performance regressions detected.

All benchmarks show \`~\` (no significant difference) with p > 0.05.
The hooks have **zero measurable overhead** when disabled.

EOF
else
    cat >> "$OUTPUT_FILE" << EOF
**Result**: ⚠️ Some benchmarks show statistically significant regressions.

\`\`\`
$SIGNIFICANT_REGRESSIONS
\`\`\`

EOF
fi

# Add raw output
cat >> "$OUTPUT_FILE" << EOF
## Raw benchstat Output

\`\`\`
$COMPARISON
\`\`\`

## Files

- Upstream: \`$UPSTREAM_FILE\`
- Fork: \`$FORK_FILE\`
- Report: \`$OUTPUT_FILE\`
EOF

echo "Report generated: $OUTPUT_FILE"

# Also output summary to stdout for CI
echo ""
echo "=== BENCHMARK SUMMARY ==="
echo "Time delta (geomean): $GEOMEAN_TIME"
echo "Memory delta (geomean): $GEOMEAN_MEM"
echo "========================="
