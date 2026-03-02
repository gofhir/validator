---
title: "Errores de Cardinalidad"
linkTitle: "Cardinalidad"
description: "Errores relacionados con restricciones de ocurrencias mínimas y máximas en elementos."
weight: 2
---

Los errores de cardinalidad ocurren cuando el número de ocurrencias de un elemento no está dentro del rango declarado en su ElementDefinition. Cada elemento en un StructureDefinition de FHIR tiene propiedades `min` y `max` que definen cuántas veces puede (o debe) aparecer.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `CARDINALITY_MIN` | error | Minimum cardinality of '{path}' is {min}, but found {count} |
| `CARDINALITY_MAX` | error | Maximum cardinality of '{path}' is {max}, but found {count} |

---

## CARDINALITY_MIN

El elemento aparece menos veces de lo que requiere la cardinalidad mínima. Cuando `min` es `1` o mayor, el elemento es obligatorio y debe estar presente en el recurso.

**Ejemplo -- recurso inválido:**

Un recurso Patient sin el elemento requerido `gender` cuando se valida contra un perfil que establece `Patient.gender` con `min: 1`:

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

Salida de validación:

```text
ERROR: Minimum cardinality of 'Patient.gender' is 1, but found 0
  Path: Patient.gender
  MessageID: CARDINALITY_MIN
```

**Corrección:** Agrega el elemento requerido:

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ],
  "gender": "male"
}
```

### Casos Comunes

Muchos recursos base de FHIR tienen elementos requeridos. Por ejemplo:

- `Observation.status` (min: 1)
- `Observation.code` (min: 1)
- `Patient.name` en US Core (min: 1)
- `Bundle.type` (min: 1)
- `Medication.code` en muchos perfiles (min: 1)

{{< callout type="info" >}}
Las restricciones de cardinalidad siempre se derivan de `ElementDefinition.min` y `ElementDefinition.max` en el StructureDefinition. Los perfiles pueden aumentar el mínimo (haciendo elementos obligatorios) pero nunca pueden disminuirlo por debajo del valor de la definición base.
{{< /callout >}}

---

## CARDINALITY_MAX

El elemento aparece más veces de lo que permite la cardinalidad máxima. Cuando `max` es `"1"`, el elemento debe aparecer como máximo una vez. Cuando `max` es `"0"`, el elemento está prohibido.

**Ejemplo -- recurso inválido:**

Un Patient con múltiples valores de `birthDate` (la cardinalidad máxima es `"1"`):

```json
{
  "resourceType": "Patient",
  "birthDate": ["1990-01-01", "1991-02-02"]
}
```

Dado que `Patient.birthDate` tiene `max: "1"`, proporcionar un array con múltiples valores no está permitido.

Salida de validación:

```text
ERROR: Maximum cardinality of 'Patient.birthDate' is 1, but found 2
  Path: Patient.birthDate
  MessageID: CARDINALITY_MAX
```

**Corrección:** Proporciona solo un valor:

```json
{
  "resourceType": "Patient",
  "birthDate": "1990-01-01"
}
```

### Cardinalidad Máxima de Cero

Los perfiles pueden establecer `max: "0"` para prohibir un elemento por completo. Cuando esto ocurre, cualquier aparición del elemento produce un error `CARDINALITY_MAX` con `max` mostrado como `0`.

```text
ERROR: Maximum cardinality of 'Patient.deceased[x]' is 0, but found 1
  Path: Patient.deceased[x]
  MessageID: CARDINALITY_MAX
```

## Cómo Se Determina la Cardinalidad

El validador lee `min` y `max` del `ElementDefinition` en el snapshot del perfil:

```json
{
  "id": "Patient.gender",
  "path": "Patient.gender",
  "min": 1,
  "max": "1",
  "type": [
    { "code": "code" }
  ]
}
```

- `min` es un entero (0, 1, 2, ...).
- `max` es una cadena que es un número (`"0"`, `"1"`, `"2"`, ...) o `"*"` (sin límite).
- Los perfiles pueden restringir la cardinalidad (aumentar min, disminuir max) pero no relajarla más allá de la definición base.
