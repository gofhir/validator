package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// accumulateFields lists ElementDefinition fields whose values are appended (not replaced)
// when merging differential onto base. All other fields use override semantics.
var accumulateFields = map[string]bool{
	"constraint": true,
	"mapping":    true,
	"code":       true,
	"alias":      true,
}

// The slicing rules a generated slicing uses (profiling.html).
const (
	slicingOpen   = "open"
	slicingClosed = "closed"
)

// fhirPathSystemTypes is the prefix of the FHIRPath system types (System.String) a primitive's value
// element is typed with (structuredefinition.html).
const fhirPathSystemTypes = "http://hl7.org/fhirpath/System."

// errSnapshotCycle is returned for a StructureDefinition whose snapshot is needed to generate its
// own: a profile its own elements, directly or through others, declare as their type.
var errSnapshotCycle = errors.New("its snapshot is needed to generate itself")

// permanentError is a snapshot generation failure that cannot change, so EnsureSnapshot keeps it:
// a StructureDefinition without a differential or a base, or whose base cannot be had and no
// resolver could provide later.
type permanentError struct{ error }

func (e permanentError) Unwrap() error { return e.error }

// generationKey is the context key of the generation a call chain holds (snapshotGen).
type generationKey struct{}

// generation is a call chain generating snapshots under snapshotGen. Its frames are the
// StructureDefinitions being generated, outermost first.
type generation struct {
	frames []*generationFrame
}

type generationFrame struct {
	sd *StructureDefinition
	// cyclic reports that a cycle was cut while generating it, or something it depends on.
	cyclic bool
}

func (g *generation) generating(sd *StructureDefinition) bool {
	for _, f := range g.frames {
		if f.sd == sd {
			return true
		}
	}
	return false
}

// EnsureSnapshot generates a snapshot for the given StructureDefinition if it
// only has a differential. It resolves the base chain, deep-copies the base
// snapshot, and merges the differential on top.
// If the SD already has a snapshot, this is a no-op. When generating it fails for a reason that
// cannot change (permanentError), it fails again with the same error without trying again; a
// failure that may not recur (a canceled context, a resolver) is not kept.
//
// Safe for concurrent use: generation is serialized by one lock per registry, held once per call
// chain, under which a generation makes the snapshots it needs (bases, type profiles). One lock
// cannot be taken in two orders, and a snapshot made is never made again. Profiles that need each
// other's snapshots (a cycle of type profiles) are detected in the chain.
func (r *Registry) EnsureSnapshot(ctx context.Context, sd *StructureDefinition) error {
	if sd.storedSnapshot() != nil {
		return nil
	}
	g, held := ctx.Value(generationKey{}).(*generation)
	if !held {
		r.snapshotGen.Lock()
		defer r.snapshotGen.Unlock()
		g = &generation{}
		ctx = context.WithValue(ctx, generationKey{}, g)
	}
	_, err := r.snapshotOf(ctx, g, sd)
	return err
}

// storedSnapshot returns the snapshot sd holds, under its lock.
func (sd *StructureDefinition) storedSnapshot() *Snapshot {
	sd.snapshotMu.Lock()
	defer sd.snapshotMu.Unlock()
	return sd.Snapshot
}

// SnapshotNotes returns the elements of the StructureDefinition's differential its generated
// snapshot leaves out, because they name no element of the base, and why. The HL7 validator leaves
// them out too.
func (sd *StructureDefinition) SnapshotNotes() []string {
	sd.snapshotMu.Lock()
	defer sd.snapshotMu.Unlock()
	return sd.snapshotNotes
}

// snapshotOf returns sd's snapshot, generating and keeping it when it has none. Under g (the lock
// held): a StructureDefinition already being generated in the chain is a cycle, which marks every
// frame of the chain cyclic. A snapshot generated while a cycle was cut is kept only for the
// StructureDefinition the chain started from, and generated again where another needs it, so that
// the result does not depend on the order snapshots were asked for.
func (r *Registry) snapshotOf(ctx context.Context, g *generation, sd *StructureDefinition) (*Snapshot, error) {
	sd.snapshotMu.Lock()
	stored, failed, cyclic := sd.Snapshot, sd.snapshotErr, sd.snapshotCyclic
	sd.snapshotMu.Unlock()
	switch {
	case stored != nil && (!cyclic || len(g.frames) == 0):
		return stored, nil
	case failed != nil:
		return nil, failed
	case g.generating(sd):
		for _, f := range g.frames {
			f.cyclic = true
		}
		return nil, fmt.Errorf("cannot generate snapshot for %s: %w", sd.URL, errSnapshotCycle)
	}

	frame := &generationFrame{sd: sd}
	g.frames = append(g.frames, frame)
	snapshot, notes, err := r.generateSnapshot(ctx, sd)
	g.frames = g.frames[:len(g.frames)-1]
	if err != nil {
		var permanent permanentError
		if errors.As(err, &permanent) && ctx.Err() == nil {
			sd.snapshotMu.Lock()
			sd.snapshotErr = err
			sd.snapshotMu.Unlock()
		}
		return nil, err
	}
	if frame.cyclic && len(g.frames) > 0 {
		return snapshot, nil // made for the one that needs it, not kept
	}
	sd.snapshotMu.Lock()
	if sd.Snapshot == nil {
		sd.Snapshot, sd.snapshotNotes, sd.snapshotCyclic = snapshot, notes, frame.cyclic
	}
	snapshot = sd.Snapshot
	sd.snapshotMu.Unlock()
	return snapshot, nil
}

