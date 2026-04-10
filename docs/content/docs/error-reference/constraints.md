---
title: "Constraint Errors"
linkTitle: "Constraints"
description: "Errors related to FHIRPath constraint evaluation failures."
weight: 7
---

Constraint errors occur when a FHIRPath invariant defined in the StructureDefinition evaluates to `false` or encounters an evaluation error. FHIR defines constraints (invariants) on elements using FHIRPath expressions. Each constraint has a key (e.g., `ele-1`, `dom-6`), a severity (`error` or `warning`), a human-readable description, and a FHIRPath expression that must evaluate to `true` for the resource to be valid.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `CONSTRAINT_FAILED` | varies | Constraint failed: {constraint}: '{human}' |
| `CONSTRAINT_ERROR` | warning | Constraint '{constraint}' evaluation error: {error} |

---

## CONSTRAINT_FAILED

A FHIRPath constraint evaluated to `false`. The severity of this issue is determined by the constraint definition itself -- each constraint declares whether violation is an `error` or a `warning`.

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
Constraints are evaluated using FHIRPath, a path-based navigation and extraction language for FHIR. The constraint expression must return a boolean value. When it returns `false`, the constraint is considered violated.
{{< /callout >}}

---

## CONSTRAINT_ERROR

The FHIRPath expression for a constraint could not be evaluated due to a runtime error. This does not necessarily mean the resource is invalid -- it means the validator could not determine whether the constraint is satisfied. This is always reported as a warning.

**Common causes:**

- The FHIRPath expression references an element type that the engine does not fully support
- The expression uses a FHIRPath function that is not implemented
- A runtime error occurred during expression evaluation (e.g., type mismatch in comparison)

**Example output:**

```text
WARNING: Constraint 'inv-1' evaluation error: Function 'iif' not implemented
  Path: Observation
  MessageID: CONSTRAINT_ERROR
```

{{< callout type="warning" >}}
Constraint evaluation errors indicate a limitation of the FHIRPath engine, not necessarily a problem with the resource. If you encounter these consistently, check whether the FHIRPath function used in the constraint is supported by the GoFHIR Validator.
{{< /callout >}}

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
