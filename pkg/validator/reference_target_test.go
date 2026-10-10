package validator

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/reference"
)

// A reference's target that resolves conforms to one of the profiles of its type its element
// allows (plan B, B5b). It resolves as bundle.html#references says: a relative reference from the
// referring entry's fullUrl base, an absolute one by its fullUrl, in the Bundle whose entry the
// referring resource is, a version matched against meta.versionId, "#id" among the container's
// contained resources. Resources that reference each other conform when nothing else is wrong with
// them. A profile of a type the target derives from (Resource, DomainResource) is met by any target.
// The verdicts are the HL7 validator's, but for the divergences declared: tp_11 (B-D15), tp_22 and
// tp_24 (B-D17), tp_30 (B-D16).
func TestReferenceTargetConformance(t *testing.T) {
	const dir = "../../testdata/m12-slice-scoping"
	sources, err := filepath.Glob(filepath.Join(dir, "packages", "src", "acme.targets", "package", "StructureDefinition-*.json"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("definitions: %v", err)
	}
	cycle, err := filepath.Glob(filepath.Join(dir, "packages", "src", "acme.nm", "package", "StructureDefinition-*.json"))
	if err != nil || len(cycle) == 0 {
		t.Fatalf("definitions: %v", err)
	}
	sources = append(sources, cycle...)
	defs := make([][]byte, 0, len(sources))
	for _, f := range sources {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs = append(defs, b)
	}
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // where REFERENCE_TARGET_PROFILE is reported
	}{
		{"01_urnuuid_relative", nil}, // a urn:uuid fullUrl is no base: not resolved
		{"02_versioned", []string{"Bundle.entry[0].resource.subject"}},
		{"03_absolute", []string{"Bundle.entry[0].resource.subject"}},
		{"04_other_base", nil}, // the target has another base
		{"05_two_bases", nil},  // the one with the referring entry's base conforms
		{"06_nested_bundle", []string{"Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"07_nested_shadow", nil}, // the inner Bundle's conforms
		{"09_contained_to_contained", []string{"Observation.contained[1].subject"}},
		{"10_mix_resource", nil},
		{"11_mix_domainresource", nil},
		{"12_cycle", nil},
		{"15_entry_two_profiles", []string{"Bundle.entry[0].resource.subject"}},
		{"16_cycle_via_obs", nil},
		{"17_root_obs_cycle_contained", nil},
		{"18_contained_target_relative", []string{"Bundle.entry[0].resource.subject"}}, // from the container's entry
		{"19_hash_container", []string{"Patient.contained[0].subject"}},                // "#": the container
		{"20_hash_container_ok", nil},
		{"21_cycle_one_bad", []string{"Bundle.entry[0].resource.subject", "Bundle.entry[1].resource.subject"}},
		// A cycle through a slice a profile discriminator assigns: B conforms to pb, whatever the
		// entries' order, since A does not conform to ps (HL7 reports B too when Z comes first).
		{"22_cycle_slice_z_first", []string{"Bundle.entry[0].resource.link[0].other"}},
		{"23_cycle_slice_c_first", []string{"Bundle.entry[1].resource.link[0].other"}},
		// The same, one check deeper: B's answer relied on assuming A, outside it, which fails (HL7
		// reports D too when Z comes first).
		{"24_cycle_slice_outer_z_first", []string{"Bundle.entry[0].resource.link[0].other"}},
		{"25_cycle_slice_outer_d_first", []string{"Bundle.entry[1].resource.link[0].other"}},
		{"26_nested_outer_target", nil}, // a nested Bundle's entry does not resolve in the outer Bundle
		// Not resolved from the nested Bundle, the target is still checked from the outer one.
		{"27_nested_then_flat", []string{"Bundle.entry[1].resource.subject"}},
		{"28_version_mismatch", nil},     // the entry is another version: not resolved
		{"29_history_two_versions", nil}, // the version referenced conforms
		// c references b, which references a, which does not conform: both whatever the order (HL7
		// reports only a's when a's referrer comes first, as in B-D16).
		{"30_cycle_three_a_first", []string{"Bundle.entry[0].resource.subject", "Bundle.entry[1].resource.subject"}},
		{"31_cycle_three_c_first", []string{"Bundle.entry[0].resource.subject", "Bundle.entry[1].resource.subject"}},
		{"32_history_unversioned", nil}, // two entries match: ambiguous, not resolved
		// A contained target's entry target resolves its own "#k", not among the container's.
		{"33_contained_target_entry", []string{"Bundle.entry[0].resource.subject"}},
		{"34_contained_target_entry_first", []string{"Bundle.entry[0].resource.subject", "Bundle.entry[1].resource.subject"}},
		{"35_contained_target_entry_second", []string{"Bundle.entry[0].resource.subject", "Bundle.entry[1].resource.subject"}},
		// A discriminator's check met again through a target's is assumed, as the target's is: a and
		// b, which link each other, conform whichever reference is checked first.
		{"36_discriminator_cycle_subject", nil},
		{"37_discriminator_cycle_focus", nil},
		{"38_discriminator_cycle_focus_first", nil},
		// A discriminator met again within itself answers false; what relied on that is dropped when
		// it answers true: z conforms, whatever the entries' order.
		{"39_discriminator_cycle_refuted", nil},
		{"40_discriminator_cycle_refuted_other_order", nil},
		{"45_root_two_profiles", []string{"Observation.subject"}}, // once, whichever profile finds it
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "probes", "tp_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if is.MessageID == "REFERENCE_TARGET_PROFILE" {
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

// A reference's target has the type of the resource it resolves to, whatever its literal says: a
// targetProfile that does not allow that type, or a Reference.type that names another, is an
// error, and the target is not checked against profiles of another type. A version resolves in an
// absolute reference too ("either relative or absolute", bundle.html#references), which HL7 does
// not resolve (B-D19).
func TestReferenceTargetType(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.targets")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>"
	}{
		{"41_entry_of_other_type", []string{"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.subject"}},
		{"42_type_element_vs_urn", []string{"REFERENCE_TYPE_MISMATCH @ Bundle.entry[0].resource.subject"}},
		{"43_base_entry_of_other_type", []string{"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.hasMember[0]"}},
		{"44_absolute_versioned", []string{"REFERENCE_TARGET_PROFILE @ Bundle.entry[0].resource.subject"}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "tp_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, is := range res.Issues {
				if strings.HasPrefix(is.MessageID, "REFERENCE_") && is.Severity == issue.SeverityError {
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

// Two root profiles report an issue once, but for a cardinality each sets, which each reports, as
// the HL7 validator reports it with its profile ("(from X)"): tp_45 reports its target once, tp_46
// the missing subject twice and dom-3 once.
func TestRootProfilesReportOnce(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.targets")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		probe string
		want  map[string]int // errors by diagnostic
	}{
		{"45_root_two_profiles", map[string]int{"REFERENCE_TARGET_PROFILE": 1}},
		{"46_root_two_profiles_cardinality", map[string]int{"CARDINALITY_MIN": 2, "CONSTRAINT_FAILED": 1}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "probes", "tp_"+tt.probe+".json"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]int{}
			for _, is := range res.Issues {
				if is.Severity == issue.SeverityError {
					got[is.MessageID]++
				}
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// patientGraph is a Bundle of n Patients, the i-th declaring profiles[i%len(profiles)] and linking
// the Patients links(i) names; those bad says lack an identifier.
func patientGraph(n int, profiles []string, links func(i int) []int, bad func(i int) bool) []byte {
	entries := make([]string, 0, n)
	for i := range n {
		var link []string
		for _, j := range links(i) {
			link = append(link, fmt.Sprintf(`{"other":{"reference":"Patient/p%d"},"type":"seealso"}`, ((j%n)+n)%n))
		}
		identifier := fmt.Sprintf(`"identifier":[{"value":"%d"}],`, i)
		if bad(i) {
			identifier = ""
		}
		entries = append(entries, fmt.Sprintf(`{"fullUrl":"http://example.org/fhir/Patient/p%d","resource":{"resourceType":"Patient","id":"p%d",
"meta":{"profile":["%s"]},%s"gender":"male","link":[%s]}}`, i, i, profiles[i%len(profiles)], identifier, strings.Join(link, ",")))
	}
	return []byte(`{"resourceType":"Bundle","type":"collection","entry":[` + strings.Join(entries, ",") + `]}`)
}

// definitions reads the StructureDefinitions of the packages' sources.
func definitions(t *testing.T, packages ...string) [][]byte {
	t.Helper()
	var defs [][]byte
	for _, pkg := range packages {
		sources, err := filepath.Glob(filepath.Join("..", "..", "testdata", "m12-slice-scoping", "packages", "src", pkg, "package", "StructureDefinition-*.json"))
		if err != nil || len(sources) == 0 {
			t.Fatalf("definitions: %v", err)
		}
		for _, f := range sources {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			defs = append(defs, b)
		}
	}
	return defs
}

// A graph of targets whose nodes each reference others that reference back is checked in a time
// polynomial in its size: an answer that relied on an assumed cycle is reused while that cycle's
// check runs, and a false that relied on one is kept when no discriminator swayed it (a check
// that fails then drops only the answers that could change).
func TestReferenceTargetCyclesStayPolynomial(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.targets")))
	if err != nil {
		t.Fatal(err)
	}
	const n = 26
	profile := []string{"https://example.org/fhir/rv/StructureDefinition/rv-pat"}
	for _, tt := range []struct {
		name  string
		links func(int) []int
		bad   func(int) bool
		want  int // REFERENCE_TARGET_PROFILE issues
	}{
		// Exponential, it took 50 s.
		{"all conform", func(i int) []int { return []int{i + 1, i + 2} }, func(int) bool { return false }, 0},
		// Exponential, it took 48 s: every Patient reaches the last, which does not conform.
		{"the last does not conform", func(i int) []int { return []int{i - 2, i - 1, i + 1, i + 2} },
			func(i int) bool { return i == n-1 }, 4 * n},
	} {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			res, err := v.Validate(context.Background(), patientGraph(n, profile, tt.links, tt.bad))
			if err != nil {
				t.Fatal(err)
			}
			took := time.Since(start)
			if took > 10*time.Second {
				t.Errorf("validation took %s", took)
			}
			t.Logf("validation took %s", took)
			got := 0
			for _, is := range res.Issues {
				if is.MessageID == "REFERENCE_TARGET_PROFILE" {
					got++
				}
			}
			if got != tt.want {
				t.Errorf("REFERENCE_TARGET_PROFILE: %d, want %d", got, tt.want)
			}
		})
	}
}

// A graph whose targets are sorted into slices by a profile discriminator is checked in bounded
// time: an answer a discriminator swayed is checked again when what it assumed fails, a bounded
// number of times (maxSwayedChecks). Unbounded, 25 Patients took 39 s.
func TestDiscriminatorCyclesStayBounded(t *testing.T) {
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(definitions(t, "acme.nm")))
	if err != nil {
		t.Fatal(err)
	}
	const base = "https://example.org/fhir/nm/StructureDefinition/"
	data := patientGraph(40, []string{base + "pz", base + "pb"},
		func(i int) []int { return []int{i - 2, i - 1, i + 1, i + 2} }, func(i int) bool { return i%3 == 0 })
	start := time.Now()
	if _, err := v.Validate(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	if took > 10*time.Second {
		t.Errorf("validation took %s", took)
	}
	t.Logf("validation took %s", took)
}

// Validations that share a context do not share their entry indexes, which key Bundles by
// address: a Bundle validated after another, which the collector may have freed, gets its own.
func TestEntryIndexesPerValidation(t *testing.T) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	// Two Bundles of one shape, so that the second may take the first's address: the first holds
	// the entry the Observation references, the second does not.
	bundle := func(patient string) []byte {
		return []byte(`{"resourceType":"Bundle","type":"collection","entry":[
{"fullUrl":"urn:uuid:aaaaaaaa-0000-0000-0000-00000000000` + patient + `","resource":{"resourceType":"Patient"}},
{"fullUrl":"urn:uuid:aaaaaaaa-0000-0000-0000-000000000002","resource":{"resourceType":"Observation","status":"final",
"code":{"text":"x"},"subject":{"reference":"urn:uuid:aaaaaaaa-0000-0000-0000-000000000001"}}}]}`)
	}
	ctx := reference.WithIndexes(context.Background())
	missed := 0
	for range 200 {
		if _, err := v.Validate(ctx, bundle("1")); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		res, err := v.Validate(ctx, bundle("9"))
		if err != nil {
			t.Fatal(err)
		}
		if issueFor(t, res, issue.DiagReferenceNotInBundle) == nil {
			missed++
		}
		runtime.GC()
	}
	if missed > 0 {
		t.Errorf("%s not reported %d times in 200", issue.DiagReferenceNotInBundle, missed)
	}
}

// The references in a value checked against a profile resolve where the value is: a nested
// Bundle's entries in that Bundle, a urn:uuid reference to its type there, a datatype's "#id" among
// its resource's contained resources, a target's "#id" among its own; resolve() then looks in the
// Bundles that hold it. The verdicts are the HL7 validator's, but for vs_22, where HL7 also takes
// a type from one of the entries an ambiguous reference matches.
func TestReferencesInCheckedValues(t *testing.T) {
	const dir = "../../testdata/m12-slice-scoping"
	var defs [][]byte
	for _, pkg := range []string{"acme.targets", "acme.vscope"} {
		sources, err := filepath.Glob(filepath.Join(dir, "packages", "src", pkg, "package", "StructureDefinition-*.json"))
		if err != nil || len(sources) == 0 {
			t.Fatalf("definitions: %v", err)
		}
		for _, f := range sources {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			defs = append(defs, b)
		}
	}
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	const (
		doc = "SLICING_CARDINALITY_MIN @ Bundle.entry:doc"
		obs = "SLICING_CARDINALITY_MIN @ Bundle.entry:obs"
	)
	for _, tt := range []struct {
		probe string
		want  []string // "<diagnostic> @ <location>"
	}{
		{"01_nested_urn_bad", []string{doc}},
		{"02_nested_urn_ok", nil},
		{"03_inner_urn_bad", []string{obs}},
		{"04_inner_urn_metaprofile", []string{"REFERENCE_TARGET_PROFILE @ Bundle.entry[0].resource.subject"}},
		{"05_nested_urn_metaprofile", []string{"REFERENCE_TARGET_PROFILE @ Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"06_inner_restful_bad", []string{obs}},
		{"07_nested_restful_bad", []string{doc}},
		{"08_nested_no_obs", []string{doc}},
		{"09_nested_restful_ok", nil},
		{"10_nested_urn_collision", nil}, // the inner Bundle's urn:uuid is a Patient
		{"11_datatype_contained_ref", nil},
		{"12_datatype_no_ref", nil},
		// resolve() in a nested Bundle's constraint finds the outer Bundle's entry.
		{"13_nested_resolve_outer", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"14_nested_resolve_outer_declared", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"15_nested_resolve_inner", nil},
		// A nested Bundle the constraints walk resolves its entries' urn:uuid references in it, not
		// in the outer Bundle, whose same urn:uuid is a Practitioner: the value is in its slice.
		{"16_nested_datatype_urn_collision", []string{"vs-2 @ Bundle.entry[0].resource.entry[0].resource.identifier[0]"}},
		{"17_nested_datatype", []string{"vs-2 @ Bundle.entry[0].resource.entry[0].resource.identifier[0]"}},
		{"18_inner_declared_urn_collision", []string{"vs-3 @ Bundle.entry[0].resource.entry[0]"}},
		{"19_inner_declared", []string{"vs-3 @ Bundle.entry[0].resource.entry[0]"}},
		// A datatype's target resolves its own "#c", not among the datatype's resource's contained.
		{"20_target_contained_ref", []string{"vs-8 @ Bundle.entry[0].resource.identifier[0]"}},
		{"21_target_no_contained", []string{"vs-8 @ Bundle.entry[0].resource.identifier[0]"}},
		{"22_urn_ambiguous", []string{"REFERENCE_MULTIPLE_MATCHES @ Bundle.entry[0].resource.subject"}},
		// An extension's context invariant's resolve() in a nested Bundle checked as a value finds
		// the outer Bundle's entry, as a profile's invariant does.
		{"23_extension_resolve_outer", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"24_no_extension_outer", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.subject"}},
		{"25_extension_resolve_inner", nil},
		// A target resolve() finds in the outer Bundle resolves its own references there.
		{"26_resolve_outer_target_shadowed", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.focus[0]"}},
		{"27_resolve_outer_target_bad", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.focus[0]", doc}},
		{"28_resolve_outer_target_ok", []string{"REFERENCE_NOT_IN_BUNDLE @ Bundle.entry[0].resource.entry[0].resource.focus[0]"}},
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "probes", "vs_"+tt.probe+".json"))
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
				case strings.HasPrefix(is.MessageID, "SLICING_"), strings.HasPrefix(is.MessageID, "REFERENCE_"):
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				case strings.HasPrefix(is.Diagnostics, "Constraint failed: vs-"):
					key, _, _ := strings.Cut(strings.TrimPrefix(is.Diagnostics, "Constraint failed: "), ":")
					got = append(got, key+" @ "+strings.Join(is.Expression, ","))
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
