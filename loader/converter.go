package loader

import (
	"github.com/gofhir/fhir/r4"
	"github.com/gofhir/validator/service"
)

// R4Converter converts R4 FHIR models to internal service models.
type R4Converter struct{}

// NewR4Converter creates a new R4 converter.
func NewR4Converter() *R4Converter {
	return &R4Converter{}
}

// ConvertStructureDefinition converts an r4.StructureDefinition to service.StructureDefinition.
func (c *R4Converter) ConvertStructureDefinition(sd *r4.StructureDefinition) *service.StructureDefinition {
	if sd == nil {
		return nil
	}

	result := &service.StructureDefinition{
		URL:            derefString(sd.Url),
		Name:           derefString(sd.Name),
		Type:           derefString(sd.Type),
		Kind:           c.convertKind(sd.Kind),
		Abstract:       derefBool(sd.Abstract),
		BaseDefinition: derefString(sd.BaseDefinition),
		FHIRVersion:    c.convertFHIRVersion(sd.FhirVersion),
		Context:        c.convertContext(sd.Context),
	}

	// Convert snapshot elements
	if sd.Snapshot != nil {
		result.Snapshot = c.convertElementDefinitions(sd.Snapshot.Element)
	}

	// Convert differential elements
	if sd.Differential != nil {
		result.Differential = c.convertElementDefinitions(sd.Differential.Element)
	}

	return result
}

// convertElementDefinitions converts a slice of r4.ElementDefinition to service.ElementDefinition.
func (c *R4Converter) convertElementDefinitions(elements []r4.ElementDefinition) []service.ElementDefinition {
	if len(elements) == 0 {
		return nil
	}

	result := make([]service.ElementDefinition, 0, len(elements))
	for i := range elements {
		result = append(result, c.convertElementDefinition(&elements[i]))
	}
	return result
}

// convertElementDefinition converts a single r4.ElementDefinition to service.ElementDefinition.
func (c *R4Converter) convertElementDefinition(ed *r4.ElementDefinition) service.ElementDefinition {
	result := service.ElementDefinition{
		ID:          derefString(ed.Id),
		Path:        derefString(ed.Path),
		SliceName:   derefString(ed.SliceName),
		Min:         c.convertMin(ed.Min),
		Max:         derefString(ed.Max),
		Types:       c.convertTypes(ed.Type),
		Binding:     c.convertBinding(ed.Binding),
		Constraints: c.convertConstraints(ed.Constraint),
		MustSupport: derefBool(ed.MustSupport),
		IsModifier:  derefBool(ed.IsModifier),
		IsSummary:   derefBool(ed.IsSummary),
		Slicing:     c.convertSlicing(ed.Slicing),
		Fixed:       c.extractFixedValue(ed),
		Pattern:     c.extractPatternValue(ed),
	}
	return result
}

// convertTypes converts r4.ElementDefinitionType slice to service.TypeRef slice.
func (c *R4Converter) convertTypes(types []r4.ElementDefinitionType) []service.TypeRef {
	if len(types) == 0 {
		return nil
	}

	result := make([]service.TypeRef, 0, len(types))
	for i := range types {
		t := &types[i]
		result = append(result, service.TypeRef{
			Code:          derefString(t.Code),
			Profile:       t.Profile,
			TargetProfile: t.TargetProfile,
		})
	}
	return result
}

// convertBinding converts r4.ElementDefinitionBinding to service.Binding.
func (c *R4Converter) convertBinding(binding *r4.ElementDefinitionBinding) *service.Binding {
	if binding == nil {
		return nil
	}

	return &service.Binding{
		Strength:    c.convertBindingStrength(binding.Strength),
		ValueSet:    derefString(binding.ValueSet),
		Description: derefString(binding.Description),
	}
}

// convertConstraints converts r4.ElementDefinitionConstraint slice to service.Constraint slice.
func (c *R4Converter) convertConstraints(constraints []r4.ElementDefinitionConstraint) []service.Constraint {
	if len(constraints) == 0 {
		return nil
	}

	result := make([]service.Constraint, 0, len(constraints))
	for i := range constraints {
		con := &constraints[i]
		result = append(result, service.Constraint{
			Key:        derefString(con.Key),
			Severity:   c.convertConstraintSeverity(con.Severity),
			Human:      derefString(con.Human),
			Expression: derefString(con.Expression),
			XPath:      derefString(con.Xpath),
			Source:     derefString(con.Source),
		})
	}
	return result
}

// convertSlicing converts r4.ElementDefinitionSlicing to service.Slicing.
func (c *R4Converter) convertSlicing(slicing *r4.ElementDefinitionSlicing) *service.Slicing {
	if slicing == nil {
		return nil
	}

	return &service.Slicing{
		Discriminator: c.convertDiscriminators(slicing.Discriminator),
		Description:   derefString(slicing.Description),
		Ordered:       derefBool(slicing.Ordered),
		Rules:         c.convertSlicingRules(slicing.Rules),
	}
}

// convertDiscriminators converts r4.ElementDefinitionSlicingDiscriminator slice.
func (c *R4Converter) convertDiscriminators(discriminators []r4.ElementDefinitionSlicingDiscriminator) []service.Discriminator {
	if len(discriminators) == 0 {
		return nil
	}

	result := make([]service.Discriminator, 0, len(discriminators))
	for i := range discriminators {
		d := &discriminators[i]
		result = append(result, service.Discriminator{
			Type: c.convertDiscriminatorType(d.Type),
			Path: derefString(d.Path),
		})
	}
	return result
}

// convertContext extracts context paths from StructureDefinitionContext.
func (c *R4Converter) convertContext(contexts []r4.StructureDefinitionContext) []string {
	if len(contexts) == 0 {
		return nil
	}

	result := make([]string, 0, len(contexts))
	for i := range contexts {
		if contexts[i].Expression != nil {
			result = append(result, *contexts[i].Expression)
		}
	}
	return result
}

