package elementvalues

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gofhir/validator/pkg/registry"
)

// node is an element of the given id and types, as a tree would hold it.
func node(id string, types ...string) *registry.ElementNode {
	def := &registry.ElementDefinition{ID: id, Path: id}
	for _, t := range types {
		def.Type = append(def.Type, registry.Type{Code: t})
	}
	return &registry.ElementNode{Def: def}
}

// choiceType names the types this test knows, as Registry.ChoiceType does: the property is the
// element name followed by the type with its first letter capitalized.
func choiceType(base, key string) string {
	suffixes := map[string]string{"Quantity": "Quantity", "String": "string", "Period": "Period"}
	if len(key) > len(base) && key[:len(base)] == base {
		return suffixes[key[len(base):]]
	}
	return ""
}

// render is a value as "path type value ext".
func render(v Value) string {
	val, _ := json.Marshal(v.Value)
	ext, _ := json.Marshal(v.Ext)
	return fmt.Sprintf("%s %s %s %s", v.Path("P"), v.TypeCode, val, ext)
}

func TestOf(t *testing.T) {
	for _, tt := range []struct {
		name string
		node *registry.ElementNode
		inst string
		want []string
	}{
		{"a single value", node("P.gender", "code"), `{"gender":"male"}`,
			[]string{`P.gender code "male" null`}},
		{"absent", node("P.gender", "code"), `{}`, nil},
		{"an array", node("P.name", "HumanName"), `{"name":[{"family":"A"},{"family":"B"}]}`,
			[]string{`P.name[0] HumanName {"family":"A"} null`, `P.name[1] HumanName {"family":"B"} null`}},
		{"a primitive with its extensions", node("P.birthDate", "date"), `{"birthDate":"2020","_birthDate":{"id":"x"}}`,
			[]string{`P.birthDate date "2020" {"id":"x"}`}},
		{"a primitive present only through its extensions", node("P.birthDate", "date"), `{"_birthDate":{"id":"x"}}`,
			[]string{`P.birthDate date null {"id":"x"}`}},
		{"a repeating primitive, extensions aligned by index", node("P.given", "string"), `{"given":["a","b"],"_given":[null,{"id":"y"}]}`,
			[]string{`P.given[0] string "a" null`, `P.given[1] string "b" {"id":"y"}`}},
		{"a repeating primitive present only through its extensions", node("P.given", "string"), `{"_given":[{"id":"x"},{"id":"y"}]}`,
			[]string{`P.given[0] string null {"id":"x"}`, `P.given[1] string null {"id":"y"}`}},
		{"a choice of a type it allows", node("O.value[x]", "Quantity", "string"), `{"valueQuantity":{"value":1}}`,
			[]string{`P.valueQuantity Quantity {"value":1} null`}},
		{"a primitive choice present only through its extensions", node("O.value[x]", "Quantity", "string"), `{"_valueString":{"id":"x"}}`,
			[]string{`P.valueString string null {"id":"x"}`}},
		{"a choice of a type it does not allow is present, with that type", node("O.value[x]", "Quantity"), `{"valuePeriod":{}}`,
			[]string{`P.valuePeriod Period {} null`}},
		{"declared types first, then others by name", node("O.value[x]", "string"), `{"valuePeriod":{},"valueQuantity":{},"valueString":"s"}`,
			[]string{`P.valueString string "s" null`, `P.valuePeriod Period {} null`, `P.valueQuantity Quantity {} null`}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var inst map[string]any
			if err := json.Unmarshal([]byte(tt.inst), &inst); err != nil {
				t.Fatal(err)
			}
			values := Of(tt.node, inst, choiceType)
			got := make([]string, 0, len(values))
			for _, v := range values {
				got = append(got, render(v))
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}
