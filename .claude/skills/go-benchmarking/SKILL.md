---
name: go-benchmarking
description: Benchmarks y profiling en Go. Usar al medir rendimiento, comparar implementaciones, o identificar bottlenecks.
allowed-tools: Read, Grep, Glob, Bash(go test -bench:*, go tool pprof:*, go tool trace:*)
---

# Skill: go-benchmarking

Benchmarks y profiling de rendimiento en Go.

## Cuándo usar este skill

- Al medir rendimiento de código
- Al comparar implementaciones
- Al identificar bottlenecks

## Escribir Benchmarks

```go
func BenchmarkCompile(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _, _ = fhirpath.Compile("Patient.name.given")
    }
}

func BenchmarkEvaluate(b *testing.B) {
    expr := fhirpath.MustCompile("Patient.name.given")
    b.ResetTimer()  // No contar setup
    for i := 0; i < b.N; i++ {
        _, _ = expr.Evaluate(patientJSON)
    }
}
```

## Ejecutar Benchmarks

```bash
# Todos los benchmarks
go test -bench=. -benchmem ./...

# Benchmark específico
go test -bench=BenchmarkValidate ./validator

# Con CPU profile
go test -bench=BenchmarkX -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Con memory profile
go test -bench=BenchmarkX -memprofile=mem.prof
go tool pprof mem.prof
```

## Interpretar Resultados

```
BenchmarkCompile-8     50000     30123 ns/op     12048 B/op     234 allocs/op
                       │         │               │              │
                       │         │               │              └─ Allocations por op
                       │         │               └─ Bytes por op
                       │         └─ Nanosegundos por op
                       └─ Número de iteraciones
```
