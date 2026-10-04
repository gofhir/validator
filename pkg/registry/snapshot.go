package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// EnsureSnapshot generates a snapshot for the given StructureDefinition if it
// only has a differential. It resolves the base chain, deep-copies the base
// snapshot, and merges the differential on top.
// If the SD already has a snapshot, this is a no-op.
//
// Safe for concurrent use: a per-SD mutex serializes lazy snapshot generation,
// so multiple goroutines validating the same differential-only profile observe
// a single, consistent snapshot. Different SDs can be generated in parallel,
// and the per-SD lock keeps the recursive call to the base SD deadlock-free.
func (r *Registry) EnsureSnapshot(ctx context.Context, sd *StructureDefinition) error {
	sd.snapshotMu.Lock()
	defer sd.snapshotMu.Unlock()

	if sd.Snapshot != nil {
		return nil
	}

	if sd.Differential == nil || len(sd.Differential.Element) == 0 {
		return fmt.Errorf("cannot generate snapshot for %s: no differential", sd.URL)
	}
	if sd.BaseDefinition == "" {
		return fmt.Errorf("cannot generate snapshot for %s: no baseDefinition", sd.URL)
	}

	// Resolve the base SD.
	baseSD := r.ResolveByCanonical(ctx, sd.BaseDefinition, "")
	if baseSD == nil {
		return fmt.Errorf("cannot generate snapshot for %s: base %s not found", sd.URL, sd.BaseDefinition)
	}

	// Recursively ensure the base has a snapshot (handles chained profiles).
	// Each SD has its own mutex, so recursing into the base does not deadlock.
	if err := r.EnsureSnapshot(ctx, baseSD); err != nil {
		return fmt.Errorf("cannot generate snapshot for %s: base snapshot failed: %w", sd.URL, err)
	}

	if baseSD.Snapshot == nil {
		return fmt.Errorf("cannot generate snapshot for %s: base %s has no snapshot", sd.URL, sd.BaseDefinition)
	}

	snapshot, err := r.applyDifferential(ctx, baseSD.Snapshot, sd.Differential)
	if err != nil {
		return fmt.Errorf("cannot generate snapshot for %s: %w", sd.URL, err)
	}

	// The merge rebuilds a changed element from its base's published JSON, so the corrections are
	// applied again, for the profile's FHIR version, or its base's when it declares none.
	version := sd.FHIRVersion
	if version == "" {
		version = baseSD.FHIRVersion
	}
	correctElements(version, snapshot.Element)

	sd.Snapshot = snapshot
	return nil
}

// snapshotBuilder holds a snapshot being generated.
type snapshotBuilder struct {
	r        *Registry
	ctx      context.Context
	elements []ElementDefinition
	// constrained holds the ids the differential has constrained so far.
	constrained map[string]bool
}

// applyDifferential merges differential elements onto a deep copy of the base snapshot, as
// profiling.html describes snapshot generation. Each differential element is placed by its id: the
// element of the base with that id, or one created for it. A slice is created as a copy of the
// element it slices (its types, cardinality and base), and the children of an element the base
// does not expand are unrolled from its type, the profile its type declares or its
// contentReference. Matching by path instead would let a slice's child overwrite the base element
// (Extension.extension:a.url and Extension.extension.url share a path).
func (r *Registry) applyDifferential(ctx context.Context, base *Snapshot, diff *Differential) (*Snapshot, error) {
	elements, err := deepCopyElements(base.Element)
	if err != nil {
		return nil, fmt.Errorf("deep copy failed: %w", err)
	}
	b := &snapshotBuilder{r: r, ctx: ctx, elements: elements, constrained: map[string]bool{}}

	ids := differentialIDs(diff.Element)
	for i := range diff.Element {
		diffElem := diff.Element[i]
		idx, id, err := b.ensure(ids[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ids[i], err)
		}
		// The differential's own id and path may be a renamed choice's ("Observation.valueQuantity"):
		// the element keeps the id and path it is placed at.
		renamed, err := withIDAndPath(&diffElem, id, b.elements[idx].Path)
		if err != nil {
			return nil, err
		}
		merged, err := mergeElements(&b.elements[idx], renamed)
		if err != nil {
			return nil, fmt.Errorf("merge failed for %s: %w", id, err)
		}
		b.elements[idx] = *merged
		b.constrained[id] = true
		if err := b.typeSliceOfChoice(idx, renamed); err != nil {
			return nil, err
		}
	}
	if err := b.slicesRaiseMin(); err != nil {
		return nil, err
	}
	return &Snapshot{Element: b.elements}, nil
}

