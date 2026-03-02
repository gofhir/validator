---
title: "Errores de Referencias"
linkTitle: "Referencias"
description: "Errores relacionados con referencias a recursos FHIR, tipos de destino y resolución de referencias."
weight: 6
---

Los errores de referencias ocurren cuando las referencias a recursos FHIR están malformadas, apuntan a tipos de destino inválidos o no pueden resolverse. Las referencias FHIR conectan recursos entre sí y están restringidas por las entradas `ElementDefinition.type` que declaran qué tipos de recursos destino están permitidos. El validador verifica el formato de referencia, la compatibilidad del tipo de destino y opcionalmente si el recurso referenciado existe.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `REFERENCE_INVALID_FORMAT` | error | Reference '{value}' has invalid format |
| `REFERENCE_INVALID_TARGET` | error | Reference at '{path}' to '{value}' is not a valid target (expected {expected}) |
| `REFERENCE_NOT_FOUND` | warning | Referenced resource '{value}' not found |
| `REFERENCE_TYPE_MISMATCH` | error | Reference targets {type} but only {expected} allowed |

---

## REFERENCE_INVALID_FORMAT

El valor de la referencia no se ajusta a ningún formato de referencia FHIR válido. Las referencias FHIR pueden tomar varias formas:

- **Referencia relativa**: `ResourceType/id` (ej., `Patient/123`)
- **Referencia absoluta**: `http://example.org/fhir/Patient/123`
- **Referencia de fragmento interno**: `#resource-id`
- **Referencia lógica** (via `identifier`): Usa el elemento `identifier` en lugar de `reference`
- **Referencia UUID**: `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "just-an-id"
  }
}
```

El valor `just-an-id` no coincide con ningún formato de referencia válido.

**Corrección:** Usa un formato de referencia válido:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "Patient/123"
  }
}
```

---

## REFERENCE_INVALID_TARGET

La referencia apunta a un tipo de recurso que no está permitido por el ElementDefinition. Cada elemento de referencia en un StructureDefinition lista los tipos de destino permitidos a través de `ElementDefinition.type.targetProfile`.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "Organization/456"
  }
}
```

Si el perfil restringe `Observation.subject` a referenciar solo `Patient` o `Group`, una referencia a `Organization` no es un destino válido.

**Corrección:** Referencia uno de los tipos de destino permitidos:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "Patient/123"
  }
}
```

---

## REFERENCE_NOT_FOUND

El recurso referenciado no se pudo resolver dentro del contexto de validación. Esto es un warning porque el recurso puede existir en un servidor externo al que el validador no tiene acceso. Este error es más relevante al validar Bundles, donde se espera que las referencias contenidas y las referencias internas del Bundle sean resolubles.

**Ejemplo:**

```json
{
  "resourceType": "Bundle",
  "type": "transaction",
  "entry": [
    {
      "resource": {
        "resourceType": "Observation",
        "status": "final",
        "code": {
          "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
        },
        "subject": {
          "reference": "Patient/999"
        }
      }
    }
  ]
}
```

Si `Patient/999` no está incluido en las entradas del Bundle, el validador produce este warning.

**Corrección:** Incluye el recurso referenciado en el Bundle o usa un recurso contenido:

```json
{
  "resourceType": "Bundle",
  "type": "transaction",
  "entry": [
    {
      "resource": {
        "resourceType": "Patient",
        "id": "999",
        "name": [{ "family": "Smith" }]
      }
    },
    {
      "resource": {
        "resourceType": "Observation",
        "status": "final",
        "code": {
          "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
        },
        "subject": {
          "reference": "Patient/999"
        }
      }
    }
  ]
}
```

---

## REFERENCE_TYPE_MISMATCH

El recurso referenciado se resolvió y su `resourceType` no coincide con los tipos de destino permitidos para este elemento de referencia. Esto difiere de `REFERENCE_INVALID_TARGET` en que el formato de referencia en sí es válido, pero el recurso resuelto real tiene el tipo incorrecto.

**Ejemplo:**

Un Bundle donde `Observation.subject` referencia una entrada que resulta ser un `Device` en lugar del `Patient` esperado:

```json
{
  "resourceType": "Bundle",
  "type": "collection",
  "entry": [
    {
      "resource": {
        "resourceType": "Device",
        "id": "123"
      }
    },
    {
      "resource": {
        "resourceType": "Observation",
        "status": "final",
        "code": {
          "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
        },
        "subject": {
          "reference": "Device/123"
        }
      }
    }
  ]
}
```

Si el ElementDefinition para `Observation.subject` solo permite `Patient` y `Group`, esta referencia a `Device` produce un error.

{{< callout type="info" >}}
Los tipos de destino permitidos se leen de las entradas `ElementDefinition.type` donde `code` es `Reference`. Los valores `targetProfile` dentro de esas entradas de tipo especifican a qué tipos de recurso puede apuntar la referencia. El validador deriva estas restricciones completamente del StructureDefinition.
{{< /callout >}}
