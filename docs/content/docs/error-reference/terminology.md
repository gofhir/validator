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
| `CODING_NO_SYSTEM` | warning | Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided |
| `CODING_NO_CODE` | error | Coding has no code for system {system} and cannot be validated |
| `CODE_NOT_IN_CODESYSTEM` | error | The code '{code}' is not valid in the CodeSystem '{system}' |
| `CODESYSTEM_NOT_FOUND` | see below | CodeSystem is unknown and can't be validated: {system} for '{systemCode}' |
| `CODESYSTEM_NO_CODES` | see below | The CodeSystem '{codeSystem}' includes none of its codes (content '{content}'), so the code '{code}' cannot be validated |
| `CODE_NOT_IN_PARTIAL_CODESYSTEM` | see below | The code '{code}' is not among the codes the CodeSystem '{codeSystem}' includes, which are only some of its codes (content '{content}'), so it cannot be validated |
| `BINDING_CANNOT_VALIDATE` | see below | Code '{code}' in system '{system}' cannot be validated - external terminology system requires a terminology server |
| `BINDING_REQUIRED` | error | The value provided ('{code}') is not in the value set '{valueSet}' (required){detail} |
| `BINDING_REQUIRED_NO_CODING` | error | None of the codings provided are in the value set '{valueSet}', and a coding from this value set is required (codes = {codes}) |
| `BINDING_REQUIRED_NO_CODE` | error | No code provided, and a code is required from the value set '{valueSet}' |
| `BINDING_EXTENSIBLE` | warning | The value provided ('{code}') is not in the value set '{valueSet}' (extensible){detail} |
| `BINDING_EXTENSIBLE_NO_CODING` | warning | None of the codings provided are in the value set '{valueSet}', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = {codes}) |
| `BINDING_EXTENSIBLE_OTHER_SYSTEM` | information | The value provided ('{code}') is not in the value set '{valueSet}', and comes from a system the value set does not declare (extensible bindings permit this){detail} |
| `BINDING_EXTENSIBLE_UNKNOWN_SYSTEM` | warning | The value provided ('{code}') is not in the value set '{valueSet}' (extensible); whether its system belongs to the value set could not be determined{detail} |
| `BINDING_TEXT_ONLY_WARNING` | warning | No code provided, and a code should be provided from the value set '{valueSet}' (extensible) |
| `BINDING_DISPLAY_MISMATCH` | error | Display '{provided}' for code '{code}' does not match expected '{expected}' |
| `BINDING_UNRESOLVED` | information (the unresolved policy: `warn`, the default, information; `error`, error) | The value provided ('{code}') could not be checked against value set '{valueSet}': no terminology source could resolve it{detail} |
| `BINDING_VALUESET_NOT_FOUND` | warning | ValueSet '{valueSet}' not found |

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

## Codes that cannot be checked

A code whose code system cannot be checked here (`CODESYSTEM_NOT_FOUND`: no CodeSystem with its url; `CODESYSTEM_NO_CODES`: one with content `not-present`; `CODE_NOT_IN_PARTIAL_CODESYSTEM`: one with content `example` or `fragment`; `BINDING_CANNOT_VALIDATE`: a system only a terminology server checks) is reported with the severity the HL7 validator gives it. The specification sets none, and allows locally defined systems for tags (resource.html#tags). The severity follows from the definitions, never from the element's name:

| Where the Coding is | Severity |
|---------------------|----------|
| A CodeableConcept's coding (`Observation.code`) | warning, `CODESYSTEM_NOT_FOUND` at `.system`, a `not-present` or `example` CodeSystem too |
| A Coding a binding checks (not example, its value set resolving: `meta.security`, extensible) | warning |
| either, under a required binding (a CodeableConcept's coding: the concept's), the system in HL7's namespaces (`http://hl7.org/fhir`, `http://terminology.hl7.org`); for a Coding, not a `not-present` or `example` CodeSystem, which stays a warning | error |
| A Coding no binding checks (`meta.tag`, an extension's `valueCoding`) | information; a warning for an `example` CodeSystem |

The systems the HL7 validator treats apart are treated as it treats them:

| System | A CodeableConcept's coding | A Coding a binding checks | A Coding no binding checks |
|--------|----------------------------|---------------------------|----------------------------|
| LOINC, SNOMED CT, RxNorm, CPT | nothing | information (`BINDING_CANNOT_VALIDATE`) | warning |
| UCUM | warning (`CODESYSTEM_NOT_FOUND`) | warning (`BINDING_CANNOT_VALIDATE`) | warning |
| NDC, CVX, ICD-9-CM, ICD-10, ICD-10-CM | nothing | information | nothing |
| HL7's other known SIDs (`http://hl7.org/fhir/sid/srt`, `icd-9`, `icpc-2`, ...) | warning (an error under a required binding) | warning (an error under a required binding) | nothing |
| NDF-RT | nothing | information | warning |
| bcp47, ISO 3166 | warning | warning (`BINDING_CANNOT_VALIDATE`) | information |

A code a `fragment` CodeSystem does not have is a warning, at `.code`.

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

When terminology validation is disabled (using `-no-terminology` in the CLI or `WithNoTerminology()` in the API), all terminology-related errors are suppressed.
