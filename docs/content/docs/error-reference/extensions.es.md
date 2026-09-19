---
title: "Errores de Extensions"
linkTitle: "Extensions"
description: "Errores relacionados con extensions desconocidas, inválidas y modifier extensions."
weight: 5
---

Los errores de extension ocurren cuando las extensions FHIR no se ajustan a sus StructureDefinitions declarados o violan las reglas de extensión definidas en la especificación FHIR. Las extensions son un mecanismo central de extensibilidad en FHIR, y el validador verifica sus URLs, contextos, valores y cardinalidad. Las modifier extensions reciben un tratamiento más estricto porque pueden cambiar el significado del elemento que extienden.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `EXTENSION_UNKNOWN` | warning | Unknown extension '{url}' |
| `EXTENSION_INVALID_CONTEXT` | error | Extension '{url}' not allowed in context '{path}' |
| `EXTENSION_MISSING_URL` | error | Extension at '{path}' has no url |
| `EXTENSION_NO_VALUE` | error | Extension at '{path}' has no value[x] |
| `EXTENSION_MULTIPLE_VALUES` | error | Extension at '{path}' has multiple value[x] elements |
| `EXTENSION_WRONG_TYPE` | error | Extension '{url}' expects {expected}, got {type} |
| `EXTENSION_INVALID_URL` | error | Extension URL must be an absolute URL ({reason}): '{url}' |
| `MODIFIER_EXTENSION_UNKNOWN` | error | Unknown modifier extension '{url}' |

---

## EXTENSION_UNKNOWN

Se encontró una extension cuya URL no coincide con ningún StructureDefinition cargado. El validador no puede verificar la estructura o contenido de la extension. Esto es un warning porque la extension puede ser válida pero simplemente no estar cargada en el validador.

**Ejemplo:**

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/custom-ext",
      "valueString": "some value"
    }
  ]
}
```

Si el StructureDefinition para `http://example.org/fhir/StructureDefinition/custom-ext` no está cargado, el validador produce este warning.

**Corrección:** Carga la Implementation Guide o el StructureDefinition que define esta extension:

```bash
gofhir-validator -ig http://example.org/fhir/ImplementationGuide/example patient.json
```

---

## EXTENSION_INVALID_CONTEXT

La extension está definida con una restricción de contexto que no incluye el elemento donde fue utilizada. Cada StructureDefinition de extension declara dónde está permitida a través de `StructureDefinition.context`.

**Ejemplo:**

Una extension definida con contexto `Patient` utilizada en un Observation:

```json
{
  "resourceType": "Observation",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/patient-only-ext",
      "valueString": "not allowed here"
    }
  ]
}
```

**Corrección:** Usa la extension solo en los contextos declarados en su StructureDefinition, o actualiza la definición de la extension para incluir el contexto deseado.

---

## EXTENSION_MISSING_URL

Se encontró un elemento extension que no contiene una propiedad `url`. Toda extension en FHIR debe tener una URL que identifique su definición.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "valueString": "missing url"
    }
  ]
}
```

**Corrección:** Agrega la propiedad `url`:

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/my-ext",
      "valueString": "has url now"
    }
  ]
}
```

---

## EXTENSION_NO_VALUE

Una extension simple (una sin sub-extensions anidadas) no tiene un elemento `value[x]`. Las extensions simples deben contener un valor a menos que sean extensions complejas con sub-extensions.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/simple-ext"
    }
  ]
}
```

**Corrección:** Agrega el elemento de valor apropiado:

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/simple-ext",
      "valueString": "the value"
    }
  ]
}
```

{{< callout type="info" >}}
Las extensions complejas (aquellas con elementos `extension` anidados) no tienen un `value[x]` -- llevan sus datos en sub-extensions. Este error solo aplica a extensions simples.
{{< /callout >}}

---

## EXTENSION_MULTIPLE_VALUES

