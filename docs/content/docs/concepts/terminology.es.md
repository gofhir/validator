---
title: "Terminología"
linkTitle: "Terminología"
description: "Cómo el GoFHIR Validator maneja CodeSystems, ValueSets y fuerzas de binding."
weight: 3
---

FHIR utiliza terminología estructurada -- CodeSystems y ValueSets -- para asegurar que los valores codificados sean clínicamente significativos e interoperables. El GoFHIR Validator verifica los elementos codificados contra sus bindings de terminología declarados, con un comportamiento que varía según la fuerza del binding.

## CodeSystems vs. ValueSets

Entender la distinción entre estos dos recursos es esencial:

- **CodeSystem** -- Define un conjunto de códigos y sus significados. Ejemplos incluyen LOINC, SNOMED CT y sistemas de códigos definidos por FHIR como `http://hl7.org/fhir/administrative-gender`. Un CodeSystem es la fuente autoritativa de códigos.

- **ValueSet** -- Define una selección de códigos extraídos de uno o más CodeSystems. Un ValueSet puede incluir todos los códigos de un CodeSystem, un subconjunto filtrado o una combinación de códigos de múltiples sistemas. Los ValueSets son lo que referencian los bindings.

```text
CodeSystem: http://hl7.org/fhir/administrative-gender
  Codes: male, female, other, unknown

ValueSet: http://hl7.org/fhir/ValueSet/administrative-gender
  Includes: all codes from http://hl7.org/fhir/administrative-gender
```

## Fuerzas de Binding

Cuando un ElementDefinition declara un `binding`, especifica tanto un ValueSet como una **fuerza** que determina cuán estrictamente el validador aplica la pertenencia. FHIR define cuatro fuerzas de binding:

### Required

El código **debe** provenir del ValueSet especificado. Si el código no es miembro, el validador produce un **error**.

Este es el binding más estricto. Se usa para elementos donde la interoperabilidad exige un conjunto fijo de valores, como `Patient.gender` u `Observation.status`.

```json
{
  "path": "Patient.gender",
  "binding": {
    "strength": "required",
    "valueSet": "http://hl7.org/fhir/ValueSet/administrative-gender"
  }
}
```

### Extensible

El código **debería** provenir del ValueSet especificado. Si el código no es miembro pero proviene de un sistema alternativo, el validador produce un **warning**. Si no existe un código apropiado en el ValueSet, los implementadores pueden usar códigos de otros sistemas.

Esta fuerza equilibra interoperabilidad con flexibilidad. Se usa comúnmente en perfiles como US Core.

```json
{
  "path": "Condition.code",
  "binding": {
    "strength": "extensible",
    "valueSet": "http://hl7.org/fhir/us/core/ValueSet/us-core-condition-code"
  }
}
```

### Preferred

El código es **recomendado** del ValueSet especificado. El validador produce una nota **informativa** si el código no es miembro. Es una recomendación suave sin impacto en la conformancia.

```json
{
  "path": "Encounter.type",
  "binding": {
    "strength": "preferred",
    "valueSet": "http://hl7.org/fhir/ValueSet/encounter-type"
  }
}
```

### Example

El ValueSet se proporciona solo como **ejemplo**. El validador **no realiza validación** contra bindings de ejemplo. Existen puramente para documentación y orientación.

```json
{
  "path": "Procedure.code",
  "binding": {
    "strength": "example",
    "valueSet": "http://hl7.org/fhir/ValueSet/procedure-code"
  }
}
```

## Resumen del Comportamiento de Binding

| Fuerza | Código no en ValueSet | Severidad del Issue |
|--------|----------------------|---------------------|
| **required** | Debe estar en el ValueSet | error |
| **extensible** | Debería estar en el ValueSet | warning |
| **preferred** | Recomendado | information |
| **example** | Sin validación | _(ninguna)_ |

## Binding en un ElementDefinition

Aquí hay un ejemplo completo de cómo aparece un binding de terminología dentro de un ElementDefinition:

```json
{
  "id": "Observation.status",
  "path": "Observation.status",
  "min": 1,
  "max": "1",
  "type": [
    { "code": "code" }
  ],
  "binding": {
    "strength": "required",
    "description": "Codes providing the status of an observation.",
    "valueSet": "http://hl7.org/fhir/ValueSet/observation-status"
  }
}
```

El validador lee este binding, carga el ValueSet referenciado, lo expande en una lista plana de códigos válidos y verifica si el valor en el recurso es miembro.

## Validación de Terminología Local vs. Externa

El GoFHIR Validator soporta **validación de terminología local** usando recursos CodeSystem y ValueSet cargados en el registro. Cuando cargas una Guía de Implementación o proporcionas recursos de terminología directamente, el validador puede resolver y expandir ValueSets localmente sin ninguna dependencia externa.

Para recursos de terminología cargados localmente:

1. El validador resuelve la URL del ValueSet desde el binding.
2. Expande el ValueSet evaluando las reglas `include` y `exclude` contra los CodeSystems cargados.
3. Verifica si el código proporcionado es miembro del conjunto expandido.

```go
v, err := validator.New(
    validator.WithValueSets(myValueSets...),
    validator.WithCodeSystems(myCodeSystems...),
)
```

{{< callout type="warning" >}}
**Los servidores de terminología externos aún no son soportados.** El GoFHIR Validator actualmente realiza toda la validación de terminología localmente usando recursos cargados. El soporte para servidores de terminología FHIR externos (la operación `$validate-code`) está planificado para una versión futura. Por ahora, asegúrate de que todos los recursos CodeSystem y ValueSet requeridos estén cargados en el validador.
{{< /callout >}}

## Deshabilitar la Validación de Terminología

En algunos escenarios puedes querer omitir la validación de terminología por completo -- por ejemplo, al probar conformancia estructural sin cargar recursos de terminología. Usa el flag `-tx n/a` con el CLI:

```bash
gofhir-validator -tx n/a patient.json
```

O programáticamente:

```go
v, err := validator.New(
    validator.WithTerminologyDisabled(),
)
```

Cuando la validación de terminología está deshabilitada, la fase de binding se omite y no se reportan issues relacionados con terminología.
