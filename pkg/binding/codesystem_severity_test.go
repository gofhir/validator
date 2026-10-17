package binding

import (
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
)

// A code its code system cannot check is reported as HL7 6.10.2 reports it, by context and system.
func TestReportUnchecked(t *testing.T) {
	const (
		acme  = "http://acme.local/cs"
		tho   = "http://terminology.hl7.org/CodeSystem/acme"
		loinc = "http://loinc.org"
		ndc   = "http://hl7.org/fhir/sid/ndc"
		srt   = "http://hl7.org/fhir/sid/srt"
		bcp47 = "urn:ietf:bcp:47"
		ndfrt = "http://hl7.org/fhir/ndfrt"
		cpt   = "http://www.ama-assn.org/go/cpt"
		ucum  = "http://unitsofmeasure.org"
	)
	bare := codingContext{}
	checked := codingContext{checked: true}
	required := codingContext{checked: true, required: true}
	concept := codingContext{inConcept: true}
	conceptRequired := codingContext{inConcept: true, checked: true, required: true}
	none := uncheckedReport{}
	r := func(s issue.Severity, id issue.DiagnosticID, at string) uncheckedReport {
		return uncheckedReport{severity: s, id: id, at: at, report: true}
	}
	for _, tt := range []struct {
		name   string
		why    unchecked
		system string
		c      codingContext
		want   uncheckedReport
	}{
		{"unknown, no binding", notFound, acme, bare, r(issue.SeverityInformation, issue.DiagCodeSystemNotFound, ".system")},
		{"unknown, checked", notFound, acme, checked, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"unknown, a concept's", notFound, acme, concept, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"unknown, required", notFound, acme, required, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"HL7's namespace, required", notFound, tho, required, r(issue.SeverityError, issue.DiagCodeSystemNotFound, ".system")},
		{"HL7's namespace, a concept's under required", notFound, tho, conceptRequired, r(issue.SeverityError, issue.DiagCodeSystemNotFound, ".system")},
		{"HL7's namespace, no binding", notFound, tho, bare, r(issue.SeverityInformation, issue.DiagCodeSystemNotFound, ".system")},
		{"LOINC, no binding", notFound, loinc, bare, r(issue.SeverityWarning, issue.DiagBindingCannotValidate, "")},
		{"LOINC, checked", notFound, loinc, checked, r(issue.SeverityInformation, issue.DiagBindingCannotValidate, "")},
		{"LOINC, a concept's", notFound, loinc, concept, none},
		{"CPT, no binding", notFound, cpt, bare, r(issue.SeverityWarning, issue.DiagBindingCannotValidate, "")},
		{"NDC, no binding", notFound, ndc, bare, none},
		{"NDF-RT, no binding", notFound, ndfrt, bare, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"a known SID, no binding", notFound, srt, bare, none},
		{"a known SID, a concept's", notFound, srt, concept, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"a known SID, checked", notFound, srt, checked, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"bcp47, no binding", notFound, bcp47, bare, r(issue.SeverityInformation, issue.DiagBindingCannotValidate, "")},
		{"bcp47, checked", notFound, bcp47, checked, r(issue.SeverityWarning, issue.DiagBindingCannotValidate, "")},
		{"bcp47, a concept's", notFound, bcp47, concept, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"not-present, no binding", noCodes, acme, bare, r(issue.SeverityInformation, issue.DiagCodeSystemNoCodes, "")},
		{"not-present, checked", noCodes, acme, checked, r(issue.SeverityWarning, issue.DiagCodeSystemNoCodes, "")},
		{"not-present, a concept's", noCodes, acme, concept, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"example, no binding", exampleCodes, acme, bare, r(issue.SeverityWarning, issue.DiagCodeSystemExampleContent, "")},
		{"example, a concept's", exampleCodes, acme, concept, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
		{"fragment, anywhere", fragmentCodes, acme, conceptRequired, r(issue.SeverityWarning, issue.DiagCodeNotInPartialCodeSystem, ".code")},
		// Not usable, not unknown: no error under a required binding, in HL7's namespaces too.
		{"not-present, required, HL7's namespace", noCodes, tho, required, r(issue.SeverityWarning, issue.DiagCodeSystemNoCodes, "")},
		{"example, required, HL7's namespace", exampleCodes, tho, required, r(issue.SeverityWarning, issue.DiagCodeSystemExampleContent, "")},
		{"UCUM, no binding", notFound, ucum, bare, r(issue.SeverityWarning, issue.DiagBindingCannotValidate, "")},
		{"UCUM, checked", notFound, ucum, checked, r(issue.SeverityWarning, issue.DiagBindingCannotValidate, "")},
		{"UCUM, a concept's", notFound, ucum, concept, r(issue.SeverityWarning, issue.DiagCodeSystemNotFound, ".system")},
	} {
		if got := reportOne(tt.why, tt.system, tt.c); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// A CodeableConcept's coding a binding on the coding itself checks is reported on both of HL7's
// paths: as a concept's, and as a Coding that binding checks.
func TestReportUncheckedCodingBinding(t *testing.T) {
	c := codingContext{inConcept: true, checked: true, codingBound: true, codingChecked: true}
	got := reportUnchecked(notFound, "http://loinc.org", c, systemFacts{})
	want := []uncheckedReport{{severity: issue.SeverityInformation, id: issue.DiagBindingCannotValidate, report: true}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("LOINC: %+v, want %+v", got, want)
	}
	got = reportUnchecked(notFound, "urn:ietf:bcp:47", c, systemFacts{})
	if len(got) != 2 || got[0].id != issue.DiagCodeSystemNotFound || got[1].id != issue.DiagBindingCannotValidate || got[1].severity != issue.SeverityWarning {
		t.Errorf("bcp47: %+v, want the concept's unknown and the Coding's warning", got)
	}
	// An example binding on the coding: the Coding is reported as one no binding checks.
	got = reportUnchecked(notFound, "http://loinc.org", codingContext{inConcept: true, codingBound: true}, systemFacts{})
	if len(got) != 1 || got[0].severity != issue.SeverityWarning || got[0].id != issue.DiagBindingCannotValidate {
		t.Errorf("LOINC under an example binding on the coding: %+v, want the Coding's warning", got)
	}
	// A required binding on the coding escalates the Coding's report, not the concept's: a
	// not-present CodeSystem in HL7's namespace, two warnings.
	got = reportUnchecked(noCodes, "http://terminology.hl7.org/CodeSystem/acme-np", codingContext{inConcept: true, checked: true, codingBound: true, codingChecked: true, codingRequired: true}, systemFacts{})
	if len(got) != 2 || got[0].severity != issue.SeverityWarning || got[1].severity != issue.SeverityWarning {
		t.Errorf("not-present under a required binding on the coding: %+v, want two warnings", got)
	}
	// The concept's binding alone, not the coding's: the concept's report only.
	if got := reportUnchecked(notFound, "http://loinc.org", codingContext{inConcept: true, checked: true}, systemFacts{}); len(got) != 0 {
		t.Errorf("LOINC under the concept's binding: %+v, want nothing", got)
	}
}

// Each issue is merged once, at the highest severity it was reported with.
func TestOnceKeepsTheHighestSeverity(t *testing.T) {
	from, into := issue.NewResult(), issue.NewResult()
	from.AddWithID(issue.SeverityInformation, issue.DiagCodeSystemNotFound, map[string]any{"system": "s", "systemCode": "c"}, "X.system")
	from.AddWithID(issue.SeverityWarning, issue.DiagCodeSystemNotFound, map[string]any{"system": "s", "systemCode": "c"}, "X.system")
	from.AddWithID(issue.SeverityInformation, issue.DiagCodeSystemNotFound, map[string]any{"system": "s", "systemCode": "c"}, "X.system")
	(&once{seen: map[string]int{}}).merge(from, into)
	if len(into.Issues) != 1 || into.Issues[0].Severity != issue.SeverityWarning {
		t.Errorf("merged %+v, want one warning", into.Issues)
	}
}

// A Coding no binding checks whose system no CodeSystem has is checked as a URI first, as HL7 6.10.2
// checks it: an error for each problem, and the unknown system besides only where HL7 reports it.
func TestReportUncheckedURIs(t *testing.T) {
	bare := codingContext{}
	for _, tt := range []struct {
		system  string
		f       systemFacts
		errors  int
		unknown bool
	}{
		{"http://acme.local/cs", systemFacts{}, 0, true},
		{"http://hl7.org/fhir/acme-unknown", systemFacts{}, 1, false},
		{"https://www.hl7.org/fhir/acme-unknown", systemFacts{}, 1, false},
		{"http://hl7.org/fhirx/acme", systemFacts{}, 1, false},
		{"http://hl7.org/fhir/sid/us-npi", systemFacts{}, 1, false}, // an identifier system, no code system
		{"http://hl7.org/fhir/test/acme", systemFacts{}, 0, false},
		{"http://hl7.org/fhir/acme.html", systemFacts{}, 1, false},
		{"http://build.fhir.org/ig/acme/cs", systemFacts{}, 1, false},
		{"http://snomed.info/sct/900000000000207008", systemFacts{}, 1, false},
		{"https://loinc.org", systemFacts{}, 1, false},
		{"https://acme.local/cs", systemFacts{httpKnown: true}, 1, false},
		{"https://acme.local/cs", systemFacts{}, 0, true},
		{"http://acme.local/vs", systemFacts{valueSet: true}, 1, false},
		{"http://hl7.org/fhir/ValueSet/administrative-gender", systemFacts{valueSet: true}, 1, false},
	} {
		got := reportUnchecked(notFound, tt.system, bare, tt.f)
		errors, unknown := 0, false
		for _, r := range got {
			switch r.id {
			case issue.DiagCodeSystemURIInvalid:
				errors++
			case issue.DiagCodeSystemNotFound:
				unknown = true
			}
		}
		if errors != tt.errors || unknown != tt.unknown {
			t.Errorf("%s %+v: %+v", tt.system, tt.f, got)
		}
	}
	// In a CodeableConcept's coding the system is unknown, not checked as a URI.
	if got := reportUnchecked(notFound, "http://hl7.org/fhir/acme-unknown", codingContext{inConcept: true}, systemFacts{}); len(got) != 1 || got[0].id != issue.DiagCodeSystemNotFound {
		t.Errorf("a concept's: %+v", got)
	}
}
