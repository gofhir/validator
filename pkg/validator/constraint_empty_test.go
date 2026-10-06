package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// An invariant whose result is empty does not hold: it "must evaluate to true when run on the
// element" (conformance-rules.html#constraints), as the HL7 validator converts it (plan C, C-2).
// The published invariants that are empty where they should not apply are corrected to their later
// publications, and a primitive with no value is checked against its definitions' invariants.
func TestEmptyInvariantFails(t *testing.T) {
	const dir = "../../testdata/m12-slice-scoping"
	sources, err := filepath.Glob(filepath.Join(dir, "packages", "src", "acme.invariants", "package", "StructureDefinition-*.json"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("definitions: %v", err)
	}
	defs := make([][]byte, 0, len(sources))
	for _, f := range sources {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, b)
	}
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<severity> <diagnostic> @ <location>" of the errors and failed invariants
	}{
		{"r04_entry_other", []string{"error CONSTRAINT_FAILED @ Bundle.entry[0].resource.hasMember[0]"}},               // resolve() is empty
		{"r06_top_unres", []string{"error CONSTRAINT_FAILED @ Observation.hasMember[0]"}},                              // resolve() is empty
		{"v16_prim_arr_no_value", []string{"error CONSTRAINT_FAILED @ MolecularSequence.quality[0].roc.precision[0]"}}, // no value
		{"v18_primitive_id_only", []string{"error CONSTRAINT_FAILED @ Patient.name[0].family"}},                        // ele-1
		{"e01_logical_reference", nil},         // ref-1 applies where there is a reference
		{"e02_entry_no_fullurl", nil},          // bdl-8 applies where there is a fullUrl
		{"e03_prediction_no_probability", nil}, // ras-2 applies where there is a probability
		{"e04_valueset_no_name", nil},          // vsd-0, a warning, where there is a name
		{"e05_two_enablewhen", []string{"error CONSTRAINT_FAILED @ Questionnaire.item[1]"}}, // que-12 from two
		{"e06_two_when_offset", nil},             // tim-9 tests each when
		{"e07_condition_no_clinicalstatus", nil}, // con-3 tests the category's coding
		{"e08_when_c_with_offset", []string{"error CONSTRAINT_FAILED @ MedicationRequest.dosageInstruction[0].timing.repeat"}}, // tim-9 still fails
		{"e09_problem_confirmed_no_clinicalstatus", []string{"warning CONSTRAINT_FAILED @ Condition"}},                         // con-3 still applies
		{"e10_two_enablewhen_behavior", nil},                                        // que-12 is met
		{"e11_valueset_bad_name", []string{"warning CONSTRAINT_FAILED @ ValueSet"}}, // vsd-0 still applies to a name
		{"e12_fragment_not_contained", []string{"error CONSTRAINT_FAILED @ Observation.subject", "error REFERENCE_CONTAINED_NOT_FOUND @ Observation.subject.reference"}}, // ref-1 still applies to a reference
		{"e13_vitals_effective_period", nil}, // vs-1 applies to a dateTime
		{"e14_vitals_month_only", []string{"error CONSTRAINT_FAILED @ Observation.effectiveDateTime"}}, // vs-1 still applies to one
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "probes", "ci_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				// dom-6 asks the probes, which have no narrative, for one.
				failed := is.MessageID == "CONSTRAINT_FAILED" && !strings.Contains(is.Diagnostics, "dom-6:")
				if is.Severity == "error" || failed {
					got = append(got, string(is.Severity)+" "+is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// R4B's ref-1 is corrected as R4's is: a logical reference is no fragment to resolve.
func TestR4BLogicalReference(t *testing.T) {
	v, err := New(WithVersion("4.3.0"))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},
"subject":{"identifier":{"system":"http://s","value":"1"}},"performer":[{"display":"Dr X"}]}`)
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range res.Issues {
		if is.Severity == "error" {
			t.Errorf("%s @ %v: %s", is.MessageID, is.Expression, is.Diagnostics)
		}
	}
}

// A primitive with no value is of its element's type, where a profile's path below a data type is
// no path the model knows (Patient.name.family is a HumanName's family).
func TestValuelessPrimitiveOfItsElementType(t *testing.T) {
	const url = "https://example.org/fhir/StructureDefinition/family-typed"
	profile := []byte(`{"resourceType":"StructureDefinition","url":"` + url + `","name":"FamilyTyped","status":"active",
"fhirVersion":"4.0.1","kind":"resource","abstract":false,"type":"Patient","derivation":"constraint",
"baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","differential":{"element":[
 {"id":"Patient","path":"Patient"},
 {"id":"Patient.name.family","path":"Patient.name.family","constraint":[
  {"key":"ft-1","severity":"error","human":"a string","expression":"$this is string"}]}]}}`)
	v, err := New(WithVersion("4.0.1"), WithConformanceResources([][]byte{profile}))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"resourceType":"Patient","id":"p","meta":{"profile":["` + url + `"]},"name":[{"_family":
{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}}]}`)
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range res.Issues {
		if is.Severity == "error" {
			t.Errorf("%s @ %v: %s", is.MessageID, is.Expression, is.Diagnostics)
		}
	}
}
