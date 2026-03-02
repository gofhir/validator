---
title: "Resultado e Issues"
linkTitle: "Resultado e Issues"
description: "Resultados de validacion, tipos de issues, niveles de severidad y constantes de codigo."
weight: 2
---

Despues de llamar a `Validate` o `ValidateJSON`, se recibe un `*issue.Result` que contiene todos los issues de validacion y estadisticas.

```go
import "github.com/gofhir/validator/pkg/issue"
```

## Result

```go
type Result struct {
    Issues []Issue
    Stats  *Stats
}
```

El tipo `Result` recopila cada issue encontrado durante la validacion y proporciona metodos de conveniencia para consultarlos.

### Metodos

#### HasErrors

```go
func (r *Result) HasErrors() bool
```

Retorna `true` si hay al menos un issue con severidad `error` o `fatal`.

```go
if result.HasErrors() {
    // el recurso no es valido
}
```

#### ErrorCount

```go
func (r *Result) ErrorCount() int
```

Retorna el numero de issues con severidad `error` o `fatal`.

#### WarningCount

```go
func (r *Result) WarningCount() int
```

Retorna el numero de issues con severidad `warning`.

#### InfoCount

```go
func (r *Result) InfoCount() int
```

Retorna el numero de issues con severidad `information`.

#### Merge

```go
func (r *Result) Merge(other *Result)
```

Agrega todos los issues de `other` a este resultado. Util al combinar resultados de multiples pasadas de validacion.

```go
combined := issue.NewResult()
combined.Merge(resultA)
combined.Merge(resultB)
```

#### Filter

```go
func (r *Result) Filter(severity Severity) *Result
```

Retorna un nuevo `Result` que contiene solo los issues que coinciden con la severidad indicada.

```go
errorsOnly := result.Filter(issue.SeverityError)
for _, iss := range errorsOnly.Issues {
    fmt.Println(iss.Diagnostics)
}
```

#### EnrichLocations

```go
func (r *Result) EnrichLocations(locator func(expression string) *Location)
```

Agrega informacion de linea y columna a los issues basandose en sus expresiones FHIRPath. El validador llama a este metodo automaticamente despues de la validacion; normalmente no es necesario invocarlo manualmente.

---

## Issue

```go
type Issue struct {
    Severity    Severity
    Code        Code
    Diagnostics string
    Expression  []string
    Location    *Location
    Source      string
    MessageID   string
}
```

