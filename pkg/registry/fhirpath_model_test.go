package registry

import (
	"slices"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
)

// The model answers from the loaded definitions, in every version.
func TestFHIRPathModel(t *testing.T) {
	for _, version := range []string{"4.0.1", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			m := loadVersion(t, version).FHIRPathModel()
			if got := m.FHIRVersion(); got != version {
				t.Errorf("FHIRVersion = %q, want %q", got, version)
			}
			for path, want := range map[string]string{
				"Patient.name":              "HumanName",
				"Patient.birthDate":         "date",
				"Observation.valueQuantity": "Quantity", // a choice, named as an instance names it
				"Bundle.entry.resource":     "Resource",
				"Observation.value[x]":      "", // the choice itself has no one type
				"Patient.nothing":           "",
			} {
				if got := m.TypeOf(path); got != want {
					t.Errorf("TypeOf(%s) = %q, want %q", path, got, want)
				}
			}
			if got := m.ChoiceTypes("Observation.value"); !slices.Contains(got, "Quantity") || !slices.Contains(got, "string") {
				t.Errorf("ChoiceTypes(Observation.value) = %v", got)
			}
			if got := m.ResolvePath("Questionnaire.item.item"); got != "Questionnaire.item" {
				t.Errorf("ResolvePath(Questionnaire.item.item) = %q", got)
			}
			if got := m.ResolvePath("Patient.name"); got != "Patient.name" {
				t.Errorf("ResolvePath(Patient.name) = %q", got)
			}
			if got := m.ReferenceTargets("Observation.subject"); !slices.Contains(got, "Patient") {
				t.Errorf("ReferenceTargets(Observation.subject) = %v", got)
			}
			for path, targets := range m.targets {
				if len(slices.Compact(slices.Sorted(slices.Values(targets)))) != len(targets) {
					t.Errorf("ReferenceTargets(%s) = %v, repeats a type", path, targets)
				}
			}
			if got := m.ParentType("Patient"); got != "DomainResource" {
				t.Errorf("ParentType(Patient) = %q", got)
			}
			if !m.IsSubtype("Patient", "Resource") || m.IsSubtype("Resource", "Patient") {
				t.Error("IsSubtype does not follow baseDefinition")
			}
			if !m.IsResource("Patient") || m.IsResource("HumanName") {
				t.Error("IsResource does not follow kind")
			}
			if !m.HasType("HumanName") || !m.HasType("string") || m.HasType("string1") {
				t.Error("HasType does not follow the definitions")
			}
		})
	}
}

// A logical model is not a type an instance can have.
func TestFHIRPathModelLeavesOutLogicalModels(t *testing.T) {
	r := loadVersion(t, "4.0.1")
	sd := r.GetByType("FiveWs")
	if sd == nil || sd.Kind != KindLogical {
		t.Skip("no logical model FiveWs in this package")
	}
	if r.FHIRPathModel().HasType("FiveWs") {
		t.Error("HasType(FiveWs) = true for a logical model")
	}
}

// The engine, given the model, applies what only a model can tell it (plan B, PR B8).
func TestFHIRPathModelInTheEngine(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"1111","gender":"male","name":[{"family":"a"},{"family":"b"}],
"contained":[{"resourceType":"Patient","id":"2020"}]}`)
	models := map[string]*FHIRPathModel{}
	for _, version := range []string{"4.0.1", "5.0.0"} {
		models[version] = loadVersion(t, version).FHIRPathModel()
	}
	for _, tt := range []struct {
		version, expr, want string
		wantErr             bool
	}{
		{"4.0.1", "'#' + contained.id", "#2020", false},     // an id is a string, not a year
		{"4.0.1", "gender.as(string1)", "", true},           // a name that is no type
		{"4.0.1", "gender.as(code)", "male", false},         // the declared type
		{"4.0.1", "name.as(HumanName).count()", "2", false}, // before R5, as filters
		{"5.0.0", "name.as(HumanName).count()", "", true},   // from R5, as takes one item
	} {
		t.Run(tt.version+" "+tt.expr, func(t *testing.T) {
			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			ctx := eval.NewContext(patient)
			ctx.SetModel(models[tt.version])
			got, err := expr.EvaluateWithContext(ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && (len(got) != 1 || got[0].String() != tt.want) {
				t.Errorf("got %v, want %s", got, tt.want)
			}
		})
	}
}
