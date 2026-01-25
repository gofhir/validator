// Package service defines small, composable interfaces for validation services.
// Following Go's philosophy of small interfaces, each interface has 1-2 methods.
package service

import (
	"context"
)

// StructureDefinition represents a FHIR StructureDefinition.
// This is a simplified internal representation.
type StructureDefinition struct {
	URL            string
	Name           string
	Type           string
	Kind           string
	Abstract       bool
	BaseDefinition string
	FHIRVersion    string
	Snapshot       []ElementDefinition
	Differential   []ElementDefinition
	Context        []string // Context paths where extension can be used
	IsModifier     bool     // True if this is a modifier extension
}

// ElementDefinition represents a FHIR ElementDefinition.
type ElementDefinition struct {
	ID               string
	Path             string
	SliceName        string
	Min              int
	Max              string
	Types            []TypeRef
	Fixed            any
	Pattern          any
	Binding          *Binding
	Constraints      []Constraint
	MustSupport      bool
	IsModifier       bool
	IsSummary        bool
	Slicing          *Slicing
	ContentReference string // Reference to another element definition (e.g., "#CapabilityStatement.rest.resource.operation")
}

// TypeRef represents a type reference in an ElementDefinition.
type TypeRef struct {
	Code          string
	Profile       []string
	TargetProfile []string
}

// Binding represents a terminology binding.
type Binding struct {
	Strength    string
	ValueSet    string
	Description string
}

// Constraint represents a FHIRPath constraint.
type Constraint struct {
	Key        string
	Severity   string
	Human      string
	Expression string
	XPath      string
	Source     string
}

// Slicing represents element slicing rules.
type Slicing struct {
	Discriminator []Discriminator
	Description   string
	Ordered       bool
	Rules         string
}

// Discriminator defines how slices are differentiated.
type Discriminator struct {
	Type string
	Path string
}

// --- Small Interfaces (Go idiom: 1-2 methods per interface) ---

// StructureDefinitionFetcher fetches StructureDefinitions by URL.
type StructureDefinitionFetcher interface {
	FetchStructureDefinition(ctx context.Context, url string) (*StructureDefinition, error)
}

// StructureDefinitionByTypeFetcher fetches StructureDefinitions by resource type.
type StructureDefinitionByTypeFetcher interface {
	FetchStructureDefinitionByType(ctx context.Context, resourceType string) (*StructureDefinition, error)
}

// SnapshotGenerator generates snapshots for differential-only profiles.
type SnapshotGenerator interface {
	GenerateSnapshot(ctx context.Context, profile *StructureDefinition) (*StructureDefinition, error)
}

// ProfileResolver resolves profile URLs to StructureDefinitions.
// This is a common combined interface.
type ProfileResolver interface {
	StructureDefinitionFetcher
	StructureDefinitionByTypeFetcher
}

// FullProfileService combines all profile-related functionality.
type FullProfileService interface {
	ProfileResolver
	SnapshotGenerator
}

// ProfileCache caches resolved profiles.
type ProfileCache interface {
	Get(url string) (*StructureDefinition, bool)
	Set(url string, profile *StructureDefinition)
}
