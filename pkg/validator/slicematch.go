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
	"github.com/gofhir/validator/v2/internal/scopeindex"
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
	// profileOnly judges the value on what the profile adds, as the HL7 validator judges each of the
	// profiles an element's type declares: the shape and primitive content the value has whatever the
	// profile, and a UCUM code, are the value's own and are reported once (the structural, primitive
	// and UCUM phases), and do not make it conform to none (typeChooser).
	profileOnly bool
	// at is where the value is, when it is a datatype's value chosen among the profiles of its element
	// (typeChooser); nil otherwise.
	at *datatypeAt
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
	value   uintptr // identity of the value's map within this validation; one map has one Scope, so the key needs none
	// pos is the identity of a primitive, which has no map: its place (registry.PrimitiveValue); the zero
	// Position for a value that has one (value is then its identity).
	pos    registry.Position
	assume bool // the check's mode: a cycle's answer differs between them
	// profileOnly is the other mode (conformer.profileOnly): its answer differs too.
	profileOnly bool
}

// datatypeAt is where the value a conformance check decides on is, when it is a datatype's value
// among the alternatives its element declares: the extensions it holds must be allowed there
// (extension.DatatypeAt).
type datatypeAt struct {
	alt   registry.Alternatives
	owner map[string]any // the resource that holds the value
	scope slicematch.Scope
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

// scopeCheck recibe, antes de leer el memo, el Scope con que se pide la conformidad de un recurso y
// el que el índice de posiciones tiene para él. Solo los tests lo fijan en un contexto
// (withScopeCheck), de modo que pertenece a la validación que lleva ese contexto.
type scopeCheck func(resource map[string]any, asked, indexed slicematch.Scope)

type scopeCheckKey struct{}

// typeChooser decides which of the profiles an element's type declares a value conforms to
// (registry.ProfileChooser), with the conformer, in the Scope of the resource that holds it.
type typeChooser struct{ v *Validator }

// probeKey y conformProbe cuentan, solo en los tests y los benchmarks, cuántas veces se pide una
// elección entre perfiles de tipo ("visit") y cuántas comprobaciones de conformidad corren de verdad
// ("check"), sin contar las respuestas de memo.
type conformProbe func(event string)

type probeKey struct{}

func probe(ctx context.Context, event string) {
	if p, _ := ctx.Value(probeKey{}).(conformProbe); p != nil {
		p(event)
	}
}

// Conforms implements registry.ProfileChooser. The owner's Scope is the canonical one of its resource
// (scopeindex), or, with no owner, the one the validation gives the value it validates.
func (c typeChooser) Conforms(ctx context.Context, inst registry.Instance, alt registry.Alternatives, profile *registry.StructureDefinition) (met, decided bool) {
	probe(ctx, "visit")
	scope, ok := c.scopeOf(ctx, inst.Owner)
	if !ok {
		return false, false
	}
	// El valor de un tipo de datos está en un solo lugar, el que da su mapa: el sitio que se le pasa
	// a la comprobación (para el contexto de uso de las Extension) no entra en la clave del memo.
	at := &datatypeAt{alt: alt, owner: inst.Owner, scope: scope}
	var value any = inst.Value
	if inst.Primitive.Valid() {
		value = inst.Primitive
	}
	return conformer{v: c.v, profileOnly: true, at: at}.Conforms(ctx, value, profile, scope), true
}

func (c typeChooser) scopeOf(ctx context.Context, owner map[string]any) (slicematch.Scope, bool) {
	root, hasRoot := ctx.Value(rootScopeKey{}).(slicematch.Scope)
	switch {
	case owner == nil:
		return root, hasRoot
	case scopeindex.From(ctx) != nil:
		return scopeindex.Resolve(ctx, owner, "type profile choice", slicematch.Scope{})
	case hasRoot && root.Resource != nil && sameMap(root.Resource, owner):
		return root, true
	}
	return slicematch.Scope{Resource: owner, RootResource: owner, Container: owner}, true
}

// Conforms implements slicematch.Conformer. A primitive comes as a registry.PrimitiveValue, and is
// identified by its place.
func (c conformer) Conforms(ctx context.Context, value any, profile *registry.StructureDefinition, scope slicematch.Scope) bool {
	if profile == nil {
		return false
	}
	key := conformKey{profile: profile.URL + "|" + profile.Version, assume: c.assume, profileOnly: c.profileOnly}
	data, prim, ok := identify(ctx, value, scope, &key)
	if !ok {
		return false
	}
	st, _ := ctx.Value(conformStateKey{}).(*conformState)
	if st == nil {
		ctx = withConformState(ctx)
		st, _ = ctx.Value(conformStateKey{}).(*conformState)
	}

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

	probe(ctx, "check")
	var ans bool
	if prim.Valid() {
		ans = c.checkPrimitive(ctx, prim, profile, scope)
	} else {
		ans = c.check(ctx, data, profile, scope)
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if scopeindex.Aborted(ctx) {
		// La validación está abortada (Validate): esta respuesta se apoya en un recurso pedido sin
		// Scope, así que no se guarda, ni las respuestas dadas dentro de ella.
		st.stack = st.stack[:depth]
		delete(st.running, key)
		return false
	}
	st.settle(key, depth, ans)
	return ans
}

// identify sets in key the identity of value, the subject of a conformance check, and returns it as a map or a
// primitive; false when there is no check to make.
func identify(ctx context.Context, value any, scope slicematch.Scope, key *conformKey) (data map[string]any, prim registry.PrimitiveValue, ok bool) {
	switch x := value.(type) {
	case map[string]any:
		// Un map, un Scope: el de un recurso es el que le da su posición, quienquiera que pregunte
		// (scopeindex), y el memo se apoya en eso; el que llega lo confirma el test del invariante. Un valor
		// que no es recurso está en el Scope de su recurso, el que da quien llama. Un recurso que el índice no tiene aborta la validación
		// (Validate): nada se comprueba ni se guarda para él.
		if _, isResource := x[resourceTypeKey].(string); isResource {
			indexed, found := scopeindex.Resolve(ctx, x, "Conforms", scope)
			if !found {
				return nil, prim, false
			}
			if check, _ := ctx.Value(scopeCheckKey{}).(scopeCheck); check != nil {
				check(x, scope, indexed)
			}
		} else if scopeindex.Aborted(ctx) {
			return nil, prim, false
		}
		key.value = reflect.ValueOf(x).Pointer()
		return x, prim, true
	case registry.PrimitiveValue:
		// Un escalar no tiene identidad: su lugar la da (el map que lo contiene, la propiedad y el índice). El
		// Scope es el del recurso que lo contiene, que da quien llama.
		if !x.Valid() || scopeindex.Aborted(ctx) {
			return nil, prim, false
		}
		key.pos = x.Position()
		return nil, x, true
	}
	return nil, prim, false
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
	// Una corrida especulativa no se memoiza en los grupos de los holders: se armaron con
	// respuestas que pueden ser provisionales, y settle las descarta sin descartar los grupos
	// (holderGroups).
	ctx = withoutHolderGroups(ctx)
	c.v.validateAgainstProfile(withExtensionScope(constraint.WithReportScope(ctx)), data, raw, profile, &valueScope{scope: scope, state: st, profileOnly: c.profileOnly, at: c.at}, result)
	return result.ErrorCount() == 0
}

// checkPrimitive decides whether a primitive conforms to profile, a definition of its type: the pipeline runs on
// its "_key" sibling (an empty object when it has none), which is an instance of the profile's children (id and
// extension), and the constraints of the profile's root run on the scalar with that sibling
// (valueScope.primitive). What the profile declares for the value itself is not applied (piece 3b).
func (c conformer) checkPrimitive(ctx context.Context, prim registry.PrimitiveValue, profile *registry.StructureDefinition, scope slicematch.Scope) bool {
	if prim.TypeCode != "" && prim.TypeCode != profile.Type {
		return false
	}
	if err := c.v.registry.EnsureSnapshot(ctx, profile); err != nil {
		return false
	}
	data := prim.Ext
	if data == nil {
		data = map[string]any{}
	}
	exact := exactOf(ctx, data)
	if exact == nil {
		exact = data
	}
	raw, err := json.Marshal(exact)
	if err != nil {
		return false
	}
	result := issue.NewResult()
	result.Stats = &issue.Stats{}
	st, _ := ctx.Value(conformStateKey{}).(*conformState)
	ctx = withoutHolderGroups(ctx)
	c.v.validateAgainstProfile(withExtensionScope(constraint.WithReportScope(ctx)), data, raw, profile, &valueScope{scope: scope, state: st, profileOnly: c.profileOnly, at: c.at, primitive: prim}, result)
	return result.ErrorCount() == 0
}

// valueScope marks a run of the pipeline on a value inside another resource (a conformance
// check), with the resources it sits in.
type valueScope struct {
	scope slicematch.Scope
	state *conformState
	// profileOnly is conformer.profileOnly.
	profileOnly bool
	// at is conformer.at.
	at *datatypeAt
	// primitive is the value when it is a primitive, whose "_key" sibling is the data of the run (checkPrimitive).
	primitive registry.PrimitiveValue
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
		if !ok {
			return nil, slicematch.Scope{}, false
		}
		// El contenido está donde está, no donde se hace la referencia.
		in, ok := scopeindex.Resolve(r.ctx, res, "ResolveScoped #id", slicematch.Scope{})
		return res, in, ok
	}
	bundles := append([]map[string]any{scope.Container}, scope.Outer...)
	// Se parte de la entry que contiene al recurso donde está la referencia.
	from := bundleref.IndexOf(r.ctx, scope.Container).FullURLOf(scope.Resource)
	for i, bundle := range bundles {
		res, ok, ambiguous := bundleref.IndexOf(r.ctx, bundle).Find(ref, from)
		if ok {
			// Es una entry de bundle. Los pasos siguientes de la ruta de FHIRPath siguen resolviendo en el
			// Bundle donde la encontró, no en el que ella misma sea: así lo hace HL7 (sonda r1b). Este Scope
			// es solo el de la cadena; al comprobar un perfil, valueScope lo canoniza con ScopeOf.
			return res, slicematch.Scope{Resource: res, RootResource: res, Container: bundle, Outer: bundles[i+1:]}, true
		}
		if ambiguous {
			break
		}
	}
	return nil, slicematch.Scope{}, false
}

// ScopeOf implements slicematch.ScopedResolver.
func (r referenceResolver) ScopeOf(scope slicematch.Scope, resource map[string]any) slicematch.Scope {
	if scopeindex.From(r.ctx) == nil {
		return constraint.ScopeOf(scope, resource) // una comprobación suelta, sin índice
	}
	in, _ := scopeindex.Resolve(r.ctx, resource, "ScopeOf", slicematch.Scope{})
	return in
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
func (m memberChecker) InValueSet(ctx context.Context, valueSetURL string, value any, typeCode string) slicematch.Membership {
	if m.v.config.NoTerminology || m.v.bindValidator == nil {
		return slicematch.MembershipUnknown
	}
	result := issue.NewResult()
	binding := &registry.Binding{Strength: "required", ValueSet: valueSetURL}
	m.v.bindValidator.ValidateValueBinding(terminology.WithStrictMembership(ctx), value, typeCode, binding, "", result)
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
