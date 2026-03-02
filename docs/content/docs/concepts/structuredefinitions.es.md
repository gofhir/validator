---
title: "StructureDefinitions"
linkTitle: "StructureDefinitions"
description: "Cómo el GoFHIR Validator deriva todas las reglas de validación desde los StructureDefinitions de FHIR -- nunca hardcodeadas."
weight: 2
---

El GoFHIR Validator sigue un principio de diseño fundamental: **cada regla de validación se deriva de los StructureDefinitions**. El validador nunca hardcodea nombres de elementos, cardinalidades, tipos permitidos ni ninguna otra regla. Esto hace que el validador sea agnóstico de versión y capaz de validar contra cualquier perfil, incluyendo los personalizados.

## Qué Es un StructureDefinition

Un [StructureDefinition](https://hl7.org/fhir/R4/structuredefinition.html) es el recurso FHIR que define la forma de otros recursos. Describe qué elementos existen, qué tipos pueden tener, cuántas veces pueden repetirse y qué restricciones adicionales aplican. Cada tipo de recurso FHIR (Patient, Observation, etc.) tiene un StructureDefinition base, y los perfiles crean StructureDefinitions adicionales que restringen aún más la base.

## Campos Clave de ElementDefinition

Cada elemento dentro de un StructureDefinition se describe mediante un [ElementDefinition](https://hl7.org/fhir/R4/elementdefinition.html). El validador utiliza los siguientes campos para dirigir la validación:

### `path`

La ubicación del elemento separada por puntos dentro del árbol del recurso.

```text
Patient.name
Patient.name.given
Observation.value[x]
```

### `min` / `max`

Restricciones de cardinalidad. `min` es un entero (0 o mayor); `max` es un string (`"0"`, `"1"`, `"*"`, etc.).

```json
{
  "path": "Patient.identifier",
  "min": 1,
  "max": "*"
}
```

Esto significa que se requiere al menos un `identifier`, sin límite superior.

### `type`

La lista de tipos de datos permitidos para el elemento. Cada tipo puede opcionalmente declarar perfiles de destino.

```json
{
  "path": "Observation.value[x]",
  "type": [
    { "code": "Quantity" },
    { "code": "string" },
    { "code": "CodeableConcept" }
  ]
}
```

### `binding`

Un binding de terminología que vincula un elemento codificado a un ValueSet. Consulta [Terminología](../terminology) para detalles sobre las fuerzas de binding.

```json
{
  "path": "Observation.status",
  "binding": {
    "strength": "required",
    "valueSet": "http://hl7.org/fhir/ValueSet/observation-status"
  }
}
```

### `constraint`

Invariantes FHIRPath que deben evaluarse como `true` para que el recurso sea válido.

```json
{
  "path": "Patient",
  "constraint": [
    {
      "key": "pat-1",
      "severity": "error",
      "human": "SHALL at least contain a contact's details or a reference to an organization",
      "expression": "name.exists() or telecom.exists() or address.exists() or organization.exists()"
    }
  ]
}
```

### `fixed[x]` / `pattern[x]`

Los valores fixed requieren una coincidencia exacta. Los valores pattern requieren que el elemento del recurso contenga al menos los campos especificados (pero puede contener más).

```json
{
  "path": "Observation.code",
  "patternCodeableConcept": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9"
      }
    ]
  }
}
```

### `slicing`

Define cómo un elemento repetido se divide en slices nombrados usando discriminadores.

```json
{
  "path": "Patient.identifier",
  "slicing": {
    "discriminator": [
      { "type": "value", "path": "system" }
    ],
    "rules": "open"
  }
}
```

## Snapshot vs. Differential

Un StructureDefinition puede contener dos representaciones de sus elementos:

- **Snapshot** -- El conjunto completo y totalmente expandido de elementos. Cada elemento del tipo de recurso base se lista con cualquier sobreescritura aplicada. Esto es lo que usa el validador.
- **Differential** -- Solo los elementos que difieren de la definición base. Es una representación compacta usada para la autoría de perfiles.

```text
Profile (differential: 5 overridden elements)
  + Base StructureDefinition (snapshot: 80 elements)
  = Computed snapshot (80 elements with 5 overrides applied)
```

El GoFHIR Validator puede trabajar con StructureDefinitions que proporcionen solo un differential. Cuando no está presente un snapshot, el validador genera uno fusionando el differential con el snapshot de la definición base.

## Cadena de Resolución de Perfiles

Los perfiles forman una cadena de derivación a través del campo `baseDefinition`. El validador resuelve esta cadena para construir el conjunto completo de restricciones:

```text
http://example.org/fhir/StructureDefinition/MyPatient
  --> baseDefinition: http://hl7.org/fhir/StructureDefinition/Patient
    --> baseDefinition: http://hl7.org/fhir/StructureDefinition/DomainResource
      --> baseDefinition: http://hl7.org/fhir/StructureDefinition/Resource
```

En cada nivel, el differential se aplica sobre el snapshot del padre. El resultado final es un snapshot completamente resuelto que incluye todas las restricciones de cada nivel en la cadena.

## Ejemplo Mínimo de StructureDefinition

A continuación se muestra un StructureDefinition simplificado que restringe `Patient` para requerir al menos un `identifier`:

```json
{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/fhir/StructureDefinition/RequiredIdentifierPatient",
  "name": "RequiredIdentifierPatient",
  "status": "active",
  "kind": "resource",
  "abstract": false,
  "type": "Patient",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
  "derivation": "constraint",
  "differential": {
    "element": [
      {
        "id": "Patient.identifier",
        "path": "Patient.identifier",
        "min": 1
      }
    ]
  }
}
```

Cuando el validador procesa este perfil, resuelve el StructureDefinition base de `Patient`, genera un snapshot aplicando el differential (estableciendo `Patient.identifier.min` en `1`), y luego ejecuta las 9 fases de validación contra el resultado fusionado.

{{< callout type="info" >}}
El GoFHIR Validator es **agnóstico de versión**. No asume ninguna versión particular de FHIR (R4, R4B, R5, etc.). Todo el comportamiento se controla mediante los StructureDefinitions que cargues. Proporciona definiciones R5 y valida recursos R5; proporciona definiciones R4 y valida recursos R4.
{{< /callout >}}
