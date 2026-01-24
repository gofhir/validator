# FHIR Validator

A high-performance, Go-idiomatic FHIR resource validator with support for parallel validation, streaming, and extensible terminology services.

## Features

- **Multi-phase validation pipeline** - Structure, primitives, cardinality, terminology, references, extensions, constraints, slicing
- **Parallel validation** - Worker pool for batch validation with configurable concurrency
- **Streaming support** - Validate large bundles without loading them entirely into memory
- **Generic caches** - LRU caches with Go generics for type safety
- **sync.Pool optimization** - Reduced GC pressure through object pooling
- **Extensible services** - Pluggable profile resolution, terminology, and reference resolution
- **FHIR R4 support** - Full support for FHIR R4, with R4B and R5 planned

## Installation

```bash
go get github.com/robertoaraneda/gofhir/fhirvalidator
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    fv "github.com/robertoaraneda/gofhir/fhirvalidator"
    "github.com/robertoaraneda/gofhir/fhirvalidator/engine"
)

func main() {
    ctx := context.Background()

    // Create a validator for FHIR R4
    validator, err := engine.New(ctx, fv.R4)
    if err != nil {
        log.Fatal(err)
    }

    // Validate a resource
    patient := []byte(`{
        "resourceType": "Patient",
        "id": "example",
        "gender": "male",
        "birthDate": "1990-01-15"
    }`)

    result, err := validator.Validate(ctx, patient)
    if err != nil {
        log.Fatal(err)
    }

    if result.Valid {
        fmt.Println("Resource is valid!")
    } else {
        fmt.Printf("Found %d errors:\n", result.ErrorCount())
        for _, issue := range result.Errors() {
            fmt.Printf("  - %s\n", issue.Diagnostics)
        }
    }
}
```

## Configuration Options

```go
validator, err := engine.New(ctx, fv.R4,
    // Enable/disable validation phases
    fv.WithTerminology(true),      // Enable terminology validation
    fv.WithConstraints(true),       // Enable FHIRPath constraint validation
    fv.WithReferences(true),        // Enable reference validation
    fv.WithExtensions(true),        // Enable extension validation
    fv.WithUnknownElements(true),   // Check for unknown elements

    // Performance tuning
    fv.WithParallelPhases(true),    // Run independent phases in parallel
    fv.WithWorkerCount(8),          // Number of workers for batch validation
    fv.WithMaxErrors(100),          // Stop after N errors

    // Caching
    fv.WithStructureDefCache(1000), // Cache size for StructureDefinitions
    fv.WithValueSetCache(500),      // Cache size for ValueSets
    fv.WithExpressionCache(2000),   // Cache size for FHIRPath expressions
)
```

## Services

### Profile Service

Load and resolve StructureDefinitions:

```go
import (
    "github.com/robertoaraneda/gofhir/fhirvalidator/loader"
    "github.com/robertoaraneda/gofhir/r4"
)

// Create an in-memory profile service
profileService := loader.NewInMemoryProfileService()

// Load a custom profile
customProfile := &r4.StructureDefinition{
    // ... profile definition
}
err := profileService.LoadR4StructureDefinition(customProfile)

// Set on validator
validator.SetProfileService(profileService)
```

### Terminology Service

Validate codes against ValueSets and CodeSystems:

```go
import "github.com/robertoaraneda/gofhir/fhirvalidator/terminology"

// Create terminology service (pre-loaded with common FHIR code systems)
termService := terminology.NewInMemoryTerminologyService()

// Add custom ValueSet
termService.AddCustomValueSet(
    "http://example.org/ValueSet/priority",
    "http://example.org/CodeSystem/priority",
    map[string]string{
        "high":   "High Priority",
        "medium": "Medium Priority",
        "low":    "Low Priority",
    },
)

// Load R4 ValueSet
vs := &r4.ValueSet{...}
termService.LoadR4ValueSet(vs)

// Set on validator
validator.SetTerminologyService(termService)
```

## Batch Validation

Validate multiple resources in parallel:

```go
import "github.com/robertoaraneda/gofhir/fhirvalidator/worker"

// Using BatchValidator
bv := worker.NewBatchValidator(func(ctx context.Context, resource []byte) (*fv.Result, error) {
    return validator.Validate(ctx, resource)
}, 4) // 4 workers

resources := [][]byte{patient1, patient2, observation1, ...}
result := bv.ValidateBatch(ctx, resources)

fmt.Printf("Validated %d resources, %d completed\n",
    result.TotalJobs, result.CompletedJobs)

for _, jobResult := range result.Results {
    if jobResult.Error != nil {
        fmt.Printf("Job %s failed: %v\n", jobResult.ID, jobResult.Error)
    }
}
```

