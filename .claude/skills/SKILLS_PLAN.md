# Plan de Skills para Desarrollo Go - GoFHIR

## Reglas Fundamentales

### 1. Verificación con Documentación Oficial FHIR

**OBLIGATORIO**: Siempre verificar la información de negocio FHIR con la documentación oficial antes de implementar.

| Recurso | URL | Uso |
|---------|-----|-----|
| FHIR R4 Spec | <https://hl7.org/fhir/R4/> | Especificación base R4 |
| FHIR R4B Spec | <https://hl7.org/fhir/R4B/> | Especificación base R4B |
| FHIR R5 Spec | <https://hl7.org/fhir/R5/> | Especificación base R5 |
| FHIRPath Spec | <https://hl7.org/fhirpath/> | Lenguaje de expresiones |
| FHIR Validator | <https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator> | Referencia del validador |
| Terminology | <https://terminology.hl7.org/> | Sistemas de códigos |
| FHIR Registry | <https://registry.fhir.org/> | IGs publicados |

### 2. Trazabilidad de Información

**OBLIGATORIO**: Documentar siempre de dónde se obtuvo la información.

```markdown
## Referencias
- Fuente: [Nombre del documento/página]
- URL: [URL completa]
- Sección: [Sección específica consultada]
- Fecha de consulta: [YYYY-MM-DD]
- Versión FHIR: [R4/R4B/R5]
```

Ejemplo:

```markdown
## Referencias
- Fuente: FHIR R4 Patient Resource
- URL: https://hl7.org/fhir/R4/patient.html
- Sección: Constraints (dom-2, pat-1)
- Fecha de consulta: 2026-01-24
- Versión FHIR: R4
```

### 3. Architecture Decision Records (ADR)

**OBLIGATORIO**: Crear un ADR para cada decisión arquitectónica o de implementación significativa.

Ubicación: `docs/adr/` o `validator/docs/adr/`

Template ADR:

```markdown
# ADR-XXX: [Título de la Decisión]

## Estado
[Propuesto | Aceptado | Deprecado | Reemplazado por ADR-YYY]

## Contexto
[Descripción del problema o situación que requiere una decisión]

## Decisión
[La decisión tomada y justificación]

## Consecuencias

### Positivas
- [Beneficio 1]
- [Beneficio 2]

### Negativas
- [Trade-off 1]
- [Trade-off 2]

## Referencias
- [URL o documento consultado]
- [Especificación FHIR relevante]

## Fecha
[YYYY-MM-DD]
```

Ejemplo de ADR:

```markdown
# ADR-001: Usar Pipeline Pattern para Validación

## Estado
Aceptado

## Contexto
La validación FHIR requiere múltiples fases (estructura, constraints,
terminología, referencias). Necesitamos una arquitectura extensible
que permita añadir/remover fases sin modificar código existente.

## Decisión
Implementar el patrón Pipeline con fases ordenadas por prioridad.
Cada fase implementa la interface PhaseValidator.

## Consecuencias

### Positivas
- Fácil añadir nuevas fases de validación
- Cada fase es testeable independientemente
- Las fases pueden skippearse según opciones

### Negativas
- Mayor complejidad inicial
- Overhead de coordinación entre fases

## Referencias
- FHIR Validator: https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator
- Diseño interno: validator/ARCHITECTURE_ANALYSIS.md

## Fecha
2026-01-24
```

---

## Análisis del Proyecto

Basado en el análisis exhaustivo del proyecto GoFHIR (fhir, fhirpath, validator), se identificaron los siguientes patrones y prácticas:

### Patrones de Diseño Encontrados
- **Opciones Funcionales** (`WithX()`)
- **Constructores** (`New*`, `MustNew*`)
- **Builder/Fluent** (method chaining)
- **Inyección de Dependencias** (interfaces pequeñas)
- **Pipeline/Chain of Responsibility**
- **Visitor Pattern** (tree walking)
- **Registry Pattern** (registro dinámico)
- **Adapter Pattern** (compatibilidad)
- **Composite Pattern** (composición de servicios)
- **Object Pool** (`sync.Pool`)
- **LRU Cache** (expresiones compiladas)

### Convenciones de Código
- Interfaces pequeñas y específicas
- Receptores de valor para tipos pequeños, punteros para mutables
- Error wrapping con contexto
- Documentación en paquetes y funciones exportadas
- Tests con subtests (`t.Run`)
- Benchmarks con `b.ResetTimer()`

---

## Módulos del Proyecto

Cuando trabajas con FHIR en este proyecto, usa estos módulos:

```go
// Estructuras FHIR tipadas por versión
import "github.com/robertoaraneda/gofhir/r4"      // FHIR R4 (v4.0.1)
import "github.com/robertoaraneda/gofhir/r4b"     // FHIR R4B (v4.3.0)
import "github.com/robertoaraneda/gofhir/r5"      // FHIR R5 (v5.0.0)

// FHIRPath - Evaluación de expresiones de navegación
import "github.com/robertoaraneda/gofhir/fhirpath"

// Validator - Validación de recursos FHIR
import "github.com/robertoaraneda/gofhir/validator"
```

