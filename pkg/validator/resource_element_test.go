package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// An element holds a resource when its definition says so (each of its types is a resource type),
// not when its value has a resourceType: ExampleScenario.instance.resourceType is a code, checked as
// one; a resource where a datatype is expected is unknown elements, not a resource; an entry a
// profile types as one of two resource types is a resource. The verdicts are the HL7 validator's.
func TestResourceElementsByDefinition(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.reselem")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<severity> <diagnostic> @ <location>"
	}{
		{"01_examplescenario_instance", []string{
			"error BINDING_REQUIRED @ ExampleScenario.instance[1].resourceType",
			"error CARDINALITY_MIN @ ExampleScenario.instance[0].resourceId",
		}},
		{"02_misplaced_resource", []string{
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation._status.resourceType",
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation.code.birthDate",
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation.code.resourceType",
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation.subject.birthDate",
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation.subject.gender",
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation.subject.resourceType",
		}},
		{"03_entry_two_resource_types", []string{
			"error STRUCTURE_UNKNOWN_ELEMENT @ Bundle.entry[0].resource.foo",
			"error TYPE_INVALID_FORMAT @ Bundle.entry[0].resource.birthDate",
			"warning CONSTRAINT_FAILED @ Bundle.entry[0].resource",
		}},
		// A resource validated against a profile of another type: its resourceType is its own.
		{"04_profile_of_other_type", []string{
			"error STRUCTURE_UNKNOWN_ELEMENT @ Patient.code",
			"error STRUCTURE_UNKNOWN_ELEMENT @ Patient.status",
			"warning CONSTRAINT_FAILED @ Observation",
		}},
		// A datatype has no resourceType, though it names the datatype.
		{"05_datatype_with_resourcetype", []string{
			"error STRUCTURE_UNKNOWN_ELEMENT @ Observation.valueQuantity.resourceType",
			"warning CONSTRAINT_FAILED @ Observation",
		}},
		// A type discriminator on an element of one type (a non-choice element) matches its type.
		{"06_type_discriminator_single_type", nil},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "re_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.Severity == "error" || is.Severity == "warning" {
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
