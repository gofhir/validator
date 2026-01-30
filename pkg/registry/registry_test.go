package registry

import (
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

func TestNewRegistry(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.Count() != 0 {
		t.Errorf("New registry should be empty, got %d", r.Count())
	}
}

func TestRegistryLoadFromPackages(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	count := r.Count()
	if count == 0 {
		t.Error("Registry should have loaded StructureDefinitions")
	}
	t.Logf("Loaded %d StructureDefinitions", count)

	typeCount := r.TypeCount()
	t.Logf("Indexed %d types", typeCount)
}

func TestRegistryGetByURL(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	// Test getting Patient StructureDefinition
	patientURL := "http://hl7.org/fhir/StructureDefinition/Patient"
	sd := r.GetByURL(patientURL)
	if sd == nil {
		t.Fatalf("GetByURL(%q) returned nil", patientURL)
	}

	if sd.Type != "Patient" {
		t.Errorf("Patient SD Type = %q, want %q", sd.Type, "Patient")
	}
	if sd.Kind != "resource" {
		t.Errorf("Patient SD Kind = %q, want %q", sd.Kind, "resource")
	}
	if sd.BaseDefinition != "http://hl7.org/fhir/StructureDefinition/DomainResource" {
		t.Errorf("Patient SD BaseDefinition = %q, want DomainResource", sd.BaseDefinition)
	}

	t.Logf("Patient SD: Kind=%s, Type=%s, Base=%s", sd.Kind, sd.Type, sd.BaseDefinition)
}

func TestRegistryGetByType(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	tests := []struct {
		typeName     string
		expectedKind string
	}{
		{"Patient", "resource"},
		{"Observation", "resource"},
		{"HumanName", "complex-type"},
		{"Identifier", "complex-type"},
		{"Coding", "complex-type"},
		{"string", "primitive-type"},
		{"boolean", "primitive-type"},
		{"date", "primitive-type"},
	}

	for _, tt := range tests {
		sd := r.GetByType(tt.typeName)
		if sd == nil {
			t.Errorf("GetByType(%q) returned nil", tt.typeName)
			continue
		}
		if sd.Kind != tt.expectedKind {
			t.Errorf("GetByType(%q).Kind = %q, want %q", tt.typeName, sd.Kind, tt.expectedKind)
		}
	}
}

func TestRegistryGetElementDefinition(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	tests := []struct {
		path        string
		expectMin   uint32
		expectMax   string
		expectTypes []string
	}{
		{"Patient.name", 0, "*", []string{"HumanName"}},
		{"Patient.birthDate", 0, "1", []string{"date"}},
		{"Patient.gender", 0, "1", []string{"code"}},
		{"Patient.active", 0, "1", []string{"boolean"}},
		{"Observation.status", 1, "1", []string{"code"}},
		{"Observation.code", 1, "1", []string{"CodeableConcept"}},
	}

	for _, tt := range tests {
		ed := r.GetElementDefinition(tt.path)
		if ed == nil {
			t.Errorf("GetElementDefinition(%q) returned nil", tt.path)
			continue
		}

		if ed.Min != tt.expectMin {
			t.Errorf("ElementDefinition(%q).Min = %d, want %d", tt.path, ed.Min, tt.expectMin)
		}
		if ed.Max != tt.expectMax {
			t.Errorf("ElementDefinition(%q).Max = %q, want %q", tt.path, ed.Max, tt.expectMax)
		}

		// Check types
		if len(ed.Type) != len(tt.expectTypes) {
			t.Errorf("ElementDefinition(%q) has %d types, want %d", tt.path, len(ed.Type), len(tt.expectTypes))
		} else {
			for i, expectedType := range tt.expectTypes {
				if ed.Type[i].Code != expectedType {
					t.Errorf("ElementDefinition(%q).Type[%d].Code = %q, want %q",
						tt.path, i, ed.Type[i].Code, expectedType)
				}
			}
		}

		t.Logf("%s: min=%d, max=%s, types=%v", tt.path, ed.Min, ed.Max, getTypeCodes(ed.Type))
	}
}

func TestRegistryElementDefinitionBinding(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	// Patient.gender has a required binding
	ed := r.GetElementDefinition("Patient.gender")
	if ed == nil {
		t.Fatal("GetElementDefinition(Patient.gender) returned nil")
	}

	if ed.Binding == nil {
		t.Fatal("Patient.gender should have a binding")
	}

	if ed.Binding.Strength != "required" {
		t.Errorf("Patient.gender binding strength = %q, want %q", ed.Binding.Strength, "required")
	}

	t.Logf("Patient.gender binding: strength=%s, valueSet=%s", ed.Binding.Strength, ed.Binding.ValueSet)
}

func TestRegistryElementDefinitionConstraints(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	// Patient.contact has constraints
	ed := r.GetElementDefinition("Patient.contact")
	if ed == nil {
		t.Fatal("GetElementDefinition(Patient.contact) returned nil")
	}

	if len(ed.Constraint) == 0 {
		t.Fatal("Patient.contact should have constraints")
	}

	// Look for pat-1 constraint
	var found bool
	for _, c := range ed.Constraint {
		t.Logf("Constraint: key=%s, severity=%s, human=%s", c.Key, c.Severity, c.Human)
		if c.Key == "pat-1" {
			found = true
			if c.Severity != "error" {
				t.Errorf("pat-1 severity = %q, want %q", c.Severity, "error")
			}
		}
	}

	if !found {
		t.Error("Patient.contact should have pat-1 constraint")
	}
}

func getTypeCodes(types []Type) []string {
	codes := make([]string, len(types))
	for i, t := range types {
		codes[i] = t.Code
	}
	return codes
}

// Tests for type classification methods derived from StructureDefinitions.

func TestRegistryIsPrimitiveType(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	primitives := []string{
		"string", "boolean", "integer", "decimal", "uri", "url",
		"code", "date", "dateTime", "time", "instant", "base64Binary",
		"id", "markdown", "oid", "positiveInt", "unsignedInt", "uuid",
	}

	for _, typeName := range primitives {
		if !r.IsPrimitiveType(typeName) {
			t.Errorf("IsPrimitiveType(%q) = false, want true", typeName)
		}
	}

	nonPrimitives := []string{
		"Patient", "HumanName", "Coding", "CodeableConcept", "Identifier",
	}

	for _, typeName := range nonPrimitives {
		if r.IsPrimitiveType(typeName) {
			t.Errorf("IsPrimitiveType(%q) = true, want false", typeName)
		}
	}
}

func TestRegistryIsDataType(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	dataTypes := []string{
		"HumanName", "Address", "ContactPoint", "Identifier",
		"CodeableConcept", "Coding", "Quantity", "Range", "Ratio",
		"Period", "Attachment", "Reference", "Annotation", "Signature",
		"Timing", "Meta", "Dosage", "ContactDetail", "Contributor",
	}

	for _, typeName := range dataTypes {
		if !r.IsDataType(typeName) {
			t.Errorf("IsDataType(%q) = false, want true", typeName)
		}
	}

	nonDataTypes := []string{
		"Patient", "Observation", "string", "boolean",
	}

	for _, typeName := range nonDataTypes {
		if r.IsDataType(typeName) {
			t.Errorf("IsDataType(%q) = true, want false", typeName)
		}
	}
}

func TestRegistryIsDomainResource(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	domainResources := []string{
		"Patient", "Observation", "Encounter", "MedicationRequest",
		"Condition", "Procedure", "DiagnosticReport", "ValueSet",
	}

	for _, typeName := range domainResources {
		if !r.IsDomainResource(typeName) {
			t.Errorf("IsDomainResource(%q) = false, want true", typeName)
		}
	}

	nonDomainResources := []string{
		"Bundle", "Binary", "Parameters",
	}

	for _, typeName := range nonDomainResources {
		if r.IsDomainResource(typeName) {
			t.Errorf("IsDomainResource(%q) = true, want false", typeName)
		}
	}

	// Non-resources should return false
	nonResources := []string{
		"HumanName", "string", "Coding",
	}

	for _, typeName := range nonResources {
		if r.IsDomainResource(typeName) {
			t.Errorf("IsDomainResource(%q) = true, want false (not a resource)", typeName)
		}
	}
}

func TestRegistryIsCanonicalResource(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	// Canonical resources have required 'url' element
	canonicalResources := []string{
		"StructureDefinition", "ValueSet", "CodeSystem", "ConceptMap",
		"CapabilityStatement", "OperationDefinition", "SearchParameter",
		"Questionnaire", "Library", "Measure", "PlanDefinition",
	}

	for _, typeName := range canonicalResources {
		if !r.IsCanonicalResource(typeName) {
			t.Errorf("IsCanonicalResource(%q) = false, want true", typeName)
		}
	}

	nonCanonicalResources := []string{
		"Patient", "Observation", "Encounter", "Bundle",
	}

	for _, typeName := range nonCanonicalResources {
		if r.IsCanonicalResource(typeName) {
			t.Errorf("IsCanonicalResource(%q) = true, want false", typeName)
		}
	}
}

func TestRegistryIsMetadataResource(t *testing.T) {
	l := loader.NewLoader("")
	packages, err := l.LoadVersion("4.0.1")
	if err != nil {
		t.Skipf("Cannot load FHIR packages: %v", err)
	}

	r := New()
	if err := r.LoadFromPackages(packages); err != nil {
		t.Fatalf("LoadFromPackages failed: %v", err)
	}

	// MetadataResources have url + name + status + experimental
	metadataResources := []string{
		"ValueSet", "CodeSystem", "Library", "Questionnaire",
		"Measure", "PlanDefinition", "ActivityDefinition",
	}

	for _, typeName := range metadataResources {
		if !r.IsMetadataResource(typeName) {
			t.Errorf("IsMetadataResource(%q) = false, want true", typeName)
		}
	}

	// Patient is definitely not a metadata resource
	if r.IsMetadataResource("Patient") {
		t.Error("IsMetadataResource(Patient) = true, want false")
	}
}
