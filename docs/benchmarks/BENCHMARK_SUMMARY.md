# Benchmark Summary - Quick Reference

## Performance at a Glance

| Metric | Value |
|--------|-------|
| **Average Speedup** | 1.43x |
| **Best Speedup** | 1.61x (GreedyAI) |
| **Throughput Gain** | +48.5% |
| **Memory Overhead** | < 0.5% |
| **Optimal Batch Size** | 500-1000 games |

## Speedup by Batch Size

```
Batch     Speedup    When to Use
─────     ───────    ───────────
10        1.34x      Quick tests
50        1.51x      Development
100       1.40x      Rapid iteration
500       1.51x      ← OPTIMAL START
1000      1.43x      ← RECOMMENDED DEFAULT
5000      1.46x      High confidence
10000     1.22x      Final validation
```

## Commands to Run Benchmarks

### All benchmarks:
```bash
cd src/gosim/simulation
go test -bench=. -benchmem -benchtime=5s -run=^$
```

### Specific batch size:
```bash
go test -bench=Batch1000 -benchmem -run=^$
```

### Compare serial vs parallel:
```bash
go test -bench=BenchmarkSerial -benchmem -count=5 > serial.txt
go test -bench=BenchmarkParallel -benchmem -count=5 > parallel.txt
```

### Test different AI types:
```bash
go test -bench=AIRandom -benchmem -run=^$
go test -bench=AIGreedy -benchmem -run=^$
```

## Interpretation Guide

### Good Performance Indicators
- Speedup > 1.3x ✓
- Memory increase < 1% ✓
- Consistent results across runs ✓
- Throughput > 2000 games/sec ✓

### Red Flags (None Observed)
- Speedup < 1.0x (slower than serial)
- Memory leaks (growing allocations)
- High variance between runs
- Crashes or errors

## System Information

- **CPU:** Intel(R) N100 (4 cores)
- **OS:** Linux 5.4.0-216-generic
- **Go Version:** (check with `go version`)
- **Architecture:** amd64

## Files

- `benchmark_test.go` - Comprehensive benchmark suite
- `BENCHMARK_ANALYSIS.md` - Detailed analysis and recommendations
- `benchmark_results.txt` - Raw benchmark output
- `benchmark_serial.txt` - Serial-only runs (5x)
- `benchmark_parallel.txt` - Parallel-only runs (5x)
