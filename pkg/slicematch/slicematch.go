// Package slicematch decides which slice of a sliced element governs one instance of that
// element, following the slicing rules of the FHIR specification
// (profiling.html#discriminator).
//
// Everything is derived from the StructureDefinitions: the discriminator path is walked through
// the element tree ([registry.ElementTree]), into type profiles and, through resolve(), into the
// referenced resource. Types come from ElementDefinition.type and from the JSON property a value
// was read from, never from the shape of the value. The only names the package knows are the
// spec's grammar: the discriminator types, the path functions and the FHIR JSON property
// resourceType.
package slicematch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofhir/validator/pkg/jsoncompare"
	"github.com/gofhir/validator/pkg/registry"
)

// Scope carries the resources a value sits in. Constraints read the first two as %resource and
// %rootResource (fhirpath.html#variables): the resource the value is in, and the resource that
// contains it when it is a contained resource, else the same one. Container is where references
// resolve: the Bundle, or the resource, being validated.
type Scope struct {
	Resource     map[string]any
	RootResource map[string]any
	Container    map[string]any
}

// Conformer answers whether value conforms to profile (discriminator type "profile"). The
// implementation runs the full validation pipeline on the value, with a fresh result.
type Conformer interface {
	Conforms(ctx context.Context, value any, profile *registry.StructureDefinition, scope Scope) bool
}

// Resolver follows a reference made from within scope (discriminator function resolve()): a
// contained resource of the resource that makes it, or an entry of the Bundle being validated.
type Resolver interface {
	Resolve(ref string, scope Scope) (map[string]any, bool)
}

// Membership is the answer to a ValueSet membership question.
type Membership int

const (
	// MembershipUnknown means membership could not be determined, e.g. without terminology.
	MembershipUnknown Membership = iota
	// MembershipIn means the value is in the ValueSet.
	MembershipIn
	// MembershipOut means the value is not in the ValueSet.
	MembershipOut
)

// MemberChecker answers ValueSet membership, for discriminator values given by a required
// binding.
type MemberChecker interface {
	InValueSet(ctx context.Context, valueSetURL string, value any) Membership
}

// NoteKind classifies a note.
type NoteKind int

const (
	// NoteCannotEvaluate means a discriminator could not be evaluated for a slice, which then
	// does not match: an unresolvable profile, no value source, an unsupported discriminator.
	NoteCannotEvaluate NoteKind = iota
	// NoteMembershipUnknown means a required-binding discriminator could not be decided because
	// ValueSet membership is unknown (decision D-6); the slice does not match.
	NoteMembershipUnknown
)

// Note explains why a slice could not be decided for an instance.
type Note struct {
	Kind    NoteKind
	Slice   *registry.ElementNode
	Message string
}

// Match is the outcome of resolving one instance.
type Match struct {
	// Node governs the instance: the first matching slice (or reslice), or the sliced element
	// itself when no slice matches.
	Node *registry.ElementNode
	// Matched reports whether Node is a slice that matched.
	Matched bool
	// AlsoMatch are the other slices at the same level that also matched (decision D-1).
	AlsoMatch []*registry.ElementNode
	// Notes explain slices that could not be decided.
	Notes []Note
}

// Matcher resolves instances to slices. It is safe for concurrent use.
type Matcher struct {
	reg       *registry.Registry
	conformer Conformer
	members   MemberChecker
}

// Option configures a Matcher.
type Option func(*Matcher)

// WithConformer sets the Conformer for "profile" discriminators. Without one, a profile
// discriminator cannot be evaluated.
func WithConformer(c Conformer) Option { return func(m *Matcher) { m.conformer = c } }

// WithMemberChecker sets the MemberChecker for values given by required bindings. Without one,
// membership is unknown.
func WithMemberChecker(c MemberChecker) Option { return func(m *Matcher) { m.members = c } }

