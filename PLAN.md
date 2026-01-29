# Plan Maestro: GoFHIR Validator

## Resumen Ejecutivo

Desarrollar un validador FHIR de alto rendimiento en Go que sea 100% dinámico (basado en StructureDefinitions), agnóstico de versión, y comparable en funcionalidad con el HL7 Validator oficial.

---

## Librerías del Ecosistema GoFHIR

### Disponibles (Ya Implementadas)

| Librería | Versión | Propósito | Impacto |
|----------|---------|-----------|---------|
| `github.com/gofhir/fhirpath` | v1.x | Motor FHIRPath 2.0 completo | Elimina necesidad de implementar FHIRPath |
| `github.com/gofhir/fhir` | v1.x | Structs R4/R4B/R5 | Provee modelos tipados para StructureDefinition |

### Por Desarrollar

| Librería | Propósito |
|----------|-----------|
| `github.com/gofhir/validator` | **Este proyecto** - Validador FHIR |

---

## Sistema de Paquetes FHIR (Carga Automática)

### Cómo funciona HL7 Validator

Al seleccionar `-version 4.0.1`, HL7 validator carga automáticamente:

```
Loading FHIR v4.0.1 from hl7.fhir.r4.core#4.0.1
  Load hl7.terminology.r4#6.2.0 - 4288 resources
  Load hl7.fhir.uv.extensions.r4#5.2.0 - 759 resources
  Loaded FHIR - 8265 resources
```

### Paquetes por Versión FHIR

| Versión | Core Package | Terminology | Extensions | Total Resources |
|---------|--------------|-------------|------------|-----------------|
| **R4** (4.0.1) | `hl7.fhir.r4.core#4.0.1` | `hl7.terminology.r4#7.0.1` | `hl7.fhir.uv.extensions.r4#5.2.0` | ~9,000 |
| **R4B** (4.3.0) | `hl7.fhir.r4b.core#4.3.0` | `hl7.terminology.r4#7.0.1` | `hl7.fhir.uv.extensions.r4b#5.2.0` | ~9,000 |
| **R5** (5.0.0) | `hl7.fhir.r5.core#5.0.0` | `hl7.terminology.r5#6.5.0` | `hl7.fhir.uv.extensions.r5#5.2.0` | ~9,500 |

### Catálogo de Paquetes FHIR Disponibles

**Core (por versión):**
| Paquete | Descripción |
|---------|-------------|
| `hl7.fhir.r4.core` | StructureDefinitions, SearchParameters, OperationDefinitions |
| `hl7.fhir.r4.examples` | Recursos de ejemplo (~192MB) |
| `hl7.fhir.r4.expansions` | ValueSet expansions pre-computadas |
| `hl7.fhir.r4.elements` | ElementDefinitions separados |

**Terminología:**
| Paquete | Descripción | Tamaño |
|---------|-------------|--------|
| `hl7.terminology.r4` | CodeSystems y ValueSets para R4 | ~71MB |
| `hl7.terminology.r5` | CodeSystems y ValueSets para R5 | ~73MB |
| `hl7.terminology` | Versión unificada (R5 format) | ~71MB |

**Extensiones Globales:**
| Paquete | Descripción |
|---------|-------------|
| `hl7.fhir.uv.extensions.r4` | Extensiones universales para R4 |
| `hl7.fhir.uv.extensions.r4b` | Extensiones universales para R4B |
| `hl7.fhir.uv.extensions.r5` | Extensiones universales para R5 |
| `hl7.fhir.xver-extensions` | Extensiones cross-version |

### Nuestra Implementación

