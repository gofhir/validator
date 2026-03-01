package registry

import "strings"

// ParseCanonical splits a FHIR canonical reference "url|version" into its parts.
// If no "|" is present, version is empty.
// Uses LastIndex because canonical URLs themselves may contain "|" in theory,
// but per FHIR convention the last "|" is the version separator.
//
// Examples:
//
//	"http://example.org/SD/patient|2.0.0" -> ("http://example.org/SD/patient", "2.0.0")
//	"http://example.org/SD/patient"       -> ("http://example.org/SD/patient", "")
func ParseCanonical(canonical string) (url, version string) {
	if idx := strings.LastIndex(canonical, "|"); idx >= 0 {
		return canonical[:idx], canonical[idx+1:]
	}
	return canonical, ""
}
