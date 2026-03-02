---
title: "Referencia de Errores"
linkTitle: "Referencia de Errores"
description: "Catálogo completo de códigos de error, plantillas de mensajes y severidades producidos por el GoFHIR Validator."
weight: 8
---

El GoFHIR Validator produce issues de validación estructurados que están alineados con la salida del [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator). Cada issue incluye un identificador de mensaje legible por máquinas, un nivel de severidad, un diagnóstico legible por humanos y la expresión FHIRPath que identifica el elemento con problemas.

## Estructura del Issue

Cada issue de validación contiene los siguientes campos:

| Campo | Tipo | Descripción |
|-------|------|-------------|
| **Severity** | `error` \| `warning` \| `information` | Qué tan crítico es el problema |
| **Code** | string | El tipo de issue FHIR (ej., `structure`, `value`, `processing`) |
| **Diagnostics** | string | Una descripción legible por humanos del problema |
| **Expression** | string[] | Expresión(es) FHIRPath que apuntan al elemento |
| **MessageID** | string | Un identificador estable y legible por máquinas para el error |

Ejemplo de issue en JSON (formato OperationOutcome):

```json
{
  "severity": "error",
  "code": "value",
  "diagnostics": "Value 'not-a-date' is not a valid date format",
  "expression": ["Patient.birthDate"],
  "details": {
    "text": "TYPE_INVALID_DATE"
  }
}
```

## Placeholders en los Mensajes

Los mensajes de diagnóstico usan placeholders que se reemplazan con valores específicos del contexto en tiempo de ejecución:

| Placeholder | Descripción | Ejemplo |
|-------------|-------------|---------|
| `{path}` | FHIRPath al elemento | `Patient.identifier` |
| `{value}` | El valor encontrado | `not-a-date` |
| `{expected}` | El valor o tipo esperado | `dateTime` |
| `{min}` | Cardinalidad o conteo mínimo | `1` |
| `{max}` | Cardinalidad o conteo máximo | `1` |
| `{count}` | Conteo real encontrado | `0` |
| `{type}` | El tipo de dato encontrado | `string` |
| `{element}` | El nombre del elemento | `unknownField` |
| `{valueSet}` | URL del ValueSet | `http://hl7.org/fhir/ValueSet/gender` |
| `{constraint}` | Clave del constraint | `ele-1` |
| `{profile}` | URL del perfil | `http://hl7.org/fhir/StructureDefinition/Patient` |
| `{system}` | URL del sistema de códigos | `http://loinc.org` |
| `{url}` | URL de extension o recurso | `http://example.org/ext` |
| `{error}` | Mensaje de error subyacente | `unexpected token` |
| `{human}` | Texto legible del constraint | `All FHIR elements must have a @value or children` |
| `{slice}` | Nombre del slice | `Observation.component:diastolic` |

{{< callout type="info" >}}
**Manejo programático de errores.** Utiliza el campo `MessageID` para identificar errores específicos en tu código en lugar de parsear el texto del diagnóstico. Los MessageID son estables entre versiones y no se ven afectados por cambios de redacción en los mensajes de diagnóstico.
{{< /callout >}}

## Categorías de Errores

Navega la referencia de errores por categoría:

{{< cards >}}
  {{< card link="structural" title="Errores Estructurales" subtitle="Estructura JSON, elementos desconocidos, resourceType" icon="cube" >}}
  {{< card link="cardinality" title="Errores de Cardinalidad" subtitle="Violaciones de ocurrencias mínimas y máximas" icon="calculator" >}}
  {{< card link="types" title="Errores de Tipos" subtitle="Formato de primitivos y discrepancias de tipos complejos" icon="variable" >}}
  {{< card link="terminology" title="Errores de Terminología" subtitle="Validación de codificación, binding y ValueSet" icon="book-open" >}}
  {{< card link="extensions" title="Errores de Extensions" subtitle="Extensions desconocidas, inválidas y modifier" icon="puzzle" >}}
  {{< card link="references" title="Errores de Referencias" subtitle="Referencias inválidas, targets y resolución" icon="link" >}}
  {{< card link="constraints" title="Errores de Constraints" subtitle="Fallos en evaluación de constraints FHIRPath" icon="shield-check" >}}
  {{< card link="fixed-pattern" title="Errores de Fixed/Pattern" subtitle="Fallos en coincidencia de valores fijos y patrones" icon="lock-closed" >}}
  {{< card link="slicing" title="Errores de Slicing" subtitle="Coincidencia de slices, cardinalidad y slicing cerrado" icon="scissors" >}}
  {{< card link="profiles" title="Errores de Perfiles" subtitle="Resolución de perfiles y discrepancias de tipos" icon="document-search" >}}
{{< /cards >}}
