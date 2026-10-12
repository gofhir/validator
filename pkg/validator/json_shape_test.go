package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A value written otherwise than its element says (json.html) is reported where the HL7 validator
// reports it: an array for an element that repeats (a single value where it goes is checked as
// written), an object for a complex type or a resource, a simple value for a primitive (a null only
// where its "_key" item holds its id and extensions), and a primitive's "_key" sibling shaped as its
// values are; nothing in an array written where one value goes is checked, as HL7 does not.
func TestJSONShape(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>", for shape and the phases that read values
	}{
		{"a1_contained_object", []string{"STRUCTURE_ARRAY_EXPECTED @ Patient.contained"}},
		{"a2_resource_array", []string{"STRUCTURE_OBJECT_EXPECTED @ Bundle.entry[0].resource"}},
		{"a3_resource_primitive", []string{
			"STRUCTURE_OBJECT_EXPECTED @ Parameters.parameter[0].resource",
			"STRUCTURE_OBJECT_EXPECTED @ Parameters.parameter[1].resource",
			"STRUCTURE_OBJECT_EXPECTED @ Parameters.parameter[2].resource",
		}},
		{"a4_contained_primitives", []string{
			"STRUCTURE_OBJECT_EXPECTED @ Patient.contained[0]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.contained[1]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.contained[2]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.contained[3]",
		}},
		{"a5_complex_array_for_one", []string{"STRUCTURE_OBJECT_EXPECTED @ Patient.maritalStatus"}},
		{"a6_primitive_object", []string{
			"STRUCTURE_VALUE_EXPECTED @ Patient.birthDate",
			"STRUCTURE_VALUE_EXPECTED @ Patient.gender",
		}},
		{"a7_primitive_single_for_many", []string{
			"STRUCTURE_ARRAY_EXPECTED @ Patient.name[0].given",
			"STRUCTURE_VALUE_EXPECTED @ Patient.name[0].family",
		}},
		{"a8_complex_primitive", []string{
			"STRUCTURE_ARRAY_EXPECTED @ Patient.name",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.identifier[1]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.maritalStatus",
		}},
		{"a9_complex_single_for_many", []string{
			"STRUCTURE_ARRAY_EXPECTED @ Patient.identifier",
			"STRUCTURE_ARRAY_EXPECTED @ Patient.name",
		}},
		{"b1_sibling_shapes", []string{
			"STRUCTURE_ARRAY_EXPECTED @ Patient.name[0].given",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.birthDate",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.gender",
		}},
		{"b2_nulls", []string{
			"STRUCTURE_OBJECT_EXPECTED @ Patient.name[0]",
			"STRUCTURE_VALUE_EXPECTED @ Patient.active",
		}},
		{"b3_entry_object", []string{"STRUCTURE_ARRAY_EXPECTED @ Bundle.entry"}},
		{"b4_array_for_one_content", []string{
			"STRUCTURE_OBJECT_EXPECTED @ Patient.maritalStatus",
			"STRUCTURE_VALUE_EXPECTED @ Patient.birthDate",
			"STRUCTURE_VALUE_EXPECTED @ Patient.gender",
		}},
		// A single value has no null, whatever its "_key" sibling holds.
		{"b5_single_null_with_sibling", []string{"STRUCTURE_VALUE_EXPECTED @ Patient.birthDate"}},
		// A value written otherwise is no value of its element: the minimum is not met, as HL7 has it.
		{"b6_misshapen_minimum", []string{
			"CARDINALITY_MIN @ Observation.code",
			"CARDINALITY_MIN @ Observation.status",
			"STRUCTURE_OBJECT_EXPECTED @ Observation.code",
			"STRUCTURE_VALUE_EXPECTED @ Observation.status",
		}},
		// A primitive array's items are named by the array, as HL7 names them.
		{"b7_primitive_array_items", []string{
			"STRUCTURE_OBJECT_EXPECTED @ Patient.name[0].given",
			"STRUCTURE_VALUE_EXPECTED @ Patient.name[0].given",
		}},
		// A single null is no value, its "_key" sibling notwithstanding: the minimum is not met; an
		// array's items written otherwise are reported once per shape, at the array, as HL7 does.
		{"b9_null_minimum_and_repeats", []string{
			"CARDINALITY_MIN @ Patient.text.status",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.name[1].given",
			"STRUCTURE_VALUE_EXPECTED @ Patient.name[0].given",
			"STRUCTURE_VALUE_EXPECTED @ Patient.text.status",
		}},
		// A primitive's "_key" sibling is not checked when its value is an array where one goes.
		{"c1_array_for_one_siblings", []string{
			"STRUCTURE_VALUE_EXPECTED @ Patient.birthDate",
			"STRUCTURE_VALUE_EXPECTED @ Patient.gender",
		}},
		// Nor are the extension contexts of a "_key" sibling of a single value where an array goes.
		{"c2_single_for_many_sibling_context", []string{
			"STRUCTURE_ARRAY_EXPECTED @ Patient.name[0].given",
			"STRUCTURE_ARRAY_EXPECTED @ Patient.name[0].given",
		}},
		// The extension contexts in a single value where an array goes are named as its one item.
		{"c3_single_for_many_context_paths", []string{
			"EXTENSION_INVALID_CONTEXT @ Patient.contact[0].name",
			"EXTENSION_INVALID_CONTEXT @ Patient.contact[0].telecom[0]",
			"EXTENSION_INVALID_CONTEXT @ Patient.contained[0]",
			"STRUCTURE_ARRAY_EXPECTED @ Patient.contact",
			"STRUCTURE_ARRAY_EXPECTED @ Patient.contact.telecom",
			"STRUCTURE_ARRAY_EXPECTED @ Patient.contained",
		}},
		// Nothing is evaluated on a value written otherwise (cpt-2, ele-1, txt-1 and txt-2 are not).
		{"b8_misshapen_invariants", []string{
			"CARDINALITY_MIN @ Patient.text.div",
			"STRUCTURE_ARRAY_EXPECTED @ Patient.address[0].line",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.telecom[0]",
			"STRUCTURE_VALUE_EXPECTED @ Patient.text.div",
		}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "js_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				for _, p := range []string{"STRUCTURE_", "TYPE_", "BINDING_", "CARDINALITY_", "EXTENSION_"} {
					if strings.HasPrefix(is.MessageID, p) {
						got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
					}
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if tt.probe == "b8_misshapen_invariants" {
				for _, is := range res.Issues {
					if is.MessageID == "CONSTRAINT_FAILED" {
						t.Errorf("an invariant evaluated on a value written otherwise: %s @ %v", is.Diagnostics, is.Expression)
					}
				}
			}
		})
	}
}