// profileSnapshot returns the snapshot of a definition an element of the snapshot being generated
// needs (a profile its type declares, its type's definition), generating it in the same chain.
func (r *Registry) profileSnapshot(ctx context.Context, sd *StructureDefinition) (*Snapshot, error) {
	g, held := ctx.Value(generationKey{}).(*generation)
	if !held {
		if err := r.EnsureSnapshot(ctx, sd); err != nil {
			return nil, err
		}
		return sd.storedSnapshot(), nil
	}
	return r.snapshotOf(ctx, g, sd)
}

// generateSnapshot generates the snapshot of sd from its base's and its differential, without
// storing it, and returns the differential elements it leaves out.
func (r *Registry) generateSnapshot(ctx context.Context, sd *StructureDefinition) (*Snapshot, []string, error) {
	if sd.Differential == nil || len(sd.Differential.Element) == 0 {
		return nil, nil, permanentError{fmt.Errorf("cannot generate snapshot for %s: no differential", sd.URL)}
	}
	if sd.BaseDefinition == "" {
		return nil, nil, permanentError{fmt.Errorf("cannot generate snapshot for %s: no baseDefinition", sd.URL)}
	}
	baseURL, baseVersion := ParseCanonical(sd.BaseDefinition)
	baseSD := r.ResolveByCanonical(ctx, baseURL, baseVersion)
	if baseSD == nil {
		err := fmt.Errorf("cannot generate snapshot for %s: base %s not found", sd.URL, sd.BaseDefinition)
		r.mu.RLock()
		resolver := r.resolver
		r.mu.RUnlock()
		if resolver == nil {
			err = permanentError{err} // nothing could provide it later
		}
		return nil, nil, err
	}
	baseSnapshot, err := r.profileSnapshot(ctx, baseSD)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot generate snapshot for %s: base snapshot failed: %w", sd.URL, err)
	}
	if baseSnapshot == nil {
		return nil, nil, permanentError{fmt.Errorf("cannot generate snapshot for %s: base %s has no snapshot", sd.URL, sd.BaseDefinition)}
	}
	snapshot, notes, err := r.applyDifferential(ctx, baseSnapshot, sd.Differential)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot generate snapshot for %s: %w", sd.URL, err)
	}
	// The merge rebuilds a changed element from its base's published JSON, so the corrections are
	// applied again, for the profile's FHIR version, or its base's when it declares none.
	version := sd.FHIRVersion
	if version == "" {
		version = baseSD.FHIRVersion
	}
	correctElements(version, snapshot.Element)
	return snapshot, notes, nil
}

// snapshotBuilder holds a snapshot being generated.
type snapshotBuilder struct {
	r        *Registry
	ctx      context.Context
	elements []ElementDefinition
	// inBase holds the ids of the base snapshot's elements.
	inBase map[string]bool
	// diffKeys holds, by id, the fields the differential has set on each element so far.
	diffKeys map[string]map[string]bool
	// floor holds, by id, the minimum an element's source sets, which the differential's own changes
	// to its type do not relax: a base element's or an unrolled element's own minimum, and for a
	// slice's copy of a child, the floor of the child it copies.
	floor map[string]uint32
	// renamed holds the ids of the type slices the differential named as renamed choices
	// ("Observation.valueQuantity").
	renamed map[string]bool
}

// applyDifferential merges differential elements onto a deep copy of the base snapshot, as
// profiling.html describes snapshot generation. Each differential element is placed by its id: the
// element of the base with that id, or one created for it (ensure). Matching by path instead would
// let a slice's child overwrite the base element (Extension.extension:a.url and
// Extension.extension.url share a path). An element that names nothing the base has is left out,
// as the HL7 validator leaves it out, and returned with why.
func (r *Registry) applyDifferential(ctx context.Context, base *Snapshot, diff *Differential) (*Snapshot, []string, error) {
	elements, err := deepCopyElements(base.Element)
	if err != nil {
		return nil, nil, fmt.Errorf("deep copy failed: %w", err)
	}
	b := &snapshotBuilder{r: r, ctx: ctx, elements: elements, inBase: map[string]bool{},
		diffKeys: map[string]map[string]bool{}, floor: map[string]uint32{}, renamed: map[string]bool{}}
	for i := range elements {
		b.inBase[elements[i].ID] = true
		b.floor[elements[i].ID] = elements[i].Min
	}

	var notes []string
	ids := differentialIDs(diff.Element, true)
	pathIDs := differentialIDs(diff.Element, false)
	for i := range diff.Element {
		diffElem := diff.Element[i]
		idx, id, err := b.place(ids[i])
		if err != nil && pathIDs[i] != ids[i] {
			// An id that does not follow the convention is placed by its path, as the HL7
			// validator matches elements.
			idx, id, err = b.place(pathIDs[i])
		}
		if err != nil {
			notes = append(notes, fmt.Sprintf("differential element %s is left out: %v", ids[i], err))
			continue
		}
		// The differential's own id and path may be a renamed choice's ("Observation.valueQuantity"):
		// the element keeps the id and path it is placed at.
		placed, err := withIDAndPath(&diffElem, id, b.elements[idx].Path)
		if err != nil {
			return nil, nil, err
		}
		before := typeCodesOf(&b.elements[idx])
		merged, err := mergeElements(&b.elements[idx], placed)
		if err != nil {
			return nil, nil, fmt.Errorf("merge failed for %s: %w", id, err)
		}
		b.elements[idx] = *merged
		keys, err := rawToMap(placed.raw)
		if err != nil {
			return nil, nil, err
		}
		if b.diffKeys[id] == nil {
			b.diffKeys[id] = map[string]bool{}
		}
		for k := range keys {
			b.diffKeys[id][k] = true
		}
		if err := b.profileCardinality(idx, before); err != nil {
			return nil, nil, err
		}
		if err := b.typedTypeSlice(idx); err != nil {
			return nil, nil, err
		}
	}
	if err := b.implicitExtensionSlicing(); err != nil {
		return nil, nil, err
	}
	if err := b.closedTypeSlicing(); err != nil {
		return nil, nil, err
	}
	return &Snapshot{Element: b.elements}, notes, nil
}

