// Package constraint validates FHIR constraints (invariants) using FHIRPath.
package constraint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/terminology"
)

// elementInstance represents a single instance of a FHIR element found in the resource JSON.
type elementInstance struct {
	data     json.RawMessage // JSON bytes of this element instance.
	fhirPath string          // FHIRPath location (e.g., "Patient.contact[0]").
}

// jsonNode is a parsed JSON object with its FHIRPath location, used during element extraction.
type jsonNode struct {
	data     map[string]any
	value    json.RawMessage // a primitive's JSON value, set instead of data
	fhirPath string
}

// ValidateOptions holds per-call options for constraint validation.
type ValidateOptions struct {
	// BundleData is the parsed Bundle JSON, enabling resolve() in FHIRPath.
	// When non-nil, a resolver is created that can find resources by fullUrl.
	BundleData map[string]any

	// Resource and RootResource are the resources the validated value sits in, for %resource and
	// %rootResource, as FHIRPath collections, when the value is not itself the root of its
	// variables (a datatype or extension checked against its profile, or a contained resource).
	// The caller prepares them once, so a large Bundle is not converted per check. Nil means the
	// value is the resource.
	Resource     fhirpath.Collection
	RootResource fhirpath.Collection
}

// constraintEvalOpts carries all contextual data for a single constraint evaluation.
type constraintEvalOpts struct {
	ctx             context.Context
	resourceCol     fhirpath.Collection // %resource variable.
	rootResourceCol fhirpath.Collection // %rootResource variable (for contained/Bundle).
	resolver        eval.Resolver       // For resolve() in FHIRPath.
	termService     eval.TerminologyService
	timeout         time.Duration
}

// Validator validates constraints defined in ElementDefinitions.
type Validator struct {
	registry     *registry.Registry
	termRegistry *terminology.Registry

	// Cache of compiled FHIRPath expressions.
	exprCache   map[string]*fhirpath.Expression
	exprCacheMu sync.RWMutex

	// Cache of which complex types carry constraints worth descending into, keyed by
	// type code. Choice elements declare up to ~50 types, so this question is asked
	// far more often than there are types to answer it for.
	typeConstraintCache   map[string]*registry.StructureDefinition
	typeConstraintCacheMu sync.RWMutex
}

// New creates a new constraint Validator.
// The termRegistry may be nil to disable memberOf() support (e.g., when -tx n/a is set).
func New(reg *registry.Registry, termReg *terminology.Registry) *Validator {
	return &Validator{
		registry:            reg,
		termRegistry:        termReg,
		exprCache:           make(map[string]*fhirpath.Expression),
		typeConstraintCache: make(map[string]*registry.StructureDefinition),
	}
}

// constrainedTypeSD returns the StructureDefinition for typeCode when it is a complex type
// that carries constraints of its own, and nil otherwise — meaning there is nothing to
// descend into.
//
// Everything here is read from the StructureDefinition: whether the type is a complex type
// comes from Kind, and whether it is worth visiting comes from its elements' constraints.
func (v *Validator) constrainedTypeSD(typeCode string) *registry.StructureDefinition {
	v.typeConstraintCacheMu.RLock()
	sd, ok := v.typeConstraintCache[typeCode]
	v.typeConstraintCacheMu.RUnlock()
	if ok {
		return sd
	}

	sd = v.registry.GetByType(typeCode)
	if sd == nil || sd.Snapshot == nil || sd.Kind != "complex-type" {
		sd = nil
	} else {
		hasConstraints := false
		for i := range sd.Snapshot.Element {
			if len(sd.Snapshot.Element[i].Constraint) > 0 {
				hasConstraints = true
				break
			}
		}
		if !hasConstraints {
			sd = nil
		}
	}

	v.typeConstraintCacheMu.Lock()
	v.typeConstraintCache[typeCode] = sd
	v.typeConstraintCacheMu.Unlock()
	return sd
}

// choiceElementPath renders the concrete path of a choice element for one of the types it
// declares — "Observation.value[x]" with Quantity becomes "Observation.valueQuantity".
//
// This is the naming rule the specification defines for [x] elements (§2.1.0: the property
// name is the base name plus the type name, capitalized), applied to the type list read
// from ElementDefinition.type. No path or type is named here.
func choiceElementPath(elemPath, typeCode string) string {
	if typeCode == "" {
		return elemPath
	}
	base := strings.TrimSuffix(elemPath, "[x]")
	return base + strings.ToUpper(typeCode[:1]) + typeCode[1:]
}

