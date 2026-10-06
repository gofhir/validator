package registry

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
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

// The latest version wins whatever the load order, for every lookup without a version.
func TestResolveCanonicalLatestIgnoresLoadOrder(t *testing.T) {
	const url = "http://example.org/StructureDefinition/p"
	sd := func(v string) string {
		return `{"resourceType":"StructureDefinition","url":"` + url + `","version":"` + v + `","id":"` + v + `"}`
	}
	r := registryWith(t, sd("2.0.0"), sd("10.0.0"), sd("10.0.0-ballot"), sd("9.1.0"))
	if got, _ := r.ResolveCanonical(url); got == nil || got.ID != "10.0.0" {
		t.Errorf("ResolveCanonical = %v, want 10.0.0", got)
	}
	if got := r.GetByCanonical(url, ""); got == nil || got.ID != "10.0.0" {
		t.Errorf("GetByCanonical = %v, want 10.0.0", got)
	}
	if got := r.GetByURL(url); got == nil || got.ID != "10.0.0" {
		t.Errorf("GetByURL = %v, want 10.0.0", got)
	}
	if got := r.GetByCanonical(url, "9.1.0"); got == nil || got.ID != "9.1.0" {
		t.Errorf("GetByCanonical 9.1.0 = %v, want 9.1.0", got)
	}
}

// Among the versions of a URL, those written for the FHIR version validated come first: an R5
// flavor of a guide does not replace the R4 one when R4 is validated, even at a higher version.
func TestResolveCanonicalPrefersTheFHIRVersionValidated(t *testing.T) {
	const url = "http://example.org/StructureDefinition/p"
	sd := func(id, v, fhir string) string {
		return `{"resourceType":"StructureDefinition","url":"` + url + `","version":"` + v + `","id":"` + id +
			`","fhirVersion":"` + fhir + `","type":"P","derivation":"specialization"}`
	}
	r := registryWith(t, sd("r4", "1.0.0", "4.0.1"), sd("r5", "2.0.0", "5.0.0"), sd("r5same", "1.0.0", "5.0.0"),
		sd("r40", "1.5.0", "4.0.0"))
	if got := r.GetByURL(url); got == nil || got.ID != "r5" {
		t.Errorf("no FHIR version set: %v, want the highest version, r5", got)
	}

	r.SetFHIRVersion("4.0.1")
	for _, tt := range []struct{ canonical, want string }{
		{url, "r4"},            // written for 4.0.1, although lower than the others
		{url + "|1.0.0", "r4"}, // the same version in two flavors: the R4 one
		{url + "|2.0.0", "r5"}, // pinned: exactly that one, whatever it is written for
		{url + "|1.5.0", "r40"},
	} {
		if got, _ := r.ResolveCanonical(tt.canonical); got == nil || got.ID != tt.want {
			t.Errorf("ResolveCanonical(%s) = %v, want %s", tt.canonical, got, tt.want)
		}
	}
	if got := r.GetByType("P"); got == nil || got.ID != "r4" {
		t.Errorf("GetByType = %v, want r4", got)
	}

	// One that states no FHIR version is not taken for another's.
	unstated := `{"resourceType":"StructureDefinition","url":"` + url + `","version":"3.0.0","id":"unstated"}`
	r = registryWith(t, sd("r4", "1.0.0", "4.0.1"), unstated, sd("r5", "4.0.0", "5.0.0"))
	r.SetFHIRVersion("4.0.1")
	if got := r.GetByURL(url); got == nil || got.ID != "unstated" {
		t.Errorf("GetByURL = %v, want unstated", got)
	}

	// Without one for 4.0.1, the same release (4.0) comes before another.
	r = registryWith(t, sd("r5", "2.0.0", "5.0.0"), sd("r40", "1.5.0", "4.0.0"))
	r.SetFHIRVersion("4.0.1")
	if got := r.GetByURL(url); got == nil || got.ID != "r40" {
		t.Errorf("GetByURL = %v, want r40", got)
	}
}

// The R4 core package's copy of a definition another package loaded publishes ranks below that
// package's, although its version is higher, whatever the order they are loaded in.
func TestCoreCopyRanksBelowThePublisher(t *testing.T) {
	const url = "http://terminology.hl7.org/StructureDefinition/p"
	sd := func(v string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"sd": json.RawMessage(`{"resourceType":"StructureDefinition","url":"` + url + `","version":"` + v + `","id":"` + v + `"}`)}
	}
	core := &loader.Package{Name: "hl7.fhir.r4.core", Version: "4.0.1", Type: "fhir.core", Canonical: "http://hl7.org/fhir", Resources: sd("4.0.1")}
	tho := &loader.Package{Name: "hl7.terminology.r4", Version: "7.4.0", Type: "IG", Canonical: "http://terminology.hl7.org", Resources: sd("3.0.1")}
	for _, order := range [][][]*loader.Package{{{core, tho}}, {{tho, core}}, {{core}, {tho}}, {{tho}, {core}}} {
		r := New()
		r.SetFHIRVersion("4.0.1")
		for _, packages := range order {
			if err := r.LoadFromPackages(packages); err != nil {
				t.Fatal(err)
			}
		}
		if got, _ := r.ResolveCanonical(url); got == nil || got.Version != "3.0.1" {
			t.Errorf("ResolveCanonical = %v, want the publisher's 3.0.1", got)
		}
		if got, _ := r.ResolveCanonical(url + "|4.0.1"); got == nil || got.Version != "4.0.1" {
			t.Errorf("ResolveCanonical|4.0.1 = %v, want the copy", got)
		}
	}
}

func TestIsSubtype(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
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

func TestChoiceType(t *testing.T) {
	r := sharedVersion(t, "4.0.1")
	for _, tt := range []struct{ base, key, want string }{
		{"value", "valueQuantity", "Quantity"},
		{"value", "valueBoolean", "boolean"},
		{"value", "valueDateTime", "dateTime"},
		{"effective", "effectivePeriod", "Period"},
		{"value", "value", ""},
		{"value", "valuestring", ""}, // the type's first letter is capitalized
		{"value", "valueFoo", ""},    // no such type
		{"value", "other", ""},
	} {
		if got := r.ChoiceType(tt.base, tt.key); got != tt.want {
			t.Errorf("ChoiceType(%q, %q) = %q, want %q", tt.base, tt.key, got, tt.want)
		}
	}
}
