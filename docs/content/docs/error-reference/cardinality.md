---
title: "Cardinality Errors"
linkTitle: "Cardinality"
description: "Errors related to minimum and maximum occurrence constraints on elements."
weight: 2
---

Cardinality errors occur when the number of occurrences of an element does not fall within the range declared in its ElementDefinition. Every element in a FHIR StructureDefinition has `min` and `max` properties that define how many times it may (or must) appear.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `CARDINALITY_MIN` | error | Minimum cardinality of '{path}' is {min}, but found {count} |
| `CARDINALITY_MAX` | error | Maximum cardinality of '{path}' is {max}, but found {count} |

---

## CARDINALITY_MIN

The element appears fewer times than the minimum cardinality requires. When `min` is `1` or greater, the element is mandatory and must be present in the resource.

**Example -- invalid resource:**

A Patient resource missing the required `gender` element when validated against a profile that sets `Patient.gender` to `min: 1`:

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

Validation output:

```text
ERROR: Minimum cardinality of 'Patient.gender' is 1, but found 0
  Path: Patient.gender
  MessageID: CARDINALITY_MIN
```

**Fix:** Add the required element:

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ],
  "gender": "male"
}
```

### Common Cases

Many base FHIR resources have required elements. For example:

- `Observation.status` (min: 1)
- `Observation.code` (min: 1)
- `Patient.name` in US Core (min: 1)
- `Bundle.type` (min: 1)
- `Medication.code` in many profiles (min: 1)

{{< callout type="info" >}}
Cardinality constraints are always derived from `ElementDefinition.min` and `ElementDefinition.max` in the StructureDefinition. Profiles can increase the minimum (making elements mandatory) but can never decrease it below the base definition value.
{{< /callout >}}

---

## CARDINALITY_MAX

The element appears more times than the maximum cardinality allows. When `max` is `"1"`, the element must appear at most once. When `max` is `"0"`, the element is prohibited.

**Example -- invalid resource:**

A Patient with multiple `birthDate` values (max cardinality is `"1"`):

```json
{
  "resourceType": "Patient",
  "birthDate": ["1990-01-01", "1991-02-02"]
}
```

Since `Patient.birthDate` has `max: "1"`, providing an array with multiple values is not allowed.

Validation output:

```text
ERROR: Maximum cardinality of 'Patient.birthDate' is 1, but found 2
  Path: Patient.birthDate
  MessageID: CARDINALITY_MAX
```

**Fix:** Provide only a single value:

```json
{
  "resourceType": "Patient",
  "birthDate": "1990-01-01"
}
```

### Max Cardinality of Zero

Profiles can set `max: "0"` to prohibit an element entirely. When this occurs, any occurrence of the element produces a `CARDINALITY_MAX` error with `max` shown as `0`.

```text
ERROR: Maximum cardinality of 'Patient.deceased[x]' is 0, but found 1
  Path: Patient.deceased[x]
  MessageID: CARDINALITY_MAX
```

## How Cardinality Is Determined

The validator reads `min` and `max` from the `ElementDefinition` in the profile's snapshot:

```json
{
  "id": "Patient.gender",
  "path": "Patient.gender",
  "min": 1,
  "max": "1",
  "type": [
    { "code": "code" }
  ]
}
```

- `min` is an integer (0, 1, 2, ...).
- `max` is a string that is either a number (`"0"`, `"1"`, `"2"`, ...) or `"*"` (unbounded).
- Profiles can tighten cardinality (increase min, decrease max) but not relax it beyond the base definition.
