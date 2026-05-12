# Benchmarks

This directory contains performance benchmarks for mtx.

## Running

```bash
make test-bench
```

## Writing Benchmarks

```go
func BenchmarkOperation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        // Operation to benchmark
    }
}
```

## Guidelines

- Reset timer after setup: `b.ResetTimer()`
- Report allocations: `b.ReportAllocs()`
- Use `b.RunParallel` for concurrent benchmarks
- Name benchmarks descriptively: `BenchmarkParse_LargeInput`
