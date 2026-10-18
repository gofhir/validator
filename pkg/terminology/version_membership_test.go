package terminology

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// versionRegistry holds kv at 1.0.0, known with no version, v2 at url|2.7 (as R4's v2 tables), and
// value sets over kv.
func versionRegistry(t *testing.T) *Registry {
	t.Helper()
	pkg := &loader.Package{Name: "acme", Version: "1.0.0", Resources: map[string]json.RawMessage{
		"CodeSystem/kv":        json.RawMessage(`{"resourceType":"CodeSystem","id":"kv","url":"http://acme.local/cs/kv","version":"1.0.0","content":"complete","concept":[{"code":"a"}]}`),
		"CodeSystem/known":     json.RawMessage(`{"resourceType":"CodeSystem","id":"known","url":"http://acme.local/cs/known","content":"complete","concept":[{"code":"a"}]}`),
		"CodeSystem/v2":        json.RawMessage(`{"resourceType":"CodeSystem","id":"v2","url":"http://acme.local/cs/v2|2.7","version":"0360","content":"complete","concept":[{"code":"a"}]}`),
		"ValueSet/whole":       json.RawMessage(`{"resourceType":"ValueSet","id":"whole","url":"http://acme.local/vs/whole","compose":{"include":[{"system":"http://acme.local/cs/kv"}]}}`),
		"ValueSet/enum":        json.RawMessage(`{"resourceType":"ValueSet","id":"enum","url":"http://acme.local/vs/enum","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"zz"}]}]}}`),
		"ValueSet/pin":         json.RawMessage(`{"resourceType":"ValueSet","id":"pin","url":"http://acme.local/vs/pin","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"1.0.0"}]}}`),
		"ValueSet/other":       json.RawMessage(`{"resourceType":"ValueSet","id":"other","url":"http://acme.local/vs/other","compose":{"include":[{"system":"http://acme.local/cs/known"}]}}`),
		"ValueSet/nested":      json.RawMessage(`{"resourceType":"ValueSet","id":"nested","url":"http://acme.local/vs/nested","compose":{"include":[{"valueSet":["http://acme.local/vs/whole"]}]}}`),
		"ValueSet/cmod":        json.RawMessage(`{"resourceType":"ValueSet","id":"cmod","url":"http://acme.local/vs/cmod","compose":{"include":[{"system":"http://acme.local/cs/kv","concept":[{"code":"zz","modifierExtension":[{"url":"http://acme.local/mod","valueBoolean":true}]}]}]}}`),
		"ValueSet/enumnest":    json.RawMessage(`{"resourceType":"ValueSet","id":"enumnest","url":"http://acme.local/vs/enumnest","compose":{"include":[{"valueSet":["http://acme.local/vs/other"]},{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"zz"}]}]}}`),
		"ValueSet/star":        json.RawMessage(`{"resourceType":"ValueSet","id":"star","url":"http://acme.local/vs/star","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"*","concept":[{"code":"a"}]}]}}`),
		"ValueSet/excall":      json.RawMessage(`{"resourceType":"ValueSet","id":"excall","url":"http://acme.local/vs/excall","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}]}],"exclude":[{"system":"http://acme.local/cs/kv"}]}}`),
		"ValueSet/excother":    json.RawMessage(`{"resourceType":"ValueSet","id":"excother","url":"http://acme.local/vs/excother","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}]}],"exclude":[{"system":"http://acme.local/cs/kv","concept":[{"code":"b"}]}]}}`),
		"ValueSet/excvs":       json.RawMessage(`{"resourceType":"ValueSet","id":"excvs","url":"http://acme.local/vs/excvs","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}]}],"exclude":[{"valueSet":["http://acme.local/vs/whole"]}]}}`),
		"ValueSet/locked":      json.RawMessage(`{"resourceType":"ValueSet","id":"locked","url":"http://acme.local/vs/locked","compose":{"lockedDate":"2020-01-01","include":[{"system":"http://acme.local/cs/kv","concept":[{"code":"a"}]}]}}`),
		"ValueSet/nestenum":    json.RawMessage(`{"resourceType":"ValueSet","id":"nestenum","url":"http://acme.local/vs/nestenum","compose":{"include":[{"valueSet":["http://acme.local/vs/enum"]}]}}`),
		"ValueSet/inter":       json.RawMessage(`{"resourceType":"ValueSet","id":"inter","url":"http://acme.local/vs/inter","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"zz"}],"valueSet":["http://acme.local/vs/other"]}]}}`),
		"ValueSet/incmod":      json.RawMessage(`{"resourceType":"ValueSet","id":"incmod","url":"http://acme.local/vs/incmod","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}],"modifierExtension":[{"url":"http://acme.local/mod","valueBoolean":true}]}]}}`),
		"ValueSet/cyc1":        json.RawMessage(`{"resourceType":"ValueSet","id":"cyc1","url":"http://acme.local/vs/cyc1","compose":{"include":[{"valueSet":["http://acme.local/vs/cyc2"]}]}}`),
		"ValueSet/cyc2":        json.RawMessage(`{"resourceType":"ValueSet","id":"cyc2","url":"http://acme.local/vs/cyc2","compose":{"include":[{"valueSet":["http://acme.local/vs/cyc1"]}]}}`),
		"ValueSet/lockpinexcw": json.RawMessage(`{"resourceType":"ValueSet","id":"lockpinexcw","url":"http://acme.local/vs/lockpinexcw","compose":{"lockedDate":"2020-01-01","include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}]}],"exclude":[{"system":"http://acme.local/cs/kv"}]}}`),
		"ValueSet/samex":       json.RawMessage(`{"resourceType":"ValueSet","id":"samex","url":"http://acme.local/vs/samex","compose":{"include":[{"valueSet":["http://acme.local/vs/whole"]}],"exclude":[{"valueSet":["http://acme.local/vs/whole"]}]}}`),
		"ValueSet/enumnov":     json.RawMessage(`{"resourceType":"ValueSet","id":"enumnov","url":"http://acme.local/vs/enumnov","compose":{"include":[{"system":"http://acme.local/cs/kv","concept":[{"code":"a"}]}]}}`),
		"ValueSet/locknest":    json.RawMessage(`{"resourceType":"ValueSet","id":"locknest","url":"http://acme.local/vs/locknest","compose":{"lockedDate":"2020-01-01","include":[{"valueSet":["http://acme.local/vs/enumnov"]}]}}`),
		"ValueSet/intwe":       json.RawMessage(`{"resourceType":"ValueSet","id":"intwe","url":"http://acme.local/vs/intwe","compose":{"include":[{"system":"http://acme.local/cs/kv","valueSet":["http://acme.local/vs/enum"]}]}}`),
		"ValueSet/a2":          json.RawMessage(`{"resourceType":"ValueSet","id":"a2","url":"http://acme.local/vs/a2","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}]},{"valueSet":["http://acme.local/vs/b2"]}]}}`),
		"ValueSet/b2":          json.RawMessage(`{"resourceType":"ValueSet","id":"b2","url":"http://acme.local/vs/b2","compose":{"include":[{"valueSet":["http://acme.local/vs/a2"]}]}}`),
		"ValueSet/cycord1":     json.RawMessage(`{"resourceType":"ValueSet","id":"cycord1","url":"http://acme.local/vs/cycord1","compose":{"include":[{"valueSet":["http://acme.local/vs/a2","http://acme.local/vs/other"]},{"valueSet":["http://acme.local/vs/b2"]}]}}`),
		"ValueSet/cycord2":     json.RawMessage(`{"resourceType":"ValueSet","id":"cycord2","url":"http://acme.local/vs/cycord2","compose":{"include":[{"valueSet":["http://acme.local/vs/b2"]},{"valueSet":["http://acme.local/vs/a2","http://acme.local/vs/other"]}]}}`),
		"ValueSet/lockmix":     json.RawMessage(`{"resourceType":"ValueSet","id":"lockmix","url":"http://acme.local/vs/lockmix","compose":{"include":[{"valueSet":["http://acme.local/vs/locknest"]},{"valueSet":["http://acme.local/vs/enumnov"]}]}}`),
		"ValueSet/exc":         json.RawMessage(`{"resourceType":"ValueSet","id":"exc","url":"http://acme.local/vs/exc","compose":{"include":[{"system":"http://acme.local/cs/kv","concept":[{"code":"zz"}]}],"exclude":[{"system":"http://acme.local/cs/kv","concept":[{"code":"zz"}]}]}}`),
	}}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	return r
}

