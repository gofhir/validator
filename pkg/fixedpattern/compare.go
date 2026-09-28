package fixedpattern

import (
	"encoding/json"

	"github.com/gofhir/validator/pkg/jsoncompare"
)

// DeepEqual compares two JSON values for exact equality.
// Used for validating fixed[x] constraints where values must match exactly.
//
// It is kept for API compatibility; the implementation lives in [jsoncompare.DeepEqual].
func DeepEqual(actual, expected json.RawMessage) bool {
	return jsoncompare.DeepEqual(actual, expected)
}

// ContainsPattern checks if the actual value contains/matches the pattern.
// Used for validating pattern[x] constraints.
//
// It is kept for API compatibility; the implementation lives in [jsoncompare.ContainsPattern].
func ContainsPattern(actual, pattern json.RawMessage) bool {
	return jsoncompare.ContainsPattern(actual, pattern)
}