Una extension contiene más de un elemento `value[x]`. Cada extension puede tener como máximo un valor.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/my-ext",
      "valueString": "first",
      "valueInteger": 42
    }
  ]
}
```

**Corrección:** Usa solo un elemento `value[x]` por extension:

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/my-ext",
      "valueString": "first"
    }
  ]
}
```

---

## EXTENSION_WRONG_TYPE

El tipo de valor de la extension no coincide con el tipo declarado en su StructureDefinition. Por ejemplo, la definición de la extension especifica `valueCodeableConcept` pero el recurso proporciona `valueString`.

**Ejemplo:**

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://hl7.org/fhir/StructureDefinition/patient-religion",
      "valueString": "Christian"
    }
  ]
}
```

Si la definición de la extension declara el tipo de valor como `CodeableConcept`, proporcionar un `valueString` no es válido.

**Corrección:** Usa el tipo de valor correcto como se declara en el StructureDefinition:

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://hl7.org/fhir/StructureDefinition/patient-religion",
      "valueCodeableConcept": {
        "coding": [
          {
            "system": "http://terminology.hl7.org/CodeSystem/v3-ReligiousAffiliation",
            "code": "1013",
            "display": "Christian"
          }
        ]
      }
    }
  ]
}
```

---

## EXTENSION_INVALID_URL

La URL de la extension no es una URL absoluta. Según FHIR R4 §2.5.0.1: *"The url SHALL be a URL, not a URN (e.g. not an OID or a UUID), and it SHALL be the canonical URL of a StructureDefinition that defines the extension."* Se rechazan tanto las referencias relativas como las URN.

El requisito es una **URL** absoluta, no una URI absoluta a secas: `urn:uuid:…` y `ex:createdAt` son URIs absolutas válidas según RFC 3986 y ninguna se acepta acá, porque la especificación nombra a las URN como el caso a excluir.

Las extensions hijas dentro de una extension compleja son la excepción documentada (*"Except for child extensions defined within complex extensions, the URL SHALL be an absolute URL"*): se resuelven por nombre contra la definición del padre, así que nunca llegan a esta comprobación.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "my-custom-extension",
      "valueString": "bad"
    }
  ]
}
```

La URL `my-custom-extension` es una referencia relativa, no una URL absoluta. `urn:oid:1.2.3.4.5` también se rechazaría, por ser una URN.

**Corrección:** Usa la URL absoluta completa:

```json
{
  "resourceType": "Patient",
  "extension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/my-custom-extension",
      "valueString": "good"
    }
  ]
}
```

{{< callout type="info" >}}
Esta validación aplica una regla en prosa de la especificación FHIR (§2.5.0.1). El elemento `Extension.url` en el StructureDefinition está tipado como `System.String` sin constraint de regex, por lo que esta verificación no puede derivarse solo del SD.

Una extension cuya URL está bien formada pero cuya definición no se puede resolver es un caso *distinto*: ese solo transgrede un `SHOULD` y se reporta como warning, no como error. Ver `docs/VALIDATION-GAPS.md`.
{{< /callout >}}

---

## MODIFIER_EXTENSION_UNKNOWN

Se encontró una modifier extension cuya URL no coincide con ningún StructureDefinition cargado. A diferencia de las extensions regulares desconocidas (que producen warnings), las modifier extensions desconocidas producen **errores** porque las modifier extensions pueden cambiar el significado del elemento contenedor. Procesar un recurso con una modifier extension no reconocida es inseguro.

**Ejemplo:**

```json
{
  "resourceType": "Patient",
  "modifierExtension": [
    {
      "url": "http://example.org/fhir/StructureDefinition/unknown-modifier",
      "valueBoolean": true
    }
  ]
}
```

**Corrección:** Carga el StructureDefinition que define esta modifier extension, o elimina la modifier extension si no es necesaria.

{{< callout type="warning" >}}
Las modifier extensions se tratan más estrictamente que las extensions regulares porque pueden alterar el significado del recurso o elemento. Una modifier extension desconocida siempre produce un error, mientras que una extension regular desconocida produce solo un warning.
{{< /callout >}}
