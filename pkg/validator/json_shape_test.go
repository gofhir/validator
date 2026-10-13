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
		// A value written otherwise is dropped, as HL7's parser drops it: the items after it are
		// renumbered (the contained Patient's binding at contained[0], cpt-2 at telecom[0]).
		{"d1_renumbered", []string{
			"BINDING_REQUIRED @ Patient.contained[0].gender",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.contained[0]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.name[0]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.telecom[0]",
			"STRUCTURE_VALUE_EXPECTED @ Patient.name[1].given",
		}},
		// The invariants of the element that held it do not see it (pat-1, ext-1), and the element
		// whose only content it was is no element without content (no ele-1), as HL7 has it.
		{"d2_parent_invariants", []string{
			"CARDINALITY_MIN @ Patient.extension[0].value[x]",
			"EXTENSION_VALUE_REQUIRED @ Patient.extension[0]",
			"STRUCTURE_OBJECT_EXPECTED @ Patient.contact[0].name",
			"STRUCTURE_VALUE_EXPECTED @ Patient.extension[0].valueString",
		}},
		// A primitive written where its array goes is dropped; an object, the array's one item.
		{"d3_single_for_many_dropped", []string{
			"STRUCTURE_ARRAY_EXPECTED @ Observation.component",
			"STRUCTURE_ARRAY_EXPECTED @ Observation.effectiveTiming.event",
		}},
		{"d4_primitive_items_renumbered", []string{
			"EXTENSION_UNKNOWN @ Patient.name[0]._given[0].extension[0]",
			"STRUCTURE_VALUE_EXPECTED @ Patient.name[0].given",
		}},
		// A primitive's "_key" sibling is an Element: its extensions are shaped and checked.
		{"d5_sibling_extension_value", []string{
			"EXTENSION_UNKNOWN @ Patient._birthDate.extension[0]",
			"STRUCTURE_VALUE_EXPECTED @ Patient._birthDate.extension[0].valueString",
		}},
		// An element written as an empty object is reported as HL7's parser reports it, not again by
		// ele-1, nor is the element that holds it; it is no value for FHIRPath (pat-1, ext-1 fail),
		// and still one for cardinality, as HL7 counts it.
		{"e1_empty_objects", []string{
			"STRUCTURE_EMPTY_OBJECT @ Patient.contact[0].name",
			"STRUCTURE_EMPTY_OBJECT @ Patient.extension[0].valueAddress",
		}},
		// An element with only an id has no content either: ele-1 says so (HL7 does not judge it).
		{"e2_empty_and_id_only", []string{
			"BINDING_TEXT_ONLY_WARNING @ Patient.maritalStatus", // no code, as HL7 warns
			"STRUCTURE_EMPTY_OBJECT @ Patient.name[0]",
		}},
		// A resource written empty, and a primitive's "_key" sibling, are empty objects too.
		{"e3_empty_resource_and_siblings", []string{
			"STRUCTURE_EMPTY_OBJECT @ Patient.birthDate",
			"STRUCTURE_EMPTY_OBJECT @ Patient.contained[0]",
			"STRUCTURE_EMPTY_OBJECT @ Patient.name[0].given",
			"STRUCTURE_NO_RESOURCE_TYPE @ Patient.contained[0]",
		}},
		{"e4_empty_entry_resource", []string{
			"STRUCTURE_EMPTY_OBJECT @ Bundle.entry[0].resource",
			"STRUCTURE_NO_RESOURCE_TYPE @ Bundle.entry[0].resource",
		}},
		// A primitive's "_key" sibling written otherwise drops its value with it, as HL7's parser
		// does: no format is checked on it, and no invariant sees it (f2: obs-6 does not fail).
		{"f1_sibling_misshapen_drops_value", []string{"STRUCTURE_OBJECT_EXPECTED @ Patient.birthDate"}},
		{"f2_sibling_misshapen_invariants", []string{
			"BINDING_TEXT_ONLY_WARNING @ Observation.dataAbsentReason",
			"STRUCTURE_OBJECT_EXPECTED @ Observation.valueString",
		}},
		{"f3_sibling_single_for_many", []string{"STRUCTURE_ARRAY_EXPECTED @ Patient.name[0].given"}},
		{"f4_sibling_item_misshapen", []string{"STRUCTURE_OBJECT_EXPECTED @ Patient.name[0].given"}},
		// Nothing is evaluated on a value written otherwise (cpt-2, ele-1, txt-1 and txt-2 are not),
		// and the invariants of the elements that hold it do not see it (dom-6 fails, as in HL7).
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
			// The invariants see the instance as HL7 does, without the values written otherwise: no
			// cpt-2 nor ele-1 on them, dom-6 finds no div (b8); pat-1 finds no contact name, ext-1
			// no value, and the contact the name was in is not without content (d2).
			if want, ok := map[string][]string{
				"b8_misshapen_invariants":         {"dom-6 @ Patient"},
				"d2_parent_invariants":            {"ext-1 @ Patient.extension[0]", "pat-1 @ Patient.contact[0]"},
				"e1_empty_objects":                {"ext-1 @ Patient.extension[0]", "pat-1 @ Patient.contact[0]"},
				"e2_empty_and_id_only":            {"ele-1 @ Patient.maritalStatus", "pat-1 @ Patient.contact[0]"},
				"f2_sibling_misshapen_invariants": nil,
			}[tt.probe]; ok {
				var invariants []string
				for _, is := range res.Issues {
					if is.MessageID == "CONSTRAINT_FAILED" {
						key := strings.TrimPrefix(is.Diagnostics, "Constraint failed: ")
						key = key[:strings.IndexByte(key, ':')]
						invariants = append(invariants, key+" @ "+strings.Join(is.Expression, ","))
					}
				}
				slices.Sort(invariants)
				if !slices.Equal(invariants, want) {
					t.Errorf("invariants %q, want %q", invariants, want)
				}
			}
		})
	}
}

// An issue is located in the source as written: the shape check's at the value it reports, the
// others at the value they report on, the items after a dropped one renumbered in their paths but
// not in the source.
func TestNormalizedLocations(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	src := "{\n" +
		"  \"resourceType\": \"Patient\",\n" +
		"  \"text\": {\"status\": \"generated\", \"div\": \"<div xmlns=\\\"http://www.w3.org/1999/xhtml\\\">x</div>\"},\n" +
		"  \"telecom\": [\n" +
		"    5,\n" +
		"    {\"value\": \"1\"},\n" +
		"    {\"value\": \"2\", \"foo\": 2}\n" +
		"  ]\n" +
		"}\n"
	res, err := v.Validate(context.Background(), []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, is := range res.Issues {
		if is.Location != nil {
			got[is.MessageID+" @ "+strings.Join(is.Expression, ",")] = is.Location.Line
		}
	}
	want := map[string]int{
		"STRUCTURE_OBJECT_EXPECTED @ Patient.telecom[0]": 5,
		"CONSTRAINT_FAILED @ Patient.telecom[0]":         6,
		// structural names it as written (telecom[2], not the renumbered telecom[1]), and it is
		// located by that path, once
		"STRUCTURE_UNKNOWN_ELEMENT @ Patient.telecom[2].foo": 7,
	}
	for k, line := range want {
		if got[k] != line {
			t.Errorf("%s: line %d, want %d (all: %v)", k, got[k], line, got)
		}
	}
}
