---
name: go-caching
description: Patrones de caching y pooling en Go. Usar al implementar caches, reducir allocations, o cachear resultados costosos.
allowed-tools: Read, Grep, Glob
---

# Skill: go-caching

Patrones de caching y pooling en Go con ejemplos de GoFHIR.

## Caching en GoFHIR

### 1. Expression Cache (fhirpath/cache.go)

Cache LRU para expresiones FHIRPath compiladas:

```go
// fhirpath/cache.go

type ExpressionCache struct {
    mu    sync.RWMutex
    cache map[string]*cacheEntry
    order []string  // Para tracking LRU
    limit int
}

type cacheEntry struct {
    expr     *Expression
    lastUsed time.Time
}

func NewExpressionCache(limit int) *ExpressionCache {
    return &ExpressionCache{
        cache: make(map[string]*cacheEntry),
        limit: limit,
    }
}

// Thread-safe get con actualización de LRU
func (c *ExpressionCache) Get(exprStr string) (*Expression, bool) {
    c.mu.RLock()
    entry, ok := c.cache[exprStr]
    c.mu.RUnlock()

    if ok {
        // Actualizar timestamp
        c.mu.Lock()
        entry.lastUsed = time.Now()
        c.mu.Unlock()
        return entry.expr, true
    }
    return nil, false
}

// Thread-safe put con eviction
func (c *ExpressionCache) Put(exprStr string, expr *Expression) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Evict oldest si está lleno
    if len(c.cache) >= c.limit {
        c.evictOldest()
    }

    c.cache[exprStr] = &cacheEntry{
        expr:     expr,
        lastUsed: time.Now(),
    }
}

// GetOrCompile - Pattern común
func (c *ExpressionCache) GetOrCompile(exprStr string) (*Expression, error) {
    if expr, ok := c.Get(exprStr); ok {
        return expr, nil
    }

    expr, err := Compile(exprStr)
    if err != nil {
        return nil, err
    }

    c.Put(exprStr, expr)
    return expr, nil
}
```

**Uso:**
```go
// Cache global para expresiones frecuentes
var globalCache = fhirpath.NewExpressionCache(1000)

// Evaluar con cache
expr, err := globalCache.GetOrCompile("Patient.name.family")
if err != nil {
    return err
}
result, _ := expr.Evaluate(patient)
```

### 2. Integer Cache (fhirpath/types/pool.go)

Cache de valores pequeños para evitar allocations:

```go
// fhirpath/types/pool.go

// Cache para integers -128 a 127 (como Java)
var integerCache [256]Integer

func init() {
    for i := range integerCache {
        integerCache[i] = Integer{value: int64(i - 128)}
    }
}

func NewInteger(v int64) Integer {
    // Usar cache para valores pequeños
    if v >= -128 && v < 128 {
        return integerCache[v+128]
    }
    return Integer{value: v}
}
```

### 3. Collection Pool (fhirpath/types/pool.go)

sync.Pool para colecciones reutilizables:

```go
// fhirpath/types/pool.go

var collectionPool = sync.Pool{
    New: func() interface{} {
        return make(Collection, 0, 8)  // Capacidad inicial 8
    },
}

// Obtener del pool (slice vacío pero con capacidad)
func GetCollection() Collection {
    return collectionPool.Get().(Collection)[:0]
}

// Devolver al pool
func PutCollection(c Collection) {
    // No guardar colecciones muy grandes
    if cap(c) <= 1024 {
        collectionPool.Put(c[:0])
    }
}

// Crear con capacidad específica
func NewCollectionWithCap(capacity int) Collection {
    if capacity <= 8 {
        return GetCollection()
    }
    return make(Collection, 0, capacity)
}
```

**Uso en evaluación:**
```go
func (e *Evaluator) evaluateWhere(input Collection, criteria Expression) Collection {
    result := types.GetCollection()  // Del pool
    defer func() {
        // NO devolver result al pool - se retorna al caller
    }()

    for _, item := range input {
        matches, _ := criteria.Evaluate(item)
        if matches.IsTrue() {
            result = append(result, item)
        }
    }
    return result
}
```

### 4. StructureDefinition Cache (validator/registry.go)