```go
// Configuración de paquetes por versión
var DefaultPackages = map[string][]PackageRef{
    "4.0.1": {
        {Name: "hl7.fhir.r4.core", Version: "4.0.1"},
        {Name: "hl7.terminology.r4", Version: "7.0.1"},
        {Name: "hl7.fhir.uv.extensions.r4", Version: "5.2.0"},
    },
    "4.3.0": {
        {Name: "hl7.fhir.r4b.core", Version: "4.3.0"},
        {Name: "hl7.terminology.r4", Version: "7.0.1"},
        {Name: "hl7.fhir.uv.extensions.r4b", Version: "5.2.0"},
    },
    "5.0.0": {
        {Name: "hl7.fhir.r5.core", Version: "5.0.0"},
        {Name: "hl7.terminology.r5", Version: "6.5.0"},
        {Name: "hl7.fhir.uv.extensions.r5", Version: "5.2.0"},
    },
}

// Uso
v, err := validator.New(
    validator.WithVersion("4.0.1"),  // Carga automática de paquetes R4
)

// O cargar paquetes específicos
v, err := validator.New(
    validator.WithPackages(
        "hl7.fhir.r4.core#4.0.1",
        "hl7.fhir.us.core#6.1.0",  // US Core IG adicional
    ),
)
```

### Ubicación de Paquetes

```
~/.fhir/packages/
├── packages.ini                          # Metadata del cache (version=4)
├── hl7.fhir.r4.core#4.0.1/
│   └── package/
│       ├── .index.json                   # Índice de recursos
│       ├── StructureDefinition-*.json    # 668 StructureDefinitions
│       ├── ValueSet-*.json               # ValueSets
│       ├── CodeSystem-*.json             # CodeSystems
│       └── ...
├── hl7.terminology.r4#7.0.1/
│   └── package/
│       └── ...                           # 4,066 resources
└── hl7.fhir.uv.extensions.r4#5.2.0/
    └── package/
        └── ...                           # 759 resources
```

### Descarga Automática

```go
// Si el paquete no existe localmente, descargarlo
loader := NewPackageLoader(
    WithCachePath("~/.fhir/packages"),
    WithRegistry("https://packages.fhir.org"),
    WithAutoDownload(true),  // Descargar si no existe
)

// O modo offline estricto
loader := NewPackageLoader(
    WithCachePath("~/.fhir/packages"),
    WithAutoDownload(false),  // Error si no existe
)
```

---

## Capacidades de gofhir/fhirpath

```go
// Evaluación directa
result, err := fhirpath.Evaluate(resourceJSON, "Patient.name.given")

// Evaluación booleana (para constraints)
valid, err := fhirpath.EvaluateToBoolean(resourceJSON, "name.exists()")

// Expresión compilada (reutilizable)
expr := fhirpath.MustCompile("identifier.exists()")
result, err := expr.Evaluate(resourceJSON)

// Con cache automático
result, err := fhirpath.EvaluateCached(resourceJSON, expression)
```

**Funciones disponibles:** 70+ funciones incluyendo string, math, collection, temporal, type conversion.

**UCUM:** Normalización de unidades (`1000 'mg' = 1 'g'`).

---

## Capacidades de gofhir/fhir

```go
import "github.com/gofhir/fhir/r4"

// Parsear StructureDefinition
var sd r4.StructureDefinition
json.Unmarshal(data, &sd)

// Acceder a ElementDefinitions
for _, elem := range sd.Snapshot.Element {
    path := *elem.Path
    min := elem.Min        // *uint32
    max := elem.Max        // *string ("*" para unbounded)
    types := elem.Type     // []ElementDefinitionType
    binding := elem.Binding
    constraints := elem.Constraint
    // ... todo derivado del struct
}
```

---

## Formato de OperationOutcome (Igual que HL7 Validator)

### Estructura de Issue

Cada issue incluye el **FHIRPath** en el campo `expression`:

```json
{
  "resourceType": "OperationOutcome",
  "issue": [
    {
      "severity": "error",
      "code": "invalid",
      "details": {
        "text": "Not a valid date format: 'invalid-date'"
      },
      "expression": ["Patient.birthDate"],
      "extension": [
        {
          "url": "http://hl7.org/fhir/StructureDefinition/operationoutcome-issue-line",
          "valueInteger": 10
        },
        {
          "url": "http://hl7.org/fhir/StructureDefinition/operationoutcome-issue-col",
          "valueInteger": 30
        },
        {
          "url": "http://hl7.org/fhir/StructureDefinition/operationoutcome-issue-source",
          "valueString": "InstanceValidator"
        },
        {
          "url": "http://hl7.org/fhir/StructureDefinition/operationoutcome-message-id",
          "valueCode": "Type_Specific_Checks_DT_Date_Valid"
        }
      ]
    }
  ]
}
```

