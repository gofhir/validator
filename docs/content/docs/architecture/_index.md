---
title: "Architecture"
linkTitle: "Architecture"
description: "Internal design, principles, and architectural decisions behind the GoFHIR Validator."
weight: 7
---

The GoFHIR Validator is designed as a modular, pipeline-based validation engine that derives all rules from FHIR StructureDefinitions. This section explains the internal architecture, the reasoning behind key decisions, and the optimization strategies that make the validator both correct and fast.

## Project Structure

```text
go-validator/
├── cmd/gofhir-validator/   # CLI application
├── pkg/
│   ├── validator/          # Main validator API and orchestrator
│   ├── binding/            # Terminology binding validation
│   ├── cardinality/        # Min/max cardinality validation
│   ├── constraint/         # FHIRPath constraint evaluation
│   ├── extension/          # Extension validation
│   ├── fixedpattern/       # Fixed/pattern value validation
│   ├── issue/              # Diagnostic messages and results
│   ├── loader/             # FHIR package loading
│   ├── location/           # Source location tracking
│   ├── logger/             # Logging utilities
│   ├── primitive/          # Primitive type validation
│   ├── reference/          # Reference validation
│   ├── registry/           # StructureDefinition registry
│   ├── slicing/            # Slicing validation
│   ├── specs/              # Embedded FHIR specifications
│   ├── structural/         # JSON structure validation
│   ├── terminology/        # Terminology services
│   └── walker/             # Resource tree traversal
├── testdata/               # Test fixtures (5,390 files)
└── docs/                   # Documentation (Hugo site)
```

Each `pkg/` sub-package is a self-contained module with its own `Validator` type, tests, and zero coupling to other validation packages. The orchestrator in `pkg/validator` composes them into a sequential pipeline.

## High-Level Data Flow

When you call `validator.Validate()`, the resource passes through these stages:

```text
JSON input
  │
  ▼
Parse & unmarshal (raw JSON → typed map)
  │
  ▼
Resolve StructureDefinition (registry lookup + snapshot)
  │
  ▼
Walker traversal (depth-first walk of the resource tree)
  │
  ▼
Phase execution (9 phases run sequentially)
  │  ┌─ 1. Structural
  │  ├─ 2. Cardinality
  │  ├─ 3. Primitive
  │  ├─ 4. Binding
  │  ├─ 5. Extension
  │  ├─ 6. Reference
  │  ├─ 7. Constraint
  │  ├─ 8. Fixed/Pattern
  │  └─ 9. Slicing
  │
  ▼
Result aggregation (merge issues → OperationOutcome)
```

Each phase receives the full resource tree and the resolved StructureDefinition. Phases produce `Issue` objects that are collected into a single `OperationOutcome` at the end.

{{< callout type="info" >}}
The pipeline design means that each phase can be developed, tested, and reasoned about independently. Adding a new validation rule typically means adding logic to a single package without touching the others.
{{< /callout >}}

## Explore the Architecture

{{< cards >}}
  {{< card link="design-principles" title="Design Principles" subtitle="Core principles that guide every implementation decision" icon="light-bulb" >}}
  {{< card link="decisions" title="Architecture Decisions" subtitle="21 ADRs documenting key technical choices" icon="document-text" >}}
  {{< card link="milestones" title="Milestones" subtitle="15-milestone development roadmap and status" icon="flag" >}}
  {{< card link="performance" title="Performance" subtitle="Optimization strategies and benchmark results" icon="lightning-bolt" >}}
{{< /cards >}}
