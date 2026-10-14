package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every value of type Reference is checked under the definition that governs it: an extension's
// value under the extension's definition (at any depth: sub-extensions, on datatypes and
// primitives, modifier extensions, in contained resources, entries and parameters), a choice
// element under the type its property names, a contentReference under the element it names, a
// slice's under the slice, a datatype's under the profile its type names. "#" names the container
// only from a contained resource. Aggregation is decided by where the target is found; Reference.type
// is a target the element must allow. The verdicts are the HL7 validator's, but for B-D13 (a
// relative URL that does not resolve tells its type).
func TestReferenceWalk(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.refwalk")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>", for references
	}{
		// An extension's value, wherever the extension is, under its definition.
		{"a01_root_any_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.extension[0].valueReference.reference"}},
		{"a02_root_unknown_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.extension[0].valueReference.reference"}},
		{"a03_root_patient_contained_obs", []string{"REFERENCE_INVALID_TARGET @ Patient.extension[0].valueReference"}},
		{"a05_nested_complex_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.extension[0].extension[0].valueReference.reference"}},
		{"a06_nested_complex_obs", []string{"REFERENCE_INVALID_TARGET @ Patient.extension[0].extension[0].valueReference"}},
		{"a07_datatype_name_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.name[0].extension[0].valueReference.reference"}},
		{"a08_primitive_birthDate_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.birthDate.extension[0].valueReference.reference"}},
		{"a09_modifier_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.modifierExtension[0].valueReference.reference"}},
		{"a10_in_contained_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.contained[0].extension[0].valueReference.reference"}},
		{"a11_bundle_entry_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Bundle.entry[0].resource.extension[0].valueReference.reference"}},
		{"a12_parameters_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Parameters.parameter[0].resource.extension[0].valueReference.reference"}},
		{"a15_reference_ext_on_reference", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.generalPractitioner[0].extension[0].valueReference.reference"}},
		// B-D13.
		{"a04_root_patient_relative_obs", []string{"REFERENCE_INVALID_TARGET @ Patient.extension[0].valueReference"}},
		// Choice elements, and under a contentReference.
		{"d01_choice_reportedReference_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ MedicationRequest.reportedReference.reference"}},
		{"d02_qr_answer_valueReference_nope", []string{"REFERENCE_CONTAINED_NOT_FOUND @ QuestionnaireResponse.item[0].answer[0].valueReference.reference"}},
		{"d03_ext_urn_not_in_bundle", []string{
			"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.extension[0].valueReference",
			"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.generalPractitioner[0]",
		}},
		{"d05_ext_type_mismatch", []string{"REFERENCE_TYPE_MISMATCH @ Patient.extension[0].valueReference"}},
		{"d06_ext_bundle_resolves_wrong_type", []string{"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.extension[0].valueReference"}},
		{"q2_qr_answer_item_answer", []string{"REFERENCE_CONTAINED_NOT_FOUND @ QuestionnaireResponse.item[0].answer[0].item[0].answer[0].valueReference.reference"}},
		{"q1_qr_nested_item_answer", []string{"REFERENCE_CONTAINED_NOT_FOUND @ QuestionnaireResponse.item[0].item[0].answer[0].valueReference.reference"}},
		// "#": only a contained resource names its container with it.
		{"c01_root_hash", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Observation.focus[0].reference"}},
		{"c02_root_hash_with_contained_ref_container", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Observation.focus[0].reference"}},
		{"c03_contained_hash_only", nil},
		{"c04_bundle_entry_root_hash", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Bundle.entry[0].resource.focus[0].reference"}},
		{"c05_parameters_hash", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Parameters.parameter[0].resource.focus[0].reference"}},
		{"c06_root_hash_contained_unrelated", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Observation.focus[0].reference"}},
		{"c07_ext_hash_root", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Patient.extension[0].valueReference.reference"}},
		{"c08_hash_bad_type_from_contained", nil},
		{"c09_hash_wrong_type_from_contained", []string{"REFERENCE_INVALID_TARGET @ Observation.contained[0].subject"}},
		// "#" from a held resource, contained in contained, and in conformance checks.
		{"h2_contained_parameters_held_hash", []string{
			"REFERENCE_CONTAINER_NOT_FOUND @ Patient.contained[0].parameter[0].resource.focus[0].reference",
			"REFERENCE_INVALID_TARGET @ Patient.link[0].other",
		}},
		{"h3_contained_in_contained_hash", []string{
			"REFERENCE_CONTAINED_NOT_FOUND @ Patient.contained[0].hasMember[0].reference",
			"REFERENCE_INVALID_TARGET @ Patient.generalPractitioner[0]",
		}},
		{"h4_conformance_contained_hash", nil},
		{"h5_slice_profile_contained_hash", nil},
		{"h6_entry_conformance_hash", []string{"REFERENCE_CONTAINER_NOT_FOUND @ Bundle.entry[1].resource.focus[0].reference"}},
		// A slice's targets, and a datatype profile's.
		{"s1_slice_target_type", []string{"REFERENCE_INVALID_TARGET @ Patient.generalPractitioner[0]"}},
		{"s3_slice_target_profile", []string{"REFERENCE_TARGET_PROFILE @ Patient.generalPractitioner[0]"}},
		{"s2_slice_target_type_unresolved", []string{"REFERENCE_INVALID_TARGET @ Patient.generalPractitioner[0]"}},
		{"t1_identifier_profile_assigner", []string{"REFERENCE_TARGET_PROFILE @ Patient.identifier[0].assigner"}},
		{"t2_annotation_profile_author", []string{"REFERENCE_INVALID_TARGET @ Observation.note[0].authorReference"}},
		// A choice property naming a type its element does not declare is unknown: nothing in it is checked.
		{"rv_u1_r4_obs_valueReference", nil},
		{"rv_u2_r4_pat_badchoice", nil},
		// Under a type profile that does not resolve, its type's definition.
		{"rv_q02_unresolved_type_profile", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.identifier[0].assigner.reference"}},
		// Aggregation: where the target is found; no Bundle, nothing bundled.
		{"g1_aggregation", []string{
			"REFERENCE_AGGREGATION_MODE @ Bundle.entry[0].resource.generalPractitioner[0].reference",
			"REFERENCE_AGGREGATION_MODE @ Bundle.entry[0].resource.managingOrganization.reference",
		}},
		{"g2_aggregation_root", []string{
			"REFERENCE_AGGREGATION_MODE @ Patient.generalPractitioner[0].reference",
			"REFERENCE_AGGREGATION_MODE @ Patient.managingOrganization.reference",
		}},
		{"g3_bundled_rel_resolves", nil},
		{"g4_bundled_urn_missing", []string{
			"REFERENCE_AGGREGATION_MODE @ Bundle.entry[0].resource.generalPractitioner[0].reference",
			"REFERENCE_AGGREGATION_MODE @ Bundle.entry[0].resource.managingOrganization.reference",
			"REFERENCE_CONTAINED_NOT_FOUND @ Bundle.entry[0].resource.generalPractitioner[0].reference",
			"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.managingOrganization",
		}},
		{"rv_q01_hash_contained_agg", []string{
			"REFERENCE_AGGREGATION_MODE @ Patient.contained[0].focus[0].reference",
			"REFERENCE_INVALID_TARGET @ Patient.link[0].other",
		}},
		{"rv_q03_root_obs_agg", []string{"REFERENCE_AGGREGATION_MODE @ Observation.focus[0].reference"}},
		{"rv_q09_root_hash_contained_agg", []string{
			"REFERENCE_AGGREGATION_MODE @ Observation.subject.reference",
			"REFERENCE_CONTAINER_NOT_FOUND @ Observation.subject.reference",
		}},
		// Reference.type outside the targets, besides inconsistent with the target.
		{"w2_literal_type_mismatch", []string{
			"REFERENCE_INVALID_TARGET @ Observation.subject",
			"REFERENCE_TYPE_MISMATCH @ Observation.subject",
		}},
		// A logical reference's type is not checked; a value of a type the extension does not allow is;
		// B-D13.
		{"w1_logical_type_not_allowed", nil},
		{"w3_ext_forbids_reference", []string{"REFERENCE_CONTAINED_NOT_FOUND @ Patient.extension[0].valueReference.reference"}},
		{"w4_rel_unresolved_wrong_type", []string{"REFERENCE_INVALID_TARGET @ Observation.subject"}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "rw_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "REFERENCE_") {
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