### Campos del Issue

| Campo | Tipo | Descripción | Ejemplo |
|-------|------|-------------|---------|
| `severity` | code | error, warning, information, fatal | `"error"` |
| `code` | code | Tipo de issue (IssueType) | `"invalid"`, `"required"`, `"code-invalid"` |
| `details.text` | string | Mensaje descriptivo | `"Not a valid date format"` |
| `expression` | string[] | **FHIRPath al elemento** | `["Patient.birthDate"]`, `["Patient.name[0].given"]` |

### Extensions Estándar

| Extension URL | Tipo | Descripción |
|---------------|------|-------------|
| `operationoutcome-issue-line` | integer | Línea en el archivo fuente |
| `operationoutcome-issue-col` | integer | Columna en el archivo fuente |
| `operationoutcome-issue-source` | string | Componente que generó el issue |
| `operationoutcome-message-id` | code | ID del mensaje (para i18n) |

### Issue Codes (IssueType)

| Code | Cuándo Usar |
|------|-------------|
| `invalid` | Valor inválido (formato, tipo) |
| `required` | Elemento requerido faltante (cardinalidad min) |
| `value` | Valor fuera de rango |
| `code-invalid` | Código no encontrado en ValueSet |
| `not-found` | Sistema de código no encontrado |
| `invariant` | Constraint/invariante falló |
| `structure` | Error estructural |
| `business-rule` | Regla de negocio violada |

### Nuestra Implementación Go

```go
// Issue representa un problema de validación
type Issue struct {
    Severity    IssueSeverity  // error, warning, information
    Code        IssueType      // invalid, required, code-invalid, etc.
    Diagnostics string         // Mensaje descriptivo
    Expression  []string       // FHIRPath(s) al elemento
    Line        int            // Línea en el fuente (opcional)
    Column      int            // Columna en el fuente (opcional)
    Source      string         // Fase que generó el issue
    MessageID   string         // ID para i18n (opcional)
}

// Result contiene el resultado de validación
type Result struct {
    Valid    bool
    Issues   []Issue
    Warnings int
    Errors   int
}

// ToOperationOutcome convierte a FHIR OperationOutcome
func (r *Result) ToOperationOutcome() *r4.OperationOutcome {
    oo := &r4.OperationOutcome{
        ResourceType: "OperationOutcome",
    }
    for _, issue := range r.Issues {
        ooIssue := r4.OperationOutcomeIssue{
            Severity:   &issue.Severity,
            Code:       &issue.Code,
            Details:    &r4.CodeableConcept{Text: &issue.Diagnostics},
            Expression: issue.Expression,
        }
        // Añadir extensions para line/col/source
        oo.Issue = append(oo.Issue, ooIssue)
    }
    return oo
}
```

---

## Arquitectura del Validador

