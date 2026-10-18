// Package binding validates FHIR terminology bindings.
package binding

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/internal/reslang"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/terminology"
	"github.com/gofhir/validator/v2/pkg/walker"
)

// Binding strength constants.
const (
	strengthRequired   = "required"
	strengthExtensible = "extensible"
	strengthExample    = "example"
)

// The datatypes that carry codes, by name.
const (
	codingType          = "Coding"
	codeableConceptType = "CodeableConcept"
	quantityType        = "Quantity"
	contentNotPresent   = "not-present"
	contentExample      = "example"
	ucumSystem          = "http://unitsofmeasure.org"
)

// Validator validates terminology bindings.
type Validator struct {
	sdRegistry   *registry.Registry
	termRegistry *terminology.Registry
	walker       *walker.Walker

	// unresolvedPolicy decides what happens when no terminology source can decide
	// a binding. Zero value is UnresolvedWarn.
	unresolvedPolicy terminology.UnresolvedPolicy

	// displayLanguage is the BCP-47 tag preferred when checking Coding.display.
	// Empty means no preference.
	displayLanguage string

	// definitions names the definition a value declares for itself (an extension's url).
	definitions DefinitionSource

	// bindable are the types a binding applies to (bindableTypes), read once.
	bindableOnce sync.Once
	bindable     map[string]bool
}

// New creates a new binding Validator.
func New(sdRegistry *registry.Registry, termRegistry *terminology.Registry) *Validator {
	return &Validator{
		sdRegistry:   sdRegistry,
		termRegistry: termRegistry,
		walker:       walker.New(sdRegistry),
	}
}

// SetUnresolvedPolicy decides how bindings that no terminology source could
// resolve are reported. Defaults to terminology.UnresolvedWarn.
func (v *Validator) SetUnresolvedPolicy(p terminology.UnresolvedPolicy) {
	v.unresolvedPolicy = p
}

// SetDisplayLanguage sets the BCP-47 language preferred when checking
// Coding.display. Empty means no preference.
func (v *Validator) SetDisplayLanguage(lang string) {
	v.displayLanguage = lang
}

// detailSuffix renders a backend diagnostic for appending to a message. The
// terminology port lets a backend explain its verdict — a retired code, an edition
// mismatch — and dropping that would discard the most useful part of the answer.
// Formatted here rather than in the template so an absent message leaves no trace.
func detailSuffix(message string) string {
	if message == "" {
		return ""
	}
	return ": " + message
}

// reportUnresolved records a binding that could not be checked, at the severity
// the configured policy asks for. Only required and extensible bindings reach
// here, so a weaker binding never produces noise.
func (v *Validator) reportUnresolved(code, valueSet, strength, detail, fhirPath string, result *issue.Result) {
	if strength != strengthRequired && strength != strengthExtensible {
		return
	}
	args := map[string]any{"code": code, "valueSet": valueSet, "detail": detailSuffix(detail)}
	if v.unresolvedPolicy == terminology.UnresolvedError {
		result.AddErrorWithID(issue.DiagBindingUnresolved, args, fhirPath)
		return
	}
	result.AddInfoWithID(issue.DiagBindingUnresolved, args, fhirPath)
}

// Validate validates bindings for a resource.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(context.Background(), resource, sd, result)
}

// ValidateData validates bindings for a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(ctx context.Context, resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType := sd.RootName(resource)
	root := sd.Tree().Root()
	if resourceType == "" || root == nil {
		return
	}

	// The root resource, in its language, over the definitions (walk).
	checked := issue.GetPooledResult()
	defer issue.ReleaseResult(checked)
	ctx = reslang.With(ctx, resource)
	v.walk(ctx, sd, root, "", resource, nil, resourceType, checked)
	languages := reslang.NewWalk(ctx, resourceType)

	// Every resource it holds, at any depth (contained resources, Bundle entries, the resources
	// other elements hold), against each definition it is checked against (its type, the profiles
	// it declares), in its language or else its container's: the walk above stops at the elements
	// that hold them.
	v.walker.WalkAllWithProfilesContext(ctx, resource, resourceType, resourceType, func(rc *walker.ResourceContext) bool {
		if rc.FHIRPath == resourceType {
			return true
		}
		if r := rc.SD.Tree().Root(); r != nil {
			v.walk(languages.Context(ctx, rc), rc.SD, r, "", rc.Data, nil, rc.FHIRPath, checked)
		}
		return true
	})
	(&once{seen: map[string]int{}}).merge(checked, result)
}

