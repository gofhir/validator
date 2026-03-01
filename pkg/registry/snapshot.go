package registry

import (
	"context"
	"encoding/json"
	"fmt"
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
func (r *Registry) EnsureSnapshot(ctx context.Context, sd *StructureDefinition) error {
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
	if err := r.EnsureSnapshot(ctx, baseSD); err != nil {
		return fmt.Errorf("cannot generate snapshot for %s: base snapshot failed: %w", sd.URL, err)
	}

	if baseSD.Snapshot == nil {
		return fmt.Errorf("cannot generate snapshot for %s: base %s has no snapshot", sd.URL, sd.BaseDefinition)
	}

	snapshot, err := applyDifferential(baseSD.Snapshot, sd.Differential)
	if err != nil {
		return fmt.Errorf("cannot generate snapshot for %s: %w", sd.URL, err)
	}

	sd.Snapshot = snapshot
	return nil
}

// applyDifferential merges differential elements onto a deep copy of the base snapshot.
func applyDifferential(base *Snapshot, diff *Differential) (*Snapshot, error) {
	// Deep copy base snapshot elements.
	elements, err := deepCopyElements(base.Element)
	if err != nil {
		return nil, fmt.Errorf("deep copy failed: %w", err)
	}

	for _, diffElem := range diff.Element {
		idx := findMatchingElement(elements, diffElem.Path, diffElem.SliceName)
		if idx >= 0 {
			// Merge diff onto existing base element.
			merged, mergeErr := mergeElements(&elements[idx], &diffElem)
			if mergeErr != nil {
				return nil, fmt.Errorf("merge failed for %s: %w", diffElem.Path, mergeErr)
			}
			elements[idx] = *merged
		} else {
			// New element (slice, extension, etc.) — insert at correct position.
			newElem, mergeErr := buildNewElement(&diffElem)
			if mergeErr != nil {
				return nil, fmt.Errorf("build failed for %s: %w", diffElem.Path, mergeErr)
			}
			insertIdx := findInsertionPoint(elements, diffElem.Path)
			elements = insertElement(elements, insertIdx, *newElem)
		}
	}

	return &Snapshot{Element: elements}, nil
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

// findMatchingElement returns the index of the element in the snapshot that
// matches the given path and optional sliceName. Returns -1 if not found.
func findMatchingElement(elements []ElementDefinition, path string, sliceName *string) int {
	for i, elem := range elements {
		if elem.Path != path {
			continue
		}
		// If the differential element has a sliceName, it must match.
		if sliceName != nil {
			if elem.SliceName == nil || *elem.SliceName != *sliceName {
				continue
			}
		} else if elem.SliceName != nil {
			// Base element has a sliceName but diff doesn't — skip (slice vs. unsliced).
			continue
		}
		return i
	}
	return -1
}

// findInsertionPoint returns the index at which a new element should be inserted.
// It finds the last element that shares the same parent path or is a sibling.
func findInsertionPoint(elements []ElementDefinition, path string) int {
	parentPath := parentOf(path)

	// Find the last element that is a child of the same parent.
	lastSibling := -1
	for i, elem := range elements {
		if elem.Path == parentPath || strings.HasPrefix(elem.Path, parentPath+".") {
			lastSibling = i
		}
	}

	if lastSibling >= 0 {
		return lastSibling + 1
	}

	// Fallback: append at end.
	return len(elements)
}

// parentOf returns the parent path. E.g., "Patient.name.given" → "Patient.name".
func parentOf(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return path
	}
	return path[:idx]
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

// buildNewElement creates an ElementDefinition from a differential-only element.
func buildNewElement(diff *ElementDefinition) (*ElementDefinition, error) {
	if diff.raw != nil {
		rawCopy := make(json.RawMessage, len(diff.raw))
		copy(rawCopy, diff.raw)

		var result ElementDefinition
		if err := json.Unmarshal(rawCopy, &result); err != nil {
			return nil, err
		}
		result.raw = rawCopy
		return &result, nil
	}
	// No raw — return a struct copy.
	elem := *diff
	return &elem, nil
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

// insertElement inserts an element at the given index.
func insertElement(elements []ElementDefinition, idx int, elem ElementDefinition) []ElementDefinition {
	if idx >= len(elements) {
		return append(elements, elem)
	}
	elements = append(elements, ElementDefinition{})
	copy(elements[idx+1:], elements[idx:])
	elements[idx] = elem
	return elements
}
