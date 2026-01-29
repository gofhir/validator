---
name: go-code-review
description: Revisión de código Go. Usar al revisar PRs, verificar calidad de código, o identificar problemas potenciales.
allowed-tools: Read, Grep, Glob, Bash(golangci-lint:*)
---

# Skill: go-code-review

Revisión de código Go.

## Cuándo usar este skill

- Al revisar PRs
- Al verificar calidad de código
- Al identificar problemas potenciales

## Checklist de Revisión

### Correctitud
- [ ] ¿El código hace lo que debería?
- [ ] ¿Se manejan todos los casos edge?
- [ ] ¿Los errores se manejan correctamente?
- [ ] ¿Hay race conditions?

### Diseño
- [ ] ¿Sigue los patrones del proyecto?
- [ ] ¿Las interfaces son pequeñas?
- [ ] ¿Se evita duplicación?

### Testing
- [ ] ¿Hay tests para código nuevo?
- [ ] ¿Los tests pasan?

## Patrones a Verificar

### Error Handling
```go
// ✅ BIEN
if err != nil {
    return fmt.Errorf("loading %s: %w", name, err)
}

// ❌ MAL
result, _ := doSomething()
```

### Concurrencia
```go
// ✅ BIEN
func (c *Cache) Get(key string) (Item, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.items[key]
}
```

## Herramientas

```bash
golangci-lint run ./...
go vet ./...
```