// place returns where the element with the id goes (ensure). When it cannot be placed, what
// trying made (slices, unrolled children) is undone.
func (b *snapshotBuilder) place(id string) (idx int, placed string, err error) {
	elements := slices.Clone(b.elements)
	floor := maps.Clone(b.floor)
	renamed := maps.Clone(b.renamed)
	idx, placed, err = b.ensure(id, true)
	if err != nil {
		b.elements, b.floor, b.renamed = elements, floor, renamed
	}
	return idx, placed, err
}

// closedTypeSlicing restricts a choice sliced by type with closed rules to the types of its slices:
// closed slicing allows no value its slices do not match (profiling.html), so the choice can take
// no other type (ch-ext-ech-10-linetype's value[x], closed with one slice valueCode, is code).
func (b *snapshotBuilder) closedTypeSlicing() error {
	for i := range b.elements {
		choice := &b.elements[i]
		if choice.Slicing == nil || choice.Slicing.Rules != slicingClosed || !strings.HasSuffix(choice.Path, "[x]") ||
			!slices.ContainsFunc(choice.Slicing.Discriminator, func(d Discriminator) bool { return d.Type == "type" && d.Path == "$this" }) {
			continue
		}
		allowed := map[string]bool{}
		for j := range b.elements {
			if slice, ok := strings.CutPrefix(b.elements[j].ID, choice.ID+":"); ok && !strings.ContainsAny(slice, ".:/") {
				for _, t := range b.elements[j].Type {
					allowed[t.Code] = true
				}
			}
		}
		if len(allowed) == 0 {
			continue
		}
		var types []Type
		for _, t := range choice.Type {
			if allowed[t.Code] {
				types = append(types, t)
			}
		}
		if len(types) == len(choice.Type) || len(types) == 0 {
			continue
		}
		restricted, err := cloneElement(choice, map[string]any{"type": types})
		if err != nil {
			return err
		}
		b.elements[i] = *restricted
	}
	return nil
}

// set reports whether the differential has set the field on the element with the id.
func (b *snapshotBuilder) set(id, field string) bool {
	return b.diffKeys[id][field]
}

// profileCardinality gives an element whose type the differential changes or profiles the minimum
// the root of that type's definition declares (of the profile it names, or of several, the lowest), and the
// maximum of a single profile's root when lower, where the differential sets none, as the HL7
// validator generates them: an extension slice declaring a profile whose root is 0..1 is 0..1
// (au-patient birthPlace), and a new Bundle entry slice's resource typed Condition is 0..1 (mcode),
// although Bundle.entry.resource is 1..1 there. The minimum never goes below the one the element's
// own definition sets (base.min, for an element that is not a slice), nor its source's (floor): a
// base profile's constraint is not relaxed.
func (b *snapshotBuilder) profileCardinality(idx int, before []string) error {
	e := &b.elements[idx]
	if !b.set(e.ID, "type") || len(e.Type) == 0 || (b.set(e.ID, "min") && b.set(e.ID, "max")) {
		return nil
	}
	// Only a type the differential changes (its code) or profiles redefines the element's
	// cardinality: restating it (Reference with a targetProfile) keeps the element's own.
	profiles := slices.ContainsFunc(e.Type, func(t Type) bool { return len(t.Profile) > 0 })
	if !profiles && slices.Equal(before, typeCodesOf(e)) {
		return nil
	}
	roots := b.typeRoots(e)
	if len(roots) == 0 {
		return nil // without the definitions' snapshots, the element keeps its own
	}
	set := map[string]any{}
	if !b.set(e.ID, "min") {
		lowest := roots[0].Min
		for _, root := range roots[1:] {
			lowest = min(lowest, root.Min)
		}
		if e.Base != nil && e.SliceName == nil {
			lowest = max(lowest, e.Base.Min)
		}
		lowest = max(lowest, b.floor[e.ID])
		if lowest != e.Min {
			set["min"] = lowest
		}
	}
	if !b.set(e.ID, "max") && profiles && len(roots) == 1 && maxBelow(roots[0].Max, e.Max) {
		set["max"] = roots[0].Max
	}
	if len(set) == 0 {
		return nil
	}
	narrowed, err := cloneElement(e, set)
	if err != nil {
		return err
	}
	b.elements[idx] = *narrowed
	return nil
}

