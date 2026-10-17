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
	report   bool
}

// reportUnchecked returns how a code that cannot be checked, why, of system, is reported in c, as
// the HL7 validator 6.10.2 reports it (with -tx n/a): a CodeableConcept's coding, or a Coding a
// binding checks, goes to its terminology layer, where a code system it cannot use is unknown, a
// warning (an error under a required binding for a system in HL7's namespaces); a Coding no binding
// checks gets a hint, information, but for the systems it treats apart. A CodeableConcept's coding
// a binding on the coding itself binds is reported, besides, as a Coding that binding checks, or
// none does (an example binding).
func reportUnchecked(why unchecked, system string, c codingContext) []uncheckedReport {
	var out []uncheckedReport
	if r := reportOne(why, system, c); r.report {
		out = append(out, r)
	}
	if c.inConcept && c.codingBound {
		if r := reportOne(why, system, codingContext{checked: c.codingChecked, required: c.codingRequired}); r.report {
			out = append(out, r)
		}
	}
	return out
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
		return uncheckedReport{severity: issue.SeverityWarning, id: issue.DiagCodeNotInPartialCodeSystem, report: true}
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