// typePathsFor pairs each type a element declares with the concrete path that type occupies
// in an instance. A single-type element yields its own path unchanged; a choice element
// yields one path per declared type.
//
// Only types that carry constraints are returned, so callers do not navigate the instance
// once per declared type — a choice element such as Parameters.parameter.value[x] declares
// around fifty, of which a handful are constrained.
func (v *Validator) typePathsFor(elem *registry.ElementDefinition) []typePath {
	isChoice := strings.HasSuffix(elem.Path, "[x]")

	out := make([]typePath, 0, len(elem.Type))
	for i := range elem.Type {
		typeCode := elem.Type[i].Code
		typeSD := v.constrainedTypeSD(typeCode)
		if typeSD == nil {
			continue
		}
		path := elem.Path
		if isChoice {
			path = choiceElementPath(elem.Path, typeCode)
		}
		out = append(out, typePath{typeCode: typeCode, path: path, sd: typeSD})
	}
	return out
}

// typePath is one declared type of an element, paired with where that type lives in an
// instance and the StructureDefinition holding its constraints.
type typePath struct {
	typeCode string
	path     string
	sd       *registry.StructureDefinition
}

// Validate validates all constraints in a resource.
func (v *Validator) Validate(ctx context.Context, resourceData json.RawMessage, sd *registry.StructureDefinition, opts *ValidateOptions, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	resourceType := sd.RootName(resource)
	if resourceType == "" {
		return
	}

	// Pre-compute resource collection for use as %resource variable in nested constraints.
	resourceCollection, err := types.JSONToCollection(resourceData)
	if err != nil {
		resourceCollection = nil
	}

	// Build eval options shared by all constraints in this resource. A value that is not a resource
	// takes %resource and %rootResource from the resources it sits in.
	resourceVar, rootVar := resourceCollection, resourceCollection
	if opts != nil && opts.Resource != nil {
		resourceVar, rootVar = opts.Resource, opts.Resource
	}
	if opts != nil && opts.RootResource != nil {
		rootVar = opts.RootResource
	}
	evalOpts := v.buildEvalOpts(ctx, resourceVar, rootVar, opts)

	// Evaluate constraints on ALL elements in the snapshot.
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]

		if len(elem.Constraint) == 0 {
			continue
		}

		// Skip slice-specific elements (ID contains ":") to avoid duplicate evaluation.
		if elem.ID != "" && strings.Contains(elem.ID, ":") {
			continue
		}

		if elem.Path == resourceType {
			// Root element: evaluate against full resource.
			v.evaluateConstraintsWithCtx(resourceData, elem.Constraint, resourceType, elem.Path, evalOpts, result)
			continue
		}

		// Nested element: extract instances and evaluate each.
		instances := extractElementInstances(resource, elem.Path, resourceType, resourceType)
		for _, inst := range instances {
			v.evaluateConstraintsWithCtx(inst.data, elem.Constraint, inst.fhirPath, concretePath(elem.Path, inst.fhirPath), evalOpts, result)
		}
	}

	// Evaluate constraints from data type StructureDefinitions.
	// The resource snapshot may not include sub-elements of complex types (e.g.,
	// Patient.name is type HumanName, but Patient.name.period is not in the snapshot).
	// We need to find all complex-type elements, extract their instances, and evaluate
	// constraints defined in the type's SD (e.g., per-1 on Period).
	v.evaluateTypeConstraints(resource, sd, resourceType, evalOpts, result)

	// Validate constraints on contained resources.
	v.validateContainedConstraints(ctx, resource, resourceData, resourceType, opts, result)
}