// typeRoots returns the root elements of the definitions an element's types name: the profiles
// each declares, or else the type's own definition. It returns none when one cannot be had.
func (b *snapshotBuilder) typeRoots(e *ElementDefinition) []*ElementDefinition {
	var roots []*ElementDefinition
	for _, t := range e.Type {
		var defs []*StructureDefinition
		for _, p := range t.Profile {
			url, version := ParseCanonical(p)
			defs = append(defs, b.r.ResolveByCanonical(b.ctx, url, version))
		}
		if len(t.Profile) == 0 {
			defs = append(defs, b.r.GetByType(t.Code))
		}
		for _, sd := range defs {
			if sd == nil {
				return nil
			}
			snapshot, err := b.r.profileSnapshot(b.ctx, sd)
			if err != nil || snapshot == nil || len(snapshot.Element) == 0 {
				return nil
			}
			roots = append(roots, &snapshot.Element[0])
		}
	}
	return roots
}

// typeCodesOf returns the codes of an element's types.
func typeCodesOf(e *ElementDefinition) []string {
	codes := make([]string, len(e.Type))
	for i, t := range e.Type {
		codes[i] = t.Code
	}
	return codes
}

// maxBelow reports whether the cardinality maximum a is below b ("1" is below "*").
func maxBelow(a, b string) bool {
	if a == "" || a == "*" {
		return false
	}
	if b == "" || b == "*" {
		return true
	}
	var na, nb int
	_, errA := fmt.Sscan(a, &na)
	_, errB := fmt.Sscan(b, &nb)
	return errA == nil && errB == nil && na < nb
}

// implicitExtensionSlicing gives an extension element the differential slices without defining
// its slicing the slicing every extension element has ("Extensions are always sliced by url",
// profiling.html), as the HL7 validator generates it. A slicing the base defines is left as it
// is. The element's own cardinality is not changed: each slice's is checked on its own.
func (b *snapshotBuilder) implicitExtensionSlicing() error {
	for i := range b.elements {
		sliced := &b.elements[i]
		if sliced.Slicing != nil || len(sliced.Type) != 1 || sliced.Type[0].Code != "Extension" {
			continue
		}
		hasSlices := slices.ContainsFunc(b.elements, func(e ElementDefinition) bool {
			slice, ok := strings.CutPrefix(e.ID, sliced.ID+":")
			return ok && !strings.ContainsAny(slice, ".:/")
		})
		if !hasSlices {
			continue
		}
		implied, err := cloneElement(sliced, map[string]any{"slicing": Slicing{
			Discriminator: []Discriminator{{Type: "value", Path: "url"}}, Rules: slicingOpen,
		}})
		if err != nil {
			return err
		}
		b.elements[i] = *implied
	}
	return nil
}

// typedTypeSlice restricts a choice to the type of its type slice at idx when the choice can only
// take that type: the slice is required and the choice holds one value (value[x] 1..1 with
// value[x]:valueCode 1..1), or the differential named the slice as a renamed choice and typed it
// (Observation.valueCodeableConcept with type CodeableConcept). Its slicing is closed, and it takes
// the cardinality the differential sets on the slice, never below its own minimum, as the HL7
// validator 6.10 generates them (ch-ext-ech-10-linetype, genomics-reporting variant).
func (b *snapshotBuilder) typedTypeSlice(idx int) error {
	e := &b.elements[idx]
	if e.SliceName == nil || len(e.Type) != 1 {
		return nil
	}
	cut := strings.LastIndexByte(e.ID, ':')
	choiceID := e.ID[:cut]
	if !strings.HasSuffix(choiceID, "[x]") {
		return nil
	}
	dot := strings.LastIndexByte(choiceID, '.')
	ci := b.find(choiceID)
	if ci < 0 || choiceType(&b.elements[ci], strings.TrimSuffix(choiceID[dot+1:], "[x]"), e.ID[cut+1:]) == nil {
		return nil
	}
	choice := &b.elements[ci]
	required := e.Min >= 1 && choice.Max == "1"
	if !required && (!b.renamed[e.ID] || !b.set(e.ID, "type")) {
		return nil
	}
	slicing := Slicing{Discriminator: []Discriminator{{Type: "type", Path: "$this"}}, Rules: slicingClosed}
	if choice.Slicing != nil {
		slicing = *choice.Slicing
		slicing.Rules = slicingClosed
	}
	set := map[string]any{"type": e.Type, "slicing": slicing}
	if e.Min > choice.Min {
		set["min"] = e.Min
	}
	if b.set(e.ID, "max") && maxBelow(e.Max, choice.Max) {
		set["max"] = e.Max
	}
	updated, err := cloneElement(choice, set)
	if err != nil {
		return err
	}
	b.elements[ci] = *updated
	return nil
}

// differentialIDs returns the id of each differential element. With own, an element's own id is
// used when it has one; otherwise, and for a differential written without ids, the id is built
// from its path and the slices named before it (an element under a path a slice was named at
// belongs to that slice, until an element at that path names no slice).
func differentialIDs(diff []ElementDefinition, own bool) []string {
	ids := make([]string, len(diff))
	current := map[string]string{} // path -> the slice named at it last
	for i, e := range diff {
		for p := range current {
			if p == e.Path || strings.HasPrefix(p, e.Path+".") {
				delete(current, p)
			}
		}
		if e.SliceName != nil {
			current[e.Path] = *e.SliceName
		}
		if own && e.ID != "" {
			ids[i] = e.ID
			continue
		}
		segments := strings.Split(e.Path, ".")
		var id strings.Builder
		for k, seg := range segments {
			if k > 0 {
				id.WriteByte('.')
			}
			id.WriteString(seg)
			if slice, ok := current[strings.Join(segments[:k+1], ".")]; ok {
				id.WriteString(":" + slice)
			}
		}
		ids[i] = id.String()
	}
	return ids
}

