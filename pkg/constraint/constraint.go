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

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
	"github.com/gofhir/validator/v2/pkg/terminology"
)

// ValidateOptions holds per-call options for constraint validation.
type ValidateOptions struct {
	// BundleData is the parsed Bundle JSON, enabling resolve() in FHIRPath.
	// When non-nil, a resolver is created that can find resources by fullUrl.
	BundleData map[string]any
	// OuterBundles are the Bundles that hold BundleData, innermost first, where resolve() looks for
	// a reference BundleData does not resolve.
	OuterBundles []map[string]any
	// Exact returns an object of BundleData decoded with its numbers as the JSON spells them, or nil
	// when it has none: what resolve() returns, so that a decimal keeps its precision (1.50 is not
	// 1.5). Nil returns the objects as parsed.
	Exact func(map[string]any) map[string]any

	// Resource and RootResource are the resources the validated value sits in, for %resource and
	// %rootResource, as FHIRPath collections, when the value is not itself the root of its
	// variables (a datatype or extension checked against its profile, or a contained resource).
	// The caller prepares them once, so a large Bundle is not converted per check. Nil means the
	// value is the resource.
	Resource     fhirpath.Collection
	RootResource fhirpath.Collection

	// Data is the instance resourceData holds, already parsed by the caller. Slice matching, which
	// tells the slice a value belongs to, keeps what it learns about a value (its conformance to a
	// profile) by the value's identity, so the phases that share a parse share that too. Nil means
	// resourceData is parsed here.
	Data map[string]any

	// Scope, Resolver and Containment are what slice matching reads (slicematch.Request). A nil
	// Scope means the instance is its own resource, root resource and container.
	Scope       *slicematch.Scope
	Resolver    slicematch.Resolver
	Containment func(container, resource map[string]any) bool
}

// constraintEvalOpts carries all contextual data for a single constraint evaluation.
type constraintEvalOpts struct {
	ctx             context.Context
	resourceCol     fhirpath.Collection // %resource variable.
	rootResourceCol fhirpath.Collection // %rootResource variable (for contained/Bundle).
	resolver        eval.Resolver       // For resolve() in FHIRPath.
	// exact returns an object with its numbers as the JSON spells them (ValidateOptions.Exact), for
	// what resolve() returns.
	exact       func(map[string]any) map[string]any
	termService eval.TerminologyService
	timeout     time.Duration

	// What slice matching reads: the resources the value is in, and how references resolve.
	scope         slicematch.Scope
	sliceResolver slicematch.Resolver
	containment   func(container, resource map[string]any) bool
}

// Validator validates constraints defined in ElementDefinitions.
type Validator struct {
	registry     *registry.Registry
	termRegistry *terminology.Registry

	// Cache of compiled FHIRPath expressions.
	exprCache   map[string]compiledExpr
	exprCacheMu sync.RWMutex

	// matcher tells the slice a value of a sliced element belongs to.
	matcher *slicematch.Matcher

	// definitions names the definition a value declares for itself.
	definitions DefinitionSource

	// values checks a value against each definition the walk finds governing it, besides its
	// invariants: its fixed and pattern values.
	values ValueChecker
}

// ValueChecker checks a value against an element definition. Implemented by fixedpattern.Checker.
type ValueChecker interface {
	// Governs reports whether def has anything a value is checked against.
	Governs(def *registry.ElementDefinition) bool
	// CheckValue reports each issue through report with its diagnostic, parameters and location.
	// Its raw argument is the value's JSON as the resource writes it (nil for a primitive that has
	// only extensions), and element a primitive's "_x" sibling (nil for none).
	CheckValue(def *registry.ElementDefinition, raw, element json.RawMessage, fhirPath string, report func(issue.DiagnosticID, map[string]any, string))
}

// DefinitionSource names the definition a value declares for itself, which governs the value
// wherever it is used, besides the definitions of the element that holds it: an extension's url
// names the definition the extension conforms to (extensibility.html). It returns nil for a value
// that declares none, or whose definition cannot be used. The definition has a snapshot.
type DefinitionSource interface {
	DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *registry.StructureDefinition
}

// Option configures a Validator.
type Option func(*Validator)

// WithMatcher sets the slice matcher, the one the slicing phase uses, so that both assign a value
// to the same slice. Without it, the validator's matcher cannot check profile conformance or
// ValueSet membership, and a value only those decide is in no slice.
func WithMatcher(m *slicematch.Matcher) Option { return func(v *Validator) { v.matcher = m } }

