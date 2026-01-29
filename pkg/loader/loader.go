// Package loader handles loading FHIR packages from the NPM cache,
// local .tgz files, or remote URLs.
package loader

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPackagePath returns the default FHIR package cache path.
func DefaultPackagePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fhir", "packages")
}

// PackageRef represents a reference to a FHIR package.
type PackageRef struct {
	Name    string
	Version string
}

// String returns the package spec in "name#version" format.
func (p PackageRef) String() string {
	return fmt.Sprintf("%s#%s", p.Name, p.Version)
}

// Package represents a loaded FHIR package.
type Package struct {
	Name        string
	Version     string
	Path        string
	FHIRVersion string
	Resources   map[string]json.RawMessage // URL or resourceType/id -> raw JSON
}

// PackageManifest represents the package.json of a FHIR NPM package.
type PackageManifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	FHIRVersion  string            `json:"fhirVersion,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// DefaultPackages maps FHIR versions to their default package configurations.
// Based on latest stable versions as of January 2025.
var DefaultPackages = map[string][]PackageRef{
	"4.0.1": {
		{Name: "hl7.fhir.r4.core", Version: "4.0.1"},
		{Name: "hl7.terminology.r4", Version: "7.0.1"},
		{Name: "hl7.fhir.uv.extensions.r4", Version: "5.2.0"},
	},
	"4.3.0": {
		{Name: "hl7.fhir.r4b.core", Version: "4.3.0"},
		{Name: "hl7.terminology.r4", Version: "7.0.1"},
		// R4B has no stable extensions package, use R4 as fallback
		{Name: "hl7.fhir.uv.extensions.r4", Version: "5.2.0"},
	},
	"5.0.0": {
		{Name: "hl7.fhir.r5.core", Version: "5.0.0"},
		{Name: "hl7.terminology.r5", Version: "7.0.1"},
		{Name: "hl7.fhir.uv.extensions.r5", Version: "5.2.0"},
	},
}

// Loader loads FHIR packages from the NPM cache.
type Loader struct {
	basePath string
}

// NewLoader creates a new Loader with the given base path.
func NewLoader(basePath string) *Loader {
	if basePath == "" {
		basePath = DefaultPackagePath()
	}
	return &Loader{basePath: basePath}
}

// BasePath returns the base path for packages.
func (l *Loader) BasePath() string {
	return l.basePath
}

// LoadPackage loads a specific package by name and version.
func (l *Loader) LoadPackage(name, version string) (*Package, error) {
	pkgDir := filepath.Join(l.basePath, fmt.Sprintf("%s#%s", name, version))

	// Check if package directory exists
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("package %s#%s not found at %s", name, version, pkgDir)
	}

	// Load package manifest
	manifestPath := filepath.Join(pkgDir, "package", "package.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package manifest: %w", err)
	}

	var manifest PackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse package manifest: %w", err)
	}

	pkg := &Package{
		Name:        name,
		Version:     version,
		Path:        pkgDir,
		FHIRVersion: manifest.FHIRVersion,
		Resources:   make(map[string]json.RawMessage),
	}

	// Load all JSON resources from package directory
	packageDir := filepath.Join(pkgDir, "package")
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read package directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.Name() == "package.json" || entry.Name() == ".index.json" {
			continue
		}

		filePath := filepath.Join(packageDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		// Extract resourceType and id for indexing
		var resource struct {
			ResourceType string `json:"resourceType"`
			ID           string `json:"id"`
			URL          string `json:"url"`
		}
		if err := json.Unmarshal(data, &resource); err != nil {
			continue
		}

		// Index by URL for StructureDefinitions and other conformance resources
		if resource.URL != "" {
			pkg.Resources[resource.URL] = data
		}
		// Also index by resourceType/id
		if resource.ResourceType != "" && resource.ID != "" {
			key := fmt.Sprintf("%s/%s", resource.ResourceType, resource.ID)
			pkg.Resources[key] = data
		}
	}

	return pkg, nil
}

// LoadPackageRef loads a package from a PackageRef.
func (l *Loader) LoadPackageRef(ref PackageRef) (*Package, error) {
	return l.LoadPackage(ref.Name, ref.Version)
}

// LoadVersion loads all default packages for a specific FHIR version.
func (l *Loader) LoadVersion(version string) ([]*Package, error) {
	refs, ok := DefaultPackages[version]
	if !ok {
		return nil, fmt.Errorf("unknown FHIR version: %s (supported: 4.0.1, 4.3.0, 5.0.0)", version)
	}

	packages := make([]*Package, 0, len(refs))
	var errors []string

	for _, ref := range refs {
		pkg, err := l.LoadPackageRef(ref)
		if err != nil {
			// Core package is required, others are optional
			if strings.Contains(ref.Name, ".core") {
				return nil, fmt.Errorf("failed to load core package: %w", err)
			}
			errors = append(errors, fmt.Sprintf("%s: %v", ref.String(), err))
			continue
		}
		packages = append(packages, pkg)
	}

	if len(errors) > 0 {
		// Log warnings for optional packages that failed to load
		// In production, this should use a proper logger
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", e)
		}
	}

	return packages, nil
}

// ListPackages returns all available packages in the cache.
func (l *Loader) ListPackages() ([]string, error) {
	entries, err := os.ReadDir(l.basePath)
	if err != nil {
		return nil, err
	}

	var packages []string
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), "#") {
			packages = append(packages, entry.Name())
		}
	}
	return packages, nil
}

// ParsePackageSpec parses "name#version" into separate components.
func ParsePackageSpec(spec string) (name, version string) {
	parts := strings.SplitN(spec, "#", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return spec, ""
}

// LoadFromTgz loads a FHIR package from a local .tgz file.
// The package is extracted to a temporary directory and loaded.
func (l *Loader) LoadFromTgz(tgzPath string) (*Package, error) {
	// Open the .tgz file
	file, err := os.Open(tgzPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tgz file: %w", err)
	}
	defer file.Close()

	return l.loadFromTgzReader(file, tgzPath)
}

// LoadFromURL loads a FHIR package from a remote URL pointing to a .tgz file.
// The package is downloaded to a temporary location and loaded.
func (l *Loader) LoadFromURL(url string) (*Package, error) {
	// Download the .tgz file
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download package from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download package: HTTP %d", resp.StatusCode)
	}

	return l.loadFromTgzReader(resp.Body, url)
}

// loadFromTgzReader loads a package from a gzipped tar reader.
func (l *Loader) loadFromTgzReader(reader io.Reader, source string) (*Package, error) {
	// Create gzip reader
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	pkg := &Package{
		Resources: make(map[string]json.RawMessage),
	}

	var manifestData []byte

	// Extract and process files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Normalize path (remove leading "package/" if present)
		name := header.Name
		name = strings.TrimPrefix(name, "package/")

		// Skip non-JSON files (except package.json)
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		// Read file content
		data, err := io.ReadAll(tarReader)
		if err != nil {
			continue
		}

		// Handle package.json specially
		if name == "package.json" {
			manifestData = data
			continue
		}

		// Skip index files
		if name == ".index.json" {
			continue
		}

		// Extract resourceType and id for indexing
		var resource struct {
			ResourceType string `json:"resourceType"`
			ID           string `json:"id"`
			URL          string `json:"url"`
		}
		if err := json.Unmarshal(data, &resource); err != nil {
			continue
		}

		// Index by URL for StructureDefinitions and other conformance resources
		if resource.URL != "" {
			pkg.Resources[resource.URL] = data
		}
		// Also index by resourceType/id
		if resource.ResourceType != "" && resource.ID != "" {
			key := fmt.Sprintf("%s/%s", resource.ResourceType, resource.ID)
			pkg.Resources[key] = data
		}
	}

	// Parse manifest
	if manifestData == nil {
		return nil, fmt.Errorf("package.json not found in %s", source)
	}

	var manifest PackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse package manifest: %w", err)
	}

	pkg.Name = manifest.Name
	pkg.Version = manifest.Version
	pkg.FHIRVersion = manifest.FHIRVersion
	pkg.Path = source

	return pkg, nil
}
