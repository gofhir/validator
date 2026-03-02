---
title: "Fixed/Pattern Errors"
linkTitle: "Fixed/Pattern"
description: "Errors related to fixed value mismatches and pattern matching failures."
weight: 8
---

Fixed and pattern errors occur when an element's value does not match a `fixed[x]` or `pattern[x]` constraint declared in the ElementDefinition. These two constraint types serve different purposes:

- **fixed[x]** -- The element value must match the fixed value **exactly**. No additional properties are allowed beyond what is specified.
- **pattern[x]** -- The element value must contain at least the properties specified in the pattern, but additional properties are permitted.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `FIXED_VALUE_MISMATCH` | error | Value '{value}' does not match fixed value '{expected}' |
| `PATTERN_MISMATCH` | error | Value does not match required pattern at '{path}' |

---

## FIXED_VALUE_MISMATCH

The actual value of the element does not match the fixed value declared in the ElementDefinition. When an element has a `fixed[x]` constraint, the value in the resource must be identical to the fixed value.

**Example -- invalid resource:**

A profile sets `Observation.status` to a fixed value of `"final"`:

```json
{
  "id": "Observation.status",
  "path": "Observation.status",
  "fixedCode": "final"
}
```

A resource that violates this constraint:

```json
{
  "resourceType": "Observation",
  "status": "preliminary",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  }
}
```

Validation output:

```text
ERROR: Value 'preliminary' does not match fixed value 'final'
  Path: Observation.status
  MessageID: FIXED_VALUE_MISMATCH
```

**Fix:** Use the exact value specified by the profile:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  }
}
```

### Fixed Values on Complex Types

Fixed values can also apply to complex types. In this case, every property in the fixed value must match exactly:

```json
{
  "id": "Observation.code",
  "path": "Observation.code",
  "fixedCodeableConcept": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9"
      }
    ]
  }
}
```

The resource must include exactly the same coding with no additional codings or properties.

{{< callout type="warning" >}}
Fixed values are very strict. For complex types, using `pattern[x]` is usually more appropriate because it allows additional data alongside the required pattern. Fixed values are typically used only for primitive elements where an exact match is required.
{{< /callout >}}

---

## PATTERN_MISMATCH

The element's value does not include all the properties specified in the pattern. Unlike fixed values, patterns only require that the specified properties are present with the correct values -- additional properties are allowed.

**Example -- invalid resource:**

A profile defines a pattern on `Observation.code`:

```json
{
  "id": "Observation.code",
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

A resource that violates this pattern:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://snomed.info/sct",
        "code": "271649006"
      }
    ]
  }
}
```

The pattern requires a coding with system `http://loinc.org` and code `85354-9`, but the resource only contains a SNOMED coding.

Validation output:

```text
ERROR: Value does not match required pattern at 'Observation.code'
  Path: Observation.code
  MessageID: PATTERN_MISMATCH
```

**Fix:** Include the required pattern coding (additional codings are allowed):

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [
      {
        "system": "http://loinc.org",
        "code": "85354-9",
        "display": "Blood pressure panel"
      },
      {
        "system": "http://snomed.info/sct",
        "code": "271649006",
        "display": "Systemic arterial pressure"
      }
    ]
  }
}
```

### Key Differences Between Fixed and Pattern

| Aspect | fixed[x] | pattern[x] |
|--------|----------|------------|
| Match type | Exact match | Subset match |
| Additional properties | Not allowed | Allowed |
| Common use | Primitive values | Complex types |
| Strictness | Very strict | Flexible |

{{< callout type="info" >}}
Both `fixed[x]` and `pattern[x]` values are read from the ElementDefinition in the StructureDefinition. The validator compares the resource value against the declared constraint using deep equality (for fixed) or subset matching (for pattern).
{{< /callout >}}
