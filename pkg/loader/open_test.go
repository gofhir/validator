package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writePackage writes a package of the cache: its manifest, its files and, unless index is nil,
// its .index.json.
func writePackage(t *testing.T, cache, id string, files map[string]string, index any) {
	t.Helper()
	dir := filepath.Join(cache, id, "package")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	files["package.json"] = `{"name":"acme","version":"1.0.0","type":"IG","canonical":"http://example.org","fhirVersions":["4.0.1"]}`
	if index != nil {
		data, _ := json.Marshal(index)
		files[".index.json"] = string(data)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// With an index, a package's StructureDefinitions are read, its ValueSets and CodeSystems deferred,
// and nothing else read; without one, it is loaded whole.
func TestOpenPackage(t *testing.T) {
	files := map[string]string{
		"StructureDefinition-p.json": `{"resourceType":"StructureDefinition","id":"p","url":"http://example.org/sd/p"}`,
		"ValueSet-v.json":            `{"resourceType":"ValueSet","id":"v","url":"http://example.org/vs/v","version":"2"}`,
		"CodeSystem-c.json":          `{"resourceType":"CodeSystem","id":"c","url":"http://example.org/cs/c"}`,
		"Patient-x.json":             `{"resourceType":"Patient","id":"x"}`,
	}
	index := map[string]any{"index-version": 2, "files": []map[string]string{
		{"filename": "StructureDefinition-p.json", "resourceType": "StructureDefinition", "id": "p", "url": "http://example.org/sd/p"},
		{"filename": "ValueSet-v.json", "resourceType": "ValueSet", "id": "v", "url": "http://example.org/vs/v", "version": "2"},
		{"filename": "CodeSystem-c.json", "resourceType": "CodeSystem", "id": "c", "url": "http://example.org/cs/c"},
		{"filename": "Patient-x.json", "resourceType": "Patient", "id": "x"},
		{"filename": "../outside.json", "resourceType": "ValueSet", "id": "o", "url": "http://example.org/vs/o"},
	}}
	cache := t.TempDir()
	copied := map[string]string{}
	for k, v := range files {
		copied[k] = v
	}
	writePackage(t, cache, "acme#1.0.0", copied, index)
	pkg, err := NewLoader(cache).OpenPackage("acme", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Canonical != "http://example.org" || len(pkg.FHIRVersions) != 1 {
		t.Errorf("manifest: %+v", pkg)
	}
	if len(pkg.Resources) != 1 || pkg.Resources["StructureDefinition/p"] == nil {
		t.Errorf("resources read: %v", keys(pkg.Resources))
	}
	if len(pkg.Deferred) != 2 {
		t.Fatalf("deferred: %+v", pkg.Deferred)
	}
	for _, d := range pkg.Deferred {
		data, err := d.Read()
		if err != nil || len(data) == 0 {
			t.Errorf("%s: %v", d.URL, err)
		}
		if d.URL == "http://example.org/vs/v" && d.Version != "2" {
			t.Errorf("version %q", d.Version)
		}
	}

	// Without an index: whole, as LoadPackage.
	cache = t.TempDir()
	writePackage(t, cache, "acme#1.0.0", files, nil)
	if pkg, err = NewLoader(cache).OpenPackage("acme", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if len(pkg.Deferred) != 0 || pkg.Resources["Patient/x"] == nil {
		t.Errorf("without an index: resources %v, deferred %d", keys(pkg.Resources), len(pkg.Deferred))
	}
}

// Each visits a resource once, although Resources keys it twice.
func TestEach(t *testing.T) {
	data := json.RawMessage(`{"resourceType":"ValueSet","id":"v","url":"http://example.org/vs/v"}`)
	pkg := &Package{Resources: map[string]json.RawMessage{"http://example.org/vs/v": data, "ValueSet/v": data,
		"ValueSet/w": json.RawMessage(`{"resourceType":"ValueSet","id":"w"}`)}}
	n := 0
	pkg.Each(func(json.RawMessage) { n++ })
	if n != 2 {
		t.Errorf("visited %d, want 2", n)
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
