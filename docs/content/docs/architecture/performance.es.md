---
title: "Rendimiento"
linkTitle: "Rendimiento"
description: "Estrategias de optimizacion, resultados de benchmarks y mejores practicas para validacion FHIR de alto rendimiento."
weight: 4
---

El rendimiento fue una preocupacion transversal abordada en el Milestone 15, aplicando optimizaciones en todos los paquetes. El GoFHIR Validator usa varias estrategias para minimizar allocations, reducir computacion y maximizar el throughput sin sacrificar la correctitud.

## Estrategias de Optimizacion

### Object Pooling con sync.Pool

El validador usa `sync.Pool` para reutilizar objetos frecuentemente creados, particularmente los structs `Result` y `Stats` que se crean en cada llamada de validacion. Esto elimina miles de allocations en el heap en escenarios de procesamiento por lotes.

```go
var resultPool = sync.Pool{
    New: func() interface{} {
        return &Result{
            Issues: make([]issue.Issue, 0, 16),
        }
    },
}

func acquireResult() *Result {
    r := resultPool.Get().(*Result)
    r.Issues = r.Issues[:0] // Resetear slice, mantener backing array
    return r
}

func releaseResult(r *Result) {
    resultPool.Put(r)
}
```

### Cache de Compilacion de Regex

Los tipos primitivos FHIR se validan contra expresiones regulares derivadas de sus StructureDefinitions. Como los mismos patrones se usan en miles de elementos, los regexes compilados se cachean para evitar llamadas repetidas a `regexp.Compile`.

```go
var regexCache sync.Map

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
    if cached, ok := regexCache.Load(pattern); ok {
        return cached.(*regexp.Regexp), nil
    }
    compiled, err := regexp.Compile("^" + pattern + "$")
    if err != nil {
        return nil, err
    }
    regexCache.Store(pattern, compiled)
    return compiled, nil
}
```

### Cache de Indices de Elementos

Cada StructureDefinition contiene un snapshot con potencialmente cientos de entradas `ElementDefinition`. El validador construye un indice path-a-elemento en el primer acceso y lo cachea para busquedas posteriores, convirtiendo escaneos lineales O(n) en busquedas O(1) en un mapa.

```go
type elementIndex struct {
    byPath map[string]*ElementDefinition
    once   sync.Once
}

func (idx *elementIndex) Get(path string) *ElementDefinition {
    idx.once.Do(func() {
        idx.byPath = make(map[string]*ElementDefinition, len(idx.elements))
        for i := range idx.elements {
            idx.byPath[idx.elements[i].Path] = &idx.elements[i]
        }
    })
    return idx.byPath[path]
}
```

### Especificaciones Embebidas

Las definiciones base de FHIR R4, R4B y R5 estan embebidas directamente en el binario usando el paquete `embed` de Go via `pkg/specs`. Esto elimina I/O de disco al inicio y hace que el binario del validador sea completamente autocontenido.

```go
//go:embed data/r4/*.json
var r4Specs embed.FS
```

Esto significa que `validator.New()` puede cargar todos los StructureDefinitions base, ValueSets y CodeSystems desde memoria sin tocar el sistema de archivos.

## Resultados de Benchmarks

Todos los benchmarks se ejecutaron con `go test -bench=. -benchmem ./pkg/validator/` en una maquina de desarrollo estandar.

| Escenario | Mejora | Allocations |
|-----------|--------|-------------|
| Patient Minimo | 4.0x mas rapido | 86% menos |
| Patient con Datos | 1.3x mas rapido | 43% menos |
| Validacion Paralela | 8.5x mas rapido | 74% menos |
| Procesamiento por Lotes | 2.1x mas rapido | 72% menos |

La columna "Mejora" compara el validador optimizado (M15) contra la linea base pre-optimizacion. La columna "Allocations" muestra la reduccion en allocations del heap por operacion.

## Ejecutar Benchmarks

Para ejecutar la suite completa de benchmarks:

```bash
go test -bench=. -benchmem ./pkg/validator/
```

Para ejecutar un benchmark especifico:

```bash
go test -bench=BenchmarkValidatePatient -benchmem ./pkg/validator/
```

Para comparar resultados de benchmarks entre ejecuciones, usa `benchstat`:

```bash
# Ejecutar benchmarks antes de los cambios
go test -bench=. -benchmem -count=10 ./pkg/validator/ > old.txt

# Hacer cambios, luego ejecutar de nuevo
go test -bench=. -benchmem -count=10 ./pkg/validator/ > new.txt

# Comparar
benchstat old.txt new.txt
```

## Profiling de Memoria

Para identificar hotspots de allocations:

```bash
# Generar un perfil de memoria
go test -bench=BenchmarkValidatePatient -memprofile mem.prof ./pkg/validator/

# Analizar con pprof
go tool pprof -http=:8080 mem.prof
```

Para generar un perfil de CPU:

```bash
# Generar un perfil de CPU
go test -bench=BenchmarkValidatePatient -cpuprofile cpu.prof ./pkg/validator/

# Analizar
go tool pprof -http=:8080 cpu.prof
```

## Mejores Practicas

### Reutilizar la Instancia del Validador

Crear un validador carga e indexa StructureDefinitions. Crealo una vez y reutilizalo en todas las validaciones:

```go
// CORRECTO: Crear una vez, reutilizar muchas veces
v, err := validator.New()
if err != nil {
    log.Fatal(err)
}

for _, resource := range resources {
    result := v.Validate(resource)
    // procesar resultado
}
```

```go
// INCORRECTO: Crear un nuevo validador para cada recurso
for _, resource := range resources {
    v, _ := validator.New() // Costoso! Recarga todos los SDs
    result := v.Validate(resource)
}
```

### Usar Validacion por Lotes

Al validar multiples recursos, pasalos como lote en lugar de validar uno a la vez. Esto habilita optimizaciones internas como reutilizacion de pools y menor presion en el GC:

```go
results := v.ValidateBatch(resources)
```

### Deshabilitar Fases No Utilizadas

Si sabes que ciertas fases de validacion no son necesarias para tu caso de uso, deshabilitalas para ahorrar tiempo de procesamiento:

```go
// Omitir evaluacion de constraints FHIRPath y slicing si no se necesitan
v, err := validator.New(
    validator.WithDisabledPhases("constraint", "slicing"),
)
```

### Deshabilitar Terminologia Sin Conexion

Si no hay un servidor de terminologia disponible, deshabilita la validacion de terminologia explicitamente para evitar retrasos por timeout:

```go
v, err := validator.New(
    validator.WithTerminologyDisabled(),
)
```

O via la CLI:

```bash
gofhir-validator -tx n/a patient.json
```

{{< callout type="info" >}}
Las estrategias de optimizacion descritas aqui se aplican automaticamente. No necesitas configurar pooling, cache ni indexacion -- estan integradas en el validador. Las mejores practicas anteriores se enfocan en como usar el validador para aprovechar al maximo estas optimizaciones integradas.
{{< /callout >}}
