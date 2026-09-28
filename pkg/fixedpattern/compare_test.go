package fixedpattern

import (
	"encoding/json"
	"testing"
)

// The comparison itself is tested in jsoncompare; these tests only pin the wrappers to it.
func TestWrappersDelegate(t *testing.T) {
	if !DeepEqual(json.RawMessage(`{"a":1}`), json.RawMessage(`{"a":1.0}`)) {
		t.Error("DeepEqual: equal objects reported different")
	}
	if DeepEqual(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"a":1}`)) {
		t.Error("DeepEqual: an extra property must not be equal")
	}
	if !ContainsPattern(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"a":1}`)) {
		t.Error("ContainsPattern: a contained pattern was not found")
	}
	if ContainsPattern(json.RawMessage(`{"b":2}`), json.RawMessage(`{"a":1}`)) {
		t.Error("ContainsPattern: a missing property matched")
	}
}