// New returns a Matcher over the registry.
func New(reg *registry.Registry, opts ...Option) *Matcher {
	m := &Matcher{reg: reg}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Request is one instance to resolve.
type Request struct {
	// SD is the StructureDefinition whose tree Node belongs to.
	SD *registry.StructureDefinition
	// Node is the sliced element (its Def declares slicing).
	Node *registry.ElementNode
	// Key is the JSON property the value was read from; it decides the type of a choice element.
	Key string
	// Value is the instance, one item of the element.
	Value any
	// Scope holds the resources the value sits in.
	Scope Scope
	// Resolver follows references for resolve(); it depends on the resource being validated.
	Resolver Resolver
	// Containment reports whether a resource is contained in another, for %rootResource.
	Containment func(container, resource map[string]any) bool
}

// Resolve returns the node that governs one instance of a sliced element.
func (m *Matcher) Resolve(ctx context.Context, req Request) Match {
	match := Match{Node: req.Node}
	if req.Node == nil || req.Node.Def.Slicing == nil {
		return match
	}
	var matched []*registry.ElementNode
	for _, slice := range req.Node.Slices {
		ok, notes := m.sliceMatches(ctx, req, slice)
		match.Notes = append(match.Notes, notes...)
		if ok {
			matched = append(matched, slice)
		}
	}
	if len(matched) == 0 {
		return match
	}
	match.Node, match.Matched, match.AlsoMatch = matched[0], true, matched[1:]

	// A matched slice that is itself sliced is resolved again among its reslices.
	if match.Node.Def.Slicing != nil && len(match.Node.Slices) > 0 {
		sub := req
		sub.Node = match.Node
		inner := m.Resolve(ctx, sub)
		match.Notes = append(match.Notes, inner.Notes...)
		if inner.Matched {
			match.Node = inner.Node
			match.AlsoMatch = append(match.AlsoMatch, inner.AlsoMatch...)
		}
	}
	return match
}

// sliceMatches reports whether the instance matches every discriminator of the slicing, for one
// slice.
func (m *Matcher) sliceMatches(ctx context.Context, req Request, slice *registry.ElementNode) (bool, []Note) {
	var notes []Note
	constrained := false
	for _, d := range req.Node.Def.Slicing.Discriminator {
		v := m.discriminatorMatches(ctx, req, slice, d)
		if v.note != nil {
			notes = append(notes, *v.note)
		}
		if !v.ok {
			return false, notes
		}
		constrained = constrained || v.constrained
	}
	// A slice that none of the discriminators constrains cannot be told apart from the others;
	// the HL7 validator reports it as slicing that cannot be evaluated ("Could not match
	// discriminator for slice") rather than assigning every element to it.
	if !constrained {
		return false, append(notes, Note{Kind: NoteCannotEvaluate, Slice: slice,
			Message: fmt.Sprintf("%s: no discriminator constrains this slice", slice.Def.ID)})
	}
	return true, notes
}

// verdict is one discriminator's answer for one slice: whether the instance matches, whether the
// slice constrains the discriminated element at all, and why it could not be decided.
type verdict struct {
	ok          bool
	constrained bool
	note        *Note
}

// Discriminator types (ElementDefinition.slicing.discriminator.type).
const (
	discValue   = "value"
	discPattern = "pattern"
	discExists  = "exists"
	discType    = "type"
	discProfile = "profile"
)

func (m *Matcher) discriminatorMatches(ctx context.Context, req Request, slice *registry.ElementNode, d registry.Discriminator) verdict {
	cannot := func(format string, args ...any) verdict {
		return verdict{constrained: true, note: &Note{Kind: NoteCannotEvaluate, Slice: slice, Message: fmt.Sprintf(format, args...)}}
	}
	steps, err := parsePath(d.Path)
	if err != nil {
		return cannot("discriminator path %q: %v", d.Path, err)
	}

	start := cursor{sd: req.SD, node: slice, key: req.Key}
	w := walker{m: m, resolver: req.Resolver, scope: req.Scope}
	ends, err := w.walk(start, []any{req.Value}, steps)
	if err != nil {
		return cannot("%s: %v", slice.Def.ID, err)
	}

	// Each alternative (a type with several profiles) is decided on its own; the instance matches
	// when it matches any of them.
	var out verdict
	for _, group := range byBranch(ends) {
		v := m.decide(ctx, req, slice, group, d)
		if v.ok {
			return v
		}
		out.constrained = out.constrained || v.constrained
		if out.note == nil {
			out.note = v.note
		}
	}
	return out
}

// decide evaluates one discriminator on one alternative. A slice that prohibits the element at the
// path (max 0) requires it to be absent, whatever the discriminator type.
func (m *Matcher) decide(ctx context.Context, req Request, slice *registry.ElementNode, group []end, d registry.Discriminator) verdict {
	present := len(instanceValues(group)) > 0
	for _, e := range group {
		if e.def != nil && e.def.Def.Max == "0" {
			return verdict{ok: !present, constrained: true}
		}
	}
	switch d.Type {
	case discValue, discPattern:
		return m.valueMatches(ctx, slice, group)
	case discExists:
		return existsMatches(slice, group)
	case discType:
		if len(allowedTypes(group)) == 0 {
			return verdict{ok: true}
		}
		return verdict{ok: m.typeMatches(group), constrained: true}
	case discProfile:
		return m.profileMatches(ctx, req, slice, group)
	}
	return verdict{constrained: true, note: &Note{Kind: NoteCannotEvaluate, Slice: slice,
		Message: fmt.Sprintf("discriminator type %q is not supported", d.Type)}}
}

// allowedTypes returns the types the slice allows where the walk ended.
func allowedTypes(group []end) []registry.Type {
	for _, e := range group {
		if e.def != nil && len(e.allowed) > 0 {
			return e.allowed
		}
	}
	return nil
}

// byBranch groups ends by alternative, keeping the order they were reached in.
func byBranch(ends []end) [][]end {
	idx := map[string]int{}
	var groups [][]end
	for _, e := range ends {
		i, ok := idx[e.branch]
		if !ok {
			i = len(groups)
			idx[e.branch] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], e)
	}
	return groups
}