// slicesRaiseMin gives an extension element the differential slices without defining its slicing
// the slicing every extension element has ("Extensions are always sliced by url",
// profiling.html), and raises its minimum to the sum of its slices' minimums when they require more
// (MeasureReport.extension with a required slice is 1..*), as the HL7 validator generates it. A
// slicing the differential or the base defines is left as it is.
func (b *snapshotBuilder) slicesRaiseMin() error {
	for i := range b.elements {
		sliced := &b.elements[i]
		if sliced.Slicing != nil || b.constrained[sliced.ID] || len(sliced.Type) != 1 || sliced.Type[0].Code != "Extension" {
			continue
		}
		var sum uint32
		found := false
		for j := range b.elements {
			if slice, ok := strings.CutPrefix(b.elements[j].ID, sliced.ID+":"); ok && !strings.ContainsAny(slice, ".:/") {
				sum += b.elements[j].Min
				found = true
			}
		}
		if !found {
			continue
		}
		set := map[string]any{"slicing": Slicing{
			Discriminator: []Discriminator{{Type: "value", Path: "url"}}, Rules: "open",
		}}
		if sum > sliced.Min {
			set["min"] = sum
		}
		implied, err := cloneElement(sliced, set)
		if err != nil {
			return err
		}
		b.elements[i] = *implied
	}
	return nil
}

