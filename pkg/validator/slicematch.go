package validator

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"

	"github.com/gofhir/validator/v2/internal/bundleref"
	"github.com/gofhir/validator/v2/internal/fhirpathcache"
	"github.com/gofhir/validator/v2/pkg/constraint"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
	"github.com/gofhir/validator/v2/pkg/terminology"
)

// The three services the slice matcher needs, backed by this validator.

// conformer decides a "profile" discriminator by running the whole validation pipeline on the
// value against the profile, with a fresh result: the value conforms when that finds no error.
//
// A check met again while it runs is a cycle. For a discriminator it answers false: a profile whose
// discriminator requires conforming to itself decides nothing. A reference's target is checked
// assuming it (assume), and so is a discriminator met again through a target's check: resources
// that reference each other (Patient.link, hasMember) conform when nothing else is wrong with them,
// as the HL7 validator finds. An answer that relied on what a cycle answered is not kept while the
// check met again still runs, and is dropped if that check answers otherwise.
type conformer struct {
	v      *Validator
	assume bool
}

// conformState is shared by the conformance checks of one top-level validation: a memo of the
// answers, the checks in progress, which answer as conformer says when met again, and the answers
// that relied on that.
type conformState struct {
	mu   sync.Mutex
	memo map[conformKey]bool
	// running holds the checks in progress, by their depth in stack, the outermost first.
	running map[conformKey]int
	// checked counts the checks of each key made: an answer a discriminator swayed is kept at its
	// last (maxSwayedChecks).
	checked map[conformKey]int
	stack   []conformFrame
	// provisional holds the answers that relied on assuming a check still running: reused while
	// it runs, kept when it ends if what was assumed holds, dropped if it was assumed true and
	// fails. A false relies on it as a true does: a discriminator that matches because of it can put
	// a value in a slice it may not be in.
	provisional map[conformKey]provisionalAnswer
	// made numbers the provisional answers in the order they are made.
	made int
	// collections caches a resource's FHIRPath collection, for %resource and %rootResource: the
	// same Bundle is the scope of every check inside it.
	collections map[uintptr]fhirpath.Collection
	// exact returns a resource with its numbers as the JSON spells them (exactIn), or is nil.
	exact func(map[string]any) map[string]any
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
	written := m // as the JSON spells its numbers, when the validation read it
	if st.exact != nil {
		if e := st.exact(m); e != nil {
			written = e
		}
	}
	raw, err := json.Marshal(written)
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

// conformFrame is a check in progress: low is the depth of the outermost check its answer so far
// relies on having assumed (its own depth when none), assumed is set when a cycle assumed it true,
// made is the number of provisional answers made before it started, and swayed is set when its
// answer relies on a discriminator's that relied on an assumption; target is its key's mode, and
// refuted is set when a cycle answered it false, a discriminator met again within itself.
type conformFrame struct {
	low     int
	assumed bool
	refuted bool
	made    int
	swayed  bool
	target  bool
}

// throughTarget reports whether a target's check runs within the check at depth: a cycle back to
// it then goes through a reference.
func (st *conformState) throughTarget(depth int) bool {
	for _, f := range st.stack[depth+1:] {
		if f.target {
			return true
		}
	}
	return false
}

// provisionalAnswer is an answer that relied on assuming the check at depth low, and made numbers
// it; swayed as its check's frame was.
type provisionalAnswer struct {
	ans       bool
	low, made int
	swayed    bool
}

// relyOn records that the innermost running check relies on the check at depth low.
func (st *conformState) relyOn(low int) {
	if n := len(st.stack); n > 0 && low < st.stack[n-1].low {
		st.stack[n-1].low = low
	}
}

// maxSwayedChecks is how many times an answer a discriminator swayed is checked: each time a check
// it relied on fails it is dropped and checked again, which in a graph of such checks would take
// exponential time. Bounded, every key is checked at most this many times. The cycles HL7 answers
// depending on the order (tp_22 to tp_25) need the answer checked again once, when what it assumed
// fails; the third check lets a second failure be taken into account too. Past the bound the answer
// is kept as it is, which in dense cycles of discriminators may not be a fixed point.
const maxSwayedChecks = 3

type conformKey struct {
	profile string
	value   uintptr // identity of the value's map within this validation
	assume  bool    // the check's mode: a cycle's answer differs between them
}

type conformStateKey struct{}

func withConformState(ctx context.Context) context.Context {
	if _, ok := ctx.Value(conformStateKey{}).(*conformState); ok {
		return ctx
	}
	return context.WithValue(ctx, conformStateKey{}, &conformState{
		memo: map[conformKey]bool{}, running: map[conformKey]int{}, checked: map[conformKey]int{}, collections: map[uintptr]fhirpath.Collection{},
		provisional: map[conformKey]provisionalAnswer{},
		exact:       exactIn(ctx),
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
	key := conformKey{profile: profile.URL + "|" + profile.Version, value: reflect.ValueOf(data).Pointer(), assume: c.assume}

	st.mu.Lock()
	if ans, ok := st.memo[key]; ok {
		st.mu.Unlock()
		return ans
	}
	if p, ok := st.provisional[key]; ok {
		// Right as long as what it assumed holds: what relies on it assumes the same.
		st.relyOn(p.low)
		if p.swayed {
			st.stack[len(st.stack)-1].swayed = true
		}
		st.mu.Unlock()
		return p.ans
	}
	if depth, ok := st.running[key]; ok {
		// A target's check met again is assumed. So is a discriminator's met again through a
		// target's check, the cycle going through a reference: its answer is the target's. A
		// discriminator's met again within itself decides nothing: false.
		assume := c.assume || st.throughTarget(depth)
		if assume {
			st.stack[depth].assumed = true
		} else {
			st.stack[depth].refuted = true
		}
		st.relyOn(depth)
		if !c.assume {
			// A discriminator's answer that relies on what a cycle answered.
			st.stack[len(st.stack)-1].swayed = true
		}
		st.mu.Unlock()
		return assume
	}
	depth := len(st.stack)
	st.running[key] = depth
	st.checked[key]++
	st.stack = append(st.stack, conformFrame{low: depth, made: st.made, target: c.assume})
	st.mu.Unlock()

	ans := c.check(ctx, data, profile, scope)

	st.mu.Lock()
	defer st.mu.Unlock()
	st.settle(key, depth, ans)
	return ans
}

// settle ends the check of key at depth, which answered ans: it keeps the answer, and those made
// within it, when what they relied on holds, and drops them when a check assumed true fails.
func (st *conformState) settle(key conformKey, depth int, ans bool) {
	frame := st.stack[depth]
	st.stack = st.stack[:depth]
	delete(st.running, key)
	// What a cycle answered for this check is not what it answers: what relied on that is wrong.
	failed := (frame.assumed && !ans) || (frame.refuted && ans)
	for k, p := range st.provisional {
		switch {
		case p.made <= frame.made:
			// Made before this check: it does not rely on it.
		case failed:
			// It may have relied on this check conforming, which it does not: undecided again.
			delete(st.provisional, k)
		case p.low >= depth && frame.low >= depth:
			// What it relied on was assumed within this check, which relied on nothing outside
			// it: it holds.
			st.memo[k] = p.ans
			delete(st.provisional, k)
		case p.low >= depth:
			// It relies on this check, which relies on one outside it.
			p.low = frame.low
			st.provisional[k] = p
		}
	}
	// A discriminator's answer that relied on an assumption can turn either way when it fails: a
	// value that conforms may fall in a slice it may not be in. Without one, a target's check only
	// conforms less when an assumed target turns out not to conform, so a false that relied on
	// assumptions holds whatever they turn out to be.
	swayed := frame.swayed || (!key.assume && frame.low < depth)
	if frame.low >= depth || (!ans && !swayed) || (swayed && st.checked[key] >= maxSwayedChecks) {
		// It relied on no check outside it, or nothing outside it can make it conform; or a
		// discriminator swayed it, and checking it again would not bound the work.
		st.memo[key] = ans
	} else {
		st.made++
		st.provisional[key] = provisionalAnswer{ans: ans, low: frame.low, made: st.made, swayed: swayed}
		st.relyOn(frame.low)
		if swayed {
			st.stack[depth-1].swayed = true
		}
	}
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
	opts := &constraint.ValidateOptions{BundleData: s.scope.Container, OuterBundles: s.scope.Outer, Scope: &s.scope}
	if s.state != nil {
		opts.Resource = s.state.collection(s.scope.Resource)
		opts.RootResource = s.state.collection(s.scope.RootResource)
	}
	return opts
}

// referenceResolver follows references for resolve(): "#id" among the contained resources of
// the resource that makes the reference (its %rootResource, which a contained resource shares with
// its container; references.html#contained), any other reference among the entries of the Bundle
// the resource is in, then of the Bundles that hold it, as bundle.html#references resolves it from
// the entry the %rootResource is (bundleref).
type referenceResolver struct {
	ctx context.Context // the validation's Bundle indexes (bundleref.IndexOf)
}

// Resolve implements slicematch.Resolver.
func (r referenceResolver) Resolve(ref string, scope slicematch.Scope) (map[string]any, bool) {
	res, _, ok := r.ResolveScoped(ref, scope)
	return res, ok
}

// ResolveScoped implements slicematch.ScopedResolver.
func (r referenceResolver) ResolveScoped(ref string, scope slicematch.Scope) (map[string]any, slicematch.Scope, bool) {
	if id, ok := strings.CutPrefix(ref, "#"); ok {
		res, ok := constraint.ContainedByID(scope.RootResource, id)
		return res, slicematch.Scope{Resource: res, RootResource: scope.RootResource, Container: scope.Container, Outer: scope.Outer}, ok
	}
	bundles := append([]map[string]any{scope.Container}, scope.Outer...)
	// Made from the entry that holds the resource the reference is in.
	from := bundleref.IndexOf(r.ctx, scope.Container).FullURLOf(scope.Resource)
	for i, bundle := range bundles {
		res, ok, ambiguous := bundleref.IndexOf(r.ctx, bundle).Find(ref, from)
		if ok {
			// It is an entry of bundle, where its own references resolve.
			return res, slicematch.Scope{Resource: res, RootResource: res, Container: bundle, Outer: bundles[i+1:]}, true
		}
		if ambiguous {
			break
		}
	}
	return nil, slicematch.Scope{}, false
}

// ScopeOf implements slicematch.ScopedResolver.
func (referenceResolver) ScopeOf(scope slicematch.Scope, resource map[string]any) slicematch.Scope {
	return constraint.ScopeOf(scope, resource)
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

// sameMap reports whether a and b are the same map.
func sameMap(a, b map[string]any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
