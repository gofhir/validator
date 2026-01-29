---
name: go-concurrency
description: Concurrencia, goroutines y thread-safety en Go. Usar al diseñar código concurrente o manejar recursos compartidos.
allowed-tools: Read, Grep, Glob, Bash(go test -race:*)
---

# Skill: go-concurrency

Concurrencia, goroutines y thread-safety en Go.

## Cuándo usar este skill

- Al diseñar código concurrente
- Al manejar recursos compartidos
- Al implementar caching thread-safe

## Mecanismos de Sincronización

### sync.Mutex

```go
type Registry struct {
    mu   sync.Mutex
    data map[string]Item
}

func (r *Registry) Set(key string, item Item) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.data[key] = item
}
```

### sync.RWMutex (lecturas frecuentes)

```go
func (c *Cache) Get(key string) (Item, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    item, ok := c.items[key]
    return item, ok
}
```

### sync/atomic (contadores)

```go
func (s *Stats) RecordHit() {
    atomic.AddUint64(&s.hits, 1)
}
```

### sync.Pool (reutilizar objetos)

```go
var bufPool = sync.Pool{
    New: func() interface{} { return new(bytes.Buffer) },
}
```

## Patrones con Context

```go
func (v *Validator) Validate(ctx context.Context, resource []byte) (*Result, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // continuar...
}
```

## Checklist

```markdown
- [ ] ¿Los recursos compartidos están protegidos?
- [ ] ¿Se usa RWMutex donde hay más lecturas?
- [ ] ¿Los goroutines tienen forma de terminar?
- [ ] ¿Se propaga context correctamente?
```
