---
name: go-performance
description: Optimización de rendimiento en Go. Usar al optimizar hot paths, reducir allocations, o mejorar latencia.
allowed-tools: Read, Grep, Glob, Bash(go test -bench:*, go tool pprof:*)
---

# Skill: go-performance

Optimización de rendimiento en Go.

## Cuándo usar este skill

- Al optimizar hot paths
- Al reducir allocations
- Al mejorar latencia

## Principios

1. **Medir primero** con benchmarks
2. **Identificar hot paths**
3. **Optimizar solo lo necesario**

## Técnicas de Optimización

### Reducir Allocations

```go
// ❌ MAL
var results []Result
for _, item := range items {
    results = append(results, transform(item))
}

// ✅ BIEN: Pre-allocate
results := make([]Result, 0, len(items))
```

### Object Pooling

```go
var bufferPool = sync.Pool{
    New: func() interface{} { return new(bytes.Buffer) },
}

func process() {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() { buf.Reset(); bufferPool.Put(buf) }()
    // usar buf...
}
```

### Caching

```go
type ExpressionCache struct {
    mu    sync.RWMutex
    cache map[string]*Expression
}

func (c *ExpressionCache) GetOrCompile(expr string) (*Expression, error) {
    // Fast path: lectura
    c.mu.RLock()
    if e, ok := c.cache[expr]; ok {
        c.mu.RUnlock()
        return e, nil
    }
    c.mu.RUnlock()
    // Slow path: compilar y guardar
}
```

## Benchmarks

```bash
go test -bench=. -benchmem ./...
go test -cpuprofile=cpu.prof -bench=BenchmarkX
go tool pprof cpu.prof
```
