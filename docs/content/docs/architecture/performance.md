---
title: "Performance"
linkTitle: "Performance"
description: "Optimization strategies, benchmark results, and best practices for high-performance FHIR validation."
weight: 4
---

Performance was a cross-cutting concern addressed in Milestone 15, applying optimizations across all packages. The GoFHIR Validator uses several strategies to minimize allocations, reduce computation, and maximize throughput without sacrificing correctness.

## Optimization Strategies

### Object Pooling with sync.Pool

The validator uses `sync.Pool` to reuse frequently allocated objects, particularly `Result` and `Stats` structs that are created for every validation call. This eliminates thousands of heap allocations in batch scenarios.

```go
var resultPool = sync.Pool{
    New: func() interface{} {
        return &Result{
            Issues: make([]issue.Issue, 0, 16),
        }
    },
}

func acquireResult() *Result {
    r := resultPool.Get().(*Result)
    r.Issues = r.Issues[:0] // Reset slice, keep backing array
    return r
}

func releaseResult(r *Result) {
    resultPool.Put(r)
}
```

### Regex Compilation Caching

FHIR primitive types are validated against regular expressions derived from their StructureDefinitions. Since the same patterns are used across thousands of elements, compiled regexes are cached to avoid repeated `regexp.Compile` calls.

```go
var regexCache sync.Map

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
    if cached, ok := regexCache.Load(pattern); ok {
        return cached.(*regexp.Regexp), nil
    }
    compiled, err := regexp.Compile("^" + pattern + "$")
    if err != nil {
        return nil, err
    }
    regexCache.Store(pattern, compiled)
    return compiled, nil
}
```

### Element Index Caching

Each StructureDefinition contains a snapshot with potentially hundreds of `ElementDefinition` entries. The validator builds a path-to-element index on first access and caches it for subsequent lookups, turning O(n) linear scans into O(1) map lookups.

```go
type elementIndex struct {
    byPath map[string]*ElementDefinition
    once   sync.Once
}

func (idx *elementIndex) Get(path string) *ElementDefinition {
    idx.once.Do(func() {
        idx.byPath = make(map[string]*ElementDefinition, len(idx.elements))
        for i := range idx.elements {
            idx.byPath[idx.elements[i].Path] = &idx.elements[i]
        }
    })
    return idx.byPath[path]
}
```

### Embedded Specifications

FHIR R4 base definitions are embedded directly into the binary using Go's `embed` package via `pkg/specs`. This eliminates disk I/O at startup and makes the validator binary fully self-contained.

```go
//go:embed data/r4/*.json
var r4Specs embed.FS
```

This means `validator.New()` can load all base StructureDefinitions, ValueSets, and CodeSystems from memory without touching the filesystem.

## Benchmark Results

All benchmarks were run with `go test -bench=. -benchmem ./pkg/validator/` on a standard development machine.

| Scenario | Improvement | Allocations |
|----------|-------------|-------------|
| Minimal Patient | 4.0x faster | 86% fewer |
| Patient with Data | 1.3x faster | 43% fewer |
| Parallel Validation | 8.5x faster | 74% fewer |
| Batch Processing | 2.1x faster | 72% fewer |

The "Improvement" column compares the optimized validator (M15) against the pre-optimization baseline. The "Allocations" column shows the reduction in heap allocations per operation.

## Running Benchmarks

To run the full benchmark suite:

```bash
go test -bench=. -benchmem ./pkg/validator/
```

To run a specific benchmark:

```bash
go test -bench=BenchmarkValidatePatient -benchmem ./pkg/validator/
```

To compare benchmark results between runs, use `benchstat`:

```bash
# Run benchmarks before changes
go test -bench=. -benchmem -count=10 ./pkg/validator/ > old.txt

# Make changes, then run again
go test -bench=. -benchmem -count=10 ./pkg/validator/ > new.txt

# Compare
benchstat old.txt new.txt
```

## Memory Profiling

To identify allocation hotspots:

```bash
# Generate a memory profile
go test -bench=BenchmarkValidatePatient -memprofile mem.prof ./pkg/validator/

# Analyze with pprof
go tool pprof -http=:8080 mem.prof
```

To generate a CPU profile:

```bash
# Generate a CPU profile
go test -bench=BenchmarkValidatePatient -cpuprofile cpu.prof ./pkg/validator/

# Analyze
go tool pprof -http=:8080 cpu.prof
```

## Best Practices

### Reuse the Validator Instance

Creating a validator loads and indexes StructureDefinitions. Create it once and reuse it across validations:

```go
// CORRECT: Create once, reuse many times
v, err := validator.New()
if err != nil {
    log.Fatal(err)
}

for _, resource := range resources {
    result := v.Validate(resource)
    // process result
}
```

```go
// WRONG: Creating a new validator for each resource
for _, resource := range resources {
    v, _ := validator.New() // Expensive! Reloads all SDs
    result := v.Validate(resource)
}
```

### Use Batch Validation

When validating multiple resources, pass them as a batch rather than validating one at a time. This enables internal optimizations like pool reuse and reduced GC pressure:

```go
results := v.ValidateBatch(resources)
```

### Disable Unused Phases

If you know certain validation phases are not needed for your use case, disable them to save processing time:

```go
// Skip FHIRPath constraint evaluation and slicing if not needed
v, err := validator.New(
    validator.WithDisabledPhases("constraint", "slicing"),
)
```

### Disable Terminology When Offline

If no terminology server is available, disable terminology validation explicitly to avoid timeout delays:

```go
v, err := validator.New(
    validator.WithTerminologyDisabled(),
)
```

Or via the CLI:

```bash
gofhir-validator -tx n/a patient.json
```

{{< callout type="info" >}}
The optimization strategies described here are applied automatically. You do not need to configure pooling, caching, or indexing -- they are built into the validator. The best practices above focus on how you use the validator to get the most out of these built-in optimizations.
{{< /callout >}}
