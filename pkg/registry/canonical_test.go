package registry

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/gofhir/validator/pkg/loader"
)

func TestParseCanonical(t *testing.T) {
	tests := []struct {
		input       string
		wantURL     string
		wantVersion string
	}{
		{
			input:       "http://hl7.org/fhir/StructureDefinition/Patient|4.0.1",
			wantURL:     "http://hl7.org/fhir/StructureDefinition/Patient",
			wantVersion: "4.0.1",
		},
		{
			input:       "http://example.org/SD/my-profile|2.0.0",
			wantURL:     "http://example.org/SD/my-profile",
			wantVersion: "2.0.0",
		},
		{
			input:       "http://hl7.org/fhir/StructureDefinition/Patient",
			wantURL:     "http://hl7.org/fhir/StructureDefinition/Patient",
			wantVersion: "",
		},
		{
			input:       "",
			wantURL:     "",
			wantVersion: "",
		},
		{
			input:       "url|",
			wantURL:     "url",
			wantVersion: "",
		},
		{
			input:       "|version",
			wantURL:     "",
			wantVersion: "version",
		},
		{
			input:       "http://example.org/SD/pipe|in|url|1.0.0",
			wantURL:     "http://example.org/SD/pipe|in|url",
			wantVersion: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotURL, gotVersion := ParseCanonical(tt.input)
			if gotURL != tt.wantURL {
				t.Errorf("ParseCanonical(%q) url = %q, want %q", tt.input, gotURL, tt.wantURL)
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("ParseCanonical(%q) version = %q, want %q", tt.input, gotVersion, tt.wantVersion)
			}
		})
	}
}

// registryWith loads the given StructureDefinitions, one package per item so that load order is
// the order given.
func registryWith(t *testing.T, sds ...string) *Registry {
	t.Helper()
	r := New()
	for i, sd := range sds {
		pkg := &loader.Package{Name: "test", Version: strconv.Itoa(i), Resources: map[string]json.RawMessage{"sd": json.RawMessage(sd)}}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatalf("LoadFromPackages: %v", err)
		}
	}
	return r
}

func TestResolveCanonical(t *testing.T) {
	const url = "http://example.org/StructureDefinition/p"
	r := registryWith(t,
		`{"resourceType":"StructureDefinition","url":"`+url+`","version":"1.0.0","id":"v1"}`,
		`{"resourceType":"StructureDefinition","url":"`+url+`","version":"2.0.0","id":"v2"}`,
	)

	tests := []struct {
		canonical string
		wantID    string
		want      Resolution
	}{
		{url, "v2", ResolutionExact}, // no version pinned: the latest
		{url + "|", "v2", ResolutionExact},
		{url + "|1.0.0", "v1", ResolutionExact},
		{url + "|2.0.0", "v2", ResolutionExact},
		{url + "|3.0.0", "", ResolutionVersionMissing}, // D-2: never another version
		{"http://example.org/StructureDefinition/unknown", "", ResolutionNotFound},
		{"http://example.org/StructureDefinition/unknown|1.0.0", "", ResolutionNotFound},
		{url + "|1.0", "", ResolutionVersionMissing}, // partial versions are not matched (R4)
		{"", "", ResolutionNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.canonical, func(t *testing.T) {
			sd, res := r.ResolveCanonical(tt.canonical)
			if res != tt.want {
				t.Fatalf("resolution = %v, want %v", res, tt.want)
			}
			gotID := ""
			if sd != nil {
				gotID = sd.ID
			}
			if gotID != tt.wantID {
				t.Errorf("sd = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

// The latest version wins whatever the load order, and the index GetByCanonical uses is unchanged.
func TestResolveCanonicalLatestIgnoresLoadOrder(t *testing.T) {
	const url = "http://example.org/StructureDefinition/p"
	sd := func(v string) string {
		return `{"resourceType":"StructureDefinition","url":"` + url + `","version":"` + v + `","id":"` + v + `"}`
	}
	r := registryWith(t, sd("2.0.0"), sd("10.0.0"), sd("10.0.0-ballot"), sd("9.1.0"))
	if got, _ := r.ResolveCanonical(url); got == nil || got.ID != "10.0.0" {
		t.Errorf("latest = %v, want 10.0.0", got)
	}
	if got := r.GetByCanonical(url, ""); got == nil || got.ID != "2.0.0" {
		t.Errorf("GetByCanonical = %v, want the first loaded (unchanged)", got)
	}
}

func TestVersionLess(t *testing.T) {
	ordered := []string{"", "0.9", "1.0", "1.0.0-ballot", "1.0.0-ballot2", "1.0.0", "1.0.1", "1.2", "1.10.0", "2.0.0-snapshot1", "2.0.0", "10.0.0"}
	for i := range ordered {
		for j := range ordered {
			if got, want := versionLess(ordered[i], ordered[j]), i < j; got != want {
				t.Errorf("versionLess(%q, %q) = %v, want %v", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestIsSubtype(t *testing.T) {
	r := loadVersion(t, "4.0.1")
	for _, tt := range []struct {
		child, ancestor string
		want            bool
	}{
		{"Patient", "Patient", true},
		{"Patient", "DomainResource", true},
		{"Patient", "Resource", true},
		{"Bundle", "DomainResource", false},
		{"Age", "Quantity", true},
		{"Quantity", "Age", false},
		{"Patient", "Observation", false},
	} {
		if got := r.IsSubtype(tt.child, tt.ancestor); got != tt.want {
			t.Errorf("IsSubtype(%s, %s) = %v, want %v", tt.child, tt.ancestor, got, tt.want)
		}
	}
}