// A code of a version of its code system that is not loaded is in a value set as its composition
// tells: listed by an include of that version, not admitted by any include, or not known.
func TestVersionMembership(t *testing.T) {
	r := versionRegistry(t)
	for _, tt := range []struct {
		vs, code string
		want     Resolution
	}{
		{"http://acme.local/vs/enum", "zz", Valid},
		{"http://acme.local/vs/enum", "a", Invalid},
		{"http://acme.local/vs/pin", "a", Invalid},
		{"http://acme.local/vs/other", "a", Invalid},
		{"http://acme.local/vs/whole", "a", Unresolved},
		{"http://acme.local/vs/nested", "a", Unresolved},
		{"http://acme.local/vs/exc", "zz", Invalid},
		{"http://acme.local/vs/star", "a", Valid},
		{"http://acme.local/vs/excall", "a", Invalid},
		{"http://acme.local/vs/excother", "a", Valid},
		{"http://acme.local/vs/excvs", "a", Invalid},
		{"http://acme.local/vs/incmod", "a", Unresolved},
		{"http://acme.local/vs/cyc1", "a", Unresolved},
		{"http://acme.local/vs/locked", "a", Unresolved},
		{"http://acme.local/vs/locked", "q", Invalid},
		{"http://acme.local/vs/lockpinexcw", "a", Unresolved},
		{"http://acme.local/vs/samex", "a", Invalid},
		{"http://acme.local/vs/enumnov", "a", Valid},
		{"http://acme.local/vs/locknest", "a", Unresolved},
		{"http://acme.local/vs/lockmix", "a", Valid},
		{"http://acme.local/vs/intwe", "zz", Valid},
		{"http://acme.local/vs/cycord1", "a", Valid},
		{"http://acme.local/vs/cycord2", "a", Valid},
		{"http://acme.local/vs/nestenum", "zz", Valid},
		{"http://acme.local/vs/nestenum", "a", Invalid},
		{"http://acme.local/vs/inter", "zz", Invalid},
		{"http://acme.local/vs/cmod", "zz", Unresolved},
		{"http://acme.local/vs/enumnest", "zz", Valid},
		{"http://acme.local/vs/enumnest", "a", Invalid},
		{"http://acme.local/vs/missing", "a", Unresolved},
	} {
		if got := r.VersionMembership(tt.vs, "http://acme.local/cs/kv", "9.9.9", tt.code); got != tt.want {
			t.Errorf("%s %s: got %v, want %v", tt.vs, tt.code, got, tt.want)
		}
	}
}

