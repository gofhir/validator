---
title: "Errores de Fixed/Pattern"
linkTitle: "Fixed/Pattern"
description: "Errores relacionados con discrepancias de valores fijos y fallos en coincidencia de patrones."
weight: 8
---

Los errores de fixed y pattern ocurren cuando el valor de un elemento no coincide con un constraint `fixed[x]` o `pattern[x]` declarado en el ElementDefinition. Estos dos tipos de constraints sirven propósitos diferentes:

- **fixed[x]** -- El valor del elemento debe coincidir con el valor fijo **exactamente**. No se permiten propiedades adicionales más allá de lo especificado.
- **pattern[x]** -- El valor del elemento debe contener al menos las propiedades especificadas en el patrón, pero se permiten propiedades adicionales.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `FIXED_VALUE_MISMATCH` | error | Value '{value}' does not match fixed value '{expected}' |
| `PATTERN_MISMATCH` | error | Value does not match required pattern at '{path}' |

---

## FIXED_VALUE_MISMATCH

El valor actual del elemento no coincide con el valor fijo declarado en el ElementDefinition. Cuando un elemento tiene un constraint `fixed[x]`, el valor en el recurso debe ser idéntico al valor fijo.

**Ejemplo -- recurso inválido:**

Un perfil establece `Observation.status` con un valor fijo de `"final"`:

```json
{
  "id": "Observation.status",
  "path": "Observation.status",
  "fixedCode": "final"
}
```

Un recurso que viola este constraint:

```json
{
  "resourceType": "Observation",
  "status": "preliminary",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  }
}
```

Salida de validación:

```text
ERROR: Value 'preliminary' does not match fixed value 'final'
  Path: Observation.status
  MessageID: FIXED_VALUE_MISMATCH
```

**Corrección:** Usa el valor exacto especificado por el perfil:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  }
}
```

### Valores Fijos en Tipos Complejos

Los valores fijos también pueden aplicarse a tipos complejos. En este caso, cada propiedad en el valor fijo debe coincidir exactamente:

```json
{
  "id": "Observation.code",
  "path": "Observation.code",
  "fixedCodeableConcept": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9"
      }
    ]
  }
}
```

El recurso debe incluir exactamente el mismo coding sin codings o propiedades adicionales.

{{< callout type="warning" >}}
Los valores fijos son muy estrictos. Para tipos complejos, usar `pattern[x]` es generalmente más apropiado porque permite datos adicionales junto al patrón requerido. Los valores fijos se usan típicamente solo para elementos primitivos donde se requiere una coincidencia exacta.
{{< /callout >}}

---

## PATTERN_MISMATCH

El valor del elemento no incluye todas las propiedades especificadas en el patrón. A diferencia de los valores fijos, los patrones solo requieren que las propiedades especificadas estén presentes con los valores correctos -- se permiten propiedades adicionales.

**Ejemplo -- recurso inválido:**

Un perfil define un patrón en `Observation.code`:

```json
{
  "id": "Observation.code",
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

Un recurso que viola este patrón:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://snomed.info/sct",
        "code": "271649006"
      }
    ]
  }
}
```

El patrón requiere un coding con system `http://loinc.org` y code `85354-9`, pero el recurso solo contiene un coding SNOMED.

Salida de validación:

```text
ERROR: Value does not match required pattern at 'Observation.code'
  Path: Observation.code
  MessageID: PATTERN_MISMATCH
```

**Corrección:** Incluye el coding del patrón requerido (se permiten codings adicionales):

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9",
        "display": "Blood pressure panel"
      },
      {
        "system": "http://snomed.info/sct",
        "code": "271649006",
        "display": "Systemic arterial pressure"
      }
    ]
  }
}
```

### Diferencias Clave Entre Fixed y Pattern

| Aspecto | fixed[x] | pattern[x] |
|---------|----------|------------|
| Tipo de coincidencia | Coincidencia exacta | Coincidencia de subconjunto |
| Propiedades adicionales | No permitidas | Permitidas |
| Uso común | Valores primitivos | Tipos complejos |
| Estrictez | Muy estricto | Flexible |

{{< callout type="info" >}}
Tanto los valores `fixed[x]` como `pattern[x]` se leen del ElementDefinition en el StructureDefinition. El validador compara el valor del recurso contra el constraint declarado usando igualdad profunda (para fixed) o coincidencia de subconjunto (para pattern).
{{< /callout >}}
