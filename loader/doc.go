// Package loader provides functionality to load FHIR resources and convert them
// to the internal service models used by the validator.
//
// The loader package bridges the gap between the full FHIR models (r4, r5)
// and the simplified internal models used by the validator phases.
//
// Key components:
//   - Converter: Converts r4/r5 StructureDefinition to service.StructureDefinition
//   - ProfileLoader: Loads StructureDefinitions from various sources
//   - TerminologyLoader: Loads ValueSets and CodeSystems
//
// Example usage:
//
//	// Create an R4 converter
//	converter := loader.NewR4Converter()
//
//	// Convert a StructureDefinition
//	serviceDef, err := converter.ConvertStructureDefinition(r4StructDef)
//
//	// Create a profile service from loaded definitions
//	profileService := loader.NewInMemoryProfileService(converter, structDefs...)
package loader