```
github.com/gofhir/validator/
├── validator.go           # API pública principal
├── options.go             # Opciones funcionales
├── result.go              # OperationOutcome, Issue
├── context.go             # Contexto de validación
│
├── loader/                # Carga de StructureDefinitions
│   ├── loader.go          # Interface PackageLoader
│   ├── npm.go             # Carga desde ~/.fhir/packages/
│   ├── registry.go        # Registro de definiciones
│   └── snapshot.go        # Generador de snapshots
│
├── schema/                # Abstracción sobre ElementDefinition
│   ├── element.go         # Wrapper con helpers
│   ├── types.go           # Resolución de tipos
│   └── path.go            # Manejo de paths FHIR
│
├── phase/                 # Fases de validación
│   ├── phase.go           # Interface Phase
│   ├── pipeline.go        # Orquestador de fases
│   ├── structure.go       # Elementos válidos
│   ├── cardinality.go     # Min/Max
│   ├── types.go           # Tipos permitidos
│   ├── fixed.go           # Fixed/Pattern values
│   ├── binding.go         # Terminología
│   ├── constraint.go      # FHIRPath constraints
│   ├── reference.go       # Referencias
│   ├── extension.go       # Extensiones
│   ├── slicing.go         # Slicing rules
│   └── invariant.go       # Invariantes
│
├── terminology/           # Validación de terminología
│   ├── service.go         # Interface TerminologyService
│   ├── local.go           # Validación local (CodeSystem/ValueSet)
│   └── client.go          # Cliente TX server (opcional)
│
├── walker/                # Tree traversal
│   ├── walker.go          # Recorrido de recursos
│   └── element.go         # Elemento navegable
│
├── cache/                 # Caching
│   └── lru.go             # Cache LRU genérico
│
├── pool/                  # Object pooling
│   └── pool.go            # sync.Pool wrappers
│
├── worker/                # Concurrencia
│   └── pool.go            # Worker pool para batch
│
├── compare/               # Comparación con HL7 validator
│   ├── compare.go         # Ejecutar comparaciones
│   └── report.go          # Generar reportes
│
└── cmd/
    └── gofhir-validator/  # CLI
        ├── main.go
        ├── validate.go
        └── compare.go
```

---

## API Pública Propuesta

### Uso Básico

```go
import "github.com/gofhir/validator"

// Crear validador con contexto R4
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithPackagePath("~/.fhir/packages"),
)

// Validar recurso (JSON bytes)
result, err := v.Validate(ctx, patientJSON)
if !result.IsValid() {
    for _, issue := range result.Issues {
        fmt.Printf("[%s] %s: %s\n",
            issue.Severity,  // error, warning, information
            issue.Expression, // Patient.birthDate
            issue.Diagnostics,
        )
    }
}
```

### Uso con Perfiles

```go
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithProfiles(
        "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient",
    ),
)

result, err := v.Validate(ctx, patientJSON)
```

### Uso Avanzado

```go
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithTerminologyService(txService),
    validator.WithCache(cache),
    validator.WithLogger(logger),
)

result, err := v.ValidateWithOptions(ctx, resource,
    validator.CheckReferences(true),
    validator.BindingStrength(validator.Required),
    validator.StopOnFirstError(false),
    validator.MaxIssues(100),
)
```

### Validación Batch

```go
// Validar múltiples recursos en paralelo
results, err := v.ValidateBatch(ctx, resources,
    validator.Concurrency(runtime.NumCPU()),
    validator.BatchSize(100),
)

// Streaming para grandes volúmenes
for result := range v.ValidateStream(ctx, resourceChan) {
    // Procesar resultado
}
```

---

## Plan de Fases de Implementación

### Fase 0: Setup del Proyecto
**Skill:** `/go-project-structure`, `/go-dev-workflow`

| # | Tarea | Descripción |
|---|-------|-------------|
| 0.1 | go.mod | `github.com/gofhir/validator` con dependencias |
| 0.2 | Estructura | Crear directorios según arquitectura |
| 0.3 | Makefile | Targets: test, bench, lint, build |
| 0.4 | golangci-lint | Configuración estricta |
| 0.5 | CI/CD | GitHub Actions básico |

**Dependencias iniciales:**
```go
require (
    github.com/gofhir/fhir v1.0.0
    github.com/gofhir/fhirpath v1.0.0
    github.com/spf13/cobra v1.8.0
    github.com/rs/zerolog v1.31.0
)
```

---

### Fase 1: Loader de Paquetes FHIR
**Skill:** `/go-interfaces`, `/go-error-handling`

**Objetivo:** Cargar StructureDefinitions desde paquetes NPM FHIR.

