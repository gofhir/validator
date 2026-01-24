package fhirvalidator

import (
	"testing"
)

func TestFHIRVersion_String(t *testing.T) {
	tests := []struct {
		version FHIRVersion
		want    string
	}{
		{R4, "R4"},
		{R4B, "R4B"},
		{R5, "R5"},
	}

	for _, tt := range tests {
		if got := tt.version.String(); got != tt.want {
			t.Errorf("%v.String() = %q; want %q", tt.version, got, tt.want)
		}
	}
}

func TestFHIRVersion_IsValid(t *testing.T) {
	tests := []struct {
		version FHIRVersion
		want    bool
	}{
		{R4, true},
		{R4B, true},
		{R5, true},
		{"R3", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.version.IsValid(); got != tt.want {
			t.Errorf("%v.IsValid() = %v; want %v", tt.version, got, tt.want)
		}
	}
}

func TestGetVersionConfig_R4(t *testing.T) {
	cfg, ok := getVersionConfig(R4)
	if !ok {
		t.Fatal("getVersionConfig(R4) returned false")
	}

	if cfg.CorePackageName != "hl7.fhir.r4.core" {
		t.Errorf("CorePackageName = %q; want %q", cfg.CorePackageName, "hl7.fhir.r4.core")
	}
	if cfg.CorePackageVersion != "4.0.1" {
		t.Errorf("CorePackageVersion = %q; want %q", cfg.CorePackageVersion, "4.0.1")
	}
	if cfg.TermPackageName != "hl7.terminology.r4" {
		t.Errorf("TermPackageName = %q; want %q", cfg.TermPackageName, "hl7.terminology.r4")
	}
	if cfg.FHIRVersionString != "4.0.1" {
		t.Errorf("FHIRVersionString = %q; want %q", cfg.FHIRVersionString, "4.0.1")
	}
}

func TestGetVersionConfig_R4B(t *testing.T) {
	cfg, ok := getVersionConfig(R4B)
	if !ok {
		t.Fatal("getVersionConfig(R4B) returned false")
	}

	if cfg.CorePackageName != "hl7.fhir.r4b.core" {
		t.Errorf("CorePackageName = %q; want %q", cfg.CorePackageName, "hl7.fhir.r4b.core")
	}
	if cfg.CorePackageVersion != "4.3.0" {
		t.Errorf("CorePackageVersion = %q; want %q", cfg.CorePackageVersion, "4.3.0")
	}
	if cfg.FHIRVersionString != "4.3.0" {
		t.Errorf("FHIRVersionString = %q; want %q", cfg.FHIRVersionString, "4.3.0")
	}
}

func TestGetVersionConfig_R5(t *testing.T) {
	cfg, ok := getVersionConfig(R5)
	if !ok {
		t.Fatal("getVersionConfig(R5) returned false")
	}

	if cfg.CorePackageName != "hl7.fhir.r5.core" {
		t.Errorf("CorePackageName = %q; want %q", cfg.CorePackageName, "hl7.fhir.r5.core")
	}
	if cfg.CorePackageVersion != "5.0.0" {
		t.Errorf("CorePackageVersion = %q; want %q", cfg.CorePackageVersion, "5.0.0")
	}
	if cfg.TermPackageName != "hl7.terminology.r5" {
		t.Errorf("TermPackageName = %q; want %q", cfg.TermPackageName, "hl7.terminology.r5")
	}
	if cfg.FHIRVersionString != "5.0.0" {
		t.Errorf("FHIRVersionString = %q; want %q", cfg.FHIRVersionString, "5.0.0")
	}
}

func TestGetVersionConfig_Invalid(t *testing.T) {
	_, ok := getVersionConfig("R3")
	if ok {
		t.Error("getVersionConfig(R3) should return false")
	}
}

func BenchmarkFHIRVersion_IsValid(b *testing.B) {
	versions := []FHIRVersion{R4, R4B, R5, "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = versions[i%len(versions)].IsValid()
	}
}
