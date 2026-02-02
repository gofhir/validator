// Package location provides utilities to find line and column positions
// in JSON source for FHIRPath expressions.
package location

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Location represents a position in the source JSON.
type Location struct {
	Line   int
	Column int
}

// Find locates the position of a FHIRPath expression in JSON source.
// Returns nil if the path cannot be found.
func Find(jsonData []byte, fhirPath string) *Location {
	if len(jsonData) == 0 || fhirPath == "" {
		return nil
	}

	segments := parseFHIRPath(fhirPath)
	if len(segments) == 0 {
		return nil
	}

	dec := json.NewDecoder(strings.NewReader(string(jsonData)))

	offset, err := navigateToPath(dec, segments)
	if err != nil {
		return nil
	}

	line, col := offsetToLineCol(jsonData, offset)
	return &Location{Line: line, Column: col}
}

// parseFHIRPath parses a FHIRPath expression into path segments.
// Examples:
//   - "Patient.identifier[0].value" -> ["identifier", "0", "value"]
//   - "Bundle.entry[0].resource.id" -> ["entry", "0", "resource", "id"]
func parseFHIRPath(path string) []string {
	// Remove resource type prefix if present (Patient.identifier -> identifier)
	if idx := strings.Index(path, "."); idx > 0 {
		first := path[:idx]
		// Check if first segment looks like a resource type (starts with uppercase)
		if first != "" && first[0] >= 'A' && first[0] <= 'Z' {
			path = path[idx+1:]
		}
	}

	var segments []string
	current := ""

	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch ch {
		case '.':
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
		case '[':
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
			// Read array index
			j := i + 1
			for j < len(path) && path[j] != ']' {
				j++
			}
			if j > i+1 {
				segments = append(segments, path[i+1:j])
			}
			i = j
		default:
			current += string(ch)
		}
	}
	if current != "" {
		segments = append(segments, current)
	}

	return segments
}

// navigateToPath navigates through JSON to find the target path.
func navigateToPath(dec *json.Decoder, segments []string) (int, error) {
	segIdx := 0

	for segIdx < len(segments) {
		target := segments[segIdx]

		// Check if target is array index
		if idx, err := strconv.Atoi(target); err == nil {
			// Navigate to array index
			offset, err := navigateToArrayIndex(dec, idx)
			if err != nil {
				return 0, err
			}
			segIdx++
			if segIdx == len(segments) {
				return offset, nil
			}
		} else {
			// Navigate to object key
			offset, err := navigateToKey(dec, target)
			if err != nil {
				return 0, err
			}
			segIdx++
			if segIdx == len(segments) {
				return offset, nil
			}
		}
	}

	return 0, fmt.Errorf("path not found")
}

// navigateToKey finds a key in the current JSON object.
func navigateToKey(dec *json.Decoder, key string) (int, error) {
	for {
		offset := int(dec.InputOffset())
		tok, err := dec.Token()
		if err != nil {
			return 0, fmt.Errorf("key %q not found: %w", key, err)
		}

		// Looking for the key
		if k, ok := tok.(string); ok && k == key {
			return offset, nil
		}

		// Handle delimiters
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				// Enter object, continue searching
			case '[':
				// Skip the array
				if err := skipRest(dec, '['); err != nil {
					return 0, err
				}
			case '}', ']':
				return 0, fmt.Errorf("key %q not found in object", key)
			}
		}
	}
}

// navigateToArrayIndex navigates to a specific index in a JSON array.
func navigateToArrayIndex(dec *json.Decoder, targetIdx int) (int, error) {
	// We should be at the start of an array value
	tok, err := dec.Token()
	if err != nil {
		return 0, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return 0, fmt.Errorf("expected array, got %v", tok)
	}

	idx := 0
	for dec.More() {
		offset := int(dec.InputOffset())
		if idx == targetIdx {
			return offset, nil
		}
		// Skip this element
		if err := skipValue(dec); err != nil {
			return 0, err
		}
		idx++
	}

	// Consume closing ]
	if _, err := dec.Token(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("array index %d out of bounds (size %d)", targetIdx, idx)
}

// skipValue skips a single JSON value (primitive, object, or array).
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}

	if delim, ok := tok.(json.Delim); ok {
		return skipRest(dec, delim)
	}
	return nil
}

// skipRest skips the rest of an object or array after the opening delimiter.
func skipRest(dec *json.Decoder, _ json.Delim) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

// offsetToLineCol converts a byte offset to line and column numbers.
// Line and column are 1-indexed (human-readable).
func offsetToLineCol(input []byte, offset int) (line, col int) {
	line = 1
	col = 1
	for i := 0; i < offset && i < len(input); i++ {
		if input[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return
}

// EnrichIssues adds Location information to issues based on their Expression.
// The jsonData is the original JSON source, issues are modified in place.
func EnrichIssues(jsonData []byte, issues []interface {
	GetExpression() []string
	SetLocation(line, col int)
}) {
	for _, issue := range issues {
		exprs := issue.GetExpression()
		if len(exprs) > 0 {
			if loc := Find(jsonData, exprs[0]); loc != nil {
				issue.SetLocation(loc.Line, loc.Column)
			}
		}
	}
}