// Type conversion helpers

func (c *R4Converter) convertKind(kind *r4.StructureDefinitionKind) string {
	if kind == nil {
		return ""
	}
	return string(*kind)
}

func (c *R4Converter) convertFHIRVersion(version *r4.FHIRVersion) string {
	if version == nil {
		return ""
	}
	return string(*version)
}

func (c *R4Converter) convertBindingStrength(strength *r4.BindingStrength) string {
	if strength == nil {
		return ""
	}
	return string(*strength)
}

func (c *R4Converter) convertConstraintSeverity(severity *r4.ConstraintSeverity) string {
	if severity == nil {
		return ""
	}
	return string(*severity)
}

func (c *R4Converter) convertSlicingRules(rules *r4.SlicingRules) string {
	if rules == nil {
		return ""
	}
	return string(*rules)
}

func (c *R4Converter) convertDiscriminatorType(dtype *r4.DiscriminatorType) string {
	if dtype == nil {
		return ""
	}
	return string(*dtype)
}

func (c *R4Converter) convertMin(minVal *uint32) int {
	if minVal == nil {
		return 0
	}
	return int(*minVal)
}

// primitiveValues holds pointers to primitive type values.
type primitiveValues struct {
	String    *string
	Boolean   *bool
	Integer   *int
	Decimal   *float64
	Code      *string
	URI       *string
	URL       *string
	Canonical *string
}

// complexValues holds pointers to complex type values.
type complexValues struct {
	Coding          *r4.Coding
	CodeableConcept *r4.CodeableConcept
	Identifier      *r4.Identifier
}

// extractPolymorphicValue extracts a value from primitive and complex type pointers.
func (c *R4Converter) extractPolymorphicValue(prim primitiveValues, comp complexValues) any {
	// Check primitive types first
	if prim.String != nil {
		return *prim.String
	}
	if prim.Boolean != nil {
		return *prim.Boolean
	}
	if prim.Integer != nil {
		return *prim.Integer
	}
	if prim.Decimal != nil {
		return *prim.Decimal
	}
	if prim.Code != nil {
		return *prim.Code
	}
	if prim.URI != nil {
		return *prim.URI
	}
	if prim.URL != nil {
		return *prim.URL
	}
	if prim.Canonical != nil {
		return *prim.Canonical
	}

	// Check complex types
	if comp.Coding != nil {
		return c.codingToMap(comp.Coding)
	}
	if comp.CodeableConcept != nil {
		return c.codeableConceptToMap(comp.CodeableConcept)
	}
	if comp.Identifier != nil {
		return c.identifierToMap(comp.Identifier)
	}

	return nil
}

// extractFixedValue extracts the fixed[x] value from ElementDefinition.
// Returns nil if no fixed value is set.
func (c *R4Converter) extractFixedValue(ed *r4.ElementDefinition) any {
	return c.extractPolymorphicValue(
		primitiveValues{
			String:    ed.FixedString,
			Boolean:   ed.FixedBoolean,
			Integer:   ed.FixedInteger,
			Decimal:   ed.FixedDecimal,
			Code:      ed.FixedCode,
			URI:       ed.FixedUri,
			URL:       ed.FixedUrl,
			Canonical: ed.FixedCanonical,
		},
		complexValues{
			Coding:          ed.FixedCoding,
			CodeableConcept: ed.FixedCodeableConcept,
			Identifier:      ed.FixedIdentifier,
		},
	)
}

// extractPatternValue extracts the pattern[x] value from ElementDefinition.
// Returns nil if no pattern value is set.
func (c *R4Converter) extractPatternValue(ed *r4.ElementDefinition) any {
	return c.extractPolymorphicValue(
		primitiveValues{
			String:    ed.PatternString,
			Boolean:   ed.PatternBoolean,
			Integer:   ed.PatternInteger,
			Decimal:   ed.PatternDecimal,
			Code:      ed.PatternCode,
			URI:       ed.PatternUri,
			URL:       ed.PatternUrl,
			Canonical: ed.PatternCanonical,
		},
		complexValues{
			Coding:          ed.PatternCoding,
			CodeableConcept: ed.PatternCodeableConcept,
			Identifier:      ed.PatternIdentifier,
		},
	)
}

// Helper functions to convert FHIR types to maps

func (c *R4Converter) codingToMap(coding *r4.Coding) map[string]any {
	if coding == nil {
		return nil
	}
	result := make(map[string]any)
	if coding.System != nil {
		result["system"] = *coding.System
	}
	if coding.Version != nil {
		result["version"] = *coding.Version
	}
	if coding.Code != nil {
		result["code"] = *coding.Code
	}
	if coding.Display != nil {
		result["display"] = *coding.Display
	}
	return result
}

func (c *R4Converter) codeableConceptToMap(cc *r4.CodeableConcept) map[string]any {
	if cc == nil {
		return nil
	}
	result := make(map[string]any)
	if len(cc.Coding) > 0 {
		codings := make([]any, 0, len(cc.Coding))
		for i := range cc.Coding {
			codings = append(codings, c.codingToMap(&cc.Coding[i]))
		}
		result["coding"] = codings
	}
	if cc.Text != nil {
		result["text"] = *cc.Text
	}
	return result
}

func (c *R4Converter) identifierToMap(id *r4.Identifier) map[string]any {
	if id == nil {
		return nil
	}
	result := make(map[string]any)
	if id.System != nil {
		result["system"] = *id.System
	}
	if id.Value != nil {
		result["value"] = *id.Value
	}
	if id.Use != nil {
		result["use"] = string(*id.Use)
	}
	return result
}

// Generic helpers

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
