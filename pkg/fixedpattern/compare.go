package fixedpattern

import (
	"encoding/json"
	"reflect"
)

// DeepEqual compares two JSON values for exact equality.
// Used for validating fixed[x] constraints where values must match exactly.
func DeepEqual(actual, expected json.RawMessage) bool {
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	var a, e any
	if err := json.Unmarshal(actual, &a); err != nil {
		return false
	}
	if err := json.Unmarshal(expected, &e); err != nil {
		return false
	}

	return reflect.DeepEqual(normalizeJSON(a), normalizeJSON(e))
}

// ContainsPattern checks if the actual value contains/matches the pattern.
// Used for validating pattern[x] constraints where:
// - For primitives: values must be equal.
// - For objects: all properties in pattern must exist and match in actual.
// - For arrays: all items in pattern must be found in actual (order independent).
func ContainsPattern(actual, pattern json.RawMessage) bool {
	if pattern == nil {
		return true // No pattern = always matches
	}
	if actual == nil {
		return false // Pattern exists but actual is nil
	}

	var a, p any
	if err := json.Unmarshal(actual, &a); err != nil {
		return false
	}
	if err := json.Unmarshal(pattern, &p); err != nil {
		return false
	}

	return matchRecursive(normalizeJSON(a), normalizeJSON(p))
}

// matchRecursive performs recursive pattern matching.
func matchRecursive(actual, pattern any) bool {
	switch p := pattern.(type) {
	case map[string]any:
		// Pattern is an object - actual must be an object with all pattern properties
		a, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, pval := range p {
			aval, exists := a[key]
			if !exists {
				return false
			}
			if !matchRecursive(aval, pval) {
				return false
			}
		}
		return true

	case []any:
		// Pattern is an array - each pattern item must be found in actual
		a, ok := actual.([]any)
		if !ok {
			return false
		}
		for _, pitem := range p {
			found := false
			for _, aitem := range a {
				if matchRecursive(aitem, pitem) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true

	default:
		// Primitive value - must be equal
		return reflect.DeepEqual(actual, pattern)
	}
}

// normalizeJSON normalizes JSON values for comparison.
// Converts all numbers to float64 (JSON standard) and ensures consistent types.
func normalizeJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			result[k] = normalizeJSON(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = normalizeJSON(v)
		}
		return result
	case float64:
		// JSON numbers are always float64 after unmarshaling
		return val
	case int:
		// Convert to float64 for consistency
		return float64(val)
	case int64:
		return float64(val)
	default:
		return val
	}
}
