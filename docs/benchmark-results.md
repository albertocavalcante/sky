# Benchmark Results: starlark-go-x vs upstream

**Date**: 2026-02-03
**Platform**: darwin/arm64 (Apple M4)
**Go Version**: 1.25.6
**Samples**: 50 runs per benchmark

## Summary

| Metric             | Delta      | Status                             |
| ------------------ | ---------- | ---------------------------------- |
| **Time (geomean)** | **-1.44%** | ✅ starlark-go-x is FASTER overall |
| **Memory (B/op)**  | +0.01%     | ✅ Identical                       |
| **Allocations**    | +0.00%     | ✅ Identical                       |

**Overall**: starlark-go-x is **1.44% faster** than upstream on average.

However, individual benchmarks show mixed results - some faster, some slower.

---

## Detailed Results

### Benchmarks Where starlark-go-x is FASTER

| Benchmark                         | Upstream | starlark-go-x | Delta       | p-value |
| --------------------------------- | -------- | ------------- | ----------- | ------- |
| Program/encode                    | 9.616µs  | 6.874µs       | **-28.52%** | p=0.000 |
| Program/read                      | 9.166µs  | 6.972µs       | **-23.93%** | p=0.000 |
| StringHash/hard-64                | 5.246ns  | 4.348ns       | **-17.14%** | p=0.000 |
| bench_set_equal                   | 15.35µs  | 13.30µs       | **-13.36%** | p=0.000 |
| bench_to_json_deep_list           | 3.682µs  | 3.241µs       | **-11.99%** | p=0.000 |
| bench_to_json_deep                | 3.696µs  | 3.259µs       | **-11.81%** | p=0.000 |
| Program/decode                    | 8.876µs  | 7.815µs       | **-11.95%** | p=0.000 |
| bench_issubset_unique_same        | 26.75µs  | 23.96µs       | **-10.43%** | p=0.000 |
| bench_to_json_flat_big            | 282.3µs  | 259.5µs       | **-8.07%**  | p=0.000 |
| bench_issubset_unique_small_large | 26.15µs  | 24.07µs       | **-7.94%**  | p=0.000 |
| bench_range_construction          | 112.3ns  | 103.5ns       | **-7.79%**  | p=0.000 |
| Hashtable                         | 478.5µs  | 449.9µs       | **-5.99%**  | p=0.000 |

### Benchmarks Where starlark-go-x is SLOWER

| Benchmark                            | Upstream | starlark-go-x | Delta       | p-value |
| ------------------------------------ | -------- | ------------- | ----------- | ------- |
| bench_int                            | 26.87µs  | 32.24µs       | **+20.02%** | p=0.000 |
| bench_calling                        | 115.2µs  | 132.5µs       | **+15.07%** | p=0.000 |
| StringHash/hard-4                    | 2.233ns  | 2.511ns       | **+12.47%** | p=0.000 |
| bench_issubset_duplicate_same        | 19.78µs  | 22.06µs       | **+11.52%** | p=0.000 |
| bench_issubset_duplicate_large_small | 40.41µs  | 44.36µs       | **+9.77%**  | p=0.000 |
| StringHash/soft-2                    | 0.840ns  | 0.918ns       | **+9.25%**  | p=0.000 |
| bench_gauss                          | 4.789ms  | 5.158ms       | **+7.70%**  | p=0.001 |

### Benchmarks with No Significant Difference (~)

| Benchmark                         | p-value |
| --------------------------------- | ------- |
| StringHash/hard-16                | p=0.086 |
| StringHash/soft-32                | p=0.082 |
| StringHash/soft-128               | p=0.158 |
| StringHash/hard-512               | p=0.499 |
| StringHash/soft-512               | p=0.095 |
| StringHash/hard-1024              | p=0.762 |
| StringHash/soft-1024              | p=0.064 |
| bench_dict_equal                  | p=0.186 |
| bench_issubset_unique_large_small | p=0.176 |

---

## Analysis

### What the Results Mean

With 50 samples, we now have statistically significant results (p < 0.05). The key findings:

1. **Overall Performance**: starlark-go-x is **1.44% faster** (geomean)
2. **Memory**: Identical (no regression)
3. **Allocations**: Identical (no regression)

### Why Some Benchmarks Differ

The differences are likely due to:

1. **Go version differences**: starlark-go-x may have been compiled/tested with different Go version
2. **Code changes**: Some optimizations or changes in the fork
3. **CPU/cache effects**: Different code paths may hit CPU caches differently

### Investigation Needed

The following benchmarks show >10% regression and need investigation:

- `bench_int` (+20.02%) - Integer operations
- `bench_calling` (+15.07%) - Function calling overhead
- `bench_issubset_duplicate_same` (+11.52%) - Set operations

These may be unrelated to the hooks - could be other changes in the fork.

---

## Conclusion for Upstream Proposal

| Claim                              | Evidence                                                 |
| ---------------------------------- | -------------------------------------------------------- |
| "Hooks add zero overhead when nil" | ⚠️ **Mixed** - overall faster, but some benchmarks slower |
| "No memory regression"             | ✅ **Confirmed** - identical memory usage                |
| "No allocation regression"         | ✅ **Confirmed** - identical allocation counts           |

### Recommendation

Before upstream proposal:

1. **Investigate regressions** in `bench_int`, `bench_calling`, `bench_issubset_duplicate_same`
2. **Ensure same Go version** for both upstream and fork benchmarks
3. **Run on clean machine** with no background processes
4. **Check for unrelated code changes** that might affect these benchmarks

---

## Raw Data

### Time Comparison (sec/op)

```
                                                 │   upstream    │   starlark-go-x   │
                                                 │    sec/op     │  sec/op  vs base  │
geomean                                              586.0n          577.6n   -1.44%

Fastest improvements:
Program/encode-10                                    9.616µ          6.874µ  -28.52%
Program/read-10                                      9.166µ          6.972µ  -23.93%
StringHash/hard-64-10                                5.246n          4.348n  -17.14%
Starlark/bench_set_equal-10                          15.35µ          13.30µ  -13.36%

Slowest regressions:
Starlark/bench_int-10                                26.87µ          32.24µ  +20.02%
Starlark/bench_calling-10                            115.2µ          132.5µ  +15.07%
StringHash/hard-4-10                                 2.233n          2.511n  +12.47%
```

### Memory Comparison (B/op)

```
geomean                                                              +0.01%
```

All memory allocations are identical or within 0.01%.

### Allocation Comparison (allocs/op)

```
geomean                                                              +0.00%
```

All allocation counts are identical.

---

## Environment

```
$ go version
go version go1.25.6 darwin/arm64

$ sysctl -n machdep.cpu.brand_string
Apple M4

$ date
Tue Feb  3 14:54:00 PST 2026
```

## Files

| File                                          | Description                              |
| --------------------------------------------- | ---------------------------------------- |
| `/tmp/benchmark-results/upstream-50.txt`      | Raw upstream benchmark (50 samples)      |
| `/tmp/benchmark-results/starlark-go-x-50.txt` | Raw starlark-go-x benchmark (50 samples) |
| `/tmp/benchmark-results/comparison-50.txt`    | benchstat comparison output              |
