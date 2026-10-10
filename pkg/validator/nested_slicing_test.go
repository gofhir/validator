package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A resource another holds (a Bundle's entry, a contained resource, an entry of a nested Bundle) is
// sliced against the profiles its meta.profile declares, as the resource validated is: a required
// slice that is missing is reported at the resource, once whatever root profiles hold it. The
// verdicts are the HL7 validator's.
func TestNestedProfileSlicing(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.nested")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // where SLICING_CARDINALITY_MIN is reported
	}{
		{"01_root", []string{"Observation.identifier:s"}},
		{"02_entry", []string{"Bundle.entry[0].resource.identifier:s"}},
		{"03_contained", []string{"Observation.contained[0].identifier:s"}},
		{"04_nested_bundle", []string{"Bundle.entry[0].resource.entry[0].resource.identifier:s"}},
		{"05_entry_ok", nil},
		{"06_entry_no_profile", nil},
		{"07_two_root_profiles", []string{"Bundle.entry[0].resource.identifier:s"}},
		{"08_contained_in_entry", []string{"Bundle.entry[0].resource.contained[0].identifier:s"}},
		// A nested Bundle's resolve() discriminator finds its own entries, then the outer Bundle's.
		{"10_nested_bundle_resolve_inner", nil},
		{"11_nested_bundle_resolve_inner_urn", nil},
		{"12_nested_bundle_resolve_outer", nil},
		// A nested Bundle is sliced against its own profile: the inner Patient that does not
		// conform keeps its Observation out of the slice, the outer one notwithstanding.
		{"14_nested_bundle_resolve_inner_bad", []string{"Bundle.entry[0].resource.entry:withpat"}},
		{"15_nested_bundle_resolve_inner_first", []string{"Bundle.entry[0].resource.entry:withpat"}},
		{"16_nested_bundle_resolve_unresolved", []string{"Bundle.entry[0].resource.entry:withpat"}},
		{"17_nested_bundle_resolve_outer_bad", nil},
		{"18_unknown_type_entry", nil}, // a resource of an unknown type is not walked, nor what it holds
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "ns_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == "SLICING_CARDINALITY_MIN" {
					got = append(got, strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A resource a Bundle validated against two root profiles holds is validated once: the cardinality
// its own profile sets is reported once (HL7 reports it once; it was reported twice before). A
// cardinality each root profile sets on the Bundle, at the element that holds the resource, is
// reported once per profile, as HL7 reports it.
func TestNestedCardinalityOnce(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.nested", "acme.targets")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>"
	}{
		{"09_two_root_profiles_entry_cardinality", []string{"CARDINALITY_MIN @ Bundle.entry[0].resource.subject"}},
		{"13_two_root_profiles_entry_resource_max0", []string{
			"CARDINALITY_MAX @ Bundle.entry[0].resource", "CARDINALITY_MAX @ Bundle.entry[0].resource"}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "ns_"+tt.probe+".json"))
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
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// within finds an issue in a nested resource by the prefixes of its expression that end before a
// ".": an element of the resource, not the element that holds it.
func TestWithinNestedResource(t *testing.T) {
	paths := map[string]bool{"Bundle.entry[1].resource": true, "Observation.contained[0]": true}
	for _, tt := range []struct {
		expression string
		want       bool
	}{
		{"Bundle.entry[1].resource.identifier:s", true},
		{"Bundle.entry[1].resource", false}, // the holder's element
		{"Bundle.entry[10].resource.identifier", false},
		{"Bundle.entry:doc", false},
		{"Observation.contained[0].subject", true},
		{"Observation.contained[0]", false},
		{"Observation.extension[0].url", false},
	} {
		if got := within([]string{tt.expression}, paths); got != tt.want {
			t.Errorf("within(%q) = %v, want %v", tt.expression, got, tt.want)
		}
	}
}
