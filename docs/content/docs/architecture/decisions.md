---
title: "Architecture Decisions"
linkTitle: "Architecture Decisions"
description: "All 21 Architecture Decision Records (ADRs) documenting the key technical choices in the GoFHIR Validator."
weight: 2
---

This page documents every significant architecture decision made during the development of the GoFHIR Validator. We follow the Architecture Decision Record (ADR) format: each decision captures the **context** (why the decision was needed), the **decision** itself (what was chosen), and the **consequences** (trade-offs and implications).

## ADR Summary

| ADR | Title | Status |
|-----|-------|--------|
| ADR-001 | StructureDefinitions as Single Source of Truth | Accepted |
| ADR-002 | Pipeline Architecture with Phases | Accepted |
| ADR-003 | Small Interfaces (1-2 methods) | Accepted |
| ADR-004 | Functional Options for Configuration | Accepted |
| ADR-005 | Use gofhir/fhirpath for FHIRPath Evaluation | Accepted |
| ADR-006 | Use gofhir/fhir for Typed Structs | Accepted |
| ADR-007 | Automatic Package Loading by Version | Accepted |
| ADR-008 | OperationOutcome with Location Extensions | Accepted |
| ADR-009 | Incremental Development by Milestones | Accepted |
| ADR-010 | Centralized Message Catalog | Accepted |
| ADR-011 | Lightweight Registry Structs | Accepted |
| ADR-012 | BackboneElement Uses Root SD | Accepted |
| ADR-013 | Choice Types with Case-Insensitive Comparison | Accepted |
| ADR-014 | Primitive Types from SD Regex | Accepted |
| ADR-015 | FHIRPath Type Codes Handling | Accepted |
| ADR-016 | Three Independent Validation Phases | Accepted |
| ADR-017 | JSON Type Validation Before Regex | Accepted |
| ADR-018 | Recursive Complex Type Validation | Accepted |
| ADR-019 | StructureDefinition Caching Strategy | Proposed |
| ADR-020 | Validation Parallelism | Proposed |
| ADR-021 | Snapshot Generation | Proposed |

---

## Core Architecture (ADR-001 through ADR-004)

### ADR-001: StructureDefinitions as Single Source of Truth

**Context**: FHIR defines hundreds of resource types, profiles, and extensions. Hardcoding validation rules for each one would be unmaintainable and version-specific.

**Decision**: All validation rules are derived at runtime from StructureDefinition snapshots. The validator reads `ElementDefinition` entries to determine cardinality, types, bindings, constraints, and every other rule. No resource-specific logic exists in the codebase.

**Consequences**: The validator supports any FHIR version or custom profile without code changes. The trade-off is that a valid StructureDefinition snapshot must be available for every type being validated.

### ADR-002: Pipeline Architecture with Phases

**Context**: Validation involves many different categories of checks (structural, cardinality, terminology, FHIRPath constraints, etc.). Mixing them into a single pass would create tangled, hard-to-test code.

**Decision**: Validation is organized as a sequential pipeline of independent phases. Each phase is a separate package (`pkg/structural`, `pkg/cardinality`, etc.) with its own `Validator` type. The orchestrator in `pkg/validator` runs them in order and merges the results.

**Consequences**: Each phase can be developed, tested, and debugged in isolation. New phases can be added without modifying existing ones. The trade-off is that the resource tree is traversed multiple times (once per phase), but the clarity and maintainability benefits outweigh the performance cost.

### ADR-003: Small Interfaces (1-2 methods)

**Context**: The validator needs extension points for terminology services, profile resolution, and logging. Large interfaces create tight coupling and are difficult to implement.

**Decision**: All interfaces in the project have at most two methods. Key examples: `terminology.Provider` (2 methods), `registry.ProfileResolver` (1 method). This follows Go's standard library convention of small, composable interfaces.

**Consequences**: Interfaces are trivial to mock for testing and easy to wrap with middleware (caching, logging, metrics). Consumers only implement what they need. The trade-off is that complex operations may require combining multiple small interfaces rather than calling a single large one.

### ADR-004: Functional Options for Configuration

**Context**: The validator needs configurable behavior (terminology provider, logger, disabled phases, etc.) but must remain simple to use with sensible defaults.

**Decision**: Use the functional options pattern (`With*` functions returning `Option` closures) for all construction-time configuration. `validator.New()` works with zero arguments, and options are added as needed.

**Consequences**: The API is clean, discoverable, and backward-compatible. Adding new configuration options never changes the `New()` function signature. The trade-off is a slightly higher initial learning curve for users unfamiliar with the pattern.

---

## Dependencies (ADR-005 through ADR-006)

### ADR-005: Use gofhir/fhirpath for FHIRPath Evaluation

