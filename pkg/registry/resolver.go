package registry

import "context"

// ProfileResolver resolves StructureDefinitions on demand from an external source.
// This enables the validator to fetch profiles not pre-loaded in the registry,
// supporting version-aware lookup and on-demand loading from databases or APIs.
//
// This interface is optional. When not configured, the validator works exclusively
// with pre-loaded StructureDefinitions (standalone mode). When configured by a
// FHIR server, it enables dynamic profile resolution (server mode).
//
// This follows the same pattern as terminology.Provider: optional, fail-open,
// and implementation-agnostic. Comparable to HAPI FHIR's IValidationSupport,
// HL7 Validator's IWorkerContext.fetchResource, and Firely's IResourceResolver.
type ProfileResolver interface {
	// ResolveProfile fetches a StructureDefinition by canonical URL and optional version.
	// When version is empty, the resolver SHOULD return the latest available version
	// (per FHIR spec: "If no version is specified, the server SHOULD return the
	// latest version it has").
	// Returns (nil, nil) if the profile is not found (not an error condition).
	// Returns (nil, error) on resolver failure (network, DB, etc.).
	ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
}
