package service

import (
	"context"
)

// Reference represents a FHIR Reference datatype.
type Reference struct {
	Reference  string
	Type       string
	Identifier *Identifier
	Display    string
}

// Identifier represents a FHIR Identifier datatype.
type Identifier struct {
	System string
	Value  string
	Use    string
	Type   *CodeableConcept
}

// ResolvedReference holds the result of resolving a reference.
type ResolvedReference struct {
	// Found indicates if the reference was resolved
	Found bool

	// Resource is the resolved resource (as map[string]any)
	Resource map[string]any

	// ResourceType is the type of the resolved resource
	ResourceType string

	// ResourceID is the ID of the resolved resource
	ResourceID string

	// URL is the full URL of the resolved resource
	URL string

	// Error contains any resolution error
	Error error
}

// ReferenceType indicates the type of reference.
type ReferenceType string

const (
	// ReferenceTypeRelative is a relative reference (e.g., "Patient/123")
	ReferenceTypeRelative ReferenceType = "relative"

	// ReferenceTypeAbsolute is an absolute URL reference
	ReferenceTypeAbsolute ReferenceType = "absolute"

	// ReferenceTypeLogical is a logical reference (identifier-based)
	ReferenceTypeLogical ReferenceType = "logical"

	// ReferenceTypeContained references a contained resource
	ReferenceTypeContained ReferenceType = "contained"

	// ReferenceTypeBundleEntry references a bundle entry by fullUrl
	ReferenceTypeBundleEntry ReferenceType = "bundle"

	// ReferenceTypeCanonical is a canonical URL reference
	ReferenceTypeCanonical ReferenceType = "canonical"
)

// --- Small Interfaces (Go idiom: 1-2 methods per interface) ---

// ReferenceResolver resolves FHIR references to resources.
type ReferenceResolver interface {
	ResolveReference(ctx context.Context, reference string) (*ResolvedReference, error)
}

// ReferenceTypeResolver resolves references with type hints.
type ReferenceTypeResolver interface {
	ResolveReferenceWithType(ctx context.Context, reference string, targetType string) (*ResolvedReference, error)
}

// ContainedResolver resolves contained resources within a parent.
type ContainedResolver interface {
	ResolveContained(ctx context.Context, parentResource map[string]any, reference string) (*ResolvedReference, error)
}

// BundleResolver resolves references within a bundle.
type BundleResolver interface {
	ResolveBundleReference(ctx context.Context, bundleEntries []map[string]any, reference string) (*ResolvedReference, error)
}

// ReferenceParser parses reference strings.
type ReferenceParser interface {
	ParseReference(reference string) (*ParsedReference, error)
}

// ParsedReference holds parsed components of a reference.
type ParsedReference struct {
	Type          ReferenceType
	ResourceType  string
	ResourceID    string
	VersionID     string
	Fragment      string
	FullURL       string
	Identifier    *Identifier
	CanonicalURL  string
	CanonicalVer  string
	IsContained   bool
	OriginalValue string
}

// ReferenceValidator validates reference formats.
type ReferenceValidator interface {
	ValidateReferenceFormat(reference string) error
}

// FullReferenceService combines all reference-related functionality.
type FullReferenceService interface {
	ReferenceResolver
	ReferenceTypeResolver
	ContainedResolver
	BundleResolver
	ReferenceParser
}

// ReferenceCache caches resolved references.
type ReferenceCache interface {
	Get(reference string) (*ResolvedReference, bool)
	Set(reference string, resolved *ResolvedReference)
}
