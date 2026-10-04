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
| `EXTENSION_INVALID_CONTEXT` | error | Extension '{url}' is not allowed in context '{context}' |
| `EXTENSION_CONTEXT_INVARIANT` | error | Extension '{url}' is not allowed here: its context invariant '{expression}' does not hold |
| `EXTENSION_MISSING_URL` | error | Extension at '{path}' has no url |
| `EXTENSION_NO_VALUE` | error | Extension at '{path}' has no value[x] |
| `EXTENSION_MULTIPLE_VALUES` | error | Extension at '{path}' has multiple value[x] elements |
| `EXTENSION_WRONG_TYPE` | error | Extension '{url}' expects {expected}, got {type} |
| `EXTENSION_INVALID_URL` | error | Extension URL must be an absolute URL ({reason}): '{url}' |
| `EXTENSION_SUBEXTENSION_INVALID` | error | Sub-extension url '{url}' is not defined by the extension '{parent}' |
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

La extension se usa en un target que su definición no permite (`StructureDefinition.context`; "Extensions SHALL only be used on a target that appears in their context list", defining-extensions.html). El target es el elemento que contiene la extension, y el issue se reporta ahí. Qué es el target sale de las definiciones, no de su path JSON:

- un contexto `element` nombra un id de elemento. El target coincide con el id del elemento que instancia y del elemento en el que ese se basa (`Patient.text` se basa en `DomainResource.text`), con el elemento al que apunta un contentReference, con la raíz de su tipo y de cada ancestro del tipo (`HumanName`, `Element`; `Patient`, `DomainResource`, `Resource`), y con su path desde el recurso a través de los tipos de datos en que está (`Patient.name.given`). Un contexto no cubre los elementos bajo su target: `Patient` permite la raíz del recurso, no `Patient.name`. `Element` no nombra la raíz de un recurso, que no es un Element. Un recurso también se nombra por las interfaces que implementa su tipo: las que declara su definición (R5: `CanonicalResource`, `MetadataResource`; el validador de HL7 6.10.4 no reconoce `MetadataResource` en el ValueSet de R5, que la declara); en R4 y R4B, que no declaran ninguna, `CanonicalResource` nombra los recursos que references.html lista como canónicos, y `MetadataResource`, un modelo lógico del que no deriva ningún recurso, no nombra ninguno, como en el validador de HL7;
- un contexto `extension` nombra la extension que lo contiene: su url, sea cual sea la versión que fije el contexto, o `url#code` para una sub-extension de una extension compleja;
- un contexto `fhirpath` selecciona el target: la expresión se evalúa desde la raíz del recurso en que está el target (la raíz propia de un recurso contenido, con su contenedor como `%rootResource`), y el target debe ser uno de los nodos que devuelve. El mismo nodo, no uno igual: `Patient.name.where(use = 'official')` permite `Patient.name[0]` cuando ese nombre es oficial, y no `Patient.contact[0].name`, por parecido que sea. Una expresión que no devuelve elementos, como un booleano, no permite ningún target;
- una definición sin contexto no permite ningún target.

Los nombres de contexto que lista el issue son los del target.

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

## EXTENSION_CONTEXT_INVARIANT

La extension se usa en un target que su contexto permite, pero uno de los context invariants de su definición (`StructureDefinition.contextInvariant`) no se cumple. Un context invariant es "a rule that is executed on the element that contains the extension when it is present" (defining-extensions.html): se evalúa sobre el target, como un invariante de perfil: en un primitivo, con su id y sus extensions; con `%rootResource` el contenedor de un recurso contenido, y con `resolve()` encontrando una referencia de fragmento (`#id`) entre los recursos que contiene el recurso del target, y cualquier otra referencia entre las entradas del Bundle más interno que lo contiene, y luego de los Bundles que contienen a ese. Su resultado se lee como booleano igual que en el validador de HL7: un booleano único vale su valor, un resultado vacío es falso y cualquier otro resultado es verdadero. El issue se reporta en el target, una sola vez, por el primer invariante que no se cumple.

**Ejemplo:** una extension para `Patient` con el context invariant `gender.exists()`, en un Patient sin gender.

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

Las extensions hijas dentro de una extension compleja son la excepción documentada (*"Except for child extensions defined within complex extensions, the URL SHALL be an absolute URL"*): una url relativa se resuelve por nombre contra la definición del padre, y una que el padre no declara es `EXTENSION_SUBEXTENSION_INVALID`.

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

## EXTENSION_SUBEXTENSION_INVALID

Una extensión compleja contiene una parte cuya `url` relativa su definición no declara. Las partes de una extensión compleja son *"locales/relativas a la referencia a la definición de la extensión"* (extensibility.html): una `url` relativa solo puede nombrar una parte que la definición declara, así que una que no declara no tiene significado. El validador de HL7 reporta el mismo error (`Extension_EXT_SubExtension_Invalid`).

Una parte con `url` absoluta es una extensión definida por separado, que una extensión compleja también puede contener: se valida contra su propia definición, como cualquier extensión.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "extension": [{
    "url": "http://hl7.org/fhir/StructureDefinition/patient-nationality",
    "extension": [{"url": "nope", "valueString": "x"}]
  }]
}
```

`patient-nationality` declara las partes `code` y `period`; `nope` no es ninguna de ellas.

**Solución:** Usa una parte que la definición declare, o una extensión definida por separado, con su URL absoluta.

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