// ValidateCodedValue checks a Coding against the CodeSystem the data itself declares:
// does this code exist in this system, and does the display match.
//
// Deliberately independent of any binding. Two different rules are at play and they
// used to be tangled: a binding asks whether a code belongs to the ValueSet an element
// requires, which depends on strength; Coding.system + Coding.code ask whether the code
// exists in the system the instance names, which depends on nothing else. A Coding
// saying "system: v3-ActCode, code: INVENTED" is wrong whether the element is bound
// required, bound example, or not bound at all.
//
// Running it only inside binding validation meant every element bound example,
// preferred or unbound — Observation.code, Condition.code, Procedure.code, most
// clinical codes in FHIR — accepted codes that do not exist in a CodeSystem we hold.
// Verified against HL7 validator_cli 6.9.12, which reports these as errors regardless
// of binding strength. A code its code system cannot check is reported as the binding phase's
// walk reports it, in the context elemDef gives it.
//
// Deprecated: the binding phase checks every Coding in its walk; this remains for
// [extension.BindingValidator].
func (v *Validator) ValidateCodedValue(ctx context.Context, value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
	if elemDef == nil || len(elemDef.Type) == 0 {
		return
	}

	switch elemDef.Type[0].Code {
	case codingType:
		v.validateCodingValue(ctx, value, func() codingContext { return v.codingContextOf(ctx, elemDef, nil) }, fhirPath, result)
	case codeableConceptType:
		v.validateCodeableConceptValue(ctx, value, func() codingContext { return v.codingContextOf(ctx, nil, elemDef) }, fhirPath, result)
	}
}

// validateCodingValue handles a Coding, or an array of them.
func (v *Validator) validateCodingValue(ctx context.Context, value any, c func() codingContext, fhirPath string, result *issue.Result) {
	switch val := value.(type) {
	case map[string]any:
		v.checkCodingAgainstCodeSystem(ctx, val, c, fhirPath, result)
	case []any:
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				v.checkCodingAgainstCodeSystem(ctx, m, c, fmt.Sprintf("%s[%d]", fhirPath, i), result)
			}
		}
	}
}

// validateCodeableConceptValue handles a CodeableConcept, or an array of them, checking
// each of its codings.
func (v *Validator) validateCodeableConceptValue(ctx context.Context, value any, c func() codingContext, fhirPath string, result *issue.Result) {
	switch val := value.(type) {
	case map[string]any:
		v.checkConceptCodings(ctx, val, c, fhirPath, result)
	case []any:
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				v.checkConceptCodings(ctx, m, c, fmt.Sprintf("%s[%d]", fhirPath, i), result)
			}
		}
	}
}

// checkConceptCodings visits each coding of one CodeableConcept.
func (v *Validator) checkConceptCodings(ctx context.Context, cc map[string]any, cctx func() codingContext, fhirPath string, result *issue.Result) {
	codings, ok := cc["coding"].([]any)
	if !ok {
		return
	}
	for i, c := range codings {
		if m, ok := c.(map[string]any); ok {
			v.checkCodingAgainstCodeSystem(ctx, m, cctx, fmt.Sprintf("%s.coding[%d]", fhirPath, i), result)
		}
	}
}