// typeSliceOfChoice completes a choice's type slice ("Observation.value[x]:valueQuantity", which a
// differential may write "Observation.valueQuantity": a type slice, profiling.html) once the
// differential is merged into it. The choice is sliced by type ($this) when it is not sliced yet.
// When the differential types the slice or requires it (min 1 or more), the choice can only take
// that type: it is restricted to it, its slicing closed, and the cardinality the differential sets
// on the slice is the choice's too. This is how the HL7 validator generates them.
func (b *snapshotBuilder) typeSliceOfChoice(idx int, diff *ElementDefinition) error {
	e := &b.elements[idx]
	if e.SliceName == nil || !strings.HasSuffix(e.Path, "[x]") || len(e.Type) != 1 {
		return nil
	}
	dot := strings.LastIndexByte(e.ID, '.')
	name, slice, _ := strings.Cut(e.ID[dot+1:], ":")
	if choiceType(e, strings.TrimSuffix(name, "[x]"), slice) == nil {
		return nil // not a type slice
	}
	ci := b.find(e.ID[:dot+1] + name)
	if ci < 0 {
		return nil
	}
	m, err := rawToMap(diff.raw)
	if err != nil {
		return err
	}
	choice := &b.elements[ci]
	set := map[string]any{}
	slicing := choice.Slicing
	if slicing == nil {
		slicing = &Slicing{Discriminator: []Discriminator{{Type: "type", Path: "$this"}}, Rules: "open"}
	}
	_, typed := m["type"]
	if typed || e.Min >= 1 {
		set["type"] = e.Type
		closed := *slicing
		closed.Rules = "closed"
		slicing = &closed
		if _, ok := m["min"]; ok {
			set["min"] = e.Min
		}
		if _, ok := m["max"]; ok {
			set["max"] = e.Max
		}
	}
	set["slicing"] = slicing
	updated, err := cloneElement(choice, set)
	if err != nil {
		return err
	}
	b.elements[ci] = *updated
	return nil
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

// differentialIDs returns the id of each differential element: its own, or, for a differential
// written without ids, one built from its path and the slices named before it (an element under a
// path a slice was named at belongs to that slice, until an element at that path names no slice).
func differentialIDs(diff []ElementDefinition) []string {
	ids := make([]string, len(diff))
	current := map[string]string{} // path -> the slice named at it last
	for i, e := range diff {
		if e.ID != "" {
			ids[i] = e.ID
			continue
		}
		for p := range current {
			if p == e.Path || strings.HasPrefix(p, e.Path+".") {
				delete(current, p)
			}
		}
		if e.SliceName != nil {
			current[e.Path] = *e.SliceName
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
// "Observation.value[x]:valueQuantity").
func (b *snapshotBuilder) ensure(id string) (idx int, placed string, err error) {
	if idx := b.find(id); idx >= 0 {
		return idx, id, nil
	}
	dot := strings.LastIndexByte(id, '.')
	if dot < 0 {
		return -1, id, errors.New("no element with that id in the base")
	}
	parentID, segment := id[:dot], id[dot+1:]
	if name, slice, sliced := strings.Cut(segment, ":"); sliced {
		return b.ensureSlice(parentID, name, slice)
	}

	parentIdx, parentID, err := b.ensure(parentID)
	if err != nil {
		return -1, id, err
	}
	id = parentID + "." + segment
	if idx := b.find(id); idx >= 0 {
		return idx, id, nil
	}
	if !b.hasChildren(parentID) {
		if err := b.unrollChildren(parentIdx); err != nil {
			return -1, id, err
		}
		if idx := b.find(id); idx >= 0 {
			return idx, id, nil
		}
	}
	if choice, ok := b.renamedChoice(parentID, segment); ok {
		return b.ensureSlice(parentID, choice, segment)
	}
	return -1, id, fmt.Errorf("no element %s in the base", id)
}

// ensureSlice returns the index of the slice of parentID.name named slice, creating it as a copy of
// the element it slices (of the slice it reslices, for "a/b"), placed after that element's
// subtree, its other slices included.
func (b *snapshotBuilder) ensureSlice(parentID, name, slice string) (idx int, placed string, err error) {
	_, parentID, err = b.ensure(parentID)
	if err != nil {
		return -1, "", err
	}
	id := parentID + "." + name + ":" + slice
	if idx := b.find(id); idx >= 0 {
		return idx, id, nil
	}
	slicedID := parentID + "." + name
	if k := strings.LastIndexByte(slice, '/'); k >= 0 {
		slicedID += ":" + slice[:k]
	}
	slicedIdx, slicedID, err := b.ensure(slicedID)
	if err != nil {
		return -1, id, err
	}
	// A slice of an element defined by reference has that element's children, and so has the
	// sliced element, as the HL7 validator generates them.
	if b.elements[slicedIdx].ContentReference != nil && !b.hasChildren(slicedID) {
		if err := b.unrollFrom(slicedIdx, b.find(contentReferenceID(*b.elements[slicedIdx].ContentReference))); err != nil {
			return -1, id, err
		}
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
	// A renamed choice ("valueQuantity") is the slice of the choice for that one type.
	if strings.HasSuffix(name, "[x]") {
		if t := choiceType(sliced, strings.TrimSuffix(name, "[x]"), slice); t != nil {
			if entry, err = cloneElement(entry, map[string]any{"type": []Type{*t}}); err != nil {
				return -1, id, err
			}
		}
	}
	at := b.subtreeEnd(slicedID)
	b.elements = slices.Insert(b.elements, at, *entry)
	return at, id, nil
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
// element it slices, as that element's constraints apply to each slice; when that element has none
// in the snapshot either, they come from where its definition takes them (unroll).
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
		if rest, ok := strings.CutPrefix(b.elements[i].ID, slicedID+"."); ok {
			child, err := cloneElement(&b.elements[i], map[string]any{"id": id + "." + rest})
			if err != nil {
				return err
			}
			children = append(children, *child)
		}
	}
	if len(children) == 0 {
		if si := b.find(slicedID); si >= 0 && b.elements[si].ContentReference != nil {
			return b.unrollFrom(idx, b.find(contentReferenceID(*b.elements[si].ContentReference)))
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

// unrollFrom inserts, after the element at idx, the children of the element at src.
func (b *snapshotBuilder) unrollFrom(idx, src int) error {
	if src < 0 {
		return fmt.Errorf("%s: the element it is defined by is not in the snapshot", b.elements[idx].ID)
	}
	parent, root := b.elements[idx], b.elements[src]
	var children []ElementDefinition
	for i := range b.elements {
		if rest, ok := strings.CutPrefix(b.elements[i].ID, root.ID+"."); ok {
			child, err := cloneElement(&b.elements[i], map[string]any{
				"id":   parent.ID + "." + rest,
				"path": parent.Path + strings.TrimPrefix(b.elements[i].Path, root.Path),
			})
			if err != nil {
				return err
			}
			children = append(children, *child)
		}
	}
	b.elements = slices.Insert(b.elements, idx+1, children...)
	return nil
}

// unroll inserts, after the element at idx, the children its definition takes from elsewhere: the
// element its contentReference names, or its one type (the profile that type declares, when it
// declares exactly one). Each keeps the base its source declares.
func (b *snapshotBuilder) unroll(idx int) error {
	parent := b.elements[idx]
	var source []ElementDefinition
	var rootID, rootPath string
	switch {
	case parent.ContentReference != nil:
		return b.unrollFrom(idx, b.find(contentReferenceID(*parent.ContentReference)))
	default:
		sd, err := b.typeDefinition(&parent)
		if err != nil {
			return err
		}
		root := sd.Snapshot.Element[0]
		rootID, rootPath = root.ID, root.Path
		source = sd.Snapshot.Element[1:]
	}
	children := make([]ElementDefinition, 0, len(source))
	for i := range source {
		child, err := cloneElement(&source[i], map[string]any{
			"id":   parent.ID + strings.TrimPrefix(source[i].ID, rootID),
			"path": parent.Path + strings.TrimPrefix(source[i].Path, rootPath),
		})
		if err != nil {
			return err
		}
		children = append(children, *child)
	}
	b.elements = slices.Insert(b.elements, idx+1, children...)
	return nil
}

// typeDefinition returns the definition an element's children come from: the profile its one type
// declares, when it declares exactly one, or that type's definition.
func (b *snapshotBuilder) typeDefinition(e *ElementDefinition) (*StructureDefinition, error) {
	codes := map[string]bool{}
	for _, t := range e.Type {
		codes[t.Code] = true
	}
	if len(codes) != 1 {
		return nil, fmt.Errorf("%s has %d types: its children cannot be unrolled", e.ID, len(codes))
	}
	t := e.Type[0]
	var sd *StructureDefinition
	if len(e.Type) == 1 && len(t.Profile) == 1 {
		url, version := ParseCanonical(t.Profile[0])
		sd = b.r.ResolveByCanonical(b.ctx, url, version)
	}
	if sd == nil {
		sd = b.r.GetByType(t.Code)
	}
	if sd == nil {
		return nil, fmt.Errorf("%s: no definition of type %s", e.ID, t.Code)
	}
	if err := b.r.EnsureSnapshot(b.ctx, sd); err != nil {
		return nil, err
	}
	if sd.Snapshot == nil || len(sd.Snapshot.Element) == 0 {
		return nil, fmt.Errorf("%s: the definition of type %s has no snapshot", e.ID, t.Code)
	}
	return sd, nil
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
