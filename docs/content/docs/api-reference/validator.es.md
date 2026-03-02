---
title: "Validator"
linkTitle: "Validator"
description: "Constructor, metodos y opciones de configuracion del GoFHIR Validator."
weight: 1
---

El `Validator` es el punto de entrada principal para validar recursos FHIR. Se crea una sola vez con opciones de construccion y luego se reutiliza para multiples llamadas de validacion.

```go
import "github.com/gofhir/validator/pkg/validator"
```

## Constructor

```go
func New(opts ...Option) (*Validator, error)
```

Crea un nuevo `Validator`. Durante la construccion, el validador carga paquetes FHIR, construye el registro de StructureDefinitions, indexa recursos de terminologia y compila expresiones FHIRPath. Este es el paso costoso -- las llamadas subsecuentes a `Validate` son rapidas.

La version FHIR por defecto es `4.0.1` (R4). Use `WithVersion` para cambiarla.

```go
v, err := validator.New(
    validator.WithVersion("4.0.1"),
)
if err != nil {
    log.Fatal(err)
}
```

## Metodos

### Validate

```go
func (v *Validator) Validate(
    ctx context.Context,
    resource []byte,
    opts ...ValidateOption,
) (*issue.Result, error)
```

Valida un recurso FHIR proporcionado como bytes JSON sin procesar. Retorna un `Result` con todos los issues de validacion, o un error si el contexto es cancelado.

Cuando un recurso declara multiples perfiles en `meta.profile`, se valida contra **todos** ellos, como lo requiere la especificacion FHIR.

```go
result, err := v.Validate(ctx, patientJSON)
if err != nil {
    log.Fatal(err)
}
if result.HasErrors() {
    fmt.Printf("Se encontraron %d errores\n", result.ErrorCount())
}
```

### ValidateJSON

```go
func (v *Validator) ValidateJSON(
    ctx context.Context,
    jsonStr string,
    opts ...ValidateOption,
) (*issue.Result, error)
```

Wrapper de conveniencia que acepta un string JSON en lugar de `[]byte`. Internamente llama a `Validate`.

```go
result, err := v.ValidateJSON(ctx, `{"resourceType":"Patient"}`)
```

### Registry

```go
func (v *Validator) Registry() *registry.Registry
```

Retorna el registro de StructureDefinitions subyacente. Util para casos de uso avanzados como inspeccionar perfiles cargados o consultar definiciones de elementos directamente.

### Config

```go
func (v *Validator) Config() *Config
```

Retorna la configuracion aplicada durante la construccion.

### Version

```go
func (v *Validator) Version() string
```

Retorna el string de version FHIR (por ejemplo, `"4.0.1"`).

---

## Opciones de Construccion

Las opciones de construccion se pasan a `New` y configuran el validador durante toda su vida util. Siguen el patron de opciones funcionales: cada opcion es una funcion de tipo `Option`.

```go
type Option func(*Config)
```

### WithVersion

```go
func WithVersion(version string) Option
```

Establece la version FHIR a cargar. Los valores soportados incluyen `"4.0.1"` (R4), `"4.3.0"` (R4B) y `"5.0.0"` (R5). Por defecto es `"4.0.1"`.

```go
v, _ := validator.New(
    validator.WithVersion("4.0.1"),
)
```

### WithProfile

```go
func WithProfile(profileURL string) Option
```

Agrega una URL de perfil contra la cual se validara cada recurso. Se puede llamar multiples veces para agregar varios perfiles.

```go
v, _ := validator.New(
    validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
```

### WithStrictMode

```go
func WithStrictMode(strict bool) Option
```

Cuando se habilita, las advertencias se promueven a errores. Util en pipelines de CI donde se requiere tolerancia cero para cualquier issue.

```go
v, _ := validator.New(
    validator.WithStrictMode(true),
)
```

### WithPackagePath

```go
func WithPackagePath(path string) Option
```

Establece una ruta personalizada para el directorio de cache de paquetes FHIR. Por defecto el validador usa la ubicacion estandar del cache de paquetes NPM FHIR (`~/.fhir/packages`).

```go
v, _ := validator.New(
    validator.WithPackagePath("/opt/fhir/packages"),
)
```

### WithPackage

```go
func WithPackage(name, version string) Option
```

Carga un paquete FHIR adicional desde el cache de paquetes NPM. El paquete debe estar instalado previamente en el directorio de cache. Se puede llamar multiples veces.

```go
v, _ := validator.New(
    validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    validator.WithPackage("hl7.fhir.uv.ips", "1.1.0"),
)
```

### WithPackageTgz

```go
func WithPackageTgz(path string) Option
```

Carga un paquete FHIR desde un archivo `.tgz` local. Util cuando el paquete no esta en el cache NPM. Se puede llamar multiples veces.

```go
v, _ := validator.New(
    validator.WithPackageTgz("/path/to/custom-ig.tgz"),
)
```

### WithPackageURL

```go
func WithPackageURL(url string) Option
```

Carga un paquete FHIR desde una URL remota de un archivo `.tgz`. El archivo se descarga durante la construccion. Se puede llamar multiples veces.

```go
v, _ := validator.New(
    validator.WithPackageURL("https://packages.fhir.org/hl7.fhir.us.core/6.1.0"),
)
```

