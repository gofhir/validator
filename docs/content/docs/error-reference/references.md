---
title: "Reference Errors"
linkTitle: "References"
description: "Errors related to FHIR resource references, target types, and reference resolution."
weight: 6
---

Reference errors occur when FHIR resource references are malformed, point to invalid target types, or cannot be resolved. FHIR references connect resources to each other and are constrained by the `ElementDefinition.type` entries that declare which target resource types are permitted. The validator checks reference format, target type compatibility, and optionally whether the referenced resource exists.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `REFERENCE_INVALID_FORMAT` | error | Reference '{value}' has invalid format |
| `REFERENCE_INVALID_TARGET` | error | Reference at '{path}' to '{value}' is not a valid target (expected {expected}) |
| `REFERENCE_NOT_FOUND` | warning | Referenced resource '{value}' not found |
| `REFERENCE_TYPE_MISMATCH` | error | Reference targets {type} but only {expected} allowed |

---

## REFERENCE_INVALID_FORMAT

The reference value does not conform to any valid FHIR reference format. FHIR references can take several forms:

- **Relative reference**: `ResourceType/id` (e.g., `Patient/123`)
- **Absolute reference**: `http://example.org/fhir/Patient/123`
- **Internal fragment reference**: `#resource-id`
- **Logical reference** (via `identifier`): Uses the `identifier` element instead of `reference`
- **UUID reference**: `urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

**Example -- invalid resource:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "just-an-id"
  }
}
```

The value `just-an-id` does not match any valid reference format.

**Fix:** Use a valid reference format:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "Patient/123"
  }
}
```

---

## REFERENCE_INVALID_TARGET

The reference points to a resource type that is not permitted by the ElementDefinition. Each reference element in a StructureDefinition lists the allowed target types via `ElementDefinition.type.targetProfile`.

**Example -- invalid resource:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "Organization/456"
  }
}
```

If the profile constrains `Observation.subject` to reference only `Patient` or `Group`, a reference to `Organization` is not a valid target.

**Fix:** Reference one of the allowed target types:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
  },
  "subject": {
    "reference": "Patient/123"
  }
}
```

---

## REFERENCE_NOT_FOUND

The referenced resource could not be resolved within the validation context. This is a warning because the resource may exist in an external server that the validator does not have access to. This error is most relevant when validating Bundles, where contained references and internal Bundle references are expected to be resolvable.

**Example:**

```json
{
  "resourceType": "Bundle",
  "type": "transaction",
  "entry": [
    {
      "resource": {
        "resourceType": "Observation",
        "status": "final",
        "code": {
          "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
        },
        "subject": {
          "reference": "Patient/999"
        }
      }
    }
  ]
}
```

If `Patient/999` is not included in the Bundle entries, the validator produces this warning.

**Fix:** Include the referenced resource in the Bundle or use a contained resource:

```json
{
  "resourceType": "Bundle",
  "type": "transaction",
  "entry": [
    {
      "resource": {
        "resourceType": "Patient",
        "id": "999",
        "name": [{ "family": "Smith" }]
      }
    },
    {
      "resource": {
        "resourceType": "Observation",
        "status": "final",
        "code": {
          "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
        },
        "subject": {
          "reference": "Patient/999"
        }
      }
    }
  ]
}
```

---

## REFERENCE_TYPE_MISMATCH

The referenced resource was resolved and its `resourceType` does not match the allowed target types for this reference element. This differs from `REFERENCE_INVALID_TARGET` in that the reference format itself is valid, but the actual resolved resource has the wrong type.

**Example:**

A Bundle where `Observation.subject` references an entry that turns out to be a `Device` instead of the expected `Patient`:

```json
{
  "resourceType": "Bundle",
  "type": "collection",
  "entry": [
    {
      "resource": {
        "resourceType": "Device",
        "id": "123"
      }
    },
    {
      "resource": {
        "resourceType": "Observation",
        "status": "final",
        "code": {
          "coding": [{ "system": "http://loinc.org", "code": "85354-9" }]
        },
        "subject": {
          "reference": "Device/123"
        }
      }
    }
  ]
}
```

If the ElementDefinition for `Observation.subject` only allows `Patient` and `Group`, this reference to `Device` produces an error.

{{< callout type="info" >}}
Allowed target types are read from `ElementDefinition.type` entries where `code` is `Reference`. The `targetProfile` values within those type entries specify which resource types the reference can point to. The validator derives these constraints entirely from the StructureDefinition.
{{< /callout >}}