// validateContainedConstraints validates constraints on contained resources.
// The rootResourceData is the parent resource's JSON — used as %rootResource per FHIRPath spec.
func (v *Validator) validateContainedConstraints(ctx context.Context, resource map[string]any, rootResourceData json.RawMessage, baseFhirPath string, vopts *ValidateOptions, result *issue.Result) {
	containedRaw, ok := resource["contained"]
	if !ok {
		return
	}

	contained, ok := containedRaw.([]any)
	if !ok {
		return
	}

	// Build root resource collection for %rootResource.
	rootResourceCol, _ := types.JSONToCollection(rootResourceData)

	for i, item := range contained {
		resourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		if resourceType == "" {
			continue
		}

		// Get the StructureDefinition for this contained resource type.
		containedSD := v.registry.GetByType(resourceType)
		if containedSD == nil || containedSD.Snapshot == nil {
			continue
		}

		// Convert back to JSON for constraint evaluation.
		containedJSON, err := json.Marshal(resourceMap)
		if err != nil {
			continue
		}

		// Build resource collection for the contained resource itself (%resource).
		containedCollection, _ := types.JSONToCollection(containedJSON)

		containedFhirPath := fmt.Sprintf("%s.contained[%d]", baseFhirPath, i)

		// Build eval options: %resource = contained resource, %rootResource = parent resource.
		evalOpts := v.buildEvalOpts(ctx, containedCollection, rootResourceCol, vopts)

		// Evaluate constraints on ALL elements of the contained resource.
		for j := range containedSD.Snapshot.Element {
			elem := &containedSD.Snapshot.Element[j]

			if len(elem.Constraint) == 0 {
				continue
			}

			if elem.ID != "" && strings.Contains(elem.ID, ":") {
				continue
			}

			if elem.Path == resourceType {
				v.evaluateConstraintsWithCtx(containedJSON, elem.Constraint, containedFhirPath, elem.Path, evalOpts, result)
				continue
			}

			// Nested element in contained resource.
			instances := extractElementInstances(resourceMap, elem.Path, resourceType, containedFhirPath)
			for _, inst := range instances {
				v.evaluateConstraintsWithCtx(inst.data, elem.Constraint, inst.fhirPath, concretePath(elem.Path, inst.fhirPath), evalOpts, result)
			}
		}
	}
}

// buildEvalOpts constructs constraintEvalOpts from the validator state and per-call options.
func (v *Validator) buildEvalOpts(ctx context.Context, resourceCol, rootResourceCol fhirpath.Collection, vopts *ValidateOptions) *constraintEvalOpts {
	opts := &constraintEvalOpts{
		ctx:             ctx,
		resourceCol:     resourceCol,
		rootResourceCol: rootResourceCol,
		timeout:         5 * time.Second,
	}

	// Wire terminology service if available.
	if v.termRegistry != nil {
		opts.termService = &fhirpathTermService{termRegistry: v.termRegistry}
	}

	// Wire resolver if Bundle data is available.
	if vopts != nil && vopts.BundleData != nil {
		opts.resolver = &fhirpathResolver{bundleData: vopts.BundleData}
	}

	return opts
}

// evaluateConstraintsWithCtx evaluates all constraints on an element using eval.Context.
// This is the unified method that handles both root and nested element constraints,
// wiring resolve(), memberOf(), %resource, %rootResource, and timeout. The definition path
// defPath names a choice element as the instance does ("Observation.valueQuantity"); the model
// types the focus from it.
func (v *Validator) evaluateConstraintsWithCtx(data json.RawMessage, constraints []registry.Constraint, fhirPath, defPath string, opts *constraintEvalOpts, result *issue.Result) {
	for _, c := range constraints {
		if c.Expression == "" {
			continue
		}

		if v.isBestPractice(c.Key) {
			continue
		}

		expr, err := v.getCompiledExpression(c.Expression)
		if err != nil {
			// An expression that does not parse cannot hold, whatever the constraint's severity.
			// The HL7 validator reports it the same way, as an error (checkInvariant,
			// PROBLEM_PROCESSING_EXPRESSION).
			result.AddErrorWithID(
				issue.DiagConstraintCompileError,
				map[string]any{
					"key":   c.Key,
					"error": err.Error(),
				},
				fhirPath,
			)
			continue
		}

		evalResult, err := v.evaluateWithContext(expr, data, defPath, opts)
		if err != nil {
			if opts.ctx.Err() != nil {
				return // the validation was canceled; that says nothing about the instance
			}
			if hitTimeLimit(err) {
				// The evaluation stopped at this validator's own time limit, which says nothing
				// about the instance: a processing notice, not a failed invariant.
				result.AddWarningWithID(issue.DiagConstraintEvalError,
					map[string]any{"key": c.Key, "error": err.Error()}, fhirPath)
				continue
			}
			// Any other error leaves the invariant unsatisfied. It fails at its own
			// severity, as in the HL7 validator, whose checkInvariant takes an exception from
			// the FHIRPath engine as a failed invariant.
			v.addConstraintViolation(c, fhirPath, err, result)
			continue
		}

		if !v.constraintPassed(evalResult) {
			v.addConstraintViolation(c, fhirPath, nil, result)
		}
	}
}