// WithDefinitions sets the source of the definitions values declare for themselves. Without it, a
// value is checked against the definitions of the element that holds it only.
func WithDefinitions(d DefinitionSource) Option { return func(v *Validator) { v.definitions = d } }

// WithValueChecker checks every value the walk reaches against each definition that governs it, as
// the invariants are (ValueChecker).
func WithValueChecker(c ValueChecker) Option { return func(v *Validator) { v.values = c } }

// New creates a new constraint Validator.
// The termRegistry may be nil to disable memberOf() support (e.g., when -tx n/a is set).
func New(reg *registry.Registry, termReg *terminology.Registry, opts ...Option) *Validator {
	v := &Validator{
		registry:     reg,
		termRegistry: termReg,
		exprCache:    make(map[string]compiledExpr),
	}
	for _, o := range opts {
		o(v)
	}
	if v.matcher == nil {
		v.matcher = slicematch.New(reg)
	}
	return v
}

// Validate validates all constraints in a resource, or in a datatype value checked against its
// profile, walking it with its definition's element tree (see tree.go).
func (v *Validator) Validate(ctx context.Context, resourceData json.RawMessage, sd *registry.StructureDefinition, opts *ValidateOptions, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !hasReportScope(ctx) {
		// The definitions that govern one value report an issue once, however the caller scopes it.
		ctx = WithReportScope(ctx)
	}
	var resource map[string]any
	if opts != nil && opts.Data != nil {
		resource = opts.Data
	} else if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}
	root := sd.Tree().Root()
	if root == nil || sd.RootName(resource) == "" {
		return
	}

	// A value that is not a resource takes %resource and %rootResource from the resources it sits
	// in; a resource is both.
	resourceCollection := rootCollection(resourceData)
	resourceVar, rootVar := resourceCollection, resourceCollection
	if opts != nil && opts.Resource != nil {
		resourceVar, rootVar = opts.Resource, opts.Resource
	}
	if opts != nil && opts.RootResource != nil {
		rootVar = opts.RootResource
	}
	evalOpts := v.buildEvalOpts(ctx, resourceVar, rootVar, opts)
	if evalOpts.scope.Resource == nil {
		evalOpts.scope = slicematch.Scope{Resource: resource, RootResource: resource, Container: resource}
	}
	evalOpts.resolver = resolverWithin(evalOpts.resolver, evalOpts.scope.RootResource, nil)

	v.walk(sd, root, "", resource, resourceData, nil, sd.RootName(resource), evalOpts, result)
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
		opts.resolver = &fhirpathResolver{bundleData: vopts.BundleData, outer: vopts.OuterBundles, exact: vopts.Exact}
	}
	if vopts != nil {
		opts.exact = vopts.Exact
		opts.sliceResolver, opts.containment = vopts.Resolver, vopts.Containment
		if vopts.Scope != nil {
			opts.scope = *vopts.Scope
		}
	}

	return opts
}

// evaluateConstraintsWithCtx evaluates all constraints on an element using eval.Context.
// This is the unified method that handles both root and nested element constraints,
// wiring resolve(), memberOf(), %resource, %rootResource, and timeout. The definition path
// defPath names a choice element as the instance does ("Observation.valueQuantity"); the model
// types the focus from it.
func (v *Validator) evaluateConstraintsWithCtx(data json.RawMessage, constraints []registry.Constraint, fhirPath, defPath string, opts *constraintEvalOpts, result *issue.Result) {
	v.evaluateConstraintsOn(func() fhirpath.Collection { return focus(v.model(), data, defPath) }, constraints, fhirPath, defPath, opts, result)
}

// evaluateConstraintsOn evaluates constraints as evaluateConstraintsWithCtx does, on the focus that
// on returns, read once and only when an expression compiles.
func (v *Validator) evaluateConstraintsOn(on func() fhirpath.Collection, constraints []registry.Constraint, fhirPath, defPath string, opts *constraintEvalOpts, result *issue.Result) {
	var focused fhirpath.Collection
	read := false
	for _, c := range constraints {
		if c.Expression == "" {
			continue
		}

		if v.isBestPractice(c.Key) {
			continue
		}

		expr, err := v.getCompiledExpression(c.Expression)
		if err != nil {
			v.reportCompileError(c, err, fhirPath, opts, result)
			continue
		}

		if !read {
			focused, read = on(), true
		}
		evalResult, err := v.evaluateOn(expr, focused, defPath, opts)
		if err != nil {
			if opts.ctx.Err() != nil {
				return // the validation was canceled; that says nothing about the instance
			}
			if hitTimeLimit(err) {
				// The evaluation stopped at this validator's own time limit, which says nothing
				// about the instance: a processing notice, not a failed invariant.
				result.AddWarningWithID(issue.DiagConstraintEvalError,
					failureParams(c, err), fhirPath)
				continue
			}
			// Any other error leaves the invariant unsatisfied. It fails at its own
			// severity, as in the HL7 validator, whose checkInvariant takes an exception from
			// the FHIRPath engine as a failed invariant.
			if firstReport(opts.ctx, c, fhirPath) {
				v.addConstraintViolation(c, fhirPath, err, result)
			}
			continue
		}

		if !toBoolean(evalResult) {
			if firstReport(opts.ctx, c, fhirPath) {
				v.addConstraintViolation(c, fhirPath, nil, result)
			}
		}
	}
}

