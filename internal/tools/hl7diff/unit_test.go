package main

import (
	"os"
	"strings"
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
		{"parent", "Bundle.entry", "Bundle.entry[3]", false},                      // an index is not optional
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
	if len(u1.GoFHIR) != 1 || len(u2.GoFHIR) != 1 || fam.GoIdentity(u1.GoFHIR[0]) != fam.GoIdentity(u2.GoFHIR[0]) {
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

func TestVersionLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"0.11.0", "0.22.0", true}, {"5.9.0", "5.10.0", true}, {"5.10.0", "5.9.0", false}, {"5.4.0", "5.5.0", true}, {"1.0", "1.0.1", true}, {"5.3.0-ballot", "5.3.0", true}, {"5.3.0", "5.3.0-ballot", false}, {"5.3.0-ballot", "5.4.0", true}} {
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

func TestNamesAgree(t *testing.T) {
	h := func(text string) HL7Issue { return HL7Issue{Text: text} }
	cases := []struct {
		loc, text string
		want      bool
	}{
		{"Bundle.entry[0].request.method", "Bundle.entry:gaps-composition-deqm.request.method: minimum required = 1, but only found 0", true},
		{"Bundle.entry:composition", "Bundle.identifier: minimum required = 1, but only found 0", false}, // review finding 1
		{"Bundle.entry:composition", "Slice 'Bundle.entry:composition': a matching slice is required, but not found", true},
		{"MedicationRequest.dispenseRequest.quantity.comparator", "Quantity.comparator: max allowed = 0, but found 1", true},
		{"MeasureReport.extension[0].valueIdentifier", "MeasureReport.extension:cehrt.value[x]: minimum required = 1", true},
		{"Patient.name", "The Extension 'x' definition is for a simple extension, so it must contain a value", true}, // unnamed
	}
	for _, c := range cases {
		if got := namesAgree(GoIssue{Expression: []string{c.loc}}, h(c.text)); got != c.want {
			t.Errorf("namesAgree(%q, %q) = %v, want %v", c.loc, c.text, got, c.want)
		}
	}
}

func TestConstraintIdentityIgnoresDefinitionSite(t *testing.T) {
	a := GoIssue{Severity: "error", MessageID: "CONSTRAINT_FAILED", Expression: []string{"X"}, Diagnostics: "Constraint failed: ext-1: 'Must have...' (defined in http://hl7.org/fhir/StructureDefinition/Extension)"}
	b := a
	b.Diagnostics = "Constraint failed: ext-1: 'Must have...'"
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	if fam.GoIdentity(a) != fam.GoIdentity(b) {
		t.Error("where a constraint was defined must not change its identity")
	}
}

func TestEqualRuleNeedsExactIndices(t *testing.T) {
	if Located("equal", "Questionnaire.item", "Questionnaire.item[3]") {
		t.Error("under the equal rule a missing index must not match any index")
	}
	if Located("parent", "Bundle.entry", "Bundle.entry[3]") {
		t.Error("a missing index is not a wildcard, under any rule")
	}
}

// Review of the redesign, finding 1: which error a matching leaves unpaired must not decide the
// verdict.
func TestMatchingChoiceDoesNotDecideVerdict(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	const f = "b.json"
	cov := map[string]bool{f: true}
	g := func(id, loc string) GoIssue {
		return GoIssue{File: f, Severity: "error", MessageID: id, Expression: []string{loc}}
	}
	hMin := func(text string) HL7Issue {
		return HL7Issue{File: f, Severity: "error", Key: "Validation_VAL_Profile_Minimum", HasID: true, Location: "Bundle", Text: text}
	}
	hl7 := HL7Run{Covered: cov, Errors: map[string][]HL7Issue{f: {
		hMin("Bundle.identifier: minimum required = 1, but only found 0"),
		{File: f, Severity: "error", Key: "Validation_VAL_Profile_Minimum_SLICE", HasID: true, Location: "Bundle",
			Text: "Slice 'Bundle.entry:composition': a matching slice is required, but not found"},
	}}}
	run := func(gs ...GoIssue) GoRun { return GoRun{Covered: cov, Errors: map[string][]GoIssue{f: gs}} }

	// The slice error must pair with the slice, not with Bundle.identifier, so losing it is seen.
	base := run(g("SLICING_CARDINALITY_MIN", "Bundle.entry:composition"))
	lost := run(g("CARDINALITY_MIN", "Bundle.identifier"))
	if rep, _ := Check(fam, base, lost, hl7, nil); rep.OK() {
		t.Error("dropping the true slice error must fail even when an unrelated true error appears")
	}

	// A location without an index is not a wildcard: it pairs with no item, so making it precise is
	// an improvement, never a loss.
	h2 := HL7Run{Covered: cov, Errors: map[string][]HL7Issue{f: {
		{File: f, Severity: "error", Key: "Validation_VAL_Profile_Maximum", HasID: true, Location: "Bundle.entry[0]", Text: "Bundle.entry.x: max allowed = 1, but found 2"},
		{File: f, Severity: "error", Key: "Validation_VAL_Profile_Maximum", HasID: true, Location: "Bundle.entry[1]", Text: "Bundle.entry.x: max allowed = 1, but found 2"},
	}}}
	vague := run(g("CARDINALITY_MAX", "Bundle.entry.x"))
	precise := run(g("CARDINALITY_MAX", "Bundle.entry[1].x"))
	if rep, _ := Check(fam, vague, precise, h2, nil); !rep.OK() {
		t.Errorf("making a location precise must pass: %+v", rep.Findings)
	}
}

func TestReadDivergencesRejectsUnscoped(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"no location": `[{"decision":"D-5","reason":"r","side":"gofhir","file":"x.json","messageId":"SLICING_NO_MATCH"}]`,
		"failure":     `[{"decision":"D-5","reason":"r","side":"gofhir","file":"x.json","location":"X","messageId":"FAILURE"}]`,
		"bad side":    `[{"decision":"D-5","reason":"r","side":"both","file":"x.json","location":"X","messageId":"A"}]`,
		"bad glob":    `[{"decision":"D-5","reason":"r","side":"hl7","file":"[","location":"X","messageId":"A"}]`,
	} {
		p := dir + "/" + strings.ReplaceAll(name, " ", "_") + ".json"
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadDivergences(p, fam); err == nil {
			t.Errorf("%s: an unscoped divergence must be rejected", name)
		}
	}
}

