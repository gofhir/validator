---
title: "Terminology Errors"
linkTitle: "Terminology"
description: "Errors related to coding validation, binding strength enforcement, and ValueSet membership."
weight: 4
---

Terminology errors occur when coded values do not conform to the terminology bindings declared in the StructureDefinition. The validator checks that codes, systems, and displays are valid according to the referenced ValueSets and CodeSystems. The severity of terminology errors depends on the binding strength.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
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

A Coding element was found that has a `system` but no `code`. A Coding is only meaningful when it contains a code value.

**Example -- invalid resource:**

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

**Fix:** Add the `code` property:

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

A Coding element has a `code` but no `system`. While not always an error (hence the warning severity), a code without a system is ambiguous because the same code string can mean different things in different code systems.

**Example:**

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

**Fix:** Add the `system` property:

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

The `system` value in a Coding is not a valid URI. Code system identifiers in FHIR must be valid URIs.

**Example:**

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

The coded value is not a member of the ValueSet specified in a **required** binding. Required bindings are the strictest -- the code must be in the ValueSet.

**Example -- invalid resource:**

```json
{
  "resourceType": "Patient",
  "gender": "m"
}
```

`Patient.gender` has a required binding to `http://hl7.org/fhir/ValueSet/administrative-gender`, which only accepts `male`, `female`, `other`, and `unknown`. The value `m` is not in that ValueSet.

**Fix:**

```json
{
  "resourceType": "Patient",
  "gender": "male"
}
```

---

## BINDING_EXTENSIBLE_MISSING

The coded value is not a member of the ValueSet specified in an **extensible** binding. This produces a warning rather than an error, because extensible bindings allow codes from other systems when no appropriate code exists in the bound ValueSet.

**Example:**

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

If the profile has an extensible binding to a standard ValueSet and the custom code is not a member, the validator produces a warning.

---

## BINDING_UNKNOWN_SYSTEM

The code system referenced in a Coding could not be found in the loaded terminology resources. The validator cannot determine whether the code is valid without access to the CodeSystem.

---

## BINDING_INVALID_CODE

The code was found in a known CodeSystem, but it is not a valid code within that system. This indicates the code system was loaded, but the specific code does not exist.

**Example:**

```json
{
  "resourceType": "Patient",
  "gender": "m"
}
```

The code system `http://hl7.org/fhir/administrative-gender` is known and loaded, but `m` is not a valid code in it.

---

## BINDING_VALUESET_NOT_FOUND

The ValueSet referenced by a binding could not be resolved. The validator cannot validate the code against the binding because the ValueSet is not available.

This is a warning because the inability to resolve a ValueSet is an environment issue (missing terminology resources) rather than a problem with the validated resource itself.

**Fix:** Ensure the required ValueSet resources are loaded into the validator. When using the CLI, load the appropriate Implementation Guide:

```bash
gofhir-validator -ig hl7.fhir.us.core patient.json
```

Or programmatically:

```go
v, err := validator.New(
    validator.WithValueSets(myValueSets...),
)
```

## Binding Strength and Severity

The severity of terminology errors depends on the binding strength declared in the ElementDefinition:

| Binding Strength | Code Not in ValueSet | Issue Severity |
|------------------|---------------------|----------------|
| **required** | Must be in ValueSet | error |
| **extensible** | Should be in ValueSet | warning |
| **preferred** | Recommended | information |
| **example** | No validation | _(none)_ |

{{< callout type="info" >}}
The binding strength is read from `ElementDefinition.binding.strength` in the StructureDefinition. The validator never hardcodes which elements have which binding strength -- it is always derived from the profile.
{{< /callout >}}

When terminology validation is disabled (using `-tx n/a` in the CLI or `WithTerminologyDisabled()` in the API), all terminology-related errors are suppressed.