```go
// Interfaces
type PackageLoader interface {
    LoadPackage(ctx context.Context, name, version string) (*Package, error)
    ListPackages(ctx context.Context) ([]PackageInfo, error)
}

type DefinitionRegistry interface {
    GetStructureDefinition(url string) (*r4.StructureDefinition, error)
    GetValueSet(url string) (*r4.ValueSet, error)
    GetCodeSystem(url string) (*r4.CodeSystem, error)
}
```

| # | Tarea |
|---|-------|
| 1.1 | Parsear `~/.fhir/packages/{name}#{version}/package/` |
| 1.2 | Indexar recursos desde `.index.json` |
| 1.3 | Cargar StructureDefinitions bajo demanda |
| 1.4 | Resolver URLs a definiciones |
| 1.5 | Cache de definiciones cargadas |
| 1.6 | Generar snapshot desde differential (si falta) |

---

### Fase 2: Walker de Recursos
**Skill:** `/go-patterns`, `/go-performance`

**Objetivo:** Recorrer recursos JSON con contexto de tipo.

```go
// Elemento navegable (dinámico, no tipado)
type Element struct {
    Name     string
    Path     string
    Value    any           // primitivo o nil
    Children []*Element    // sub-elementos
    Index    int           // posición en array (-1 si no es array)
}

type Walker interface {
    Walk(ctx context.Context, resource []byte, visitor Visitor) error
}

type Visitor interface {
    Enter(ctx *WalkContext, elem *Element) error
    Leave(ctx *WalkContext, elem *Element) error
}

type WalkContext struct {
    Path          string                      // "Patient.name[0].given[0]"
    ElementDef    *r4.ElementDefinition       // definición del elemento
    Profile       *r4.StructureDefinition     // perfil activo
    Parent        *WalkContext
}
```

| # | Tarea |
|---|-------|
| 2.1 | Parser JSON → Element tree |
| 2.2 | Mapear Element → ElementDefinition |
| 2.3 | Resolver choice types (`value[x]`) |
| 2.4 | Manejar arrays con índices |
| 2.5 | Pool de contextos para performance |

---

### Fase 3: Pipeline de Validación
**Skill:** `/go-architecture`, `/go-composition`

**Objetivo:** Framework extensible de fases.

```go
type Phase interface {
    Name() string
    Priority() int  // menor = primero
    Validate(ctx *ValidationContext, elem *Element) []Issue
}

type Pipeline struct {
    phases   []Phase
    registry DefinitionRegistry
}

func (p *Pipeline) Execute(ctx context.Context, resource []byte) (*Result, error) {
    // 1. Parse resource
    // 2. Determine profile(s)
    // 3. Walk resource, executing phases
    // 4. Collect issues
}
```

| # | Tarea |
|---|-------|
| 3.1 | Interface Phase |
| 3.2 | Pipeline con prioridades ordenadas |
| 3.3 | ValidationContext con acumulador de issues |
| 3.4 | Result con OperationOutcome |

---

### Fase 4: Fases de Validación Core
**Skill:** `/go-testing`, `/validator-compare`

**Prioridad de ejecución:**

| Prioridad | Fase | Descripción | Source |
|-----------|------|-------------|--------|
| 100 | **StructurePhase** | Elementos válidos según profile | ElementDefinition.path |
| 200 | **CardinalityPhase** | min ≤ count ≤ max | ElementDefinition.min/max |
| 300 | **TypePhase** | Tipo correcto | ElementDefinition.type |
| 400 | **FixedPhase** | Valores fijos/patrones | ElementDefinition.fixed[x]/pattern[x] |
| 500 | **PrimitivePhase** | Formato de primitivos | Regex según tipo |
| 600 | **ReferencePhase** | Referencias válidas | ElementDefinition.type.targetProfile |
| 700 | **ExtensionPhase** | Extensiones válidas | Extension.url → profile |
| 800 | **SlicingPhase** | Discriminadores | ElementDefinition.slicing |
| 900 | **BindingPhase** | Terminología | ElementDefinition.binding |
| 1000 | **ConstraintPhase** | FHIRPath constraints | ElementDefinition.constraint |

