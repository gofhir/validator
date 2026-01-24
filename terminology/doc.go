// Package terminology provides implementations of terminology services
// for validating codes against ValueSets and CodeSystems.
//
// The package provides:
//   - InMemoryTerminologyService: Validates codes against locally loaded ValueSets
//   - CommonCodeSystems: Pre-loaded common code systems (yes/no, gender, etc.)
//
// Example usage:
//
//	// Create an in-memory terminology service
//	ts := terminology.NewInMemoryTerminologyService()
//
//	// Load a ValueSet
//	ts.LoadR4ValueSet(valueSet)
//
//	// Validate a code
//	result, err := ts.ValidateCode(ctx, "http://example.org", "code123", "http://example.org/vs")
package terminology
