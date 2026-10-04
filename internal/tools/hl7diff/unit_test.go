package main

import (
	"os"
	"path/filepath"
	"slices"
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

func TestNamesAgree(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
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
		{"MeasureReport.extension:cehrt.value[x]", "Slice 'MeasureReport.extension:cehrt.value[x]:valueIdentifier': a matching slice is required", true}, // a type slice is not an element slice
		{"Observation.component:SystolicBP.code", "Observation.component:DiastolicBP.code: minimum required = 1", false},                                 // slices named in the location differ
		{"Patient.name", "The Extension 'x' definition is for a simple extension, so it must contain a value", true},                                     // unnamed
	}
	for _, c := range cases {
		if got := namesAgree(fam, GoIssue{Expression: []string{c.loc}}, h(c.text)); got != c.want {
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
	// error (E.a) can pair with the HL7 error at its parent (E) or at itself (E.a); the second (E.b)
	// only with the one at the parent. Every error pairs only if the first gives way.
	g := func(loc string) GoIssue {
		return GoIssue{File: "f", Severity: "error", MessageID: "EXTENSION_INVALID_VALUE_TYPE", Expression: []string{loc}}
	}
	h := func(loc string) HL7Issue {
		return HL7Issue{File: "f", Severity: "error", Key: "Extension_EXT_Type", HasID: true, Location: loc, Text: "The Extension 'u' definition allows for the types [Identifier] but found type string"}
	}
	u := Assign(fam, []GoIssue{g("E.a"), g("E.b")}, []HL7Issue{h("E"), h("E.a")})
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
	t.Run("1: a missing value reported on a child element cannot stand for the extension's", func(t *testing.T) {
		x := hl7(h("Extension_EXT_Simple_ABSENT", "MeasureReport.extension[0]", "The Extension 'u' definition is for a simple extension, so it must contain a value"))
		base := run(g("EXTENSION_VALUE_REQUIRED", "MeasureReport.extension[0]", "requires a value"))
		head := run(g("EXTENSION_VALUE_REQUIRED", "MeasureReport.extension[0].url", "requires a value"))
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
	t.Run("6: a list-level finding moved to one item is a finding to inspect", func(t *testing.T) {
		// HL7 reports slice cardinality at the list's owner. An owner never pairs with an item of
		// its list (it would stand for every item), so moving gofhir's report from the list to one
		// item cannot be verified and is reported: conservative by design, never a false pass.
		x := hl7(h("Validation_VAL_Profile_Maximum", "Bundle", "Bundle.entry:composition: max allowed = 1, but found 2"))
		base := run(g("SLICING_CARDINALITY_MAX", "Bundle.entry:composition", "max"))
		head := run(g("SLICING_CARDINALITY_MAX", "Bundle.entry[1]", "max"))
		if verdict(base, head, x) {
			t.Error("must fail")
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

// The fourth review of the redesign.
func TestFourthReviewScenarios(t *testing.T) {
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
	extType := func(loc string) HL7Issue {
		return h("Extension_EXT_Type", loc, "The Extension 'u' definition allows for the types [Identifier] but found type string")
	}

	t.Run("2: a location made precise among sibling items passes", func(t *testing.T) {
		x := hl7(extType("E.extension[0]"), extType("E.extension[1]"))
		base := run(g("EXTENSION_INVALID_VALUE_TYPE", "E", "type"))
		head := run(g("EXTENSION_INVALID_VALUE_TYPE", "E.extension[1]", "type"))
		if !ok(base, head, x) {
			t.Error("must pass")
		}
	})
	t.Run("4: an unnamed finding moved to the wrong item fails", func(t *testing.T) {
		x := hl7(extType("P"))
		base := run(g("EXTENSION_INVALID_VALUE_TYPE", "P.extension[1]", "type"))
		head := run(g("EXTENSION_INVALID_VALUE_TYPE", "P.extension[0]", "type"))
		if ok(base, head, x) {
			// Neither item pairs with the owner, so the move shows as a new false positive.
			t.Error("must fail")
		}
	})
}

func TestPortableNamesDoNotCollide(t *testing.T) {
	a := "/home/u/.fhir/packages/p#1/package/example/X.json"
	b := "/srv/vendored/.fhir/packages/p#1/package/example/X.json"
	run := func(files ...string) GoRun {
		r := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{}}
		for _, f := range files {
			r.Covered[f] = true
		}
		return r
	}
	base, head := run(a, b), run(a, b)
	hl7 := HL7Run{Errors: map[string][]HL7Issue{}, Covered: map[string]bool{a: true, b: true}}
	if err := portable(&base, &head, &hl7); err == nil {
		t.Error("two files with one portable name must be an error")
	}
	base, head = run(a), run(a)
	hl7 = HL7Run{Errors: map[string][]HL7Issue{}, Covered: map[string]bool{a: true}}
	if err := portable(&base, &head, &hl7); err != nil || !base.Covered["fhir-cache:/p#1/package/example/X.json"] {
		t.Errorf("one file renames cleanly: %v", err)
	}
}

// The fifth review of the redesign.
func TestFifthReviewScenarios(t *testing.T) {
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

	t.Run("4: errors differing only in a type slice have different identities", func(t *testing.T) {
		base := run(g("SLICING_CARDINALITY_MIN", "Observation.value[x].unit", "Minimum cardinality of 'Observation.value[x]:valueQuantity.unit' is 1"))
		head := run(g("SLICING_CARDINALITY_MIN", "Observation.value[x].unit", "Minimum cardinality of 'Observation.value[x]:valueString.unit' is 1"))
		if ok(base, head, hl7()) {
			t.Error("must fail")
		}
	})
	t.Run("5: a different type slice does not pair when both sides name one", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Minimum_SLICE", "MeasureReport.extension[0]", "Slice 'MeasureReport.extension:cehrt.value[x]:valueIdentifier': a matching slice is required, but not found"))
		base := run(g("SLICING_CARDINALITY_MIN", "MeasureReport.extension[0].value[x]", "Minimum cardinality of 'MeasureReport.extension:cehrt.value[x]' is 1, but found 0"))
		head := run(g("SLICING_CARDINALITY_MIN", "MeasureReport.extension[0].value[x]", "Minimum cardinality of 'MeasureReport.extension:cehrt.value[x]:valueString' is 1, but found 0"))
		if ok(base, head, x) {
			t.Error("must fail")
		}
	})
	t.Run("3: dropping a true error the rules could not pair is flagged", func(t *testing.T) {
		// HL7 reports slice max at the owner; gofhir at an item, which never pairs with an owner.
		x := hl7(h("Validation_VAL_Profile_Maximum", "Bundle", "Bundle.entry:composition: max allowed = 1, but found 2"))
		base := run(g("SLICING_CARDINALITY_MAX", "Bundle.entry[1]", "max"))
		if ok(base, run(), x) {
			t.Error("an unverifiable removal must be reported")
		}
	})
	t.Run("an element minimum does not stand for a required slice", func(t *testing.T) {
		x := hl7(h("Validation_VAL_Profile_Minimum_SLICE", "MeasureReport.extension[0]", "Slice 'MeasureReport.extension:cehrt.value[x]:valueIdentifier': a matching slice is required, but not found"))
		u := Assign(fam, []GoIssue{g("CARDINALITY_MIN", "MeasureReport.extension[0].value[x]", "Minimum cardinality of 'MeasureReport.extension[0].value[x]' is 1")}, x.Errors[f])
		if len(u.Paired) != 0 {
			t.Error("CARDINALITY_MIN must not pair with HL7's required-slice message")
		}
	})
}

func TestPortableNamesMergeAcrossMachines(t *testing.T) {
	ci := "/home/ci/.fhir/packages/p#1/package/example/X.json"
	laptop := "/Users/me/.fhir/packages/p#1/package/example/X.json"
	base := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{ci: true}}
	head := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{laptop: true}}
	hl7 := HL7Run{Errors: map[string][]HL7Issue{}, Covered: map[string]bool{laptop: true}}
	if err := portable(&base, &head, &hl7); err != nil {
		t.Fatalf("the same example cached on two machines is one file: %v", err)
	}
	if !base.Covered["fhir-cache:/p#1/package/example/X.json"] || !head.Covered["fhir-cache:/p#1/package/example/X.json"] {
		t.Error("both runs must use the portable name")
	}
}

func TestApplyExclusions(t *testing.T) {
	files := []string{"/c/pkg/A.json", "/c/pkg/B.json", "probes/C.json", "/c/other/B2.json"}
	kept, excluded, err := applyExclusions("g", files, []Exclusion{{File: "B.json", Reason: "crash"}, {File: "C.json", Reason: "crash"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/c/pkg/A.json", "/c/other/B2.json"}; !slices.Equal(kept, want) {
		t.Errorf("kept = %v, want %v", kept, want)
	}
	if len(excluded) != 2 {
		t.Errorf("excluded = %v", excluded)
	}
	if kept, _, err := applyExclusions("g", files, nil); err != nil || !slices.Equal(kept, files) {
		t.Errorf("no exclusions: %v, %v", kept, err)
	}

	// A stale exclusion, or one that would leave out more than it names, is an error.
	if _, _, err := applyExclusions("g", files, []Exclusion{{File: "gone.json", Reason: "x"}}); err == nil {
		t.Error("an exclusion that matches no file must fail")
	}
	twice := append(slices.Clone(files), "/c/elsewhere/A.json")
	if _, _, err := applyExclusions("g", twice, []Exclusion{{File: "A.json", Reason: "x"}}); err == nil {
		t.Error("an exclusion that matches several files must fail")
	}
}

func TestReadManifestExclusions(t *testing.T) {
	dir := t.TempDir()
	for name, exclude := range map[string]string{
		"no reason":  `[{"file":"a.json"}]`,
		"blank":      `[{"file":"a.json","reason":"  "}]`,
		"no file":    `[{"reason":"x"}]`,
		"path":       `[{"file":"dir/a.json","reason":"x"}]`,
		"twice":      `[{"file":"a.json","reason":"x"},{"file":"a.json","reason":"y"}]`,
		"misspelled": `[{"file":"a.json","reson":"x"}]`,
	} {
		path := filepath.Join(dir, "m.json")
		body := `{"groups":[{"name":"g","files":["*.json"],"exclude":` + exclude + `}]}`
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readManifest(path); err == nil {
			t.Errorf("%s: accepted %s", name, exclude)
		}
	}
	path := filepath.Join(dir, "ok.json")
	if err := os.WriteFile(path, []byte(`{"groups":[{"name":"g","files":["*.json"],"exclude":[{"file":"a.json","reason":"crash"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(path)
	if err != nil || len(m.Groups[0].Exclude) != 1 {
		t.Errorf("valid exclusion: %v, %v", m, err)
	}
}

// The packages the HL7 validator loaded are read from its log.
func TestParsePackageSummary(t *testing.T) {
	log := "  Load hl7.terminology.r4#6.2.0 - 4288 resources\n  Package Summary: [hl7.fhir.r4.core#4.0.1, hl7.terminology.r4#6.2.0, hl7.fhir.uv.extensions#5.3.0]\n  Get set...\n"
	got, err := ParsePackageSummary(log)
	if err != nil {
		t.Fatal(err)
	}
	if want := "hl7.fhir.r4.core#4.0.1,hl7.terminology.r4#6.2.0,hl7.fhir.uv.extensions#5.3.0"; joinIDs(got) != want {
		t.Errorf("got %s, want %s", joinIDs(got), want)
	}
	if _, err := ParsePackageSummary("no summary here"); err == nil {
		t.Error("a log with no Package Summary is accepted")
	}
}

// gofhir runs with each base package in the version the HL7 validator uses of its family: the
// highest among the flavors it loaded, under gofhir's name; a family it did not load keeps gofhir's.
func TestEffectiveBase(t *testing.T) {
	embedded := map[string]string{"hl7.fhir.r4.core": "4.0.1", "hl7.terminology.r4": "7.0.1", "hl7.fhir.uv.extensions.r4": "5.2.0", "x.only.gofhir": "1.0.0"}
	loaded := []PackageID{
		{"hl7.fhir.r4.core", "4.0.1"}, {"hl7.terminology.r4", "6.2.0"}, {"hl7.terminology", "7.4.0"}, {"hl7.terminology.r5", "7.1.0"},
		{"hl7.fhir.uv.extensions.r4", "5.2.0"}, {"hl7.fhir.uv.extensions", "5.3.0"}, {"hl7.fhir.uv.extensions.r5", "5.2.0"},
	}
	want := "hl7.fhir.r4.core#4.0.1,hl7.fhir.uv.extensions.r4#5.3.0,hl7.terminology.r4#7.4.0,x.only.gofhir#1.0.0"
	if got := joinIDs(EffectiveBase(embedded, loaded)); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