// hitTimeLimit reports whether an evaluation error is the time limit set in buildEvalOpts.
func hitTimeLimit(err error) bool {
	var evalErr *eval.EvalError
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &evalErr) && evalErr.Type == eval.ErrTimeout)
}

// evaluateWithContext builds an eval.Context with all services wired and evaluates the expression.
func (v *Validator) evaluateWithContext(expr *fhirpath.Expression, data json.RawMessage, defPath string, opts *constraintEvalOpts) (fhirpath.Collection, error) {
	model := v.model()
	evalCtx := eval.NewContextForRoot(focus(model, data, defPath))
	if model != nil {
		// The types come from the loaded definitions, not from the engine's guesses: without them
		// a string that begins with four digits is read as a date.
		evalCtx.SetModel(model)
		evalCtx.SetPath(defPath)
	}

	// Wire Go context with timeout.
	goCtx := opts.ctx
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		goCtx, cancel = context.WithTimeout(goCtx, opts.timeout)
		defer cancel()
	}
	evalCtx.SetContext(goCtx)

	// Set safety limits.
	evalCtx.SetLimit("maxDepth", 100)
	evalCtx.SetLimit("maxCollectionSize", 10000)

	// Override %resource if provided (for nested elements, this points to the full resource).
	if opts.resourceCol != nil {
		evalCtx.SetVariable("resource", opts.resourceCol)
	}

	// Override %rootResource if provided (for contained/Bundle resources, this points to the parent).
	if opts.rootResourceCol != nil {
		evalCtx.SetVariable("rootResource", opts.rootResourceCol)
	}

	// Wire resolve() support.
	if opts.resolver != nil {
		evalCtx.SetResolver(opts.resolver)
	}

	// Wire memberOf() support.
	if opts.termService != nil {
		evalCtx.SetTerminologyService(opts.termService)
	}

	return expr.EvaluateWithContext(evalCtx)
}

// model is the FHIRPath model of the validator's registry, or nil without a registry.
func (v *Validator) model() *registry.FHIRPathModel {
	if v.registry == nil {
		return nil
	}
	return v.registry.FHIRPathModel()
}

// focus reads the element a constraint is evaluated on as the type the model assigns its
// definition path: a resource or a data type by its own name, an element by the type of its
// definition, so an object's fields resolve and a primitive is read as its FHIR type
// ("2019-12-08" at a dateTime is a dateTime, not a date).
func focus(model *registry.FHIRPathModel, data json.RawMessage, defPath string) fhirpath.Collection {
	var typ string
	if model != nil && defPath != "" {
		typ = model.TypeOf(defPath)
		if typ == "" && model.HasType(defPath) {
			typ = defPath
		}
	}
	col, _ := types.JSONToCollectionWithType(data, typ)
	return col
}

// concretePath is the definition path of an element instance: defPath, except that a choice
// element ("Observation.value[x]") is named by the property the instance at fhirPath uses
// ("Observation.valueQuantity").
func concretePath(defPath, fhirPath string) string {
	parent, ok := strings.CutSuffix(defPath, "[x]")
	if !ok {
		return defPath
	}
	if i := strings.LastIndexByte(parent, '.'); i >= 0 {
		parent = parent[:i]
	}
	key := fhirPath[strings.LastIndexByte(fhirPath, '.')+1:]
	if i := strings.IndexByte(key, '['); i >= 0 {
		key = key[:i]
	}
	return parent + "." + key
}

