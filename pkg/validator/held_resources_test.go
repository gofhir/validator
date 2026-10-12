package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The resources other elements hold (a Parameters' parameters and parts, a Bundle entry's response
// outcome, at any depth) are checked for cardinality, slicing and references against the profiles
// their meta.profile declares and their type, as contained resources and entries are, each in its
// own scope: its own contained resources for "#id", and the Bundle and fullUrl of the entry that
// holds it for the rest. The verdicts are the HL7 validator's.
func TestHeldResources(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.held")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>", for cardinality, slicing and references
	}{
		{"h1_params_slice", []string{"SLICING_CARDINALITY_MIN @ Parameters.parameter[0].resource.identifier:s"}},
		{"h3_outcome", []string{ // against the profile its meta.profile declares
			"CARDINALITY_MIN @ Bundle.entry[0].response.outcome.code",
			"CARDINALITY_MIN @ Bundle.entry[0].response.outcome.status",
		}},
		{"h4_entry_params_refs", []string{ // "#c" resolves among the held resource's own contained
			"REFERENCE_CONTAINED_NOT_FOUND @ Bundle.entry[1].resource.parameter[0].resource.derivedFrom[0].reference",
			"REFERENCE_INVALID_TARGET @ Bundle.entry[1].resource.parameter[0].resource.derivedFrom[1]", // B-D13
			"REFERENCE_INVALID_TARGET @ Bundle.entry[1].resource.parameter[0].resource.specimen",
		}},
		{"h6_params_nested", []string{"REFERENCE_INVALID_TARGET @ Parameters.parameter[1].resource.entry[1].resource.specimen"}},
		{"c_relref", []string{
			"REFERENCE_CONTAINED_NOT_FOUND @ Parameters.parameter[1].resource.derivedFrom[0].reference",
			// "Patient/1" resolves to no entry (a root Parameters is no Bundle): its type is the
			// URL's (B-D13); HL7 resolves it among the Parameters' resources, to the same verdict.
			"REFERENCE_INVALID_TARGET @ Parameters.parameter[1].resource.specimen",
		}},
		{"l_held_in_contained", []string{ // a held resource does not see its holder's contained
			"REFERENCE_CONTAINED_NOT_FOUND @ Observation.contained[1].parameter[0].resource.subject.reference",
			"REFERENCE_INVALID_TARGET @ Observation.derivedFrom[0]",
		}},
		// A held Bundle's entries resolve among its entries.
		{"m_held_bundle_in_params", []string{"REFERENCE_INVALID_TARGET @ Parameters.parameter[0].resource.entry[0].resource.specimen"}},
		// An entry's response outcome is not its entry's resource: its entry's fullUrl is no base.
		{"n_outcome_base", nil},
		// "#q" names the one contained resource written where the array goes, as HL7 finds it.
		{"x_contained_object", nil},
		// A Bundle that is an entry's resource resolves its own references among its entries, from
		// its fullUrl, and its entries' outcomes from it too.
		{"o_entry_bundle_signature", []string{"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.signature.who"}},
		{"p_entry_bundle_outcome", []string{
			"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.entry[0].response.outcome.contained[0].specimen",
		}},
		// A contained or held Bundle is no entry's resource: its own relative references have no base.
		{"q_contained_bundle_signature", nil},
		{"r_held_bundle_signature", nil},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "hr_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "CARDINALITY_") || strings.HasPrefix(is.MessageID, "SLICING_") ||
					strings.HasPrefix(is.MessageID, "REFERENCE_") {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// The values of every resource another holds are checked once (dates, codes against required
// bindings, units), each resource against its type: the resources other elements hold, a contained
// resource's contained resources and a contained Bundle's entries too; a contained resource's or
// an entry's meta and language once, not once more through the element that holds it. The verdicts
// are the HL7 validator's.
func TestHeldResourceValues(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.held")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>", for bindings and primitive values
	}{
		{"s_entry_meta", []string{
			"BINDING_EXTENSIBLE @ Bundle.entry[0].resource.meta.security[0]",
			"BINDING_REQUIRED @ Bundle.entry[0].resource.gender",
			"TYPE_INVALID_FORMAT @ Bundle.entry[0].resource.birthDate",
		}},
		{"t_held_patient", []string{
			"BINDING_EXTENSIBLE @ Parameters.parameter[0].resource.meta.security[0]",
			"BINDING_REQUIRED @ Parameters.parameter[0].resource.gender",
			"TYPE_INVALID_FORMAT @ Parameters.parameter[0].resource.birthDate",
		}},
		{"u_contained_meta", []string{
			"BINDING_EXTENSIBLE @ Patient.contained[0].meta.security[0]",
			"BINDING_REQUIRED @ Patient.contained[0].gender",
			"TYPE_INVALID_FORMAT @ Patient.contained[0].birthDate",
		}},
		{"v_meta_security", []string{
			"BINDING_EXTENSIBLE @ Bundle.entry[0].resource.meta.security[0]",
			"BINDING_EXTENSIBLE @ Bundle.entry[0].resource.parameter[0].resource.meta.security[0]",
		}},
		{"a_cbundle", []string{
			"BINDING_REQUIRED @ Observation.contained[0].entry[0].resource.gender",
			"TYPE_INVALID_FORMAT @ Observation.contained[0].entry[0].resource.birthDate",
		}},
		{"b_ccc", []string{
			"BINDING_REQUIRED @ Observation.contained[0].contained[0].gender",
			"TYPE_INVALID_FORMAT @ Observation.contained[0].contained[0].birthDate",
		}},
		{"h6_params_nested", []string{
			"BINDING_REQUIRED @ Parameters.parameter[1].resource.entry[0].resource.gender",
			"TYPE_INVALID_FORMAT @ Parameters.parameter[0].resource.parameter[0].resource.birthDate",
		}},
		{"w_outcome_values", []string{
			"BINDING_REQUIRED @ Bundle.entry[0].response.outcome.issue[0].code",
			"BINDING_REQUIRED @ Bundle.entry[0].response.outcome.issue[0].extension[0].valueCode", // S4
			"BINDING_REQUIRED @ Bundle.entry[0].response.outcome.issue[0].severity",
			"TYPE_INVALID_FORMAT @ Bundle.entry[0].response.outcome.meta.lastUpdated",
		}},
		// A single resource where an array goes is checked as its one item, as HL7 does.
		{"x_contained_object", []string{
			"BINDING_REQUIRED @ Patient.contained[0].gender",
			"TYPE_INVALID_FORMAT @ Patient.contained[0].birthDate",
			"TYPE_INVALID_FORMAT @ Patient.contained[0].meta.lastUpdated",
		}},
		// A primitive or a null where a resource goes is the structural phase's (S5), not the
		// primitive phase's.
		{"y_contained_primitives", nil},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "hr_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "BINDING_") || strings.HasPrefix(is.MessageID, "TYPE_") ||
					strings.HasPrefix(is.MessageID, "UCUM_") {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A Quantity in a resource a parameter holds is checked for its UCUM code, once.
func TestHeldResourceUnits(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"Parameters","parameter":[{"name":"a","resource":
{"resourceType":"Observation","status":"final","code":{"text":"x"},"valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"kg//"}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if is.MessageID == "UCUM_INVALID_CODE" {
			got = append(got, strings.Join(is.Expression, ","))
		}
	}
	if want := []string{"Parameters.parameter[0].resource.valueQuantity.code"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The extensions of every element are checked, once, those of the resources another holds in
// them: an entry's own (Bundle.entry.extension, List.entry.extension), a response outcome's, a
// held resource's; the walk follows the definitions, stops at the resources elements hold (checked
// as resources), and does not go into a value written as an array where one goes nor into an
// element the definitions do not have. The verdicts are the HL7 validator's.
func TestHeldResourceExtensions(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.held")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>", for extensions and their values' bindings
	}{
		{"z1_list_entry_extension", []string{ // a URN is no URL; HL7: no definition found
			"EXTENSION_INVALID_URL @ List.entry[0].extension[0]",
			"EXTENSION_INVALID_URL @ List.note[0].extension[0]",
		}},
		{"z2_bundle_entry_extension", []string{
			"BINDING_REQUIRED @ Bundle.entry[0].extension[0].valueCode",
			"EXTENSION_INVALID_CONTEXT @ Bundle.entry[0]",
			"EXTENSION_UNKNOWN @ Bundle.entry[0].extension[1]",
		}},
		{"w_outcome_values", []string{
			"BINDING_REQUIRED @ Bundle.entry[0].response.outcome.issue[0].extension[0].valueCode",
			"EXTENSION_INVALID_CONTEXT @ Bundle.entry[0].response.outcome.issue[0]",
		}},
		// Not walked, as HL7 does not: an array where one resource goes (HL7: "must be an Object,
		// not an Array"), a resource of an unknown type.
		{"z3_array_for_one_extension", nil},
		{"z4_unknown_type_extension", nil},
		// Not walked either, as HL7 does not: an array where one value goes, on the way to a
		// resource or not (response, request, maritalStatus), and an unknown element.
		{"z5_arrays_for_one_extension", nil},
		{"z6_datatype_array_for_one_extension", nil},
		{"z7_unknown_element_extension", nil},
		// A primitive's extensions in its "_key" sibling (with nulls aligning the arrays), an entry
		// written where the array goes named as its one item; not a complex element's "_key"
		// sibling nor an object written for a primitive, which HL7 reports and does not check.
		// The extension in the e-choice extension's "_valueString", which HL7 checks, is not: a
		// gap of the extension value's check, on main too (the plan, S4).
		{"z8_element_extensions", []string{
			"EXTENSION_UNKNOWN @ Bundle.entry[0].extension[0]",
			"EXTENSION_UNKNOWN @ Bundle.entry[0].resource.extension[0]",
			"EXTENSION_UNKNOWN @ Bundle.entry[0].resource.name[0]._given[0].extension[0]",
		}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "hr_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				loc := strings.Join(is.Expression, ",")
				if strings.HasPrefix(is.MessageID, "EXTENSION_") || strings.Contains(loc, ".extension[") {
					got = append(got, is.MessageID+" @ "+loc)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Only a primitive has a "_key" sibling (json.html#primitive): a complex element's is no property
// of it, so it neither satisfies the element's minimum nor counts toward its maximum. The verdicts
// are the HL7 validator's.
func TestComplexKeySibling(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "hr_z9_complex_key_sibling.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if strings.HasPrefix(is.MessageID, "CARDINALITY_") {
			got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
		}
	}
	if want := []string{"CARDINALITY_MIN @ Observation.code"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
