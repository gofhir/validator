package structural

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// normalize parses src and normalizes it as a resource of its type.
func normalize(t *testing.T, src string) (map[string]any, *Normalized) {
	t.Helper()
	reg := setupTestRegistry(t)
	var data map[string]any
	if err := json.Unmarshal([]byte(src), &data); err != nil {
		t.Fatal(err)
	}
	rt, _ := data["resourceType"].(string)
	sd := reg.GetByType(rt)
	if sd == nil {
		t.Fatalf("no definition of %s", rt)
	}
	return data, New(reg).Normalize(data, []byte(src), sd)
}

// The resource is left as HL7's parser leaves it, and a path in it is named as the source writes
// it: the items after the dropped ones renumbered back, the one item of a single value written where
// an array goes without an index, a primitive's "_key" items dropped with their values.
func TestNormalizeWritten(t *testing.T) {
	data, n := normalize(t, `{"resourceType":"Bundle","type":"collection","entry":{"resource":{
		"resourceType":"Patient",
		"telecom":[5,{"value":"a"},"x",{"value":"b"}],
		"name":[{"given":["a",{},"b","c"],"_given":[null,null,7,{"id":"g"}]}]}}}`)
	shapes := make([]string, 0, len(n.Issues.Issues))
	for _, is := range n.Issues.Issues {
		shapes = append(shapes, is.MessageID+" @ "+strings.Join(is.Expression, ","))
	}
	slices.Sort(shapes)
	if want := []string{
		"STRUCTURE_ARRAY_EXPECTED @ Bundle.entry",
		"STRUCTURE_OBJECT_EXPECTED @ Bundle.entry.resource.name[0].given",
		"STRUCTURE_OBJECT_EXPECTED @ Bundle.entry.resource.telecom[0]",
		"STRUCTURE_OBJECT_EXPECTED @ Bundle.entry.resource.telecom[2]",
		"STRUCTURE_VALUE_EXPECTED @ Bundle.entry.resource.name[0].given",
	}; !slices.Equal(shapes, want) {
		t.Errorf("issues %q, want %q", shapes, want)
	}

	patient := data["entry"].([]any)[0].(map[string]any)["resource"].(map[string]any)
	name := patient["name"].([]any)[0].(map[string]any)
	if got := name["given"]; !slices.Equal(got.([]any), []any{"a", "c"}) {
		t.Errorf("given %v, want [a c]", got)
	}
	if got := name["_given"].([]any); len(got) != 2 || got[0] != nil {
		t.Errorf("_given %v, want [null {id:g}]", got)
	}
	if got := patient["telecom"].([]any); len(got) != 2 {
		t.Errorf("telecom %v, want its two objects", got)
	}

	for path, want := range map[string]string{
		"Bundle.entry[0].resource.telecom[0]":        "Bundle.entry.resource.telecom[1]",
		"Bundle.entry[0].resource.telecom[1].value":  "Bundle.entry.resource.telecom[3].value",
		"Bundle.entry[0].resource.name[0].given[1]":  "Bundle.entry.resource.name[0].given[3]",
		"Bundle.entry[0].resource.name[0]._given[1]": "Bundle.entry.resource.name[0]._given[3]",
		"Bundle.type": "Bundle.type",
	} {
		if got := n.Written(path); got != want {
			t.Errorf("Written(%s) = %s, want %s", path, got, want)
		}
	}
}

// The JSON of the resource as left spells its numbers as the source does: a decimal's precision is
// its value's.
func TestNormalizeRawKeepsNumbers(t *testing.T) {
	_, n := normalize(t, `{"resourceType":"Observation","status":"final","code":{"text":"x"},
		"valueQuantity":{"value":1.50,"unit":"mg"},"component":[5]}`)
	if !strings.Contains(string(n.Raw), `"value":1.50`) {
		t.Errorf("raw %s does not keep 1.50", n.Raw)
	}
	if strings.Contains(string(n.Raw), "component") {
		t.Errorf("raw %s keeps the array its only item was dropped from", n.Raw)
	}
}