// getCompiledExpression returns a cached compiled expression or compiles a new one.
func (v *Validator) getCompiledExpression(expr string) (*fhirpath.Expression, error) {
	v.exprCacheMu.RLock()
	compiled, ok := v.exprCache[expr]
	v.exprCacheMu.RUnlock()
	if ok {
		return compiled, nil
	}

	// Compile the expression.
	compiled, err := fhirpath.Compile(expr)
	if err != nil {
		return nil, err
	}

	// Cache it.
	v.exprCacheMu.Lock()
	v.exprCache[expr] = compiled
	v.exprCacheMu.Unlock()

	return compiled, nil
}

// constraintPassed checks if a FHIRPath result indicates the constraint passed.
func (v *Validator) constraintPassed(result fhirpath.Collection) bool {
	// Empty collection = constraint not applicable = passes.
	if result.Empty() {
		return true
	}

	// Try to convert to boolean using Collection's ToBoolean method.
	b, err := result.ToBoolean()
	if err != nil {
		// If conversion fails, treat non-empty collection as truthy.
		return true
	}

	return b
}

// addConstraintViolation adds an issue for a failed constraint. When evalErr is not nil, it is
// why the constraint could not be evaluated.
func (v *Validator) addConstraintViolation(c registry.Constraint, fhirPath string, evalErr error, result *issue.Result) {
	diag := fmt.Sprintf("Constraint failed: %s: '%s'", c.Key, c.Human)
	if c.Source != "" {
		diag += fmt.Sprintf(" (defined in %s)", c.Source)
	}
	if evalErr != nil {
		diag += fmt.Sprintf(" (could not be evaluated: %s)", evalErr)
	}
	if c.Severity == "warning" {
		diag += " (Best Practice Recommendation)"
	}

	params := map[string]any{
		"key":     c.Key,
		"human":   c.Human,
		"details": diag,
	}

	if c.Severity == "error" {
		result.AddErrorWithID(issue.DiagConstraintFailed, params, fhirPath)
	} else {
		result.AddWarningWithID(issue.DiagConstraintFailed, params, fhirPath)
	}
}

// IsBestPractice returns true if the constraint is a best-practice recommendation.
// All FHIR spec constraints are now evaluated - none are skipped.
// Dom-3: contained resource references - works with fhirpath v1.0.2.
// Dom-6: narrative requirement - warning severity per FHIR spec.
func (v *Validator) isBestPractice(_ string) bool {
	return false
}

// evaluateTypeConstraints evaluates constraints from data type SDs on complex-typed elements.
// Resource snapshots don't always expand sub-elements of complex types (e.g., Patient.name
// is HumanName, but Patient.name.period doesn't appear in the Patient snapshot).
// This method walks the resource snapshot, and for each complex-typed element, loads the
// type's SD and recursively evaluates its constraints on matching instances.
func (v *Validator) evaluateTypeConstraints(resource map[string]any, sd *registry.StructureDefinition, resourceType string, evalOpts *constraintEvalOpts, result *issue.Result) {
	if sd.Snapshot == nil {
		return
	}

	// Build a set of element paths in the resource snapshot so we can skip
	// type SD elements that are already covered by the main constraint loop.
	snapshotPaths := make(map[string]struct{}, len(sd.Snapshot.Element))
	for i := range sd.Snapshot.Element {
		snapshotPaths[sd.Snapshot.Element[i].Path] = struct{}{}
	}

	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]

		// Every type the element declares, not just the first. A choice element such as
		// Observation.value[x] declares many, and each one it can hold brings its own
		// constraints — qty-3 when it holds a Quantity, per-1 when it holds a Period.
		// Treating only single-type elements made every constraint of every choice type
		// unreachable, which in R4 is 167 elements across 15 constrained types.
		//
		// Primitives, resources and backbone elements yield nothing here: what comes back
		// is decided by Kind and by whether the type carries constraints, both read from
		// the StructureDefinition.
		for _, tp := range v.typePathsFor(elem) {
			// Extract only the instances of this specific type. A choice element holds one
			// type at a time, so the concrete path is what distinguishes valueQuantity from
			// valuePeriod and keeps a type's constraints off a sibling's data.
			instances := extractElementInstances(resource, tp.path, resourceType, resourceType)
			if len(instances) == 0 {
				continue
			}

			// Pass the resource snapshot paths so we can skip elements already covered.
			v.evaluateTypeSDConstraints(instances, tp.sd, tp.typeCode, tp.path, resourceType, snapshotPaths, evalOpts, result)
		}
	}
}

