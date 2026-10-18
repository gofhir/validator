package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// DeferredResource is a resource of a package that is read only when it is needed: its
// identifying elements come from the package's index (.index.json), and Read returns its JSON.
type DeferredResource struct {
	ResourceType string
	ID           string
	URL          string
	Version      string
	Read         func() ([]byte, error)
}

// Each calls fn once for each resource held in Resources, which keys a resource both by its URL
// and by "resourceType/id", and an example folder's by "example/resourceType/id"; the package
// folder's first, in the order of their keys.
func (p *Package) Each(fn func(data json.RawMessage)) {
	p.EachResource(func(data json.RawMessage, _ bool) { fn(data) })
}

// EachResource calls fn once for each resource held in Resources, telling whether it is one of the
// package's example folder: the package folder's first, in the order of their keys.
func (p *Package) EachResource(fn func(data json.RawMessage, example bool)) {
	keys := make([]string, 0, len(p.Resources))
	for k := range p.Resources {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		ea, eb := strings.HasPrefix(a, exampleFolder+"/"), strings.HasPrefix(b, exampleFolder+"/")
		if ea != eb {
			if ea {
				return 1
			}
			return -1
		}
		return strings.Compare(a, b)
	})
	seen := make(map[*byte]bool, len(p.Resources))
	for _, k := range keys {
		data := p.Resources[k]
		if len(data) == 0 || seen[&data[0]] {
			continue
		}
		seen[&data[0]] = true
		fn(data, strings.HasPrefix(k, exampleFolder+"/"))
	}
}

// deferredTypes are the resource types a package opened with OpenPackage defers: the validator's
// terminology reads a ValueSet, CodeSystem or NamingSystem when a validation needs it.
// StructureDefinitions are read at once, as the registry indexes them by type; other resource types
// are not read.
var deferredTypes = map[string]bool{"ValueSet": true, "CodeSystem": true, "NamingSystem": true}

// packageIndex is a package's .index.json, version 2 of the NPM package specification's index.
type packageIndex struct {
	IndexVersion int `json:"index-version"`
	Files        []struct {
		Filename     string `json:"filename"`
		ResourceType string `json:"resourceType"`
		ID           string `json:"id"`
		URL          string `json:"url"`
		Version      string `json:"version"`
	} `json:"files"`
}

// OpenPackage opens a package of the cache for validation: from its index (.index.json), it reads
// its StructureDefinitions, and defers its ValueSets, CodeSystems and NamingSystems (Deferred), which are read
// when needed; it reads no other resource. Its example folder is opened the same way. A package
// without a usable index is loaded whole, as LoadPackage does.
func (l *Loader) OpenPackage(name, version string) (*Package, error) {
	if err := checkPackageID(name, version); err != nil {
		return nil, err
	}
	dir := filepath.Join(l.packageDir(name, version), "package")
	data, err := os.ReadFile(filepath.Join(dir, indexFile))
	if err != nil {
		return l.LoadPackage(name, version)
	}
	var index packageIndex
	if err := json.Unmarshal(data, &index); err != nil || index.IndexVersion != 2 {
		return l.LoadPackage(name, version)
	}
	manifest, err := l.Manifest(name, version)
	if err != nil {
		return nil, err
	}
	pkg := &Package{
		Name:         name,
		Version:      version,
		Path:         l.packageDir(name, version),
		FHIRVersion:  manifest.FHIRVersion,
		FHIRVersions: manifest.FHIRVersions,
		Type:         manifest.Type,
		Canonical:    manifest.Canonical,
		Dependencies: manifest.Dependencies,
		Resources:    make(map[string]json.RawMessage),
	}
	if err := pkg.openIndexed(dir, index, ""); err != nil {
		return nil, err
	}
	// The examples are the package's content too (packages.html: "example resources in
	// /package/example", indexed as the package folder is): their definitions are opened as the
	// package's are, as the HL7 validator loads them.
	exampleDir := filepath.Join(dir, exampleFolder)
	if data, err := os.ReadFile(filepath.Join(exampleDir, indexFile)); err == nil {
		var exampleIndex packageIndex
		if json.Unmarshal(data, &exampleIndex) == nil && exampleIndex.IndexVersion == 2 {
			if err := pkg.openIndexed(exampleDir, exampleIndex, exampleFolder+"/"); err != nil {
				return nil, err
			}
			return pkg, nil
		}
	}
	pkg.loadExamples(exampleDir)
	return pkg, nil
}

// exampleFolder is the package folder the NPM package specification puts example resources in, and
// indexFile the index of a folder's resources.
const (
	exampleFolder = "example"
	indexFile     = ".index.json"
)

// openIndexed opens the files of dir that index names: StructureDefinitions read at once, ValueSets,
// CodeSystems and NamingSystems deferred. Prefix keys the StructureDefinitions apart from another folder's.
func (p *Package) openIndexed(dir string, index packageIndex, prefix string) error {
	for _, f := range index.Files {
		if f.Filename == "" || filepath.Base(f.Filename) != f.Filename {
			continue // the index names a file outside the folder
		}
		path := filepath.Join(dir, f.Filename)
		switch {
		case f.ResourceType == "StructureDefinition":
			data, err := os.ReadFile(path) //nolint:gosec // G304: a file of the package directory, named by its index
			if err != nil {
				return fmt.Errorf("package %s#%s: %w", p.Name, p.Version, err)
			}
			p.Resources[prefix+f.ResourceType+"/"+f.ID] = data
		case deferredTypes[f.ResourceType]:
			p.Deferred = append(p.Deferred, DeferredResource{
				ResourceType: f.ResourceType, ID: f.ID, URL: f.URL, Version: f.Version,
				Read: func() ([]byte, error) { return os.ReadFile(path) }, //nolint:gosec // G304: as above
			})
		}
	}
	return nil
}

// loadExamples reads the definitions of a package's example folder that has no usable index: its
// StructureDefinitions, ValueSets and CodeSystems, as a package opened from its index has them. A
// package with no example folder has none.
func (p *Package) loadExamples(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // no example folder: nothing to load
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == indexFile {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // G304: a file of the package's example folder
		if err != nil {
			continue
		}
		p.addExample(data)
	}
}

// exampleTypes are the resource types a package's examples contribute: the definitions the
// validator reads from the package folder (OpenPackage).
var exampleTypes = map[string]bool{"StructureDefinition": true, "ValueSet": true, "CodeSystem": true, "NamingSystem": true}

// addExample adds data, a resource of the example folder, when it is one of exampleTypes, keyed
// apart from the package folder's resources.
func (p *Package) addExample(data []byte) {
	var resource struct {
		ResourceType string `json:"resourceType"`
		ID           string `json:"id"`
	}
	if json.Unmarshal(data, &resource) != nil || !exampleTypes[resource.ResourceType] || resource.ID == "" {
		return
	}
	p.Resources[exampleFolder+"/"+resource.ResourceType+"/"+resource.ID] = data
}
