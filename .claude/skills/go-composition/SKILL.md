---
name: go-composition
description: Composición, embedding y extensibilidad en Go. Usar al extender tipos existentes o combinar comportamientos.
allowed-tools: Read, Grep, Glob
---

# Skill: go-composition

Composición, embedding y extensibilidad en Go.

## Cuándo usar este skill

- Al extender tipos existentes
- Al combinar comportamientos
- Al implementar herencia de comportamiento

## Struct Embedding

```go
type BaseValidator struct {
    name string
}

func (b *BaseValidator) Name() string { return b.name }

type TerminologyValidator struct {
    BaseValidator  // Embedding
    service TerminologyService
}

// TerminologyValidator hereda Name() automáticamente
```

## Interface Embedding

```go
type Reader interface { Read(...) }
type Writer interface { Write(...) }

type ReadWriter interface {
    Reader
    Writer
}
```

## Composición de Servicios

```go
type CompositeTerminologyService struct {
    services []TerminologyService
}

func (c *CompositeTerminologyService) ValidateCode(...) (bool, error) {
    for _, svc := range c.services {
        valid, err := svc.ValidateCode(...)
        if err == nil {
            return valid, nil
        }
    }
    return false, errors.New("no service could validate")
}
```

## Decorator Pattern

```go
type LoggingHandler struct {
    wrapped Handler
    logger  Logger
}

func (l *LoggingHandler) Handle(ctx context.Context, req Request) (Response, error) {
    l.logger.Info("handling request")
    return l.wrapped.Handle(ctx, req)
}
```
