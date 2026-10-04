package loader

import (
	"slices"
	"strings"
)

// coreCopiesErrata lists the FHIR versions whose specification packages (the core package, and the
// examples and expansions packages, which carry the same definitions) carry copies of definitions another
// package publishes: the R4 and R4B packages carry the code systems and value sets of
// terminology.hl7.org (consentpolicycodes, v3-ActCode, ...), versioned as the FHIR version
// ("4.0.1") or by date ("2018-08-12") rather than as HL7 Terminology (THO) versions them. HL7
// Terminology is where they are maintained (R5 terminologies-systems.html refers to it as the
// authoritative source), and the HL7 validator ranks THO's definitions above the core package's
// copies ("special case logic for UTG support prior to version 5"). By version alone, the copy
// would win: "4.0.1" is above THO's "3.0.1" for consentpolicycodes.
var coreCopiesErrata = map[string]bool{fhirR4: true, fhirR4B: true}

// The FHIR versions the errata lists.
const (
	fhirR4  = "4.0.1"
	fhirR4B = "4.3.0"
)

// Publishers tells, for the packages loaded, which definitions are copies: a definition that a
// specification package listed in the errata carries, whose URL is not under that package's own
// canonical but under the canonical of another package loaded (that package publishes it). A copy
// ranks below the publisher's definitions of the same URL, whatever their versions; where the
// publisher does not define the URL, the copy is used.
type Publishers struct {
	canonicals []string          // the canonicals of the other packages loaded
	carriers   map[string]string // "name#version" of the specification packages listed, to their canonical
}

// Add records a package loaded. It reports whether it changes which definitions are copies.
func (p *Publishers) Add(pkg *Package) bool {
	canonical := strings.TrimRight(pkg.Canonical, "/")
	if specificationPackage(pkg) {
		if !coreCopiesErrata[pkg.Version] {
			return false
		}
		id := pkg.Name + "#" + pkg.Version
		if _, ok := p.carriers[id]; ok {
			return false
		}
		if p.carriers == nil {
			p.carriers = map[string]string{}
		}
		p.carriers[id] = canonical
		return true
	}
	if canonical == "" || slices.Contains(p.canonicals, canonical) {
		return false
	}
	p.canonicals = append(p.canonicals, canonical)
	return true
}

// IsCopy reports whether the definition of url that the package packageID ("name#version") carries
// is a copy of one another package loaded publishes.
func (p *Publishers) IsCopy(packageID, url string) bool {
	own, carrier := p.carriers[packageID]
	if !carrier || under(url, own) {
		return false
	}
	for _, c := range p.canonicals {
		if under(url, c) {
			return true
		}
	}
	return false
}

// under reports whether url is under the canonical base given.
func under(url, canonical string) bool {
	return canonical != "" && strings.HasPrefix(url, canonical+"/")
}

// specificationPackage reports whether a package is one the FHIR specification publishes: its core
// package, or the examples or expansions package published with it (the NPM package
// specification's types fhir.core, fhir.examples and fhir.expansions).
func specificationPackage(pkg *Package) bool {
	return pkg.IsCore() || strings.EqualFold(pkg.Type, "fhir.examples") || strings.EqualFold(pkg.Type, "fhir.expansions")
}
