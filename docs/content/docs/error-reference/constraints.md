---
title: "Constraint Errors"
linkTitle: "Constraints"
description: "Errors related to FHIRPath constraint evaluation failures."
weight: 7
---

Constraint errors occur when a FHIRPath invariant defined in the StructureDefinition does not evaluate to `true`, or cannot be compiled or evaluated. FHIR defines constraints (invariants) on elements using FHIRPath expressions. Each constraint has a key (e.g., `ele-1`, `dom-6`), a severity (`error` or `warning`), a human-readable description, and a FHIRPath expression that must evaluate to `true` for the resource to be valid.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `CONSTRAINT_FAILED` | varies | Constraint failed: {constraint}: '{human}' |
| `CONSTRAINT_COMPILE_ERROR` | error (warning for the specification's own definitions) | Could not compile constraint '{key}': {error} |
| `CONSTRAINT_EVAL_ERROR` | warning | Could not evaluate constraint '{key}': {error} |

---

## CONSTRAINT_FAILED

A FHIRPath constraint did not evaluate to `true`. The severity of this issue is determined by the constraint definition itself -- each constraint declares whether violation is an `error` or a `warning`.

An invariant "must evaluate to true when run on the element" (conformance-rules.html#constraints): a single boolean is its value, an empty result fails, and any other result holds. So `resolve().code.text = 'target'` fails where the reference does not resolve, as the HL7 validator evaluates it. Invariants are evaluated on every element, a primitive with only an id or extensions (`"_status": {...}` with no `status`) included.

A few published invariants are empty where they should not apply. Each is corrected, in the version it is published in and only where its expression is the published one, to the expression a later official publication gives, of the same definition or of the same constraint (`Constraint.source`):

| Key | Published in | Corrected from |
|-----|--------------|----------------|
| `ref-1` | R4 and R4B `Reference` (a reference with no `reference`) | R5 |
| `bdl-8` | R4 `Bundle.entry` (an entry with no `fullUrl`) | R4B |
| `ras-2` | R4 `RiskAssessment.prediction` (a prediction with no probability) | R4B |
| `pd-1`, `us-core-13` | US Core 5.0.1 and 6.1.0 `PractitionerRole` (`telecom or endpoint`, `... or healthcareService or location`: no boolean with two of them) | US Core 9.0.0 |
| `vs-1` | R4, R4B and R5 vital signs profiles (`$this as dateTime` on a Period) | US Core 9.0.0, which republishes the same constraint corrected |
| `vsd-0`, `csd-0`, `que-0`, `lib-0`, … | R4, 30 canonical resources (`name.matches(...)`, a resource with no name) | R4B (`csd-0`) |

Three other R4 invariants are wrong as published, and corrected the same way: `que-12` asks for `enableBehavior` from three `enableWhen` on (R4B: from two), `tim-9` cannot be evaluated with several `when` (R4 and R4B, corrected from R5), and `con-3` compares a CodeableConcept with a string (R4B, which asks for a clinicalStatus only where there is a verificationStatus).

**Example -- invalid resource:**

The constraint `ele-1` is defined on the base `Element` type with the expression `hasValue() or (children().count() > id.count())` and the human description "All FHIR elements must have a @value or children". If an element has neither a value nor children, this constraint fails:

```json
{
  "resourceType": "Patient",
  "name": [
    {}
  ]
}
```

Validation output:

```text
ERROR: Constraint failed: ele-1: 'All FHIR elements must have a @value or children'
  Path: Patient.name[0]
  MessageID: CONSTRAINT_FAILED
```

**Fix:** Ensure the element has a value or children:

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

### Common Constraints

Here are some frequently encountered FHIR constraints:

| Key | Context | Human Description | Severity |
|-----|---------|-------------------|----------|
| `ele-1` | Element | All FHIR elements must have a @value or children | error |
| `dom-3` | DomainResource | If the resource is contained in another resource, it SHALL be referred to from elsewhere in the resource | error |
| `dom-6` | DomainResource | A resource should have narrative for robust management | warning |
| `obs-6` | Observation | dataAbsentReason SHALL only be present if Observation.value[x] is not present | error |
| `obs-7` | Observation | If Observation.code is the same as an Observation.component.code then the value element associated with the code SHALL NOT be present | error |
| `pat-1` | Patient.contact | SHALL at least contain a contact's details or a reference to an organization | error |
| `ref-1` | Reference | SHALL have a contained resource if a local reference is provided | error |
| `per-1` | Period | If present, start SHALL have a lower value than end | error |
| `txt-1` | Narrative.div | The narrative SHALL contain only the basic html formatting elements and attributes | error |
| `txt-2` | Narrative.div | The narrative SHALL have some non-whitespace content | error |

### Severity From the Constraint Definition

The severity of `CONSTRAINT_FAILED` is not fixed. It is read from the `ElementDefinition.constraint.severity` field in the StructureDefinition:

```json
{
  "key": "ele-1",
  "severity": "error",
  "human": "All FHIR elements must have a @value or children",
  "expression": "hasValue() or (children().count() > id.count())"
}
```

If `severity` is `"error"`, the issue is an error. If `severity` is `"warning"`, the issue is a warning. Profiles can add new constraints or change constraint severity (within limits).

{{< callout type="info" >}}
Constraints are evaluated using FHIRPath, a path-based navigation and extraction language for FHIR. The constraint expression should return a boolean value. When it returns `false`, or nothing, the constraint is violated.
{{< /callout >}}

---

## CONSTRAINT_COMPILE_ERROR and CONSTRAINT_EVAL_ERROR

An expression that does not compile cannot hold: `CONSTRAINT_COMPILE_ERROR` is an error, reported once per location, as the HL7 validator reports it. In the specification's own definitions it is a warning, since the defect is the specification's, not the instance's (R5's `eld-11` quotes a string with double quotes).

An expression that compiles but fails while it is evaluated is a failed invariant: `CONSTRAINT_FAILED` at the constraint's own severity, with the error in its message, as in the HL7 validator, which takes an exception from its FHIRPath engine as a failed invariant. For example, `or` on a collection of two items is an error in FHIRPath ("singleton evaluation of collections"):

```text
ERROR: Constraint failed: inv-1: '...' (could not be evaluated: SingletonExpectedError: or expects a single item on each side, got 2 on the left)
  Path: Patient
  MessageID: CONSTRAINT_FAILED
```

Only an evaluation stopped by the validator's own time limit says nothing about the instance: it is `CONSTRAINT_EVAL_ERROR`, a warning.

---

## Type-Level Constraint Evaluation

The validator evaluates FHIRPath constraints not only from the resource's own StructureDefinition, but also from the StructureDefinitions of complex data types used within the resource. For example, when a `Patient` resource contains a `name[0].period` element of type `Period`, the validator loads the Period StructureDefinition and evaluates its constraints (such as `per-1`: start <= end) against each Period instance found in the resource.

This recursive evaluation covers all nesting levels. For example, `Patient.name` (type HumanName) contains `HumanName.period` (type Period), and the Period SD's `per-1` constraint is evaluated even though it does not appear in the Patient snapshot.

### FHIR Additional Functions

The constraint engine supports FHIR-specific FHIRPath functions defined in [FHIR R4 §2.9.1.5](https://hl7.org/fhir/R4/fhirpath.html):

| Function | Description |
|----------|-------------|
| `resolve()` | Resolves a FHIR reference to the target resource |
| `memberOf()` | Checks if a code is in a ValueSet |
| `extension()` | Returns extensions matching a URL |
| `hasExtension()` | Checks if an extension exists |
| `conformsTo()` | Checks if a resource conforms to a profile |
| `htmlChecks()` | Validates XHTML narrative content per FHIR rules |

The `htmlChecks()` function validates that `text.div` content follows the FHIR XHTML rules (R4 §2.4.1): `<div>` root element, allowed HTML element whitelist, prohibited elements/attributes, and non-empty content. This enables the `txt-1` and `txt-2` constraints to be evaluated dynamically from the Narrative StructureDefinition.
