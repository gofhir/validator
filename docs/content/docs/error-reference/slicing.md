---
title: "Slicing Errors"
linkTitle: "Slicing"
description: "Errors related to slice matching, cardinality violations within slices, and closed slicing rules."
weight: 9
---

Slicing errors occur when elements in a repeated list do not conform to the slicing rules declared in the StructureDefinition. FHIR slicing allows profiles to define sub-groups (slices) within a repeating element, each with its own constraints. The validator checks that elements match the correct slices, that slice cardinality is respected, and that closed slicing rules are not violated.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `SLICE_UNMATCHED_CLOSED` | error | Element at '{path}' does not match any slice (closed slicing) |
| `SLICE_MIN_NOT_MET` | error | Slice '{slice}' requires minimum {min} occurrence(s), found {count} |
| `SLICE_MAX_EXCEEDED` | error | Slice '{slice}' allows maximum {max} occurrence(s), found {count} |

---

## SLICE_UNMATCHED_CLOSED

An element in a sliced list does not match any of the defined slices, and the slicing is declared as **closed** (i.e., `slicing.rules` is `"closed"` or `"openAtEnd"`). In closed slicing, every element must match exactly one slice.

**Example -- invalid resource:**

A profile defines slicing on `Observation.component` with closed rules, defining two slices: `systolic` and `diastolic`. An element that does not match either slice triggers this error:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8462-4" }]
      },
      "valueQuantity": { "value": 80, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8867-4" }]
      },
      "valueQuantity": { "value": 72, "unit": "/min" }
    }
  ]
}
```

If the third component (heart rate, LOINC 8867-4) does not match either the `systolic` or `diastolic` slice and the slicing is closed, the validator produces this error.

Validation output:

```text
ERROR: Element at 'Observation.component[2]' does not match any slice (closed slicing)
  Path: Observation.component[2]
  MessageID: SLICE_UNMATCHED_CLOSED
```

**Fix:** Remove the unmatched element or use a profile that allows it (open slicing).

### Slicing Rules

The `slicing.rules` field in the ElementDefinition determines how unmatched elements are handled:

| Rules | Behavior |
|-------|----------|
| `open` | Unmatched elements are allowed (no error) |
| `closed` | All elements must match a slice (error if unmatched) |
| `openAtEnd` | Unmatched elements are allowed only at the end of the list |

---

## SLICE_MIN_NOT_MET

A slice has a minimum cardinality that is not satisfied. Each slice can have its own `min` value that requires a minimum number of elements matching that slice.

**Example -- invalid resource:**

A blood pressure profile requires at least one `systolic` component (min: 1) and at least one `diastolic` component (min: 1). A resource with only the systolic component:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    }
  ]
}
```

Validation output:

```text
ERROR: Slice 'Observation.component:diastolic' requires minimum 1 occurrence(s), found 0
  Path: Observation.component
  MessageID: SLICE_MIN_NOT_MET
```

**Fix:** Add the required diastolic component:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8462-4" }]
      },
      "valueQuantity": { "value": 80, "unit": "mmHg" }
    }
  ]
}
```

---

## SLICE_MAX_EXCEEDED

More elements match a slice than the slice's maximum cardinality allows. Each slice can have its own `max` value that limits how many elements can match it.

**Example:**

A profile defines a slice `Observation.component:systolic` with max cardinality of `"1"`. If two components match the systolic discriminator:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "component": [
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 120, "unit": "mmHg" }
    },
    {
      "code": {
        "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
      },
      "valueQuantity": { "value": 118, "unit": "mmHg" }
    }
  ]
}
```

Validation output:

```text
ERROR: Slice 'Observation.component:systolic' allows maximum 1 occurrence(s), found 2
  Path: Observation.component
  MessageID: SLICE_MAX_EXCEEDED
```

**Fix:** Ensure only the allowed number of elements match each slice.

## How Slicing Works

Slicing is defined in the StructureDefinition by a `slicing` element on the base path, followed by individual slice definitions. The `slicing.discriminator` specifies how elements are matched to slices:

```json
{
  "id": "Observation.component",
  "path": "Observation.component",
  "slicing": {
    "discriminator": [
      {
        "type": "pattern",
        "path": "code"
      }
    ],
    "rules": "open"
  }
}
```

Each slice then uses a pattern or value on the discriminator path to define which elements belong to it:

```json
{
  "id": "Observation.component:systolic",
  "path": "Observation.component",
  "sliceName": "systolic",
  "min": 1,
  "max": "1",
  "patternCodeableConcept": {
    "coding": [{ "system": "http://loinc.org", "code": "8480-6" }]
  }
}
```

{{< callout type="info" >}}
Slicing rules, discriminators, and slice cardinalities are all read from the StructureDefinition. The validator does not hardcode any assumptions about which elements can be sliced or how slices are defined.
{{< /callout >}}
