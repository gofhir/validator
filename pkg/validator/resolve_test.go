package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// resolve() finds a reference's target as bundle.html#references says, in an invariant as in a
// discriminator: a relative reference from the base of the referring entry's fullUrl (not by the
// end of any fullUrl), a version against meta.versionId; and a path that steps into an entry
// resolves "#id" among the entry's contained resources. The verdicts are the HL7 validator's, but
// for the divergences declared (B-D18, B-D20, B-D21).
func TestResolveInBundle(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.resolve")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>"
	}{
		{"01_invariant_relative_base_ok", nil},
		{"02_invariant_relative_base_bad", []string{"CONSTRAINT_FAILED @ Bundle.entry[1].resource"}},
		{"03_discriminator_relative_base_ok", nil},
		{"04_discriminator_relative_base_bad", []string{"SLICING_CARDINALITY_MIN @ Bundle.entry:x"}},
		{"05_discriminator_entry_contained_ok", nil},
		{"06_discriminator_entry_contained_bad", []string{"SLICING_CARDINALITY_MIN @ Bundle.entry:x"}},
		{"07_invariant_versioned", nil},
		{"08_invariant_versioned_bad", []string{"CONSTRAINT_FAILED @ Bundle.entry[0].resource"}},
		// A resource an entry holds (a Parameters' parameter.resource) resolves from that entry's base.
		{"09_parameters_in_entry_invariant", nil},
		{"10_parameters_in_entry_discriminator", nil},
		{"11_parameters_in_entry_two_bases", nil},
		// Ambiguous, the reference names no target (B-D18); from an entry with no fullUrl, a relative
		// reference has no defined meaning (B-D20).
		{"12_versions_ambiguous", []string{"CONSTRAINT_FAILED @ Bundle.entry[0].resource", "REFERENCE_MULTIPLE_MATCHES @ Bundle.entry[0].resource.hasMember[0]"}},
		{"13_referring_entry_no_fullurl", []string{"CONSTRAINT_FAILED @ Bundle.entry[0].resource"}},
		// A RESTful fullUrl may name a version: the base is before the type and id (bdl-8 still
		// rejects it).
		{"14_referring_entry_versioned_fullurl", []string{"CONSTRAINT_FAILED @ Bundle.entry[0]"}},
		{"15_parameters_bundle_two_bases", nil},
		{"16_nested_bundle_two_bases", nil},
		{"17_contained_invariant_relative", nil}, // from the entry that holds the contained resource
		// A target's fullUrl that names a version (against bdl-8) matches no reference (B-D21).
		{"18_target_versioned_fullurl_relative", []string{"CONSTRAINT_FAILED @ Bundle.entry[0].resource", "CONSTRAINT_FAILED @ Bundle.entry[1]"}},
		{"19_target_versioned_fullurl_absolute", []string{"CONSTRAINT_FAILED @ Bundle.entry[0].resource", "CONSTRAINT_FAILED @ Bundle.entry[1]"}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "rs_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.Severity == "error" {
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
