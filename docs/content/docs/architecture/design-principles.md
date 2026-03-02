---
title: "Design Principles"
linkTitle: "Design Principles"
description: "The four core design principles that guide every implementation decision in the GoFHIR Validator."
weight: 1
---

The GoFHIR Validator is built on four core principles that shape its architecture, API design, and implementation patterns. These principles ensure that the validator remains correct, extensible, and easy to integrate.

## 1. StructureDefinitions as Single Source of Truth

All validation rules are derived from FHIR StructureDefinitions at runtime. Nothing is hardcoded -- not element names, not cardinalities, not allowed types, not binding rules. This means the same validator code works with any FHIR version (R4, R4B, R5) and any profile without modification.

```go
// The validator reads rules from the StructureDefinition snapshot.
// This code works for ANY resource type or profile.
for _, elem := range snapshot.Element {
    if elem.Min > 0 {
        // Validate required element -- derived from the SD, not hardcoded
        validateMinCardinality(resource, elem.Path, elem.Min)
    }
    if elem.Binding != nil {
        // Validate terminology binding -- strength and valueSet from the SD
        validateBinding(resource, elem.Path, elem.Binding)
    }
}
```

```go
// WRONG: Hardcoding assumptions about specific resources
if resourceType == "Patient" {
    if patient.Identifier == nil {
        addError("Patient.identifier is required")
    }
}
```

This principle has a direct benefit: when a new FHIR profile is published, the validator supports it immediately by loading its StructureDefinition. No code changes are needed.

## 2. Pipeline Architecture

Validation is organized as a sequence of independent phases. Each phase is a separate Go package with its own `Validator` type that focuses on a single category of rules. The orchestrator in `pkg/validator` runs them in order and collects the results.

```go
// Each phase package exports a Validator type with a Validate method.
// The orchestrator composes them into a pipeline.
type Orchestrator struct {
    structural  *structural.Validator
    cardinality *cardinality.Validator
    primitive   *primitive.Validator
    binding     *binding.Validator
    extension   *extension.Validator
    reference   *reference.Validator
    constraint  *constraint.Validator
    fixedpattern *fixedpattern.Validator
    slicing     *slicing.Validator
}
```

Each phase:
- **Receives** the resource tree and the resolved StructureDefinition
- **Produces** zero or more `Issue` objects
- **Knows nothing** about other phases

This separation means you can test each phase in isolation, disable phases you do not need, and add new phases without modifying existing ones.

## 3. Small Interfaces (1-2 Methods)

Following Go's interface design philosophy, the validator defines small, focused interfaces that are easy to implement, mock, and compose. No interface in the project has more than two methods.

```go
// terminology.Provider -- 2 methods
type Provider interface {
    ValidateCode(ctx context.Context, system, code string) (bool, error)
    ValidateCoding(ctx context.Context, system, code, valueSetURL string) (bool, error)
}
```

```go
// registry.ProfileResolver -- 1 method
type ProfileResolver interface {
    ResolveProfile(url string) (*StructureDefinition, error)
}
```

Small interfaces make the validator highly composable:

- **Testing**: Provide a stub implementation with a few lines of code.
- **Extension**: Wrap an existing implementation with middleware (caching, logging, metrics).
- **Integration**: Implement just the interface you need for your environment.

```go
// A simple mock for testing -- implement 2 methods, done.
type mockProvider struct{}

func (m *mockProvider) ValidateCode(_ context.Context, system, code string) (bool, error) {
    return system == "http://loinc.org" && code == "12345-6", nil
}

func (m *mockProvider) ValidateCoding(_ context.Context, system, code, vsURL string) (bool, error) {
    return true, nil
}
```

## 4. Functional Options

Construction-time configuration uses the functional options pattern. This provides a clean API with reasonable defaults that can be extended without breaking changes.

```go
// Create a validator with default settings
v, err := validator.New()

// Create a validator with custom options
v, err := validator.New(
    validator.WithTerminologyProvider(myProvider),
    validator.WithLogger(myLogger),
    validator.WithDisabledPhases("constraint"),
)
```

Each `With*` function returns an `Option` that modifies the validator configuration:

```go
type Option func(*config)

func WithTerminologyProvider(p terminology.Provider) Option {
    return func(c *config) {
        c.terminologyProvider = p
    }
}

func WithLogger(l logger.Logger) Option {
    return func(c *config) {
        c.logger = l
    }
}
```

This pattern means:
- **Sensible defaults**: `validator.New()` works out of the box with no configuration.
- **Discoverable**: IDE auto-complete shows all available `With*` functions.
- **Non-breaking**: Adding a new option never changes the function signature.

{{< callout type="info" >}}
These four principles work together. StructureDefinitions provide the data, the pipeline organizes the logic, small interfaces enable composition, and functional options make configuration clean. The result is a validator that is correct by design, easy to test, and straightforward to extend.
{{< /callout >}}
