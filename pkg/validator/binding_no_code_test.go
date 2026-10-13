package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A CodeableConcept that carries no code (no coding, or codings with neither a code nor a system,
// and text or nothing: an id, an empty object, a text written otherwise and dropped) provides none
// to its binding, as HL7 reports it: a warning under an extensible binding, an error under a
// required one, and only that the value set is not found when it does not resolve. A Coding without
// a code is not reported by the binding. No such concept is a member of a value set a slice's
// binding discriminates by.
func TestBindingNoCode(t *testing.T) {
	v, err := New(WithVersion("4.0.1"),
		WithPackageTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "acme.nocode-0.1.0.tgz")))
	if err != nil {
		t.Fatal(err)
	}
	for probe, want := range map[string][]string{
		"bn_cc_empty":             {"BINDING_TEXT_ONLY_WARNING @ Patient.maritalStatus"},
		"bn_cc_id_only":           {"BINDING_TEXT_ONLY_WARNING @ Patient.maritalStatus"},
		"bn_cc_text_misshapen":    {"BINDING_TEXT_ONLY_WARNING @ Patient.maritalStatus"},
		"bn_cc_required_no_code":  {"BINDING_REQUIRED_NO_CODE @ AllergyIntolerance.clinicalStatus"},
		"bn_coding_no_code":       nil,
		"bp_ext_cc_id":            {"BINDING_TEXT_ONLY_WARNING @ Patient.extension[0].valueCodeableConcept"},
		"bp_ext_coding_id":        nil,
		"bp_ext_unres_id":         {"BINDING_VALUESET_NOT_FOUND @ Patient.extension[0].valueCodeableConcept"},
		"bp_ext_unres_text":       {"BINDING_VALUESET_NOT_FOUND @ Patient.extension[0].valueCodeableConcept"},
		"bp_unres_extensible_id":  {"BINDING_VALUESET_NOT_FOUND @ Patient.maritalStatus"},
		"bp_unres_required_id":    {"BINDING_VALUESET_NOT_FOUND @ Patient.contact[0].relationship[0]"},
		"bp_slice_id":             {"SLICING_CARDINALITY_MIN @ Observation.category:cat"},
		"bp_slice_extension_only": {"SLICING_CARDINALITY_MIN @ Observation.category:cat"},
		"bp_slice_text":           {"SLICING_CARDINALITY_MIN @ Observation.category:cat"},
	} {
		t.Run(probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "BINDING_") || strings.HasPrefix(is.MessageID, "SLICING_") {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// A CodeableConcept whose codings carry neither a code nor a system provides no code, nor does an
// empty coding list (HL7 warns on both); codings of a system without a code are not told so.
func TestBindingNoCodeInCodings(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	const text = `"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}`
	for name, tt := range map[string]struct {
		maritalStatus string
		want          []string
	}{
		"empty coding list":  {`{"coding":[]}`, []string{"BINDING_TEXT_ONLY_WARNING @ Patient.maritalStatus"}},
		"coding with an id":  {`{"coding":[{"id":"y"}]}`, []string{"BINDING_TEXT_ONLY_WARNING @ Patient.maritalStatus"}},
		"coding of a system": {`{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/v3-MaritalStatus"}]}`, nil},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := v.Validate(context.Background(),
				[]byte(`{"resourceType":"Patient",`+text+`,"maritalStatus":`+tt.maritalStatus+`}`))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "BINDING_") {
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