// find returns the index of the element with the id, or -1.
func (b *snapshotBuilder) find(id string) int {
	for i := range b.elements {
		if b.elements[i].ID == id {
			return i
		}
	}
	return -1
}

// ensure returns the index of the element with the id, creating it, and the elements it is under,
// when the snapshot does not have it yet. It also returns the element's id as placed: a renamed
// choice is placed as the type slice it stands for ("Observation.valueQuantity" as
// "Observation.value[x]:valueQuantity"), and a choice named without its [x] as the choice. Leaf
// reports whether the id is the differential element's own, not one of the elements it is under.
func (b *snapshotBuilder) ensure(id string, leaf bool) (idx int, placed string, err error) {
	if i := b.find(id); i >= 0 {
		return i, id, nil
	}
	dot := strings.LastIndexByte(id, '.')
	if dot < 0 {
		return -1, id, errors.New("no element with that id in the base")
	}
	parentID, segment := id[:dot], id[dot+1:]
	if name, slice, sliced := strings.Cut(segment, ":"); sliced {
		return b.ensureSlice(parentID, name, slice)
	}

	parentIdx, parentID, err := b.ensure(parentID, false)
	if err != nil {
		return -1, id, err
	}
	id = parentID + "." + segment
	if i := b.find(id); i >= 0 {
		return i, id, nil
	}
	if !b.hasChildren(parentID) {
		if err := b.unrollChildren(parentIdx); err != nil {
			return -1, id, err
		}
		if i := b.find(id); i >= 0 {
			return i, id, nil
		}
	}
	if choice, ok := b.renamedChoice(parentID, segment); ok {
		choiceID := parentID + "." + choice
		ci := b.find(choiceID)
		if i := b.find(choiceID + ":" + segment); i >= 0 {
			return i, choiceID + ":" + segment, nil // the type slice the differential made
		}
		switch {
		case !b.inBase[choiceID]:
			// A choice the base does not have itself (the copy a new slice makes of its sliced
			// element's) is restricted to the type, without a slice, as the HL7 validator
			// generates it (component:systolic.valueQuantity is component:systolic.value[x]).
			return b.restrictChoice(ci, strings.TrimSuffix(choice, "[x]"), segment)
		case !leaf && len(b.elements[ci].Type) == 1:
			// Named only as the parent of an element, a choice that has that one type is the
			// choice itself (quantity-accuracy's Extension.valueQuantity.value), as HL7 6.10
			// generates it.
			return ci, choiceID, nil
		}
		i, placed, err := b.ensureSlice(parentID, choice, segment)
		if err == nil && leaf {
			b.renamed[placed] = true
		}
		return i, placed, err
	}
	// A choice named without its [x] ("Library.subject" for subject[x]).
	if i := b.find(id + "[x]"); i >= 0 {
		return i, id + "[x]", nil
	}
	return -1, id, fmt.Errorf("no element %s in the base", id)
}

// restrictChoice restricts the choice element at idx (named name[x]) to the one type segment
// names, and returns it.
func (b *snapshotBuilder) restrictChoice(idx int, name, segment string) (at int, placed string, err error) {
	e := &b.elements[idx]
	if t := choiceType(e, name, segment); t != nil && len(e.Type) > 1 {
		restricted, err := cloneElement(e, map[string]any{"type": []Type{*t}})
		if err != nil {
			return -1, "", err
		}
		b.elements[idx] = *restricted
	}
	return idx, b.elements[idx].ID, nil
}

// ensureSlice returns the index of the slice of parentID.name named slice, creating it as a copy of
// the element it slices (of the slice it reslices, for "a/b"), with what the differential has
// constrained it with so far, placed after that element's subtree, its other slices included.
func (b *snapshotBuilder) ensureSlice(parentID, name, slice string) (idx int, placed string, err error) {
	_, parentID, err = b.ensure(parentID, false)
	if err != nil {
		return -1, "", err
	}
	id := parentID + "." + name + ":" + slice
	if i := b.find(id); i >= 0 {
		return i, id, nil
	}
	slicedID := parentID + "." + name
	if k := strings.LastIndexByte(slice, '/'); k >= 0 {
		slicedID += ":" + slice[:k]
	}
	slicedIdx, slicedID, err := b.ensure(slicedID, false)
	if err != nil {
		return -1, id, err
	}
	// A slice of an element defined by reference has that element's children, and so has the
	// sliced element, as the HL7 validator generates them.
	if b.elements[slicedIdx].ContentReference != nil && !b.hasChildren(slicedID) {
		if err := b.unrollFrom(slicedIdx, *b.elements[slicedIdx].ContentReference); err != nil {
			return -1, id, err
		}
	}
	// Making the sliced element may have made the slice: unrolled from a profile that has it.
	if i := b.find(id); i >= 0 {
		return i, id, nil
	}
	sliced := &b.elements[slicedIdx]
	// A slice starts at min 0, whatever the sliced element's minimum: that applies to all its
	// slices together (profiling.html), and the differential sets the slice's own.
	entry, err := cloneElement(sliced, map[string]any{"id": id, "sliceName": slice, "min": 0}, "slicing")
	if err != nil {
		return -1, id, err
	}
	// A slice of an element defined by reference (Parameters.parameter.part) is that element: it
	// takes its type, and its children are unrolled from it.
	if sliced.ContentReference != nil {
		ref := b.find(contentReferenceID(*sliced.ContentReference))
		if ref < 0 {
			return -1, id, fmt.Errorf("contentReference %s names no element", *sliced.ContentReference)
		}
		if entry, err = cloneElement(entry, map[string]any{"type": b.elements[ref].Type}, "contentReference"); err != nil {
			return -1, id, err
		}
	}
	// A type slice of a choice ("valueQuantity") has that one type, and the choice is sliced by
	// type ($this).
	if strings.HasSuffix(name, "[x]") {
		if t := choiceType(sliced, strings.TrimSuffix(name, "[x]"), slice); t != nil {
			if entry, err = cloneElement(entry, map[string]any{"type": []Type{*t}}); err != nil {
				return -1, id, err
			}
			if err := b.sliceByType(slicedIdx); err != nil {
				return -1, id, err
			}
		}
	}
	at := b.subtreeEnd(slicedID)
	b.elements = slices.Insert(b.elements, at, *entry)
	return at, id, nil
}

