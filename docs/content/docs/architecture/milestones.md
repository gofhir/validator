---
title: "Milestones"
linkTitle: "Milestones"
description: "The 15-milestone development roadmap that guided the GoFHIR Validator from infrastructure to production-ready validation."
weight: 3
---

The GoFHIR Validator was developed incrementally across 15 milestones, following the FHIR type hierarchy from foundational infrastructure through advanced validation features. Each milestone delivered testable, working functionality before moving on to the next, enabling continuous integration testing and early feedback throughout development.

## Milestone Overview

| Milestone | Package | Description | Status |
|-----------|---------|-------------|--------|
| M0 | `pkg/loader`, `pkg/registry` | Infrastructure: package loading, SD registry | Complete |
| M1 | `pkg/structural` | Structural validation, unknown elements | Complete |
| M2 | `pkg/cardinality` | Min/max cardinality validation | Complete |
| M3 | `pkg/primitive` | Primitive type validation (regex, JSON types) | Complete |
| M4 | (integrated) | Complex type recursive validation | Complete |
| M5-M6 | (integrated in binding) | Coding and CodeableConcept validation | Complete |
| M7 | `pkg/binding` | Terminology binding validation | Complete |
| M8 | `pkg/extension` | Extension validation | Complete |
| M9 | `pkg/reference` | Reference validation | Complete |
| M10 | `pkg/constraint` | FHIRPath constraint evaluation | Complete |
| M11 | `pkg/fixedpattern` | Fixed and pattern value validation | Complete |
| M12 | `pkg/slicing` | Slicing validation | Complete |
| M13 | (integrated) | Profile-based validation | Complete |
| M14 | `cmd/gofhir-validator` | CLI tool | Complete |
| M15 | (cross-cutting) | Performance optimization | Complete |

## Milestone Details

### M0 -- Infrastructure

The foundation milestone established the two core infrastructure packages:

- **`pkg/loader`**: Loads FHIR specification packages from NPM tarballs, directories, and embedded sources. Parses StructureDefinitions, ValueSets, and CodeSystems from the raw JSON files.
- **`pkg/registry`**: Stores parsed StructureDefinitions in a lookup-optimized registry. Resolves profiles by URL, maps resource types to their base definitions, and provides element-level lookups by path.

These packages are used by every subsequent milestone and form the backbone of the validator.

### M1 -- Structural Validation

Validates that a JSON resource is structurally sound according to its StructureDefinition:

- Every element in the JSON corresponds to a known path in the snapshot
- Unknown or misspelled elements are reported as errors
- Resource type declarations match the expected StructureDefinition

### M2 -- Cardinality Validation

Enforces the min/max constraints declared in each `ElementDefinition`:

- Required elements (`min >= 1`) that are absent produce errors
- Elements exceeding `max` cardinality produce errors
- Handles both singular elements and arrays

### M3 -- Primitive Type Validation

Validates FHIR primitive types using rules derived from the specification:

- JSON type checking (string, number, boolean) runs first
- Regex patterns are read from the primitive type's StructureDefinition
- Compiled regexes are cached to avoid repeated parsing

### M4 -- Complex Type Validation

Extends validation to complex types (HumanName, Address, Quantity, etc.):

- Recursively resolves the StructureDefinition for each complex type
- Validates children against the type's ElementDefinitions
- Handles arbitrary nesting depth within extensions and BackboneElements

### M5-M6 -- Coding and CodeableConcept

Validates the structure and semantics of coded values:

- `Coding` elements must have valid `system` and `code` fields
- `CodeableConcept` elements validate each contained Coding
- Prepares the foundation for terminology binding validation in M7

### M7 -- Terminology Binding Validation

Checks codes against bound ValueSets according to binding strength:

- **required**: Code must be in the ValueSet (error if not)
- **extensible**: Code should be in the ValueSet (warning if not, unless from a different system)
- **preferred**: Informational suggestion only
- Integrates with `terminology.Provider` for code lookup

### M8 -- Extension Validation

Validates FHIR extensions against their StructureDefinitions:

- Resolves each `Extension.url` to its defining StructureDefinition
- Validates the extension content recursively against its profile
- Checks that extensions are used in allowed contexts
- Handles nested extensions (extensions within extensions)

### M9 -- Reference Validation

Validates FHIR references against the allowed target types:

- Checks `Reference.reference` format (relative, absolute, internal)
- Validates `Reference.type` against `ElementDefinition.type.targetProfile`
- Validates aggregation modes (contained, bundled, referenced)

### M10 -- FHIRPath Constraint Evaluation

Evaluates FHIRPath expressions from `ElementDefinition.constraint`:

- Integrates with `gofhir/fhirpath` for expression evaluation
- Supports FHIR-specific functions: `resolve()`, `memberOf()`, `hasValue()`
- Context-aware evaluation with proper type resolution
- Configurable timeout for expression evaluation

### M11 -- Fixed and Pattern Value Validation

Enforces `fixed[x]` and `pattern[x]` constraints from ElementDefinitions:

- **fixed**: Resource value must match exactly
- **pattern**: Resource value must contain at least the specified fields
- Handles all FHIR data types (primitive, complex, coded)

### M12 -- Slicing Validation

Validates FHIR array slicing rules:

- Matches array elements to named slices using discriminators
- Supports discriminator types: value, pattern, type, profile, exists
- Validates per-slice cardinality constraints
- Handles ordered and unordered slicing

### M13 -- Profile-Based Validation

Integrates all phases for full profile validation:

- Resolves profiles declared in `meta.profile`
- Walks `baseDefinition` chains to build full constraint sets
- Validates against each declared profile independently
- Merges issues from all profile validations

### M14 -- CLI Tool

Delivers a command-line interface comparable with the HL7 FHIR Validator:

- Validates JSON resources from files or stdin
- Supports `-ig` for loading Implementation Guides
- Provides `-output json` for machine-readable results
- Includes `-tx n/a` to disable terminology validation

### M15 -- Performance Optimization

Cross-cutting optimizations applied to all packages:

- Object pooling with `sync.Pool` for Result and Stats objects
- Regex compilation caching
- Element index caching per StructureDefinition
- Embedded specifications to avoid disk I/O at runtime

See the [Performance](../performance) page for detailed benchmark results.

## Development Approach

The milestone ordering follows the FHIR type hierarchy, starting from the most fundamental validations and building up to more complex ones:

```text
M0  Infrastructure (loader, registry)
 │
 ▼
M1  Structural (is the JSON well-formed?)
 │
 ▼
M2  Cardinality (are required elements present?)
 │
 ▼
M3  Primitive (are values correctly formatted?)
 │
 ▼
M4  Complex Types (recursive type validation)
 │
 ▼
M5-M7  Terminology (are codes valid?)
 │
 ▼
M8-M9  Extensions & References (FHIR-specific constructs)
 │
 ▼
M10-M12  Constraints, Fixed/Pattern, Slicing (advanced rules)
 │
 ▼
M13-M14  Integration (profiles, CLI)
 │
 ▼
M15  Performance (optimization pass)
```

{{< callout type="info" >}}
Each milestone was designed to be independently testable. The test suite grew with each milestone, and all previous milestone tests continue to pass as new functionality is added. The `testdata/` directory contains over 5,390 test fixtures covering every milestone.
{{< /callout >}}
