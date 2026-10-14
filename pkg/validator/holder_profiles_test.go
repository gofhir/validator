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

// A held resource (an entry's, a parameter's or a part's resource, an entry's response outcome, a
// contained resource) is checked against the profiles its holder element's type.profile names for
// its type, in the slice its value matched or the element: one is checked against; several are
// alternatives (ElementDefinition.type.profile), the resource checked against those it conforms to,
// or, none, against every one, with an error. A profile of another type is one it does not meet;
// one that does not resolve, or has no snapshot, is reported. Each element is reported once. The
// verdicts are the HL7 validator's, but for the divergences declared (B-D31, B-D32, B-D33).
func TestHolderProfiles(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.holder")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<severity> <diagnostic> @ <location>", for holder profiles and cardinality
	}{
		// Alternatives: none met, an error and every one's; one met, nothing of the other's.
		{"alt_bundle_two_profiles_none", []string{
			"error CONSTRAINT_FAILED (Constraint failed: hd-c2: 'needs coding') @ Bundle.entry[0].resource",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
			"error SLICING_CARDINALITY_MIN @ Bundle.entry[0].resource.identifier:s",
		}},
		{"alt_bundle_two_profiles_one", nil},
		// Several met: a warning, as HL7 gives it.
		{"rv_x7_both_met", []string{"warning HOLDER_PROFILE_MULTIPLE_MATCHES @ Bundle.entry[0].resource"}},
		// Two groups, one per definition the Bundle is checked against: each applies.
		{"rv_p10_two_groups", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.subject",
			"error CONSTRAINT_FAILED (Constraint failed: hd-c: 'needs identifier') @ Bundle.entry[0].resource",
			"error CONSTRAINT_FAILED (Constraint failed: rv-c: 'rv-c') @ Bundle.entry[0].resource",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
			"error SLICING_CARDINALITY_MIN @ Bundle.entry[0].resource.identifier:s",
		}},
		// The holder is the slice a contained resource or an entry matched, re-sliced too; its
		// cardinality is reported once.
		{"rv_p1_ctype", []string{
			"error CARDINALITY_MIN @ Observation.contained[0].identifier",
			"error CONSTRAINT_FAILED (Constraint failed: rv-c: 'rv-c') @ Observation.contained[0]",
		}},
		{"rv_p1b_rtype", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"error CONSTRAINT_FAILED (Constraint failed: rv-c: 'rv-c') @ Bundle.entry[0].resource",
		}},
		{"rv_p5_reslice", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"error CONSTRAINT_FAILED (Constraint failed: rv-c: 'rv-c') @ Bundle.entry[0].resource",
		}},
		{"rv_x3_cslice", []string{"error CARDINALITY_MIN @ Patient.contained[0].subject"}},
		// Under a contentReference (B-D32), and nested: a Bundle's entries, a contained Bundle's.
		{"rv_p2_part", []string{
			"error CARDINALITY_MIN @ Parameters.parameter[0].part[0].resource.identifier",
			"error CONSTRAINT_FAILED (Constraint failed: rv-c: 'rv-c') @ Parameters.parameter[0].part[0].resource",
		}},
		{"rv_x4_nested", []string{"error CARDINALITY_MIN @ Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"rv_x5_contained_bundle", []string{"error CARDINALITY_MIN @ Patient.contained[0].entry[0].resource.subject"}},
		// A version the holder pins, whatever version meta.profile names (B-D31).
		{"rv_p7_ver", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.subject",
			"error CONSTRAINT_FAILED (Constraint failed: hd-c2: 'needs coding') @ Bundle.entry[0].resource",
			"error CONSTRAINT_FAILED (Constraint failed: hd-c: 'needs identifier') @ Bundle.entry[0].resource",
		}},
		// Profiles of another type only, a constraint on a type the resource derives from included:
		// none is met.
		{"rv_x1_mixed_other", []string{"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource"}},
		{"rv_x6_dr1", []string{"error HOLDER_PROFILE_NO_MATCH @ Patient.contained[0]"}},
		{"rv_x6_dr2", []string{"error HOLDER_PROFILE_NO_MATCH @ Patient.contained[0]"}},
		// A profile not found, beside its type's definition, which is met, too; one without a
		// snapshot (B-D33).
		{"u1_unres", []string{"error HOLDER_PROFILE_NOT_FOUND @ Bundle.entry[0].resource"}},
		{"u2_base_and_unres", []string{"error HOLDER_PROFILE_NOT_FOUND @ Bundle.entry[0].resource"}},
		{"rv_x2_snapfail", []string{"error PROFILE_SNAPSHOT_FAILED @ Bundle.entry[0].resource"}},
		// One alternative of its type, others of another: checked for conformance as among several;
		// none met, an error and its profile's.
		{"m1_mixed_obs_bad", []string{
			"error CONSTRAINT_FAILED (Constraint failed: hd-c2: 'needs coding') @ Bundle.entry[0].resource",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
		}},
		{"m2_mixed_obs_good", []string{
			"error CONSTRAINT_FAILED (Constraint failed: hd-c2: 'needs coding') @ Bundle.entry[0].resource",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
		}},
		// Met.
		{"m3_mixed_pat_good", nil},
		{"rw_b_own_pat", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
		}},
		{"rw_b_two_pat", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.subject",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
		}},
		// Its type's definition among the alternatives: met.
		{"rw_b_base_own", nil},
		// A supertype's definition is of another type (B-D34: the profile's errors as errors).
		{"rw_b_dr", []string{
			"error CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource",
		}},
		{"rw_b_pat_only", []string{"error HOLDER_PROFILE_NO_MATCH @ Bundle.entry[0].resource"}},
		// What a slice holding a resource constrains inline is checked, its type naming no profile.
		{"rw_p_cinline", []string{
			"error SLICING_CARDINALITY_MIN @ Patient.contained[0].id",
			"error SLICING_CARDINALITY_MIN @ Patient.contained[0].meta",
		}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "ht_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				switch {
				case is.MessageID == "CONSTRAINT_FAILED" && is.Severity == issue.SeverityError:
					got = append(got, "error "+is.MessageID+" ("+is.Diagnostics+") @ "+strings.Join(is.Expression, ","))
				case strings.HasPrefix(is.MessageID, "HOLDER_") || strings.HasPrefix(is.MessageID, "CARDINALITY_") ||
					strings.HasPrefix(is.MessageID, "SLICING_") || is.MessageID == "PROFILE_SNAPSHOT_FAILED":
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