// sliceByType gives the choice element at idx the slicing by type ($this, open) its type slices
// need, when it has no slicing.
func (b *snapshotBuilder) sliceByType(idx int) error {
	if b.elements[idx].Slicing != nil {
		return nil
	}
	sliced, err := cloneElement(&b.elements[idx], map[string]any{"slicing": Slicing{
		Discriminator: []Discriminator{{Type: "type", Path: "$this"}}, Rules: slicingOpen,
	}})
	if err != nil {
		return err
	}
	b.elements[idx] = *sliced
	return nil
}

// hasChildren reports whether the snapshot has an element under id.
func (b *snapshotBuilder) hasChildren(id string) bool {
	for i := range b.elements {
		if strings.HasPrefix(b.elements[i].ID, id+".") {
			return true
		}
	}
	return false
}

// subtreeEnd returns the index after the element with the id, its children and its slices.
func (b *snapshotBuilder) subtreeEnd(id string) int {
	end := -1
	for i := range b.elements {
		e := b.elements[i].ID
		if e == id || strings.HasPrefix(e, id+".") || strings.HasPrefix(e, id+":") || strings.HasPrefix(e, id+"/") {
			end = i
		}
	}
	return end + 1
}

// unrollChildren inserts, after the element at idx, its children. A slice has the children of the
// element it slices, with what the differential has constrained them with, as that element's
// constraints apply to each slice; when it has none, they come from where the slice's own
// definition takes them (unroll).
func (b *snapshotBuilder) unrollChildren(idx int) error {
	id := b.elements[idx].ID
	dot := strings.LastIndexByte(id, '.')
	name, slice, sliced := strings.Cut(id[dot+1:], ":")
	if !sliced {
		return b.unroll(idx)
	}
	slicedID := id[:dot+1] + name
	if k := strings.LastIndexByte(slice, '/'); k >= 0 {
		slicedID += ":" + slice[:k]
	}
	var children []ElementDefinition
	for i := range b.elements {
		rest, ok := strings.CutPrefix(b.elements[i].ID, slicedID+".")
		if !ok {
			continue
		}
		child, err := cloneElement(&b.elements[i], map[string]any{"id": id + "." + rest})
		if err != nil {
			return err
		}
		if f, ok := b.floor[b.elements[i].ID]; ok {
			b.floor[child.ID] = f
		}
		children = append(children, *child)
	}
	if len(children) == 0 {
		if si := b.find(slicedID); si >= 0 && b.elements[si].ContentReference != nil {
			return b.unrollFrom(idx, *b.elements[si].ContentReference)
		}
		return b.unroll(idx)
	}
	b.elements = slices.Insert(b.elements, idx+1, children...)
	return nil
}

// contentReferenceID is the id of the element a contentReference names ("#Questionnaire.item", or
// "http://hl7.org/fhir/StructureDefinition/Parameters#Parameters.parameter"): the base element ids
// its path.
func contentReferenceID(ref string) string {
	_, id, _ := strings.Cut(ref, "#")
	return id
}

// unrollFrom inserts, after the element at idx, the children of the element a contentReference
// names: in the snapshot being generated, or else in the StructureDefinition its url names.
func (b *snapshotBuilder) unrollFrom(idx int, ref string) error {
	url, rootID, _ := strings.Cut(ref, "#")
	source := b.elements
	src := b.find(rootID)
	if src < 0 && url != "" {
		refURL, refVersion := ParseCanonical(url)
		if sd := b.r.ResolveByCanonical(b.ctx, refURL, refVersion); sd != nil {
			if snapshot, err := b.r.profileSnapshot(b.ctx, sd); err == nil && snapshot != nil {
				source = snapshot.Element
				for i := range source {
					if source[i].ID == rootID {
						src = i
						break
					}
				}
			}
		}
	}
	if src < 0 {
		return fmt.Errorf("%s: contentReference %s names no element", b.elements[idx].ID, ref)
	}
	parent, root := b.elements[idx], source[src]
	var children []ElementDefinition
	for i := range source {
		if rest, ok := strings.CutPrefix(source[i].ID, root.ID+"."); ok {
			child, err := cloneElement(&source[i], map[string]any{
				"id":   parent.ID + "." + rest,
				"path": parent.Path + strings.TrimPrefix(source[i].Path, root.Path),
			})
			if err != nil {
				return err
			}
			b.floor[child.ID] = child.Min
			children = append(children, *child)
		}
	}
	b.elements = slices.Insert(b.elements, idx+1, children...)
	return nil
}

