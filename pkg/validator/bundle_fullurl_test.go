package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// An entry must have a fullUrl but in a transaction, a batch, their responses, and where its request
// is a POST (bundle.html#bundle-unique; R5's bdl-15, which R4 carries as an erratum), whichever
// Bundle it is in: the innermost Bundle's type decides. A relative reference made in an entry that
// must have one and has none, in its resource, its contained resources or an extension, has no
// base. The verdicts are the HL7 validator's, but for B-D35 (bdl-15 on the Bundle, HL7 on each
// entry) and B-D36, and where HL7 reports what the specification allows (transaction and batch
// entries, history POST entries).
func TestBundleFullURL(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.refwalk")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>", for bdl-15 and references without a base
	}{
		// Entries that must have a fullUrl.
		{"b_collection_none", []string{"bdl-15 @ Bundle"}},
		{"b_collection_rel", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[0].resource.subject",
			"bdl-15 @ Bundle",
		}},
		{"b_collection_abs", []string{"bdl-15 @ Bundle"}},
		{"b_document_rel", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[1].resource.subject",
			"bdl-15 @ Bundle",
		}},
		{"b_searchset_rel", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[0].resource.subject",
			"bdl-15 @ Bundle",
		}},
		{"b1_searchset_outcome_nofullurl", []string{"bdl-15 @ Bundle"}},
		// Transactions, batches, their responses, POST entries: none required.
		{"b_transaction_rel", nil},
		{"b4_transaction_post_rel", nil},
		{"b_batch_rel", nil},
		{"b_transaction-response_rel", nil},
		{"b_history_none", nil},
		// A history PUT entry.
		{"b9_history_put_nofullurl", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[0].resource.subject",
			"bdl-15 @ Bundle",
		}},
		// The innermost Bundle's type decides (B-D36: HL7 reports no reference in b5).
		{"b5_nested_collection_in_transaction", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[0].resource.entry[0].resource.subject",
			"bdl-15 @ Bundle.entry[0].resource",
		}},
		{"b6_nested_transaction_in_collection", nil},
		{"b7_parameters_bundle_nofullurl", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Parameters.parameter[0].resource.entry[0].resource.subject",
			"bdl-15 @ Parameters.parameter[0].resource",
		}},
		{"b8_contained_bundle_nofullurl", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Patient.contained[0].entry[0].resource.subject",
			"bdl-15 @ Patient.contained[0]",
		}},
		// The Bundle's own elements are in no entry; a contained resource and an extension in one are.
		{"e01_root_bundle_signature_rel", nil},
		{"e04_contained_in_entry_nofullurl_rel", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[0].resource.contained[0].performer[0]",
			"bdl-15 @ Bundle",
		}},
		{"d04_ext_rel_no_fullurl", []string{
			"REFERENCE_RELATIVE_NO_FULLURL @ Bundle.entry[0].resource.extension[0].valueReference",
			"bdl-15 @ Bundle",
		}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "fu_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.Contains(is.Diagnostics, "bdl-15") || is.MessageID == "REFERENCE_RELATIVE_NO_FULLURL" {
					id := is.MessageID
					if id == "CONSTRAINT_FAILED" {
						id = "bdl-15"
					}
					got = append(got, id+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