Cache de definiciones cargadas:

```go
// validator/registry.go

type Registry struct {
    mu          sync.RWMutex
    version     FHIRVersion
    definitions map[string]*StructureDefinition
}

func (r *Registry) GetStructureDefinition(url string) (*StructureDefinition, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    sd, ok := r.definitions[url]
    return sd, ok
}

func (r *Registry) LoadStructureDefinition(sd *StructureDefinition) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.definitions[sd.URL] = sd
}

// GetOrLoad con lazy loading
func (r *Registry) GetOrLoad(ctx context.Context, url string) (*StructureDefinition, error) {
    // Fast path - ya cacheado
    r.mu.RLock()
    if sd, ok := r.definitions[url]; ok {
        r.mu.RUnlock()
        return sd, nil
    }
    r.mu.RUnlock()

    // Slow path - cargar
    sd, err := r.loadFromSource(ctx, url)
    if err != nil {
        return nil, err
    }

    r.mu.Lock()
    r.definitions[url] = sd
    r.mu.Unlock()

    return sd, nil
}
```

### 5. Regex Cache (fhirpath/funcs/regex.go)

Cache para expresiones regulares compiladas:

```go
// fhirpath/funcs/regex.go

type RegexCache struct {
    mu      sync.RWMutex
    cache   map[string]*regexp.Regexp
    limit   int
    maxLen  int           // Max length de pattern
    timeout time.Duration // Timeout de compilación
}

func NewRegexCache(limit, maxLen int, timeout time.Duration) *RegexCache {
    return &RegexCache{
        cache:   make(map[string]*regexp.Regexp),
        limit:   limit,
        maxLen:  maxLen,
        timeout: timeout,
    }
}

func (c *RegexCache) Get(pattern string) (*regexp.Regexp, error) {
    // Validar longitud (seguridad)
    if len(pattern) > c.maxLen {
        return nil, fmt.Errorf("pattern too long: %d > %d", len(pattern), c.maxLen)
    }

    c.mu.RLock()
    if re, ok := c.cache[pattern]; ok {
        c.mu.RUnlock()
        return re, nil
    }
    c.mu.RUnlock()

    // Compilar con timeout
    re, err := c.compileWithTimeout(pattern)
    if err != nil {
        return nil, err
    }

    c.mu.Lock()
    if len(c.cache) < c.limit {
        c.cache[pattern] = re
    }
    c.mu.Unlock()

    return re, nil
}
```

## Patrones de Caching

### Thread-Safety

```go
// Read-heavy: usar RWMutex
type Cache struct {
    mu    sync.RWMutex
    items map[string]interface{}
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.items[key]
    return v, ok
}

func (c *Cache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = value
}
```

### Eviction Strategy

```go
// LRU eviction
func (c *Cache) evictOldest() {
    var oldestKey string
    var oldestTime time.Time

    for key, entry := range c.cache {
        if oldestKey == "" || entry.lastUsed.Before(oldestTime) {
            oldestKey = key
            oldestTime = entry.lastUsed
        }
    }

    if oldestKey != "" {
        delete(c.cache, oldestKey)
    }
}
```

## Cuándo Usar Cada Patrón

| Patrón | Usar cuando... | Ejemplo GoFHIR |
|--------|----------------|----------------|
| LRU Cache | Resultados costosos de calcular | ExpressionCache |
| sync.Pool | Objetos frecuentes, corta vida | Collection pool |
| Value Cache | Valores inmutables pequeños | Integer cache |
| Registry Cache | Datos cargados una vez | StructureDefinitions |
| GetOrCreate | Lazy loading con cache | GetOrCompile |

## Checklist

```markdown
- [ ] ¿El cache es thread-safe?
- [ ] ¿Hay estrategia de eviction?
- [ ] ¿El límite de tamaño es configurable?
- [ ] ¿sync.Pool solo para objetos de corta vida?
- [ ] ¿Se limpian objetos antes de devolver al pool?
```

## Referencias

```text
fhirpath/cache.go            → ExpressionCache
fhirpath/types/pool.go       → Collection pool, Integer cache
fhirpath/funcs/regex.go      → RegexCache
validator/registry.go        → StructureDefinition cache
```
