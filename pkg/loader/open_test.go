package loader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// writeExamples writes a package's example folder: its files and, unless index is nil, its
// .index.json.
func writeExamples(t *testing.T, cache, id string, files map[string]string, index any) {
	t.Helper()
	dir := filepath.Join(cache, id, "package", "example")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
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

// A package's example folder contributes its definitions as the package folder does: its
// StructureDefinitions read, its ValueSets and CodeSystems deferred, nothing else; from its index,
// or, without one, from its files.
func TestOpenPackageExamples(t *testing.T) {
	examples := func() map[string]string {
		return map[string]string{
			"StructureDefinition-e.json": `{"resourceType":"StructureDefinition","id":"e","url":"http://example.org/sd/e"}`,
			"CodeSystem-ec.json":         `{"resourceType":"CodeSystem","id":"ec","url":"http://example.org/cs/ec"}`,
			"Patient-y.json":             `{"resourceType":"Patient","id":"y"}`,
		}
	}
	index := map[string]any{"index-version": 2, "files": []map[string]string{
		{"filename": "StructureDefinition-e.json", "resourceType": "StructureDefinition", "id": "e", "url": "http://example.org/sd/e"},
		{"filename": "CodeSystem-ec.json", "resourceType": "CodeSystem", "id": "ec", "url": "http://example.org/cs/ec"},
		{"filename": "Patient-y.json", "resourceType": "Patient", "id": "y"},
	}}
	root := map[string]any{"index-version": 2, "files": []map[string]string{}}
	for _, tt := range []struct {
		name  string
		index any
	}{{"indexed", index}, {"not indexed", nil}} {
		t.Run(tt.name, func(t *testing.T) {
			cache := t.TempDir()
			writePackage(t, cache, "acme#1.0.0", map[string]string{}, root)
			writeExamples(t, cache, "acme#1.0.0", examples(), tt.index)
			pkg, err := NewLoader(cache).OpenPackage("acme", "1.0.0")
			if err != nil {
				t.Fatal(err)
			}
			var sd, cs, other int
			for k := range pkg.Resources {
				switch {
				case strings.Contains(k, "StructureDefinition"):
					sd++
				case strings.Contains(k, "CodeSystem"):
					cs++
				default:
					other++
				}
			}
			for _, d := range pkg.Deferred {
				if d.ResourceType == "CodeSystem" {
					cs++
				}
			}
			if sd != 1 || cs != 1 || other != 0 {
				t.Errorf("examples: %d StructureDefinitions, %d CodeSystems, %d others (%v)", sd, cs, other, keys(pkg.Resources))
			}
		})
	}
}

// A package in a .tgz contributes its package folder's resources and its example folder's
// definitions, as a package of the cache does, and no other folder's.
func TestTgzExamples(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range map[string]string{
		"package/package.json":                       `{"name":"acme","version":"1.0.0","fhirVersions":["4.0.1"]}`,
		"package/CodeSystem-c.json":                  `{"resourceType":"CodeSystem","id":"c","url":"http://example.org/cs/c"}`,
		"package/example/CodeSystem-ec.json":         `{"resourceType":"CodeSystem","id":"ec","url":"http://example.org/cs/ec"}`,
		"package/example/StructureDefinition-e.json": `{"resourceType":"StructureDefinition","id":"e","url":"http://example.org/sd/e"}`,
		"package/example/Patient-y.json":             `{"resourceType":"Patient","id":"y"}`,
		"package/other/ValueSet-o.json":              `{"resourceType":"ValueSet","id":"o","url":"http://example.org/vs/o"}`,
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	pkg, err := NewLoader(t.TempDir()).LoadFromTgzData(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	got := keys(pkg.Resources)
	slices.Sort(got)
	want := []string{"CodeSystem/c", "example/CodeSystem/ec", "example/StructureDefinition/e", "http://example.org/cs/c"}
	if !slices.Equal(got, want) {
		t.Errorf("resources %v, want %v", got, want)
	}
}