### WithPackageData

```go
func WithPackageData(data []byte) Option
```

Carga un paquete FHIR desde bytes `.tgz` en memoria. Ideal para paquetes embebidos en el binario via `//go:embed`. Se puede llamar multiples veces.

```go
//go:embed packages/custom-ig.tgz
var customIG []byte

v, _ := validator.New(
    validator.WithPackageData(customIG),
)
```

### WithConformanceResources

```go
func WithConformanceResources(resources [][]byte) Option
```

Carga recursos de conformance individuales (StructureDefinition, ValueSet, CodeSystem, etc.) directamente en el registro. Cada entrada debe ser JSON valido. Util cuando los recursos de conformance se almacenan en una base de datos en lugar de paquetes.

```go
resources := [][]byte{structureDefJSON, valueSetJSON}
v, _ := validator.New(
    validator.WithConformanceResources(resources),
)
```

### WithTerminologyProvider

```go
func WithTerminologyProvider(provider terminology.Provider) Option
```

Establece un proveedor de terminologia externo para validar codigos en sistemas que no pueden expandirse localmente (por ejemplo, SNOMED CT, LOINC). Consulte la pagina [Proveedor de Terminologia](terminology-provider) para mas detalles.

```go
v, _ := validator.New(
    validator.WithTerminologyProvider(myTermProvider),
)
```

### WithProfileResolver

```go
func WithProfileResolver(resolver registry.ProfileResolver) Option
```

Establece un resolvedor de perfiles externo para carga de StructureDefinitions bajo demanda. Cuando se configura, el registro recurre a este resolvedor para perfiles no encontrados en memoria. Consulte la pagina [Resolvedor de Perfiles](profile-resolver) para mas detalles.

```go
v, _ := validator.New(
    validator.WithProfileResolver(myResolver),
)
```

### WithNoTerminology

```go
func WithNoTerminology() Option
```

Deshabilita toda la validacion de terminologia y bindings. Equivalente al flag `-tx n/a` del HL7 Validator.

```go
v, _ := validator.New(
    validator.WithNoTerminology(),
)
```

---

## Opciones por Llamada

Las opciones por llamada se pasan a `Validate` o `ValidateJSON` y afectan solo esa validacion individual. No modifican la configuracion de construccion del validador.

```go
type ValidateOption func(*validateConfig)
```

### ValidateWithProfile

```go
func ValidateWithProfile(profileURL string) ValidateOption
```

Agrega una URL de perfil para validar solo en esta llamada. Se puede pasar multiples veces.

```go
result, _ := v.Validate(ctx, resource,
    validator.ValidateWithProfile("http://example.org/StructureDefinition/my-patient"),
)
```

### ValidateWithCanonicalProfile

```go
func ValidateWithCanonicalProfile(canonical string) ValidateOption
```

Agrega una referencia canonica (`url|version`) como perfil para validar solo en esta llamada. La version se usa para resolucion con reconocimiento de version. Si no hay separador `|`, se comporta como `ValidateWithProfile`.

```go
result, _ := v.Validate(ctx, resource,
    validator.ValidateWithCanonicalProfile("http://example.org/StructureDefinition/my-patient|1.0.0"),
)
```

---

## Config

La estructura `Config` contiene todos los valores de configuracion establecidos via opciones de construccion. Es accesible via `v.Config()` despues de la creacion.

```go
type Config struct {
    FHIRVersion          string                   // e.g., "4.0.1", "4.3.0", "5.0.0"
    Profiles             []string                 // Profiles to validate against
    StrictMode           bool                     // Treat warnings as errors
    PackagePath          string                   // Path to FHIR package cache
    AdditionalPackages   []PackageSpec            // Additional packages to load
    PackageTgzPaths      []string                 // Paths to local .tgz files
    PackageURLs          []string                 // URLs to remote .tgz files
    PackageData          [][]byte                 // In-memory .tgz bytes
    ConformanceResources [][]byte                 // Individual conformance resource JSON
    TerminologyProvider  terminology.Provider     // External terminology provider
    ProfileResolver      registry.ProfileResolver // External profile resolver
    NoTerminology        bool                     // Skip terminology validation
}
```

---

## Ejemplo Completo

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/gofhir/validator/pkg/validator"
)

func main() {
    // Crear un validador con varias opciones
    v, err := validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
        validator.WithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
        validator.WithStrictMode(true),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Leer un recurso FHIR
    resource, err := os.ReadFile("patient.json")
    if err != nil {
        log.Fatal(err)
    }

    // Validar con un perfil adicional por llamada
    ctx := context.Background()
    result, err := v.Validate(ctx, resource,
        validator.ValidateWithProfile("http://example.org/StructureDefinition/custom-patient"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Procesar resultados
    fmt.Printf("Errores:      %d\n", result.ErrorCount())
    fmt.Printf("Advertencias: %d\n", result.WarningCount())
    fmt.Printf("Informacion:  %d\n", result.InfoCount())
    fmt.Printf("Duracion:     %.2f ms\n", result.Stats.DurationMs())

    for _, iss := range result.Issues {
        fmt.Printf("[%s] %s: %s\n", iss.Severity, iss.Code, iss.Diagnostics)
    }
}
```
