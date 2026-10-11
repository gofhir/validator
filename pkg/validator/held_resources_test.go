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
