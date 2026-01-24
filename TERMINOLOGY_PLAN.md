# Plan: Auto-Loading Specs + Terminology Service

## Objetivo

Implementar carga automática de specs según versión FHIR y servicio de terminología integrado.

## API Final

```go
// Caso 1: Validación básica (solo estructura)
validator, err := engine.New(ctx, fv.R4)

// Caso 2: Con terminología local
validator, err := engine.New(ctx, fv.R4,
    fv.WithTerminology(true),
)

// Caso 3: Con servidor externo para LOINC/SNOMED
validator, err := engine.New(ctx, fv.R4,
    fv.WithTerminology(true),
    fv.WithExternalTerminologyServer("https://tx.fhir.org/r4",
        "http://loinc.org",
        "http://snomed.info/sct",
    ),
)

// Caso 4: Agregar IGs adicionales
validator, err := engine.New(ctx, fv.R4)
validator.LoadIG("./packages/clcore")
```

---

## Estructura de Archivos

```
fhirvalidator/
├── specs/
│   ├── embed.go                 # go:embed directives
│   ├── r4/                      # (existing)
│   ├── r4b/                     # (existing)
│   └── r5/                      # (existing)
│
├── context/
│   ├── doc.go                   # Package documentation
│   ├── context.go               # SpecContext
│   ├── loader.go                # Auto-loader from embedded FS
│   └── options.go               # Context options
│
├── terminology/
│   ├── doc.go                   # Package documentation
│   ├── service.go               # TerminologyService (public API)
│   ├── options.go               # Service options
│   ├── cache.go                 # Sharded concurrent cache
│   ├── pool.go                  # Object pools (sync.Pool)
│   │
│   └── internal/
│       ├── local.go             # Loads from embedded specs
│       ├── remote.go            # Calls external TX server
│       ├── router.go            # Routes system → local or remote
│       ├── parser.go            # Streaming JSON parser
│       └── fnv.go               # Fast hash for cache sharding
│
├── engine/
│   └── validator.go             # MODIFY: Use SpecContext
│
└── options.go                   # MODIFY: Add terminology options
```

---

## Fase 1: Embedded Specs

### 1.1 Mover specs a fhirvalidator

```bash
# Copiar specs al paquete fhirvalidator
cp -r specs/r4 fhirvalidator/specs/
cp -r specs/r4b fhirvalidator/specs/
cp -r specs/r5 fhirvalidator/specs/
```

### 1.2 Crear embed.go

```go
// fhirvalidator/specs/embed.go
package specs

import "embed"

//go:embed r4/*.json
var R4Specs embed.FS

//go:embed r4b/*.json
var R4BSpecs embed.FS

//go:embed r5/*.json
var R5Specs embed.FS

func GetSpecsFS(version string) (embed.FS, string, error) {
    switch version {
    case "R4", "4.0.1":
        return R4Specs, "r4", nil
    case "R4B", "4.3.0":
        return R4BSpecs, "r4b", nil
    case "R5", "5.0.0":
        return R5Specs, "r5", nil
    default:
        return embed.FS{}, "", fmt.Errorf("unsupported version: %s", version)
    }
}
```

---

## Fase 2: SpecContext

### 2.1 context/context.go

```go
package context

type SpecContext struct {
    Version     fv.FHIRVersion
    Profiles    service.ProfileResolver
    Terminology terminology.Service
    loaded      bool
    mu          sync.RWMutex
}

func New(ctx context.Context, version fv.FHIRVersion, opts Options) (*SpecContext, error) {
    sc := &SpecContext{Version: version}

    // Get embedded specs
    specsFS, dir, err := specs.GetSpecsFS(string(version))
    if err != nil {
        return nil, err
    }

    // Load profiles (always)
    sc.Profiles, err = loadProfiles(specsFS, dir)
    if err != nil {
        return nil, fmt.Errorf("failed to load profiles: %w", err)
    }

    // Load terminology (if enabled)
    if opts.LoadTerminology {
        sc.Terminology, err = loadTerminology(specsFS, dir, opts.TerminologyOptions)
        if err != nil {
            return nil, fmt.Errorf("failed to load terminology: %w", err)
        }
    }

    sc.loaded = true
    return sc, nil
}
```

---

## Fase 3: TerminologyService

### 3.1 terminology/service.go

```go
package terminology

type Service struct {
    version fv.FHIRVersion
    opts    Options
    local   *internal.LocalSource
    remote  *internal.RemoteSource
    cache   *cache
    router  *internal.Router
}

func New(version fv.FHIRVersion, opts Options) (*Service, error) { ... }
func NewFromFS(fs embed.FS, dir string, opts Options) (*Service, error) { ... }

func (s *Service) ValidateCode(ctx context.Context, system, code, valueSet string) (*ValidateResult, error) { ... }
func (s *Service) ValidateCoding(ctx context.Context, coding *Coding, valueSet string) (*ValidateResult, error) { ... }
func (s *Service) LookupCode(ctx context.Context, system, code string) (*CodeInfo, error) { ... }
func (s *Service) ExpandValueSet(ctx context.Context, url string) (*ValueSetExpansion, error) { ... }
func (s *Service) Close() error { ... }
```

### 3.2 terminology/internal/local.go

```go
package internal

type LocalSource struct {
    codeSystems     map[string]*CodeSystem  // url -> CodeSystem
    codeSystemsOnce sync.Once
    valueSets       map[string]*ValueSet    // url -> ValueSet
    valueSetsOnce   sync.Once
    fs              embed.FS
    dir             string
}

func (l *LocalSource) ValidateCode(ctx context.Context, system, code, valueSet string) (*ValidateResult, error) {
    cs, err := l.getCodeSystem(system)
    if err != nil {
        return nil, err
    }

    // Check if code exists
    concept := cs.FindCode(code)
    if concept == nil {
        return &ValidateResult{
            Valid:   false,
            Message: fmt.Sprintf("Code '%s' not found in CodeSystem '%s'", code, system),
        }, nil
    }

    return &ValidateResult{
        Valid:   true,
        Display: concept.Display,
        Code:    code,
        System:  system,
    }, nil
}
```