**Context**: FHIR constraints (`ElementDefinition.constraint`) are expressed as FHIRPath expressions. Evaluating them requires a FHIRPath engine capable of handling FHIR-specific functions like `resolve()` and `memberOf()`.

**Decision**: Use the `gofhir/fhirpath` library as the FHIRPath evaluation engine. It supports the FHIR-specific function set and integrates with Go's type system.

**Consequences**: Constraint evaluation is fully functional with support for `resolve()`, `memberOf()`, and context-aware evaluation. The dependency is well-scoped and can be replaced if needed since it is accessed through an internal interface.

### ADR-006: Use gofhir/fhir for Typed Structs

**Context**: The validator needs to represent FHIR data types (StructureDefinition, ElementDefinition, OperationOutcome, etc.) in Go code. Hand-writing these structs is error-prone and hard to keep in sync with the specification.

**Decision**: Use the `gofhir/fhir` library for typed Go structs that mirror FHIR resource definitions. These structs are used for loading StructureDefinitions and producing OperationOutcome results.

**Consequences**: Type safety at compile time and automatic JSON serialization/deserialization. The dependency is limited to data types and does not affect the validation logic itself.

---

## Infrastructure (ADR-007 through ADR-011)

### ADR-007: Automatic Package Loading by Version

**Context**: Users need to validate resources against the correct FHIR version (R4, R4B, R5). Requiring manual configuration of base definitions creates friction.

**Decision**: The validator automatically loads the appropriate FHIR specification package based on the declared version. FHIR R4, R4B, and R5 base definitions are embedded in the binary via `pkg/specs`, eliminating external file dependencies.

**Consequences**: Zero-configuration validation works out of the box. The binary is self-contained with no need to download specification files. The trade-off is increased binary size due to embedded specifications.

### ADR-008: OperationOutcome with Location Extensions

**Context**: FHIR's `OperationOutcome` provides `expression` (FHIRPath) and `location` (XPath) fields, but neither captures the exact JSON position (line and column) of an error.

**Decision**: Extend `OperationOutcome.issue` with custom extensions that carry source location information (line number, column number) from the original JSON input.

**Consequences**: Users get precise error locations for debugging. The output remains a valid FHIR OperationOutcome since extensions are part of the FHIR extensibility model.

### ADR-009: Incremental Development by Milestones

**Context**: Building a complete FHIR validator is a large undertaking. Attempting everything at once would delay delivery and make testing difficult.

**Decision**: Develop the validator incrementally across 15 milestones, following the FHIR type hierarchy from infrastructure (M0) through structural validation (M1) to advanced features like slicing (M12) and performance optimization (M15).

**Consequences**: Each milestone delivers testable, working functionality. The approach enabled continuous integration testing and early feedback. See the [Milestones](../milestones) page for the complete roadmap.

### ADR-010: Centralized Message Catalog

**Context**: Validation produces diagnostic messages that should be consistent, translatable, and easy to maintain. Scattering message strings across phase implementations makes them hard to keep consistent.

**Decision**: All diagnostic messages are defined in a centralized message catalog in the `pkg/issue` package. Each message has an identifier and a template string. Phases reference messages by identifier rather than constructing strings inline.

**Consequences**: Messages are consistent across phases, easy to review, and could be translated in the future. The catalog also serves as documentation of all possible validation outputs.

### ADR-011: Lightweight Registry Structs

**Context**: The full FHIR `StructureDefinition` type is large and includes fields not needed during validation (e.g., narrative, publisher metadata). Loading thousands of full SDs into memory is wasteful.

**Decision**: The `pkg/registry` package uses lightweight internal structs that extract only the fields needed for validation (snapshot elements, type information, binding details). Full SDs are parsed during loading and then discarded.

**Consequences**: Significantly reduced memory usage when loading large Implementation Guides with hundreds of profiles. The trade-off is a transformation step during loading, but this happens once at startup.

---

## Validation Logic (ADR-012 through ADR-018)

### ADR-012: BackboneElement Uses Root SD

**Context**: BackboneElement types are defined inline within a resource's StructureDefinition rather than as standalone types. The validator needs to locate the correct ElementDefinitions for these nested structures.

**Decision**: When validating BackboneElement children, the validator navigates the root resource's StructureDefinition rather than looking up a separate SD. Element paths like `Patient.contact.name` are resolved within the Patient SD.

**Consequences**: BackboneElement validation works correctly without requiring separate SDs for inline types. The walker maintains the full element path context to enable this navigation.

### ADR-013: Choice Types with Case-Insensitive Comparison

**Context**: FHIR choice types (e.g., `value[x]`) can appear in JSON as `valueString`, `valueQuantity`, etc. The type suffix must be matched against allowed types from the ElementDefinition.

