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
| `REFERENCE_INVALID_TARGET` | error | Invalid reference target type '{type}'. Allowed: {allowed} |
| `REFERENCE_TARGET_PROFILE` | error | Unable to find a profile match for {reference} among choices: {profiles} |
| `REFERENCE_MULTIPLE_MATCHES` | error | Multiple matches in bundle for reference {reference} |
| `REFERENCE_NOT_FOUND` | warning | Referenced resource '{value}' not found |
| `REFERENCE_TYPE_MISMATCH` | error | Reference type element '{type}' does not match reference target '{reference}' |

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

The reference points to a resource type that is not permitted by the ElementDefinition. Each reference element in a StructureDefinition lists the allowed target types via `ElementDefinition.type.targetProfile`. The target's type is the type of the resource the reference resolves to (see `REFERENCE_TARGET_PROFILE`), whatever its literal names: `Patient/p` resolving to an entry that holds a `Group` is a `Group`. A reference that does not resolve has the type its literal names (`[type]/[id]`).

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

If the profile constrains `Observation.subject` to reference only `Patient` or `Group`, a reference to `Organization` is not a valid target. The issue is reported at the Reference (`Observation.subject`), as the HL7 validator reports it.

A `targetProfile` may pin a version (`http://example.org/StructureDefinition/my-patient|1.0.0`); its type is the type of that version of the profile. The type of a literal reference is checked whether or not the target resolves: the target "must conform to at least one" of the profiles (ElementDefinition.type.targetProfile), and a resource of another type cannot. The HL7 validator 6.10.4 checks the type only of a target it resolves (decision B-D13 of plan B).

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

## REFERENCE_TARGET_PROFILE

The resource a reference resolves to conforms to none of the profiles its element allows for its type. `ElementDefinition.type.targetProfile` says "the content must conform to at least one of them".

The target is checked only when it resolves, as bundle.html#references says: `#id`, a contained resource of the resource that makes the reference (or of the resource that contains it), and `#`, from a contained resource, the resource that contains it; an absolute reference, the entry whose `fullUrl` it is; a relative one (`Patient/p`), the entry whose `fullUrl` is the referring entry's base followed by it, which needs the referring entry's `fullUrl` to be RESTful (not a `urn:uuid:`). Only the Bundle whose entry the referring resource is is searched: the specification gives a reference no meaning in a Bundle that holds that one. A version (`/_history/1`) is removed before matching the `fullUrl`, then matched against the entry's `meta.versionId`. A reference that several entries match is ambiguous and does not resolve (`REFERENCE_MULTIPLE_MATCHES`). A target that does not resolve is not checked.

The target is validated against each profile of its type (`derivation: constraint`) with the whole validation, and it conforms when one of them finds no error. When the element allows the definition of the target's type, or of a type it derives from (`Resource`, `DomainResource`), any target of that type conforms: its own validation reports what is wrong with it. Resources that reference each other (`Patient.link`, `Observation.hasMember`) conform when nothing else is wrong with them. The issue is reported at the Reference, as the HL7 validator reports it (`Reference_REF_CantMatchChoice`).

**Example:** a profile's `Observation.subject` allows `Reference(https://example.org/fhir/StructureDefinition/my-patient)`, which requires an `identifier`. A Bundle entry is an Observation whose subject is `Patient/p`, and the Bundle's `Patient/p` has no identifier:

```text
ERROR: Unable to find a profile match for Patient/p among choices: https://example.org/fhir/StructureDefinition/my-patient
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_TARGET_PROFILE
```

**Fix:** make the target conform to one of the profiles, or reference a resource that does.

---

## REFERENCE_MULTIPLE_MATCHES

Several entries of the Bundle match a reference, as bundle.html#references matches them: the same `fullUrl`, and the same `meta.versionId` when the reference names a version. The specification says "it is ambiguous which is correct" and that applications "MAY return an error"; gofhir does, as the HL7 validator does (`Bundle_BUNDLE_MultipleMatches`). The reference does not resolve: no type or profile of a target is checked for it.

**Example:** a `history` Bundle holds two versions of `Patient/p` (the same `fullUrl`, `meta.versionId` 1 and 2), and an Observation's subject is `Patient/p`:

```text
ERROR: Multiple matches in bundle for reference Patient/p
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_MULTIPLE_MATCHES
```

**Fix:** reference the version meant (`Patient/p/_history/2`), or give each entry its own `fullUrl`.

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

The reference's `type` element names another type than its target's: `Reference.type` and the reference "SHALL be consistent" (Reference.type). The target's type is the type of the resource the reference resolves to, a `urn:uuid:` reference's or a `#id` reference's included, else the type its literal names (`[type]/[id]`).

**Example:** an Observation's subject is `{"reference": "urn:uuid:…", "type": "Group"}`, and the Bundle's entry with that `fullUrl` is a Patient:

```text
ERROR: Reference type element 'Group' does not match reference target 'Patient'
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_TYPE_MISMATCH
```

**Fix:** make `type` the target's type, or reference a resource of that type.