**Para cada fase:**
```go
// Ejemplo: CardinalityPhase
type CardinalityPhase struct{}

func (p *CardinalityPhase) Name() string { return "cardinality" }
func (p *CardinalityPhase) Priority() int { return 200 }

func (p *CardinalityPhase) Validate(ctx *ValidationContext, elem *Element) []Issue {
    ed := ctx.ElementDef
    if ed == nil {
        return nil
    }

    count := len(elem.Children) // o 1 si es primitivo

    // Verificar min (derivado de ElementDefinition, NO hardcoded)
    if ed.Min != nil && uint32(count) < *ed.Min {
        return []Issue{{
            Severity:    IssueSeverityError,
            Code:        IssueTypeRequired,
            Expression:  []string{elem.Path},
            Diagnostics: fmt.Sprintf("minimum cardinality %d not met", *ed.Min),
        }}
    }

    // Verificar max
    if ed.Max != nil && *ed.Max != "*" {
        maxVal, _ := strconv.Atoi(*ed.Max)
        if count > maxVal {
            // ...
        }
    }

    return nil
}
```

---

### Fase 5: Integración FHIRPath (Constraints)
**Skill:** `/go-interfaces`

**Objetivo:** Usar gofhir/fhirpath para evaluar constraints.

```go
// ConstraintPhase
func (p *ConstraintPhase) Validate(ctx *ValidationContext, elem *Element) []Issue {
    ed := ctx.ElementDef
    if ed == nil || len(ed.Constraint) == 0 {
        return nil
    }

    var issues []Issue

    for _, constraint := range ed.Constraint {
        if constraint.Expression == nil {
            continue
        }

        // Usar gofhir/fhirpath
        valid, err := fhirpath.EvaluateToBoolean(
            ctx.ResourceJSON,
            *constraint.Expression,
        )

        if err != nil {
            issues = append(issues, Issue{
                Severity:    IssueSeverityWarning,
                Code:        IssueTypeInvariant,
                Diagnostics: fmt.Sprintf("constraint %s failed to evaluate: %v",
                    *constraint.Key, err),
            })
            continue
        }

        if !valid {
            severity := IssueSeverityWarning
            if constraint.Severity != nil && *constraint.Severity == "error" {
                severity = IssueSeverityError
            }

            issues = append(issues, Issue{
                Severity:    severity,
                Code:        IssueTypeInvariant,
                Expression:  []string{elem.Path},
                Diagnostics: fmt.Sprintf("constraint %s: %s",
                    *constraint.Key,
                    safeString(constraint.Human)),
            })
        }
    }

    return issues
}
```

---

### Fase 6: Terminología
**Skill:** `/go-interfaces`, `/go-caching`

```go
type TerminologyService interface {
    ValidateCode(ctx context.Context, params ValidateCodeParams) (bool, error)
    Expand(ctx context.Context, valueSetURL string) ([]Coding, error)
}

// Implementación local (offline)
type LocalTerminologyService struct {
    registry DefinitionRegistry
    cache    *lru.Cache
}

// BindingPhase usa TerminologyService
func (p *BindingPhase) Validate(ctx *ValidationContext, elem *Element) []Issue {
    binding := ctx.ElementDef.Binding
    if binding == nil || binding.ValueSet == nil {
        return nil
    }

    strength := BindingStrengthPreferred
    if binding.Strength != nil {
        strength = *binding.Strength
    }

    // Solo validar required y extensible
    if strength != "required" && strength != "extensible" {
        return nil
    }

    valid, err := p.terminology.ValidateCode(ctx.Context, ValidateCodeParams{
        ValueSetURL: *binding.ValueSet,
        Code:        extractCode(elem),
    })

    // Generar issue según strength
}
```

---

### Fase 7: CLI
**Skill:** `/go-constructors`

