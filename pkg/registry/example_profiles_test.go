package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// A profile of a package's example folder resolves by its url, and is no profile the package
// declares: GetProfilesByPackage, which ValidateWithIG applies, leaves it out.
func TestExampleProfilesAreNotThePackages(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "acme#1.0.0", "package")
	if err := os.MkdirAll(filepath.Join(dir, "example"), 0o750); err != nil {
		t.Fatal(err)
	}
	sd := func(id string) string {
		return `{"resourceType":"StructureDefinition","id":"` + id + `","url":"http://acme.local/sd/` + id +
			`","type":"Patient","kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","fhirVersion":"4.0.1"}`
	}
	for name, content := range map[string]string{
		"package.json":                       `{"name":"acme","version":"1.0.0","fhirVersions":["4.0.1"]}`,
		"StructureDefinition-p.json":         sd("p"),
		"example/StructureDefinition-e.json": sd("e"),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := loader.NewLoader(cache).OpenPackage("acme", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	r := New()
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	if r.GetByURL("http://acme.local/sd/e") == nil {
		t.Error("the example profile does not resolve by its url")
	}
	profiles := r.GetProfilesByPackage("acme#1.0.0")
	if len(profiles) != 1 || profiles[0].URL != "http://acme.local/sd/p" {
		t.Errorf("the package's profiles: %d, want the package folder's only", len(profiles))
	}
}
