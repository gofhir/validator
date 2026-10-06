package terminology

import (
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// A clone holds what the registry holds; what is loaded into either afterwards is not in the other.
func TestClone(t *testing.T) {
	const a, b, d = "http://example.org/ValueSet/a", "http://example.org/ValueSet/b", "http://example.org/ValueSet/d"
	vs := func(url string) *loader.Package {
		return &loader.Package{Name: url, Version: "1", Resources: map[string]json.RawMessage{"vs": json.RawMessage(
			`{"resourceType":"ValueSet","url":"` + url + `"}`)}}
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{vs(a)}); err != nil {
		t.Fatal(err)
	}
	c := r.Clone()
	if err := c.LoadFromPackages([]*loader.Package{vs(b)}); err != nil {
		t.Fatal(err)
	}
	if err := r.LoadFromPackages([]*loader.Package{vs(d)}); err != nil {
		t.Fatal(err)
	}
	if c.GetValueSet(a) == nil || c.GetValueSet(b) == nil {
		t.Error("the clone does not hold both")
	}
	if r.GetValueSet(b) != nil {
		t.Error("the registry holds what was loaded into its clone")
	}
	if c.GetValueSet(d) != nil {
		t.Error("the clone holds what was loaded into the registry after cloning")
	}
}