// checkCodingAgainstCodeSystem is where the actual check happens: the code must exist in
// the system the Coding names, and the display must match the concept's.
//
// The code diagnostic is reported on the .code child rather than the element, matching
// where the reference puts it: the defect is in that value, not in the element as a
// whole, and a CodeableConcept with several codings needs to say which one is wrong.
func (v *Validator) checkCodingAgainstCodeSystem(ctx context.Context, coding map[string]any, cctx func() codingContext, fhirPath string, result *issue.Result) {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)

	// A Coding that names no system cannot be placed in any vocabulary: a warning, as the
	// concept may still be conveyed by CodeableConcept.text. One that names a system and no code
	// is allowed (Coding.code 0..1): only a CodeableConcept's is reported (checkNoCode), as HL7
	// 6.10.2 reports it; a required binding asks for a code (validateCodingBinding).
	switch {
	case system == "":
		result.AddWarningWithID(issue.DiagCodingNoSystem, nil, fhirPath)
		return
	case code == "":
		v.checkNoCode(system, cctx(), fhirPath, result)
		return
	}

	systemVersion, _ := coding["version"].(string)
	providedDisplay, _ := coding["display"].(string)

	res, err := v.termRegistry.ResolveCodeInCodeSystem(ctx, system, code,
		terminology.LookupOptions{SystemVersion: systemVersion})
	if err != nil {
		return
	}
	// A CodeSystem with example content only cannot tell a code is right, though it lists it: it
	// is not usable, as HL7 6.10.2 treats it.
	if res.Resolution == terminology.Valid {
		if cs := v.codeSystemOf(system, systemVersion); cs != nil && cs.Content == contentExample {
			res = terminology.CodeResult{Resolution: terminology.Unresolved, Partial: &terminology.PartialCodeSystem{Content: cs.Content, Version: cs.Version}}
		}
	}

	switch res.Resolution {
	case terminology.Unresolved:
		// A code its code system cannot check, reported as the HL7 validator reports it in this
		// context (reportUnchecked).
		why := notFound
		params := map[string]any{"system": system, "systemCode": code, "code": code, "codeSystem": system}
		if p := res.Partial; p != nil {
			if p.Version != "" {
				params["codeSystem"] = system + "|" + p.Version
			}
			params["content"] = p.Content
			switch p.Content {
			case contentNotPresent:
				why = noCodes
			case contentExample:
				why = exampleCodes
			default:
				why = fragmentCodes
			}
		}
		// The context is worked out only here: it resolves the value sets that bind the Coding.
		facts := systemFacts{
			valueSet:     v.termRegistry.GetValueSet(system) != nil,
			structure:    v.sdRegistry.GetByURL(system) != nil,
			namingSystem: v.termRegistry.NamesCodeSystem(system),
		}
		if rest, ok := strings.CutPrefix(system, "https:"); ok {
			facts.httpKnown = v.termRegistry.GetCodeSystem("http:"+rest) != nil
		}
		for _, r := range reportUnchecked(why, system, cctx(), facts) {
			params["reason"] = r.reason
			result.AddWithID(r.severity, r.id, params, fhirPath+r.at)
		}
		return
	case terminology.Invalid:
		result.AddErrorWithID(
			issue.DiagCodeNotInCodeSystem,
			map[string]any{"code": code, "system": system},
			fhirPath+".code",
		)
		return
	case terminology.Valid:
	}

	if providedDisplay != "" {
		v.validateDisplayMismatch(ctx, system, systemVersion, code, providedDisplay, fhirPath, result)
	}
}

// checkNoCode checks a Coding with a system and no code, in context c. Only a CodeableConcept's is
// reported: a code system it cannot use as with a code (reportOne), one it can use that no code is
// provided, a warning, as HL7 reports them (UNKNOWN_CODESYSTEM, NO_CODE_PROVIDED); nothing of a
// system HL7 checks on a server only.
func (v *Validator) checkNoCode(system string, c codingContext, fhirPath string, result *issue.Result) {
	cs := v.termRegistry.GetCodeSystem(system)
	switch class := hl7SystemClasses[system]; {
	case !c.inConcept || class.serverChecked():
	case cs != nil && class == 0 && cs.Content != contentNotPresent && cs.Content != contentExample:
		result.AddWarningWithID(issue.DiagCodingNoCodeInConcept, map[string]any{"system": system}, fhirPath+".code")
	default:
		why := notFound
		if cs != nil && cs.Content == contentNotPresent {
			why = noCodes
		} else if cs != nil && cs.Content == contentExample {
			why = exampleCodes
		}
		if r := reportOne(why, system, c); r.report {
			result.AddWithID(r.severity, r.id, map[string]any{"system": system, "systemCode": ""}, fhirPath+r.at)
		}
	}
}

