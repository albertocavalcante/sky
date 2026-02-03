# Benchmark Tools

Tools for benchmarking starlark-go-x against upstream google/starlark-go.

## Quick Start

```bash
# Run full comparison (default: 5 iterations)
./tools/benchmark/run-comparison.sh

# Run with more iterations for statistical confidence
./tools/benchmark/run-comparison.sh 10

# Results are saved to /tmp/benchmark-results/
```

## Scripts

### `run-comparison.sh`

Full A/B comparison workflow:

1. Clones/updates upstream google/starlark-go
2. Benchmarks upstream
3. Benchmarks starlark-go-x trunk
4. Compares with benchstat
5. Generates markdown report

**Environment variables:**

- `STARLARK_GO_X` - Path to starlark-go-x/trunk (auto-detected if not set)
- `RESULTS_DIR` - Output directory (default: `/tmp/benchmark-results`)

### `generate-benchmark-report.sh`

Generates a markdown report from raw benchmark files.

```bash
./tools/benchmark/generate-benchmark-report.sh \
    upstream.txt \
    starlark-go-x.txt \
    report.md
```

## Output Files

| File                  | Description                                         |
| --------------------- | --------------------------------------------------- |
| `upstream.txt`        | Raw `go test -bench` output from google/starlark-go |
| `starlark-go-x.txt`   | Raw `go test -bench` output from starlark-go-x      |
| `comparison.txt`      | benchstat comparison output                         |
| `benchmark-report.md` | Human-readable markdown report                      |

## GitHub Actions

The benchmark workflow runs automatically on:

- Push to `main`/`trunk` (when `internal/starlark/**` changes)
- Pull requests
- Manual trigger (workflow_dispatch)

**Manual trigger with upstream comparison:**

1. Go to Actions → Benchmark
2. Click "Run workflow"
3. Check "Compare against upstream google/starlark-go"
4. Download artifacts when complete

## Interpreting Results

### What to Look For

- **geomean delta** - Overall performance change (should be < 1%)
- **`~` symbol** - No statistically significant difference (good!)
- **p-value** - p > 0.05 means difference is likely noise

### Example Good Result

```
geomean    552.4n    560.3n    +1.44%  (p > 0.05, not significant)
```

This shows +1.44% overhead but it's **not statistically significant** - within noise margin.

### Red Flags

- Any benchmark without `~` showing > 5% regression
- Memory (B/op) or allocation increases
- Consistent regression across multiple runs
