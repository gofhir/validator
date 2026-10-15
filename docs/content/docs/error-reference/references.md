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
| `REFERENCE_INVALID_FORMAT` | error | Invalid reference format: '{reference}' |
| `REFERENCE_INVALID_TARGET` | error | Invalid reference target type '{type}'. Allowed: {allowed} |
| `REFERENCE_TARGET_PROFILE` | error | Unable to find a profile match for {reference} among choices: {profiles} |
| `REFERENCE_MULTIPLE_MATCHES` | error | Multiple matches in bundle for reference {reference} |
| `REFERENCE_NOT_IN_BUNDLE` | warning | URN reference is not locally contained within the bundle {reference} |
| `REFERENCE_TYPE_MISMATCH` | error | Reference type element '{type}' does not match reference target '{reference}' |
| `REFERENCE_CONTAINED_NOT_FOUND` | error | Contained resource '{id}' not found. Reference is not defined within the resource |
| `REFERENCE_CONTAINER_NOT_FOUND` | error | The reference '#' names the resource that contains this one, but this resource is not contained |
| `REFERENCE_AGGREGATION_MODE` | error | Reference '{reference}' is not aggregated as the element allows: its target is not found where the aggregation modes {allowed} allow it |
| `REFERENCE_RELATIVE_NO_FULLURL` | error | Relative reference '{reference}' is made in a Bundle entry with no fullUrl, which it resolves from |

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

## REFERENCE_NOT_IN_BUNDLE

A `urn:uuid:` or `urn:oid:` reference, made in a Bundle, names no entry of it: such a reference has no meaning outside the Bundle (bundle.html#references). It is a warning, as the HL7 validator reports it; a relative or absolute reference to a resource outside the Bundle is not reported, unless the element's aggregation modes require it in the Bundle (`REFERENCE_AGGREGATION_MODE`).

**Example:** an entry's Observation has `"subject": {"reference": "urn:uuid:9f1c…"}`, and no entry has that `fullUrl`:

```text
WARNING: URN reference is not locally contained within the bundle urn:uuid:9f1c…
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_NOT_IN_BUNDLE
```

**Fix:** include the resource as an entry whose `fullUrl` is the URN, or reference it by its RESTful URL.

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

---

## REFERENCE_CONTAINED_NOT_FOUND

A fragment reference `#id` names no contained resource of the resource that makes it (or of the resource that contains it): "SHALL have a contained resource if a local reference is provided" (ref-1, references.html#contained). Every value of type Reference is checked, an extension's value and a choice element's included.

**Example:** a Patient's extension is `{"url": "…", "valueReference": {"reference": "#nope"}}`, and the Patient contains no resource `nope`:

```text
ERROR: Contained resource 'nope' not found. Reference is not defined within the resource
  Path: Patient.extension[0].valueReference.reference
  MessageID: REFERENCE_CONTAINED_NOT_FOUND
```

**Fix:** contain the resource the reference names, or reference it another way.

---

## REFERENCE_CONTAINER_NOT_FOUND

The reference `#` names the resource that contains the one making it ("For a resource that references the container, the reference is '#'", references.html#contained): made from a resource that is not contained (the resource validated, a Bundle entry's, a parameter's), it names nothing.

**Example:** an Observation that is not contained has `"focus": [{"reference": "#"}]`:

```text
ERROR: The reference '#' names the resource that contains this one, but this resource is not contained
  Path: Observation.focus[0].reference
  MessageID: REFERENCE_CONTAINER_NOT_FOUND
```

**Fix:** reference the resource by its id, or make the reference from a contained resource.

---

## REFERENCE_AGGREGATION_MODE

The element allows references aggregated only some ways (`ElementDefinition.type.aggregation`), decided by where the target is found: `contained`, a resource the referring one contains (`#id`), or, from a contained resource, its container (`#`); `bundled`, an entry of the Bundle the referring resource is in (none, when it is in no Bundle); `referenced`, a resource outside it, so any literal reference that is not a fragment.

**Example:** a profile allows `Patient.managingOrganization` only `bundled`, and `Organization/o9` is no entry of the Bundle:

```text
ERROR: Reference 'Organization/o9' is not aggregated as the element allows: its target is not found where the aggregation modes bundled allow it
  Path: Bundle.entry[0].resource.managingOrganization.reference
  MessageID: REFERENCE_AGGREGATION_MODE
```

**Fix:** include the target where the modes allow it (an entry of the Bundle, a contained resource), or reference it as the profile allows.

---

## REFERENCE_RELATIVE_NO_FULLURL

A relative reference (`Patient/p`) resolves from the fullUrl of the entry that holds the referring resource (bundle.html#references). Made in an entry that must have a fullUrl and has none, it has no base. Every entry must have one but a transaction's, a batch's, their responses' and one whose request is a POST (bundle.html#bundle-unique; R5's bdl-15, which R4 and R4B carry as an erratum, reports the missing fullUrl on the Bundle); the type of the innermost Bundle decides. The reference may be in the entry's resource, a resource it contains, or an extension.

**Example:** a collection's entry has no fullUrl, and its Observation has `"subject": {"reference": "Patient/1"}`:

```text
ERROR: Relative reference 'Patient/1' is made in a Bundle entry with no fullUrl, which it resolves from
  Path: Bundle.entry[0].resource.subject
  MessageID: REFERENCE_RELATIVE_NO_FULLURL
```

**Fix:** give the entry its fullUrl (a RESTful one, for relative references to resolve from it).
