package constraint

import (
	"bytes"
	"encoding/json"
)

// The JSON of the values an object or an array holds, as slices of the JSON they are written in:
// nothing is copied or decoded but the keys that hold an escape.

// objectSpans are the JSON of the values the object raw holds, by key; nil when raw is not an
// object. A key written twice keeps its last value, as encoding/json keeps it.
func objectSpans(raw []byte) map[string][]byte {
	i := skipSpace(raw, 0)
	if i >= len(raw) || raw[i] != '{' {
		return nil
	}
	spans := map[string][]byte{}
	i = skipSpace(raw, i+1)
	if i < len(raw) && raw[i] == '}' {
		return spans
	}
	for i < len(raw) {
		keyEnd, ok := skipString(raw, i)
		if !ok {
			return nil
		}
		key, ok := jsonKey(raw[i:keyEnd])
		if !ok {
			return nil
		}
		i = skipSpace(raw, keyEnd)
		if i >= len(raw) || raw[i] != ':' {
			return nil
		}
		start := skipSpace(raw, i+1)
		end, ok := skipValue(raw, start)
		if !ok {
			return nil
		}
		spans[key] = raw[start:end]
		i = skipSpace(raw, end)
		switch {
		case i < len(raw) && raw[i] == ',':
			i = skipSpace(raw, i+1)
		case i < len(raw) && raw[i] == '}':
			return spans
		default:
			return nil
		}
	}
	return nil
}

// arraySpans are the JSON of the items of the array raw, one per position, nulls included; nil
// when raw is not an array.
func arraySpans(raw []byte) [][]byte {
	i := skipSpace(raw, 0)
	if i >= len(raw) || raw[i] != '[' {
		return nil
	}
	var items [][]byte
	i = skipSpace(raw, i+1)
	if i < len(raw) && raw[i] == ']' {
		return [][]byte{}
	}
	for i < len(raw) {
		end, ok := skipValue(raw, i)
		if !ok {
			return nil
		}
		items = append(items, raw[i:end])
		i = skipSpace(raw, end)
		switch {
		case i < len(raw) && raw[i] == ',':
			i = skipSpace(raw, i+1)
		case i < len(raw) && raw[i] == ']':
			return items
		default:
			return nil
		}
	}
	return nil
}

// jsonKey is the key a JSON string is, decoded only when it holds an escape.
func jsonKey(quoted []byte) (string, bool) {
	if bytes.IndexByte(quoted, '\\') < 0 {
		return string(quoted[1 : len(quoted)-1]), true
	}
	var key string
	if json.Unmarshal(quoted, &key) != nil {
		return "", false
	}
	return key, true
}

// skipValue is where the JSON value that starts at i ends.
func skipValue(b []byte, i int) (int, bool) {
	if i >= len(b) {
		return i, false
	}
	switch b[i] {
	case '"':
		return skipString(b, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(b); j++ {
			switch b[j] {
			case '"':
				end, ok := skipString(b, j)
				if !ok {
					return j, false
				}
				j = end - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return j + 1, true
				}
			}
		}
		return len(b), false
	default: // a number, true, false or null
		j := i
		for j < len(b) && !isDelimiter(b[j]) {
			j++
		}
		return j, j > i
	}
}

// skipString is where the JSON string that starts at i, at its opening quote, ends: past its
// closing quote.
func skipString(b []byte, i int) (int, bool) {
	if i >= len(b) || b[i] != '"' {
		return i, false
	}
	for j := i + 1; j < len(b); j++ {
		switch b[j] {
		case '\\':
			j++
		case '"':
			return j + 1, true
		}
	}
	return len(b), false
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	return i
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isDelimiter(c byte) bool { return isSpace(c) || c == ',' || c == '}' || c == ']' }
