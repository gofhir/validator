---
title: "Errores de Slicing"
linkTitle: "Slicing"
description: "Errores relacionados con coincidencia de slices, violaciones de cardinalidad dentro de slices y reglas de slicing cerrado."
weight: 9
---

Los errores de slicing ocurren cuando los elementos en una lista repetida no se ajustan a las reglas de slicing declaradas en el StructureDefinition. El slicing de FHIR permite a los perfiles definir sub-grupos (slices) dentro de un elemento repetido, cada uno con sus propias restricciones. El validador verifica que los elementos coincidan con los slices correctos, que la cardinalidad de los slices se respete y que las reglas de slicing cerrado no se violen.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `SLICE_UNMATCHED_CLOSED` | error | Element at '{path}' does not match any slice (closed slicing) |
| `SLICE_MIN_NOT_MET` | error | Slice '{slice}' requires minimum {min} occurrence(s), found {count} |
| `SLICE_MAX_EXCEEDED` | error | Slice '{slice}' allows maximum {max} occurrence(s), found {count} |

---

## SLICE_UNMATCHED_CLOSED

Un elemento en una lista con slicing no coincide con ninguno de los slices definidos, y el slicing está declarado como **cerrado** (es decir, `slicing.rules` es `"closed"` o `"openAtEnd"`). En slicing cerrado, cada elemento debe coincidir con exactamente un slice.

**Ejemplo -- recurso inválido:**

Un perfil define slicing en `Observation.component` con reglas cerradas, definiendo dos slices: `systolic` y `diastolic`. Un elemento que no coincide con ningún slice dispara este error:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8462-4" }]
      },
      "valueQuantity": { "value": 80, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8867-4" }]
      },
      "valueQuantity": { "value": 72, "unit": "/min" }
    }
  ]
}
```

Si el tercer componente (frecuencia cardíaca, LOINC 8867-4) no coincide con el slice `systolic` ni `diastolic` y el slicing es cerrado, el validador produce este error.

Salida de validación:

```text
ERROR: Element at 'Observation.component[2]' does not match any slice (closed slicing)
  Path: Observation.component[2]
  MessageID: SLICE_UNMATCHED_CLOSED
```

**Corrección:** Elimina el elemento que no coincide o usa un perfil que lo permita (slicing abierto).

### Reglas de Slicing

El campo `slicing.rules` en el ElementDefinition determina cómo se manejan los elementos que no coinciden:

| Reglas | Comportamiento |
|--------|---------------|
| `open` | Los elementos que no coinciden están permitidos (sin error) |
| `closed` | Todos los elementos deben coincidir con un slice (error si no coinciden) |
| `openAtEnd` | Los elementos que no coinciden solo están permitidos al final de la lista |

---

## SLICE_MIN_NOT_MET

Un slice tiene una cardinalidad mínima que no se satisface. Cada slice puede tener su propio valor `min` que requiere un número mínimo de elementos que coincidan con ese slice.

**Ejemplo -- recurso inválido:**

Un perfil de presión arterial requiere al menos un componente `systolic` (min: 1) y al menos un componente `diastolic` (min: 1). Un recurso con solo el componente sistólico:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    }
  ]
}
```

Salida de validación:

```text
ERROR: Slice 'Observation.component:diastolic' requires minimum 1 occurrence(s), found 0
  Path: Observation.component
  MessageID: SLICE_MIN_NOT_MET
```

**Corrección:** Agrega el componente diastólico requerido:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8462-4" }]
      },
      "valueQuantity": { "value": 80, "unit": "mmHg" }
    }
  ]
}
```

---

## SLICE_MAX_EXCEEDED

Más elementos coinciden con un slice de lo que la cardinalidad máxima del slice permite. Cada slice puede tener su propio valor `max` que limita cuántos elementos pueden coincidir con él.

**Ejemplo:**

Un perfil define un slice `Observation.component:systolic` con cardinalidad máxima de `"1"`. Si dos componentes coinciden con el discriminador sistólico:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 118, "unit": "mmHg" }
    }
  ]
}
```

Salida de validación:

```text
ERROR: Slice 'Observation.component:systolic' allows maximum 1 occurrence(s), found 2
  Path: Observation.component
  MessageID: SLICE_MAX_EXCEEDED
```

**Corrección:** Asegúrate de que solo el número permitido de elementos coincida con cada slice.

## Cómo Funciona el Slicing

El slicing se define en el StructureDefinition mediante un elemento `slicing` en el path base, seguido de definiciones individuales de slices. El `slicing.discriminator` especifica cómo los elementos se emparejan con los slices:

```json
{
  "id": "Observation.component",
  "path": "Observation.component",
  "slicing": {
    "discriminator": [
      {
        "type": "pattern",
        "path": "code"
      }
    ],
    "rules": "open"
  }
}
```

Cada slice luego usa un patrón o valor en el path del discriminador para definir qué elementos le pertenecen:

```json
{
  "id": "Observation.component:systolic",
  "path": "Observation.component",
  "sliceName": "systolic",
  "min": 1,
  "max": "1",
  "patternCodeableConcept": {
    "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
  }
}
```

{{< callout type="info" >}}
Las reglas de slicing, los discriminadores y las cardinalidades de los slices se leen del StructureDefinition. El validador no hardcodea ninguna suposición sobre qué elementos pueden tener slicing o cómo se definen los slices.
{{< /callout >}}
