---
name: go-documentation
description: Documentación de código en Go. Usar al documentar paquetes, APIs públicas, o crear ejemplos de uso.
allowed-tools: Read, Grep, Glob
---

# Skill: go-documentation

Documentación de código en Go.

## Cuándo usar este skill

- Al documentar paquetes nuevos
- Al documentar APIs públicas
- Al crear ejemplos de uso

## Comentario de Paquete

```go
// Package validator provides FHIR resource validation.
//
// # Quick Start
//
//  v, _ := validator.NewInitializedValidatorR4(ctx, opts)
//  result, _ := v.Validate(ctx, resource)
//
// # Configuration
//
// Configure with ValidatorOptions...
package validator
```

## Documentación de Funciones

```go
// Validate validates a FHIR resource and returns the validation result.
//
// The resource must be a valid JSON-encoded FHIR resource.
// Validate is safe for concurrent use.
//
// Example:
//
//  result, err := v.Validate(ctx, patientJSON)
//  if result.Valid {
//      fmt.Println("Valid!")
//  }
func (v *Validator) Validate(ctx context.Context, resource []byte) (*ValidationResult, error)
```

## Ejemplos Ejecutables

```go
// example_test.go
func Example() {
    v, _ := validator.NewInitializedValidatorR4(ctx, opts)
    result, _ := v.Validate(ctx, patient)
    fmt.Println(result.Valid)
    // Output: true
}
```

## Generar Documentación

```bash
go doc github.com/user/pkg.Function
godoc -http=:6060
```