// valueMatches decides a value (or pattern) discriminator. The expected value comes, in the order
// the spec lists them, from a fixed value, a pattern, or a required binding on the slice's
// definition of the discriminated element; a fixed or pattern value may also sit on an ancestor
// that contains the rest of the path.
func (m *Matcher) valueMatches(ctx context.Context, slice *registry.ElementNode, ends []end) verdict {
	src, ok := findValueSource(ends)
	if !ok {
		// A slice that fixes no value for this discriminator is not constrained by it; another
		// discriminator must tell the slices apart (AU Core Observation.category:specificDiscipline
		// fixes coding.system but not coding.code). When none does, sliceMatches reports it.
		return verdict{ok: true}
	}
	actual := instanceValues(ends)
	if src.kind != sourceBinding {
		return verdict{ok: satisfiesAll(actual, src.expect), constrained: true}
	}
	// Required binding: the discriminated element is one value, which must be in the ValueSet
	// (FHIRPath memberOf takes a single item; the HL7 validator matches nothing otherwise).
	if len(actual) != 1 {
		return verdict{constrained: true}
	}
	if m.members == nil {
		return verdict{constrained: true, note: membershipUnknown(slice, src.valueSet)}
	}
	var v any
	_ = json.Unmarshal(actual[0], &v)
	switch m.members.InValueSet(ctx, src.valueSet, v) {
	case MembershipIn:
		return verdict{ok: true, constrained: true}
	case MembershipUnknown:
		return verdict{constrained: true, note: membershipUnknown(slice, src.valueSet)}
	}
	return verdict{constrained: true}
}

func membershipUnknown(slice *registry.ElementNode, vs string) *Note {
	return &Note{Kind: NoteMembershipUnknown, Slice: slice,
		Message: fmt.Sprintf("%s: membership in %s could not be determined", slice.Def.ID, vs)}
}

