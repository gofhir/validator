---
title: "Validation Phases"
linkTitle: "Validation Phases"
description: "The GoFHIR Validator executes 9 sequential phases to thoroughly validate every FHIR resource."
weight: 1
---

The GoFHIR Validator uses a **pipeline architecture** that breaks validation into 9 discrete phases. Each phase focuses on a specific category of rules and runs in a fixed order. This design keeps each phase simple, testable, and independently extensible.

## Phase Execution Order

When you validate a resource, the following phases run in sequence:

| # | Phase | Package | Description |
|---|-------|---------|-------------|
| 1 | Structural | `pkg/structural` | Unknown elements, valid JSON structure |
| 2 | Cardinality | `pkg/cardinality` | min/max constraints from ElementDefinition |
| 3 | Primitive | `pkg/primitive` | Type validation (regex patterns, JSON types) |
| 4 | Binding | `pkg/binding` | Terminology validation (ValueSet/CodeSystem) |
| 5 | Extension | `pkg/extension` | Extension URL resolution, context validation |
| 6 | Reference | `pkg/reference` | Reference format and type validation |
| 7 | Constraint | `pkg/constraint` | FHIRPath invariant evaluation |
| 8 | Fixed/Pattern | `pkg/fixedpattern` | `fixed[x]` and `pattern[x]` constraints |
| 9 | Slicing | `pkg/slicing` | Slice discriminator matching and cardinality |

Each phase receives the full resource tree and the resolved StructureDefinition, then produces zero or more **Issue** objects describing any violations found.

## How Phases Work

### 1. Structural

The structural phase verifies that the JSON payload is well-formed and that every element in the resource corresponds to a known path in the StructureDefinition. Unknown or misspelled elements produce an error.

### 2. Cardinality

This phase walks every ElementDefinition in the snapshot and checks that the number of values provided falls within the declared `min` and `max` bounds. A required element (`min: 1`) that is absent produces an error; an element that exceeds `max` also produces an error.

### 3. Primitive

Primitive types such as `dateTime`, `uri`, `code`, and `id` have format rules defined by FHIR (typically as regular expressions). This phase validates that each primitive value matches the expected format and JSON type.

### 4. Binding

When an ElementDefinition declares a terminology `binding`, this phase checks whether the provided code, Coding, or CodeableConcept is a member of the bound ValueSet. Validation behavior depends on the binding strength (see [Terminology](../terminology)).

### 5. Extension

Extensions are a core FHIR extensibility mechanism. This phase resolves each extension URL to its StructureDefinition, validates that the extension is used in an allowed context, and recursively validates the extension's content against its profile.

### 6. Reference

FHIR references (`Reference.reference`, `Reference.type`) must conform to the allowed target types declared in the ElementDefinition. This phase validates the reference format, checks the declared type targets, and validates aggregation modes.

### 7. Constraint

StructureDefinitions can declare FHIRPath invariants via `ElementDefinition.constraint`. This phase evaluates each constraint expression against the resource and reports violations. For example, the `Patient` resource has a constraint `name.exists() or identifier.exists()`.

### 8. Fixed/Pattern

When an ElementDefinition specifies a `fixed[x]` value, the resource element must match exactly. When it specifies a `pattern[x]` value, the resource element must contain at least the specified fields. This phase enforces both.

### 9. Slicing

FHIR arrays can be sliced into named groups using discriminators. This phase matches each array element to the correct slice based on discriminator values, then validates that each slice meets its own cardinality constraints.

## Profile Validation Flow

When a resource includes a `meta.profile` declaration, the validator follows this flow:

1. **Load declared profiles** -- Each URL in `meta.profile` is resolved to a StructureDefinition.
2. **Resolve the profile chain** -- The validator walks `baseDefinition` links up to the base resource type, building the full set of constraints.
3. **Run all 9 phases** -- The pipeline executes against each declared profile. A resource with multiple profiles is validated against each one.
4. **Merge issues** -- All issues from all profile validations are collected into a single result.

```text
Resource (meta.profile: "http://example.org/MyPatient")
  --> Load MyPatient StructureDefinition
    --> baseDefinition: "http://hl7.org/fhir/StructureDefinition/Patient"
      --> baseDefinition: "http://hl7.org/fhir/StructureDefinition/DomainResource"
        --> baseDefinition: "http://hl7.org/fhir/StructureDefinition/Resource"
  --> Run 9 phases with merged constraints
  --> Return OperationOutcome
```

## Issue Output

Every phase produces **Issue** objects that follow the FHIR `OperationOutcome.issue` structure:

```go
type Issue struct {
    Severity    IssueSeverity   // fatal | error | warning | information
    Code        IssueType       // e.g., structure, required, value, code-invalid
    Diagnostics string          // Human-readable description
    Expression  []string        // FHIRPath to the offending element
}
```

Issues are collected across all phases and returned as part of the final `OperationOutcome`. The severity levels follow the FHIR specification:

- **fatal** -- The validator could not process the resource (e.g., invalid JSON).
- **error** -- The resource violates a mandatory rule and is non-conformant.
- **warning** -- The resource may have an issue but is not necessarily invalid (e.g., extensible binding mismatch).
- **information** -- Informational notes (e.g., preferred binding suggestions).

{{< callout type="info" >}}
Phases are designed to be **fail-safe**: if an earlier phase encounters a fatal error (such as unparseable JSON), subsequent phases are skipped gracefully. Non-fatal errors in one phase do not prevent later phases from running.
{{< /callout >}}