func TestPortableName(t *testing.T) {
	for _, f := range []string{"/home/u/.fhir/packages/hl7.fhir.us.core#6.1.0/package/example/X.json", "/Users/v/.fhir/packages/hl7.fhir.us.core#6.1.0/package/example/X.json"} {
		if got := portableName(f); got != "fhir-cache:/hl7.fhir.us.core#6.1.0/package/example/X.json" {
			t.Errorf("cache file on any machine: %q", got)
		}
	}
	if got := portableName("testdata/m12-slice-scoping/probes/p.json"); got != "testdata/m12-slice-scoping/probes/p.json" {
		t.Errorf("repository file: %q", got)
	}
}

// The matching must be maximum: a first error that could pair with either of two HL7 errors must
// give way when a second error can pair only with the one it took.
func TestAssignIsMaximum(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	// Extension value types use the parent rule and HL7's message names no element. The first
	// error can pair with the HL7 error at its parent or at itself; the second only with the one
	// at the parent. Every error pairs only if the first gives way.
	g := func(loc string) GoIssue {
		return GoIssue{File: "f", Severity: "error", MessageID: "EXTENSION_INVALID_VALUE_TYPE", Expression: []string{loc}}
	}
	h := func(loc string) HL7Issue {
		return HL7Issue{File: "f", Severity: "error", Key: "Extension_EXT_Type", HasID: true, Location: loc, Text: "The Extension 'u' definition allows for the types [Identifier] but found type string"}
	}
	u := Assign(fam, []GoIssue{g("E.extension[0]"), g("E.x")}, []HL7Issue{h("E"), h("E.extension[0]")})
	if len(u.GoFHIR) != 0 || len(u.HL7) != 0 {
		t.Errorf("want every error paired, got %d gofhir and %d HL7 unpaired", len(u.GoFHIR), len(u.HL7))
	}
}

func TestReadDivergencesAcceptsRealIDs(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	p := t.TempDir() + "/d.json"
	body := `[{"decision":"D-5","reason":"r","side":"gofhir","file":"x.json","location":"X","messageId":"SLICING_NO_MATCH"},
	 {"decision":"D-1","reason":"r","side":"hl7","file":"x.json","location":"X","messageId":"Validation_VAL_Profile_MatchMultiple"},
	 {"decision":"D-9","reason":"r","side":"hl7","file":"x.json","location":"X","messageId":"http://hl7.org/fhir/StructureDefinition/Extension#ext-1"}]`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if ds, err := ReadDivergences(p, fam); err != nil || len(ds) != 3 {
		t.Fatalf("real IDs must be accepted: %v", err)
	}
}

