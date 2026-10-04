package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
// and by "resourceType/id".
func (p *Package) Each(fn func(data json.RawMessage)) {
	seen := make(map[*byte]bool, len(p.Resources))
	for _, data := range p.Resources {
		if len(data) == 0 {
			continue
		}
		if seen[&data[0]] {
			continue
		}
		seen[&data[0]] = true
		fn(data)
	}
}

// deferredTypes are the resource types a package opened with OpenPackage defers: the validator's
// terminology reads a ValueSet or CodeSystem when a validation needs it. StructureDefinitions are
// read at once, as the registry indexes them by type; other resource types are not read.
var deferredTypes = map[string]bool{"ValueSet": true, "CodeSystem": true}

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
// its StructureDefinitions, and defers its ValueSets and CodeSystems (Deferred), which are read
// when needed; it reads no other resource. A package without a usable index is loaded whole, as
// LoadPackage does.
func (l *Loader) OpenPackage(name, version string) (*Package, error) {
	if err := checkPackageID(name, version); err != nil {
		return nil, err
	}
	dir := filepath.Join(l.packageDir(name, version), "package")
	data, err := os.ReadFile(filepath.Join(dir, ".index.json"))
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
	for _, f := range index.Files {
		if f.Filename == "" || filepath.Base(f.Filename) != f.Filename {
			continue // the index names a file outside the package directory
		}
		path := filepath.Join(dir, f.Filename)
		switch {
		case f.ResourceType == "StructureDefinition":
			data, err := os.ReadFile(path) //nolint:gosec // G304: a file of the package directory, named by its index
			if err != nil {
				return nil, fmt.Errorf("package %s#%s: %w", name, version, err)
			}
			pkg.Resources[f.ResourceType+"/"+f.ID] = data
		case deferredTypes[f.ResourceType]:
			pkg.Deferred = append(pkg.Deferred, DeferredResource{
				ResourceType: f.ResourceType, ID: f.ID, URL: f.URL, Version: f.Version,
				Read: func() ([]byte, error) { return os.ReadFile(path) }, //nolint:gosec // G304: as above
			})
		}
	}
	return pkg, nil
}
