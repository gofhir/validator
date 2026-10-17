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
| `CODING_NO_SYSTEM` | warning | Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided |
| `CODING_NO_CODE_IN_CONCEPT` | warning | No code is provided for the code system '{system}' |
| `CODE_NOT_IN_CODESYSTEM` | error | The code '{code}' is not valid in the CodeSystem '{system}' |
| `CODESYSTEM_NOT_FOUND` | ver abajo | CodeSystem is unknown and can't be validated: {system} for '{systemCode}' |
| `CODESYSTEM_URI_INVALID` | error | The code system URI '{system}' is not valid: {reason} |
| `CODESYSTEM_NO_CODES` | ver abajo | The CodeSystem '{codeSystem}' includes none of its codes (content '{content}'), so the code '{code}' cannot be validated |
| `CODESYSTEM_EXAMPLE_CONTENT` | warning | The CodeSystem '{codeSystem}' has example content only (content '{content}'), so the code '{code}' cannot be validated |
| `CODE_NOT_IN_PARTIAL_CODESYSTEM` | ver abajo | The code '{code}' is not among the codes the CodeSystem '{codeSystem}' includes, which are only some of its codes (content '{content}'), so it cannot be validated |
| `BINDING_CANNOT_VALIDATE` | ver abajo | Code '{code}' in system '{system}' cannot be validated - external terminology system requires a terminology server |
| `BINDING_REQUIRED` | error | The value provided ('{code}') is not in the value set '{valueSet}' (required){detail} |
| `BINDING_REQUIRED_NO_CODING` | error | None of the codings provided are in the value set '{valueSet}', and a coding from this value set is required (codes = {codes}) |
| `BINDING_REQUIRED_NO_CODE` | error | No code provided, and a code is required from the value set '{valueSet}' |
| `BINDING_EXTENSIBLE` | warning | The value provided ('{code}') is not in the value set '{valueSet}' (extensible){detail} |
| `BINDING_EXTENSIBLE_NO_CODING` | warning | None of the codings provided are in the value set '{valueSet}', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = {codes}) |
| `BINDING_EXTENSIBLE_OTHER_SYSTEM` | warning (information para un sistema que HL7 revisa solo con servidor) | The value provided ('{code}') is not in the value set '{valueSet}', and comes from a system the value set does not declare (an extensible binding permits it only where the value set has no suitable code){detail} |
| `BINDING_EXTENSIBLE_UNKNOWN_SYSTEM` | warning | The value provided ('{code}') is not in the value set '{valueSet}' (extensible); whether its system belongs to the value set could not be determined{detail} |
| `BINDING_TEXT_ONLY_WARNING` | warning | No code provided, and a code should be provided from the value set '{valueSet}' (extensible) |
| `BINDING_DISPLAY_MISMATCH` | error | Display '{provided}' for code '{code}' does not match expected '{expected}' |
| `BINDING_UNRESOLVED` | information (según la política para lo no resuelto: `warn`, por defecto, information; `error`, error) | The value provided ('{code}') could not be checked against value set '{valueSet}': no terminology source could resolve it{detail} |
| `BINDING_VALUESET_NOT_FOUND` | warning | ValueSet '{valueSet}' not found |

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

## Códigos que no se pueden revisar