// ValidateValueBinding validates a value against an element binding.
//
// Exported for callers that hold the binding without an ElementDefinition — chiefly
// extension validation, where the binding comes from the extension's own
// StructureDefinition. Routing both traversals through here is what keeps them from
// drifting: display checks, CodeSystem membership, CodeableConcept aggregation, the
// unresolved policy and the extensible severity table apply the same way to an
// extension value as to any other element.
//
// TypeName is the value's type, "" when the caller does not know it: an object is then read as a
// Coding or a CodeableConcept by the elements it holds, and one that holds none of them is neither.
func (v *Validator) ValidateValueBinding(ctx context.Context, value any, typeName string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	if binding == nil {
		return
	}

	// Only validate required and extensible bindings
	// preferred and example bindings are informational only - no validation
	if binding.Strength != strengthRequired && binding.Strength != strengthExtensible {
		return
	}

	// Handle different value types
	switch val := value.(type) {
	case string:
		v.validateCodeBinding(ctx, val, "", binding, fhirPath, result)

	case map[string]any:
		v.validateMapBinding(ctx, val, typeName, binding, fhirPath, result)

	case []any:
		for i, item := range val {
			itemPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			v.ValidateValueBinding(ctx, item, typeName, binding, itemPath, result)
		}
	}
}

// validateMapBinding validates a map value (Coding or CodeableConcept, typeName when known) against a
// binding.
func (v *Validator) validateMapBinding(ctx context.Context, val map[string]any, typeName string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	// Check if it's a CodeableConcept with coding array
	if coding, ok := val["coding"]; ok {
		v.validateCodeableConceptWithCoding(ctx, coding, binding, fhirPath, result)
		return
	}

	// A CodeableConcept with no coding: only text, or nothing that is a concept (an id, extensions,
	// an empty object), which provides no code either, as HL7 reports it.
	if val["text"] != nil || typeName == codeableConceptType {
		v.reportNoCode(ctx, binding, fhirPath, result)
		return
	}

	// Looks like a Coding with system
	if _, ok := val["system"]; ok {
		v.validateCodingBinding(ctx, val, typeName, binding, fhirPath, result)
		return
	}

	// Coding with just code
	if code, ok := val["code"]; ok {
		if codeStr, ok := code.(string); ok {
			v.validateCodeBinding(ctx, codeStr, "", binding, fhirPath, result)
		}
	}
}

// validateCodeableConceptWithCoding validates a CodeableConcept that has a coding array.
func (v *Validator) validateCodeableConceptWithCoding(ctx context.Context, coding any, binding *registry.Binding, fhirPath string, result *issue.Result) {
	codings, isList := coding.([]any)
	if !isList || len(codings) == 0 {
		v.reportNoCode(ctx, binding, fhirPath, result) // no coding: text only, or nothing
		return
	}

	anyValidInVS, undecided, codeLabels := v.validateCodings(ctx, codings, binding)

	// Codings without a code carry none. A required binding needs one; an extensible one is told
	// there is none when no coding names a system either, else that none of them is in the value set,
	// named by their systems (HL7's Terminology_TX_NoValid_2_CC).
	if len(codeLabels) == 0 && (binding.Strength == strengthRequired || !anySystem(codings)) {
		v.reportNoCode(ctx, binding, fhirPath, result)
		return
	}
	if len(codeLabels) == 0 {
		// Codings of systems with no code: a concept with no code is none of the value set's; of a
		// value set that does not resolve, only that it is not found (reportNoCode).
		if !v.termRegistry.ResolvesValueSet(ctx, binding.ValueSet) {
			v.reportNoCode(ctx, binding, fhirPath, result)
			return
		}
		codeLabels = systemLabels(codings)
	}

	// No coding is a member, and one could not be decided: whether the concept is from the value set
	// is not known, which is not the same as "none of the codings are members" (HL7: "Could not
	// confirm that the codes provided are from the value set").
	if !anyValidInVS && len(undecided) > 0 {
		v.reportUnresolved(strings.Join(undecided, ", "), binding.ValueSet, binding.Strength, "", fhirPath, result)
		return
	}

	// A required binding is met by one coding of the concept (terminologies.html, "required": the
	// concept SHALL be from the value set): none is, reported once at the concept, as HL7 reports it.
	if !anyValidInVS && binding.Strength == strengthRequired && len(codeLabels) > 0 {
		result.AddErrorWithID(issue.DiagBindingRequiredNoCoding, map[string]any{
			"valueSet": binding.ValueSet,
			"codes":    strings.Join(codeLabels, ", "),
		}, fhirPath)
		return
	}

	// CC-level extensible warning: none of the codings matched the ValueSet.
	if !anyValidInVS && binding.Strength == strengthExtensible && len(codeLabels) > 0 {
		result.AddWarningWithID(
			issue.DiagBindingExtensibleNoCoding,
			map[string]any{
				"valueSet": binding.ValueSet,
				"codes":    strings.Join(codeLabels, ", "),
			},
			fhirPath,
		)
	}
}

