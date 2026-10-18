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
| `CODING_NO_CODE_IN_CONCEPT` | warning | No code is provided for the code system '{system}' |
| `CODE_NOT_IN_CODESYSTEM` | error | The code '{code}' is not valid in the CodeSystem '{system}' |
| `CODESYSTEM_NOT_FOUND` | see below | CodeSystem is unknown and can't be validated: {system} for '{systemCode}' |
| `CODESYSTEM_URI_INVALID` | error | The code system URI '{system}' is not valid: {reason} |
| `CODESYSTEM_VERSION_NOT_FOUND` | see below | The CodeSystem '{system}' version '{version}' is not known ({known}), so its codes cannot be validated |
| `CODESYSTEM_NO_CODES` | see below | The CodeSystem '{codeSystem}' includes none of its codes (content '{content}'), so the code '{code}' cannot be validated |
| `CODESYSTEM_EXAMPLE_CONTENT` | warning | The CodeSystem '{codeSystem}' has example content only (content '{content}'), so the code '{code}' cannot be validated |
| `CODE_NOT_IN_PARTIAL_CODESYSTEM` | see below | The code '{code}' is not among the codes the CodeSystem '{codeSystem}' includes, which are only some of its codes (content '{content}'), so it cannot be validated |
| `BINDING_CANNOT_VALIDATE` | see below | Code '{code}' in system '{system}' cannot be validated - external terminology system requires a terminology server |
| `BINDING_REQUIRED` | error | The value provided ('{code}') is not in the value set '{valueSet}' (required){detail} |
| `BINDING_REQUIRED_NO_CODING` | error | None of the codings provided are in the value set '{valueSet}', and a coding from this value set is required (codes = {codes}) |
| `BINDING_REQUIRED_NO_CODE` | error | No code provided, and a code is required from the value set '{valueSet}' |
| `BINDING_EXTENSIBLE` | warning | The value provided ('{code}') is not in the value set '{valueSet}' (extensible){detail} |
| `BINDING_EXTENSIBLE_NO_CODING` | warning | None of the codings provided are in the value set '{valueSet}', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = {codes}) |
| `BINDING_EXTENSIBLE_OTHER_SYSTEM` | warning (information for a system HL7 checks on a server only) | The value provided ('{code}') is not in the value set '{valueSet}', and comes from a system the value set does not declare (an extensible binding permits it only where the value set has no suitable code){detail} |
| `BINDING_EXTENSIBLE_UNKNOWN_SYSTEM` | warning | The value provided ('{code}') is not in the value set '{valueSet}' (extensible); whether its system belongs to the value set could not be determined{detail} |
| `BINDING_TEXT_ONLY_WARNING` | warning | No code provided, and a code should be provided from the value set '{valueSet}' (extensible) |
| `BINDING_DISPLAY_MISMATCH` | error | Display '{provided}' for code '{code}' does not match expected '{expected}' |
| `BINDING_UNRESOLVED` | information (the unresolved policy: `warn`, the default, information; `error`, error) | The value provided ('{code}') could not be checked against value set '{valueSet}': no terminology source could resolve it{detail} |
| `BINDING_VALUESET_NOT_FOUND` | warning | ValueSet '{valueSet}' not found |

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

A code whose code system cannot be checked here (`CODESYSTEM_NOT_FOUND`: no CodeSystem with its url; `CODESYSTEM_NO_CODES`: one with content `not-present`; `CODESYSTEM_EXAMPLE_CONTENT`: one with content `example`, which cannot tell a code is right though it lists it; `CODE_NOT_IN_PARTIAL_CODESYSTEM`: a code one with content `fragment` does not list; `BINDING_CANNOT_VALIDATE`: a system only a terminology server checks) is reported with the severity the HL7 validator gives it. The specification sets none, and allows locally defined systems for tags (resource.html#tags). The severity follows from the definitions, never from the element's name:

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

A Coding whose `version` names a version of a known CodeSystem that is not loaded (`CODESYSTEM_VERSION_NOT_FOUND`, at `.system`) is reported, a warning where a binding checks it or it is a CodeableConcept's, information where none does, and never an error: the code system is known, and a version not loaded here is no version that does not exist (a package carries one of a code system's versions). Neither its code nor its display is checked: what they are in that version is not known, as `CodeSystem.version` is an arbitrary value no other version stands for (versions are compared exactly). Its membership in a bound value set is decided by the value set's composition alone: in it where an include of that version lists the code (the includes are a union, less the excludes; version `*` admits every version), not where no include admits that version of the system (none names it, or each names another version or lists other codes) or an exclude removes the code, else not decided (an include of the whole system, a filter, a clause with no version under `compose.lockedDate`, which fixes a version not known, the value sets it takes in too, a clause or a listed code with a modifierExtension, a value set taken in that is not held, or what a value set taking itself in leaves undecided: the value sets taken in are decided together, whatever the order of their clauses); the value sets a clause names are told the same way, an exclude of them removing what they hold of the system, and a code a part of a clause lists is taken to be one of the version's (`BINDING_UNRESOLVED`, an error under the `error` unresolved policy). A system HL7 checks on a terminology server only (SNOMED CT, whose versions name editions, LOINC, RxNorm, ...) is reported as without a version, and bcp47 where no binding checks it keeps its report. A Coding with such a version and no code is reported so too. The HL7 validator 6.10.2 escalates the version to an error under a required binding for a system in its namespaces, and by the composition of the value set; here a required binding's error comes only from what the value set's composition decides.

In a Coding no binding checks, a system no CodeSystem has is checked as a URI first, as HL7 checks it: an error (`CODESYSTEM_URI_INVALID`) for a build.fhir.org URL, a system in HL7's namespace (`http://hl7.org/fhir`, with https or www) HL7 does not publish (a system under `http://hl7.org/fhir` a NamingSystem of kind `codesystem` of HL7's terminology package has as its preferred unique id is published: unknown, a warning, but in a CodeableConcept's coding under a required binding, an error), a web page (`.html`), a variant of SNOMED CT's, LOINC's, UCUM's or RxNorm's URL, or a known system written with https. A value set's URL as a system is an error in any Coding, at its `.system` where a binding checks it (the specification: the system SHALL NOT contain a reference to a value set), but a CodeableConcept's coding no binding of its own checks, where HL7 does not report it. A Quantity's system (a Quantity itself, not a type derived from it, as HL7 checks it) is checked as a Coding's is, but UCUM's, which the UCUM check validates; a Coding or Quantity a binding checks where none checks its base element (a profile's or an extension's binding on an element its base leaves unbound or binds by example) is checked besides as one none checks, as HL7 checks it. A Coding with a system and no code is allowed (`Coding.code` 0..1); a CodeableConcept's, with a known system, gets a warning (`CODING_NO_CODE_IN_CONCEPT`); under a required binding a Coding provides no code (`BINDING_REQUIRED_NO_CODE`); a Quantity with no code has no coded unit, which its binding does not ask for. Under an extensible binding, a CodeableConcept whose codings name systems and carry no code is not from the value set (`BINDING_EXTENSIBLE_NO_CODING`).

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