// unroll inserts, after the element at idx, the children its definition takes from elsewhere: the
// element its contentReference names, or its type (the profile that type declares, when it
// declares exactly one). Each keeps the base its source declares. An element of several types
// (a choice) has the children all of them have (id, extension).
func (b *snapshotBuilder) unroll(idx int) error {
	parent := b.elements[idx]
	if parent.ContentReference != nil {
		// An element defined by reference whose children the differential names is that element:
		// it takes its type, and no longer refers to it (sdc-valueset's
		// ValueSet.expansion.contains.designation), as the HL7 validator generates it.
		ref := *parent.ContentReference
		if src := b.find(contentReferenceID(ref)); src >= 0 {
			resolved, err := cloneElement(&parent, map[string]any{"type": b.elements[src].Type}, "contentReference")
			if err != nil {
				return err
			}
			b.elements[idx] = *resolved
		}
		return b.unrollFrom(idx, ref)
	}
	snapshots, err := b.typeSnapshots(&parent)
	if err != nil {
		return err
	}
	first := snapshots[0]
	rootID, rootPath := first.Element[0].ID, first.Element[0].Path
	common := map[string]int{} // a direct child's name -> the number of types that have it
	for _, s := range snapshots {
		for i := range s.Element[1:] {
			if rest := strings.TrimPrefix(s.Element[i+1].ID, s.Element[0].ID+"."); !strings.ContainsAny(rest, ".:") {
				common[rest]++
			}
		}
	}
	children := make([]ElementDefinition, 0, len(first.Element))
	for i := range first.Element[1:] {
		e := &first.Element[i+1]
		rest := strings.TrimPrefix(e.ID, rootID+".")
		top, _, _ := strings.Cut(rest, ".")
		top, _, _ = strings.Cut(top, ":")
		if common[top] != len(snapshots) {
			continue
		}
		child, err := cloneElement(e, map[string]any{
			"id":   parent.ID + "." + rest,
			"path": parent.Path + strings.TrimPrefix(e.Path, rootPath),
		})
		if err != nil {
			return err
		}
		b.floor[child.ID] = child.Min
		children = append(children, *child)
	}
	b.elements = slices.Insert(b.elements, idx+1, children...)
	return nil
}

// typeSnapshots returns the snapshots an element's children come from: the profile its one type
// declares, when it declares exactly one, or else the definition of each of its types. A profile
// whose snapshot is needed to generate itself (an extension that declares itself as a part's type)
// gives way to its type's definition.
func (b *snapshotBuilder) typeSnapshots(e *ElementDefinition) ([]*Snapshot, error) {
	if len(e.Type) == 0 {
		return nil, fmt.Errorf("%s has no type: its children cannot be unrolled", e.ID)
	}
	if len(e.Type) == 1 && len(e.Type[0].Profile) == 1 {
		url, version := ParseCanonical(e.Type[0].Profile[0])
		if sd := b.r.ResolveByCanonical(b.ctx, url, version); sd != nil {
			snapshot, err := b.r.profileSnapshot(b.ctx, sd)
			switch {
			case err == nil && snapshot != nil && len(snapshot.Element) > 0:
				return []*Snapshot{snapshot}, nil
			case err != nil && !errors.Is(err, errSnapshotCycle):
				return nil, err
			}
		}
	}
	seen := map[string]bool{}
	var out []*Snapshot
	for _, t := range e.Type {
		if seen[t.Code] {
			continue
		}
		seen[t.Code] = true
		code := t.Code
		// A FHIRPath system type (a primitive's value, System.Boolean) has no definition of its own:
		// what it has as an element (id, extension) is Element's.
		if strings.HasPrefix(code, fhirPathSystemTypes) {
			code = "Element"
		}
		sd := b.r.GetByType(code)
		if sd == nil {
			return nil, fmt.Errorf("%s: no definition of type %s", e.ID, t.Code)
		}
		snapshot, err := b.r.profileSnapshot(b.ctx, sd)
		if err != nil {
			return nil, err
		}
		if snapshot == nil || len(snapshot.Element) == 0 {
			return nil, fmt.Errorf("%s: the definition of type %s has no snapshot", e.ID, t.Code)
		}
		out = append(out, snapshot)
	}
	return out, nil
}

// renamedChoice reports whether segment names one type of a choice element of parentID, as FHIR
// names a choice's value in an instance ("valueQuantity" for value[x] of type Quantity), and
// returns the choice's name ("value[x]"). It does when the parent has the choice b[x], segment is
// b followed by one of its type codes with the first letter capitalized, and the parent has no
// real element named segment (SubstanceAmount.amountType sits next to amount[x]).
func (b *snapshotBuilder) renamedChoice(parentID, segment string) (string, bool) {
	if b.find(parentID+"."+segment) >= 0 {
		return "", false
	}
	for i := range b.elements {
		e := &b.elements[i]
		rest, ok := strings.CutPrefix(e.ID, parentID+".")
		if !ok || strings.ContainsAny(rest, ".:") || !strings.HasSuffix(rest, "[x]") {
			continue
		}
		if choiceType(e, strings.TrimSuffix(rest, "[x]"), segment) != nil {
			return rest, true
		}
	}
	return "", false
}

