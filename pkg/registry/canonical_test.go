package registry

import "testing"

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
