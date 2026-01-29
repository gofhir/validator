---
name: go-refactoring
description: Refactorización segura de código Go. Usar al mejorar estructura de código o eliminar duplicación.
allowed-tools: Read, Grep, Glob, Bash(go test:*)
---

# Skill: go-refactoring

Refactorización segura de código Go.

## Cuándo usar este skill

- Al mejorar estructura de código
- Al eliminar duplicación
- Al simplificar código complejo

## Principios

1. **Tests Primero**: Asegurar cobertura antes de cambiar
2. **Cambios Pequeños**: Un tipo de refactorización a la vez
3. **Mantener Comportamiento**: No añadir features durante refactor

## Patrones de Refactorización

### Extract Function

```go
// Antes: función larga
func (v *Validator) Validate(...) (*Result, error) {
    // 100 líneas...
}

// Después: funciones extraídas
func (v *Validator) Validate(...) (*Result, error) {
    sd, err := v.loadStructureDefinition(ctx, resourceType)
    v.validateStructure(resource, sd, result)
    v.validateConstraints(ctx, resource, sd, result)
    return result, nil
}
```

### Extract Interface

```go
// Antes: dependencia concreta
type Validator struct {
    registry *Registry
}

// Después: dependencia de interface
type Validator struct {
    registry StructureDefinitionProvider  // Interface
}
```

### Simplify Conditionals (Early Return)

```go
// Antes
if x > 0 {
    if x < 100 {
        return doSomething(x)
    }
}
return errors.New("invalid")

// Después
if x <= 0 || x >= 100 {
    return errors.New("invalid")
}
return doSomething(x)
```

## Checklist

```markdown
- [ ] ¿Hay tests que cubren el código?
- [ ] ¿Los tests pasan antes y después?
- [ ] ¿Cambios pequeños e incrementales?
```
