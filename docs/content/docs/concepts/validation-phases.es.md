---
title: "Fases de Validación"
linkTitle: "Fases de Validación"
description: "El GoFHIR Validator ejecuta 9 fases secuenciales para validar exhaustivamente cada recurso FHIR."
weight: 1
---

El GoFHIR Validator utiliza una **arquitectura de pipeline** que divide la validación en 9 fases discretas. Cada fase se enfoca en una categoría específica de reglas y se ejecuta en un orden fijo. Este diseño mantiene cada fase simple, testeable e independientemente extensible.

## Orden de Ejecución de las Fases

Cuando validas un recurso, las siguientes fases se ejecutan en secuencia:

| # | Fase | Paquete | Descripción |
|---|------|---------|-------------|
| 1 | Structural | `pkg/structural` | Elementos desconocidos, estructura JSON válida |
| 2 | Cardinality | `pkg/cardinality` | Restricciones min/max desde ElementDefinition |
| 3 | Primitive | `pkg/primitive` | Validación de tipos (patrones regex, tipos JSON) |
| 4 | Binding | `pkg/binding` | Validación de terminología (ValueSet/CodeSystem) |
| 5 | Extension | `pkg/extension` | Resolución de URL de extensiones, validación de contexto |
| 6 | Reference | `pkg/reference` | Formato de referencias y validación de tipos |
| 7 | Constraint | `pkg/constraint` | Evaluación de invariantes FHIRPath |
| 8 | Fixed/Pattern | `pkg/fixedpattern` | Restricciones `fixed[x]` y `pattern[x]` |
| 9 | Slicing | `pkg/slicing` | Coincidencia de discriminadores de slices y cardinalidad |

Cada fase recibe el árbol completo del recurso y el StructureDefinition resuelto, y luego produce cero o más objetos **Issue** describiendo cualquier violación encontrada.

## Cómo Funcionan las Fases

### 1. Structural

La fase estructural verifica que el payload JSON esté bien formado y que cada elemento en el recurso corresponda a un path conocido en el StructureDefinition. Los elementos desconocidos o mal escritos producen un error.

### 2. Cardinality

Esta fase recorre cada ElementDefinition en el snapshot y verifica que la cantidad de valores proporcionados esté dentro de los límites `min` y `max` declarados. Un elemento requerido (`min: 1`) que está ausente produce un error; un elemento que excede `max` también produce un error.

### 3. Primitive

Los tipos primitivos como `dateTime`, `uri`, `code` e `id` tienen reglas de formato definidas por FHIR (típicamente como expresiones regulares). Esta fase valida que cada valor primitivo coincida con el formato y tipo JSON esperados.

### 4. Binding

Cuando un ElementDefinition declara un `binding` de terminología, esta fase verifica si el code, Coding o CodeableConcept proporcionado es miembro del ValueSet vinculado. El comportamiento de validación depende de la fuerza del binding (ver [Terminología](../terminology)).

### 5. Extension

Las extensiones son un mecanismo central de extensibilidad en FHIR. Esta fase resuelve cada URL de extensión a su StructureDefinition, valida que la extensión se use en un contexto permitido y valida recursivamente el contenido de la extensión contra su perfil.

### 6. Reference

Las referencias FHIR (`Reference.reference`, `Reference.type`) deben conformar con los tipos de destino permitidos declarados en el ElementDefinition. Esta fase valida el formato de la referencia, verifica los tipos de destino declarados y valida los modos de agregación.

### 7. Constraint

Los StructureDefinitions pueden declarar invariantes FHIRPath mediante `ElementDefinition.constraint`. Esta fase evalúa cada expresión de constraint contra el recurso y reporta las violaciones. Por ejemplo, el recurso `Patient` tiene un constraint `name.exists() or identifier.exists()`.

### 8. Fixed/Pattern

Cuando un ElementDefinition especifica un valor `fixed[x]`, el elemento del recurso debe coincidir exactamente. Cuando especifica un valor `pattern[x]`, el elemento del recurso debe contener al menos los campos especificados. Esta fase aplica ambos.

### 9. Slicing

Los arrays de FHIR pueden dividirse en grupos nombrados usando discriminadores. Esta fase asocia cada elemento del array con el slice correcto basándose en los valores del discriminador, y luego valida que cada slice cumpla con sus propias restricciones de cardinalidad.

## Flujo de Validación de Perfiles

Cuando un recurso incluye una declaración `meta.profile`, el validador sigue este flujo:

1. **Cargar perfiles declarados** -- Cada URL en `meta.profile` se resuelve a un StructureDefinition.
2. **Resolver la cadena de perfiles** -- El validador recorre los enlaces `baseDefinition` hasta el tipo de recurso base, construyendo el conjunto completo de restricciones.
3. **Ejecutar las 9 fases** -- El pipeline se ejecuta contra cada perfil declarado. Un recurso con múltiples perfiles se valida contra cada uno.
4. **Fusionar issues** -- Todos los issues de todas las validaciones de perfiles se recopilan en un único resultado.

```text
Resource (meta.profile: "http://example.org/MyPatient")
  --> Load MyPatient StructureDefinition
    --> baseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient"
      --> baseDefinition: "http://hl7.org/fhir/StructureDefinition/DomainResource"
        --> baseDefinition: "http://hl7.org/fhir/StructureDefinition/Resource"
  --> Run 9 phases with merged constraints
  --> Return OperationOutcome
```

## Salida de Issues

Cada fase produce objetos **Issue** que siguen la estructura de `OperationOutcome.issue` de FHIR:

```go
type Issue struct {
    Severity    IssueSeverity   // fatal | error | warning | information
    Code        IssueType       // e.g., structure, required, value, code-invalid
    Diagnostics string          // Human-readable description
    Expression  []string        // FHIRPath to the offending element
}
```

Los issues se recopilan a través de todas las fases y se devuelven como parte del `OperationOutcome` final. Los niveles de severidad siguen la especificación FHIR:

- **fatal** -- El validador no pudo procesar el recurso (por ejemplo, JSON inválido).
- **error** -- El recurso viola una regla obligatoria y no es conformante.
- **warning** -- El recurso puede tener un problema pero no es necesariamente inválido (por ejemplo, incompatibilidad de binding extensible).
- **information** -- Notas informativas (por ejemplo, sugerencias de binding preferred).

{{< callout type="info" >}}
Las fases están diseñadas para ser **fail-safe**: si una fase anterior encuentra un error fatal (como JSON no parseable), las fases siguientes se omiten de forma controlada. Los errores no fatales en una fase no impiden que las fases posteriores se ejecuten.
{{< /callout >}}
