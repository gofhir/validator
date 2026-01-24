package service

import (
	"context"
)

// Coding represents a FHIR Coding datatype.
type Coding struct {
	System       string
	Version      string
	Code         string
	Display      string
	UserSelected bool
}

// CodeableConcept represents a FHIR CodeableConcept datatype.
type CodeableConcept struct {
	Coding []Coding
	Text   string
}

// ValueSetExpansion represents an expanded ValueSet.
type ValueSetExpansion struct {
	URL       string
	Version   string
	Total     int
	Offset    int
	Contains  []ValueSetContains
	Parameter []ExpansionParameter
}

// ValueSetContains represents an item in a ValueSet expansion.
type ValueSetContains struct {
	System   string
	Version  string
	Code     string
	Display  string
	Abstract bool
	Inactive bool
	Contains []ValueSetContains
}

// ExpansionParameter represents a parameter used in expansion.
type ExpansionParameter struct {
	Name  string
	Value any
}

// CodeInfo holds information about a code lookup result.
type CodeInfo struct {
	System      string
	Code        string
	Display     string
	Definition  string
	Designation []Designation
	Properties  []Property
}

// Designation represents a code designation (translation, synonym, etc.).
type Designation struct {
	Language string
	Use      *Coding
	Value    string
}

// Property represents a code property.
type Property struct {
	Code  string
	Value any
}

// ValidateCodeResult holds the result of code validation.
type ValidateCodeResult struct {
	Valid   bool
	Message string
	Display string
	Code    string
	System  string
}

// --- Small Interfaces (Go idiom: 1-2 methods per interface) ---

// CodeValidator validates codes against ValueSets.
type CodeValidator interface {
	ValidateCode(ctx context.Context, system, code, valueSetURL string) (*ValidateCodeResult, error)
}

// CodingValidator validates Coding values against ValueSets.
type CodingValidator interface {
	ValidateCoding(ctx context.Context, coding *Coding, valueSetURL string) (*ValidateCodeResult, error)
}

// CodeableConceptValidator validates CodeableConcept values against ValueSets.
type CodeableConceptValidator interface {
	ValidateCodeableConcept(ctx context.Context, concept *CodeableConcept, valueSetURL string) (*ValidateCodeResult, error)
}

// ValueSetExpander expands ValueSets.
type ValueSetExpander interface {
	ExpandValueSet(ctx context.Context, url string) (*ValueSetExpansion, error)
}

// CodeLookup looks up code information.
type CodeLookup interface {
	LookupCode(ctx context.Context, system, code string) (*CodeInfo, error)
}

// ValueSetContainmentChecker checks if a code is in a ValueSet without full expansion.
type ValueSetContainmentChecker interface {
	Contains(ctx context.Context, system, code, valueSetURL string) (bool, error)
}

// TerminologyService combines common terminology operations.
// This is the typical interface used by the terminology validation phase.
type TerminologyService interface {
	CodeValidator
	ValueSetExpander
}

// FullTerminologyService includes all terminology functionality.
type FullTerminologyService interface {
	CodeValidator
	CodingValidator
	CodeableConceptValidator
	ValueSetExpander
	CodeLookup
	ValueSetContainmentChecker
}

// TerminologyCache caches terminology lookups.
type TerminologyCache interface {
	GetExpansion(url string) (*ValueSetExpansion, bool)
	SetExpansion(url string, expansion *ValueSetExpansion)
	GetValidation(key string) (*ValidateCodeResult, bool)
	SetValidation(key string, result *ValidateCodeResult)
}

// FHIRPathEvaluator evaluates FHIRPath expressions.
type FHIRPathEvaluator interface {
	// Evaluate evaluates a FHIRPath expression against a resource.
	// Returns true if the constraint is satisfied, false otherwise.
	Evaluate(ctx context.Context, expression string, resource any) (bool, error)
}