// The second review of the redesign: each scenario, with the verdict it must get.
func TestSecondReviewScenarios(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	const f = "x.json"
	cov := map[string]bool{f: true}
	run := func(gs ...GoIssue) GoRun { return GoRun{Covered: cov, Errors: map[string][]GoIssue{f: gs}} }
	hl7 := func(hs ...HL7Issue) HL7Run { return HL7Run{Covered: cov, Errors: map[string][]HL7Issue{f: hs}} }
	g := func(id, loc, diag string) GoIssue {
		return GoIssue{File: f, Severity: "error", MessageID: id, Expression: []string{loc}, Diagnostics: diag}
	}
	h := func(id, loc, text string) HL7Issue {
		return HL7Issue{File: f, Severity: "error", Key: id, HasID: true, Location: loc, Text: text}
	}
	verdict := func(base, head GoRun, x HL7Run) bool {
		rep, err := Check(fam, base, head, x, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rep.OK()
	}

	t.Run("1: a false child error cannot stand for a missing extension value", func(t *testing.T) {
		x := hl7(h("Extension_EXT_Simple_ABSENT", "MeasureReport.extension[0]", "The Extension 'u' definition is for a simple extension, so it must contain a value"))
		base := run(g("EXTENSION_VALUE_REQUIRED", "MeasureReport.extension[0]", "requires a value"))
		head := run(g("CARDINALITY_MIN", "MeasureReport.extension[0].url", "min"))
		if verdict(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("1: a nested extension's missing value cannot stand for its parent's", func(t *testing.T) {
		x := hl7(h("Extension_EXT_Simple_ABSENT", "MeasureReport.extension[0]", "The Extension 'u' definition is for a simple extension, so it must contain a value"))
		base := run(g("EXTENSION_VALUE_REQUIRED", "MeasureReport.extension[0]", "requires a value"))
		head := run(g("EXTENSION_VALUE_REQUIRED", "MeasureReport.extension[0].extension[1]", "requires a value"))
		if verdict(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("1: an unknown element is named by HL7", func(t *testing.T) {
		x := hl7(HL7Issue{File: f, Severity: "error", Key: "structure", Code: "structure", Location: "Questionnaire.item[0]", Text: "Unrecognized property 'bogus'"})
		base := run(g("STRUCTURE_UNKNOWN_ELEMENT", "Questionnaire.item[0].bogus", "unknown"))
		head := run(g("STRUCTURE_UNKNOWN_ELEMENT", "Questionnaire.item[0].linkId", "unknown"))
		if verdict(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("3: a regression on one entry does not cancel an improvement on another", func(t *testing.T) {
		base := run(g("CARDINALITY_MIN", "Bundle.entry[0].resource.subject", "min"))
		head := run(g("CARDINALITY_MIN", "Bundle.entry[3].resource.subject", "min"))
		if verdict(base, head, hl7()) {
			t.Error("must fail")
		}
	})
	t.Run("4: a swap between slice children at one instance path", func(t *testing.T) {
		base := run(g("SLICING_CARDINALITY_MIN", "Observation.component[0].system", "Minimum cardinality of 'Observation.component:SystolicBP.system' is 1, but found 0"))
		head := run(g("SLICING_CARDINALITY_MIN", "Observation.component[0].system", "Minimum cardinality of 'Observation.component:DiastolicBP.system' is 1, but found 0"))
		if verdict(base, head, hl7()) {
			t.Error("must fail")
		}
	})
	t.Run("5: an error about another ancestor slice does not pair", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Minimum_SLICE", "MeasureReport.extension[0]", "Slice 'MeasureReport.extension:cehrt.value[x]:valueIdentifier': a matching slice is required, but not found"))
		base := run(g("SLICING_CARDINALITY_MIN", "MeasureReport.extension:cehrt.value[x]:valueIdentifier", "min"))
		head := run(g("SLICING_CARDINALITY_MIN", "MeasureReport.extension:other.value[x]:valueIdentifier", "min"))
		if verdict(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("6: a slice location made an instance path still pairs", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Maximum", "Bundle", "Bundle.entry:composition: max allowed = 1, but found 2"))
		base := run(g("SLICING_CARDINALITY_MAX", "Bundle.entry:composition", "max"))
		head := run(g("SLICING_CARDINALITY_MAX", "Bundle.entry[1]", "max"))
		if !verdict(base, head, x) {
			t.Error("must pass")
		}
	})
}

// The third review of the redesign: each scenario, with the verdict it must get.
func TestThirdReviewScenarios(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	const f = "x.json"
	cov := map[string]bool{f: true}
	run := func(gs ...GoIssue) GoRun { return GoRun{Covered: cov, Errors: map[string][]GoIssue{f: gs}} }
	hl7 := func(hs ...HL7Issue) HL7Run { return HL7Run{Covered: cov, Errors: map[string][]HL7Issue{f: hs}} }
	g := func(id, loc, diag string) GoIssue {
		return GoIssue{File: f, Severity: "error", MessageID: id, Expression: []string{loc}, Diagnostics: diag}
	}
	h := func(id, loc, text string) HL7Issue {
		return HL7Issue{File: f, Severity: "error", Key: id, HasID: true, Location: loc, Text: text}
	}
	ok := func(base, head GoRun, x HL7Run) bool {
		rep, err := Check(fam, base, head, x, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rep.OK()
	}

	t.Run("1: a slice named only in gofhir's message still has to match HL7's", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Minimum", "Observation.component[0]", "Observation.component:SystolicBP.system: minimum required = 1, but only found 0"))
		base := run(g("SLICING_CARDINALITY_MIN", "Observation.component[0].system", "Minimum cardinality of 'Observation.component:SystolicBP.system' is 1, but found 0"))
		head := run(g("SLICING_CARDINALITY_MIN", "Observation.component[0].system", "Minimum cardinality of 'Observation.component:DiastolicBP.system' is 1, but found 0"))
		if ok(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("2: an HL7 error lost on one entry is not offset by one gained on another", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Minimum", "Bundle.entry[0]", "Bundle.entry.request: minimum required = 1, but only found 0"),
			h("Validation_VAL_Profile_Minimum", "Bundle.entry[3]", "Bundle.entry.request: minimum required = 1, but only found 0"))
		base := run(g("CARDINALITY_MIN", "Bundle.entry[3].request", "min"))
		head := run(g("CARDINALITY_MIN", "Bundle.entry[0].request", "min"))
		if ok(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("2: a false positive moved to another entry is new", func(t *testing.T) {
		base := run(g("SLICING_NO_MATCH", "Bundle.entry[0]", "no match"))
		head := run(g("SLICING_NO_MATCH", "Bundle.entry[3]", "no match"))
		if ok(base, head, hl7()) {
			t.Error("must fail")
		}
	})
	t.Run("3: a location made precise inside an inner list passes", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Minimum", "Bundle.entry[0].resource", "Bundle.entry:composition.resource.subject: minimum required = 1"),
			h("Validation_VAL_Profile_Minimum", "Bundle.entry[2].resource", "Bundle.entry:composition.resource.subject: minimum required = 1"))
		base := run(g("CARDINALITY_MIN", "Bundle.entry:composition.resource.subject", "min"))
		head := run(g("CARDINALITY_MIN", "Bundle.entry[2].resource.subject", "min"))
		if !ok(base, head, x) {
			t.Error("must pass")
		}
	})
	t.Run("5: a fullUrl mismatch moved to a child does not pair", func(t *testing.T) {
		x := hl7(h("Bundle_BUNDLE_Entry_IdUrlMismatch", "Bundle.entry[0]", "fullUrl mismatch"))
		base := run(g("BUNDLE_FULLURL_ID_MISMATCH", "Bundle.entry[0]", "mismatch"))
		head := run(g("BUNDLE_FULLURL_ID_MISMATCH", "Bundle.entry[0].resource", "mismatch"))
		if ok(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("7: a constraint divergence must look like a canonical", func(t *testing.T) {
		p := t.TempDir() + "/d.json"
		body := `[{"decision":"D-9","reason":"r","side":"hl7","file":"x.json","location":"X","messageId":"Validation_VAL_Profile_Minmum#x"}]`
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadDivergences(p, fam); err == nil {
			t.Error("a made-up constraint ID must be rejected")
		}
	})
}