// systemLabels labels the codings that name a system, and carry no code, by their system.
func systemLabels(codings []any) []string {
	var labels []string
	for _, c := range codings {
		if m, ok := c.(map[string]any); ok {
			if s, _ := m["system"].(string); s != "" {
				labels = append(labels, s+"#")
			}
		}
	}
	return labels
}

// reportNoCode reports a CodeableConcept that carries no code (only text, or nothing): an error
// under a required binding, as a concept from a required value set is a code, and text alone is not
// one (terminologies.html, "required": "the concept SHALL be from the specified value set"); a
// warning under an extensible one, as the HL7 validator reports it. Against a value set that does not
// resolve, only that it is not found, as the HL7 validator does: an unknown one says nothing about
// what the code should have been.
func (v *Validator) reportNoCode(ctx context.Context, binding *registry.Binding, fhirPath string, result *issue.Result) {
	args := map[string]any{"valueSet": binding.ValueSet}
	switch {
	case v.termRegistry == nil:
	case !v.termRegistry.ResolvesValueSet(ctx, binding.ValueSet):
		result.AddWarningWithID(issue.DiagBindingValueSetNotFound, args, fhirPath)
	case binding.Strength == strengthRequired:
		result.AddErrorWithID(issue.DiagBindingRequiredNoCode, args, fhirPath)
	case binding.Strength == strengthExtensible:
		result.AddWarningWithID(issue.DiagBindingTextOnlyWarning, args, fhirPath)
	}
}

// anySystem reports whether one of codings names a system.
func anySystem(codings []any) bool {
	for _, c := range codings {
		if m, ok := c.(map[string]any); ok {
			if s, _ := m["system"].(string); s != "" {
				return true
			}
		}
	}
	return false
}

// codingInValueSet decides whether a Coding of a CodeableConcept is a member of binding's value
// set. Nothing is reported per coding: the caller reports at the concept, which one member meets.
// Unresolved must not be reported as "not a member".
func (v *Validator) codingInValueSet(ctx context.Context, coding map[string]any, binding *registry.Binding) terminology.Resolution {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)

	if code == "" {
		return terminology.Unresolved
	}

	// The CodeSystem check runs separately, in the walk (checkCodingAgainstCodeSystem): a code
	// missing from its own system is reported there, and the reference reports value set membership
	// alongside it rather than instead of it.

	// Check ValueSet.
	res, err := v.termRegistry.ResolveCodeInValueSet(ctx, system, code, binding.ValueSet,
		terminology.LookupOptions{})
	if err != nil || res.Resolution == terminology.Unresolved {
		// Undecidable. Reported at CC level rather than per coding, so a
		// CodeableConcept with several codings yields one issue.
		return terminology.Unresolved
	}

	if res.Resolution == terminology.Invalid {
		// Reported at CC level, for both strengths: another coding may be a member.
		return terminology.Invalid
	}

	return terminology.Valid
}

// validateCodings decides each Coding of a CodeableConcept against binding and returns what the set
// as a whole established, reporting nothing: whether any is a member, the labels of the codings with
// a code that could not be decided (undecided), and the labels of all, for an aggregate diagnostic.
func (v *Validator) validateCodings(ctx context.Context, codings []any, binding *registry.Binding) (anyValidInVS bool, undecided, codeLabels []string) {
	for _, c := range codings {
		codingMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		sys, _ := codingMap["system"].(string)
		cd, _ := codingMap["code"].(string)
		if cd == "" {
			continue // decides nothing, and names nothing
		}
		label := cd
		if sys != "" {
			label = sys + "#" + cd
		}
		codeLabels = append(codeLabels, label)
		switch v.codingInValueSet(ctx, codingMap, binding) {
		case terminology.Valid:
			anyValidInVS = true
		case terminology.Unresolved:
			undecided = append(undecided, label) // nothing was established either way
		}
	}
	return anyValidInVS, undecided, codeLabels
}