### Cuándo usar cada módulo

| Módulo | Usar cuando... |
|--------|----------------|
| `r4`, `r4b`, `r5` | Necesitas estructuras FHIR tipadas (Patient, Observation, etc.) |
| `fhirpath` | Necesitas evaluar expresiones de navegación sobre recursos |
| `validator` | Necesitas validar recursos contra StructureDefinitions |

---

## Skills Creados

### 1. Planificación y Arquitectura

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-planning` | [go-planning.md](go-planning.md) | Planificación de features y tareas |
| `go-architecture` | [go-architecture.md](go-architecture.md) | Diseño de arquitectura limpia |
| `go-project-structure` | [go-project-structure.md](go-project-structure.md) | Estructura de proyecto y módulos |

### 2. Diseño de Código

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-interfaces` | [go-interfaces.md](go-interfaces.md) | Diseño de interfaces y contratos |
| `go-composition` | [go-composition.md](go-composition.md) | Composición y embedding |
| `go-constructors` | [go-constructors.md](go-constructors.md) | Constructores y opciones funcionales |
| `go-patterns` | [go-patterns.md](go-patterns.md) | Patrones de diseño en Go |
| `go-methods` | [go-methods.md](go-methods.md) | Métodos y receptores |

### 3. Implementación

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-error-handling` | [go-error-handling.md](go-error-handling.md) | Manejo idiomático de errores |
| `go-concurrency` | [go-concurrency.md](go-concurrency.md) | Concurrencia y thread-safety |

### 4. Optimización

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-performance` | [go-performance.md](go-performance.md) | Optimización de rendimiento |
| `go-caching` | [go-caching.md](go-caching.md) | Patrones de caching y pooling |

### 5. Testing y Calidad

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-testing` | [go-testing.md](go-testing.md) | Testing unitario |
| `go-integration-testing` | [go-integration-testing.md](go-integration-testing.md) | Tests de integración |
| `go-benchmarking` | [go-benchmarking.md](go-benchmarking.md) | Benchmarks y profiling |
| `go-code-review` | [go-code-review.md](go-code-review.md) | Revisión de código |

### 6. Documentación y Mantenimiento

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-documentation` | [go-documentation/SKILL.md](go-documentation/SKILL.md) | Documentación de código |
| `go-refactoring` | [go-refactoring/SKILL.md](go-refactoring/SKILL.md) | Refactorización segura |

### 7. Flujo de Desarrollo

| Skill | Archivo | Propósito |
|-------|---------|-----------|
| `go-dev-workflow` | [go-dev-workflow/SKILL.md](go-dev-workflow/SKILL.md) | Comandos make, git hooks, CI local |

---

## Herramientas de Desarrollo

### Setup Inicial

```bash
make tools          # Instalar herramientas (golangci-lint, govulncheck, etc.)
make hooks-install  # Instalar git hooks nativos
make deps           # Descargar dependencias
```

### Git Hooks (Nativos - Sin Python)

| Hook | Cuándo | Qué hace |
|------|--------|----------|
| `pre-commit` | Antes de commit | fmt, vet, lint, detecta debug prints |
| `commit-msg` | Al escribir mensaje | Valida formato Conventional Commits |
| `pre-push` | Antes de push a main | Tests con race detector, govulncheck |

### Comandos Make Principales

```bash
# Desarrollo
make fmt            # Formatear código
make lint           # Linter completo
make test           # Tests con race detector
make test-short     # Tests rápidos
make coverage       # Ver cobertura en navegador

# Calidad
make security       # gosec + govulncheck
make ci             # Ejecutar todo el CI localmente
make ci-quick       # CI rápido

# Benchmarks
make bench          # Ejecutar benchmarks
make bench-baseline # Guardar baseline
make bench-compare  # Comparar con baseline

# Documentación
make doc            # godoc en http://localhost:6060
```

---

## Flujo de Desarrollo Recomendado

