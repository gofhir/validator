---
name: go-interfaces
description: Diseño de interfaces y contratos en Go. Usar al definir APIs extensibles, preparar código para testing, o cuando se necesita bajo acoplamiento.
allowed-tools: Read, Grep, Glob
---

# Skill: go-interfaces

Diseño de interfaces, contratos y abstracción en Go.

## Cuándo usar este skill

- Al definir contratos entre componentes
- Al diseñar APIs extensibles
- Al preparar código para testing
- Cuando se necesita bajo acoplamiento

## Principios de Diseño

### 1. Interfaces Pequeñas (1-3 métodos)

```go
// ✅ BIEN: Interface pequeña y específica
type Reader interface {
    Read(p []byte) (n int, err error)
}

// ❌ MAL: Interface gigante
type DoEverything interface {
    Read() / Write() / Close() / ...  // 20 métodos
}
```

### 2. Definir Donde se USA (No donde se implementa)

```go
// ✅ BIEN: El consumidor define lo que necesita
package validator

type TerminologyService interface {
    ValidateCode(ctx context.Context, system, code string) (bool, error)
}

type Validator struct {
    terminology TerminologyService  // Acepta cualquier implementación
}
```

## Interfaces del Proyecto GoFHIR

### Validator - Interfaces Principales

```go
// validator/interfaces.go

// TerminologyService - Validación de códigos contra ValueSets
type TerminologyService interface {
    ValidateCode(ctx context.Context, system, code, valueSetURL string) (bool, error)
}

// ReferenceResolver - Resolución de referencias FHIR
type ReferenceResolver interface {
    Resolve(ctx context.Context, reference string) (interface{}, error)
}

// StructureDefinitionProvider - Obtención de StructureDefinitions
type StructureDefinitionProvider interface {
    GetStructureDefinition(ctx context.Context, url string) (*StructureDefinition, error)
}
```

### Validator - Pipeline Pattern

```go
// validator/phase.go

// PhaseValidator - Cada fase de validación
type PhaseValidator interface {
    Name() string
    Priority() int
    Validate(ctx context.Context, resource []byte, result *ValidationResult) error
}

// PhaseRegistry - Registro de fases
type PhaseRegistry interface {
    Register(phase PhaseValidator)
    GetPhases() []PhaseValidator
}

// validator/pipeline.go
type ValidationPipeline interface {
    Execute(ctx context.Context, resource []byte) (*ValidationResult, error)
}
```

### Validator - Tree Walking

```go
// validator/treewalker.go

// NodeVisitor - Patrón Visitor para recorrer recursos FHIR
type NodeVisitor interface {
    Visit(path string, value interface{}) error
}
```

### FHIRPath - Interfaces de Evaluación

```go
// fhirpath/eval/evaluator.go

// FuncRegistry - Registro de funciones FHIRPath
type FuncRegistry interface {
    Get(name string) (FuncImpl, bool)
}

// Resolver - Resolución de referencias en expresiones
type Resolver interface {
    Resolve(ctx context.Context, reference string) (interface{}, error)
}

// TerminologyService - Para memberOf() y otras funciones
type TerminologyService interface {
    ValidateCode(ctx context.Context, system, code, valueSetURL string) (bool, error)
    ExpandValueSet(ctx context.Context, valueSetURL string) ([]Code, error)
}

// ProfileValidator - Para conformsTo()
type ProfileValidator interface {
    Validate(ctx context.Context, resource interface{}, profileURL string) (bool, error)
}
```

### FHIRPath - Tipos de Valor

```go
// fhirpath/types/value.go

// Value - Interface base para todos los valores FHIRPath
type Value interface {
    TypeName() string
    Equal(other Value) bool
}

// Comparable - Valores que pueden compararse
type Comparable interface {
    Value
    Compare(other Comparable) (int, error)
}

// Numeric - Valores numéricos
type Numeric interface {
    Comparable
    AsDecimal() decimal.Decimal
}
```

### FHIRPath - Resource Interface

```go
// fhirpath/resource.go

// Resource - Interface para recursos FHIR tipados
type Resource interface {
    ResourceType() string
    GetID() string
}
```

## Implementaciones No-Op (Para Opcionalidad)

```go
// Cuando un servicio es opcional, proveer implementación no-op

// NoopTerminologyService - Siempre retorna válido
type NoopTerminologyService struct{}

func (n *NoopTerminologyService) ValidateCode(
    ctx context.Context,
    system, code, valueSetURL string,
) (bool, error) {
    return true, nil  // Siempre válido, no hace validación real
}

// Uso en validator
func NewValidator(opts ...Option) *Validator {
    v := &Validator{
        terminology: &NoopTerminologyService{},  // Default no-op
    }
    for _, opt := range opts {
        opt(v)
    }
    return v
}

// Usuario puede reemplazar con implementación real
v := NewValidator(
    WithTerminologyService(myRealTermService),
)
```

## Verificación en Compilación

```go
// Asegurar que un tipo implementa una interface
var _ TerminologyService = (*LocalTerminologyService)(nil)
var _ TerminologyService = (*FHIRTerminologyServer)(nil)
var _ PhaseValidator = (*StructurePhase)(nil)
var _ PhaseValidator = (*ConstraintPhase)(nil)
```

## Patrones de Composición con Interfaces

```go
// Combinar múltiples servicios de terminología
type CompositeTerminologyService struct {
    services []TerminologyService
}

func (c *CompositeTerminologyService) ValidateCode(
    ctx context.Context,
    system, code, valueSetURL string,
) (bool, error) {
    for _, svc := range c.services {
        valid, err := svc.ValidateCode(ctx, system, code, valueSetURL)
        if err == nil && valid {
            return true, nil
        }
    }
    return false, nil
}
```

## Uso con Tipos FHIR (r4, r4b, r5)

```go
import "github.com/robertoaraneda/gofhir/r4"

// Interface para trabajar con recursos tipados
type PatientService interface {
    GetPatient(ctx context.Context, id string) (*r4.Patient, error)
    SavePatient(ctx context.Context, patient *r4.Patient) error
}

// Implementación
type FHIRPatientService struct {
    client *http.Client
    baseURL string
}

func (s *FHIRPatientService) GetPatient(ctx context.Context, id string) (*r4.Patient, error) {
    // Fetch from FHIR server
    resp, err := s.client.Get(s.baseURL + "/Patient/" + id)
    // ...
}
```

## Checklist

```markdown
- [ ] ¿La interface tiene 1-3 métodos?
- [ ] ¿Está definida donde se usa (no donde se implementa)?
- [ ] ¿Incluye context.Context en métodos que pueden bloquear?
- [ ] ¿Es fácil de mockear para testing?
- [ ] ¿Hay implementación no-op para cuando es opcional?
- [ ] ¿Hay verificación de compilación (var _ Interface = (*Type)(nil))?
```

## Referencias del Proyecto

```
validator/interfaces.go      → Interfaces principales del validator
validator/phase.go           → PhaseValidator, PhaseRegistry
validator/pipeline.go        → ValidationPipeline
validator/treewalker.go      → NodeVisitor
fhirpath/eval/evaluator.go   → FuncRegistry, Resolver, TerminologyService
fhirpath/types/value.go      → Value, Comparable, Numeric
fhirpath/options.go          → ReferenceResolver
```