// validateCodeBinding validates a code against a ValueSet.
func (v *Validator) validateCodeBinding(ctx context.Context, code, system string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	if code == "" {
		return // Empty code is handled by cardinality validation
	}

	res, err := v.termRegistry.ResolveCodeInValueSet(ctx, system, code, binding.ValueSet,
		terminology.LookupOptions{})
	if err != nil || res.Resolution == terminology.Unresolved {
		// Undecidable: neither the local copies nor any configured backend could
		// answer. Whether that is acceptable is a policy decision, not something
		// this function gets to assume.
		v.reportUnresolved(code, binding.ValueSet, binding.Strength, res.Message, fhirPath, result)
		return
	}

	if res.Resolution == terminology.Invalid {
		v.reportBindingViolation(system, code, res, binding, fhirPath, result)
	}
}

// validateCodingBinding validates a Coding, or a Quantity's coded unit (typeName), against a ValueSet
// and its CodeSystem.
func (v *Validator) validateCodingBinding(ctx context.Context, coding map[string]any, typeName string, binding *registry.Binding, fhirPath string, result *issue.Result) {
	system, _ := coding["system"].(string)
	code, _ := coding["code"].(string)

	if code == "" {
		// A Coding with a system and no code provides no code: under a required binding the
		// concept must come from the value set, as for a CodeableConcept with none (decided by the
		// specification; HL7 6.10.2 reports nothing for a Coding). A Quantity with no code is a
		// value with no coded unit, which its binding does not ask for (HL7 reports nothing).
		if binding.Strength == strengthRequired && typeName != quantityType {
			v.reportNoCode(ctx, binding, fhirPath, result)
		}
		return
	}

	// Validate against the ValueSet binding
	res, err := v.termRegistry.ResolveCodeInValueSet(ctx, system, code, binding.ValueSet,
		terminology.LookupOptions{})
	if err != nil || res.Resolution == terminology.Unresolved {
		codeDisplay := code
		if system != "" {
			codeDisplay = fmt.Sprintf("%s#%s", system, code)
		}
		v.reportUnresolved(codeDisplay, binding.ValueSet, binding.Strength, res.Message, fhirPath, result)
		return
	}

	if res.Resolution == terminology.Invalid {
		v.reportBindingViolation(system, code, res, binding, fhirPath, result)
		return
	}
}

// validateDisplayMismatch reports a Coding.display that does not match the
// concept's display.
//
// An error, not a warning, deliberately: per HL7's validator guidance a wrong
// display is often a sign the wrong code was chosen — right display, wrong code —
// and the specification requires the display to be valid per the code system.
//
// The comparison is skipped whenever the display cannot be established in the
// language being checked. Otherwise a submitted Spanish display would be compared
// against the English one and rejected, which is a validator bug rather than a
// data problem. Concretely: when a display language is requested and the backend
// could not honor it, there is nothing to compare against.
func (v *Validator) validateDisplayMismatch(ctx context.Context, system, systemVersion, code, providedDisplay, fhirPath string, result *issue.Result) {
	res, err := v.termRegistry.ResolveCodeInCodeSystem(ctx, system, code, terminology.LookupOptions{
		DisplayLanguage: v.displayLanguage,
		SystemVersion:   systemVersion,
	})
	if err != nil || res.Display == "" {
		return
	}
	if v.displayLanguage != "" && !res.DisplayLanguageHonored {
		// The backend has no display in the requested language, so a mismatch here
		// would say nothing about the submitted one.
		return
	}
	if validDisplay(providedDisplay, res, reslang.Of(ctx)) {
		return
	}

	result.AddErrorWithID(
		issue.DiagBindingDisplayMismatch,
		map[string]any{
			"code":     code,
			"provided": providedDisplay,
			"expected": res.Display,
		},
		fhirPath+".display",
	)
}

// validDisplay reports whether display names the concept res describes in the language asked
// for: the resource's (Resource.language), or else the CodeSystem's. Valid are the designations
// with no language (a synonym), those in that language or in one it is a variant of ("de" for
// "de-CH", languages.html), and the concept's display when the CodeSystem's language is that or
// it does not say. When the concept has no name in that language, its display is valid too: there
// is nothing else to write it with. The HL7 validator checks a display the same way, with two
// differences: with no Resource.language it asks for its JVM's locale (6.10.2), where the
// CodeSystem's language does not depend on where the validator runs, as the specification sets no
// default; and it compares language tags with their case, which BCP 47 says does not matter
// (rfc5646 2.1.1). A display's case is ignored, as HL7 ignores it.
func validDisplay(display string, res terminology.CodeResult, asked string) bool {
	if asked == "" {
		asked = res.DisplayLanguage
	}
	inAsked := res.DisplayLanguage == "" || languageMatches(res.DisplayLanguage, asked)
	if inAsked && strings.EqualFold(display, res.Display) {
		return true
	}
	for _, d := range res.Designations {
		if d.Language == "" || languageMatches(d.Language, asked) {
			inAsked = inAsked || d.Language != ""
			if strings.EqualFold(display, d.Value) {
				return true
			}
		}
	}
	return !inAsked && strings.EqualFold(display, res.Display)
}

