---
title: "Errores de Terminología"
linkTitle: "Terminología"
description: "Errores relacionados con la validación de codificación, cumplimiento de binding y membresía en ValueSets."
weight: 4
---

Los errores de terminología ocurren cuando los valores codificados no se ajustan a los bindings de terminología declarados en el StructureDefinition. El validador verifica que los códigos, sistemas y displays sean válidos de acuerdo con los ValueSets y CodeSystems referenciados. La severidad de los errores de terminología depende de la fuerza del binding.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `CODING_NO_CODE` | error | Coding at '{path}' has no code |
| `CODING_NO_SYSTEM` | warning | Coding at '{path}' has no system |
| `CODING_INVALID_SYSTEM` | error | System '{value}' is not a valid URI |
| `BINDING_REQUIRED_MISSING` | error | Value '{value}' is not in required ValueSet '{valueSet}' |
| `BINDING_EXTENSIBLE_MISSING` | warning | Value '{value}' is not in extensible ValueSet '{valueSet}' |
| `BINDING_UNKNOWN_SYSTEM` | error | Unknown code system for code '{value}' |
| `BINDING_INVALID_CODE` | error | Code '{value}' is not valid in system '{system}' |
| `BINDING_VALUESET_NOT_FOUND` | warning | ValueSet '{valueSet}' could not be resolved |

---

## CODING_NO_CODE

Se encontró un elemento Coding que tiene un `system` pero no tiene `code`. Un Coding solo es significativo cuando contiene un valor de código.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://loinc.org"
      }
    ]
  }
}
```

**Corrección:** Agrega la propiedad `code`:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9"
      }
    ]
  }
}
```

---

## CODING_NO_SYSTEM

Un elemento Coding tiene un `code` pero no tiene `system`. Aunque no siempre es un error (de ahí la severidad warning), un código sin sistema es ambiguo porque la misma cadena de código puede significar cosas diferentes en distintos sistemas de códigos.

**Ejemplo:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "code": "85354-9"
      }
    ]
  }
}
```

**Corrección:** Agrega la propiedad `system`:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9"
      }
    ]
  }
}
```

---

## CODING_INVALID_SYSTEM

El valor `system` en un Coding no es un URI válido. Los identificadores de sistemas de códigos en FHIR deben ser URIs válidos.

**Ejemplo:**

```json
{
  "coding": [
    {
      "system": "not a valid uri",
      "code": "12345"
    }
  ]
}
```

---

## BINDING_REQUIRED_MISSING

El valor codificado no es miembro del ValueSet especificado en un binding **required**. Los bindings required son los más estrictos -- el código debe estar en el ValueSet.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "gender": "m"
}
```

`Patient.gender` tiene un binding required a `http://hl7.org/fhir/ValueSet/administrative-gender`, que solo acepta `male`, `female`, `other` y `unknown`. El valor `m` no está en ese ValueSet.

**Corrección:**

```json
{
  "resourceType": "Patient",
  "gender": "male"
}
```

---

## BINDING_EXTENSIBLE_MISSING

El valor codificado no es miembro del ValueSet especificado en un binding **extensible**. Esto produce un warning en lugar de un error, porque los bindings extensible permiten códigos de otros sistemas cuando no existe un código apropiado en el ValueSet vinculado.

**Ejemplo:**

```json
{
  "resourceType": "Condition",
  "code": {
    "coding": [
      {
        "system": "http://example.org/custom-codes",
        "code": "custom-condition"
      }
    ]
  }
}
```

Si el perfil tiene un binding extensible a un ValueSet estándar y el código personalizado no es miembro, el validador produce un warning.

---

## BINDING_UNKNOWN_SYSTEM

El sistema de códigos referenciado en un Coding no se pudo encontrar en los recursos de terminología cargados. El validador no puede determinar si el código es válido sin acceso al CodeSystem.

---

## BINDING_INVALID_CODE

El código se encontró en un CodeSystem conocido, pero no es un código válido dentro de ese sistema. Esto indica que el sistema de códigos fue cargado, pero el código específico no existe.

**Ejemplo:**

```json
{
  "resourceType": "Patient",
  "gender": "m"
}
```

El sistema de códigos `http://hl7.org/fhir/administrative-gender` es conocido y está cargado, pero `m` no es un código válido en él.

---

## BINDING_VALUESET_NOT_FOUND

El ValueSet referenciado por un binding no se pudo resolver. El validador no puede validar el código contra el binding porque el ValueSet no está disponible.

Esto es un warning porque la imposibilidad de resolver un ValueSet es un problema del entorno (recursos de terminología faltantes) en lugar de un problema con el recurso validado en sí.

**Corrección:** Asegúrate de que los recursos ValueSet requeridos estén cargados en el validador. Al usar el CLI, carga la Implementation Guide apropiada:

```bash
gofhir-validator -ig hl7.fhir.us.core patient.json
```

O programáticamente:

```go
v, err := validator.New(
    validator.WithValueSets(myValueSets...),
)
```

## Fuerza de Binding y Severidad

La severidad de los errores de terminología depende de la fuerza de binding declarada en el ElementDefinition:

| Fuerza de Binding | Código No en ValueSet | Severidad del Issue |
|-------------------|----------------------|---------------------|
| **required** | Debe estar en el ValueSet | error |
| **extensible** | Debería estar en el ValueSet | warning |
| **preferred** | Recomendado | information |
| **example** | Sin validación | _(ninguna)_ |

{{< callout type="info" >}}
La fuerza de binding se lee desde `ElementDefinition.binding.strength` en el StructureDefinition. El validador nunca hardcodea qué elementos tienen qué fuerza de binding -- siempre se deriva del perfil.
{{< /callout >}}

Cuando la validación de terminología está deshabilitada (usando `-tx n/a` en el CLI o `WithTerminologyDisabled()` en la API), todos los errores relacionados con terminología se suprimen.