// satisfiesAll reports whether every expectation is met by some actual value: equal to a fixed
// value, or containing a pattern.
func satisfiesAll(actual []json.RawMessage, expect []expectation) bool {
	if len(expect) == 0 || len(actual) == 0 {
		return false
	}
	for _, e := range expect {
		eq := jsoncompare.DeepEqual
		if e.pattern {
			eq = jsoncompare.ContainsPattern
		}
		found := false
		for _, a := range actual {
			if eq(a, e.value) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// existsMatches decides an exists discriminator where the slice requires the element (min > 0);
// a prohibited element (max 0) is decided before this. A slice that does neither cannot be
// evaluated, as the HL7 validator reports: the discriminator is based on element existence, but
// the slice neither sets min>=1 nor max=0.
func existsMatches(slice *registry.ElementNode, ends []end) verdict {
	present := len(instanceValues(ends)) > 0
	for _, e := range ends {
		if e.def != nil && e.def.Def.Min > 0 {
			return verdict{ok: present, constrained: true}
		}
	}
	return verdict{constrained: true, note: &Note{Kind: NoteCannotEvaluate, Slice: slice,
		Message: fmt.Sprintf("%s: an exists discriminator, but the slice neither requires nor prohibits the element", slice.Def.ID)}}
}

// typeMatches decides a type discriminator: the instance's type must be one of the types the
// slice allows at the path, or derive from one (FHIRPath "is", as the HL7 validator tests it).
func (m *Matcher) typeMatches(ends []end) bool {
	seen := false
	for _, e := range ends {
		if e.value == nil {
			continue
		}
		seen = true
		if len(e.allowed) == 0 {
			return false
		}
		ok := false
		for _, t := range e.allowed {
			if t.Code == e.typeCode || m.reg.IsSubtype(e.typeCode, t.Code) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return seen
}

// profileMatches decides a profile discriminator: the instance must conform to one of the
// profiles the slice declares at the path (type.profile, or type.targetProfile after resolve()).
func (m *Matcher) profileMatches(ctx context.Context, req Request, slice *registry.ElementNode, ends []end) verdict {
	declared := false
	for _, e := range ends {
		declared = declared || len(e.profiles) > 0
	}
	if !declared {
		// A slice that declares no profile at the path does not constrain it; another
		// discriminator (typically type) tells the slices apart.
		return verdict{ok: true}
	}
	ok, note := m.profileConforms(ctx, req, slice, ends)
	return verdict{ok: ok, constrained: true, note: note}
}

func (m *Matcher) profileConforms(ctx context.Context, req Request, slice *registry.ElementNode, ends []end) (bool, *Note) {
	seen := false
	for _, e := range ends {
		if e.value == nil {
			continue
		}
		seen = true
		profiles := e.profiles
		if len(profiles) == 0 {
			continue
		}
		if m.conformer == nil {
			return false, &Note{Kind: NoteCannotEvaluate, Slice: slice,
				Message: fmt.Sprintf("%s: profile conformance cannot be checked", slice.Def.ID)}
		}
		conforms := false
		for _, p := range profiles {
			psd, res := m.reg.ResolveCanonical(p)
			if psd == nil {
				return false, &Note{Kind: NoteCannotEvaluate, Slice: slice,
					Message: fmt.Sprintf("profile %s on %s could not be resolved (%s)", p, slice.Def.ID, res)}
			}
			// A resource value is its own %resource. Its %rootResource is the resource containing
			// it when it is contained there, else itself: a Bundle entry is not contained
			// (fhirpath.html#variables). References still resolve in the same container. A
			// datatype value keeps the resources it sits in.
			scope := req.Scope
			if res, _ := e.value.(map[string]any); res != nil {
				if _, isResource := res[resourceTypeKey]; isResource {
					root := res
					if req.Containment != nil && req.Containment(req.Scope.RootResource, res) {
						root = req.Scope.RootResource
					}
					scope = Scope{Resource: res, RootResource: root, Container: req.Scope.Container}
				}
			}
			if m.conformer.Conforms(ctx, e.value, psd, scope) {
				conforms = true
				break
			}
		}
		if !conforms {
			return false, nil
		}
	}
	return seen, nil
}

// resourceTypeKey is the FHIR JSON property that names a resource's type (json.html#resources).
const resourceTypeKey = "resourceType"

// instanceValues returns the instance values reached by a walk, as JSON.
func instanceValues(ends []end) []json.RawMessage {
	var out []json.RawMessage
	for _, e := range ends {
		if e.value == nil {
			continue
		}
		if b, err := json.Marshal(e.value); err == nil {
			out = append(out, b)
		}
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
