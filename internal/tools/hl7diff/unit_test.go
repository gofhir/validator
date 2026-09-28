package main

import (
	"testing"
)

func TestNormalizeHL7Location(t *testing.T) {
	// Inputs are locations observed in HL7 validator 6.10.x output.
	cases := map[string]string{
		"Bundle.entry[1].resource/*MeasureReport/gaps-indv-measurereport01*/":                  "Bundle.entry[1].resource",
		"Bundle.entry[6].resource/*Patient/p*/.extension[0].extension[0].value.ofType(Coding)": "Bundle.entry[6].resource.extension[0].extension[0].valueCoding",
		"MeasureReport.extension[0].value.ofType(Identifier).system":                           "MeasureReport.extension[0].valueIdentifier.system",
		"MeasureReport.extension[0].value.ofType(string)":                                      "MeasureReport.extension[0].valueString",
		"Patient.extension[0].value.ofType(base64Binary)":                                      "Patient.extension[0].valueBase64Binary",
		"Observation.value.ofType(integer64)":                                                  "Observation.valueInteger64",
		"Bundle.entry[0].request":                                                              "Bundle.entry[0].request",
	}
	for in, want := range cases {
		if got := NormalizeHL7Location(in); got != want {
			t.Errorf("NormalizeHL7Location(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComparableGoLocation(t *testing.T) {
	cases := map[string]string{
		"Patient.identifier:B":                                          "Patient.identifier",
		"Observation.component:SystolicBP.code.coding:SBPCode":          "Observation.component.code.coding",
		"Bundle.entry[4].resource.code.coding[0]._display.extension[0]": "Bundle.entry[4].resource.code.coding[0].display.extension[0]",
		"Patient.identifier:us-core/social-history":                     "Patient.identifier",
	}
	for in, want := range cases {
		if got := ComparableGoLocation(in); got != want {
			t.Errorf("ComparableGoLocation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocated(t *testing.T) {
	cases := []struct {
		rule, g, h string
		want       bool
	}{
		{"parent", "Bundle.entry[0].request.method", "Bundle.entry[0].request", true}, // missing child at its parent
		{"parent", "Bundle.entry", "Bundle", true},                                    // required slice at the list owner
		{"parent", "Observation.component", "Observation", true},
		{"parent", "Bundle.entry", "Bundle.entry[3]", true},                       // index on one side only
		{"parent", "MeasureReport.extension[0].value[x]", "MeasureReport", false}, // not an immediate parent
		{"parent", "Bundle.entry[1]", "Bundle.entry[10]", false},
		{"equal", "Questionnaire.item[0]", "Questionnaire.item[0].item[0]", false}, // constraints: equal only
		{"equal", "Questionnaire.item[0].item[0]", "Questionnaire.item[0].item[0]", true},
	}
	for _, c := range cases {
		if got := Located(c.rule, c.g, c.h); got != c.want {
			t.Errorf("Located(%s, %q, %q) = %v, want %v", c.rule, c.g, c.h, got, c.want)
		}
	}
}

func TestAssignIsOrderIndependent(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	a := GoIssue{File: "f", Severity: "error", MessageID: "CARDINALITY_MIN", Expression: []string{"Observation.component"}}
	b := GoIssue{File: "f", Severity: "error", MessageID: "SLICING_CARDINALITY_MIN", Expression: []string{"Observation.component:SystolicBP"}}
	h := HL7Issue{File: "f", Severity: "error", Key: "Validation_VAL_Profile_Minimum", HasID: true, Location: "Observation"}
	u1 := Assign(fam, []GoIssue{a, b}, []HL7Issue{h})
	u2 := Assign(fam, []GoIssue{b, a}, []HL7Issue{h})
	if len(u1.GoFHIR) != 1 || len(u2.GoFHIR) != 1 || goIdentity(u1.GoFHIR[0]) != goIdentity(u2.GoFHIR[0]) {
		t.Fatalf("assignment depends on input order: %+v vs %+v", u1.GoFHIR, u2.GoFHIR)
	}
}

func TestCheckFailsClosed(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	run := func(files ...string) GoRun {
		r := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{}}
		for _, f := range files {
			r.Covered[f] = true
		}
		return r
	}
	hl7 := HL7Run{Errors: map[string][]HL7Issue{}, Covered: map[string]bool{"a": true}}
	if _, err := Check(fam, run("a", "b"), run("a", "b"), hl7, nil); err == nil {
		t.Error("a file HL7 did not validate must be an error, not a pass")
	}
	if _, err := Check(fam, run("a"), run("a", "b"), hl7, nil); err == nil {
		t.Error("a file only one gofhir run covered must be an error")
	}
	if _, err := Check(fam, run(), run(), HL7Run{Covered: map[string]bool{}}, nil); err == nil {
		t.Error("comparing nothing must be an error")
	}
}

func TestDivergenceScopes(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	g := GoIssue{File: "p/x.json", Severity: "error", MessageID: "SLICING_NO_MATCH", Expression: []string{"Patient.identifier[0]"}}
	base := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{"p/x.json": true}}
	head := GoRun{Errors: map[string][]GoIssue{"p/x.json": {g}}, Covered: map[string]bool{"p/x.json": true}}
	hl7 := HL7Run{Errors: map[string][]HL7Issue{}, Covered: map[string]bool{"p/x.json": true}}
	scoped := []Divergence{{Decision: "D-5", Reason: "spec", Side: "gofhir", File: "p/*.json", MessageID: "SLICING_NO_MATCH", Location: "Patient.identifier[0]"}}
	if rep, _ := Check(fam, base, head, hl7, scoped); !rep.OK() {
		t.Error("a declared divergence at its file and location must be set aside")
	}
	elsewhere := []Divergence{{Decision: "D-5", Reason: "spec", Side: "gofhir", File: "p/*.json", MessageID: "SLICING_NO_MATCH", Location: "Patient.identifier[1]"}}
	if rep, _ := Check(fam, base, head, hl7, elsewhere); rep.OK() {
		t.Error("a divergence must not cover another location")
	}
}

func TestSlicesAgree(t *testing.T) {
	h := HL7Issue{Text: "Slice 'Bundle.entry:composition': a matching slice is required, but not found"}
	if !slicesAgree(sliceNames("Bundle.entry:composition"), h) {
		t.Error("the same slice must agree")
	}
	if slicesAgree(sliceNames("Bundle.entry:allergyintolerance"), h) {
		t.Error("a different slice must not stand for the one HL7 names")
	}
	if !slicesAgree(sliceNames("Bundle.entry[0].request.method"), h) || !slicesAgree(nil, HL7Issue{Text: "minimum required = 1"}) {
		t.Error("when either side names no slice, slices do not decide")
	}
}

func TestVersionLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"0.11.0", "0.22.0", true}, {"5.9.0", "5.10.0", true}, {"5.10.0", "5.9.0", false}, {"5.4.0", "5.5.0", true}, {"1.0", "1.0.1", true}} {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestWildcardVersions(t *testing.T) {
	v, ok := highestMatching("3.3.x", []string{"3.2.0", "3.3.0", "3.3.2", "3.4.0"})
	if !ok || v != "3.3.2" {
		t.Errorf("3.3.x -> %q, %v; want 3.3.2", v, ok)
	}
	if _, ok := highestMatching("3.5.x", []string{"3.3.0"}); ok {
		t.Error("no match must be reported")
	}
	if !versionMatches("1.0.0", "1.0.0") || versionMatches("1.0.0", "1.0.1") || isWildcard("1.0.0") || !isWildcard("3.x") {
		t.Error("exact versions must match only themselves")
	}
}
