package terminology

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// A NamingSystem of kind codesystem names a code system by its preferred uri unique id, read the
// first time asked from a package opened from its index; one of another kind, an alias, or an id of
// another type, does not.
func TestNamesCodeSystem(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "hl7.terminology.r4#1.0.0", "package")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"package.json": `{"name":"hl7.terminology.r4","version":"1.0.0","fhirVersions":["4.0.1"]}`,
		".index.json": `{"index-version":2,"files":[
			{"filename":"NamingSystem-a.json","resourceType":"NamingSystem","id":"a"},
			{"filename":"NamingSystem-b.json","resourceType":"NamingSystem","id":"b"}]}`,
		"NamingSystem-a.json": `{"resourceType":"NamingSystem","id":"a","kind":"codesystem","uniqueId":[
			{"type":"uri","value":"http://acme.local/sid/a","preferred":true},{"type":"uri","value":"http://acme.local/sid/alias"},
			{"type":"oid","value":"1.2.3","preferred":true}]}`,
		"NamingSystem-b.json": `{"resourceType":"NamingSystem","id":"b","kind":"identifier","uniqueId":[
			{"type":"uri","value":"http://acme.local/sid/b","preferred":true}]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := loader.NewLoader(cache).OpenPackage("hl7.terminology.r4", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	for uri, want := range map[string]bool{"http://acme.local/sid/a": true, "http://acme.local/sid/alias": false, "1.2.3": false, "http://acme.local/sid/b": false} {
		if got := r.NamesCodeSystem(uri); got != want {
			t.Errorf("NamesCodeSystem(%q) = %v, want %v", uri, got, want)
		}
	}
	if !r.Clone().NamesCodeSystem("http://acme.local/sid/a") {
		t.Error("a clone does not name the code system")
	}
}

// Another package's NamingSystem does not make a system one HL7 publishes.
func TestNamesCodeSystemOtherPackage(t *testing.T) {
	pkg := &loader.Package{Name: "acme", Version: "1.0.0", Resources: map[string]json.RawMessage{
		"NamingSystem/a": json.RawMessage(`{"resourceType":"NamingSystem","id":"a","kind":"codesystem","uniqueId":[{"type":"uri","value":"http://hl7.org/fhir/sid/acme-fake","preferred":true}]}`),
	}}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	if r.NamesCodeSystem("http://hl7.org/fhir/sid/acme-fake") {
		t.Error("a NamingSystem of acme names the code system, want HL7's terminology package's only")
	}
}
