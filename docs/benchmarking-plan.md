# Starlark-go-x Benchmarking Plan

This document outlines how to benchmark starlark-go-x hooks against upstream google/starlark-go to validate zero-overhead when hooks are disabled.

## Goal

Prove that starlark-go-x has **zero or negligible overhead** when:

1. All hooks are `nil` (disabled)
2. Compare against upstream google/starlark-go (unmodified)

This is **required** before proposing upstream contribution.

---

## Tooling

### Required Tools

| Tool             | Purpose                                     | Install                                             |
| ---------------- | ------------------------------------------- | --------------------------------------------------- |
| `go test -bench` | Built-in Go benchmarking                    | (included with Go)                                  |
| `benchstat`      | Statistical comparison of benchmark results | `go install golang.org/x/perf/cmd/benchstat@latest` |
| `pprof`          | CPU/memory profiling                        | (included with Go)                                  |
| `gobenchdata`    | Web visualization + GitHub Actions CI       | `go install go.bobheadxi.dev/gobenchdata@latest`    |
| `hyperfine`      | CLI/binary benchmarking (optional)          | `brew install hyperfine`                            |

### Install All Tools

```bash
# Required
go install golang.org/x/perf/cmd/benchstat@latest

# Visualization & CI
go install go.bobheadxi.dev/gobenchdata@latest

# Optional - for CLI benchmarks
brew install hyperfine  # macOS
# or: cargo install hyperfine
```

Verify:

```bash
benchstat --help
gobenchdata --help
hyperfine --version
```

---

## Hyperfine vs go test -bench

**Use the right tool for the job:**

| Use `go test -bench` when:                  | Use `hyperfine` when:                         |
| ------------------------------------------- | --------------------------------------------- |
| Benchmarking Go functions/code paths        | Benchmarking CLI binaries end-to-end          |
| Testing micro-optimizations                 | Comparing different languages/implementations |
| Need tight Go testing framework integration | Need warmup runs + cache clearing             |
| Benchmarks live alongside tests             | Measuring I/O-bound operations                |

**For starlark-go-x hooks**: Use `go test -bench` (micro-benchmarks of internal code paths).

**For Sky CLI**: Could use `hyperfine` for end-to-end `sky eval` performance.

### Hyperfine Example (optional)

```bash
# Compare sky eval performance
hyperfine --warmup 3 \
  'sky eval test.star' \
  'sky eval --no-coverage test.star'

# Export to JSON for reporting
hyperfine --warmup 3 --export-json results.json \
  'sky eval test.star'
```

---

## Existing Benchmarks

### In starlark-go-x (our fork)

Located in `starlark-go-x/trunk/`:

| File                         | Benchmarks                    | What it tests           |
| ---------------------------- | ----------------------------- | ----------------------- |
| `starlark/bench_test.go`     | Multiple                      | Core Starlark execution |
| `starlark/hashtable_test.go` | String hashing, hashtable ops | Data structure perf     |
| `syntax/parse_test.go`       | Parser                        | Parsing performance     |
| `syntax/scan_test.go`        | Scanner                       | Lexical analysis        |

The `bench_test.go` uses a clever pattern: benchmark code lives in `.star` files, and functions named `bench_*` are executed as Go benchmarks.

### In Sky (this repo)

Located in `internal/starlark/`:

| Path                                   | Benchmarks            |
| -------------------------------------- | --------------------- |
| `builtins/loader/proto_loader_test.go` | ProtoLoader cold/warm |
| `builtins/loader/json_loader_test.go`  | JSONLoader cold/warm  |
| `formatter/formatter_test.go`          | Code formatting       |
| `query/index/index_test.go`            | Index operations      |

---

## Benchmark Commands

### Basic Usage

```bash
# Run all benchmarks
go test -bench=. ./...

# Run with memory stats (important!)
go test -bench=. -benchmem ./...

# Run specific benchmark
go test -bench=BenchmarkStarlark ./starlark/...

# Run multiple times for statistical validity
go test -bench=. -benchmem -count=5 ./...

# Longer duration for more stable results
go test -bench=. -benchmem -benchtime=5s ./...

# Save to file
go test -bench=. -benchmem -count=5 ./... > results.txt
```

### Output Format

```
BenchmarkStarlark/bench_range-12    1000000    1234 ns/op    256 B/op    3 allocs/op
```

- `12`: GOMAXPROCS (CPU count)
- `1000000`: iterations run
- `1234 ns/op`: nanoseconds per operation
- `256 B/op`: bytes allocated per operation
- `3 allocs/op`: allocations per operation

---

## A/B Comparison Workflow

### Step 1: Clone Upstream (baseline)

```bash
# Create temp directory for upstream
cd /tmp
git clone https://github.com/google/starlark-go.git upstream-starlark
cd upstream-starlark

# Run benchmarks
go test -bench=. -benchmem -count=10 ./starlark/... > /tmp/baseline.txt
go test -bench=. -benchmem -count=10 ./syntax/... >> /tmp/baseline.txt
```

