// Package constraint validates FHIR constraints (invariants) using FHIRPath.
package constraint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/terminology"
)

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
	exprCache   map[string]compiledExpr
	exprCacheMu sync.RWMutex
}

// New creates a new constraint Validator.
// The termRegistry may be nil to disable memberOf() support (e.g., when -tx n/a is set).
func New(reg *registry.Registry, termReg *terminology.Registry) *Validator {
	return &Validator{
		registry:     reg,
		termRegistry: termReg,
		exprCache:    make(map[string]compiledExpr),
	}
}

// Validate validates all constraints in a resource, or in a datatype value checked against its
// profile, walking it with its definition's element tree (see tree.go).
func (v *Validator) Validate(ctx context.Context, resourceData json.RawMessage, sd *registry.StructureDefinition, opts *ValidateOptions, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}
	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
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

	v.walk(sd, root, "", resource, resourceData, sd.RootName(resource), evalOpts, result)
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
			// Reported once per location, as a failure is: every definition that inherits the
			// expression fails to compile it the same way.
			if !firstReport(opts.ctx, c, fhirPath) {
				continue
			}
			params := failureParams(c, err)
			if v.definedByBaseType(c) {
				// A constraint of the specification's own definitions that does not parse is a
				// defect of the specification, not of the instance (R5's eld-11 quotes a string
				// with double quotes): a processing warning.
				result.AddWarningWithID(issue.DiagConstraintCompileError, params, fhirPath)
				continue
			}
			// Any other expression that does not parse cannot hold, whatever the constraint's
			// severity. The HL7 validator reports it the same way, as an error (checkInvariant,
			// PROBLEM_PROCESSING_EXPRESSION).
			result.AddErrorWithID(issue.DiagConstraintCompileError, params, fhirPath)
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

		if !v.constraintPassed(evalResult) {
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
