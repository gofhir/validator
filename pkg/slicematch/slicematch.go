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

// Scope carries the resources a value sits in. Constraints read them as %resource and
// %rootResource, and a datatype or extension value has none of its own.
type Scope struct {
	Resource     map[string]any
	RootResource map[string]any
}

// Conformer answers whether value conforms to profile (discriminator type "profile"). The
// implementation runs the full validation pipeline on the value, with a fresh result.
type Conformer interface {
	Conforms(ctx context.Context, value any, profile *registry.StructureDefinition, scope Scope) bool
}

// Resolver follows a reference inside the resource being validated: a Bundle entry or a
// contained resource (discriminator function resolve()).
type Resolver interface {
	Resolve(ref string) (map[string]any, bool)
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
	for _, d := range req.Node.Def.Slicing.Discriminator {
		ok, note := m.discriminatorMatches(ctx, req, slice, d)
		if note != nil {
			notes = append(notes, *note)
		}
		if !ok {
			return false, notes
		}
	}
	return true, notes
}

// Discriminator types (ElementDefinition.slicing.discriminator.type).
const (
	discValue   = "value"
	discPattern = "pattern"
	discExists  = "exists"
	discType    = "type"
	discProfile = "profile"
)

func (m *Matcher) discriminatorMatches(ctx context.Context, req Request, slice *registry.ElementNode, d registry.Discriminator) (bool, *Note) {
	cannot := func(format string, args ...any) (bool, *Note) {
		return false, &Note{Kind: NoteCannotEvaluate, Slice: slice, Message: fmt.Sprintf(format, args...)}
	}
	steps, err := parsePath(d.Path)
	if err != nil {
		return cannot("discriminator path %q: %v", d.Path, err)
	}

	start := cursor{sd: req.SD, node: slice, key: req.Key}
	w := walker{m: m, resolver: req.Resolver}
	ends, err := w.walk(start, []any{req.Value}, steps)
	if err != nil {
		return cannot("%s: %v", slice.Def.ID, err)
	}

	// Each alternative (a type with several profiles) is decided on its own; the instance matches
	// when it matches any of them.
	var note *Note
	for _, group := range byBranch(ends) {
		var ok bool
		var n *Note
		switch d.Type {
		case discValue, discPattern:
			ok, n = m.valueMatches(ctx, slice, group)
		case discExists:
			ok = existsMatches(group)
		case discType:
			ok = typeMatches(group)
		case discProfile:
			ok, n = m.profileMatches(ctx, req, slice, group)
		default:
			return cannot("discriminator type %q is not supported", d.Type)
		}
		if ok {
			return true, nil
		}
		if note == nil {
			note = n
		}
	}
	return false, note
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
func (m *Matcher) valueMatches(ctx context.Context, slice *registry.ElementNode, ends []end) (bool, *Note) {
	src, ok := findValueSource(ends)
	if !ok {
		// A slice that fixes no value for this discriminator is not constrained by it; the other
		// discriminators tell the slices apart (as the HL7 validator does, e.g. AU Core
		// Observation.category:specificDiscipline, which fixes coding.system but not coding.code).
		return true, nil
	}
	actual := instanceValues(ends)
	if src.kind != sourceBinding {
		return satisfiesAll(actual, src.expect), nil
	}
	// Required binding: every value must be in the ValueSet.
	if len(actual) == 0 {
		return false, nil
	}
	if m.members == nil {
		return false, membershipUnknown(slice, src.valueSet)
	}
	unknown := false
	for _, a := range actual {
		var v any
		_ = json.Unmarshal(a, &v)
		switch m.members.InValueSet(ctx, src.valueSet, v) {
		case MembershipOut:
			return false, nil
		case MembershipUnknown:
			unknown = true
		}
	}
	if unknown {
		return false, membershipUnknown(slice, src.valueSet)
	}
	return true, nil
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

// existsMatches decides an exists discriminator: the slice requires the element (min > 0) or
// prohibits it (max 0). A slice that does neither does not discriminate.
func existsMatches(ends []end) bool {
	present := len(instanceValues(ends)) > 0
	for _, e := range ends {
		if e.def == nil {
			continue
		}
		switch {
		case e.def.Def.Max == "0":
			return !present
		case e.def.Def.Min > 0:
			return present
		}
	}
	return true
}

// typeMatches decides a type discriminator: the instance's type must be one of the types the
// slice allows at the path.
func typeMatches(ends []end) bool {
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
			if t.Code == e.typeCode {
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
func (m *Matcher) profileMatches(ctx context.Context, req Request, slice *registry.ElementNode, ends []end) (bool, *Note) {
	seen := false
	for _, e := range ends {
		if e.value == nil {
			continue
		}
		seen = true
		profiles := e.profiles
		if len(profiles) == 0 {
			// A slice that declares no profile at the path does not constrain it; another
			// discriminator (typically type) tells the slices apart.
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
			// A resource value (a Bundle entry's resource) is its own %resource and %rootResource:
			// %rootResource is the resource a contained resource sits in, and an entry is not
			// contained (fhirpath.html#variables). A datatype value keeps the resources it sits in.
			scope := req.Scope
			if res, _ := e.value.(map[string]any); res != nil {
				if _, isResource := res[resourceTypeKey]; isResource {
					scope = Scope{Resource: res, RootResource: res}
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
