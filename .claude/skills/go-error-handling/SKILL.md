---
name: go-error-handling
description: Manejo idiomático de errores en Go. Usar al diseñar errores para APIs, propagar errores entre capas, o crear tipos de error personalizados.
allowed-tools: Read, Grep, Glob
---

# Skill: go-error-handling

Manejo idiomático de errores en Go.

## Cuándo usar este skill

- Al diseñar errores para una API
- Al propagar errores entre capas
- Al crear tipos de error personalizados
- Cuando se necesita contexto en errores

## Patrones de Errores

### Error Wrapping

```go
// ✅ BIEN: Añadir contexto
if err != nil {
    return fmt.Errorf("loading profile %s: %w", url, err)
}

// ❌ MAL: Sin contexto
if err != nil {
    return err
}
```

### Tipos de Error Personalizados

```go
type EvalError struct {
    Type       ErrorType
    Message    string
    Position   Position
    Underlying error
}

func (e *EvalError) Error() string {
    return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *EvalError) Unwrap() error {
    return e.Underlying
}
```

### Error Sentinels

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrInvalidInput = errors.New("invalid input")
)

// Verificar tipo de error
if errors.Is(err, ErrNotFound) {
    // Manejar caso no encontrado
}
```

### Usar errors.Is y errors.As

```go
// ✅ BIEN
if errors.Is(err, ErrNotFound) { }

var evalErr *EvalError
if errors.As(err, &evalErr) {
    log.Printf("Error at line %d", evalErr.Position.Line)
}
```

## Errores en el Validator FHIR

```go
type Issue struct {
    Severity    Severity  // error, warning, information
    Code        string
    Diagnostics string
    Location    []string
}

type ValidationResult struct {
    Valid  bool
    Issues []Issue
}
```

## Checklist

```markdown
- [ ] ¿Los errores tienen contexto suficiente?
- [ ] ¿Se usa %w para wrapping?
- [ ] ¿Se implementa Unwrap() para chains?
- [ ] ¿Se usa errors.Is/As en lugar de comparación?
```
