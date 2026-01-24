// Package phase provides concrete validation phase implementations.
//
// Each phase validates one aspect of FHIR resources:
//   - structure: Validates elements exist and are allowed
//   - primitives: Validates primitive type formats (string, date, etc.)
//   - cardinality: Validates min/max cardinality constraints
//   - unknown: Detects unknown/unexpected elements
//   - fixedpattern: Validates fixed and pattern values
//   - terminology: Validates code bindings
//   - constraints: Evaluates FHIRPath constraints
//   - references: Validates reference targets
//   - extensions: Validates extension usage
//   - slicing: Validates slice matching
//   - bundle: Validates Bundle-specific rules
//
// Phases implement the pipeline.Phase interface and can be registered
// with a Pipeline for execution.
package phase
