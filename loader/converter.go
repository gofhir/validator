package loader

import (
	"github.com/gofhir/validator/service"
	"github.com/gofhir/fhir/r4"
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

func (c *R4Converter) convertMin(min *uint32) int {
	if min == nil {
		return 0
	}
	return int(*min)
}

// extractFixedValue extracts the fixed[x] value from ElementDefinition.
// Returns nil if no fixed value is set.
func (c *R4Converter) extractFixedValue(ed *r4.ElementDefinition) any {
	// Check primitive types first
	if ed.FixedString != nil {
		return *ed.FixedString
	}
	if ed.FixedBoolean != nil {
		return *ed.FixedBoolean
	}
	if ed.FixedInteger != nil {
		return *ed.FixedInteger
	}
	if ed.FixedDecimal != nil {
		return *ed.FixedDecimal
	}
	if ed.FixedCode != nil {
		return *ed.FixedCode
	}
	if ed.FixedUri != nil {
		return *ed.FixedUri
	}
	if ed.FixedUrl != nil {
		return *ed.FixedUrl
	}
	if ed.FixedCanonical != nil {
		return *ed.FixedCanonical
	}

	// Check complex types
	if ed.FixedCoding != nil {
		return c.codingToMap(ed.FixedCoding)
	}
	if ed.FixedCodeableConcept != nil {
		return c.codeableConceptToMap(ed.FixedCodeableConcept)
	}
	if ed.FixedIdentifier != nil {
		return c.identifierToMap(ed.FixedIdentifier)
	}

	return nil
}

// extractPatternValue extracts the pattern[x] value from ElementDefinition.
// Returns nil if no pattern value is set.
func (c *R4Converter) extractPatternValue(ed *r4.ElementDefinition) any {
	// Check primitive types first
	if ed.PatternString != nil {
		return *ed.PatternString
	}
	if ed.PatternBoolean != nil {
		return *ed.PatternBoolean
	}
	if ed.PatternInteger != nil {
		return *ed.PatternInteger
	}
	if ed.PatternDecimal != nil {
		return *ed.PatternDecimal
	}
	if ed.PatternCode != nil {
		return *ed.PatternCode
	}
	if ed.PatternUri != nil {
		return *ed.PatternUri
	}
	if ed.PatternUrl != nil {
		return *ed.PatternUrl
	}
	if ed.PatternCanonical != nil {
		return *ed.PatternCanonical
	}

	// Check complex types
	if ed.PatternCoding != nil {
		return c.codingToMap(ed.PatternCoding)
	}
	if ed.PatternCodeableConcept != nil {
		return c.codeableConceptToMap(ed.PatternCodeableConcept)
	}
	if ed.PatternIdentifier != nil {
		return c.identifierToMap(ed.PatternIdentifier)
	}

	return nil
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
