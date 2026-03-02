---
title: "Extension Errors"
linkTitle: "Extensions"
description: "Errors related to unknown, invalid, and modifier extensions."
weight: 5
---

Extension errors occur when FHIR extensions do not conform to their declared StructureDefinitions or violate the extension rules defined in the FHIR specification. Extensions are a core extensibility mechanism in FHIR, and the validator checks their URLs, contexts, values, and cardinality. Modifier extensions receive stricter treatment because they can change the meaning of the element they extend.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `EXTENSION_UNKNOWN` | warning | Unknown extension '{url}' |
| `EXTENSION_INVALID_CONTEXT` | error | Extension '{url}' not allowed in context '{path}' |
| `EXTENSION_MISSING_URL` | error | Extension at '{path}' has no url |
| `EXTENSION_NO_VALUE` | error | Extension at '{path}' has no value[x] |
| `EXTENSION_MULTIPLE_VALUES` | error | Extension at '{path}' has multiple value[x] elements |
| `EXTENSION_WRONG_TYPE` | error | Extension '{url}' expects {expected}, got {type} |
| `MODIFIER_EXTENSION_UNKNOWN` | error | Unknown modifier extension '{url}' |

---

## EXTENSION_UNKNOWN

An extension was found whose URL does not match any loaded StructureDefinition. The validator cannot verify the extension's structure or content. This is a warning because the extension may be valid but simply not loaded into the validator.

**Example:**

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

If the StructureDefinition for `http://example.org/fhir/StructureDefinition/custom-ext` is not loaded, the validator produces this warning.

**Fix:** Load the Implementation Guide or StructureDefinition that defines this extension:

```bash
gofhir-validator -ig http://example.org/fhir/ImplementationGuide/example patient.json
```

---

## EXTENSION_INVALID_CONTEXT

The extension is defined with a context restriction that does not include the element where it was used. Every extension StructureDefinition declares where it is permitted via `StructureDefinition.context`.

**Example:**

An extension defined with context `Patient` used on an Observation:

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

**Fix:** Use the extension only in the contexts declared in its StructureDefinition, or update the extension definition to include the desired context.

---

## EXTENSION_MISSING_URL

An extension element was found that does not contain a `url` property. Every extension in FHIR must have a URL that identifies its definition.

**Example -- invalid resource:**

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

**Fix:** Add the `url` property:

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

A simple extension (one without nested sub-extensions) has no `value[x]` element. Simple extensions must carry a value unless they are complex extensions with sub-extensions.

**Example -- invalid resource:**

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

**Fix:** Add the appropriate value element:

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
Complex extensions (those with nested `extension` elements) do not have a `value[x]` -- they carry their data in sub-extensions instead. This error only applies to simple extensions.
{{< /callout >}}

---

## EXTENSION_MULTIPLE_VALUES

An extension contains more than one `value[x]` element. Each extension can have at most one value.

**Example -- invalid resource:**

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

**Fix:** Use only one `value[x]` element per extension:

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

The value type of the extension does not match the type declared in its StructureDefinition. For example, the extension definition specifies `valueCodeableConcept` but the resource provides `valueString`.

**Example:**

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

If the extension definition declares the value type as `CodeableConcept`, providing a `valueString` is not valid.

**Fix:** Use the correct value type as declared in the StructureDefinition:

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

A modifier extension was found whose URL does not match any loaded StructureDefinition. Unlike regular unknown extensions (which produce warnings), unknown modifier extensions produce **errors** because modifier extensions can change the meaning of the containing element. Processing a resource with an unrecognized modifier extension is unsafe.

**Example:**

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

**Fix:** Load the StructureDefinition that defines this modifier extension, or remove the modifier extension if it is not needed.

{{< callout type="warning" >}}
Modifier extensions are treated more strictly than regular extensions because they can alter the meaning of the resource or element. An unknown modifier extension always produces an error, while an unknown regular extension produces only a warning.
{{< /callout >}}
