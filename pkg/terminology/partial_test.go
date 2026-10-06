package terminology

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// A CodeSystem that does not include all its codes cannot tell a code is not one of them: the code
// is unresolved, not invalid, and a ValueSet that includes the whole system accepts it, unchecked.
func TestPartialCodeSystems(t *testing.T) {
	resources := map[string]json.RawMessage{}
	for _, content := range []string{"complete", "not-present", "fragment", "example"} {
		resources["cs-"+content] = json.RawMessage(`{"resourceType":"CodeSystem","url":"http://example.org/cs/` + content +
			`","version":"1","content":"` + content + `","concept":[{"code":"a"}]}`)
		resources["vs-"+content] = json.RawMessage(`{"resourceType":"ValueSet","url":"http://example.org/vs/` + content +
			`","compose":{"include":[{"system":"http://example.org/cs/` + content + `"}]}}`)
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{{Name: "test", Version: "1", Resources: resources}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, tt := range []struct {
		content string
		want    Resolution
		partial bool
	}{
		{"complete", Invalid, false},
		{"not-present", Unresolved, true},
		{"fragment", Unresolved, true},
		{"example", Unresolved, true},
	} {
		system := "http://example.org/cs/" + tt.content
		res, err := r.ResolveCodeInCodeSystem(ctx, system, "z", LookupOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Resolution != tt.want || (res.Partial != nil) != tt.partial {
			t.Errorf("%s: code z: %v, partial %v; want %v, %v", tt.content, res.Resolution, res.Partial, tt.want, tt.partial)
		}
		if tt.partial && (res.Partial.Content != tt.content || res.Partial.Version != "1") {
			t.Errorf("%s: partial = %+v", tt.content, res.Partial)
		}
		if res, _ := r.ResolveCodeInCodeSystem(ctx, system, "a", LookupOptions{}); res.Resolution != Valid {
			t.Errorf("%s: code a: %v, want valid", tt.content, res.Resolution)
		}

		vs, err := r.ResolveCodeInValueSet(ctx, system, "z", "http://example.org/vs/"+tt.content, LookupOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if accepted := vs.Resolution == Valid && vs.Assumed; accepted != tt.partial {
			t.Errorf("%s: code z in the ValueSet: %v (assumed %v)", tt.content, vs.Resolution, vs.Assumed)
		}
	}
}

// A configured provider decides the codes a CodeSystem that does not include all of them lacks.
func TestPartialCodeSystemAsksTheProvider(t *testing.T) {
	const system = "http://example.org/cs/np"
	r := NewRegistry()
	resources := map[string]json.RawMessage{"cs": json.RawMessage(`{"resourceType":"CodeSystem","url":"` + system + `","content":"not-present"}`)}
	if err := r.LoadFromPackages([]*loader.Package{{Name: "test", Version: "1", Resources: resources}}); err != nil {
		t.Fatal(err)
	}
	r.SetProvider(&mockProvider{validateCodeFn: func(_ context.Context, _, code string) (bool, error) { return code == "known", nil }})
	for code, want := range map[string]Resolution{"known": Valid, "other": Invalid} {
		res, err := r.ResolveCodeInCodeSystem(context.Background(), system, code, LookupOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Resolution != want || res.Partial != nil {
			t.Errorf("%s: %v, partial %v; want %v", code, res.Resolution, res.Partial, want)
		}
	}
}

// An external system's CodeSystem stub (SNOMED CT is not-present in the R4 core package) leaves the
// code unresolved as an external system's, not as a CodeSystem that lacks codes.
func TestExternalSystemStubIsNotPartial(t *testing.T) {
	const snomed = "http://snomed.info/sct"
	r := NewRegistry()
	resources := map[string]json.RawMessage{"cs": json.RawMessage(`{"resourceType":"CodeSystem","url":"` + snomed + `","content":"not-present"}`)}
	if err := r.LoadFromPackages([]*loader.Package{{Name: "test", Version: "1", Resources: resources}}); err != nil {
		t.Fatal(err)
	}
	res, err := r.ResolveCodeInCodeSystem(context.Background(), snomed, "12345", LookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != Unresolved || res.Partial != nil {
		t.Errorf("SNOMED: %v, partial %v; want unresolved, no partial", res.Resolution, res.Partial)
	}
}
