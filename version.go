package fhirvalidator

// FHIRVersion represents a FHIR specification version.
type FHIRVersion string

// Supported FHIR versions.
const (
	// R4 is FHIR Release 4 (4.0.1)
	R4 FHIRVersion = "R4"
	// R4B is FHIR Release 4B (4.3.0)
	R4B FHIRVersion = "R4B"
	// R5 is FHIR Release 5 (5.0.0)
	R5 FHIRVersion = "R5"
)

// String returns the version string.
func (v FHIRVersion) String() string {
	return string(v)
}

// IsValid returns true if this is a supported FHIR version.
func (v FHIRVersion) IsValid() bool {
	switch v {
	case R4, R4B, R5:
		return true
	default:
		return false
	}
}

// versionConfig holds version-specific configuration.
type versionConfig struct {
	// CorePackage is the FHIR core package name and version
	CorePackageName    string
	CorePackageVersion string

	// TerminologyPackage is the HL7 terminology package name and version
	TermPackageName    string
	TermPackageVersion string

	// FHIRVersionString is the version string used in StructureDefinitions
	FHIRVersionString string
}

// versionConfigs maps FHIR versions to their configurations.
var versionConfigs = map[FHIRVersion]versionConfig{
	R4: {
		CorePackageName:    "hl7.fhir.r4.core",
		CorePackageVersion: "4.0.1",
		TermPackageName:    "hl7.terminology.r4",
		TermPackageVersion: "6.2.0",
		FHIRVersionString:  "4.0.1",
	},
	R4B: {
		CorePackageName:    "hl7.fhir.r4b.core",
		CorePackageVersion: "4.3.0",
		TermPackageName:    "hl7.terminology.r4",
		TermPackageVersion: "6.2.0",
		FHIRVersionString:  "4.3.0",
	},
	R5: {
		CorePackageName:    "hl7.fhir.r5.core",
		CorePackageVersion: "5.0.0",
		TermPackageName:    "hl7.terminology.r5",
		TermPackageVersion: "6.2.0",
		FHIRVersionString:  "5.0.0",
	},
}

// getVersionConfig returns the configuration for a FHIR version.
func getVersionConfig(v FHIRVersion) (versionConfig, bool) {
	cfg, ok := versionConfigs[v]
	return cfg, ok
}