Cada `Issue` corresponde a un hallazgo individual del proceso de validacion, alineado con la estructura de [OperationOutcome.issue](https://hl7.org/fhir/R4/operationoutcome-definitions.html#OperationOutcome.issue) de FHIR.

| Campo | Tipo | Descripcion |
|-------|------|-------------|
| `Severity` | `Severity` | Nivel de severidad: `fatal`, `error`, `warning` o `information` |
| `Code` | `Code` | Codigo de tipo de issue (ver [Constantes de Codigo](#constantes-de-codigo) abajo) |
| `Diagnostics` | `string` | Descripcion legible del issue |
| `Expression` | `[]string` | Expresion(es) FHIRPath que apuntan al elemento que causo el issue |
| `Location` | `*Location` | Linea y columna en el JSON fuente (se completa automaticamente) |
| `Source` | `string` | Nombre de la fase de validacion que genero el issue |
| `MessageID` | `string` | Identificador del catalogo de errores para manejo programatico |

---

## Location

```go
type Location struct {
    Line   int
    Column int
}
```

Representa una posicion en el documento JSON fuente. Ambos campos comienzan en 1. El `Location` se completa automaticamente por el validador despues de que todas las fases terminan.

---

## Stats

```go
type Stats struct {
    ResourceType    string
    ResourceSize    int
    ProfileURL      string
    IsCustomProfile bool
    Duration        int64 // nanoseconds
    PhasesRun       int
}
```

Las estadisticas de validacion se adjuntan a cada `Result`.

| Campo | Tipo | Descripcion |
|-------|------|-------------|
| `ResourceType` | `string` | Tipo de recurso FHIR (por ejemplo, `"Patient"`, `"Observation"`) |
| `ResourceSize` | `int` | Tamano del JSON de entrada en bytes |
| `ProfileURL` | `string` | URL canonica del perfil principal usado para la validacion |
| `IsCustomProfile` | `bool` | `true` cuando se uso un perfil personalizado en lugar de la definicion base del recurso |
| `Duration` | `int64` | Tiempo total de validacion en nanosegundos |
| `PhasesRun` | `int` | Numero de fases de validacion ejecutadas |

### DurationMs

```go
func (s *Stats) DurationMs() float64
```

Retorna la duracion en milisegundos como numero de punto flotante.

```go
fmt.Printf("Validado en %.2f ms\n", result.Stats.DurationMs())
```

---

## Constantes de Severidad

Los niveles de severidad estan alineados con el value set [IssueSeverity](https://hl7.org/fhir/R4/valueset-issue-severity.html) de FHIR.

```go
const (
    SeverityFatal       Severity = "fatal"
    SeverityError       Severity = "error"
    SeverityWarning     Severity = "warning"
    SeverityInformation Severity = "information"
)
```

| Constante | Valor | Significado |
|-----------|-------|-------------|
| `SeverityFatal` | `"fatal"` | El issue causo que la validacion se abortara |
| `SeverityError` | `"error"` | El recurso no es valido |
| `SeverityWarning` | `"warning"` | Se encontro un problema potencial pero el recurso aun puede ser valido |
| `SeverityInformation` | `"information"` | Mensaje informativo, no es un problema |

{{< callout type="info" >}}
Cuando se establece `WithStrictMode(true)`, las advertencias se promueven a errores. `HasErrors()` retorna `true` tanto para severidades `error` como `fatal` independientemente del modo estricto.
{{< /callout >}}

---

## Constantes de Codigo

Las constantes de codigo estan alineadas con el value set [IssueType](https://hl7.org/fhir/R4/valueset-issue-type.html) de FHIR.

```go
const (
    CodeInvalid       Code = "invalid"
    CodeStructure     Code = "structure"
    CodeRequired      Code = "required"
    CodeValue         Code = "value"
    CodeInvariant     Code = "invariant"
    CodeSecurity      Code = "security"
    CodeLogin         Code = "login"
    CodeUnknown       Code = "unknown"
    CodeExpired       Code = "expired"
    CodeForbidden     Code = "forbidden"
    CodeSuppressed    Code = "suppressed"
    CodeProcessing    Code = "processing"
    CodeNotSupported  Code = "not-supported"
    CodeDuplicate     Code = "duplicate"
    CodeMultipleMatch Code = "multiple-matches"
    CodeNotFound      Code = "not-found"
    CodeDeleted       Code = "deleted"
    CodeTooLong       Code = "too-long"
    CodeCodeInvalid   Code = "code-invalid"
    CodeExtension     Code = "extension"
    CodeTooCostly     Code = "too-costly"
    CodeBusinessRule  Code = "business-rule"
    CodeConflict      Code = "conflict"
    CodeTransient     Code = "transient"
    CodeLockError     Code = "lock-error"
    CodeNoStore       Code = "no-store"
    CodeException     Code = "exception"
    CodeTimeout       Code = "timeout"
    CodeIncomplete    Code = "incomplete"
    CodeThrottled     Code = "throttled"
    CodeInformational Code = "informational"
)
```

Los codigos mas frecuentes durante la validacion FHIR son:

| Codigo | Uso Tipico |
|--------|------------|
| `CodeStructure` | Elementos desconocidos, estructura JSON invalida |
| `CodeRequired` | Elementos requeridos faltantes (cardinalidad `min > 0`) |
| `CodeValue` | Valores primitivos invalidos (por ejemplo, fecha malformada, URI invalida) |
| `CodeInvariant` | Restriccion FHIRPath fallida |
| `CodeCodeInvalid` | Codigo no encontrado en el ValueSet o CodeSystem vinculado |
| `CodeExtension` | Extension invalida o desconocida |
| `CodeNotFound` | Perfil o recurso referenciado no encontrado |
| `CodeProcessing` | Errores generales de procesamiento |

---

## Procesando Resultados Programaticamente

```go
result, _ := v.Validate(ctx, resource)

// Verificacion rapida
if !result.HasErrors() {
    fmt.Println("El recurso es valido")
    return
}

// Iterar sobre todos los issues
for _, iss := range result.Issues {
    // Construir string de ubicacion
    loc := ""
    if iss.Location != nil {
        loc = fmt.Sprintf(" (linea %d, col %d)", iss.Location.Line, iss.Location.Column)
    }

    // Construir string de expresion
    expr := ""
    if len(iss.Expression) > 0 {
        expr = fmt.Sprintf(" en %s", iss.Expression[0])
    }

    fmt.Printf("[%s] %s: %s%s%s\n",
        iss.Severity, iss.Code, iss.Diagnostics, expr, loc)
}

// Filtrar solo errores
errors := result.Filter(issue.SeverityError)
fmt.Printf("\n%d error(es) encontrado(s)\n", len(errors.Issues))

// Acceder a estadisticas
fmt.Printf("Recurso:  %s (%d bytes)\n",
    result.Stats.ResourceType, result.Stats.ResourceSize)
fmt.Printf("Perfil:   %s\n", result.Stats.ProfileURL)
fmt.Printf("Duracion: %.2f ms (%d fases)\n",
    result.Stats.DurationMs(), result.Stats.PhasesRun)
```
