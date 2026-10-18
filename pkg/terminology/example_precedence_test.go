package terminology

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// A CodeSystem of a package's example folder is one of the package's, chosen by its version as any
// other is, as the HL7 validator chooses it: a higher version in the example folder stands for the
// package folder's.
func TestExampleCodeSystemByVersion(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "acme#1.0.0", "package")
	if err := os.MkdirAll(filepath.Join(dir, "example"), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"package.json":               `{"name":"acme","version":"1.0.0","fhirVersions":["4.0.1"]}`,
		"CodeSystem-x.json":          `{"resourceType":"CodeSystem","id":"x","url":"http://acme.local/cs/x","version":"1.0.0","content":"complete","concept":[{"code":"one"}]}`,
		"example/CodeSystem-xx.json": `{"resourceType":"CodeSystem","id":"xx","url":"http://acme.local/cs/x","version":"2.0.0","content":"complete","concept":[{"code":"two"}]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := loader.NewLoader(cache).OpenPackage("acme", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	if valid, found := r.ValidateCodeInCodeSystemContext(context.Background(), "http://acme.local/cs/x", "two"); !found || !valid {
		t.Errorf("two: valid %v found %v, want the example folder's version 2.0.0", valid, found)
	}
	if valid, _ := r.ValidateCodeInCodeSystemContext(context.Background(), "http://acme.local/cs/x", "one"); valid {
		t.Error("one is valid, want the example folder's version 2.0.0 to stand for 1.0.0")
	}
}

// Between equals the package folder's CodeSystem is used, deferred from its index or not, and the
// example folder's read one is not: the package folder's are loaded first.
func TestExampleCodeSystemAfterDeferred(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "acme#1.0.0", "package")
	if err := os.MkdirAll(filepath.Join(dir, "example"), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"package.json":               `{"name":"acme","version":"1.0.0","fhirVersions":["4.0.1"]}`,
		".index.json":                `{"index-version":2,"files":[{"filename":"CodeSystem-x.json","resourceType":"CodeSystem","id":"x","url":"http://acme.local/cs/x"}]}`,
		"CodeSystem-x.json":          `{"resourceType":"CodeSystem","id":"x","url":"http://acme.local/cs/x","content":"complete","concept":[{"code":"one"}]}`,
		"example/CodeSystem-xx.json": `{"resourceType":"CodeSystem","id":"xx","url":"http://acme.local/cs/x","content":"complete","concept":[{"code":"two"}]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := loader.NewLoader(cache).OpenPackage("acme", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatal(err)
	}
	if valid, found := r.ValidateCodeInCodeSystemContext(context.Background(), "http://acme.local/cs/x", "one"); !found || !valid {
		t.Errorf("one: valid %v found %v, want the package folder's CodeSystem", valid, found)
	}
	if valid, _ := r.ValidateCodeInCodeSystemContext(context.Background(), "http://acme.local/cs/x", "two"); valid {
		t.Error("two is valid, want the package folder's CodeSystem, not the example folder's")
	}
}
