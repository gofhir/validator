---
title: "Profile Errors"
linkTitle: "Profiles"
description: "Errors related to profile resolution, validity, and type mismatches."
weight: 10
---

Profile errors occur when the validator cannot load, parse, or apply a StructureDefinition (profile) to a resource. Profiles are the foundation of FHIR validation -- they define what a valid resource looks like. If a profile cannot be resolved or does not match the resource type, validation cannot proceed correctly.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `PROFILE_NOT_FOUND` | error | Profile '{profile}' could not be resolved |
| `PROFILE_INVALID` | error | Profile '{profile}' is not a valid StructureDefinition |
| `PROFILE_WRONG_TYPE` | error | Resource type '{type}' does not match profile type '{expected}' |

---

## PROFILE_NOT_FOUND

The profile URL specified for validation could not be resolved. The validator looked for a StructureDefinition with that canonical URL in the loaded registry but could not find it.

**Common causes:**

- The Implementation Guide containing the profile is not loaded
- The profile URL has a typo
- The profile URL includes a version fragment that does not match the loaded version
- The profile has not been published or is not available in the configured registries

**Example -- CLI:**

```bash
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

If the US Core Implementation Guide is not loaded, the validator cannot find the profile.

Validation output:

```text
ERROR: Profile 'http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient' could not be resolved
  MessageID: PROFILE_NOT_FOUND
```

**Fix:** Load the Implementation Guide that contains the profile:

```bash
gofhir-validator -ig hl7.fhir.us.core -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

Or programmatically, ensure the profile is registered:

```go
v, err := validator.New(
    validator.WithProfiles(usCorePatientProfile),
)
```

---

## PROFILE_INVALID

The StructureDefinition was found but it is not a valid or usable profile. This can occur when:

- The StructureDefinition JSON is malformed
- Required fields are missing (e.g., no `snapshot` and no `differential`)
- The `kind` is not `resource`, `complex-type`, or `logical`
- The StructureDefinition references a base definition that cannot be resolved
- The snapshot could not be generated from the differential

**Example output:**

```text
ERROR: Profile 'http://example.org/fhir/StructureDefinition/broken-profile' is not a valid StructureDefinition
  MessageID: PROFILE_INVALID
```

**Fix:** Verify the StructureDefinition is complete and well-formed. Use the FHIR validator itself to validate the StructureDefinition resource:

```bash
gofhir-validator profile.json
```

Ensure the profile has either a `snapshot` element or a `differential` element with a resolvable `baseDefinition`.

---

## PROFILE_WRONG_TYPE

The resource's `resourceType` does not match the type declared in the profile's `StructureDefinition.type` field. For example, attempting to validate a Patient resource against an Observation profile.

**Example -- invalid usage:**

```bash
gofhir-validator -ig http://hl7.org/fhir/StructureDefinition/bp observation.json
```

Where `observation.json` contains:

```json
{
  "resourceType": "Patient",
  "name": [{ "family": "Smith" }]
}
```

And the blood pressure profile declares `"type": "Observation"`.

Validation output:

```text
ERROR: Resource type 'Patient' does not match profile type 'Observation'
  Path: Patient
  MessageID: PROFILE_WRONG_TYPE
```

**Fix:** Validate the resource against a profile that matches its resource type:

```bash
gofhir-validator -ig http://hl7.org/fhir/StructureDefinition/Patient patient.json
```

### Profile Declared in meta.profile

Resources can declare which profiles they claim to conform to via `meta.profile`. The validator uses these declarations to select which profiles to validate against:

```json
{
  "resourceType": "Patient",
  "meta": {
    "profile": [
      "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"
    ]
  },
  "name": [{ "family": "Smith" }]
}
```

If a profile listed in `meta.profile` cannot be found, the validator produces a `PROFILE_NOT_FOUND` error. If it does not match the resource type, it produces a `PROFILE_WRONG_TYPE` error.

## Profile Resolution Chain

The validator resolves profiles by following the derivation chain:

```text
Custom Profile
  -> baseDefinition: US Core Patient
    -> baseDefinition: Patient
      -> baseDefinition: DomainResource
        -> baseDefinition: Resource
```

At each level, the validator merges constraints from the profile with its base. If any profile in the chain cannot be resolved, the validator reports `PROFILE_NOT_FOUND` for the missing link.

{{< callout type="info" >}}
All validation rules come from StructureDefinitions. The validator loads the profile, resolves the full derivation chain, generates a snapshot if needed, and then validates the resource against the merged constraints. No resource-type-specific logic is hardcoded.
{{< /callout >}}
