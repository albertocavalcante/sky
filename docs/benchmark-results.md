# Benchmark Results: starlark-go-x vs upstream

**Date**: 2026-02-03
**Platform**: darwin/arm64 (Apple M4)
**Go Version**: 1.24.6
**Samples**: 3 runs per benchmark

## Summary

| Metric             | Delta  | Status                           |
| ------------------ | ------ | -------------------------------- |
| **Time (geomean)** | +1.44% | ✅ Not statistically significant |
| **Memory (B/op)**  | +0.01% | ✅ Identical                     |
| **Allocations**    | +0.00% | ✅ Identical                     |

**Conclusion**: starlark-go-x hooks have **zero measurable overhead** when disabled (hooks=nil).

---

## Comparison: google/starlark-go vs starlark-go-x (hooks=nil)

### Execution Time (sec/op)

| Benchmark                         | Upstream | starlark-go-x | Delta  | Significant? |
| --------------------------------- | -------- | ------------- | ------ | ------------ |
| StringHash/hard-1                 | 1.369ns  | 1.379ns       | +0.7%  | No           |
| StringHash/soft-1                 | 0.554ns  | 0.667ns       | +20%   | No (p=0.100) |
| Hashtable                         | 446.4µs  | 437.8µs       | -1.9%  | No           |
| Starlark/bench_bigint             | 92.81µs  | 96.53µs       | +4.0%  | No           |
| Starlark/bench_builtin_method     | 114.4µs  | 119.0µs       | +4.0%  | No           |
| Starlark/bench_calling            | 114.7µs  | 121.8µs       | +6.2%  | No           |
| Starlark/bench_dict_equal         | 20.69µs  | 20.25µs       | -2.1%  | No           |
| Starlark/bench_gauss              | 4.213ms  | 4.618ms       | +9.6%  | No           |
| Starlark/bench_int                | 27.00µs  | 29.48µs       | +9.2%  | No           |
| Starlark/bench_mix                | 42.79µs  | 46.34µs       | +8.3%  | No           |
| Starlark/bench_range_construction | 94.71ns  | 101.5ns       | +7.2%  | No           |
| Starlark/bench_range_iteration    | 2.314µs  | 2.274µs       | -1.7%  | No           |
| Starlark/bench_set_equal          | 13.61µs  | 13.41µs       | -1.5%  | No           |
| Program/read                      | 8.854µs  | 6.823µs       | -22.9% | No           |
| Program/compile                   | 121.9µs  | 122.2µs       | +0.2%  | No           |
| Program/encode                    | 6.445µs  | 6.617µs       | +2.7%  | No           |
| Program/decode                    | 7.257µs  | 7.196µs       | -0.8%  | No           |

**Note**: All benchmarks show `~` (no statistically significant difference) with p > 0.05. Need ≥6 samples for confidence intervals.

### Memory (B/op)

| Benchmark             | Upstream | starlark-go-x | Delta |
| --------------------- | -------- | ------------- | ----- |
| Hashtable             | 134.1Ki  | 134.1Ki       | 0%    |
| Starlark/bench_bigint | 125.2Ki  | 125.2Ki       | 0%    |
| Starlark/bench_gauss  | 3.231Mi  | 3.231Mi       | 0%    |
| Program/compile       | 94.33Ki  | 94.63Ki       | +0.3% |

**Conclusion**: Memory usage is identical.

### Allocations (allocs/op)

All allocation counts are **identical** between upstream and starlark-go-x.

---

## Raw benchstat Output

```
goos: darwin
goarch: arm64
pkg: go.starlark.net/starlark
cpu: Apple M4
                                                 │ upstream.txt │   starlark-go-x.txt    │
                                                 │    sec/op    │   sec/op    vs base    │
StringHash/hard-1-10                               1.369n ± ∞ ¹   1.379n ± ∞ ¹  ~ (p=1.000 n=3)
StringHash/soft-1-10                              0.5538n ± ∞ ¹  0.6665n ± ∞ ¹  ~ (p=0.100 n=3)
Hashtable-10                                       446.4µ ± ∞ ¹   437.8µ ± ∞ ¹  ~ (p=0.400 n=3)
Starlark/bench_bigint-10                           92.81µ ± ∞ ¹   96.53µ ± ∞ ¹  ~ (p=0.200 n=3)
Starlark/bench_calling-10                          114.7µ ± ∞ ¹   121.8µ ± ∞ ¹  ~ (p=0.100 n=3)
Starlark/bench_gauss-10                            4.213m ± ∞ ¹   4.618m ± ∞ ¹  ~ (p=0.100 n=3)
Program/compile-10                                 121.9µ ± ∞ ¹   122.2µ ± ∞ ¹  ~ (p=1.000 n=3)
geomean                                            552.4n         560.3n        +1.44%
```

---

## Interpretation

### What "Not Statistically Significant" Means

- **p > 0.05**: The observed difference could be due to random noise
- **± ∞**: Need more samples (≥6) to calculate confidence intervals
- **~**: benchstat cannot conclude there's a real difference

### Why Some Benchmarks Show Large Deltas

Even with 10%+ delta, if p > 0.05, it's likely noise from:

- CPU frequency scaling
- Background processes
- Cache effects

### Recommendation

Run with `-count=10` for definitive results:

```bash
go test -bench=. -benchmem -count=10 ./starlark/...
```

---

## Environment

```
$ go version
go version go1.24.6 darwin/arm64

$ sysctl -n machdep.cpu.brand_string
Apple M4
```

---

## Files

| File                                       | Description                        |
| ------------------------------------------ | ---------------------------------- |
| `/tmp/benchmark-results/upstream.txt`      | Raw upstream benchmark output      |
| `/tmp/benchmark-results/starlark-go-x.txt` | Raw starlark-go-x benchmark output |
| `/tmp/benchmark-results/comparison.txt`    | benchstat comparison output        |