// failureParams are the template parameters of a constraint the engine could not compile or
// evaluate.
func failureParams(c registry.Constraint, err error) map[string]any {
	return map[string]any{"key": c.Key, "error": err.Error()}
}

// definedByBaseType reports whether c comes from a definition that defines a type rather than
// constraining one (StructureDefinition.derivation other than constraint), as the specification's
// own resource and data type definitions do. A constraint whose source is not loaded is taken as a
// profile's.
func (v *Validator) definedByBaseType(c registry.Constraint) bool {
	if v.registry == nil || c.Source == "" {
		return false
	}
	url, _ := registry.ParseCanonical(c.Source)
	sd := v.registry.GetByURL(url)
	return sd != nil && sd.Derivation != registry.DerivationConstraint
}

// hitTimeLimit reports whether an evaluation error is the time limit set in buildEvalOpts.
func hitTimeLimit(err error) bool {
	var evalErr *eval.EvalError
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &evalErr) && evalErr.Type == eval.ErrTimeout)
}

// reportCompileError reports that c's expression does not compile, once per location, as a failure
// is: every definition that inherits the expression fails to compile it the same way.
func (v *Validator) reportCompileError(c registry.Constraint, err error, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {
	if !firstReport(opts.ctx, c, fhirPath) {
		return
	}
	params := failureParams(c, err)
	if v.definedByBaseType(c) {
		// A constraint of the specification's own definitions that does not parse is a defect of
		// the specification, not of the instance (R5's eld-11 quotes a string with double
		// quotes): a processing warning.
		result.AddWarningWithID(issue.DiagConstraintCompileError, params, fhirPath)
		return
	}
	// Any other expression that does not parse cannot hold, whatever the constraint's severity.
	// The HL7 validator reports it the same way, as an error (checkInvariant,
	// PROBLEM_PROCESSING_EXPRESSION).
	result.AddErrorWithID(issue.DiagConstraintCompileError, params, fhirPath)
}

// evaluateOn builds an eval.Context with all services wired and evaluates the expression with
// focus, a value read from the input whose definition path is defPath.
func (v *Validator) evaluateOn(expr *fhirpath.Expression, focus fhirpath.Collection, defPath string, opts *constraintEvalOpts) (fhirpath.Collection, error) {
	model := v.model()
	evalCtx := eval.NewContextForRoot(focus)
	if model != nil {
		// The types come from the loaded definitions, not from the engine's guesses: without them
		// a string that begins with four digits is read as a date.
		evalCtx.SetModel(model)
		evalCtx.SetPath(defPath)
	}

	// The caller's context, and the time limit as a deadline: no timer per evaluation.
	evalCtx.SetContext(opts.ctx)
	if opts.timeout > 0 {
		release := evalCtx.SetDeadline(time.Now().Add(opts.timeout))
		defer release()
	}

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
	col, _ := types.JSONToCollectionWithType(data, typeOf(model, defPath))
	// The focus is read for the evaluations of one element's invariants, in one goroutine: it may
	// keep what it works out about itself (its type, the fields read) rather than work it out
	// again. What it keeps depends on the data alone, so every evaluation may share it.
	for _, v := range col {
		if obj, ok := v.(*types.ObjectValue); ok {
			obj.MarkPrivate()
		}
	}
	return col
}

// typeOf is the type the model assigns defPath: a resource or a data type by its own name, an
// element by the type of its definition; "" without a model.
func typeOf(model *registry.FHIRPathModel, defPath string) string {
	if model == nil || defPath == "" {
		return ""
	}
	if typ := model.TypeOf(defPath); typ != "" {
		return typ
	}
	if model.HasType(defPath) {
		return defPath
	}
	return ""
}

// getCompiledExpression returns a cached compiled expression or compiles a new one.
func (v *Validator) getCompiledExpression(expr string) (*fhirpath.Expression, error) {
	v.exprCacheMu.RLock()
	cached, ok := v.exprCache[expr]
	v.exprCacheMu.RUnlock()
	if ok {
		return cached.expr, cached.err
	}

	// Compile the expression. A failure is cached too: an expression that does not parse is
	// otherwise parsed again on every element and every validation it applies to.
	compiled, err := fhirpath.Compile(expr)
	v.exprCacheMu.Lock()
	v.exprCache[expr] = compiledExpr{compiled, err}
	v.exprCacheMu.Unlock()

	return compiled, err
}

// compiledExpr is a compiled expression, or why it does not compile.
type compiledExpr struct {
	expr *fhirpath.Expression
	err  error
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

// bundleType is the resource whose entries resolve() finds references in (bundle.html#references).
const bundleType = "Bundle"

// ScopeRoot is the resource a Scope evaluates the expressions of extension contexts on.
type ScopeRoot struct {
	// Resource is the resource, as parsed.
	Resource map[string]any
	// Raw is the JSON Resource was parsed from, which the expressions read; nil reads Resource.
	Raw []byte
	// Bundle is the Bundle being validated, which resolve() finds references in, when Resource is
	// in one (a slice's conformance check).
	Bundle map[string]any
	// Outer are the Bundles that hold Bundle, the innermost first, where resolve() looks after it.
	Outer []map[string]any
	// Exact returns an object of Resource, or of Bundle, with its numbers as the JSON spells them
	// (1.50 is not 1.5), or nil when it has none: what resolve() returns, and what is read when Raw
	// is nil. Nil reads the objects as parsed.
	Exact func(map[string]any) map[string]any
}

// Scope returns what evaluates the expressions of extension contexts on root's resource;
// Scope.Within, on a resource it holds. The references resolve() follows are found in the Bundles
// that hold the resource, as for the profiles' invariants. The resource is read once, when an expression first
// needs it, and the resources it holds are nodes of that reading; what each expression selects is
// kept.
func (v *Validator) Scope(ctx context.Context, root ScopeRoot) *Scope {
	s := &Scope{v: v, ctx: ctx, raw: root.Raw, data: root.Resource, exact: root.Exact, selected: map[string]map[string]struct{}{}}
	if root.Bundle != nil {
		s.bundles = append([]map[string]any{root.Bundle}, root.Outer...)
	}
	if rt, _ := root.Resource[resourceTypeKey].(string); rt == bundleType && root.Bundle == nil {
		s.bundles = []map[string]any{root.Resource}
	}
	return s
}

// Scope evaluates the expressions of extension contexts on one resource (Validator.Scope). It is
// used by one goroutine.
type Scope struct {
	v      *Validator
	ctx    context.Context
	raw    []byte         // the JSON the root was parsed from; nil for a resource another holds
	parent *Scope         // the scope of the resource that holds this one; nil for the root
	at     string         // where the resource is in root: "" for root, "Bundle.entry[0].resource"
	data   map[string]any // the resource
	// contained reports a resource its parent contains, whose %rootResource the parent is.
	contained bool
	// bundles are the Bundles resolve() finds references in, innermost first.
	bundles []map[string]any
	exact   func(map[string]any) map[string]any // see ScopeRoot.Exact

	read     bool
	in       evaluation
	err      error
	selected map[string]map[string]struct{}       // the places each expression selects
	held     map[string]map[string]fhirpath.Value // the resources it holds, by path and place
}

// Within returns the scope of resource, at at in root ("Bundle.entry[0].resource",
// "Observation.contained[0]"), a resource the scope's resource holds: contained, its %rootResource is
// the scope's resource; otherwise its own. When resource is a Bundle, resolve() looks for a
// reference in it first, then in the Bundles that hold it. A resource that is not where at says is
// evaluated as a root of its own.
func (s *Scope) Within(at string, resource map[string]any, contained bool) *Scope {
	bundles := s.bundles
	if rt, _ := resource[resourceTypeKey].(string); rt == bundleType {
		bundles = append([]map[string]any{resource}, s.bundles...)
	}
	return &Scope{v: s.v, ctx: s.ctx, parent: s, at: at, data: resource, contained: contained, bundles: bundles, exact: s.exact, selected: map[string]map[string]struct{}{}}
}

// Holds reports whether expression, a FHIRPath invariant, is true on value, an element of the
// resource whose definition path, as the definitions type it, is focusPath ("Observation.valueQuantity",
// "Questionnaire.item"): an object, or a primitive's value, with element its "_key" sibling (its id
// and extensions). It is evaluated as this phase evaluates a constraint, and its result converted to
// a boolean as the HL7 validator converts it (toBoolean): an empty result does not hold. An
// evaluation stopped at this validator's time limit is an error that wraps context.DeadlineExceeded.
func (s *Scope) Holds(expression, focusPath string, value any, element map[string]any) (bool, error) {
	expr, err := s.v.getCompiledExpression(expression)
	if err != nil {
		return false, err
	}
	if err := s.readResource(); err != nil || s.in.resource == nil {
		return false, err
	}
	on, err := s.v.focusOn(value, element, focusPath)
	if err != nil || on == nil {
		return false, err
	}
	res, err := s.v.evaluateOn(expr, on, focusPath, s.in.opts)
	if err != nil {
		return false, timeLimited(err)
	}
	return toBoolean(res), nil
}

// focusOn reads value as the type the model assigns focusPath, with element, a primitive's "_key"
// sibling, as the element the primitive carries (json.html#primitive). A primitive with no value is
// its element.
func (v *Validator) focusOn(value any, element map[string]any, focusPath string) (fhirpath.Collection, error) {
	var carried *types.ObjectValue
	if element != nil {
		data, err := json.Marshal(element)
		if err != nil {
			return nil, err
		}
		carried = types.NewObjectValueWithType(data, typeOf(v.model(), focusPath))
	}
	if value == nil {
		if carried == nil {
			return nil, nil
		}
		return fhirpath.Collection{carried}, nil // a primitive with no value, typed as one with
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	on := focus(v.model(), data, focusPath)
	if carried != nil && len(on) == 1 {
		on[0] = withElement(on[0], carried)
	}
	return on, nil
}

// withElement is the primitive p carrying element.
func withElement(p fhirpath.Value, element *types.ObjectValue) fhirpath.Value {
	switch p := p.(type) {
	case types.String:
		return p.WithElement(element)
	case types.Boolean:
		return p.WithElement(element)
	case types.Integer:
		return p.WithElement(element)
	case types.Decimal:
		return p.WithElement(element)
	case types.Date:
		return p.WithElement(element)
	case types.DateTime:
		return p.WithElement(element)
	case types.Time:
		return p.WithElement(element)
	}
	return p
}

// toBoolean is an invariant's result as a boolean, a constraint's or a context invariant's: a
// single Boolean is its value; any other result is true when it is not empty. Empty is false: an
// invariant "must evaluate to true when run on the element" (conformance-rules.html#constraints),
// as the HL7 validator converts it (FHIRPathEngine.convertToBoolean).
func toBoolean(res fhirpath.Collection) bool {
	if len(res) == 1 {
		if b, ok := res[0].(types.Boolean); ok {
			return b.Bool()
		}
	}
	return len(res) > 0
}

// Selects reports whether expression, evaluated from the resource as this phase evaluates a
// constraint on a resource, returns the node at location in root: "Patient.name[0]", and for a
// primitive the element it carries, "Patient.name[0].given[1]". Places are root's, so a node of a
// container and one of the resource it contains are told apart; a node the expression makes rather
// than reads from root is in no place. The places an expression selects are worked out once.
// Errors are as for Holds.
func (s *Scope) Selects(expression, location string) (bool, error) {
	if places, ok := s.selected[expression]; ok {
		_, selected := places[location]
		return selected, nil
	}
	expr, err := s.v.getCompiledExpression(expression)
	if err != nil {
		return false, err
	}
	if err := s.readResource(); err != nil || s.in.resource == nil {
		return false, err
	}
	res, err := s.v.evaluateOn(expr, s.in.resource, s.in.resourceType, s.in.opts)
	if err != nil {
		return false, timeLimited(err)
	}
	places := make(map[string]struct{}, len(res))
	for _, node := range res {
		if place := locationOf(node); place != "" {
			places[place] = struct{}{}
		}
	}
	s.selected[expression] = places
	_, selected := places[location]
	return selected, nil
}

// readResource reads the resource the first time an expression needs it: root from its JSON, and
// a resource it holds as a node of that reading, found once per path among the resources its parent
// holds.
func (s *Scope) readResource() error {
	if s.read {
		return s.err
	}
	s.read = true
	if s.parent == nil {
		s.in, s.err = s.v.evaluationOf(s.ctx, s.data, s.raw, s.bundles, s.exact)
		return s.err
	}
	node, err := s.parent.heldAt(s.at)
	if err != nil {
		s.err = err
		return err
	}
	if node == nil {
		s.in, s.err = s.v.evaluationOf(s.ctx, s.data, nil, s.bundles, s.exact)
		return s.err
	}
	resource := fhirpath.Collection{node}
	rootResource, container := resource, s.data
	if s.contained {
		rootResource, container = s.parent.in.resource, s.parent.data
	}
	opts := s.v.buildEvalOpts(s.ctx, resource, rootResource, bundleOptions(s.bundles))
	opts.resolver = resolverWithin(opts.resolver, container, s.exact)
	s.in = evaluation{resourceType: node.Type(), resource: resource, opts: opts}
	return nil
}

// heldAt is the resource at at in root, one the scope's resource holds, or nil when there is none:
// the resources the scope's resource holds under the same path are found once.
func (s *Scope) heldAt(at string) (fhirpath.Value, error) {
	if err := s.readResource(); err != nil || s.in.resource == nil {
		return nil, err
	}
	prefix := locationOf(s.in.resource[0])
	rest, ok := strings.CutPrefix(at, prefix+".")
	if !ok || prefix == "" {
		return nil, nil
	}
	path := withoutIndexes(rest) // "entry.resource", "contained", "parameter.resource"
	held, ok := s.held[path]
	if !ok {
		expr, err := s.v.getCompiledExpression(path)
		if err != nil {
			return nil, err
		}
		nodes, err := s.v.evaluateOn(expr, s.in.resource, s.in.resourceType, s.in.opts)
		if err != nil {
			return nil, timeLimited(err)
		}
		held = make(map[string]fhirpath.Value, len(nodes))
		for _, node := range nodes {
			if location := locationOf(node); location != "" {
				held[location] = node
			}
		}
		if s.held == nil {
			s.held = map[string]map[string]fhirpath.Value{}
		}
		s.held[path] = held
	}
	return held[at], nil
}

// withoutIndexes is a place without its indexes: "entry.resource" for "entry[3].resource".
func withoutIndexes(place string) string {
	var b strings.Builder
	depth := 0
	for _, r := range place {
		switch {
		case r == '[':
			depth++
		case r == ']':
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// timeLimited marks an evaluation stopped at this validator's own time limit as such: it wraps
// context.DeadlineExceeded.
func timeLimited(err error) error {
	if hitTimeLimit(err) && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", context.DeadlineExceeded, err)
	}
	return err
}

// evaluation is a resource read for evaluating expressions on it.
type evaluation struct {
	resourceType string
	resource     fhirpath.Collection // the resource, read as its type within root
	opts         *constraintEvalOpts
}

// evaluationOf reads root for evaluating expressions on it, from raw, the JSON it was parsed from,
// or when raw is nil from root itself, as exact gives it: %resource and %rootResource are it, and
// resolve() finds references in bundles, innermost first.
func (v *Validator) evaluationOf(ctx context.Context, root map[string]any, raw []byte, bundles []map[string]any, exact func(map[string]any) map[string]any) (evaluation, error) {
	if raw == nil {
		var err error
		if raw, err = json.Marshal(exactOr(exact, root)); err != nil {
			return evaluation{}, err
		}
	}
	rt, _ := root[resourceTypeKey].(string)
	col := focus(v.model(), raw, rt)
	opts := v.buildEvalOpts(ctx, col, col, bundleOptions(bundles))
	opts.resolver = resolverWithin(opts.resolver, root, exact)
	return evaluation{resourceType: rt, resource: col, opts: opts}, nil
}

// exactOr is exact's twin of m, or m when there is none.
func exactOr(exact func(map[string]any) map[string]any, m map[string]any) map[string]any {
	if exact != nil {
		if e := exact(m); e != nil {
			return e
		}
	}
	return m
}

// bundleOptions are the options for resolve() to find references in bundles, innermost first.
func bundleOptions(bundles []map[string]any) *ValidateOptions {
	if len(bundles) == 0 {
		return &ValidateOptions{}
	}
	return &ValidateOptions{BundleData: bundles[0], OuterBundles: bundles[1:]}
}

// locationOf is where node is in the input it was read from, or "" when it was not read from it.
func locationOf(node fhirpath.Value) string {
	if element, ok := types.ElementOf(node); ok {
		return element.Location()
	}
	return ""
}
