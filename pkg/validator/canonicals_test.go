package validator

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A profile a nested resource declares is resolved as the root's is (plan B, B5): the version a
// canonical pins, or the one an unversioned canonical resolves to, with its snapshot generated from
// its differential. One that does not resolve is reported at its meta.profile entry, and a
// targetProfile that pins a version constrains the type of that version.
func TestVersionedCanonicals(t *testing.T) {
	const dir = "../../testdata/m12-slice-scoping"
	sources, err := filepath.Glob(filepath.Join(dir, "packages", "src", "acme.canonicals", "package", "StructureDefinition-*.json"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("definitions: %v", err)
	}
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
		want  []string // "<diagnostic> @ <location>"
	}{
		{"01_entry_versioned_bad", []string{"CARDINALITY_MIN @ Bundle.entry[0].resource.identifier"}},
		{"02_entry_unversioned_bad", []string{"CARDINALITY_MIN @ Bundle.entry[0].resource.identifier"}},
		{"03_entry_versioned_ok", nil},
		{"04_entry_version_missing", []string{"PROFILE_NOT_FOUND @ Bundle.entry[0].resource.meta.profile[0]"}},
		{"05_contained_versioned_bad", []string{"CARDINALITY_MIN @ Observation.contained[0].identifier"}},
		{"06_root_version_missing", []string{"PROFILE_NOT_FOUND @ Patient.meta.profile[0]"}},
		{"07_root_versioned_bad", []string{"CARDINALITY_MIN @ Patient.identifier"}},
		{"08_target_versioned_ok", nil},
		{"09_target_versioned_wrong_type", []string{"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.subject"}},
		{"10_entry_two_canonicals_one_profile", []string{"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.subject"}},
		{"11_entry_profile_of_other_type", []string{
			"CARDINALITY_MIN @ Bundle.entry[0].resource.identifier",
			"REFERENCE_INVALID_TARGET @ Bundle.entry[0].resource.subject", // its type's definition still applies
		}},
		{"12_root_same_missing_twice", []string{"PROFILE_NOT_FOUND @ Patient.meta.profile[0]", "PROFILE_NOT_FOUND @ Patient.meta.profile[1]"}},
		// The target a reference resolves to conforms to one of the profiles of its type (B5b).
		{"13_target_not_conformant", []string{"REFERENCE_TARGET_PROFILE @ Bundle.entry[0].resource.subject"}},
		{"14_target_conformant", nil},
		{"15_contained_target_not_conformant", []string{"REFERENCE_TARGET_PROFILE @ Observation.subject"}},
		{"16_contained_target_conformant", nil},
		{"17_target_unresolved", nil}, // not checked
	} {
		t.Run(tt.probe, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, "probes", "cn_"+tt.probe+".json"))
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
				case strings.HasPrefix(is.MessageID, "CARDINALITY_"), strings.HasPrefix(is.MessageID, "PROFILE_"),
					strings.HasPrefix(is.MessageID, "REFERENCE_"):
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

// A profile a nested resource declares that only the external profile resolver has is resolved
// through it, as the resource validated's are, whatever was validated before.
func TestNestedProfileThroughResolver(t *testing.T) {
	const dir = "../../testdata/m12-slice-scoping"
	pat, err := os.ReadFile(filepath.Join(dir, "packages", "src", "acme.canonicals", "package", "StructureDefinition-cn-pat.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(WithVersion("4.0.1"), WithProfileResolver(oneProfile{url: "https://example.org/fhir/cn/StructureDefinition/cn-pat", data: pat}))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "probes", "cn_01_entry_versioned_bad.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Validate(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range res.Issues {
		if strings.HasPrefix(is.MessageID, "CARDINALITY_") || strings.HasPrefix(is.MessageID, "PROFILE_") {
			got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
		}
	}
	if want := []string{"CARDINALITY_MIN @ Bundle.entry[0].resource.identifier"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// oneProfile resolves one StructureDefinition, by url.
type oneProfile struct {
	url  string
	data []byte
}

func (r oneProfile) ResolveProfile(_ context.Context, url, _ string) ([]byte, error) {
	if url == r.url {
		return r.data, nil
	}
	return nil, nil
}
