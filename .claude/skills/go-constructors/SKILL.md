---
name: go-constructors
description: Constructores, opciones funcionales y builders en Go. Usar al crear constructores para tipos o diseñar APIs configurables.
allowed-tools: Read, Grep, Glob
---

# Skill: go-constructors

Constructores, opciones funcionales y builders en Go.

## Cuándo usar este skill

- Al crear constructores para tipos
- Al diseñar APIs configurables
- Al implementar patrones de inicialización
- Cuando se necesita flexibilidad sin romper API

## Patrones de Constructores en GoFHIR

### 1. Constructor Simple (New*)

```go
// validator/registry.go
func NewRegistry(version FHIRVersion) *Registry {
    return &Registry{
        version:     version,
        definitions: make(map[string]*StructureDefinition),
    }
}

// fhirpath/cache.go
func NewExpressionCache(limit int) *ExpressionCache {
    return &ExpressionCache{
        cache: make(map[string]*Expression),
        limit: limit,
    }
}
```

### 2. Constructor MustNew* (Panic on Error)

Para casos donde el error indica bug del programador (expresiones hardcodeadas):

```go
// fhirpath/fhirpath.go
func MustCompile(expr string) *Expression {
    e, err := Compile(expr)
    if err != nil {
        panic(fmt.Sprintf("fhirpath: invalid expression %q: %v", expr, err))
    }
    return e
}

func MustEvaluate(resource []byte, expr string) types.Collection {
    result, err := Evaluate(resource, expr)
    if err != nil {
        panic(fmt.Sprintf("fhirpath: evaluation failed: %v", err))
    }
    return result
}

// fhirpath/types/decimal.go
func MustDecimal(s string) Decimal {
    d, err := NewDecimal(s)
    if err != nil {
        panic(fmt.Sprintf("invalid decimal: %s", s))
    }
    return d
}

// Uso: expresiones conocidas en tiempo de compilación
var patientNameExpr = fhirpath.MustCompile("Patient.name.family")
```

### 3. Constructor con Versión FHIR

```go
// validator/validator.go

// Constructor genérico con versión
func NewInitializedValidator(
    ctx context.Context,
    version FHIRVersion,
    opts ValidatorOptions,
) (*Validator, error) {
    registry := NewRegistry(version)
    if err := registry.LoadDefaults(ctx); err != nil {
        return nil, fmt.Errorf("loading defaults: %w", err)
    }
    return NewValidator(registry, opts), nil
}

// Constructores específicos por versión (más convenientes)
func NewInitializedValidatorR4(ctx context.Context, opts ValidatorOptions) (*Validator, error) {
    return NewInitializedValidator(ctx, FHIRVersionR4, opts)
}

func NewInitializedValidatorR4B(ctx context.Context, opts ValidatorOptions) (*Validator, error) {
    return NewInitializedValidator(ctx, FHIRVersionR4B, opts)
}

func NewInitializedValidatorR5(ctx context.Context, opts ValidatorOptions) (*Validator, error) {
    return NewInitializedValidator(ctx, FHIRVersionR5, opts)
}
```

### 4. Struct de Opciones

Para muchas opciones relacionadas, usar struct:

```go
// validator/validator.go
type ValidatorOptions struct {
    ValidateConstraints bool
    ValidateTerminology bool
    ValidateReferences  bool
    ValidateExtensions  bool
    SkipContained       bool
    MaxErrors           int
}

// Constructor con opciones
func NewValidator(registry StructureDefinitionProvider, opts ValidatorOptions) *Validator {
    return &Validator{
        registry: registry,
        options:  opts,
    }
}

// Uso
v, err := validator.NewInitializedValidatorR4(ctx, validator.ValidatorOptions{
    ValidateConstraints: true,
    ValidateTerminology: true,
    ValidateReferences:  true,
    MaxErrors:           100,
})
```

### 5. Opciones Funcionales (WithX)

Para APIs donde las opciones son incrementales:

```go
// fhirpath/options.go
type EvalOption func(*EvalOptions)

type EvalOptions struct {
    Timeout     time.Duration
    Variables   map[string]interface{}
    Resolver    ReferenceResolver
    Terminology TerminologyService
}

func WithTimeout(d time.Duration) EvalOption {
    return func(o *EvalOptions) {
        o.Timeout = d
    }
}

func WithVariable(name string, value interface{}) EvalOption {
    return func(o *EvalOptions) {
        if o.Variables == nil {
            o.Variables = make(map[string]interface{})
        }
        o.Variables[name] = value
    }
}

func WithResolver(r ReferenceResolver) EvalOption {
    return func(o *EvalOptions) {
        o.Resolver = r
    }
}

// Aplicar opciones
func (e *Expression) EvaluateWithOptions(resource []byte, opts ...EvalOption) (types.Collection, error) {
    options := &EvalOptions{
        Timeout: 30 * time.Second,  // Default
    }
    for _, opt := range opts {
        opt(options)
    }
    // usar options...
}

// Uso
result, err := expr.EvaluateWithOptions(patient,
    fhirpath.WithTimeout(5*time.Second),
    fhirpath.WithVariable("context", observation),
    fhirpath.WithResolver(myResolver),
)
```

### 6. Builder Pattern (Fluent API)

Para configuración post-construcción:

```go
// validator/validator.go
func (v *Validator) WithTerminologyService(ts TerminologyService) *Validator {
    v.termService = ts
    return v
}

func (v *Validator) WithReferenceResolver(r ReferenceResolver) *Validator {
    v.refResolver = r
    return v
}

func (v *Validator) WithProfile(profileURL string) *Validator {
    v.profiles = append(v.profiles, profileURL)
    return v
}

// Uso fluent
validator := NewValidator(registry, opts).
    WithTerminologyService(termService).
    WithReferenceResolver(resolver).
    WithProfile("http://example.org/StructureDefinition/MyProfile")
```

### 7. Composite/Factory Constructors

```go
// validator/terminology.go

// Servicio simple
func NewLocalTerminologyService() *LocalTerminologyService {
    return &LocalTerminologyService{
        valueSets: make(map[string]*ValueSet),
    }
}

// Servicio embebido por versión
func NewEmbeddedTerminologyServiceR4() *EmbeddedTerminologyService {
    return &EmbeddedTerminologyService{
        version: "R4",
        // carga valueSets embebidos
    }
}

// Servicio compuesto (combina múltiples)
func NewCompositeTerminologyService(services ...TerminologyService) *CompositeTerminologyService {
    return &CompositeTerminologyService{
        services: services,
    }
}

// Uso: componer servicios
termService := validator.NewCompositeTerminologyService(
    validator.NewEmbeddedTerminologyServiceR4(),  // Fallback embebido
    validator.NewLocalTerminologyService(),        // Custom local
    externalTermServer,                            // Servidor externo
)
```

## Constructores para Tipos FHIR (r4, r4b, r5)

```go
import "github.com/robertoaraneda/gofhir/r4"

// Usando opciones funcionales (generado)
patient := r4.NewPatient(
    r4.WithPatientId("123"),
    r4.WithPatientActive(true),
    r4.WithPatientName(r4.HumanName{
        Family: "Doe",
        Given:  []string{"John"},
    }),
)

// Usando builder (generado)
patient := r4.NewPatientBuilder().
    SetId("123").
    SetActive(true).
    AddName(r4.HumanName{Family: "Doe", Given: []string{"John"}}).
    Build()
```

## Cuándo Usar Cada Patrón

| Patrón | Usar cuando... |
|--------|----------------|
| `New*` simple | Tipo sin configuración o 1-2 params obligatorios |
| `MustNew*` | Expresiones/configs conocidas en compilación |
| Struct de opciones | Muchas opciones relacionadas (>3) |
| Opciones funcionales | API extensible, opciones incrementales |
| Builder | Configuración compleja post-construcción |
| Versioned (`NewXxxR4`) | Funcionalidad específica por versión FHIR |
| Composite | Combinar múltiples implementaciones |

## Checklist

```markdown
- [ ] ¿El constructor retorna error cuando puede fallar?
- [ ] ¿Hay MustNew* para casos donde error = bug?
- [ ] ¿Los defaults son razonables?
- [ ] ¿Las opciones son extensibles sin romper API?
- [ ] ¿Hay constructores convenience por versión FHIR?
```

## Referencias del Proyecto

```text
validator/validator.go       → NewValidator, NewInitializedValidator*
validator/registry.go        → NewRegistry
validator/terminology.go     → NewLocalTerminologyService, NewComposite*
fhirpath/fhirpath.go         → MustCompile, MustEvaluate, Compile
fhirpath/cache.go            → NewExpressionCache, MustGetCached
fhirpath/options.go          → WithTimeout, WithVariable, etc.
r4/options_*.go              → NewPatient(opts...), etc.
r4/builder_*.go              → NewPatientBuilder(), etc.
```