**Decision**: Use case-insensitive comparison when matching the type suffix in a choice element name against the declared `ElementDefinition.type` entries. This handles edge cases like `valueUri` vs `valueURI`.

**Consequences**: Robust matching of choice types that handles all capitalization variants defined in the FHIR specification.

### ADR-014: Primitive Types from SD Regex

**Context**: FHIR primitive types (date, dateTime, uri, code, etc.) have format rules expressed as regular expressions in their StructureDefinitions.

**Decision**: Derive all primitive validation patterns from the `regex` extension in the primitive type's StructureDefinition rather than hardcoding patterns. The regex is read from `ElementDefinition.type` and compiled once.

**Consequences**: Primitive validation is version-agnostic and automatically correct for any FHIR version. Regex compilation is cached to avoid repeated parsing.

### ADR-015: FHIRPath Type Codes Handling

**Context**: FHIRPath expressions in constraints reference FHIR type codes (e.g., `Patient`, `HumanName`). The FHIRPath engine needs to resolve these to actual type information.

**Decision**: Type codes encountered during FHIRPath evaluation are resolved through the registry, mapping them to their corresponding StructureDefinitions. This enables the FHIRPath engine to navigate type hierarchies correctly.

**Consequences**: FHIRPath constraints that reference types work correctly. The registry serves as the single source of type information for both validation and FHIRPath evaluation.

### ADR-016: Three Independent Validation Phases

**Context**: Early designs considered a single-pass validation approach. However, some validation categories have natural dependencies (e.g., structural validation should run before cardinality).

**Decision**: While the full pipeline has 9 phases, they can be logically grouped into three independent stages: structural validation (phases 1-3), semantic validation (phases 4-9), and cross-resource validation. Each stage builds on the confidence established by the previous one.

**Consequences**: Fatal structural errors prevent unnecessary semantic validation. The grouping provides a clear mental model for understanding the validation flow.

### ADR-017: JSON Type Validation Before Regex

**Context**: Primitive validation involves two checks: the JSON value must be the correct type (string, number, boolean) and the value must match the format regex. Running regex on a wrong-typed value produces confusing errors.

**Decision**: Always validate the JSON type first. If the JSON type is wrong (e.g., a number where a string is expected), report that error and skip the regex check. This produces clearer, more actionable diagnostics.

**Consequences**: Users see the most fundamental error first. A `birthDate` with a numeric value reports "expected string, got number" rather than a regex mismatch.

### ADR-018: Recursive Complex Type Validation

**Context**: FHIR complex types (HumanName, Address, CodeableConcept, etc.) contain nested elements that must also be validated. The validation must handle arbitrary nesting depth.

**Decision**: Complex type validation is handled recursively. When the walker encounters a complex type element, it resolves the type's StructureDefinition and validates the children against it. This recursion naturally handles types nested to any depth.

**Consequences**: All complex types are validated uniformly regardless of nesting depth. The same code path handles both top-level elements and deeply nested structures within extensions or BackboneElements.

---

## Proposed (ADR-019 through ADR-021)

{{< callout type="warning" >}}
The following ADRs are in **Proposed** status. They document decisions under consideration that have not yet been implemented.
{{< /callout >}}

### ADR-019: StructureDefinition Caching Strategy

**Context**: Loading and parsing StructureDefinitions is expensive. In batch validation scenarios, the same SDs are used repeatedly.

**Decision (proposed)**: Implement a multi-level caching strategy with an in-memory LRU cache for parsed SDs and a file-system cache for downloaded packages. Cache invalidation would be based on package version.

**Consequences**: Significant performance improvement for batch validation and long-running server integrations. The caching layer would sit between the registry and the loader.

### ADR-020: Validation Parallelism

**Context**: When validating a batch of resources, each resource is independent and could be validated concurrently.

**Decision (proposed)**: Provide an optional parallel validation mode using a worker pool. The number of workers would be configurable via functional options. Each worker would have its own copy of mutable state while sharing read-only StructureDefinitions.

**Consequences**: Linear throughput scaling for batch workloads on multi-core systems. The shared-nothing design for mutable state would avoid lock contention.

### ADR-021: Snapshot Generation

**Context**: Some StructureDefinitions only provide a differential, not a full snapshot. The validator requires a snapshot for validation.

**Decision (proposed)**: Implement snapshot generation that merges a profile's differential with its base definition's snapshot. The generator would walk `baseDefinition` chains and apply differential overrides.

**Consequences**: The validator would be able to work with profiles that lack pre-computed snapshots. This is particularly important for user-authored profiles and Implementation Guides that only ship differentials.