Un código cuyo code system no se puede revisar aquí (`CODESYSTEM_NOT_FOUND`: ningún CodeSystem con su url; `CODESYSTEM_NO_CODES`: uno con content `not-present`; `CODESYSTEM_EXAMPLE_CONTENT`: uno con content `example`, que no puede decir que un código es correcto aunque lo liste; `CODE_NOT_IN_PARTIAL_CODESYSTEM`: un código que uno con content `fragment` no lista; `BINDING_CANNOT_VALIDATE`: un sistema que solo un servidor de terminología revisa) se reporta con la severidad que le da el validador de HL7. La especificación no fija ninguna, y admite sistemas definidos localmente para los tags (resource.html#tags). La severidad sale de las definiciones, nunca del nombre del elemento:

| Dónde está el Coding | Severidad |
|----------------------|-----------|
| Coding de un CodeableConcept (`Observation.code`) | warning, `CODESYSTEM_NOT_FOUND` en `.system`, también un CodeSystem `not-present` o `example` |
| Coding que revisa un binding (no example, con su ValueSet resuelto: `meta.security`, extensible) | warning |
| cualquiera de los dos, bajo un binding required (para el coding de un CodeableConcept, el del concepto), con el sistema en los espacios de nombres de HL7 (`http://hl7.org/fhir`, `http://terminology.hl7.org`); para un Coding, no un CodeSystem `not-present` ni `example`, que queda en warning | error |
| Coding que ningún binding revisa (`meta.tag`, el `valueCoding` de una extensión) | information; un warning para un CodeSystem `example` |

Los sistemas que el validador de HL7 trata aparte se tratan como él los trata:

| Sistema | Coding de un CodeableConcept | Coding que revisa un binding | Coding que ningún binding revisa |
|---------|------------------------------|------------------------------|----------------------------------|
| LOINC, SNOMED CT, RxNorm, CPT | nada | information (`BINDING_CANNOT_VALIDATE`) | warning |
| UCUM | warning (`CODESYSTEM_NOT_FOUND`) | warning (`BINDING_CANNOT_VALIDATE`) | warning |
| NDC, CVX, ICD-9-CM, ICD-10, ICD-10-CM | nada | information | nada |
| Los otros SIDs conocidos de HL7 (`http://hl7.org/fhir/sid/srt`, `icd-9`, `icpc-2`, ...) | warning (un error bajo un binding required) | warning (un error bajo un binding required) | nada |
| NDF-RT | nada | information | warning |
| bcp47, ISO 3166 | warning | warning (`BINDING_CANNOT_VALIDATE`) | information |

Un código que un CodeSystem `fragment` no tiene es un warning, en `.code`.

En un Coding que ningún binding revisa, un sistema que ningún CodeSystem tiene se revisa primero como URI, como lo hace HL7: un error (`CODESYSTEM_URI_INVALID`) para una URL de build.fhir.org, un sistema en el espacio de nombres de HL7 (`http://hl7.org/fhir`, con https o www) que HL7 no publica (un sistema bajo `http://hl7.org/fhir` que un NamingSystem de kind `codesystem` del paquete de terminología de HL7 tiene como su unique id preferido está publicado: desconocido, un warning, salvo en el coding de un CodeableConcept bajo un binding required, un error), una página web (`.html`), una variante de la URL de SNOMED CT, LOINC, UCUM o RxNorm, o un sistema conocido escrito con https. La URL de un value set como sistema es un error en todo Coding, en su `.system` donde un binding lo revisa (la especificación: el sistema SHALL NOT contener una referencia a un value set), salvo en el coding de un CodeableConcept que ningún binding propio revisa, donde HL7 no lo reporta. El sistema de una Quantity (una Quantity misma, no un tipo derivado, como lo revisa HL7) se revisa como el de un Coding, salvo UCUM, que valida la revisión de UCUM; un Coding o una Quantity que un binding revisa, donde ninguno revisa su elemento base (el binding de un perfil o de una extensión sobre un elemento que su base deja sin binding o con uno example), se revisa además como uno que ninguno revisa, como lo hace HL7. Un Coding con sistema y sin código está permitido (`Coding.code` 0..1); el de un CodeableConcept, con un sistema conocido, da un warning (`CODING_NO_CODE_IN_CONCEPT`); bajo un binding required un Coding no aporta código (`BINDING_REQUIRED_NO_CODE`); una Quantity sin código no tiene unidad codificada, que su binding no pide. Bajo un binding extensible, un CodeableConcept cuyos codings nombran sistemas, sin código, no es del value set (`BINDING_EXTENSIBLE_NO_CODING`).

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

Cuando la validación de terminología está deshabilitada (usando `-no-terminology` en el CLI o `WithNoTerminology()` en la API), todos los errores relacionados con terminología se suprimen.
