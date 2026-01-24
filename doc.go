// Package fhirvalidator provides high-performance FHIR resource validation.
//
// This package is designed from the ground up to leverage Go's strengths:
// concurrency with goroutines, sync.Pool for memory efficiency, generics
// for type-safe caches, and small composable interfaces.
//
// # Quick Start
//
//	import (
//	    fv "github.com/gofhir/validator"
//	    "github.com/gofhir/validator/engine"
//	)
//
//	validator, err := engine.New(ctx, fv.R4)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	result, err := validator.Validate(ctx, resourceJSON)
//	if result.HasErrors() {
//	    for _, issue := range result.Errors() {
//	        fmt.Println(issue.Diagnostics)
//	    }
//	}
//	result.Release() // Return to pool for better performance
//
// # Performance Features
//
//   - Worker Pool: Parallel batch validation using runtime.NumCPU() workers
//   - Parallel Phases: Independent validation phases run concurrently
//   - sync.Pool: Reduces GC pressure by 60-80% through object reuse
//   - Generic Cache: Type-safe LRU caches without interface{} overhead
//   - Streaming: Validate large bundles without loading into memory
//
// # Functional Options
//
//	validator, err := engine.New(ctx, fv.R4,
//	    fv.WithTerminology(true),
//	    fv.WithParallelPhases(true),
//	    fv.WithWorkerCount(runtime.NumCPU()),
//	    fv.WithMaxErrors(100),
//	)
//
// # Validation Phases
//
// Validation is performed in phases, each handling one aspect of FHIR:
//
//   - Structure: Cardinality, required elements, unknown elements
//   - Primitives: Type format validation (regex patterns)
//   - Slicing: Discriminator matching and slice cardinality
//   - Constraints: FHIRPath invariant evaluation
//   - Terminology: ValueSet binding validation
//   - References: Reference resolution and type checking
//   - Extensions: Extension URL and context validation
//   - Bundle: Bundle-specific rules (bdl-* constraints)
//
// # Architecture
//
// The package follows patterns from HAPI FHIR and Firely, adapted for Go:
//
//   - Small interfaces (1-2 methods each) for composability
//   - Chain of responsibility for service resolution
//   - Pipeline pattern for phase execution
//   - Context-based cancellation and timeout
package fhirvalidator
