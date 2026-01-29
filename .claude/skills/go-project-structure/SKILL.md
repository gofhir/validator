---
name: go-project-structure
description: Estructura de proyecto y módulos en Go. Usar al crear nuevos proyectos u organizar código existente.
allowed-tools: Read, Grep, Glob
---

# Skill: go-project-structure

Estructura de proyecto y módulos en Go.

## Cuándo usar este skill

- Al crear nuevos proyectos
- Al organizar código existente
- Al definir estructura de paquetes

## Estructura del Proyecto GoFHIR

```
gofhir/
├── go.work                    # Go workspace
├── go.mod
├── CLAUDE.md                  # Instrucciones globales
│
├── fhirpath/                  # FHIRPath evaluator
│   ├── go.mod
│   ├── fhirpath.go           # API pública
│   ├── options.go            # Opciones funcionales
│   └── internal/             # Implementación privada
│
├── validator/                 # FHIR Validator
│   ├── go.mod
│   ├── validator.go
│   ├── interfaces.go
│   └── internal/
│
├── r4/, r4b/, r5/            # Tipos FHIR por versión
│
├── docs/adr/                  # ADRs
│
└── .claude/
    └── skills/               # Skills de desarrollo
```

## Reglas de Organización

### 1. Un Paquete = Una Responsabilidad

```
validator/   # Validación
fhirpath/    # Expresiones
terminology/ # Terminología
```

### 2. Internal para Privado

```go
// mypackage/internal/core/processor.go
// NO accesible desde fuera de mypackage
```

### 3. Nombrado de Paquetes

```go
package validator   // ✅ BIEN
package fhirvalidator  // ❌ Redundante
```

## Archivos Especiales

- `doc.go` - Documentación del paquete
- `interfaces.go` - Interfaces públicas
- `options.go` - Opciones funcionales
- `errors.go` - Tipos de error
