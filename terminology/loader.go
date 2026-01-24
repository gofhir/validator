package terminology

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofhir/fhir/r4"
	"github.com/gofhir/validator/specs"
)

// LoadStats contains statistics about terminology loading.
type LoadStats struct {
	CodeSystemsLoaded int64
	ValueSetsLoaded   int64
	Errors            int64
}

// LoadFromEmbeddedSpecs loads CodeSystems and ValueSets from embedded spec files.
// This loads v3-codesystems.json and valuesets.json for the given FHIR version.
func (s *InMemoryTerminologyService) LoadFromEmbeddedSpecs(version specs.FHIRVersion) (*LoadStats, error) {
	specsFS, dir, err := specs.GetSpecsFS(version)
	if err != nil {
		return nil, fmt.Errorf("failed to get specs for %s: %w", version, err)
	}

	stats := &LoadStats{}

	// Load v3-codesystems.json (contains CodeSystems)
	if specs.HasFile(version, specs.SpecFiles.V3CodeSystems) {
		csData, err := specsFS.ReadFile(filepath.Join(dir, specs.SpecFiles.V3CodeSystems))
		if err != nil {
			return stats, fmt.Errorf("failed to read v3-codesystems.json: %w", err)
		}

		csLoaded, csErrors := s.loadCodeSystemsFromBundle(csData)
		stats.CodeSystemsLoaded += csLoaded
		stats.Errors += csErrors
	}

	// Load valuesets.json (contains ValueSets)
	if specs.HasFile(version, specs.SpecFiles.ValueSets) {
		vsData, err := specsFS.ReadFile(filepath.Join(dir, specs.SpecFiles.ValueSets))
		if err != nil {
			return stats, fmt.Errorf("failed to read valuesets.json: %w", err)
		}

		vsLoaded, vsErrors := s.loadValueSetsFromBundle(vsData)
		stats.ValueSetsLoaded += vsLoaded
		stats.Errors += vsErrors
	}

	return stats, nil
}

// LoadFromEmbeddedSpecsParallel loads CodeSystems and ValueSets in parallel.
// This provides better performance for large spec files.
func (s *InMemoryTerminologyService) LoadFromEmbeddedSpecsParallel(version specs.FHIRVersion) (*LoadStats, error) {
	specsFS, dir, err := specs.GetSpecsFS(version)
	if err != nil {
		return nil, fmt.Errorf("failed to get specs for %s: %w", version, err)
	}

	stats := &LoadStats{}
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Load v3-codesystems.json in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		if specs.HasFile(version, specs.SpecFiles.V3CodeSystems) {
			csData, err := specsFS.ReadFile(filepath.Join(dir, specs.SpecFiles.V3CodeSystems))
			if err != nil {
				errChan <- fmt.Errorf("failed to read v3-codesystems.json: %w", err)
				return
			}

			csLoaded, csErrors := s.loadCodeSystemsFromBundle(csData)
			atomic.AddInt64(&stats.CodeSystemsLoaded, csLoaded)
			atomic.AddInt64(&stats.Errors, csErrors)
		}
	}()

	// Load valuesets.json in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		if specs.HasFile(version, specs.SpecFiles.ValueSets) {
			vsData, err := specsFS.ReadFile(filepath.Join(dir, specs.SpecFiles.ValueSets))
			if err != nil {
				errChan <- fmt.Errorf("failed to read valuesets.json: %w", err)
				return
			}

			vsLoaded, vsErrors := s.loadValueSetsFromBundle(vsData)
			atomic.AddInt64(&stats.ValueSetsLoaded, vsLoaded)
			atomic.AddInt64(&stats.Errors, vsErrors)
		}
	}()

	wg.Wait()
	close(errChan)

	// Collect errors
	var loadErr error
	for err := range errChan {
		if loadErr == nil {
			loadErr = err
		}
	}

	return stats, loadErr
}

// LoadFromFS loads CodeSystems and ValueSets from an embedded filesystem.
// This is useful for loading from custom IGs.
func (s *InMemoryTerminologyService) LoadFromFS(fs embed.FS, dir string) (*LoadStats, error) {
	stats := &LoadStats{}

	// Try to load v3-codesystems.json
	csPath := filepath.Join(dir, specs.SpecFiles.V3CodeSystems)
	if csData, err := fs.ReadFile(csPath); err == nil {
		csLoaded, csErrors := s.loadCodeSystemsFromBundle(csData)
		stats.CodeSystemsLoaded += csLoaded
		stats.Errors += csErrors
	}

	// Try to load valuesets.json
	vsPath := filepath.Join(dir, specs.SpecFiles.ValueSets)
	if vsData, err := fs.ReadFile(vsPath); err == nil {
		vsLoaded, vsErrors := s.loadValueSetsFromBundle(vsData)
		stats.ValueSetsLoaded += vsLoaded
		stats.Errors += vsErrors
	}

	return stats, nil
}