### 3.3 terminology/internal/remote.go

```go
package internal

type RemoteSource struct {
    client  *http.Client
    baseURL string
}

func (r *RemoteSource) ValidateCode(ctx context.Context, system, code, valueSet string) (*ValidateResult, error) {
    // Build $validate-code request
    url := fmt.Sprintf("%s/CodeSystem/$validate-code?system=%s&code=%s",
        r.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
    )

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := r.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Parse Parameters response
    return parseValidateCodeResponse(resp.Body)
}
```

### 3.4 terminology/cache.go (Sharded)

```go
package terminology

const numShards = 256

type cache struct {
    shards    [numShards]*cacheShard
    shardMask uint64
    ttl       time.Duration
}

type cacheShard struct {
    mu    sync.RWMutex
    items map[string]*cacheEntry
}

func (c *cache) get(system, code string) *ValidateResult {
    key := system + "|" + code
    shard := c.shards[fnv1a(key)&c.shardMask]

    shard.mu.RLock()
    entry, ok := shard.items[key]
    shard.mu.RUnlock()

    if !ok || time.Now().After(entry.expiresAt) {
        return nil
    }
    return entry.result
}
```

---

## Fase 4: Integración con Engine

### 4.1 Modificar engine/validator.go

```go
func New(ctx context.Context, version fv.FHIRVersion, opts ...fv.Option) (*Validator, error) {
    options := fv.DefaultOptions()
    for _, opt := range opts {
        opt(options)
    }

    // Create SpecContext (auto-loads specs)
    specCtx, err := context.New(ctx, version, context.Options{
        LoadTerminology:    options.ValidateTerminology,
        TerminologyOptions: options.TerminologyOptions,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to load specs for %s: %w", version, err)
    }

    v := &Validator{
        version:            version,
        options:            options,
        specContext:        specCtx,
        profileService:     specCtx.Profiles,
        terminologyService: specCtx.Terminology,
        fhirPathEvaluator:  service.NewFHIRPathAdapter(),
        metrics:            fv.NewMetrics(),
    }

    if err := v.buildPipeline(); err != nil {
        return nil, err
    }

    return v, nil
}
```

### 4.2 Nuevas opciones

```go
// fhirvalidator/options.go

func WithTerminology(enabled bool) Option {
    return func(o *Options) {
        o.ValidateTerminology = enabled
    }
}

func WithExternalTerminologyServer(serverURL string, systems ...string) Option {
    return func(o *Options) {
        o.ValidateTerminology = true
        o.TerminologyOptions.ExternalServer = serverURL
        o.TerminologyOptions.ExternalSystems = systems
    }
}

func WithTerminologyCache(path string, ttl time.Duration) Option {
    return func(o *Options) {
        o.TerminologyOptions.CachePath = path
        o.TerminologyOptions.CacheTTL = ttl
    }
}
```

---

## Fase 5: Tests y Benchmarks

```go
// terminology/service_test.go
func TestValidateCode_LocalSystem(t *testing.T) { ... }
func TestValidateCode_ExternalSystem(t *testing.T) { ... }
func TestValidateCode_Cached(t *testing.T) { ... }
func TestValidateCode_DisplayMismatch(t *testing.T) { ... }

// terminology/benchmark_test.go
func BenchmarkValidateCode_Cached(b *testing.B) { ... }
func BenchmarkValidateCode_Local(b *testing.B) { ... }
func BenchmarkCache_Concurrent(b *testing.B) { ... }
```

---

## Orden de Implementación

| # | Tarea | Archivos | Prioridad |
|---|-------|----------|-----------|
| 1 | Copiar specs a fhirvalidator | specs/* | Alta |
| 2 | Crear specs/embed.go | specs/embed.go | Alta |
| 3 | Crear context package | context/*.go | Alta |
| 4 | Crear terminology/internal/local.go | terminology/internal/local.go | Alta |
| 5 | Crear terminology/cache.go | terminology/cache.go | Alta |
| 6 | Crear terminology/service.go | terminology/service.go | Alta |
| 7 | Modificar engine/validator.go | engine/validator.go | Alta |
| 8 | Agregar nuevas options | options.go | Media |
| 9 | Crear terminology/internal/remote.go | terminology/internal/remote.go | Media |
| 10 | Tests y benchmarks | *_test.go | Media |
| 11 | Actualizar ejemplos | examples/*.go | Baja |

---

## Go Best Practices Incluidas

- [x] `sync.Pool` para reducir allocations
- [x] `sync.Once` para lazy loading thread-safe
- [x] Sharded cache para reducir lock contention
- [x] Worker pool para bounded parallelism
- [x] `context.Context` para cancellation
- [x] `rate.Limiter` para external calls
- [x] Streaming JSON parser para archivos grandes
- [x] HTTP connection pooling
- [x] `embed.FS` para specs embebidos

---

## Resultado Esperado

```
Validando Bundle...

Entry 0: ServiceRequest
  locationCode[0].coding[0]:
    system: http://terminology.hl7.org/CodeSystem/v3-RoleCode
    code: HOSPITES
    → [warning] Code 'HOSPITES' not found in CodeSystem 'v3-RoleCode'
    → [info] Did you mean 'HOSP' (Hospital)?

  code.coding[0]:
    system: http://loinc.org
    code: 12345-6
    → [valid] "Blood pressure" (external: tx.fhir.org)
```