// choiceType returns the type of the choice element e (named name[x]) that segment names
// (name followed by the type code, first letter capitalized), or nil.
func choiceType(e *ElementDefinition, name, segment string) *Type {
	suffix, ok := strings.CutPrefix(segment, name)
	if !ok || suffix == "" {
		return nil
	}
	for i := range e.Type {
		code := e.Type[i].Code
		if code != "" && strings.ToUpper(code[:1])+code[1:] == suffix {
			return &e.Type[i]
		}
	}
	return nil
}

// cloneElement returns a copy of e with the given fields set and the named fields removed.
func cloneElement(e *ElementDefinition, set map[string]any, remove ...string) (*ElementDefinition, error) {
	m, err := rawToMap(e.raw)
	if err != nil {
		return nil, err
	}
	if e.raw == nil {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		if m, err = rawToMap(data); err != nil {
			return nil, err
		}
	}
	for k, v := range set {
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		m[k] = data
	}
	for _, k := range remove {
		delete(m, k)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var out ElementDefinition
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	out.raw = data
	return &out, nil
}

// withIDAndPath returns the differential element with the id and path it is placed at.
func withIDAndPath(diff *ElementDefinition, id, path string) (*ElementDefinition, error) {
	if diff.ID == id && diff.Path == path {
		return diff, nil
	}
	return cloneElement(diff, map[string]any{"id": id, "path": path})
}

// deepCopyElements creates an independent copy of a slice of ElementDefinitions
// by round-tripping each element's raw JSON.
func deepCopyElements(src []ElementDefinition) ([]ElementDefinition, error) {
	dst := make([]ElementDefinition, len(src))
	for i, elem := range src {
		if elem.raw == nil {
			// No raw data — shallow copy the struct fields.
			dst[i] = elem
			continue
		}
		// Round-trip through JSON for a true deep copy.
		rawCopy := make(json.RawMessage, len(elem.raw))
		copy(rawCopy, elem.raw)

		if err := json.Unmarshal(rawCopy, &dst[i]); err != nil {
			return nil, err
		}
		dst[i].raw = rawCopy
	}
	return dst, nil
}

// mergeElements merges a differential element onto a base element using raw JSON.
// Override fields are replaced; accumulate fields (constraint, mapping, code, alias) are appended.
func mergeElements(base, diff *ElementDefinition) (*ElementDefinition, error) {
	baseMap, err := rawToMap(base.raw)
	if err != nil {
		return nil, fmt.Errorf("parse base raw: %w", err)
	}

	diffMap, err := rawToMap(diff.raw)
	if err != nil {
		return nil, fmt.Errorf("parse diff raw: %w", err)
	}

	// Merge each field from the differential.
	for key, diffVal := range diffMap {
		if accumulateFields[key] {
			merged, mergeErr := mergeArrayField(baseMap[key], diffVal, key)
			if mergeErr != nil {
				baseMap[key] = diffVal // Fallback to override on merge error.
			} else {
				baseMap[key] = merged
			}
		} else {
			baseMap[key] = diffVal
		}
	}

	// Marshal back to raw JSON.
	mergedRaw, err := json.Marshal(baseMap)
	if err != nil {
		return nil, fmt.Errorf("marshal merged: %w", err)
	}

	// Unmarshal into ElementDefinition struct.
	var result ElementDefinition
	if err := json.Unmarshal(mergedRaw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal merged: %w", err)
	}
	result.raw = mergedRaw

	return &result, nil
}

// rawToMap parses raw JSON into a map of field name → raw value.
func rawToMap(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if raw == nil {
		return make(map[string]json.RawMessage), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// mergeArrayField merges two JSON arrays, deduplicating by a key field
// ("key" for constraints, "identity" for mappings).
func mergeArrayField(baseVal, diffVal json.RawMessage, field string) (json.RawMessage, error) {
	if baseVal == nil {
		return diffVal, nil
	}

	var baseArr []json.RawMessage
	if err := json.Unmarshal(baseVal, &baseArr); err != nil {
		return diffVal, nil //nolint:nilerr // Base not an array — override with diff.
	}

	var diffArr []json.RawMessage
	if err := json.Unmarshal(diffVal, &diffArr); err != nil {
		return diffVal, nil //nolint:nilerr // Diff not an array — override.
	}

	// Determine dedup key based on field name.
	dedupKey := deduplicationKey(field)

	if dedupKey == "" {
		// No dedup — just append.
		result := make([]json.RawMessage, 0, len(baseArr)+len(diffArr))
		result = append(result, baseArr...)
		result = append(result, diffArr...)
		return json.Marshal(result)
	}

	// Build a set of existing keys from base.
	existing := make(map[string]bool, len(baseArr))
	for _, item := range baseArr {
		if k := extractJSONStringField(item, dedupKey); k != "" {
			existing[k] = true
		}
	}

	// Append diff items that don't already exist.
	for _, item := range diffArr {
		k := extractJSONStringField(item, dedupKey)
		if k == "" || !existing[k] {
			baseArr = append(baseArr, item)
			if k != "" {
				existing[k] = true
			}
		}
	}

	return json.Marshal(baseArr)
}

// deduplicationKey returns the JSON field name used to deduplicate array elements.
func deduplicationKey(field string) string {
	switch field {
	case "constraint":
		return "key"
	case "mapping":
		return "identity"
	default:
		return "" // code, alias — no dedup, just append.
	}
}

// extractJSONStringField extracts a string field from a raw JSON object.
func extractJSONStringField(raw json.RawMessage, field string) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	val, ok := obj[field]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(val, &s); err != nil {
		return ""
	}
	return s
}