```bash
# Instalación
go install github.com/gofhir/validator/cmd/gofhir-validator@latest

# Uso básico
gofhir-validator patient.json

# Con versión específica
gofhir-validator -version 4.0.1 patient.json

# Con profile
gofhir-validator -profile http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# Output JSON (OperationOutcome)
gofhir-validator -output json patient.json

# Comparar con HL7 validator
gofhir-validator -compare patient.json

# Batch
gofhir-validator ./resources/*.json

# Desde stdin
cat patient.json | gofhir-validator -
```

**Equivalencias con HL7 Validator:**

| gofhir-validator | HL7 validator | Descripción |
|------------------|---------------|-------------|
| `-version 4.0.1` | `-version 4.0.1` | Versión FHIR |
| `-profile <url>` | `-profile <url>` | Profile específico |
| `-ig <path>` | `-ig <path>` | Implementation Guide |
| `-output json` | `-output json` | Formato salida |
| `-tx n/a` | `-tx n/a` | Sin terminología |
| `-compare` | N/A | Comparar con JAR |

---

### Fase 8: Performance y Concurrencia
**Skill:** `/go-concurrency`, `/go-performance`, `/go-caching`, `/go-benchmarking`

| # | Optimización | Técnica |
|---|--------------|---------|
| 8.1 | Object pooling | sync.Pool para Element, Context |
| 8.2 | Cache de SD | LRU para StructureDefinitions |
| 8.3 | Worker pool | Para ValidateBatch |
| 8.4 | Zero-copy JSON | goccy/go-json |
| 8.5 | Lazy loading | Cargar SDs bajo demanda |

**Benchmarks objetivo:**
```
BenchmarkValidatePatient-8          10000        89234 ns/op      12KB/op
BenchmarkValidateBatch100-8           500      2100000 ns/op     890KB/op
BenchmarkLoadR4Core-8                  10    102000000 ns/op     48MB/op
```

---

### Fase 9: Conformance Testing
**Skill:** `/validator-compare`, `/go-integration-testing`

```go
// compare/compare.go
func CompareWithHL7Validator(resource []byte, jar string) (*CompareResult, error) {
    // 1. Ejecutar nuestro validador
    ourResult, err := validator.Validate(ctx, resource)

    // 2. Ejecutar HL7 validator JAR
    cmd := exec.Command("java", "-jar", jar, "-output", "json", "-")
    cmd.Stdin = bytes.NewReader(resource)
    hl7Output, err := cmd.Output()

    // 3. Parsear OperationOutcome de HL7
    var hl7Result OperationOutcome
    json.Unmarshal(hl7Output, &hl7Result)

    // 4. Comparar issues
    return compareIssues(ourResult.Issues, hl7Result.Issue)
}
```

**Suite de tests:**
- Ejemplos oficiales FHIR R4
- Casos edge de cardinalidad
- Constraints FHIRPath complejos
- Slicing con discriminadores
- Perfiles US Core
- Datos Synthea

---

## Métricas de Éxito

| Métrica | Objetivo |
|---------|----------|
| Conformance con HL7 | ≥99% match en casos estándar |
| Validación Patient | <100ms |
| Throughput batch | >1000 recursos/seg |
| Code coverage | ≥85% |
| Memoria R4 core | <100MB |

---

## Dependencias Finales

```go
require (
    github.com/gofhir/fhir v1.0.0
    github.com/gofhir/fhirpath v1.0.0
    github.com/goccy/go-json v0.10.2
    github.com/spf13/cobra v1.8.0
    github.com/rs/zerolog v1.31.0
    github.com/hashicorp/golang-lru/v2 v2.0.7
    github.com/stretchr/testify v1.8.4
)
```

---

## Próximos Pasos

1. **Iniciar Fase 0:** Setup del proyecto
2. **Fase 1:** Loader de paquetes
3. **Fase 2-3:** Walker + Pipeline
4. **Fase 4:** Implementar fases una por una, testeando contra HL7 validator
5. **Fase 7:** CLI cuando tengamos fases básicas

¿Aprobamos este plan y comenzamos?
