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
| `REFERENCE_INVALID_TARGET` | error | Invalid reference target type '{type}'. Allowed: {allowed} |
| `REFERENCE_TARGET_PROFILE` | error | Unable to find a profile match for {reference} among choices: {profiles} |
| `REFERENCE_MULTIPLE_MATCHES` | error | Multiple matches in bundle for reference {reference} |
| `REFERENCE_NOT_FOUND` | warning | Referenced resource '{value}' not found |
| `REFERENCE_TYPE_MISMATCH` | error | Reference type element '{type}' does not match reference target '{reference}' |

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

La referencia apunta a un tipo de recurso que no está permitido por el ElementDefinition. Cada elemento de referencia en un StructureDefinition lista los tipos de destino permitidos a través de `ElementDefinition.type.targetProfile`. El tipo del destino es el del recurso al que se resuelve la referencia (ver `REFERENCE_TARGET_PROFILE`), diga lo que diga su literal: `Patient/p` que se resuelve a una entrada que contiene un `Group` es un `Group`. Una referencia que no se resuelve tiene el tipo que nombra su literal (`[type]/[id]`).

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

Si el perfil restringe `Observation.subject` a referenciar solo `Patient` o `Group`, una referencia a `Organization` no es un destino válido. El issue se reporta en el Reference (`Observation.subject`), como lo reporta el validador de HL7.

Un `targetProfile` puede fijar una versión (`http://example.org/StructureDefinition/my-patient|1.0.0`); su tipo es el de esa versión del perfil. El tipo de una referencia literal se verifica se resuelva o no el destino: el destino "must conform to at least one" de los perfiles (ElementDefinition.type.targetProfile), y un recurso de otro tipo no puede. El validador de HL7 6.10.4 verifica el tipo solo de un destino que resuelve (decisión B-D13 del plan B).

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

## REFERENCE_TARGET_PROFILE

El recurso al que se resuelve una referencia no conforma con ninguno de los perfiles de su tipo que permite su elemento. `ElementDefinition.type.targetProfile` dice "the content must conform to at least one of them".

El destino se verifica solo cuando se resuelve, como dice bundle.html#references: `#id`, un recurso contenido del recurso que hace la referencia (o del recurso que lo contiene), y `#`, desde un recurso contenido, el recurso que lo contiene; una referencia absoluta, la entrada cuyo `fullUrl` es; una relativa (`Patient/p`), la entrada cuyo `fullUrl` es la base de la entrada que referencia seguida de ella, lo que exige que el `fullUrl` de esa entrada sea RESTful (no un `urn:uuid:`). Se busca solo en el Bundle cuya entrada es el recurso que referencia: la especificación no da significado a una referencia en un Bundle que contenga a ese. Una versión (`/_history/1`) se quita antes de comparar el `fullUrl`, y luego se compara con el `meta.versionId` de la entrada. Una referencia que coincide con varias entradas es ambigua y no se resuelve (`REFERENCE_MULTIPLE_MATCHES`). Un destino que no se resuelve no se verifica.

El destino se valida contra cada perfil de su tipo (`derivation: constraint`) con la validación completa, y conforma cuando uno de ellos no encuentra errores. Cuando el elemento permite la definición del tipo del destino, o de un tipo del que deriva (`Resource`, `DomainResource`), cualquier destino de ese tipo conforma: su propia validación reporta lo que tenga mal. Los recursos que se referencian entre sí (`Patient.link`, `Observation.hasMember`) conforman cuando no tienen ningún otro problema. El issue se reporta en el Reference, como lo reporta el validador de HL7 (`Reference_REF_CantMatchChoice`).

**Ejemplo:** el `Observation.subject` de un perfil permite `Reference(https://example.org/fhir/StructureDefinition/my-patient)`, que exige un `identifier`. Una entrada de un Bundle es una Observation cuyo subject es `Patient/p`, y el `Patient/p` del Bundle no tiene identifier:

```text
ERROR: Unable to find a profile match for Patient/p among choices: https://example.org/fhir/StructureDefinition/my-patient
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_TARGET_PROFILE
```

**Corrección:** hacer que el destino conforme con uno de los perfiles, o referenciar un recurso que lo haga.

---

## REFERENCE_MULTIPLE_MATCHES

Varias entradas del Bundle coinciden con una referencia, según las compara bundle.html#references: el mismo `fullUrl`, y el mismo `meta.versionId` cuando la referencia nombra una versión. La especificación dice que "es ambiguo cuál es la correcta" y que las aplicaciones "PUEDEN devolver un error"; gofhir lo devuelve, como el validador de HL7 (`Bundle_BUNDLE_MultipleMatches`). La referencia no se resuelve: no se verifica tipo ni perfil de un destino para ella.

**Ejemplo:** un Bundle `history` contiene dos versiones de `Patient/p` (el mismo `fullUrl`, `meta.versionId` 1 y 2), y el subject de una Observation es `Patient/p`:

```text
ERROR: Multiple matches in bundle for reference Patient/p
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_MULTIPLE_MATCHES
```

**Corrección:** referenciar la versión que corresponde (`Patient/p/_history/2`), o dar a cada entrada su propio `fullUrl`.

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

El elemento `type` de la referencia nombra otro tipo que el de su destino: `Reference.type` y la referencia "SHALL be consistent" (Reference.type). El tipo del destino es el del recurso al que se resuelve la referencia, también el de una referencia `urn:uuid:` o `#id`, o si no el tipo que nombra su literal (`[type]/[id]`).

**Ejemplo:** el subject de una Observation es `{"reference": "urn:uuid:…", "type": "Group"}`, y la entrada del Bundle con ese `fullUrl` es un Patient:

```text
ERROR: Reference type element 'Group' does not match reference target 'Patient'
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_TYPE_MISMATCH
```

**Corrección:** hacer que `type` sea el tipo del destino, o referenciar un recurso de ese tipo.
