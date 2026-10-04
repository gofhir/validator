// Package location provides utilities to find line and column positions
// in JSON source for FHIRPath expressions.
package location

import (
	"bytes"
	"encoding/json"
	"slices"
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
	return FindAll(jsonData, []string{fhirPath})[fhirPath]
}

// FindAll locates every FHIRPath expression of paths in JSON source, reading the source once:
// the cost of locating every issue of a validation does not grow with the number of issues
// times the size of the resource. A path that cannot be found has no entry in the result.
//
// A path to a property is located just after its name, and a path to an array item at the item.
func FindAll(jsonData []byte, paths []string) map[string]*Location {
	if len(jsonData) == 0 || len(paths) == 0 {
		return nil
	}
	s := scanner{
		src:      jsonData,
		dec:      json.NewDecoder(bytes.NewReader(jsonData)),
		wanted:   map[string]bool{},
		prefixes: map[string]bool{},
		offsets:  map[string]int{},
	}
	keys := make(map[string]string, len(paths)) // path -> its key
	root := false
	for _, p := range paths {
		if isTypeName(p) {
			keys[p], root = rootKey, true
			continue
		}
		segments := parseFHIRPath(p)
		if len(segments) == 0 {
			continue
		}
		for i, seg := range segments {
			// A slice ("category:VSCat") is located at the element it slices: no property name has
			// a colon.
			segments[i], _, _ = strings.Cut(seg, ":")
		}
		key := strings.Join(segments, segmentSeparator)
		keys[p] = key
		s.wanted[key] = true
		for i := 1; i < len(segments); i++ {
			s.prefixes[strings.Join(segments[:i], segmentSeparator)] = true
		}
	}
	if len(keys) == 0 {
		return nil
	}
	if root {
		// The resource itself is located just after its opening brace, as a property is just after
		// its name.
		if i := bytes.IndexByte(jsonData, '{'); i >= 0 {
			s.offsets[rootKey] = i + 1
		}
	}
	if len(s.wanted) > 0 {
		// A source that ends early keeps what was located before it.
		_ = s.walk()
	}

	located := make(map[string]*Location, len(s.offsets))
	for _, loc := range lineCols(jsonData, s.offsets) {
		located[loc.key] = &Location{Line: loc.line, Column: loc.col}
	}
	out := make(map[string]*Location, len(keys))
	for p, key := range keys {
		if loc := located[key]; loc != nil {
			out[p] = loc
		}
	}
	return out
}

// rootKey is the key of the resource itself, named by its type ("Patient").
const rootKey = ""

// isTypeName reports whether path names a resource by its type alone ("Patient"): a single name
// that starts with an uppercase letter, as FHIR type names do and element names do not.
func isTypeName(path string) bool {
	return path != "" && path[0] >= 'A' && path[0] <= 'Z' && !strings.ContainsAny(path, ".[")
}

// segmentSeparator joins a path's segments into the key it is looked up by. It cannot occur in a
// property name of valid JSON text that names a FHIR element.
const segmentSeparator = "\x00"

// scanner reads the source once, recording the offset of each wanted path and entering only the
// values some wanted path goes through.
type scanner struct {
	src      []byte
	dec      *json.Decoder
	wanted   map[string]bool // keys of the paths to locate
	prefixes map[string]bool // keys of the values a path to locate goes through
	offsets  map[string]int  // key -> offset of each path located
	path     []byte          // key of the value being read
}

// walk reads one value at s.path, recording and entering what lies under it.
func (s *scanner) walk() error {
	tok, err := s.dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		for s.dec.More() {
			tok, err := s.dec.Token()
			if err != nil {
				return err
			}
			name, _ := tok.(string)
			if err := s.member(name, int(s.dec.InputOffset())); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; s.dec.More(); i++ {
			if err := s.member(strconv.Itoa(i), s.itemStart(int(s.dec.InputOffset()))); err != nil {
				return err
			}
		}
	}
	_, err = s.dec.Token() // the closing delimiter
	return err
}

// itemStart is the offset of the array item the decoder is about to read from offset: past the
// comma that separates it from the item before, and the white space around it.
func (s *scanner) itemStart(offset int) int {
	for offset < len(s.src) {
		switch s.src[offset] {
		case ',', ' ', '\t', '\n', '\r':
			offset++
		default:
			return offset
		}
	}
	return offset
}

// member reads the value of one property or array item, segment, located at offset.
func (s *scanner) member(segment string, offset int) error {
	n := len(s.path)
	if n > 0 {
		s.path = append(s.path, segmentSeparator...)
	}
	s.path = append(s.path, segment...)
	defer func() { s.path = s.path[:n] }()
	if s.wanted[string(s.path)] {
		s.offsets[string(s.path)] = offset
	}
	if s.prefixes[string(s.path)] {
		return s.walk()
	}
	return skipValue(s.dec)
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

// lineCol is the line and column of the offset recorded for a key.
type lineCol struct {
	key       string
	line, col int
}

// lineCols converts the offsets recorded for each key to 1-indexed lines and columns, reading the
// source once.
func lineCols(input []byte, offsets map[string]int) []lineCol {
	type at struct {
		key    string
		offset int
	}
	sorted := make([]at, 0, len(offsets))
	for k, o := range offsets {
		sorted = append(sorted, at{k, o})
	}
	slices.SortFunc(sorted, func(a, b at) int { return a.offset - b.offset })

	out := make([]lineCol, 0, len(sorted))
	line, col, i := 1, 1, 0
	for _, a := range sorted {
		for ; i < a.offset && i < len(input); i++ {
			if input[i] == '\n' {
				line, col = line+1, 1
			} else {
				col++
			}
		}
		out = append(out, lineCol{a.key, line, col})
	}
	return out
}

// EnrichIssues adds Location information to issues based on their Expression.
// The jsonData is the original JSON source, issues are modified in place.
func EnrichIssues(jsonData []byte, issues []interface {
	GetExpression() []string
	SetLocation(line, col int)
}) {
	paths := make([]string, 0, len(issues))
	for _, issue := range issues {
		if exprs := issue.GetExpression(); len(exprs) > 0 {
			paths = append(paths, exprs[0])
		}
	}
	located := FindAll(jsonData, paths)
	for _, issue := range issues {
		if exprs := issue.GetExpression(); len(exprs) > 0 {
			if loc := located[exprs[0]]; loc != nil {
				issue.SetLocation(loc.Line, loc.Column)
			}
		}
	}
}