// languageMatches reports whether a name in language tag serves for asked: the same language, or
// one asked is a variant of ("de" for "de-CH"; BCP-47 tags, compared ignoring case).
func languageMatches(tag, asked string) bool {
	if asked == "" {
		return false
	}
	return strings.EqualFold(tag, asked) || (len(asked) > len(tag) && strings.EqualFold(asked[:len(tag)], tag) && asked[len(tag)] == '-')
}

// reportBindingViolation reports a binding violation based on binding strength.
//
// Severity is driven by strength alone; membership only selects which extensible
// diagnostic to emit. Escalating on membership would diverge from the HL7
// validator and HAPI, which both treat an extensible miss as a warning regardless
// of whether the ValueSet declares the code's system, and the spec conditions
// extensible conformance on whether the ValueSet covers the concept — a judgment
// — not on whether the system is declared.
func (v *Validator) reportBindingViolation(system, code string, res terminology.CodeResult, binding *registry.Binding, fhirPath string, result *issue.Result) {
	codeDisplay := code
	if system != "" {
		codeDisplay = fmt.Sprintf("%s#%s", system, code)
	}
	args := map[string]any{
		"code":     codeDisplay,
		"valueSet": binding.ValueSet,
		"detail":   detailSuffix(res.Message),
	}
	membership := res.SystemInValueSet

	switch binding.Strength {
	case strengthRequired:
		result.AddErrorWithID(issue.DiagBindingRequired, args, fhirPath)
	case strengthExtensible:
		v.reportExtensibleViolation(system, membership, args, fhirPath, result)
	}
}

// notUsable reports whether system is one whose codes cannot be told under a binding: one HL7's
// terminology has with no codes, UCUM (whose codes HL7 says it cannot validate without a server), or
// a CodeSystem loaded with content not-present or example.
func (v *Validator) notUsable(system string) bool {
	if class := hl7SystemClasses[system]; class == classNotUsable || class == classUCUM {
		return true
	}
	cs := v.termRegistry.GetCodeSystem(system)
	return cs != nil && (cs.Content == contentNotPresent || cs.Content == contentExample)
}

// codeSystemOf returns the CodeSystem a Coding names, in the version it names, or nil.
func (v *Validator) codeSystemOf(system, version string) *terminology.CodeSystem {
	if version != "" {
		return v.termRegistry.GetCodeSystemVersion(system, version)
	}
	return v.termRegistry.GetCodeSystem(system)
}

// reportExtensibleViolation picks the extensible diagnostic for a non-member code.
func (v *Validator) reportExtensibleViolation(system string, membership terminology.Membership, args map[string]any, fhirPath string, result *issue.Result) {
	// A primitive code element carries no system — it is implied by the ValueSet
	// — so there is no other system to be extending with.
	if system == "" {
		result.AddWarningWithID(issue.DiagBindingExtensible, args, fhirPath)
		return
	}

	switch membership {
	case terminology.MembershipExcluded:
		// A code from a system the ValueSet does not declare: an extensible binding permits it
		// only where the value set has no suitable code, which the validator cannot judge, so a
		// warning, as HL7 6.10.2 reports it (Terminology_TX_NoValid_13); information for a system
		// HL7 checks only on a terminology server (Terminology_TX_Binding_NoServer).
		switch {
		case hl7SystemClasses[system].serverChecked():
			result.AddInfoWithID(issue.DiagBindingExtensibleOther, args, fhirPath)
			return
		case v.notUsable(system):
			// HL7 says only that the code cannot be validated (reportUnchecked reports it).
			return
		}
		result.AddWarningWithID(issue.DiagBindingExtensibleOther, args, fhirPath)
	case terminology.MembershipUnknown:
		result.AddWarningWithID(issue.DiagBindingExtensibleUnknown, args, fhirPath)
	case terminology.MembershipIncluded:
		result.AddWarningWithID(issue.DiagBindingExtensible, args, fhirPath)
	}
}
