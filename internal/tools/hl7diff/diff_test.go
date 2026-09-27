package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeHL7Location(t *testing.T) {
	// Every input is a location observed in HL7 validator 6.10.4 output.
	cases := map[string]string{
		"Bundle.entry[1].resource/*MeasureReport/gaps-indv-measurereport01*/":                               "Bundle.entry[1].resource",
		"Bundle.entry[6].resource/*Patient/gaps-patient01*/.extension[0].extension[0].value.ofType(Coding)": "Bundle.entry[6].resource.extension[0].extension[0].valueCoding",
		"MeasureReport.extension[0].value.ofType(Identifier).system":                                        "MeasureReport.extension[0].valueIdentifier.system",
		"MeasureReport.extension[0].value.ofType(string)":                                                   "MeasureReport.extension[0].valueString",
		"Bundle.entry[0].request": "Bundle.entry[0].request",
		"Patient":                 "Patient",
	}
	for in, want := range cases {
		if got := NormalizeHL7Location(in); got != want {
			t.Errorf("NormalizeHL7Location(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeGoLocation(t *testing.T) {
	cases := map[string]string{
		"Patient.identifier:B":                                 "Patient.identifier",
		"Bundle.entry:gaps-composition-deqm.request.method":    "Bundle.entry.request.method",
		"Observation.component:SystolicBP.code.coding:SBPCode": "Observation.component.code.coding",
		"MeasureReport.extension[0].value[x]":                  "MeasureReport.extension[0].value[x]",
		"Patient.identifier:us-core/social-history":            "Patient.identifier",
	}
	for in, want := range cases {
		if got := NormalizeGoLocation(in); got != want {
			t.Errorf("NormalizeGoLocation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRelated(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Bundle.entry[0].request", "Bundle.entry[0].request.method", true},
		{"Bundle.entry[0].request.method", "Bundle.entry[0].request", true},
		{"Patient", "Patient.identifier", true},
		{"Patient.identifier", "Patient.identifier[0]", true},
		{"Patient.name", "Patient.name", true},
		{"Patient.identifier", "Patient.identifierX", false},
		{"Bundle.entry[1]", "Bundle.entry[10]", false},
		{"Patient.name", "Patient.identifier", false},
	}
	for _, c := range cases {
		if got := Related(c.a, c.b); got != c.want {
			t.Errorf("Related(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func files(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func testFamilies(t *testing.T) *Families {
	t.Helper()
	f := &Families{}
	if err := json.Unmarshal([]byte(`{"families":[
		{"name":"cardinality","gofhir":["CARDINALITY_MIN","SLICING_CARDINALITY_MIN"],"hl7":["Validation_VAL_Profile_Minimum*"]},
		{"name":"slicing","gofhir":["SLICING_*"],"hl7":["Validation_VAL_Profile_MatchMultiple"]}
	]}`), f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDiff(t *testing.T) {
	const f = "p.json"
	err := func(key, loc string) Issue { return Issue{File: f, Severity: "error", Key: key, Location: loc} }
	all := files(f)
	fam := testFamilies(t)

	t.Run("new error with an HL7 equivalent is justified", func(t *testing.T) {
		hl7 := []Issue{err("Validation_VAL_Profile_Minimum", "Bundle.entry[0].request")}
		rep := Diff(nil, []Issue{err("CARDINALITY_MIN", "Bundle.entry[0].request.method")}, all, all, hl7, nil, fam)
		if len(rep.Findings) != 1 || !rep.Findings[0].Justified || rep.Findings[0].Kind != "new" {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("a root-level HL7 error of another family justifies nothing", func(t *testing.T) {
		hl7 := []Issue{err("http://hl7.org/fhir/StructureDefinition/Bundle#bdl-3", "Bundle")}
		rep := Diff(nil, []Issue{err("CARDINALITY_MIN", "Bundle.entry[0].method")}, all, all, hl7, nil, fam)
		if len(rep.Unjustified()) != 1 {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("constraints match by key, not by family table", func(t *testing.T) {
		g := Issue{File: f, Severity: "error", Key: "CONSTRAINT_FAILED", Location: "MeasureReport.extension[0]",
			Diagnostics: "Constraint failed: ext-1: 'Must have either extensions or value[x], not both'"}
		same := []Issue{err("http://hl7.org/fhir/StructureDefinition/Extension#ext-1", "MeasureReport.extension[0]")}
		other := []Issue{err("http://hl7.org/fhir/StructureDefinition/Extension#ext-2", "MeasureReport.extension[0]")}
		if rep := Diff(nil, []Issue{g}, all, all, same, nil, fam); len(rep.Unjustified()) != 0 {
			t.Fatalf("same key: got %+v", rep.Findings)
		}
		if rep := Diff(nil, []Issue{g}, all, all, other, nil, fam); len(rep.Unjustified()) != 1 {
			t.Fatalf("other key: got %+v", rep.Findings)
		}
	})
	t.Run("issues outside every family match only at the identical location", func(t *testing.T) {
		hl7 := []Issue{err("Something_Else", "Patient")}
		if rep := Diff(nil, []Issue{err("UNMAPPED", "Patient.name")}, all, all, hl7, nil, fam); len(rep.Unjustified()) != 1 {
			t.Fatalf("got %+v", rep.Findings)
		}
		hl7 = []Issue{err("Something_Else", "Patient.name")}
		if rep := Diff(nil, []Issue{err("UNMAPPED", "Patient.name")}, all, all, hl7, nil, fam); len(rep.Unjustified()) != 0 {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("new error without an HL7 equivalent breaks the invariant", func(t *testing.T) {
		rep := Diff(nil, []Issue{err("CARDINALITY_MIN", "Patient.name")}, all, all, nil, nil, fam)
		if len(rep.Unjustified()) != 1 {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("declared divergence justifies a new error", func(t *testing.T) {
		divs := []Divergence{{MessageID: "SLICING_OPEN_AT_END", Decision: "D-5", Reason: "spec"}}
		rep := Diff(nil, []Issue{err("SLICING_OPEN_AT_END", "Patient.identifier[0]")}, all, all, nil, divs, fam)
		if len(rep.Unjustified()) != 0 {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("gone error that HL7 does not report is justified", func(t *testing.T) {
		hl7 := []Issue{err("http://hl7.org/fhir/StructureDefinition/Bundle#bdl-3", "Bundle")}
		rep := Diff([]Issue{err("SLICING_CARDINALITY_MIN", "Bundle.entry[0].method")}, nil, all, all, hl7, nil, fam)
		if len(rep.Findings) != 1 || !rep.Findings[0].Justified || rep.Findings[0].Kind != "gone" {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("gone error that HL7 still reports breaks the invariant", func(t *testing.T) {
		hl7 := []Issue{err("Validation_VAL_Profile_Minimum", "Patient")}
		rep := Diff([]Issue{err("CARDINALITY_MIN", "Patient.name")}, nil, all, all, hl7, nil, fam)
		if len(rep.Unjustified()) != 1 {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("unchanged pairs and warnings are not findings", func(t *testing.T) {
		same := err("CARDINALITY_MIN", "Patient.name")
		warn := Issue{File: f, Severity: "warning", Key: "w", Location: "Patient"}
		rep := Diff([]Issue{same}, []Issue{same, warn}, all, all, nil, nil, fam)
		if len(rep.Findings) != 0 || rep.Unchanged != 1 {
			t.Fatalf("got %+v unchanged=%d", rep.Findings, rep.Unchanged)
		}
	})
	t.Run("a duplicated pair counts as new", func(t *testing.T) {
		same := err("INVARIANT", "MeasureReport.extension[0]")
		rep := Diff([]Issue{same}, []Issue{same, same}, all, all, nil, nil, fam)
		if len(rep.Unjustified()) != 1 {
			t.Fatalf("got %+v", rep.Findings)
		}
	})
	t.Run("files present in one run only are not compared", func(t *testing.T) {
		rep := Diff(nil, []Issue{{File: "other.json", Severity: "error", Key: "k"}}, all, files(f, "other.json"), nil, nil, fam)
		if len(rep.Findings) != 0 || len(rep.FilesOnly1) != 1 {
			t.Fatalf("got %+v", rep)
		}
	})
}

// TestFamiliesFile checks that the committed family table parses and classifies known IDs.
func TestFamiliesFile(t *testing.T) {
	fam, err := ReadFamilies(filepath.Join("..", "..", "..", "testdata", "m12-slice-scoping", "message-families.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		key  string
		hl7  bool
		want string
	}{
		{"CARDINALITY_MIN", false, "cardinality"},
		{"SLICING_CARDINALITY_MIN", false, "cardinality"},
		{"SLICING_NO_MATCH", false, "slicing"},
		{"Validation_VAL_Profile_Minimum_SLICE", true, "cardinality"},
		{"Validation_VAL_Profile_MatchMultiple", true, "slicing"},
		{"http://hl7.org/fhir/us/core/StructureDefinition/us-core-practitioner#us-core-17", true, "constraint:us-core-17"},
	}
	for _, c := range cases {
		if got := fam.classify(Issue{Key: c.key}, c.hl7); got != c.want {
			t.Errorf("classify(%q, hl7=%v) = %q, want %q", c.key, c.hl7, got, c.want)
		}
	}
}

// TestReadHL7RealOutput reads a real validator_cli -output file committed as evidence.
func TestReadHL7RealOutput(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "m12-slice-scoping", "hl7-validator-output.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("evidence file not present")
	}
	issues, err := ReadHL7(path)
	if err != nil {
		t.Fatal(err)
	}
	var withID, withFile int
	for _, i := range issues {
		if i.Key != "" {
			withID++
		}
		if i.File != "" && i.File != "." {
			withFile++
		}
	}
	if len(issues) == 0 || withFile != len(issues) || withID == 0 {
		t.Fatalf("issues=%d withFile=%d withID=%d", len(issues), withFile, withID)
	}
}
