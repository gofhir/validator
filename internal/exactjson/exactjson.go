// Package exactjson gives, for an object of a parsed resource, the same object decoded with its
// numbers as the JSON spells them (json.Number): a decimal keeps its precision (1.50 is not 1.5,
// json.html#primitive), which a parse into float64 does not.
package exactjson

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sync"
)

// Index pairs the objects of a parsed resource with their exact twins. The JSON is decoded again
// the first time a twin is asked for, so that a validation that needs none pays nothing. It is safe
// for concurrent use.
type Index struct {
	parsed map[string]any
	raw    []byte

	once  sync.Once
	twins map[uintptr]map[string]any
}

// New returns the index of parsed, decoded from raw.
func New(parsed map[string]any, raw []byte) *Index {
	return &Index{parsed: parsed, raw: raw}
}

// Of is the exact twin of m, an object of the parsed resource, or nil when it has none (an index of
// nothing, JSON that does not decode, m not from the resource).
func (x *Index) Of(m map[string]any) map[string]any {
	if x == nil || m == nil {
		return nil
	}
	x.once.Do(x.pairAll)
	return x.twins[reflect.ValueOf(m).Pointer()]
}

func (x *Index) pairAll() {
	decoder := json.NewDecoder(bytes.NewReader(x.raw))
	decoder.UseNumber()
	var exact map[string]any
	if decoder.Decode(&exact) != nil {
		return
	}
	x.twins = map[uintptr]map[string]any{}
	x.pair(x.parsed, exact)
}

// pair records the exact twin of each object of parsed, decoded from the same JSON as exact.
func (x *Index) pair(parsed, exact any) {
	switch p := parsed.(type) {
	case map[string]any:
		e, ok := exact.(map[string]any)
		if !ok {
			return
		}
		x.twins[reflect.ValueOf(p).Pointer()] = e
		for k, v := range p {
			x.pair(v, e[k])
		}
	case []any:
		e, ok := exact.([]any)
		if !ok || len(e) != len(p) {
			return
		}
		for i := range p {
			x.pair(p[i], e[i])
		}
	}
}