// evaluateTypeSDConstraints evaluates all constraints from a type SD on a set of instances,
// and recursively evaluates constraints from nested complex-type SDs.
// ElemPath is the resource-level path (e.g., "Patient.name"), resourceType is the root type,
// and snapshotPaths contains paths already present in the resource snapshot (to skip duplicates).
func (v *Validator) evaluateTypeSDConstraints(instances []elementInstance, typeSD *registry.StructureDefinition, typeCode, elemPath, resourceType string, snapshotPaths map[string]struct{}, evalOpts *constraintEvalOpts, result *issue.Result) {
	for i := range typeSD.Snapshot.Element {
		typeElem := &typeSD.Snapshot.Element[i]

		if typeElem.Path == typeCode {
			// Root element of the type: evaluate its constraints against each instance.
			if len(typeElem.Constraint) > 0 {
				for _, inst := range instances {
					v.evaluateConstraintsWithCtx(inst.data, typeElem.Constraint, inst.fhirPath, typeElem.Path, evalOpts, result)
				}
			}
			continue
		}

		// Build the resource-level path for this sub-element.
		suffix := strings.TrimPrefix(typeElem.Path, typeCode+".")
		resourcePath := elemPath + "." + suffix

		// Skip if this sub-path is already in the resource snapshot (handled by main loop).
		if _, inSnapshot := snapshotPaths[resourcePath]; inSnapshot {
			continue
		}

		if len(typeElem.Constraint) > 0 {
			v.evaluateSubElementConstraints(instances, typeElem, typeCode, evalOpts, result)
		}

		v.recurseIntoSubType(instances, typeElem, typeCode, resourcePath, resourceType, snapshotPaths, evalOpts, result)
	}
}

// evaluateSubElementConstraints evaluates constraints on a nested element within a type SD.
func (v *Validator) evaluateSubElementConstraints(instances []elementInstance, typeElem *registry.ElementDefinition, typeCode string, evalOpts *constraintEvalOpts, result *issue.Result) {
	suffix := strings.TrimPrefix(typeElem.Path, typeCode+".")
	for _, inst := range instances {
		var instData map[string]any
		if err := json.Unmarshal(inst.data, &instData); err != nil {
			continue
		}
		// A primitive sub-element is evaluated on its JSON value.
		subFhirPath := inst.fhirPath + "." + suffix
		if rawVal, ok := instData[suffix]; ok {
			rawJSON, err := json.Marshal(rawVal)
			if err == nil {
				v.evaluateConstraintsWithCtx(rawJSON, typeElem.Constraint, subFhirPath, typeElem.Path, evalOpts, result)
				continue
			}
		}
		// Fallback: use extractElementInstances for complex sub-paths.
		subInstances := extractElementInstances(instData, typeElem.Path, typeCode, inst.fhirPath)
		for _, sub := range subInstances {
			v.evaluateConstraintsWithCtx(sub.data, typeElem.Constraint, sub.fhirPath, concretePath(typeElem.Path, sub.fhirPath), evalOpts, result)
		}
	}
}

// recurseIntoSubType recurses into the sub-element's type SD if it's a complex type.
func (v *Validator) recurseIntoSubType(instances []elementInstance, typeElem *registry.ElementDefinition, typeCode, resourcePath, resourceType string, snapshotPaths map[string]struct{}, evalOpts *constraintEvalOpts, result *issue.Result) {
	// As above, every declared type. This is the path that reaches a choice element nested
	// inside a data type — Extension.value[x], Dosage.doseAndRate.dose[x] — where the same
	// single-type guard hid the type's constraints.
	for _, tp := range v.typePathsFor(typeElem) {
		// A type nested in itself is already being evaluated by the caller; descending
		// again would repeat its constraints on the same data.
		if tp.typeCode == typeCode {
			continue
		}

		// tp.path is the sub-element's path inside the parent type, already made concrete
		// for this type (Extension.value[x] -> Extension.valueQuantity), so a choice
		// yields only the instances it actually holds.
		//
		// resourcePath is the same position expressed from the resource, and has to be
		// made concrete the same way — it is what the next level reports against.
		resourceSubPath := resourcePath
		if strings.HasSuffix(resourcePath, "[x]") {
			resourceSubPath = choiceElementPath(resourcePath, tp.typeCode)
		}

		for _, inst := range instances {
			var instData map[string]any
			if err := json.Unmarshal(inst.data, &instData); err != nil {
				continue
			}
			subInstances := extractElementInstances(instData, tp.path, typeCode, inst.fhirPath)
			if len(subInstances) > 0 {
				v.evaluateTypeSDConstraints(subInstances, tp.sd, tp.typeCode, resourceSubPath, resourceType, snapshotPaths, evalOpts, result)
			}
		}
	}
}

