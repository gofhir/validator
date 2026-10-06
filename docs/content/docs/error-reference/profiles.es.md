---
title: "Errores de Perfiles"
linkTitle: "Perfiles"
description: "Errores relacionados con la resolución de perfiles, validez y discrepancias de tipos."
weight: 10
---

Los errores de perfiles ocurren cuando el validador no puede cargar, parsear o aplicar un StructureDefinition (perfil) a un recurso. Los perfiles son la base de la validación FHIR -- definen cómo luce un recurso válido. Si un perfil no puede resolverse o no coincide con el tipo de recurso, la validación no puede proceder correctamente.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `PROFILE_NOT_FOUND` | warning | Profile '{url}' was not checked: {reason} |
| `PROFILE_INVALID` | error | Profile '{profile}' is not a valid StructureDefinition |
| `PROFILE_WRONG_TYPE` | error | Resource type '{type}' does not match profile type '{expected}' |

---

## PROFILE_NOT_FOUND

Un perfil que el recurso declara (`meta.profile`) o con el que se pidió validarlo no se pudo resolver: ningún StructureDefinition cargado tiene su URL, o ninguno tiene la versión que fija (`url|version`). Una versión fijada se usa exacta, nunca otra versión de la misma URL (references.html#canonical). El recurso no se verifica contra ese perfil; si no se resuelve ninguno de los perfiles que declara, se verifica contra la definición de su tipo, como lo hace el validador de HL7.

Se reporta como warning en la entrada de `meta.profile` que lo declara (`Patient.meta.profile[0]`, `Bundle.entry[0].resource.meta.profile[0]`), o en el recurso cuando se pidió por opción, como el `VALIDATION_VAL_PROFILE_UNKNOWN_ERROR` de HL7. Vale para el recurso validado y para los recursos que contiene (entradas de Bundle, `contained`).

**Causas comunes:**

- La Implementation Guide que contiene el perfil no está cargada
- La URL del perfil tiene un error tipográfico
- La URL del perfil incluye un fragmento de versión que no coincide con la versión cargada
- El perfil no ha sido publicado o no está disponible en los registros configurados

**Ejemplo -- CLI:**

```bash
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

Si la Implementation Guide de US Core no está cargada, el validador no puede encontrar el perfil.

Salida de validación:

```text
WARNING: Profile 'http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient' was not checked: no definition with its url is loaded
  MessageID: PROFILE_NOT_FOUND
```

**Corrección:** Carga la Implementation Guide que contiene el perfil:

```bash
gofhir-validator -ig hl7.fhir.us.core -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

O programáticamente, asegúrate de que el perfil esté registrado:

```go
v, err := validator.New(
    validator.WithProfiles(usCorePatientProfile),
)
```

---

## PROFILE_INVALID

El StructureDefinition se encontró pero no es un perfil válido o utilizable. Esto puede ocurrir cuando:

- El JSON del StructureDefinition está malformado
- Faltan campos requeridos (ej., no tiene `snapshot` ni `differential`)
- El `kind` no es `resource`, `complex-type` ni `logical`
- El StructureDefinition referencia una definición base que no puede resolverse
- El snapshot no se pudo generar desde el differential

**Ejemplo de salida:**

```text
ERROR: Profile 'http://example.org/fhir/StructureDefinition/broken-profile' is not a valid StructureDefinition
  MessageID: PROFILE_INVALID
```

**Corrección:** Verifica que el StructureDefinition sea completo y esté bien formado. Usa el propio validador FHIR para validar el recurso StructureDefinition:

```bash
gofhir-validator profile.json
```

Asegúrate de que el perfil tenga un elemento `snapshot` o un elemento `differential` con un `baseDefinition` resoluble.

---

## PROFILE_WRONG_TYPE

El `resourceType` del recurso no coincide con el tipo declarado en el campo `StructureDefinition.type` del perfil. Por ejemplo, intentar validar un recurso Patient contra un perfil de Observation.

**Ejemplo -- uso inválido:**

```bash
gofhir-validator -ig http://hl7.org/fhir/StructureDefinition/bp observation.json
```

Donde `observation.json` contiene:

```json
{
  "resourceType": "Patient",
  "name": [{ "family": "Smith" }]
}
```

Y el perfil de presión arterial declara `"type": "Observation"`.

Salida de validación:

```text
ERROR: Resource type 'Patient' does not match profile type 'Observation'
  Path: Patient
  MessageID: PROFILE_WRONG_TYPE
```

**Corrección:** Valida el recurso contra un perfil que coincida con su tipo de recurso:

```bash
gofhir-validator -ig http://hl7.org/fhir/StructureDefinition/Patient patient.json
```

### Perfil Declarado en meta.profile

Los recursos pueden declarar a qué perfiles afirman conformar a través de `meta.profile`. El validador usa estas declaraciones para seleccionar contra qué perfiles validar:

```json
{
  "resourceType": "Patient",
  "meta": {
    "profile": [
      "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"
    ]
  },
  "name": [{ "family": "Smith" }]
}
```

Si un perfil listado en `meta.profile` no se puede encontrar, el validador reporta un warning `PROFILE_NOT_FOUND` en esa entrada, y verifica el recurso contra la definición de su tipo. Un recurso contenido en otro (una entrada de Bundle, `contained`) se verifica de la misma forma contra los perfiles que declara.

## Cadena de Resolución de Perfiles

El validador resuelve perfiles siguiendo la cadena de derivación:

```text
Perfil Personalizado
  -> baseDefinition: US Core Patient
    -> baseDefinition: Patient
      -> baseDefinition: DomainResource
        -> baseDefinition: Resource
```

En cada nivel, el validador combina las restricciones del perfil con su base. Si algún perfil en la cadena no puede resolverse, el perfil queda sin snapshot, y el validador reporta `PROFILE_SNAPSHOT_FAILED` nombrando el eslabón faltante.

{{< callout type="info" >}}
Todas las reglas de validación provienen de los StructureDefinitions. El validador carga el perfil, resuelve la cadena completa de derivación, genera un snapshot si es necesario, y luego valida el recurso contra las restricciones combinadas. No se hardcodea lógica específica de tipos de recurso.
{{< /callout >}}
