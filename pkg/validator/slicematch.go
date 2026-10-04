package validator

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/internal/fhirpathcache"
	"github.com/gofhir/validator/pkg/constraint"
	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/slicematch"
	"github.com/gofhir/validator/pkg/terminology"
)

// The three services the slice matcher needs, backed by this validator.

// conformer decides a "profile" discriminator by running the whole validation pipeline on the
// value against the profile, with a fresh result: the value conforms when that finds no error.
type conformer struct{ v *Validator }

// conformState is shared by the conformance checks of one top-level validation: a memo of the
// answers, and the checks in progress, which answer false when met again (a profile whose
// discriminator requires conforming to itself).
type conformState struct {
	mu      sync.Mutex
	memo    map[conformKey]bool
	running map[conformKey]bool
	// collections caches a resource's FHIRPath collection, for %resource and %rootResource: the
	// same Bundle is the scope of every check inside it.
	collections map[uintptr]fhirpath.Collection
}

// collection returns the FHIRPath collection of a resource, converting it once per validation.
func (st *conformState) collection(m map[string]any) fhirpath.Collection {
	if m == nil {
		return nil
	}
	key := reflect.ValueOf(m).Pointer()
	st.mu.Lock()
	col, ok := st.collections[key]
	st.mu.Unlock()
	if ok {
		return col
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	col, err = types.JSONToCollection(raw)
	if err != nil {
		return nil
	}
	// The state belongs to one validation, which evaluates in one goroutine.
	fhirpathcache.Enable(col)
	st.mu.Lock()
	st.collections[key] = col
	st.mu.Unlock()
	return col
}

type conformKey struct {
	profile string
	value   uintptr // identity of the value's map within this validation
}

type conformStateKey struct{}

func withConformState(ctx context.Context) context.Context {
	if _, ok := ctx.Value(conformStateKey{}).(*conformState); ok {
		return ctx
	}
	return context.WithValue(ctx, conformStateKey{}, &conformState{
		memo: map[conformKey]bool{}, running: map[conformKey]bool{}, collections: map[uintptr]fhirpath.Collection{},
	})
}

// Conforms implements slicematch.Conformer.
func (c conformer) Conforms(ctx context.Context, value any, profile *registry.StructureDefinition, scope slicematch.Scope) bool {
	data, ok := value.(map[string]any)
	if !ok || profile == nil {
		return false
	}
	st, _ := ctx.Value(conformStateKey{}).(*conformState)
	if st == nil {
		ctx = withConformState(ctx)
		st, _ = ctx.Value(conformStateKey{}).(*conformState)
	}
	key := conformKey{profile: profile.URL + "|" + profile.Version, value: reflect.ValueOf(data).Pointer()}

	st.mu.Lock()
	if ans, ok := st.memo[key]; ok {
		st.mu.Unlock()
		return ans
	}
	if st.running[key] {
		st.mu.Unlock()
		return false
	}
	st.running[key] = true
	st.mu.Unlock()

	ans := c.check(ctx, data, profile, scope)

	st.mu.Lock()
	delete(st.running, key)
	st.memo[key] = ans
	st.mu.Unlock()
	return ans
}

func (c conformer) check(ctx context.Context, data map[string]any, profile *registry.StructureDefinition, scope slicematch.Scope) bool {
	// A resource conforms only to a profile of its own type: the phases look elements up by path,
	// and a Patient checked against a Practitioner profile would find nothing to object to.
	if rt, _ := data["resourceType"].(string); rt != "" && rt != profile.Type {
		return false
	}
	// EnsureSnapshot takes the definition's lock; reading Snapshot here would race with a
	// concurrent validation generating it.
	if err := c.v.registry.EnsureSnapshot(ctx, profile); err != nil {
		return false
	}
	// The value as the JSON spells its numbers, when the validation read it (1.50 is not 1.5).
	exact := exactOf(ctx, data)
	if exact == nil {
		exact = data
	}
	raw, err := json.Marshal(exact)
	if err != nil {
		return false
	}
	// A fresh result with its stats, which the phases update.
	result := issue.NewResult()
	result.Stats = &issue.Stats{}
	st, _ := ctx.Value(conformStateKey{}).(*conformState)
	// The check's issues are discarded: it reports into a scope of its own.
	c.v.validateAgainstProfile(withExtensionScope(constraint.WithReportScope(ctx)), data, raw, profile, &valueScope{scope: scope, state: st}, result)
	return result.ErrorCount() == 0
}

// valueScope marks a run of the pipeline on a value inside another resource (a conformance
// check), with the resources it sits in.
type valueScope struct {
	scope slicematch.Scope
	state *conformState
}

// constraintOptions returns the resources a value's constraints read as %resource and
// %rootResource.
func (s *valueScope) constraintOptions() *constraint.ValidateOptions {
	opts := &constraint.ValidateOptions{BundleData: s.scope.Container, Scope: &s.scope}
	if s.state != nil {
		opts.Resource = s.state.collection(s.scope.Resource)
		opts.RootResource = s.state.collection(s.scope.RootResource)
	}
	return opts
}

// referenceResolver follows references for resolve(): "#id" among the contained resources of
// the resource that makes the reference (its %rootResource, which a contained resource shares with
// its container; references.html#contained), any other reference among the entries of the Bundle
// being validated.
type referenceResolver struct{}

// Resolve implements slicematch.Resolver.
func (referenceResolver) Resolve(ref string, scope slicematch.Scope) (map[string]any, bool) {
	if id, ok := strings.CutPrefix(ref, "#"); ok {
		return constraint.ContainedByID(scope.RootResource, id)
	}
	return constraint.ResolveInBundle(scope.Container, ref)
}

// memberChecker answers ValueSet membership with the binding phase itself: the value is checked
// against a required binding to the ValueSet, with a fresh result. An error means it is not a
// member; a notice that the check could not be made means membership is unknown.
type memberChecker struct{ v *Validator }

// undecided are the binding diagnostics that mean membership could not be determined.
var undecided = map[issue.DiagnosticID]bool{
	issue.DiagBindingCannotValidate:   true,
	issue.DiagBindingValueSetNotFound: true,
	issue.DiagBindingUnresolved:       true,
	issue.DiagCodeSystemNotFound:      true,
}

// InValueSet implements slicematch.MemberChecker.
func (m memberChecker) InValueSet(ctx context.Context, valueSetURL string, value any) slicematch.Membership {
	if m.v.config.NoTerminology || m.v.bindValidator == nil {
		return slicematch.MembershipUnknown
	}
	result := issue.NewResult()
	binding := &registry.Binding{Strength: "required", ValueSet: valueSetURL}
	m.v.bindValidator.ValidateValueBinding(terminology.WithStrictMembership(ctx), value, binding, "", result)
	for _, is := range result.Issues {
		if undecided[issue.DiagnosticID(is.MessageID)] {
			return slicematch.MembershipUnknown
		}
	}
	if result.ErrorCount() > 0 {
		return slicematch.MembershipOut
	}
	return slicematch.MembershipIn
}