// LoadFromJSON loads CodeSystems or ValueSets from JSON data.
// Auto-detects Bundle vs single resource format.
func (s *InMemoryTerminologyService) LoadFromJSON(data []byte) (*LoadStats, error) {
	stats := &LoadStats{}

	// Detect resource type
	var probe struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	switch probe.ResourceType {
	case "Bundle":
		// Try loading as CodeSystems first
		csLoaded, _ := s.loadCodeSystemsFromBundle(data)
		stats.CodeSystemsLoaded += csLoaded

		// Then try ValueSets
		vsLoaded, _ := s.loadValueSetsFromBundle(data)
		stats.ValueSetsLoaded += vsLoaded

	case "CodeSystem":
		var cs r4.CodeSystem
		if err := json.Unmarshal(data, &cs); err != nil {
			return nil, fmt.Errorf("failed to parse CodeSystem: %w", err)
		}
		if err := s.LoadR4CodeSystem(&cs); err != nil {
			stats.Errors++
			return stats, err
		}
		stats.CodeSystemsLoaded++

	case "ValueSet":
		var vs r4.ValueSet
		if err := json.Unmarshal(data, &vs); err != nil {
			return nil, fmt.Errorf("failed to parse ValueSet: %w", err)
		}
		if err := s.LoadR4ValueSet(&vs); err != nil {
			stats.Errors++
			return stats, err
		}
		stats.ValueSetsLoaded++

	default:
		return nil, fmt.Errorf("unsupported resourceType: %s", probe.ResourceType)
	}

	return stats, nil
}

// LoadFromDirectory loads CodeSystems and ValueSets from a directory.
// This is useful for loading terminology from IG packages.
// CodeSystems are loaded before ValueSets to ensure filter expansion works.
func (s *InMemoryTerminologyService) LoadFromDirectory(dirPath string) (*LoadStats, error) {
	stats := &LoadStats{}

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dirPath)
	}

	// Read all JSON files
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Separate files by type for ordered loading
	// CodeSystems must be loaded before ValueSets for filter expansion
	var codeSystems, valueSets []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Skip package metadata files
		if entry.Name() == "package.json" || entry.Name() == ".index.json" {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		name := entry.Name()

		// Categorize by filename prefix (FHIR packages use consistent naming)
		switch {
		case strings.HasPrefix(name, "CodeSystem-"):
			codeSystems = append(codeSystems, filePath)
		case strings.HasPrefix(name, "ValueSet-"):
			valueSets = append(valueSets, filePath)
		}
	}

	// Load CodeSystems first
	for _, filePath := range codeSystems {
		data, err := os.ReadFile(filePath)
		if err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}

		var cs r4.CodeSystem
		if err := json.Unmarshal(data, &cs); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}

		if err := s.LoadR4CodeSystem(&cs); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}
		atomic.AddInt64(&stats.CodeSystemsLoaded, 1)
	}

	// Then load ValueSets
	for _, filePath := range valueSets {
		data, err := os.ReadFile(filePath)
		if err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}

		var vs r4.ValueSet
		if err := json.Unmarshal(data, &vs); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}

		if err := s.LoadR4ValueSet(&vs); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			continue
		}
		atomic.AddInt64(&stats.ValueSetsLoaded, 1)
	}

	return stats, nil
}

// bundleEntry represents an entry in a FHIR Bundle.
type bundleEntry struct {
	Resource json.RawMessage `json:"resource"`
}

// bundle represents a minimal FHIR Bundle structure.
type bundle struct {
	ResourceType string        `json:"resourceType"`
	Entry        []bundleEntry `json:"entry"`
}

// resourceLoader is a function type for loading a specific resource type.
type resourceLoader func(data json.RawMessage) error

// loadResourcesFromBundle is a generic function to load resources from a Bundle JSON.
func loadResourcesFromBundle(data []byte, targetType string, loader resourceLoader) (loaded, errors int64) {
	var b bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return 0, 1
	}

	if b.ResourceType != "Bundle" {
		return 0, 1
	}

	for _, entry := range b.Entry {
		if entry.Resource == nil {
			continue
		}

		var probe struct {
			ResourceType string `json:"resourceType"`
		}
		if err := json.Unmarshal(entry.Resource, &probe); err != nil {
			continue
		}

		if probe.ResourceType != targetType {
			continue
		}

		if err := loader(entry.Resource); err != nil {
			errors++
			continue
		}
		loaded++
	}

	return loaded, errors
}

// loadCodeSystemsFromBundle loads CodeSystems from a Bundle JSON.
func (s *InMemoryTerminologyService) loadCodeSystemsFromBundle(data []byte) (loaded, errors int64) {
	return loadResourcesFromBundle(data, "CodeSystem", func(raw json.RawMessage) error {
		var cs r4.CodeSystem
		if err := json.Unmarshal(raw, &cs); err != nil {
			return err
		}
		return s.LoadR4CodeSystem(&cs)
	})
}

// loadValueSetsFromBundle loads ValueSets from a Bundle JSON.
func (s *InMemoryTerminologyService) loadValueSetsFromBundle(data []byte) (loaded, errors int64) {
	return loadResourcesFromBundle(data, "ValueSet", func(raw json.RawMessage) error {
		var vs r4.ValueSet
		if err := json.Unmarshal(raw, &vs); err != nil {
			return err
		}
		return s.LoadR4ValueSet(&vs)
	})
}
