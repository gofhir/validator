---
name: go-patterns
description: Patrones de diseño idiomáticos en Go. Usar al elegir patrones para resolver problemas o diseñar sistemas extensibles.
allowed-tools: Read, Grep, Glob
---

# Skill: go-patterns

Patrones de diseño en Go con ejemplos del proyecto GoFHIR.

## Patrones Implementados en GoFHIR

### 1. Pipeline Pattern (validator/pipeline.go)

```go
// Interface para cada fase
type PhaseValidator interface {
    Name() string
    Priority() int
    Validate(ctx context.Context, resource []byte, result *ValidationResult) error
}

// Pipeline ejecuta fases en orden
type pipeline struct {
    phases []PhaseValidator
}

func (p *pipeline) Execute(ctx context.Context, resource []byte) (*ValidationResult, error) {
    result := NewValidationResult()
    for _, phase := range p.phases {
        if err := phase.Validate(ctx, resource, result); err != nil {
            return result, err
        }
    }
    return result, nil
}

// Fases: Structure, Constraints, Terminology, Extensions, References
```

### 2. Registry Pattern (fhirpath/funcs/registry.go)

```go
type Registry struct {
    mu    sync.RWMutex
    funcs map[string]*FuncDef
}

func (r *Registry) Register(def *FuncDef) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.funcs[def.Name] = def
}

func (r *Registry) Get(name string) (*FuncDef, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.funcs[name]
}
```

### 3. Visitor Pattern (validator/treewalker.go)

```go
type NodeVisitor interface {
    Visit(path string, value interface{}) error
}

type TreeWalker struct {
    visitor NodeVisitor
}

func (tw *TreeWalker) Walk(resource map[string]interface{}) error {
    return tw.walkNode("", resource)
}
```

### 4. Composite Pattern (validator/terminology.go)

```go
type CompositeTerminologyService struct {
    services []TerminologyService
}

func (c *CompositeTerminologyService) ValidateCode(ctx context.Context, system, code, url string) (bool, error) {
    for _, svc := range c.services {
        if valid, err := svc.ValidateCode(ctx, system, code, url); err == nil && valid {
            return true, nil
        }
    }
    return false, nil
}
```

### 5. Object Pool (fhirpath/types/pool.go)

```go
var collectionPool = sync.Pool{
    New: func() interface{} { return make(Collection, 0, 8) },
}

func GetCollection() Collection {
    return collectionPool.Get().(Collection)[:0]
}

func PutCollection(c Collection) {
    collectionPool.Put(c[:0])
}
```

### 6. LRU Cache (fhirpath/cache.go)

```go
type ExpressionCache struct {
    mu    sync.RWMutex
    cache map[string]*Expression
    limit int
}
```

## Cuándo Usar

| Patrón | Usar cuando... |
|--------|----------------|
| Pipeline | Procesos multi-fase (validación) |
| Registry | Registro dinámico (funciones FHIRPath) |
| Visitor | Recorrer estructuras (JSON FHIR) |
| Composite | Combinar servicios (terminology) |
| Object Pool | Reducir GC (collections) |
| LRU Cache | Cachear resultados costosos |

## Referencias

```text
validator/pipeline.go, phase.go, phases.go
validator/treewalker.go
validator/terminology.go
fhirpath/funcs/registry.go
fhirpath/types/pool.go
fhirpath/cache.go
```
