// Package versionorder orders the business versions of FHIR packages and canonical resources.
package versionorder

import "strings"

// Less orders business versions the way semantic versioning does, which FHIR recommends for
// packages and canonical resources: dot-separated parts compare numerically when both are numbers,
// and a pre-release ("2.0.0-ballot") sorts below its release.
//
// A version that is not semver ("2018-08-12", as the R4 core package versions the code systems it
// carries) sorts below every semver one: the two schemes cannot be compared, and a terminology
// package's "11.0.0" is the latest rather than the core package's "2018-08-12", as the HL7
// validator resolves v3-ActCode too. Versions that are not semver still get a total order among themselves,
// part by part, so a choice between versions never depends on the order they were loaded in.
func Less(a, b string) bool {
	a, buildA, _ := strings.Cut(a, "+") // build metadata does not order versions (semver 10)
	b, buildB, _ := strings.Cut(b, "+")
	if a == "" || b == "" { // no version is below every version
		return a == "" && b != ""
	}
	if sa, sb := isSemver(a), isSemver(b); sa != sb {
		return sb
	}
	relA, preA, _ := strings.Cut(a, "-")
	relB, preB, _ := strings.Cut(b, "-")
	if c := compareDotted(relA, relB); c != 0 {
		return c < 0
	}
	switch {
	case preA == preB:
		return buildA < buildB // equal versions: any order that does not depend on load order
	case preA == "":
		return false // a release is above its pre-releases
	case preB == "":
		return true
	}
	return compareDotted(preA, preB) < 0
}

// isSemver reports whether a version (without build metadata) is major.minor or
// major.minor.patch, numbers, with an optional pre-release label after a "-".
func isSemver(v string) bool {
	rel, _, _ := strings.Cut(v, "-")
	parts := strings.Split(rel, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if !numeric(p) {
			return false
		}
	}
	return true
}

// numeric reports whether a version part is a number: digits only.
func numeric(p string) bool {
	if p == "" {
		return false
	}
	for _, c := range p {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// compareDotted compares dot-separated versions part by part, as semver orders pre-release
// identifiers (semver 11): numbers numerically, below parts that are not numbers, which compare as
// strings; a missing part sorts below a present one. Numbers that are equal but written apart
// ("01", "1") compare as strings, so the order is total.
func compareDotted(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		switch {
		case i >= len(pa):
			return -1
		case i >= len(pb):
			return 1
		}
		x, y := pa[i], pb[i]
		if x == y {
			continue
		}
		nx, ny := numeric(x), numeric(y)
		switch {
		case nx && ny:
			if c := compareNumbers(x, y); c != 0 {
				return c
			}
		case nx:
			return -1
		case ny:
			return 1
		}
		return strings.Compare(x, y)
	}
	return 0
}

// compareNumbers compares two strings of digits by value, whatever their length.
func compareNumbers(x, y string) int {
	x, y = strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
	if len(x) != len(y) {
		if len(x) < len(y) {
			return -1
		}
		return 1
	}
	return strings.Compare(x, y)
}

// SameRelease reports whether two FHIR versions belong to the same release: their first two parts
// agree ("4.0" for 4.0.0 and 4.0.1).
func SameRelease(a, b string) bool {
	return release(a) == release(b)
}

func release(v string) string {
	if i := strings.IndexByte(v, '.'); i >= 0 {
		if j := strings.IndexByte(v[i+1:], '.'); j >= 0 {
			return v[:i+1+j]
		}
	}
	return v
}
