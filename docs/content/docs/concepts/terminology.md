---
title: "Terminology"
linkTitle: "Terminology"
description: "How the GoFHIR Validator handles CodeSystems, ValueSets, and binding strengths."
weight: 3
---

FHIR uses structured terminology -- CodeSystems and ValueSets -- to ensure that coded values are clinically meaningful and interoperable. The GoFHIR Validator checks coded elements against their declared terminology bindings, with behavior that varies according to the binding strength.

## CodeSystems vs. ValueSets

Understanding the distinction between these two resources is essential:

- **CodeSystem** -- Defines a set of codes and their meanings. Examples include LOINC, SNOMED CT, and FHIR-defined code systems like `http://hl7.org/fhir/administrative-gender`. A CodeSystem is the authoritative source of codes.

- **ValueSet** -- Defines a selection of codes drawn from one or more CodeSystems. A ValueSet can include all codes from a CodeSystem, a filtered subset, or a combination of codes from multiple systems. ValueSets are what bindings reference.

```text
CodeSystem: http://hl7.org/fhir/administrative-gender
  Codes: male, female, other, unknown

ValueSet: http://hl7.org/fhir/ValueSet/administrative-gender
  Includes: all codes from http://hl7.org/fhir/administrative-gender
```

## Binding Strengths

When an ElementDefinition declares a `binding`, it specifies both a ValueSet and a **strength** that determines how strictly the validator enforces membership. FHIR defines four binding strengths:

### Required

The code **must** come from the specified ValueSet. If the code is not a member, the validator produces an **error**.

This is the strictest binding. It is used for elements where interoperability demands a fixed set of values, such as `Patient.gender` or `Observation.status`.

```json
{
  "path": "Patient.gender",
  "binding": {
    "strength": "required",
    "valueSet": "http://hl7.org/fhir/ValueSet/administrative-gender"
  }
}
```

### Extensible

The code **should** come from the specified ValueSet. If the code is not a member but comes from an alternative system, the validator produces a **warning**. If no appropriate code exists in the ValueSet, implementers may use codes from other systems.

This strength balances interoperability with flexibility. It is commonly used in profiles like US Core.

```json
{
  "path": "Condition.code",
  "binding": {
    "strength": "extensible",
    "valueSet": "http://hl7.org/fhir/us/core/ValueSet/us-core-condition-code"
  }
}
```

### Preferred

The code is **recommended** from the specified ValueSet. The validator produces an **informational** note if the code is not a member. This is a soft recommendation with no conformance impact.

```json
{
  "path": "Encounter.type",
  "binding": {
    "strength": "preferred",
    "valueSet": "http://hl7.org/fhir/ValueSet/encounter-type"
  }
}
```

### Example

The ValueSet is provided as an **example** only. The validator performs **no validation** against example bindings. They exist purely for documentation and guidance.

```json
{
  "path": "Procedure.code",
  "binding": {
    "strength": "example",
    "valueSet": "http://hl7.org/fhir/ValueSet/procedure-code"
  }
}
```

## Summary of Binding Behavior

| Strength | Code not in ValueSet | Issue Severity |
|----------|---------------------|----------------|
| **required** | Must be in ValueSet | error |
| **extensible** | Should be in ValueSet | warning |
| **preferred** | Recommended | information |
| **example** | No validation | _(none)_ |

## Binding in an ElementDefinition

Here is a complete example of how a terminology binding appears within an ElementDefinition:

```json
{
  "id": "Observation.status",
  "path": "Observation.status",
  "min": 1,
  "max": "1",
  "type": [
    { "code": "code" }
  ],
  "binding": {
    "strength": "required",
    "description": "Codes providing the status of an observation.",
    "valueSet": "http://hl7.org/fhir/ValueSet/observation-status"
  }
}
```

The validator reads this binding, loads the referenced ValueSet, expands it into a flat list of valid codes, and checks whether the value in the resource is a member.

## Local vs. External Terminology Validation

The GoFHIR Validator supports **local terminology validation** using CodeSystem and ValueSet resources loaded into the registry. When you load an Implementation Guide or supply terminology resources directly, the validator can resolve and expand ValueSets locally without any external dependencies.

For terminology resources that are loaded locally:

1. The validator resolves the ValueSet URL from the binding.
2. It expands the ValueSet by evaluating `include` and `exclude` rules against loaded CodeSystems.
3. It checks whether the provided code is a member of the expanded set.

```go
v, err := validator.New(
    validator.WithValueSets(myValueSets...),
    validator.WithCodeSystems(myCodeSystems...),
)
```

{{< callout type="warning" >}}
**External terminology servers are not yet supported.** The GoFHIR Validator currently performs all terminology validation locally using loaded resources. Support for external FHIR terminology servers (the `$validate-code` operation) is planned for a future release. For now, ensure that all required CodeSystem and ValueSet resources are loaded into the validator.
{{< /callout >}}

## Disabling Terminology Validation

In some scenarios you may want to skip terminology validation entirely -- for example, when testing structural conformance without loading terminology resources. Use the `-tx n/a` flag with the CLI:

```bash
gofhir-validator -tx n/a patient.json
```

Or programmatically:

```go
v, err := validator.New(
    validator.WithTerminologyDisabled(),
)
```

When terminology validation is disabled, the binding phase is skipped and no terminology-related issues are reported.