// A version is unknown where the code system is loaded in other versions only (or with none), a
// url|version url counting as that version; not where no CodeSystem has the url, nor where an
// authoritative port decides lookups.
func TestCodeSystemVersionUnknown(t *testing.T) {
	r := versionRegistry(t)
	for _, tt := range []struct {
		url, version string
		want         bool
	}{
		{"http://acme.local/cs/kv", "1.0.0", false},
		{"http://acme.local/cs/kv", "9.9.9", true},
		{"http://acme.local/cs/kv", "", false},
		{"http://acme.local/cs/known", "1", true},
		{"http://acme.local/cs/v2", "2.7", false},
		{"http://acme.local/cs/v2", "2.8", true},
		{"http://acme.local/cs/none", "1", false},
	} {
		if got := r.CodeSystemVersionUnknown(tt.url, tt.version); got != tt.want {
			t.Errorf("%s|%s: got %v, want %v", tt.url, tt.version, got, tt.want)
		}
	}
	if got := r.CodeSystemVersions("http://acme.local/cs/v2"); !slices.Equal(got, []string{"2.7"}) {
		t.Errorf("versions of v2: %q, want 2.7", got)
	}
	r.SetAuthority(&chainAuthority{})
	if r.CodeSystemVersionUnknown("http://acme.local/cs/kv", "9.9.9") {
		t.Error("unknown under an authoritative port, want its lookup to decide")
	}
}

// Value sets that take the same value sets in by many paths are decided once each: thirty levels of
// two value sets each taking both of the next in are decided at once, not in 2^30 steps, a cycle back
// to the root and a value set taking itself in too.
func TestVersionMembershipDiamond(t *testing.T) {
	for name, end := range map[string]string{
		"acyclic": "",
		"cycle":   `,{"valueSet":["http://acme.local/vs/end"]},{"valueSet":["http://acme.local/vs/v0"]}`,
	} {
		t.Run(name, func(t *testing.T) { diamond(t, end) })
	}
}

// diamond decides a code at the root of thirty levels of two value sets each taking both of the next
// in, the last listing it, and taking in what extra names.
func diamond(t *testing.T, extra string) {
	t.Helper()
	const levels = 30
	resources := map[string]json.RawMessage{
		"ValueSet/end": json.RawMessage(`{"resourceType":"ValueSet","id":"end","url":"http://acme.local/vs/end","compose":{"include":[{"system":"http://acme.local/cs/kv","version":"9.9.9","concept":[{"code":"a"}]}` + extra + `]}}`),
	}
	url := func(side string, i int) string {
		if i == levels {
			return "http://acme.local/vs/end"
		}
		return fmt.Sprintf("http://acme.local/vs/%s%d", side, i)
	}
	for i := range levels {
		for _, side := range []string{"v", "w"} {
			resources[fmt.Sprintf("ValueSet/%s%d", side, i)] = json.RawMessage(fmt.Sprintf(
				`{"resourceType":"ValueSet","id":"%s%d","url":%q,"compose":{"include":[{"valueSet":[%q]},{"valueSet":[%q]}]}}`,
				side, i, url(side, i), url("v", i+1), url("w", i+1)))
		}
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{{Name: "acme", Version: "1.0.0", Resources: resources}}); err != nil {
		t.Fatal(err)
	}
	if got := r.VersionMembership(url("v", 0), "http://acme.local/cs/kv", "9.9.9", "a"); got != Valid {
		t.Errorf("got %v, want valid", got)
	}
}
