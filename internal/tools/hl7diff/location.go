package main

import (
	"regexp"
	"strings"
)

var (
	hl7Comment    = regexp.MustCompile(`/\*.*?\*/`)
	hl7OfType     = regexp.MustCompile(`\.ofType\(([A-Za-z][A-Za-z0-9]*)\)`)
	goSliceName   = regexp.MustCompile(`:[^.\[\]]+`)
	goPrimitiveEl = regexp.MustCompile(`\._([A-Za-z])`)
)

// NormalizeHL7Location rewrites an HL7 validator location into gofhir's instance-path syntax:
//
//	Bundle.entry[1].resource/*MeasureReport/x*/.extension[0] -> Bundle.entry[1].resource.extension[0]
//	MeasureReport.extension[0].value.ofType(Identifier)      -> MeasureReport.extension[0].valueIdentifier
//	Patient.extension[0].value.ofType(base64Binary)          -> Patient.extension[0].valueBase64Binary
func NormalizeHL7Location(loc string) string {
	loc = hl7Comment.ReplaceAllString(loc, "")
	loc = hl7OfType.ReplaceAllStringFunc(loc, func(m string) string {
		t := hl7OfType.FindStringSubmatch(m)[1]
		return strings.ToUpper(t[:1]) + t[1:]
	})
	return strings.TrimSpace(loc)
}

// ComparableGoLocation turns a gofhir location into the form HL7 locations are compared with:
// slice names are dropped (HL7 never writes them in instance paths), and a primitive's JSON
// "_element" key becomes the element (HL7 writes FHIRPath: Coding.display, not Coding._display).
// It is used only for matching; identities keep the raw location.
func ComparableGoLocation(loc string) string {
	loc = goSliceName.ReplaceAllString(strings.TrimSpace(loc), "")
	return goPrimitiveEl.ReplaceAllString(loc, ".$1")
}

// Located reports whether a gofhir location g and an HL7 location h satisfy a family's rule.
// "equal" requires the same element. "parent" also accepts one being the immediate parent of the
// other, which is where HL7 reports a missing child or a required slice. No other ancestor counts
// (an HL7 error at the resource root must not stand for every error below it), and indices must be
// the same: a location without an index is not a wildcard for the items of its list. Measured on
// the corpus, no real pair needed one, and allowing it let an arbitrary choice decide verdicts.
func Located(rule, g, h string) bool {
	gs, hs := strings.Split(g, "."), strings.Split(h, ".")
	same := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if same(gs, hs) {
		return true
	}
	if rule != ruleParent {
		return false
	}
	return (len(gs) == len(hs)+1 && same(gs[:len(hs)], hs)) || (len(hs) == len(gs)+1 && same(hs[:len(gs)], gs))
}
