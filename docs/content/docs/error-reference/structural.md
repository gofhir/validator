---
title: "Structural Errors"
linkTitle: "Structural"
description: "Errors related to JSON structure, unknown elements, and resource type identification."
weight: 1
---

Structural errors are detected during the earliest validation phase. They indicate problems with the basic JSON structure of the resource, the presence of unrecognized elements, or issues with the `resourceType` property. These errors must be resolved before other validation phases can produce meaningful results.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `STRUCTURE_UNKNOWN_ELEMENT` | error | Unknown element '{element}' |
| `STRUCTURE_INVALID_JSON` | error | Invalid JSON: {error} |
| `STRUCTURE_NOT_OBJECT` | error | Resource must be a JSON object |
| `STRUCTURE_NO_RESOURCE_TYPE` | error | Missing 'resourceType' property |
| `STRUCTURE_UNKNOWN_RESOURCE` | error | Unknown resourceType '{type}' |

---

## STRUCTURE_UNKNOWN_ELEMENT

An element was found in the resource that is not defined in the StructureDefinition for this resource type. This often indicates a typo in the element name or an element that belongs to a different resource type.

**Example -- invalid resource:**

```json
{
  "resourceType": "Patient",
  "nmae": [
    {
      "family": "Smith"
    }
  ]
}
```

The element `nmae` is not defined in the Patient StructureDefinition. The correct element name is `name`.

**Fix:**

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

{{< callout type="info" >}}
Unknown elements are detected by comparing each property in the JSON against the `ElementDefinition.path` entries in the snapshot of the resource's StructureDefinition. No hardcoded element lists are used.
{{< /callout >}}

---

## STRUCTURE_INVALID_JSON

The input is not valid JSON. The validator could not parse the content before any FHIR-level validation could begin.

**Example -- invalid resource:**

```json
{
  "resourceType": "Patient",
  "name": [
    { "family": "Smith", }
  ]
}
```

The trailing comma after `"Smith"` is not valid JSON.

**Fix:** Ensure the input is well-formed JSON. Use a JSON linter or your editor's built-in validation to catch syntax errors before running the FHIR validator.

---

## STRUCTURE_NOT_OBJECT

The top-level JSON value is not an object. FHIR resources must be represented as JSON objects (enclosed in curly braces).

**Example -- invalid resource:**

```json
[
  { "resourceType": "Patient" }
]
```

A JSON array was provided instead of a JSON object.

**Fix:** Ensure the resource is a single JSON object at the top level:

```json
{
  "resourceType": "Patient"
}
```

If you need to validate multiple resources, wrap them in a FHIR Bundle resource or validate each one individually.

---

## STRUCTURE_NO_RESOURCE_TYPE

The JSON object does not contain a `resourceType` property. Every FHIR resource must declare its type using this property.

**Example -- invalid resource:**

```json
{
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

**Fix:** Add the `resourceType` property:

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

---

## STRUCTURE_UNKNOWN_RESOURCE

The `resourceType` value does not match any known FHIR resource type. This may indicate a typo, a custom resource type that has not been loaded, or a version mismatch.

**Example -- invalid resource:**

```json
{
  "resourceType": "Pateint",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

The value `Pateint` is not a valid FHIR resource type (note the typo).

**Fix:** Use the correct resource type name. FHIR resource type names are case-sensitive and use PascalCase:

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

{{< callout type="warning" >}}
The set of known resource types is determined by the StructureDefinitions loaded into the validator. If you are using a custom resource type defined in an Implementation Guide, make sure the corresponding StructureDefinition is loaded.
{{< /callout >}}
