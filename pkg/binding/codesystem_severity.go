package binding

import (
	"context"
	"strings"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// A code whose code system cannot be checked here (not found, found without its codes, or one only
// a terminology server can check) is reported with the severity the HL7 validator gives it. The
// specification sets none (terminologies.html, validation.html) and allows local systems where tags
// are concerned (resource.html#tags), so HL7's verdict decides, followed for parity: its two paths
// (InstanceValidator 6.10.2) tell a CodeableConcept's codings, and a Coding a binding checks, from a
// Coding no binding checks. The Coding/CodeableConcept difference is HL7's, not the
// specification's.

// codingContext is what decides the severity for one Coding: whether it is a CodeableConcept's,
// whether a binding checks it (its strength is not example and its value set resolves), and
// whether that binding is required; for a CodeableConcept's coding, whether a binding a profile
// sets on the coding itself binds it (HL7 then checks the Coding besides the concept, as one that
// binding checks, or none does when it is an example one), and whether it checks it.
type codingContext struct {
	inConcept      bool
	checked        bool
	required       bool
	codingBound    bool
	codingChecked  bool
	codingRequired bool
	// A binding checks the Coding (or Quantity) and none checks its base element (a profile's or an
	// extension's binding on an element the base resource or type leaves unbound, or binds by
	// example): HL7 checks it besides as one no binding checks.
	baseUnchecked bool
}

// codingContextOf returns the context of a Coding governed by def, held by holder: the definition
// of the CodeableConcept it is a coding of, nil for a Coding that is no CodeableConcept's. A binding
// a profile sets on the coding itself counts too.
func (v *Validator) codingContextOf(ctx context.Context, def, holder *registry.ElementDefinition) codingContext {
	var c codingContext
	c.codingBound = holder != nil && def != nil && def.Binding != nil && def.Binding.ValueSet != ""
	for _, d := range []*registry.ElementDefinition{holder, def} {
		if d == nil {
			continue
		}
		if b := v.checkingBinding(ctx, d); b != nil {
			c.checked = true
			// A concept's report is escalated by the concept's binding only; the coding's own
			// escalates the Coding's report (codingRequired), as HL7 reports them.
			if d == holder || holder == nil {
				c.required = c.required || b.Strength == strengthRequired
			}
			if d == def && holder != nil {
				c.codingChecked, c.codingRequired = true, b.Strength == strengthRequired
			}
		}
	}
	c.inConcept = holder != nil
	if holder == nil && c.checked {
		if base := v.baseOf(def); base != nil && v.checkingBinding(ctx, base) == nil {
			c.baseUnchecked = true
		}
	}
	return c
}

// checkingBinding returns the binding that checks a value of def: def's, where its value set
// resolves, else its base element's, as the binding phase checks it; nil when its strength is
// example or neither resolves.
func (v *Validator) checkingBinding(ctx context.Context, def *registry.ElementDefinition) *registry.Binding {
	b := def.Binding
	if b != nil && b.ValueSet != "" && v.valueSetNotFound(ctx, b) {
		if base := v.baseOf(def); base != nil && otherBinding(base.Binding, b) && !v.valueSetNotFound(ctx, base.Binding) {
			b = base.Binding
		} else {
			return nil
		}
	}
	if b == nil || b.ValueSet == "" || b.Strength == strengthExample {
		return nil
	}
	return b
}

// systemClass is how the HL7 validator treats a code system it checks only on a terminology server.
type systemClass int

const (
	_ systemClass = iota // any other system
	// HL7 says it runs without terminology services for a Coding no binding checks
	// (InstanceValidator.NO_TX_SYSTEM_EXEMPT, as measured).
	classNoServer
	// UCUM: in NO_TX_SYSTEM_EXEMPT, not server side, and with no codes in HL7's terminology
	// package: unknown in a CodeableConcept's coding, that it cannot be validated (a warning) in a
	// Coding.
	classUCUM
	// HL7 says the system is not supported for a Coding no binding checks
	// (ValueSetUtilities.isServerSide).
	classUnsupported
	// HL7 reports nothing for a Coding no binding checks (a known HL7 SID, or a system its
	// terminology package declares).
	classServerSide
	// HL7 reports it unknown for a Coding no binding checks, a warning.
	classUnsupportedUnknown
	// A code system HL7 knows the SID of (SIDUtilities.CODE_SYSTEMS): nothing for a Coding no
	// binding checks, else unknown, as any other.
	classKnownSID
	// HL7's terminology package has it with no codes (content not-present).
	classNotUsable
)

// serverChecked reports whether HL7 checks the system on a terminology server only: nothing is said
// of it in a CodeableConcept's coding.
func (s systemClass) serverChecked() bool {
	return s == classNoServer || s == classUnsupported || s == classServerSide || s == classUnsupportedUnknown
}

// hl7SystemClasses are the systems the HL7 validator 6.10.2 treats apart, with how
// (InstanceValidator.NO_TX_SYSTEM_EXEMPT, ValueSetUtilities.isServerSide, SIDUtilities; measured
// with -tx n/a), as reportOne reports each.
var hl7SystemClasses = map[string]systemClass{
	"http://loinc.org":                             classNoServer,
	"http://snomed.info/sct":                       classNoServer,
	"http://unitsofmeasure.org":                    classUCUM,
	"http://www.nlm.nih.gov/research/umls/rxnorm":  classNoServer,
	"http://www.ama-assn.org/go/cpt":               classUnsupported,
	"http://hl7.org/fhir/ndfrt":                    classUnsupportedUnknown,
	"http://hl7.org/fhir/sid/icd-9-cm":             classServerSide,
	"http://hl7.org/fhir/sid/icd-10":               classServerSide,
	"http://hl7.org/fhir/sid/icd-10-cm":            classServerSide,
	"http://hl7.org/fhir/sid/ndc":                  classServerSide,
	"http://hl7.org/fhir/sid/cvx":                  classServerSide,
	"http://hl7.org/fhir/sid/icd-9":                classKnownSID,
	"http://hl7.org/fhir/sid/icd-10-am":            classKnownSID,
	"http://hl7.org/fhir/sid/icd-10-vn":            classKnownSID,
	"http://hl7.org/fhir/sid/ex-icd-10-procedures": classKnownSID,
	"http://hl7.org/fhir/sid/icpc-1":               classKnownSID,
	"http://hl7.org/fhir/sid/icpc-1-nl":            classKnownSID,
	"http://hl7.org/fhir/sid/icpc-2":               classKnownSID,
	"http://hl7.org/fhir/sid/icf-nl":               classKnownSID,
	"http://hl7.org/fhir/sid/srt":                  classKnownSID,
	"http://hl7.org/fhir/sid/dsm5":                 classKnownSID,
	"http://hl7.org/fhir/sid/ca-hc-npn":            classKnownSID,
	"http://hl7.org/fhir/sid/ca-hc-din":            classKnownSID,
	"urn:ietf:bcp:47":                              classNotUsable,
	"urn:iso:std:iso:3166":                         classNotUsable,
}

// hl7Namespaced reports whether system is in the namespaces HL7 publishes its code systems in
// (terminologies-systems.html), where one this validator does not know under a required binding is
// an error, as HL7 reports it.
func hl7Namespaced(system string) bool {
	return strings.HasPrefix(system, "http://hl7.org/fhir") || strings.HasPrefix(system, "http://terminology.hl7.org")
}

// unchecked is why a code cannot be checked against its code system.
type unchecked int

const (
	notFound      unchecked = iota // no CodeSystem with the url, and HL7 treats it as any other
	noCodes                        // a CodeSystem with content not-present
	exampleCodes                   // a CodeSystem with content example
	fragmentCodes                  // a CodeSystem with content fragment
)

// uncheckedReport is how a code that cannot be checked is reported: with what severity, as which
// diagnostic, at which child of the Coding ("" for the Coding), and whether at all.
type uncheckedReport struct {
	severity issue.Severity
	id       issue.DiagnosticID
	at       string
	reason   string // why the system URI is not valid, for DiagCodeSystemURIInvalid
	report   bool
}

// reportUnchecked returns how a code that cannot be checked, why, of system, is reported in c, as
// the HL7 validator 6.10.2 reports it (with -tx n/a): a CodeableConcept's coding, or a Coding a
// binding checks, goes to its terminology layer, where a code system it cannot use is unknown, a
// warning (an error under a required binding for a system in HL7's namespaces); a Coding no binding
// checks gets a hint, information, but for the systems it treats apart. A CodeableConcept's coding
// a binding on the coding itself binds is reported, besides, as a Coding that binding checks, or
// none does (an example binding). A Coding or Quantity a binding checks where none checks its base
// element is reported, besides, as one no binding checks.
func reportUnchecked(why unchecked, system string, c codingContext, f systemFacts) []uncheckedReport {
	out := reportPath(why, system, c, f)
	if c.inConcept && c.codingBound {
		out = append(out, reportPath(why, system, codingContext{checked: c.codingChecked, required: c.codingRequired}, f)...)
	}
	if c.baseUnchecked {
		out = append(out, reportPath(why, system, codingContext{}, f)...)
	}
	return out
}

// systemFacts are what the definitions loaded say of a system no CodeSystem has: whether a value
// set has it as its url, and whether, written with https, a CodeSystem has it with http.
type systemFacts struct {
	valueSet  bool
	httpKnown bool
	structure bool // a StructureDefinition has it as its url, which HL7 takes as a code system of its elements
	// A NamingSystem of kind codesystem has it as its preferred unique id: the code system is
	// published, its codes are not here.
	namingSystem bool
}

// reportPath is the reports of one of HL7's paths: a Coding whose system is a value set's URL is
// that error alone, which the specification states (datatypes.html#Coding: the system SHALL NOT
// contain a reference to a value set), wherever the Coding is but as a CodeableConcept's coding no
// binding of its own checks, where HL7 does not say it (at the system where a binding checks the
// Coding, as HL7 places it); a Coding no binding checks, its system unknown, is first checked as a
// URI (uriProblems).
func reportPath(why unchecked, system string, c codingContext, f systemFacts) []uncheckedReport {
	if why == notFound && f.valueSet && !c.inConcept {
		r := uncheckedReport{severity: issue.SeverityError, id: issue.DiagCodeSystemURIInvalid, reason: "it is a value set's URL, not a code system's", report: true}
		if c.checked {
			r.at = ".system"
		}
		return []uncheckedReport{r}
	}
	r := reportOne(why, system, c)
	if f.namingSystem && strings.HasPrefix(system, "http://hl7.org/fhir") && r.id == issue.DiagCodeSystemNotFound &&
		(!c.inConcept || !c.required) {
		// A code system HL7 publishes under its namespace whose codes are not here: unknown, a
		// warning, as HL7 reports it (its terminology package has a CodeSystem with no codes for
		// it), but a CodeableConcept's coding under a required binding, still an error.
		r.severity = issue.SeverityWarning
	}
	if c.inConcept || c.checked || why != notFound || hl7SystemClasses[system] != 0 {
		if r.report {
			return []uncheckedReport{r}
		}
		return nil
	}
	reasons, keepUnknown := uriProblems(system, f)
	var out []uncheckedReport
	for _, reason := range reasons {
		out = append(out, uncheckedReport{severity: issue.SeverityError, id: issue.DiagCodeSystemURIInvalid, reason: reason, report: true})
	}
	if keepUnknown && r.report {
		out = append(out, r)
	}
	return out
}

// noHTTPS are the systems HL7 reports written with https (InstanceValidator.NO_HTTPS_LIST).
var noHTTPS = map[string]bool{
	"https://loinc.org": true, "https://unitsofmeasure.org": true, "https://snomed.info/sct": true,
	"https://www.nlm.nih.gov/research/umls/rxnorm": true,
}

// uriProblems are the reasons the HL7 validator 6.10.2 gives a system URI no CodeSystem has, in a
// Coding no binding checks, an error each (InstanceValidator.checkCode); and whether the system is
// besides reported unknown. A system in HL7's namespace that HL7 does not publish is no local one,
// as the specification reserves that namespace (terminologies-systems.html).
func uriProblems(system string, f systemFacts) (reasons []string, keepUnknown bool) {
	switch {
	case strings.HasPrefix(system, "http://build.fhir.org") || strings.HasPrefix(system, "https://build.fhir.org"):
		return []string{"it is a build.fhir.org URL, not a canonical one"}, false
	case hasAnyPrefix(system, "http://hl7.org/fhir", "https://hl7.org/fhir", "http://www.hl7.org/fhir", "https://www.hl7.org/fhir"):
		switch {
		case f.structure, f.namingSystem:
			return nil, true
		case strings.HasPrefix(system, "http://hl7.org/fhir/test"):
			return nil, false
		case strings.HasSuffix(system, ".html"):
			return []string{"it is the URL of a web page, not of a code system"}, false
		}
		return []string{"it is in HL7's namespace, and no code system HL7 publishes"}, false
	}
	for _, base := range []string{"http://snomed.info/sct", "http://loinc.org", "http://unitsofmeasure.org", "http://www.nlm.nih.gov/research/umls/rxnorm"} {
		if strings.HasPrefix(system, base) && system != base {
			return []string{"it starts with " + base + " and is not it"}, false
		}
	}
	if strings.HasPrefix(system, "https:") && (f.httpKnown || noHTTPS[system]) {
		return []string{"it is a code system's URL written with https"}, false
	}
	return nil, true
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// reportOne is reportUnchecked's report for one of HL7's paths.
func reportOne(why unchecked, system string, c codingContext) uncheckedReport {
	unknown := uncheckedReport{severity: escalated(system, c), id: issue.DiagCodeSystemNotFound, at: ".system", report: true}
	server := uncheckedReport{id: issue.DiagBindingCannotValidate, report: true}
	class := hl7SystemClasses[system]
	switch {
	case why == fragmentCodes:
		return uncheckedReport{severity: issue.SeverityWarning, id: issue.DiagCodeNotInPartialCodeSystem, at: ".code", report: true}
	case class.serverChecked():
		switch {
		case c.inConcept:
			return uncheckedReport{}
		case c.checked:
			server.severity = issue.SeverityInformation
			return server
		case class == classServerSide:
			return uncheckedReport{}
		case class == classUnsupportedUnknown:
			unknown.severity = issue.SeverityWarning
			return unknown
		}
		server.severity = issue.SeverityWarning
		return server
	case c.inConcept:
		return unknown
	case class == classUCUM:
		server.severity = issue.SeverityWarning
		return server
	case class == classKnownSID:
		if !c.checked {
			return uncheckedReport{}
		}
		return unknown
	case class == classNotUsable:
		server.severity = issue.SeverityInformation
		if c.checked {
			server.severity = issue.SeverityWarning
		}
		return server
	case why == noCodes:
		// Not usable: a warning when a binding checks it, a required one too.
		r := uncheckedReport{severity: issue.SeverityInformation, id: issue.DiagCodeSystemNoCodes, report: true}
		if c.checked {
			r.severity = issue.SeverityWarning
		}
		return r
	case why == exampleCodes:
		return uncheckedReport{severity: issue.SeverityWarning, id: issue.DiagCodeSystemExampleContent, report: true}
	case c.checked:
		return unknown
	}
	unknown.severity = issue.SeverityInformation
	return unknown
}

// escalated is a warning, or an error under a required binding for a system in HL7's namespaces.
func escalated(system string, c codingContext) issue.Severity {
	if c.required && hl7Namespaced(system) {
		return issue.SeverityError
	}
	return issue.SeverityWarning
}