Or use the built-in batch validation:

```go
results := validator.ValidateBatch(ctx, resources)
for i, result := range results {
    if result.HasErrors() {
        fmt.Printf("Resource %d has errors\n", i)
    }
}
```

## Streaming Bundle Validation

For large bundles, use streaming validation:

```go
file, _ := os.Open("large-bundle.json")
defer file.Close()

// Stream validation results
for entry := range validator.ValidateBundleStream(ctx, file) {
    if entry.Result.HasErrors() {
        fmt.Printf("Entry %s has %d errors\n",
            entry.FullURL, entry.Result.ErrorCount())
    }
    entry.Result.Release() // Return to pool
}
```

Or aggregate all results:

```go
results := validator.ValidateBundleStream(ctx, file)
aggregated := engine.AggregateBundleResults(results)

fmt.Printf("Bundle has %d entries, %d valid\n",
    aggregated.TotalEntries, aggregated.ValidEntries)
```

## Profile Validation

Validate against specific profiles:

```go
result, err := validator.ValidateWithProfiles(ctx, resource,
    "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient",
)
```

## Result Handling

```go
result, _ := validator.Validate(ctx, resource)

// Check validity
if result.Valid {
    // No errors
}

// Count issues
errors := result.ErrorCount()
warnings := result.WarningCount()

// Get specific issue types
for _, issue := range result.Issues {
    switch issue.Severity {
    case fv.SeverityError:
        fmt.Printf("ERROR at %v: %s\n", issue.Expression, issue.Diagnostics)
    case fv.SeverityWarning:
        fmt.Printf("WARNING at %v: %s\n", issue.Expression, issue.Diagnostics)
    }
}

// Release result back to pool (optional, improves performance)
result.Release()
```

## Performance

Benchmarks on Apple M4 Pro:

| Benchmark | Time | Allocations |
|-----------|------|-------------|
| Simple Patient | ~11µs | 62 allocs |
| Complex Patient | ~21µs | 229 allocs |
| Observation | ~16µs | 124 allocs |
| Bundle (10 entries) | ~36µs | 478 allocs |

### Parallel Speedup

```
20 resources:
  Sequential: ~270µs
  Parallel (4 workers): ~93µs  (2.9x speedup)
```

## Architecture

```
fhirvalidator/
├── engine/          # Main validation engine
├── pipeline/        # Validation pipeline with phases
├── phase/           # Individual validation phases
├── service/         # Service interfaces (profile, terminology, reference)
├── loader/          # Profile loading and conversion
├── terminology/     # Terminology service implementation
├── worker/          # Worker pool for parallel validation
├── stream/          # Streaming bundle validation
├── cache/           # Generic LRU cache
├── pool/            # sync.Pool wrappers
└── types/           # Shared types
```

### Validation Phases

1. **Structure** - Validate JSON structure against StructureDefinition
2. **Primitives** - Validate primitive types (date, uri, id, etc.)
3. **Cardinality** - Check min/max cardinality constraints
4. **Unknown Elements** - Detect elements not in profile
5. **Fixed/Pattern** - Validate fixed and pattern values
6. **Terminology** - Validate codes against ValueSets
7. **References** - Validate reference formats and targets
8. **Extensions** - Validate extension URLs and contexts
9. **Constraints** - Evaluate FHIRPath invariants
10. **Slicing** - Validate slicing rules
11. **Bundle** - Bundle-specific validation

## Thread Safety

The validator is safe for concurrent use. Each validation creates its own context and result, so multiple goroutines can call `Validate()` simultaneously.

## Memory Management

Use `AcquireResult()` and `Release()` for better performance in high-throughput scenarios:

```go
// Manual pool management
result := fv.AcquireResult()
// ... use result
result.Release()

// Or let the validator handle it
result, _ := validator.Validate(ctx, resource)
defer result.Release()
```

## Pre-loaded Code Systems

The terminology service comes pre-loaded with common FHIR code systems:

- `http://hl7.org/fhir/administrative-gender`
- `http://hl7.org/fhir/contact-point-system`
- `http://hl7.org/fhir/contact-point-use`
- `http://hl7.org/fhir/observation-status`
- `http://hl7.org/fhir/bundle-type`
- `http://hl7.org/fhir/http-verb`
- And more...

## License

MIT License - see LICENSE file for details.
