// Package specs provides embedded FHIR specification packages for R4, R4B, and R5.
//
// Each .tgz contains only StructureDefinition, ValueSet, and CodeSystem resources,
// filtered from the full FHIR NPM packages to minimize binary size.
//
// R4 and R4B share the same terminology and extensions packages.
package specs

import _ "embed"

//go:embed r4/hl7.fhir.r4.core-4.0.1.tgz
var r4Core []byte

//go:embed r4/hl7.terminology.r4-7.0.1.tgz
var terminologyR4 []byte

//go:embed r4/hl7.fhir.uv.extensions.r4-5.2.0.tgz
var extensionsR4 []byte

//go:embed r4b/hl7.fhir.r4b.core-4.3.0.tgz
var r4bCore []byte

//go:embed r5/hl7.fhir.r5.core-5.0.0.tgz
var r5Core []byte

//go:embed r5/hl7.terminology.r5-7.0.1.tgz
var terminologyR5 []byte

//go:embed r5/hl7.fhir.uv.extensions.r5-5.2.0.tgz
var extensionsR5 []byte

var embeddedPackages = map[string][][]byte{
	"4.0.1": {r4Core, terminologyR4, extensionsR4},
	"4.3.0": {r4bCore, terminologyR4, extensionsR4},
	"5.0.0": {r5Core, terminologyR5, extensionsR5},
}

// GetPackages returns the embedded .tgz data for a FHIR version.
// Returns nil if the version is not embedded.
func GetPackages(version string) [][]byte {
	return embeddedPackages[version]
}

// HasVersion returns true if the version has embedded packages.
func HasVersion(version string) bool {
	_, ok := embeddedPackages[version]
	return ok
}
