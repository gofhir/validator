---
name: go-architecture
description: Diseño de arquitectura limpia y modular en Go. Usar al diseñar un nuevo módulo, definir capas y responsabilidades, u organizar código.
allowed-tools: Read, Grep, Glob
---

# Skill: go-architecture

Arquitectura de GoFHIR.

## Estructura del Proyecto

```
gofhir/
├── r4/, r4b/, r5/     # FHIR types por versión
├── fhirpath/          # FHIRPath evaluator
│   ├── fhirpath.go    # API pública
│   ├── cache.go       # Expression cache
│   ├── funcs/         # Registry de funciones
│   └── types/         # Sistema de tipos
├── validator/         # FHIR Validator
│   ├── validator.go   # API pública
│   ├── pipeline.go    # Pipeline pattern
│   ├── phases.go      # Fases de validación
│   └── internal/      # Implementaciones
└── docs/adr/          # ADRs
```

## Arquitectura del Validator

```
API: NewInitializedValidatorR4(), Validate()
         ↓
    Pipeline (ejecuta fases)
         ↓
    Phases: Structure → Constraints → Terminology → Extensions
         ↓
    Services: Registry, FHIRPath, TerminologyService
```

## Patrones Usados

- **Pipeline**: Fases ordenadas de validación
- **Registry**: Funciones FHIRPath, StructureDefinitions
- **Visitor**: TreeWalker para recorrer JSON
- **Composite**: Servicios de terminología combinados

## Inyección de Dependencias

```go
// Constructor con dependencias requeridas
v := NewValidator(registry, opts)

// Builder para opcionales
v = v.WithTerminologyService(ts).
      WithReferenceResolver(resolver)
```

## Referencias

```text
validator/pipeline.go, validator/interfaces.go
fhirpath/funcs/registry.go
validator/internal/
```