// extractElementInstances navigates the parsed resource JSON to find all instances
// matching the given ElementDefinition path.
// For example, for path "Patient.contact", it returns each contact item separately.
// For "Observation.component.referenceRange", it navigates component[*].referenceRange[*].
func extractElementInstances(resource map[string]any, elemPath, resourceType, baseFhirPath string) []elementInstance {
	suffix := strings.TrimPrefix(elemPath, resourceType+".")
	if suffix == elemPath {
		return nil // path does not start with resourceType
	}
	segments := strings.Split(suffix, ".")

	current := []jsonNode{{data: resource, fhirPath: baseFhirPath}}

	for i, seg := range segments {
		isLast := i == len(segments)-1
		next := navigateSegment(current, seg, isLast)

		if isLast {
			return nodesToInstances(next)
		}
		current = next
	}

	return nil
}

// navigateSegment advances all current nodes by one path segment, expanding arrays.
func navigateSegment(current []jsonNode, seg string, isLast bool) []jsonNode {
	isChoice := strings.HasSuffix(seg, "[x]")
	baseName := strings.TrimSuffix(seg, "[x]")

	var next []jsonNode
	for _, n := range current {
		if isChoice {
			next = append(next, matchChoiceType(n, baseName, isLast)...)
		} else {
			val, ok := n.data[seg]
			if !ok {
				continue
			}
			next = append(next, resolveValue(val, n.fhirPath+"."+seg, isLast)...)
		}
	}
	return next
}

// matchChoiceType scans a node's keys for choice-type matches (e.g., "value" matches "valueString").
func matchChoiceType(n jsonNode, baseName string, isLast bool) []jsonNode {
	var nodes []jsonNode
	for key, val := range n.data {
		if !strings.HasPrefix(key, baseName) || len(key) <= len(baseName) {
			continue
		}
		if !unicode.IsUpper(rune(key[len(baseName)])) {
			continue
		}
		nodes = append(nodes, resolveValue(val, n.fhirPath+"."+key, isLast)...)
	}
	return nodes
}

// nodesToInstances converts jsonNodes to elementInstances by marshaling each to JSON.
func nodesToInstances(nodes []jsonNode) []elementInstance {
	instances := make([]elementInstance, 0, len(nodes))
	for _, n := range nodes {
		if n.value != nil {
			instances = append(instances, elementInstance{data: n.value, fhirPath: n.fhirPath})
			continue
		}
		raw, err := json.Marshal(n.data)
		if err != nil {
			continue
		}
		instances = append(instances, elementInstance{data: raw, fhirPath: n.fhirPath})
	}
	return instances
}

// resolveValue handles array expansion and type assertion for JSON navigation.
// When isLast is true, primitive values are kept too, as their JSON value: a constraint on a
// primitive element is evaluated with the value as its focus.
func resolveValue(val any, fhirPath string, isLast bool) []jsonNode {
	switch v := val.(type) {
	case map[string]any:
		return []jsonNode{{data: v, fhirPath: fhirPath}}
	case []any:
		nodes := make([]jsonNode, 0, len(v))
		for i, item := range v {
			itemPath := fmt.Sprintf("%s[%d]", fhirPath, i)
			switch it := item.(type) {
			case map[string]any:
				nodes = append(nodes, jsonNode{data: it, fhirPath: itemPath})
			default:
				if isLast && it != nil {
					if raw, err := json.Marshal(it); err == nil {
						nodes = append(nodes, jsonNode{value: raw, fhirPath: itemPath})
					}
				}
			}
		}
		return nodes
	default:
		if isLast && v != nil {
			if raw, err := json.Marshal(v); err == nil {
				return []jsonNode{{value: raw, fhirPath: fhirPath}}
			}
		}
	}
	return nil
}