### Step 2: Benchmark starlark-go-x (hooks=nil)

```bash
cd /path/to/starlark-go-x/trunk

# Ensure no hooks are set (default state)
# Run same benchmarks
go test -bench=. -benchmem -count=10 ./starlark/... > /tmp/hooks-nil.txt
go test -bench=. -benchmem -count=10 ./syntax/... >> /tmp/hooks-nil.txt
```

### Step 3: Compare with benchstat

```bash
benchstat /tmp/baseline.txt /tmp/hooks-nil.txt
```

### Step 4: Benchmark with hooks enabled

Create a benchmark that sets hooks to minimal functions:

```go
func BenchmarkWithHooksEnabled(b *testing.B) {
    thread := &starlark.Thread{
        OnExec: func(fn *starlark.Function, pc uint32) {
            // Minimal work - just the call overhead
        },
    }
    // ... run benchmark with this thread
}
```

```bash
go test -bench=BenchmarkWithHooksEnabled -benchmem -count=10 ./... > /tmp/hooks-enabled.txt
benchstat /tmp/hooks-nil.txt /tmp/hooks-enabled.txt
```

---

## Expected Results

| Scenario              | Expected Overhead     | Action if violated                          |
| --------------------- | --------------------- | ------------------------------------------- |
| hooks=nil vs upstream | **< 1%**              | Must investigate - nil check should be free |
| OnExec enabled        | 5-15% per instruction | Document as expected                        |
| OnBranch enabled      | < 2%                  | Only fires on branches                      |
| OnFunctionEnter/Exit  | < 2%                  | Only fires on calls                         |
| OnIteration enabled   | < 2%                  | Only fires on loops                         |

### Red Flags

- **hooks=nil > 1% slower**: The nil pointer check is costing too much
- **Inconsistent results**: High variance (±10%+) means noisy environment
- **Memory regression**: More allocs/op even with hooks disabled

---

## Benchmark Best Practices

### Do

```go
func BenchmarkGood(b *testing.B) {
    // Setup outside the loop
    data := setupExpensiveData()

    b.ResetTimer() // Don't count setup time
    for i := 0; i < b.N; i++ {
        operation(data)
    }
}
```

### Don't

```go
func BenchmarkBad(b *testing.B) {
    for i := 0; i < b.N; i++ {
        data := setupExpensiveData() // Setup counted!
        operation(data)
    }
}
```

### Cold vs Warm Path

```go
// Cold path - fresh state each iteration
func BenchmarkCold(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        b.StopTimer()
        state := freshState()
        b.StartTimer()

        operation(state) // Only this is measured
    }
}

// Warm path - reuse state
func BenchmarkWarm(b *testing.B) {
    state := freshState()
    warmup(state)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        operation(state)
    }
}
```

### Sub-benchmarks for Variants

```go
func BenchmarkSizes(b *testing.B) {
    for _, size := range []int{10, 100, 1000, 10000} {
        b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
            data := makeData(size)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                operation(data)
            }
        })
    }
}
```

---

## Profiling Deep Dives

When benchmarks show unexpected results, profile:

### CPU Profile

```bash
go test -bench=BenchmarkSlow -cpuprofile=cpu.prof ./...
go tool pprof -http=:8080 cpu.prof
```

### Memory Profile

```bash
go test -bench=BenchmarkSlow -memprofile=mem.prof ./...
go tool pprof -http=:8080 mem.prof
```

### Trace

```bash
go test -bench=BenchmarkSlow -trace=trace.out ./...
go tool trace trace.out
```

---

## Benchmark Environment

For reliable results:

1. **Close other applications** - reduce noise
2. **Disable CPU throttling** - consistent clock speed
3. **Run multiple times** - `-count=10` minimum
4. **Same machine** - compare on identical hardware
5. **Same Go version** - compiler differences matter

### Check environment

```bash
go version
go env GOOS GOARCH
sysctl -n machdep.cpu.brand_string  # macOS
cat /proc/cpuinfo | grep "model name" | head -1  # Linux
```

---

## Automation Script

Create `scripts/benchmark-compare.sh`:

```bash
#!/bin/bash
set -e

UPSTREAM_DIR="/tmp/upstream-starlark-go"
RESULTS_DIR="./benchmark-results"
COUNT=10

mkdir -p "$RESULTS_DIR"

echo "=== Step 1: Clone/update upstream ==="
if [ -d "$UPSTREAM_DIR" ]; then
    cd "$UPSTREAM_DIR" && git pull
else
    git clone https://github.com/google/starlark-go.git "$UPSTREAM_DIR"
fi

echo "=== Step 2: Benchmark upstream ==="
cd "$UPSTREAM_DIR"
go test -bench=. -benchmem -count=$COUNT ./starlark/... > "$RESULTS_DIR/baseline.txt" 2>&1

echo "=== Step 3: Benchmark starlark-go-x ==="
cd /path/to/starlark-go-x/trunk
go test -bench=. -benchmem -count=$COUNT ./starlark/... > "$RESULTS_DIR/hooks-nil.txt" 2>&1

echo "=== Step 4: Compare ==="
benchstat "$RESULTS_DIR/baseline.txt" "$RESULTS_DIR/hooks-nil.txt" | tee "$RESULTS_DIR/comparison.txt"

echo "=== Results saved to $RESULTS_DIR ==="
```

