package constraint

import (
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// El escalar de un primitivo se toma de su gemelo exacto (json.html#primitive: 1.50 no es 1.5), en una propiedad
// y en el elemento de un arreglo; sin gemelo, es el decodificado.
func TestExactScalar(t *testing.T) {
	parent := map[string]any{"v": 1.5, "vs": []any{2.5, 1.5}}
	twin := map[string]any{"v": json.Number("1.50"), "vs": []any{json.Number("2.50"), json.Number("1.500")}}
	opts := &constraintEvalOpts{exact: func(m map[string]any) map[string]any {
		if m == nil {
			return nil
		}
		return twin
	}}
	for _, tc := range []struct {
		name string
		prim registry.PrimitiveValue
		opts *constraintEvalOpts
		want string
	}{
		{"property", registry.PrimitiveValue{Value: 1.5, Parent: parent, Key: "v", Index: -1}, opts, "1.50"},
		{"item 0", registry.PrimitiveValue{Value: 2.5, Parent: parent, Key: "vs", Index: 0}, opts, "2.50"},
		{"item 1", registry.PrimitiveValue{Value: 1.5, Parent: parent, Key: "vs", Index: 1}, opts, "1.500"},
		{"item out of range", registry.PrimitiveValue{Value: 1.5, Parent: parent, Key: "vs", Index: 9}, opts, "1.5"},
		{"no twin function", registry.PrimitiveValue{Value: 1.5, Parent: parent, Key: "v", Index: -1}, &constraintEvalOpts{}, "1.5"},
		{"parent without twin", registry.PrimitiveValue{Value: 1.5, Parent: nil, Key: "v", Index: -1}, opts, "1.5"},
	} {
		b, err := json.Marshal(exactScalar(tc.prim, tc.opts))
		if err != nil || string(b) != tc.want {
			t.Errorf("%s: %s, %v; want %s", tc.name, b, err, tc.want)
		}
	}
}
