package main

import (
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The acceptance suite for the invariant tool (plan A, PR A0). Each case is one the PR #91 review
// used against the first implementation, which got every one of them wrong. Inputs are real
// outputs: gofhir v1.21.1 (corpusrun) and the HL7 validator 6.10.4 on the plan's probes. A case
// edits the baseline into a "head" the way a real PR would, and checks the verdict.

const probes = "testdata/m12-slice-scoping/probes/"

type fixture struct {
	base GoRun
	hl7  HL7Run
}

func loadFixture(t *testing.T, group string) fixture {
	t.Helper()
	base, err := ReadGo(filepath.Join("testdata", "acceptance", group+".base.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	hl7, err := ReadHL7(filepath.Join("testdata", "acceptance", group+".hl7.json"))
	if err != nil {
		t.Fatal(err)
	}
	return fixture{base, hl7}
}

func cloneRun(r GoRun) GoRun {
	c := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{}}
	maps.Copy(c.Covered, r.Covered)
	for f, gs := range r.Errors {
		c.Errors[f] = slices.Clone(gs)
	}
	return c
}

type edit func(t *testing.T, head GoRun)

func drop(file string, pred func(GoIssue) bool) edit {
	return func(t *testing.T, head GoRun) {
		t.Helper()
		f := probes + file
		before := len(head.Errors[f])
		head.Errors[f] = slices.DeleteFunc(head.Errors[f], pred)
		if len(head.Errors[f]) == before {
			t.Fatalf("edit removed nothing from %s", file)
		}
	}
}

func add(file, id, loc, diag string, times int) edit {
	return func(_ *testing.T, head GoRun) {
		f := probes + file
		for range times {
			head.Errors[f] = append(head.Errors[f], GoIssue{File: f, Severity: "error", MessageID: id, Expression: []string{loc}, Diagnostics: diag})
		}
	}
}

func keepOnly(file string, pred func(GoIssue) bool) edit {
	return func(_ *testing.T, head GoRun) {
		f := probes + file
		head.Errors[f] = slices.DeleteFunc(head.Errors[f], func(g GoIssue) bool { return !pred(g) })
	}
}

func at(loc string) func(GoIssue) bool { return func(g GoIssue) bool { return g.Location() == loc } }

func idAt(id, loc string) func(GoIssue) bool {
	return func(g GoIssue) bool { return g.MessageID == id && g.Location() == loc }
}

func TestAcceptance(t *testing.T) {
	fam, err := LoadFamilies()
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]fixture{}
	for _, g := range []string{"deqm-probes", "core-probes", "ips"} {
		fixtures[g] = loadFixture(t, g)
	}
	ext1 := regexp.MustCompile(`^Constraint failed: ext-1:`)
	requestResponse := regexp.MustCompile(`\.(method|url|status)$`)

	cases := []struct {
		name  string
		group string
		edits []edit
		pass  bool
	}{
		// Correct fixes from the plans must pass.
		{"A3: the false value[x] minimum is gone from P1, P5 and P6", "deqm-probes", []edit{
			drop("probe_P1_scoring_ok_prof.json", func(g GoIssue) bool { return strings.HasSuffix(g.Location(), ".value[x]") }),
			drop("probe_P5_cehrt_ok_prof.json", func(g GoIssue) bool { return strings.HasSuffix(g.Location(), ".value[x]") }),
			drop("probe_P6_unknown_ext_str_prof.json", func(g GoIssue) bool { return strings.HasSuffix(g.Location(), ".value[x]") }),
		}, true},
		{"B2: ext-1 reported once instead of 13 times, as HL7 does", "deqm-probes", []edit{
			func(_ *testing.T, head GoRun) {
				f := probes + "probe_P2_scoring_novalue_prof.json"
				seen := false
				head.Errors[f] = slices.DeleteFunc(head.Errors[f], func(g GoIssue) bool {
					if !ext1.MatchString(g.Diagnostics) {
						return false
					}
					if !seen {
						seen = true
						return false
					}
					return true
				})
			},
		}, true},
		{"relabel within a family (CARDINALITY_MIN to SLICING_CARDINALITY_MIN)", "deqm-probes", []edit{
			func(_ *testing.T, head GoRun) {
				f := probes + "probe_B2_request_nomethod.json"
				for i, g := range head.Errors[f] {
					if idAt("CARDINALITY_MIN", "Bundle.entry[0].request.method")(g) {
						head.Errors[f][i].MessageID = "SLICING_CARDINALITY_MIN"
					}
				}
			},
		}, true},
		{"A4: bp-no-systolic reports exactly HL7's two errors", "core-probes", []edit{
			keepOnly("r4_bp_no_systolic.json", func(g GoIssue) bool {
				return g.Location() == "Observation.component" || g.Location() == "Observation.component:SystolicBP"
			}),
		}, true},
		{"A4: bp-ok has no errors", "core-probes", []edit{keepOnly("r4_bp_ok.json", func(GoIssue) bool { return false })}, true},
		{"A3: nested Questionnaire item reports linkId and type", "core-probes", []edit{
			add("r4_q_nested.json", "CARDINALITY_MIN", "Questionnaire.item[0].item[0].linkId", "min", 1),
			add("r4_q_nested.json", "CARDINALITY_MIN", "Questionnaire.item[0].item[0].type", "min", 1),
		}, true},
		{"B2: que-1 on the nested item, where HL7 reports it", "core-probes", []edit{
			add("r4_q_nested.json", "CONSTRAINT_FAILED", "Questionnaire.item[0].item[0]", "Constraint failed: que-1: 'Group items must have nested items'", 1),
		}, true},
		{"A4: IPS all-sections without the false request/response and entry:patient", "ips", []edit{
			drop("r4_ips_all.json", func(g GoIssue) bool {
				return requestResponse.MatchString(g.Location()) || g.Location() == "Bundle.entry:patient"
			}),
		}, true},
		{"A2: IPS minimal without its two false slice errors", "ips", []edit{
			drop("r4_ips_minimal.json", func(g GoIssue) bool {
				return g.Location() == "Bundle.entry:composition" || g.Location() == "Bundle.entry:patient"
			}),
		}, true},

		{"A3 on P4: the false value[x] minimum is gone, the true cehrt slice error stays", "deqm-probes", []edit{
			drop("probe_P4_cehrt_string_prof.json", idAt("CARDINALITY_MIN", "MeasureReport.extension[0].value[x]")),
		}, true},

		// Regressions must fail.
		{"P4: only the true cehrt slice error is dropped, the false one stays", "deqm-probes", []edit{
			drop("probe_P4_cehrt_string_prof.json", idAt("SLICING_CARDINALITY_MIN", "MeasureReport.extension[0].value[x]")),
		}, false},
		{"P4: both the true and the false error are dropped", "deqm-probes", []edit{
			drop("probe_P4_cehrt_string_prof.json", idAt("SLICING_CARDINALITY_MIN", "MeasureReport.extension[0].value[x]")),
			drop("probe_P4_cehrt_string_prof.json", idAt("CARDINALITY_MIN", "MeasureReport.extension[0].value[x]")),
		}, false},
		{"a duplicated error (bdl-3 x1 to x13)", "deqm-probes", []edit{
			add("probe_B2_request_nomethod.json", "CONSTRAINT_FAILED", "Bundle", "Constraint failed: bdl-3: 'entry.request mandatory for batch/transaction/history, otherwise prohibited'", 12),
		}, false},
		{"an injected false minimum below a root-level HL7 error", "deqm-probes", []edit{
			add("probe_P1_scoring_ok_prof.json", "CARDINALITY_MIN", "MeasureReport.group[0].population[0].count", "injected", 1),
		}, false},
		{"a true error HL7 reports is removed (B2 request.method)", "deqm-probes", []edit{
			drop("probe_B2_request_nomethod.json", idAt("CARDINALITY_MIN", "Bundle.entry[0].request.method")),
		}, false},
		{"que-1 on the wrong node (the parent item)", "core-probes", []edit{
			add("r4_q_nested.json", "CONSTRAINT_FAILED", "Questionnaire.item[0]", "Constraint failed: que-1: 'Group items must have nested items'", 1),
		}, false},
		{"a swap: the true entry:composition error replaced by a false one", "ips", []edit{
			drop("r4_ips_all.json", at("Bundle.entry:composition")),
			add("r4_ips_all.json", "SLICING_CARDINALITY_MIN", "Bundle.entry:allergyintolerance", "x", 1),
		}, false},
		{"a swap: a true root-level finding replaced by a false one deep below the root", "core-probes", []edit{
			drop("r4_bp_no_systolic.json", idAt("CARDINALITY_MIN", "Observation.component")),
			add("r4_bp_no_systolic.json", "CARDINALITY_MIN", "Observation.component[0].code.coding[0].system", "deep", 1),
		}, false},
		{"an opposite finding (a maximum where HL7 reports a minimum)", "deqm-probes", []edit{
			add("probe_B2_request_nomethod.json", "CARDINALITY_MAX", "Bundle.entry[0].request.method", "max", 1),
		}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := fixtures[c.group]
			head := cloneRun(fx.base)
			for _, e := range c.edits {
				e(t, head)
			}
			rep, err := Check(fam, fx.base, head, fx.hl7, nil)
			if err != nil {
				t.Fatal(err)
			}
			if rep.OK() != c.pass {
				t.Errorf("want pass=%v, got pass=%v; findings:", c.pass, rep.OK())
				for _, f := range rep.Findings {
					if f.Go != nil {
						t.Errorf("  %s x%d %s @ %s [%s]", f.Kind, f.Count, f.File, f.Go.Location(), f.Go.MessageID)
					} else {
						t.Errorf("  %s x%d %s @ %s [%s]", f.Kind, f.Count, f.File, f.HL7.Location, f.HL7.Key)
					}
				}
			}
		})
	}

	t.Run("an unchanged run passes", func(t *testing.T) {
		for g, fx := range fixtures {
			rep, err := Check(fam, fx.base, cloneRun(fx.base), fx.hl7, nil)
			if err != nil || !rep.OK() {
				t.Fatalf("%s: err=%v findings=%d", g, err, len(rep.Findings))
			}
		}
	})
}
