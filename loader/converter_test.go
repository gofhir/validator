package loader

import (
	"testing"

	"github.com/gofhir/fhir/r4"
)

func TestR4Converter_ConvertStructureDefinition(t *testing.T) {
	converter := NewR4Converter()

	t.Run("nil input", func(t *testing.T) {
		result := converter.ConvertStructureDefinition(nil)
		if result != nil {
			t.Error("expected nil result for nil input")
		}
	})

	t.Run("basic conversion", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/TestPatient"
		name := "TestPatient"
		typeName := "Patient"
		kind := r4.StructureDefinitionKindResource
		abstract := false
		baseDef := "http://hl7.org/fhir/StructureDefinition/Patient"
		version := r4.FHIRVersion401

		sd := &r4.StructureDefinition{
			Url:            &url,
			Name:           &name,
			Type:           &typeName,
			Kind:           &kind,
			Abstract:       &abstract,
			BaseDefinition: &baseDef,
			FhirVersion:    &version,
		}

		result := converter.ConvertStructureDefinition(sd)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.URL != url {
			t.Errorf("URL = %q; want %q", result.URL, url)
		}
		if result.Name != name {
			t.Errorf("Name = %q; want %q", result.Name, name)
		}
		if result.Type != typeName {
			t.Errorf("Type = %q; want %q", result.Type, typeName)
		}
		if result.Kind != "resource" {
			t.Errorf("Kind = %q; want %q", result.Kind, "resource")
		}
		if result.Abstract != abstract {
			t.Errorf("Abstract = %v; want %v", result.Abstract, abstract)
		}
		if result.BaseDefinition != baseDef {
			t.Errorf("BaseDefinition = %q; want %q", result.BaseDefinition, baseDef)
		}
	})

	t.Run("with snapshot elements", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path1 := "Patient"
		path2 := "Patient.id"
		minCard := uint32(0)
		maxCard := "1"

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{Path: &path1},
					{Path: &path2, Min: &minCard, Max: &maxCard},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.Snapshot) != 2 {
			t.Fatalf("len(Snapshot) = %d; want 2", len(result.Snapshot))
		}
		if result.Snapshot[0].Path != path1 {
			t.Errorf("Snapshot[0].Path = %q; want %q", result.Snapshot[0].Path, path1)
		}
		if result.Snapshot[1].Path != path2 {
			t.Errorf("Snapshot[1].Path = %q; want %q", result.Snapshot[1].Path, path2)
		}
		if result.Snapshot[1].Min != 0 {
			t.Errorf("Snapshot[1].Min = %d; want 0", result.Snapshot[1].Min)
		}
		if result.Snapshot[1].Max != maxCard {
			t.Errorf("Snapshot[1].Max = %q; want %q", result.Snapshot[1].Max, maxCard)
		}
	})

	t.Run("with types", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path := "Patient.name"
		typeCode := "HumanName"
		profile := "http://example.org/StructureDefinition/CustomName"

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{
						Path: &path,
						Type: []r4.ElementDefinitionType{
							{
								Code:    &typeCode,
								Profile: []string{profile},
							},
						},
					},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if len(result.Snapshot) != 1 {
			t.Fatalf("len(Snapshot) = %d; want 1", len(result.Snapshot))
		}
		if len(result.Snapshot[0].Types) != 1 {
			t.Fatalf("len(Types) = %d; want 1", len(result.Snapshot[0].Types))
		}
		if result.Snapshot[0].Types[0].Code != typeCode {
			t.Errorf("Types[0].Code = %q; want %q", result.Snapshot[0].Types[0].Code, typeCode)
		}
		if len(result.Snapshot[0].Types[0].Profile) != 1 || result.Snapshot[0].Types[0].Profile[0] != profile {
			t.Errorf("Types[0].Profile = %v; want [%q]", result.Snapshot[0].Types[0].Profile, profile)
		}
	})

	t.Run("with binding", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path := "Patient.gender"
		strength := r4.BindingStrengthRequired
		valueSet := "http://hl7.org/fhir/ValueSet/administrative-gender"

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{
						Path: &path,
						Binding: &r4.ElementDefinitionBinding{
							Strength: &strength,
							ValueSet: &valueSet,
						},
					},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if result.Snapshot[0].Binding == nil {
			t.Fatal("expected non-nil Binding")
		}
		if result.Snapshot[0].Binding.Strength != "required" {
			t.Errorf("Binding.Strength = %q; want %q", result.Snapshot[0].Binding.Strength, "required")
		}
		if result.Snapshot[0].Binding.ValueSet != valueSet {
			t.Errorf("Binding.ValueSet = %q; want %q", result.Snapshot[0].Binding.ValueSet, valueSet)
		}
	})

	t.Run("with constraints", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path := "Patient"
		key := "pat-1"
		severity := r4.ConstraintSeverityError
		human := "Patient must have a name"
		expression := "name.exists()"

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{
						Path: &path,
						Constraint: []r4.ElementDefinitionConstraint{
							{
								Key:        &key,
								Severity:   &severity,
								Human:      &human,
								Expression: &expression,
							},
						},
					},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if len(result.Snapshot[0].Constraints) != 1 {
			t.Fatalf("len(Constraints) = %d; want 1", len(result.Snapshot[0].Constraints))
		}
		if result.Snapshot[0].Constraints[0].Key != key {
			t.Errorf("Constraints[0].Key = %q; want %q", result.Snapshot[0].Constraints[0].Key, key)
		}
		if result.Snapshot[0].Constraints[0].Severity != "error" {
			t.Errorf("Constraints[0].Severity = %q; want %q", result.Snapshot[0].Constraints[0].Severity, "error")
		}
		if result.Snapshot[0].Constraints[0].Human != human {
			t.Errorf("Constraints[0].Human = %q; want %q", result.Snapshot[0].Constraints[0].Human, human)
		}
		if result.Snapshot[0].Constraints[0].Expression != expression {
			t.Errorf("Constraints[0].Expression = %q; want %q", result.Snapshot[0].Constraints[0].Expression, expression)
		}
	})

	t.Run("with slicing", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path := "Patient.identifier"
		discType := r4.DiscriminatorTypeValue
		discPath := "system"
		rules := r4.SlicingRulesClosed
		ordered := true

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{
						Path: &path,
						Slicing: &r4.ElementDefinitionSlicing{
							Discriminator: []r4.ElementDefinitionSlicingDiscriminator{
								{Type: &discType, Path: &discPath},
							},
							Rules:   &rules,
							Ordered: &ordered,
						},
					},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if result.Snapshot[0].Slicing == nil {
			t.Fatal("expected non-nil Slicing")
		}
		if len(result.Snapshot[0].Slicing.Discriminator) != 1 {
			t.Fatalf("len(Discriminator) = %d; want 1", len(result.Snapshot[0].Slicing.Discriminator))
		}
		if result.Snapshot[0].Slicing.Discriminator[0].Type != "value" {
			t.Errorf("Discriminator[0].Type = %q; want %q", result.Snapshot[0].Slicing.Discriminator[0].Type, "value")
		}
		if result.Snapshot[0].Slicing.Discriminator[0].Path != discPath {
			t.Errorf("Discriminator[0].Path = %q; want %q", result.Snapshot[0].Slicing.Discriminator[0].Path, discPath)
		}
		if result.Snapshot[0].Slicing.Rules != "closed" {
			t.Errorf("Slicing.Rules = %q; want %q", result.Snapshot[0].Slicing.Rules, "closed")
		}
		if !result.Snapshot[0].Slicing.Ordered {
			t.Error("Slicing.Ordered = false; want true")
		}
	})

	t.Run("with fixed value", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path := "Patient.identifier.system"
		fixedURI := "http://example.org/identifiers"

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{
						Path:     &path,
						FixedUri: &fixedURI,
					},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if result.Snapshot[0].Fixed == nil {
			t.Fatal("expected non-nil Fixed")
		}
		if result.Snapshot[0].Fixed != fixedURI {
			t.Errorf("Fixed = %v; want %q", result.Snapshot[0].Fixed, fixedURI)
		}
	})

	t.Run("with pattern coding", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/Test"
		path := "Patient.identifier.type"
		system := "http://terminology.hl7.org/CodeSystem/v2-0203"
		code := "MR"

		sd := &r4.StructureDefinition{
			Url: &url,
			Snapshot: &r4.StructureDefinitionSnapshot{
				Element: []r4.ElementDefinition{
					{
						Path: &path,
						PatternCoding: &r4.Coding{
							System: &system,
							Code:   &code,
						},
					},
				},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if result.Snapshot[0].Pattern == nil {
			t.Fatal("expected non-nil Pattern")
		}
		patternMap, ok := result.Snapshot[0].Pattern.(map[string]any)
		if !ok {
			t.Fatalf("Pattern is not map[string]any, got %T", result.Snapshot[0].Pattern)
		}
		if patternMap["system"] != system {
			t.Errorf("Pattern.system = %v; want %q", patternMap["system"], system)
		}
		if patternMap["code"] != code {
			t.Errorf("Pattern.code = %v; want %q", patternMap["code"], code)
		}
	})

	t.Run("with context", func(t *testing.T) {
		url := "http://example.org/StructureDefinition/MyExtension"
		contextType := r4.ExtensionContextTypeElement
		contextExpr := "Patient"

		sd := &r4.StructureDefinition{
			Url: &url,
			Context: []r4.StructureDefinitionContext{
				{Type: &contextType, Expression: &contextExpr},
			},
		}

		result := converter.ConvertStructureDefinition(sd)

		if len(result.Context) != 1 {
			t.Fatalf("len(Context) = %d; want 1", len(result.Context))
		}
		if result.Context[0] != contextExpr {
			t.Errorf("Context[0] = %q; want %q", result.Context[0], contextExpr)
		}
	})
}

func TestDerefString(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if result := derefString(nil); result != "" {
			t.Errorf("derefString(nil) = %q; want \"\"", result)
		}
	})

	t.Run("non-nil", func(t *testing.T) {
		s := "test"
		if result := derefString(&s); result != "test" {
			t.Errorf("derefString(&\"test\") = %q; want \"test\"", result)
		}
	})
}

func TestDerefBool(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if result := derefBool(nil); result != false {
			t.Errorf("derefBool(nil) = %v; want false", result)
		}
	})

	t.Run("true", func(t *testing.T) {
		b := true
		if result := derefBool(&b); result != true {
			t.Errorf("derefBool(&true) = %v; want true", result)
		}
	})

	t.Run("false", func(t *testing.T) {
		b := false
		if result := derefBool(&b); result != false {
			t.Errorf("derefBool(&false) = %v; want false", result)
		}
	})
}
