package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// A value is bound by the definition that governs it, as HL7 binds it: the slice it matched (a
// required category slice, a value[x] type slice), else the element; a choice element's datatype's
// own bindings (valueQuantity.comparator); a contentReference's (Questionnaire.item.item); a
// repeating code once per item; a primitive with no value under a required binding; meta.tag's
// codes in their CodeSystem; a contained resource against the profile it declares. A required
// binding missed by a CodeableConcept is reported once, at the concept.
func TestBindingWalk(t *testing.T) {
	v, err := New(WithVersion("4.0.1"),
		WithPackageTgz(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "acme.bindwalk-0.1.0.tgz")))
	if err != nil {
		t.Fatal(err)
	}
	for probe, want := range map[string][]string{
		"slice_binding": {
			"BINDING_REQUIRED_NO_CODING @ Observation.category[0]",
			"CODE_NOT_IN_CODESYSTEM @ Observation.category[0].coding[0].code",
		},
		"type_slice_binding": {"BINDING_REQUIRED_NO_CODING @ Observation.valueCodeableConcept"},
		"choice_datatype":    {"BINDING_REQUIRED @ Observation.valueQuantity.comparator"},
		"repeating_code":     {"BINDING_REQUIRED @ AllergyIntolerance.category[0]"},
		"content_reference":  {"BINDING_REQUIRED @ Questionnaire.item[0].item[0].type"},
		"no_value":           {"BINDING_REQUIRED_NO_CODE @ Patient.gender"},
		"meta_tag_code":      {"CODE_NOT_IN_CODESYSTEM @ Observation.meta.tag[0].code"},
		"contained_profile": {
			"BINDING_REQUIRED_NO_CODING @ Patient.contained[0].interpretation[0]",
			"BINDING_REQUIRED_NO_CODING @ Patient.contained[0].method",
		},
		// A concept with a coding not in the value set and one that cannot be decided is not known
		// to be from it: no binding error, and no member of a slice a required binding discriminates.
		"mixed_codings": {"CODE_NOT_IN_CODESYSTEM @ Observation.code.coding[0].code"},
		"mixed_slice": {
			"CODE_NOT_IN_CODESYSTEM @ Observation.category[0].coding[0].code",
			"SLICING_CARDINALITY_MIN @ Observation.category:cat",
		},
	} {
		t.Run(probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "bw_"+probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.Severity == issue.SeverityError &&
					(strings.HasPrefix(is.MessageID, "BINDING_") || strings.HasPrefix(is.MessageID, "CODE_") ||
						strings.HasPrefix(is.MessageID, "SLICING_")) {
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
