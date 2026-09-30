package validator

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"

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
		memo: map[conformKey]bool{}, running: map[conformKey]bool{},
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
	if profile.Snapshot == nil {
		if err := c.v.registry.EnsureSnapshot(ctx, profile); err != nil {
			return false
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return false
	}
	// A fresh result with its stats, which the phases update.
	result := issue.NewResult()
	result.Stats = &issue.Stats{}
	c.v.validateAgainstProfile(ctx, data, raw, profile, &valueScope{scope: scope}, result)
	return result.ErrorCount() == 0
}

// valueScope marks a run of the pipeline on a value inside another resource (a conformance
// check), with the resources it sits in.
type valueScope struct{ scope slicematch.Scope }

// constraintOptions returns the resources a value's constraints read as %resource and
// %rootResource.
func (s *valueScope) constraintOptions() *constraint.ValidateOptions {
	opts := &constraint.ValidateOptions{BundleData: s.scope.RootResource}
	if s.scope.Resource != nil {
		opts.Resource, _ = json.Marshal(s.scope.Resource)
	}
	if s.scope.RootResource != nil {
		opts.RootResource, _ = json.Marshal(s.scope.RootResource)
	}
	return opts
}

// referenceResolver follows references inside the resource being validated, for resolve().
type referenceResolver struct{ root map[string]any }

// Resolve implements slicematch.Resolver.
func (r referenceResolver) Resolve(ref string) (map[string]any, bool) {
	return constraint.ResolveReference(r.root, ref)
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
