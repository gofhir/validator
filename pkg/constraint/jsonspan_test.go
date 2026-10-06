package constraint

import (
	"slices"
	"testing"
)

// The JSON of an object's values and an array's items is read as written: a decimal keeps its
// text, a string its quotes and escapes, every position of an array counts.
func TestJSONSpans(t *testing.T) {
	obj := objectSpans([]byte(` { "a" : 1.50 , "b":"x\"}" ,"c":[1.0, null ,{"d":[2]}], "a":2.50, "e\u0066":true} `))
	want := map[string]string{"a": "2.50", "b": `"x\"}"`, "c": `[1.0, null ,{"d":[2]}]`, "ef": "true"}
	if len(obj) != len(want) {
		t.Fatalf("objectSpans: %q", obj)
	}
	for k, v := range want {
		if string(obj[k]) != v {
			t.Errorf("objectSpans[%q] = %q, want %q (a key written twice keeps its last value)", k, obj[k], v)
		}
	}
	spans := arraySpans(obj["c"])
	items := make([]string, 0, len(spans))
	for _, item := range spans {
		items = append(items, string(item))
	}
	if want := []string{"1.0", "null", `{"d":[2]}`}; !slices.Equal(items, want) {
		t.Errorf("arraySpans = %q, want %q", items, want)
	}
	for _, bad := range []string{``, `[1]`, `{"a":}`, `{"a":1`, `{"a" 1}`} {
		if objectSpans([]byte(bad)) != nil {
			t.Errorf("objectSpans(%q) is not nil", bad)
		}
	}
	if arraySpans([]byte(`{}`)) != nil || len(arraySpans([]byte(`[]`))) != 0 {
		t.Error("arraySpans of an object or an empty array")
	}
}
