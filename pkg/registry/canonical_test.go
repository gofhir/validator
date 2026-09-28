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
		{url, "v1", ResolutionExact}, // no version pinned: the first loaded
		{url + "|", "v1", ResolutionExact},
		{url + "|1.0.0", "v1", ResolutionExact},
		{url + "|2.0.0", "v2", ResolutionExact},
		{url + "|3.0.0", "", ResolutionVersionMissing}, // D-2: never another version
		{"http://example.org/StructureDefinition/unknown", "", ResolutionNotFound},
		{"http://example.org/StructureDefinition/unknown|1.0.0", "", ResolutionNotFound},
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
