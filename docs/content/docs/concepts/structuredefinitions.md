---
title: "StructureDefinitions"
linkTitle: "StructureDefinitions"
description: "How the GoFHIR Validator derives all validation rules from FHIR StructureDefinitions -- never hardcoded."
weight: 2
---

The GoFHIR Validator follows a fundamental design principle: **every validation rule is derived from StructureDefinitions**. The validator never hardcodes element names, cardinalities, allowed types, or any other rule. This makes the validator version-agnostic and capable of validating against any profile, including custom ones.

## What Is a StructureDefinition?

A [StructureDefinition](https://hl7.org/fhir/R4/structuredefinition.html) is the FHIR resource that defines the shape of other resources. It describes which elements exist, what types they can have, how many times they can repeat, and what additional constraints apply. Every FHIR resource type (Patient, Observation, etc.) has a base StructureDefinition, and profiles create additional StructureDefinitions that further constrain the base.

## Key ElementDefinition Fields

Each element within a StructureDefinition is described by an [ElementDefinition](https://hl7.org/fhir/R4/elementdefinition.html). The validator uses the following fields to drive validation:

### `path`

The dot-separated location of the element within the resource tree.

```text
Patient.name
Patient.name.given
Observation.value[x]
```

### `min` / `max`

Cardinality constraints. `min` is an integer (0 or higher); `max` is a string (`"0"`, `"1"`, `"*"`, etc.).

```json
{
  "path": "Patient.identifier",
  "min": 1,
  "max": "*"
}
```

This means at least one `identifier` is required, with no upper limit.

### `type`

The list of allowed data types for the element. Each type can optionally declare target profiles.

```json
{
  "path": "Observation.value[x]",
  "type": [
    { "code": "Quantity" },
    { "code": "string" },
    { "code": "CodeableConcept" }
  ]
}
```

### `binding`

A terminology binding that ties a coded element to a ValueSet. See [Terminology](../terminology) for details on binding strengths.

```json
{
  "path": "Observation.status",
  "binding": {
    "strength": "required",
    "valueSet": "http://hl7.org/fhir/ValueSet/observation-status"
  }
}
```

### `constraint`

FHIRPath invariants that must evaluate to `true` for the resource to be valid.

```json
{
  "path": "Patient",
  "constraint": [
    {
      "key": "pat-1",
      "severity": "error",
      "human": "SHALL at least contain a contact's details or a reference to an organization",
      "expression": "name.exists() or telecom.exists() or address.exists() or organization.exists()"
    }
  ]
}
```

### `fixed[x]` / `pattern[x]`

Fixed values require an exact match. Pattern values require that the resource element contains at least the specified fields (but may contain more).

```json
{
  "path": "Observation.code",
  "patternCodeableConcept": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9"
      }
    ]
  }
}
```

### `slicing`

Defines how a repeating element is divided into named slices using discriminators.

```json
{
  "path": "Patient.identifier",
  "slicing": {
    "discriminator": [
      { "type": "value", "path": "system" }
    ],
    "rules": "open"
  }
}
```

## Snapshot vs. Differential

A StructureDefinition can contain two representations of its elements:

- **Snapshot** -- The complete, fully expanded set of elements. Every element from the base resource type is listed, with any overrides applied. This is what the validator uses.
- **Differential** -- Only the elements that differ from the base definition. This is a compact representation used for authoring profiles.

```text
Profile (differential: 5 overridden elements)
  + Base StructureDefinition (snapshot: 80 elements)
  = Computed snapshot (80 elements with 5 overrides applied)
```

The GoFHIR Validator can work with StructureDefinitions that provide only a differential. When a snapshot is not present, the validator generates one by merging the differential with the base definition's snapshot.

## Profile Resolution Chain

Profiles form a chain of derivation through the `baseDefinition` field. The validator resolves this chain to build the full set of constraints:

```text
http://example.org/fhir/StructureDefinition/MyPatient
  --> baseDefinition: http://hl7.org/fhir/StructureDefinition/Patient
    --> baseDefinition: http://hl7.org/fhir/StructureDefinition/DomainResource
      --> baseDefinition: http://hl7.org/fhir/StructureDefinition/Resource
```

At each level, the differential is applied on top of the parent's snapshot. The final result is a fully resolved snapshot that includes all constraints from every level in the chain.

## Minimal StructureDefinition Example

Below is a simplified StructureDefinition that constrains `Patient` to require at least one `identifier`:

```json
{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/fhir/StructureDefinition/RequiredIdentifierPatient",
  "name": "RequiredIdentifierPatient",
  "status": "active",
  "kind": "resource",
  "abstract": false,
  "type": "Patient",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
  "derivation": "constraint",
  "differential": {
    "element": [
      {
        "id": "Patient.identifier",
        "path": "Patient.identifier",
        "min": 1
      }
    ]
  }
}
```

When the validator processes this profile, it resolves the base `Patient` StructureDefinition, generates a snapshot by applying the differential (setting `Patient.identifier.min` to `1`), and then runs all 9 validation phases against the merged result.

{{< callout type="info" >}}
The GoFHIR Validator is **version-agnostic**. It does not assume any particular FHIR version (R4, R4B, R5, etc.). All behavior is driven by the StructureDefinitions you load. Supply R5 definitions and it validates R5 resources; supply R4 definitions and it validates R4 resources.
{{< /callout >}}