```
0. VERIFICACIÓN FHIR (OBLIGATORIO antes de cualquier implementación)
   ├── Consultar especificación oficial FHIR (ver tabla de recursos arriba)
   ├── Documentar URLs, secciones y fecha de consulta
   └── Guardar referencias en el código o ADR

1. PLANIFICACIÓN
   ├── go-planning → Definir scope, requisitos, API
   └── CREAR ADR → docs/adr/ADR-XXX-titulo.md (usar template)

2. DISEÑO
   ├── go-architecture → Definir capas y responsabilidades
   ├── go-interfaces → Diseñar contratos
   ├── go-project-structure → Organizar código
   ├── go-patterns → Seleccionar patrones apropiados
   └── ACTUALIZAR ADR → Documentar alternativas y consecuencias

3. IMPLEMENTACIÓN
   ├── go-constructors → Crear constructores con opciones
   ├── go-composition → Implementar composición
   ├── go-methods → Implementar métodos
   ├── go-error-handling → Manejo de errores
   └── go-concurrency → Thread-safety si aplica

4. OPTIMIZACIÓN
   ├── go-performance → Optimizar hot paths
   └── go-caching → Implementar caching/pooling

5. TESTING
   ├── go-testing → Tests unitarios
   ├── go-integration-testing → Tests de integración
   └── go-benchmarking → Benchmarks

6. REVISIÓN
   ├── go-code-review → Revisión de código
   ├── go-documentation → Documentar
   ├── go-refactoring → Refactorizar si necesario
   └── FINALIZAR ADR → Verificar que está completo y en estado "Aceptado"
```

---

## Referencias del Proyecto

### Ejemplos de Código (para usar en skills)

```
fhirpath/options.go          → Opciones funcionales
fhirpath/cache.go            → LRU Cache
fhirpath/types/pool.go       → Object Pool
fhirpath/funcs/registry.go   → Registry Pattern
validator/pipeline.go        → Pipeline Pattern
validator/treewalker.go      → Visitor Pattern
validator/validator.go       → Builder Pattern
validator/interfaces.go      → Interfaces bien diseñadas
validator/terminology.go     → Composite Pattern
```

### Estructura de Proyecto de Referencia

```
module/
├── module.go           # API pública
├── options.go          # Opciones funcionales
├── interfaces.go       # Interfaces públicas
├── cache.go            # Caching
├── types.go            # Tipos exportados
├── errors.go           # Errores personalizados
├── module_test.go      # Tests unitarios
├── benchmark_test.go   # Benchmarks
└── internal/           # Implementaciones internas
    ├── subpackage/
    └── ...
```

---

## Ejemplos de Uso Rápido

### FHIRPath

```go
import "github.com/robertoaraneda/gofhir/fhirpath"

// Evaluar expresión simple
result, _ := fhirpath.Evaluate(patient, "Patient.name.family")

// Compilar para reutilizar
expr := fhirpath.MustCompile("Patient.name.where(use='official').family")
result, _ := expr.Evaluate(patient)

// Con opciones
result, _ := expr.EvaluateWithOptions(patient,
    fhirpath.WithTimeout(5*time.Second),
)
```

### Validator

```go
import "github.com/robertoaraneda/gofhir/validator"

// Crear validador R4
v, _ := validator.NewInitializedValidatorR4(ctx, validator.ValidatorOptions{
    ValidateConstraints: true,
    ValidateTerminology: true,
})

// Validar recurso
result, _ := v.Validate(ctx, patientJSON)
if result.Valid {
    fmt.Println("Recurso válido")
}

// Con servicios custom
v = v.WithTerminologyService(myTermService).
    WithReferenceResolver(myResolver)
```

### Tipos FHIR

```go
import "github.com/robertoaraneda/gofhir/r4"

// Crear paciente tipado
patient := &r4.Patient{
    Id: "123",
    Name: []r4.HumanName{
        {
            Use:    "official",
            Family: "Doe",
            Given:  []string{"John"},
        },
    },
}

// Serializar a JSON
data, _ := json.Marshal(patient)
```

---

## Checklist de Desarrollo

```markdown
### Nueva Feature

#### Paso 0: Verificación FHIR (OBLIGATORIO)
- [ ] Consultar especificación oficial FHIR
- [ ] Documentar URL, sección y fecha de consulta
- [ ] Crear ADR en docs/adr/ADR-XXX-titulo.md

#### Paso 1: Planificación
- [ ] Revisar go-planning para planificar
- [ ] Definir API con go-constructors
- [ ] Diseñar interfaces con go-interfaces
- [ ] Elegir patrones con go-patterns

#### Paso 2: Implementación
- [ ] Implementar con go-methods
- [ ] Manejar errores con go-error-handling
- [ ] Tests con go-testing

#### Paso 3: Finalización
- [ ] Documentar con go-documentation
- [ ] Actualizar ADR con estado "Aceptado"
- [ ] Verificar referencias FHIR en ADR

### Optimización
- [ ] Medir con go-benchmarking
- [ ] Identificar hotspots
- [ ] Aplicar go-performance
- [ ] Implementar go-caching si aplica
- [ ] Verificar mejora con benchmarks
- [ ] Documentar decisión en ADR si es cambio significativo

### Code Review
- [ ] Usar checklist de go-code-review
- [ ] Verificar patrones del proyecto
- [ ] Asegurar tests
- [ ] Revisar documentación
- [ ] Verificar que existe ADR si es cambio arquitectónico
- [ ] Verificar referencias a spec FHIR
```
