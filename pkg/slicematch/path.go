package slicematch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gofhir/validator/pkg/registry"
)

// A discriminator path is "a FHIRPath expression that uses a restricted subset": element names
// separated by ".", the functions extension(url), resolve() and ofType(type), and $this
// (profiling.html#discriminator).

type stepKind int

const (
	stepName stepKind = iota
	stepExtension
	stepResolve
	stepOfType
)

type step struct {
	kind stepKind
	arg  string // element name, extension URL or type
}

// Names the path grammar and the functions it allows are defined in terms of.
const (
	thisKeyword = "$this"
	// The function extension(url) is ".extension.where(url = ...)" (fhirpath.html#functions,
	// FHIR's additional functions), so it names the extension element and its url.
	extensionElement = "extension"
	extensionURL     = "url"
	// The function resolve() follows a Reference through its reference element.
	referenceElement = "reference"
)

// parsePath splits a discriminator path into steps.
func parsePath(path string) ([]step, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("empty path")
	}
	var steps []step
	for _, seg := range splitSegments(path) {
		switch {
		case seg == thisKeyword:
			continue
		case strings.HasPrefix(seg, "extension(") && strings.HasSuffix(seg, ")"):
			url := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(seg, "extension("), ")"), `'" `)
			if url == "" {
				return nil, fmt.Errorf("extension() without a url in %q", path)
			}
			steps = append(steps, step{kind: stepExtension, arg: url})
		case seg == "resolve()":
			steps = append(steps, step{kind: stepResolve})
		case strings.HasPrefix(seg, "ofType(") && strings.HasSuffix(seg, ")"):
			t := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(seg, "ofType("), ")"))
			// A type may be qualified with its model (FHIR.Quantity).
			if i := strings.LastIndexByte(t, '.'); i >= 0 {
				t = t[i+1:]
			}
			if t == "" {
				return nil, fmt.Errorf("ofType() without a type in %q", path)
			}
			steps = append(steps, step{kind: stepOfType, arg: t})
		case strings.ContainsAny(seg, "()"):
			return nil, fmt.Errorf("function %q is not allowed in a discriminator path", seg)
		default:
			steps = append(steps, step{kind: stepName, arg: seg})
		}
	}
	return steps, nil
}

// splitSegments splits on "." outside parentheses and quotes.
func splitSegments(path string) []string {
	var out []string
	depth, quote, start := 0, byte(0), 0
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == '.' && depth == 0:
			out = append(out, strings.TrimSpace(path[start:i]))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(path[start:]))
}

// cursor is a position on the definition side: an element of a StructureDefinition's tree and
// the types it may have there.
type cursor struct {
	sd    *registry.StructureDefinition
	node  *registry.ElementNode
	key   string          // JSON property the instance was read from
	types []registry.Type // nil means node's own types
}

func (c cursor) allowed() []registry.Type {
	if c.types != nil {
		return c.types
	}
	if c.node == nil {
		return nil
	}
	return c.node.Def.Type
}

// frame is a definition met on the way, with the element names walked after it: a fixed or
// pattern value there may contain the discriminated element.
type frame struct {
	node  *registry.ElementNode
	after []string
}

// end is where a walk stopped, for one instance value (or for none, when the instance has no
// value there).
type end struct {
	branch   string // which alternative, when a type declares several profiles
	def      *registry.ElementNode
	allowed  []registry.Type
	value    any    // nil when the instance has no value at the path
	typeCode string // the instance value's type
	profiles []string
	frames   []frame
}

type walker struct {
	m        *Matcher
	resolver Resolver
	scope    Scope
}

type state struct {
	branch   string
	cur      cursor
	value    any // nil: no instance value, walking the definition only
	typeCode string
	frames   []frame
	resolved bool     // the last step was resolve()
	targets  []string // targetProfile of the reference just resolved
}

// walk follows steps from start, for each of the start values.
func (w walker) walk(start cursor, values []any, steps []step) ([]end, error) {
	var states []state
	for _, v := range values {
		states = append(states, state{cur: start, value: v, typeCode: singleTypeCode(start.allowed(), v)})
	}
	for _, s := range steps {
		next, err := w.advance(states, s)
		if err != nil {
			return nil, err
		}
		states = next
	}
	ends := make([]end, 0, len(states))
	for _, st := range states {
		e := end{branch: st.branch, def: st.cur.node, allowed: st.cur.allowed(), value: st.value, typeCode: st.typeCode, frames: st.frames}
		if st.resolved {
			e.profiles = st.targets
		} else {
			for _, t := range e.allowed {
				e.profiles = append(e.profiles, t.Profile...)
			}
		}
		ends = append(ends, e)
	}
	return ends, nil
}