---

## Reporting & Visualization

### Option 1: benchstat CSV → Spreadsheet Charts

Export to CSV for Google Sheets/Excel visualization:

```bash
# Run comparison and export CSV
benchstat -format=csv baseline.txt hooks-nil.txt > comparison.csv
```

Then import into Google Sheets and create charts.

### Option 2: gobenchdata Web Visualization

[gobenchdata](https://github.com/bobheadxi/gobenchdata) generates interactive web charts from Go benchmark data.

```bash
# Install
go install go.bobheadxi.dev/gobenchdata@latest

# Run benchmarks and pipe to gobenchdata
go test -bench=. -benchmem ./starlark/... | gobenchdata --json bench.json

# Generate web app
gobenchdata web generate --title "starlark-go-x benchmarks"

# Serve locally
gobenchdata web serve
```

This creates an interactive chart showing benchmark history over time.

### Option 3: GitHub Actions CI Integration

Add to `.github/workflows/benchmark.yml`:

```yaml
name: Benchmark
on:
  push:
    branches: [main, trunk]
  pull_request:
    branches: [main, trunk]

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Run benchmarks
        run: go test -bench=. -benchmem -count=5 ./starlark/... | tee bench.txt

      - name: Compare with baseline
        uses: bobheadxi/gobenchdata@v1
        with:
          PRUNE_COUNT: 30
          GO_TEST_FLAGS: -bench=. -benchmem
          PUBLISH: true
          PUBLISH_BRANCH: gh-pages
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

This will:

- Run benchmarks on every push/PR
- Publish results to gh-pages branch
- Generate web visualization at `https://<user>.github.io/<repo>/`

### Option 4: PR Performance Regression Checks

Use [github-action-benchmark](https://github.com/benchmark-action/github-action-benchmark) to fail PRs that regress performance:

```yaml
- name: Check for regression
  uses: benchmark-action/github-action-benchmark@v1
  with:
    tool: "go"
    output-file-path: bench.txt
    fail-on-alert: true
    alert-threshold: "150%" # Fail if 50% slower
    comment-on-alert: true
```

### Data Pipeline Summary

```
go test -bench    →    benchstat    →    CSV/Text Report
      ↓                                        ↓
      └──────→    gobenchdata    →    JSON → Web Charts
                                              ↓
                                        GitHub Pages
```

---

## Reporting Format

For upstream proposal, format results as:

```markdown
## Performance Analysis

Benchmarked against google/starlark-go @ commit abc123

### Hooks Disabled (nil)

| Benchmark   | Upstream | starlark-go-x | Delta |
| ----------- | -------- | ------------- | ----- |
| bench_range | 1.23µs   | 1.24µs        | +0.8% |
| bench_fib   | 45.6µs   | 45.8µs        | +0.4% |

**Conclusion**: < 1% overhead when hooks are nil.

### Hooks Enabled (OnExec)

| Benchmark   | hooks=nil | OnExec enabled | Delta  |
| ----------- | --------- | -------------- | ------ |
| bench_range | 1.24µs    | 1.42µs         | +14.5% |

**Conclusion**: Expected overhead for per-instruction callback.
```

---

## Next Steps

1. [ ] Install benchstat: `go install golang.org/x/perf/cmd/benchstat@latest`
2. [ ] Run baseline benchmarks on upstream starlark-go
3. [ ] Run benchmarks on starlark-go-x trunk (hooks=nil)
4. [ ] Compare with benchstat
5. [ ] If overhead > 1%, investigate with pprof
6. [ ] Document results for upstream proposal
7. [ ] Create benchmarks with hooks enabled
8. [ ] Add results to roadmap.mdx

---

## References

### Core Tools

- [Go Benchmarking](https://pkg.go.dev/testing#hdr-Benchmarks) - Official Go benchmark documentation
- [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) - Statistical comparison tool
- [Leveraging benchstat Projections](https://www.bwplotka.dev/2024/go-microbenchmarks-benchstat/) - Advanced benchstat usage

### Visualization & CI

- [gobenchdata](https://github.com/bobheadxi/gobenchdata) - Web visualization + GitHub Actions
- [github-action-benchmark](https://github.com/benchmark-action/github-action-benchmark) - PR regression checks
- [Continuous Benchmarking with Go](https://dev.to/vearutop/continuous-benchmarking-with-go-and-github-actions-41ok) - CI setup guide

### CLI Benchmarking

- [hyperfine](https://github.com/sharkdp/hyperfine) - Command-line benchmarking tool

### Profiling

- [Profiling Go Programs](https://go.dev/blog/pprof) - Official pprof guide
- [High Performance Go Workshop](https://dave.cheney.net/high-performance-go-workshop/dotgo-paris.html) - Deep dive
