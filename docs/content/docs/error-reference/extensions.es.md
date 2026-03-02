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