// advance applies one step to every state. When no state keeps an instance value, one state
// continues on the definition side alone, so the slice's definition at the path is still known.
func (w walker) advance(states []state, s step) ([]state, error) {
	var out []state
	defOnly := map[string]state{}
	var order []string
	withValue := map[string]bool{}
	var lastErr error
	for _, st := range states {
		next, err := w.apply(st, s)
		if err != nil {
			// One alternative failing does not fail the others.
			lastErr = err
			continue
		}
		for _, n := range next {
			if n.value == nil {
				if _, seen := defOnly[n.branch]; !seen {
					defOnly[n.branch] = n
					order = append(order, n.branch)
				}
				continue
			}
			withValue[n.branch] = true
			out = append(out, n)
		}
	}
	for _, b := range order {
		if !withValue[b] {
			out = append(out, defOnly[b])
		}
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

func (w walker) apply(st state, s step) ([]state, error) {
	switch s.kind {
	case stepName:
		return w.name(st, s.arg)
	case stepExtension:
		return w.extension(st, s.arg)
	case stepResolve:
		return w.resolve(st)
	case stepOfType:
		return ofType(st, s.arg), nil
	}
	return nil, fmt.Errorf("unknown step")
}

// name selects a child element.
func (w walker) name(st state, name string) ([]state, error) {
	branches, err := w.m.children(st.cur, st.typeCode)
	if err != nil {
		return nil, err
	}
	var out []state
	var lastErr error
	for i, br := range branches {
		sub := st
		if len(branches) > 1 {
			sub.branch = fmt.Sprintf("%s/%d", st.branch, i)
		}
		next, err := w.nameIn(sub, br.sd, br.children, name)
		if err != nil {
			lastErr = err
			continue
		}
		out = append(out, next...)
	}
	if len(out) == 0 {
		return nil, lastErr
	}
	return out, nil
}

func (w walker) nameIn(st state, sd *registry.StructureDefinition, children []*registry.ElementNode, name string) ([]state, error) {
	var child *registry.ElementNode
	choice := false
	for _, c := range children {
		switch c.Name() {
		case name:
			child = c
		case name + "[x]":
			child, choice = c, true
		}
		if child != nil {
			break
		}
	}
	if child == nil {
		return nil, fmt.Errorf("%s has no element %q", describe(st.cur), name)
	}

	// Every definition met so far now has this name after it, the current one included.
	frames := make([]frame, 0, len(st.frames)+1)
	for _, f := range st.frames {
		frames = append(frames, frame{node: f.node, after: append(append([]string(nil), f.after...), name)})
	}
	frames = append(frames, frame{node: st.cur.node, after: []string{name}})

	obj, _ := st.value.(map[string]any)
	var out []state
	emit := func(key string, types []registry.Type) {
		for _, v := range items(obj, key) {
			out = append(out, state{branch: st.branch, cur: cursor{sd: sd, node: child, key: key, types: types},
				value: v, typeCode: singleTypeCode(types, v), frames: frames})
		}
	}
	if choice {
		// The JSON property of a choice element is its name followed by the type code with its
		// first letter capitalized (formats.html#choice).
		for _, t := range child.Def.Type {
			emit(name+capitalize(t.Code), []registry.Type{t})
		}
	} else {
		emit(name, nil)
	}
	if len(out) == 0 {
		out = append(out, state{branch: st.branch, cur: cursor{sd: sd, node: child}, frames: frames})
	}
	return out, nil
}

// extension selects the extensions with this URL, and on the definition side the slice of the
// extension element that declares that extension, or the extension's own definition.
func (w walker) extension(st state, url string) ([]state, error) {
	branches, err := w.m.children(st.cur, st.typeCode)
	if err != nil {
		return nil, err
	}
	var sd *registry.StructureDefinition
	var ext *registry.ElementNode
	for _, br := range branches {
		for _, c := range br.children {
			if c.Name() == extensionElement {
				sd, ext = br.sd, c
				break
			}
		}
		if ext != nil {
			break
		}
	}
	if ext == nil {
		return nil, fmt.Errorf("%s has no %s element", describe(st.cur), extensionElement)
	}
	defCur := cursor{sd: sd, node: ext}
	for _, s := range ext.Slices {
		for _, t := range s.Def.Type {
			for _, p := range t.Profile {
				if u, _ := registry.ParseCanonical(p); u == url {
					defCur = cursor{sd: sd, node: s}
				}
			}
		}
	}
	if defCur.node == ext {
		esd, res := w.m.reg.ResolveCanonical(url)
		if esd == nil {
			return nil, fmt.Errorf("extension %s could not be resolved (%s)", url, res)
		}
		tree, err := w.m.tree(esd)
		if err != nil {
			return nil, err
		}
		defCur = cursor{sd: esd, node: tree.Root()}
	}

	obj, _ := st.value.(map[string]any)
	var out []state
	for _, v := range items(obj, extensionElement) {
		if m, ok := v.(map[string]any); ok && m[extensionURL] == url {
			out = append(out, state{branch: st.branch, cur: defCur, value: v, typeCode: singleTypeCode(defCur.allowed(), v), frames: st.frames})
		}
	}
	if len(out) == 0 {
		out = append(out, state{branch: st.branch, cur: defCur, frames: st.frames})
	}
	return out, nil
}

// resolve follows a Reference to the resource it names; on the definition side it enters the
// resource type's (or the target profile's) definition.
func (w walker) resolve(st state) ([]state, error) {
	var targets []string
	for _, t := range st.cur.allowed() {
		targets = append(targets, t.TargetProfile...)
	}
	obj, _ := st.value.(map[string]any)
	ref, _ := obj[referenceElement].(string)
	if ref == "" || w.resolver == nil {
		return []state{{branch: st.branch, cur: cursor{}, resolved: true, targets: targets}}, nil
	}
	res, ok := w.resolver.Resolve(ref, w.scope)
	if !ok {
		return []state{{branch: st.branch, cur: cursor{}, resolved: true, targets: targets}}, nil
	}
	rt, _ := res[resourceTypeKey].(string)
	rsd := w.m.resourceDefinition(rt, targets)
	if rsd == nil {
		return nil, fmt.Errorf("no definition for resource type %q", rt)
	}
	tree, err := w.m.tree(rsd)
	if err != nil {
		return nil, err
	}
	// The types the reference allows are those of its target profiles.
	allowed := []registry.Type{}
	for _, tp := range targets {
		if tsd, _ := w.m.reg.ResolveCanonical(tp); tsd != nil {
			allowed = append(allowed, registry.Type{Code: tsd.Type, Profile: []string{tp}})
		}
	}
	return []state{{branch: st.branch, cur: cursor{sd: rsd, node: tree.Root(), types: allowed}, value: res, typeCode: rt, resolved: true, targets: targets}}, nil
}

// ofType keeps the values of one type, and restricts the definition to it.
func ofType(st state, t string) []state {
	var types []registry.Type
	for _, a := range st.cur.allowed() {
		if a.Code == t {
			types = append(types, a)
		}
	}
	cur := st.cur
	cur.types = types
	if cur.types == nil {
		cur.types = []registry.Type{}
	}
	if st.value != nil && st.typeCode != t {
		return []state{{branch: st.branch, cur: cur, frames: st.frames}}
	}
	st.cur = cur
	return []state{st}
}

// items returns the values of a property: each item of an array, or the single value.
func items(obj map[string]any, key string) []any {
	if obj == nil {
		return nil
	}
	switch v := obj[key].(type) {
	case nil:
		return nil
	case []any:
		return v
	default:
		return []any{v}
	}
}

// singleTypeCode returns the type of an instance value: the resourceType of a resource, else the
// element's type when it has exactly one.
func singleTypeCode(types []registry.Type, v any) string {
	if m, ok := v.(map[string]any); ok {
		if rt, _ := m[resourceTypeKey].(string); rt != "" {
			return rt
		}
	}
	if len(types) == 1 {
		return types[0].Code
	}
	return ""
}

func describe(c cursor) string {
	if c.node == nil {
		return "the discriminated value"
	}
	return c.node.Def.ID
}

// branch is one definition of an element's children: the only one, or one of the alternatives
// when the element's type declares several profiles (which the element may conform to any of).
type branch struct {
	sd       *registry.StructureDefinition
	children []*registry.ElementNode
}

// children returns the child elements of the element at c, per plan A's "children of a resolved
// member": its own children in the snapshot; else those of the element it slices; else those its
// contentReference points to; else those of its type's definition: each profile the type
// declares, or the base type when it declares none. The instance's type (typeCode) picks the
// type of a choice or of a resource element.
func (m *Matcher) children(c cursor, typeCode string) ([]branch, error) {
	if c.node == nil {
		return nil, errors.New("the path continues past a value with no definition")
	}
	if b, ok := m.snapshotChildren(c); ok {
		return []branch{b}, nil
	}
	return m.typeChildren(c, typeCode)
}

// snapshotChildren returns the children the snapshot gives the element at c: its own, those of
// the element it slices, or those its contentReference points to.
func (m *Matcher) snapshotChildren(c cursor) (branch, bool) {
	if len(c.node.Children) > 0 {
		return branch{c.sd, c.node.Children}, true
	}
	for s := c.node.SliceOf; s != nil; s = s.SliceOf {
		if len(s.Children) > 0 {
			return branch{c.sd, s.Children}, true
		}
	}
	ref := c.node.Def.ContentReference
	if ref == nil {
		return branch{}, false
	}
	target, _ := m.reg.ContentReference(c.sd, c.node)
	if target == nil {
		return branch{}, false
	}
	tsd := c.sd
	if url, _, ok := registry.SplitContentReference(*ref); ok && url != "" {
		if s, _ := m.reg.ResolveCanonical(url); s != nil {
			tsd = s
		}
	}
	return branch{tsd, target.Children}, true
}

// typeChildren returns the children of the type of the element at c: one branch per profile the
// type declares, or the base type's when it declares none.
func (m *Matcher) typeChildren(c cursor, typeCode string) ([]branch, error) {
	types := c.allowed()
	if typeCode != "" {
		var picked []registry.Type
		for _, t := range types {
			if t.Code == typeCode {
				picked = append(picked, t)
			}
		}
		if len(picked) == 0 {
			// A resource element (type Resource or a base type) holds a concrete resource.
			picked = []registry.Type{{Code: typeCode}}
		}
		types = picked
	}
	if len(types) != 1 {
		return nil, fmt.Errorf("%s: the type is ambiguous (%d types)", describe(c), len(types))
	}
	t := types[0]
	var sds []*registry.StructureDefinition
	var lastErr error
	for _, p := range t.Profile {
		psd, res := m.reg.ResolveCanonical(p)
		if psd == nil {
			lastErr = fmt.Errorf("profile %s could not be resolved (%s)", p, res)
			continue
		}
		sds = append(sds, psd)
	}
	if len(t.Profile) == 0 {
		if base := m.reg.GetByType(t.Code); base != nil {
			sds = append(sds, base)
		} else {
			lastErr = fmt.Errorf("no definition for type %q", t.Code)
		}
	}
	var out []branch
	for _, tsd := range sds {
		tree, err := m.tree(tsd)
		if err != nil {
			lastErr = err
			continue
		}
		if tree.Root() == nil {
			lastErr = fmt.Errorf("%s has no snapshot", tsd.URL)
			continue
		}
		out = append(out, branch{tsd, tree.Root().Children})
	}
	if len(out) == 0 {
		return nil, lastErr
	}
	return out, nil
}

// resourceDefinition returns the definition of a resolved resource: the target profile for its
// type when the reference declares one, else its base definition.
func (m *Matcher) resourceDefinition(resourceType string, targets []string) *registry.StructureDefinition {
	for _, t := range targets {
		if sd, _ := m.reg.ResolveCanonical(t); sd != nil && sd.Type == resourceType {
			return sd
		}
	}
	return m.reg.GetByType(resourceType)
}

// tree returns a StructureDefinition's tree, generating its snapshot first when it has only a
// differential.
func (m *Matcher) tree(sd *registry.StructureDefinition) (*registry.ElementTree, error) {
	// EnsureSnapshot takes the definition's lock and returns at once when a snapshot exists;
	// reading Snapshot here instead would race with a concurrent validation generating it.
	if err := m.reg.EnsureSnapshot(context.Background(), sd); err != nil {
		return nil, err
	}
	return sd.Tree(), nil
}

// Value sources for value and pattern discriminators.
type sourceKind int

const (
	sourceValues sourceKind = iota
	sourceBinding
)

// expectation is one value the discriminated element must have: equal to a fixed value, or
// containing a pattern.
type expectation struct {
	value   json.RawMessage
	pattern bool
}

type valueSource struct {
	kind     sourceKind
	expect   []expectation
	valueSet string
}

// bindingRequired is the binding strength that fixes a discriminator's value domain.
const bindingRequired = "required"

// findValueSource returns what the discriminated element must be, from the slice's definition, in
// the order the spec lists: its fixed value or pattern; one of an ancestor that contains it; the
// fixed values or patterns of the required slices of a sliced element on the way (every one of
// them must be present, as those slices are); or a required binding.
func findValueSource(ends []end) (valueSource, bool) {
	for _, e := range ends {
		if e.def == nil {
			continue
		}
		if exp := ownValue(e.def); exp != nil {
			return valueSource{expect: []expectation{*exp}}, true
		}
		for i := len(e.frames) - 1; i >= 0; i-- {
			f := e.frames[i]
			if f.node == nil || len(f.after) == 0 {
				continue
			}
			if exp := ownValue(f.node); exp != nil {
				if sub := extract(exp.value, f.after); len(sub) > 0 {
					return valueSource{expect: expectations(sub, exp.pattern)}, true
				}
			}
		}
		for i := len(e.frames) - 1; i >= 0; i-- {
			if exp := requiredSliceValues(e.frames[i]); len(exp) > 0 {
				return valueSource{expect: exp}, true
			}
		}
		if b := e.def.Def.Binding; b != nil && b.Strength == bindingRequired && b.ValueSet != "" {
			return valueSource{kind: sourceBinding, valueSet: b.ValueSet}, true
		}
		return valueSource{}, false
	}
	return valueSource{}, false
}

// ownValue returns an element's fixed value or pattern.
func ownValue(n *registry.ElementNode) *expectation {
	if v, _, ok := n.Def.GetFixed(); ok {
		return &expectation{value: v}
	}
	if v, _, ok := n.Def.GetPattern(); ok {
		return &expectation{value: v, pattern: true}
	}
	return nil
}

func expectations(values []json.RawMessage, pattern bool) []expectation {
	out := make([]expectation, len(values))
	for i, v := range values {
		out[i] = expectation{value: v, pattern: pattern}
	}
	return out
}

// requiredSliceValues looks inside the required slices of a sliced element met on the path: the
// element named first in f.after, when it is sliced. For each slice with min > 0, the value at the
// rest of the path (a fixed value or pattern on that slice's element, or inside one on the way)
// must be present.
func requiredSliceValues(f frame) []expectation {
	if f.node == nil || len(f.after) == 0 {
		return nil
	}
	var sliced *registry.ElementNode
	for _, c := range f.node.Children {
		if c.Name() == f.after[0] {
			sliced = c
			break
		}
	}
	if sliced == nil || sliced.Def.Slicing == nil {
		return nil
	}
	var out []expectation
	for _, s := range sliced.Slices {
		if s.Def.Min == 0 {
			continue
		}
		if v := valueWithin(s, f.after[1:]); v != nil {
			out = append(out, v...)
		}
	}
	return out
}

// valueWithin returns the fixed values or patterns at a path of element names below n, taken
// from the element the path reaches or from one on the way that contains the rest.
func valueWithin(n *registry.ElementNode, path []string) []expectation {
	if exp := ownValue(n); exp != nil {
		if len(path) == 0 {
			return []expectation{*exp}
		}
		if sub := extract(exp.value, path); len(sub) > 0 {
			return expectations(sub, exp.pattern)
		}
	}
	if len(path) == 0 {
		return nil
	}
	for _, c := range n.Children {
		if c.Name() == path[0] {
			return valueWithin(c, path[1:])
		}
	}
	return nil
}

// extract returns the values at a path of element names inside a JSON value, flattening arrays.
func extract(raw json.RawMessage, path []string) []json.RawMessage {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	cur := []any{v}
	for _, name := range path {
		var next []any
		for _, c := range cur {
			next = append(next, items(asMap(c), name)...)
		}
		cur = next
	}
	out := make([]json.RawMessage, 0, len(cur))
	for _, c := range cur {
		if b, err := json.Marshal(c); err == nil {
			out = append(out, b)
		}
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
