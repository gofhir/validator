---
name: go-methods
description: Métodos, receptores y diseño de tipos en Go. Usar al definir métodos o decidir entre receptor de valor o puntero.
allowed-tools: Read, Grep, Glob
---

# Skill: go-methods

Métodos, receptores y diseño de tipos en Go.

## Cuándo usar este skill

- Al definir métodos para tipos
- Al decidir entre receptor de valor o puntero
- Al diseñar APIs fluent

## Receptor de Valor

Usar cuando:
- El tipo es pequeño (primitivos, structs pequeños)
- El método no modifica el receptor

```go
func (b Boolean) Type() string { return "Boolean" }
func (b Boolean) Bool() bool { return b.value }
```

## Receptor de Puntero

Usar cuando:
- El método modifica el receptor
- El tipo es grande o contiene mutex/maps

```go
func (r *Registry) Register(sd *StructureDef) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.byURL[sd.URL] = sd
    return nil
}
```

## Reglas de Decisión

```
¿Modifica el receptor? → SÍ → PUNTERO
                      → NO → ¿Tipo grande o mutex? → SÍ → PUNTERO
                                                   → NO → VALOR
```

## Method Chaining

```go
func (v *Validator) WithTerminologyService(ts TerminologyService) *Validator {
    v.termService = ts
    return v  // Retorna el mismo puntero
}

// Uso:
validator := NewValidator(registry).
    WithTerminologyService(ts).
    WithReferenceResolver(resolver)
```
