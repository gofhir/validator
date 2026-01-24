// Package context provides the SpecContext which manages FHIR version-specific
// resources for validation.
//
// SpecContext automatically loads StructureDefinitions, CodeSystems, and ValueSets
// from the embedded FHIR specification files based on the selected FHIR version.
//
// Usage:
//
//	ctx := context.Background()
//	specCtx, err := fhircontext.New(ctx, fv.R4, fhircontext.Options{
//	    LoadTerminology: true,
//	})
//	if err != nil {
//	    return err
//	}
//
//	// Use specCtx.Profiles for profile resolution
//	// Use specCtx.Terminology for code validation
package context
