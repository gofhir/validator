---
title: "Errores de Constraints"
linkTitle: "Constraints"
description: "Errores relacionados con fallos en la evaluación de constraints FHIRPath."
weight: 7
---

Los errores de constraints ocurren cuando una invariante FHIRPath definida en el StructureDefinition se evalúa como `false` o encuentra un error de evaluación. FHIR define constraints (invariantes) en elementos usando expresiones FHIRPath. Cada constraint tiene una clave (ej., `ele-1`, `dom-6`), una severidad (`error` o `warning`), una descripción legible por humanos y una expresión FHIRPath que debe evaluarse como `true` para que el recurso sea válido.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `CONSTRAINT_FAILED` | varía | Constraint failed: {constraint}: '{human}' |
| `CONSTRAINT_ERROR` | warning | Constraint '{constraint}' evaluation error: {error} |

---

## CONSTRAINT_FAILED

Un constraint FHIRPath se evaluó como `false`. La severidad de este issue está determinada por la definición del constraint en sí -- cada constraint declara si la violación es un `error` o un `warning`.

**Ejemplo -- recurso inválido:**

El constraint `ele-1` está definido en el tipo base `Element` con la expresión `hasValue() or (children().count() > id.count())` y la descripción humana "All FHIR elements must have a @value or children". Si un elemento no tiene ni valor ni hijos, este constraint falla:

```json
{
  "resourceType": "Patient",
  "name": [
    {}
  ]
}
```

Salida de validación:

```text
ERROR: Constraint failed: ele-1: 'All FHIR elements must have a @value or children'
  Path: Patient.name[0]
  MessageID: CONSTRAINT_FAILED
```

**Corrección:** Asegúrate de que el elemento tenga un valor o hijos:

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

### Constraints Comunes

Aquí hay algunos constraints FHIR encontrados frecuentemente:

| Clave | Contexto | Descripción | Severidad |
|-------|----------|-------------|-----------|
| `ele-1` | Element | All FHIR elements must have a @value or children | error |
| `dom-3` | DomainResource | If the resource is contained in another resource, it SHALL be referred to from elsewhere in the resource | error |
| `dom-6` | DomainResource | A resource should have narrative for robust management | warning |
| `obs-6` | Observation | dataAbsentReason SHALL only be present if Observation.value[x] is not present | error |
| `obs-7` | Observation | If Observation.code is the same as an Observation.component.code then the value element associated with the code SHALL NOT be present | error |
| `pat-1` | Patient.contact | SHALL at least contain a contact's details or a reference to an organization | error |
| `ref-1` | Reference | SHALL have a contained resource if a local reference is provided | error |
| `per-1` | Period | If present, start SHALL have a lower value than end | error |
| `txt-1` | Narrative.div | The narrative SHALL contain only the basic html formatting elements and attributes | error |
| `txt-2` | Narrative.div | The narrative SHALL have some non-whitespace content | error |

### Severidad Desde la Definición del Constraint

La severidad de `CONSTRAINT_FAILED` no es fija. Se lee del campo `ElementDefinition.constraint.severity` en el StructureDefinition:

```json
{
  "key": "ele-1",
  "severity": "error",
  "human": "All FHIR elements must have a @value or children",
  "expression": "hasValue() or (children().count() > id.count())"
}
```

Si `severity` es `"error"`, el issue es un error. Si `severity` es `"warning"`, el issue es un warning. Los perfiles pueden agregar nuevos constraints o cambiar la severidad del constraint (dentro de ciertos límites).

{{< callout type="info" >}}
Los constraints se evalúan usando FHIRPath, un lenguaje de navegación y extracción basado en rutas para FHIR. La expresión del constraint debe retornar un valor booleano. Cuando retorna `false`, el constraint se considera violado.
{{< /callout >}}

---

## CONSTRAINT_ERROR

La expresión FHIRPath de un constraint no se pudo evaluar debido a un error en tiempo de ejecución. Esto no necesariamente significa que el recurso sea inválido -- significa que el validador no pudo determinar si el constraint se satisface. Esto siempre se reporta como un warning.

**Causas comunes:**

- La expresión FHIRPath referencia un tipo de elemento que el motor no soporta completamente
- La expresión usa una función FHIRPath que no está implementada
- Ocurrió un error en tiempo de ejecución durante la evaluación de la expresión (ej., discrepancia de tipos en una comparación)

**Ejemplo de salida:**

```text
WARNING: Constraint 'inv-1' evaluation error: Function 'iif' not implemented
  Path: Observation
  MessageID: CONSTRAINT_ERROR
```

{{< callout type="warning" >}}
Los errores de evaluación de constraints indican una limitación del motor FHIRPath, no necesariamente un problema con el recurso. Si los encuentras consistentemente, verifica si la función FHIRPath utilizada en el constraint está soportada por el GoFHIR Validator.
{{< /callout >}}

---

## Evaluación de Constraints a Nivel de Tipo

El validador evalúa constraints FHIRPath no solo del StructureDefinition del recurso, sino también de los StructureDefinitions de los tipos de datos complejos utilizados dentro del recurso. Por ejemplo, cuando un recurso `Patient` contiene un elemento `name[0].period` de tipo `Period`, el validador carga el StructureDefinition de Period y evalúa sus constraints (como `per-1`: start <= end) contra cada instancia de Period encontrada en el recurso.

Esta evaluación recursiva cubre todos los niveles de anidamiento. Por ejemplo, `Patient.name` (tipo HumanName) contiene `HumanName.period` (tipo Period), y el constraint `per-1` del SD de Period se evalúa aunque no aparezca en el snapshot del Patient.

### Funciones Adicionales FHIR

El motor de constraints soporta funciones FHIRPath específicas de FHIR definidas en [FHIR R4 §2.9.1.5](https://hl7.org/fhir/R4/fhirpath.html):

| Función | Descripción |
|---------|-------------|
| `resolve()` | Resuelve una referencia FHIR al recurso objetivo |
| `memberOf()` | Verifica si un código está en un ValueSet |
| `extension()` | Retorna extensions que coincidan con una URL |
| `hasExtension()` | Verifica si existe una extension |
| `conformsTo()` | Verifica si un recurso conforma con un perfil |
| `htmlChecks()` | Valida contenido XHTML narrativo según las reglas FHIR |

La función `htmlChecks()` valida que el contenido de `text.div` siga las reglas XHTML de FHIR (R4 §2.4.1): elemento raíz `<div>`, whitelist de elementos HTML permitidos, elementos/atributos prohibidos, y contenido no vacío. Esto permite que los constraints `txt-1` y `txt-2` se evalúen dinámicamente desde el StructureDefinition de Narrative.
