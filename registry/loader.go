package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofhir/validator/loader"
	"github.com/gofhir/validator/terminology"
	"github.com/gofhir/fhir/r4"
)

// LoadStats contains statistics about package loading.
type LoadStats struct {
	StructureDefinitions int64
	CodeSystems          int64
	ValueSets            int64
	Errors               int64
	PackagesLoaded       int
}

// PackageLoader loads FHIR packages into profile and terminology services.
type PackageLoader struct {
	profileService *loader.InMemoryProfileService
	termService    *terminology.InMemoryTerminologyService
	mu             sync.Mutex
}

// NewPackageLoader creates a new package loader.
func NewPackageLoader(
	profileService *loader.InMemoryProfileService,
	termService *terminology.InMemoryTerminologyService,
) *PackageLoader {
	return &PackageLoader{
		profileService: profileService,
		termService:    termService,
	}
}

// LoadPackage loads a single package from a directory.
// CodeSystems are loaded before ValueSets to ensure filter expansion works.
func (l *PackageLoader) LoadPackage(packageDir string) (*LoadStats, error) {
	stats := &LoadStats{}

	// Find the package content directory
	contentDir := packageDir
	packageSubDir := filepath.Join(packageDir, "package")
	if _, err := os.Stat(packageSubDir); err == nil {
		contentDir = packageSubDir
	}

	// Read all JSON files in the package
	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read package directory: %w", err)
	}

	// Separate files by type for ordered loading
	// CodeSystems must be loaded before ValueSets for filter expansion
	var structureDefs, codeSystems, valueSets, others []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Skip package.json and .index.json
		if entry.Name() == "package.json" || entry.Name() == ".index.json" {
			continue
		}

		filePath := filepath.Join(contentDir, entry.Name())
		name := entry.Name()

		// Categorize by filename prefix (FHIR packages use consistent naming)
		switch {
		case strings.HasPrefix(name, "StructureDefinition-"):
			structureDefs = append(structureDefs, filePath)
		case strings.HasPrefix(name, "CodeSystem-"):
			codeSystems = append(codeSystems, filePath)
		case strings.HasPrefix(name, "ValueSet-"):
			valueSets = append(valueSets, filePath)
		default:
			others = append(others, filePath)
		}
	}

	// Load in order: StructureDefinitions, CodeSystems, ValueSets, others
	for _, filePath := range structureDefs {
		if err := l.loadFile(filePath, stats); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
		}
	}
	for _, filePath := range codeSystems {
		if err := l.loadFile(filePath, stats); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
		}
	}
	for _, filePath := range valueSets {
		if err := l.loadFile(filePath, stats); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
		}
	}
	for _, filePath := range others {
		if err := l.loadFile(filePath, stats); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
		}
	}

	stats.PackagesLoaded = 1
	return stats, nil
}

// LoadPackages loads multiple packages.
func (l *PackageLoader) LoadPackages(resolved *ResolvedPackages) (*LoadStats, error) {
	totalStats := &LoadStats{}

	// Load core package
	if resolved.Core != "" {
		stats, err := l.LoadPackage(resolved.Core)
		if err != nil {
			return nil, fmt.Errorf("failed to load core package: %w", err)
		}
		l.mergeStats(totalStats, stats)
	}

	// Load terminology package
	if resolved.Terminology != "" {
		stats, err := l.LoadPackage(resolved.Terminology)
		if err != nil {
			// Terminology is optional, log but don't fail
			fmt.Printf("Warning: failed to load terminology package: %v\n", err)
		} else {
			l.mergeStats(totalStats, stats)
		}
	}

	// Load extensions package
	if resolved.Extensions != "" {
		stats, err := l.LoadPackage(resolved.Extensions)
		if err != nil {
			fmt.Printf("Warning: failed to load extensions package: %v\n", err)
		} else {
			l.mergeStats(totalStats, stats)
		}
	}

	// Load additional packages
	for _, pkgPath := range resolved.Additional {
		stats, err := l.LoadPackage(pkgPath)
		if err != nil {
			fmt.Printf("Warning: failed to load package %s: %v\n", pkgPath, err)
			continue
		}
		l.mergeStats(totalStats, stats)
	}

	return totalStats, nil
}

// LoadPackageParallel loads a package using parallel file processing.
func (l *PackageLoader) LoadPackageParallel(packageDir string, workers int) (*LoadStats, error) {
	stats := &LoadStats{}

	// Find the package content directory
	contentDir := packageDir
	packageSubDir := filepath.Join(packageDir, "package")
	if _, err := os.Stat(packageSubDir); err == nil {
		contentDir = packageSubDir
	}

	// Read all JSON files
	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read package directory: %w", err)
	}

	// Filter JSON files
	var jsonFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.Name() == "package.json" || entry.Name() == ".index.json" {
			continue
		}
		jsonFiles = append(jsonFiles, filepath.Join(contentDir, entry.Name()))
	}

	// Process files in parallel
	if workers <= 0 {
		workers = 4
	}

	fileChan := make(chan string, len(jsonFiles))
	for _, f := range jsonFiles {
		fileChan <- f
	}
	close(fileChan)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				if err := l.loadFile(filePath, stats); err != nil {
					atomic.AddInt64(&stats.Errors, 1)
				}
			}
		}()
	}

	wg.Wait()
	stats.PackagesLoaded = 1
	return stats, nil
}

// loadFile loads a single JSON file into the appropriate service.
func (l *PackageLoader) loadFile(filePath string, stats *LoadStats) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Detect resource type
	var probe struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}

	switch probe.ResourceType {
	case "StructureDefinition":
		if l.profileService != nil {
			var sd r4.StructureDefinition
			if err := json.Unmarshal(data, &sd); err != nil {
				return err
			}
			l.mu.Lock()
			err = l.profileService.LoadR4StructureDefinition(&sd)
			l.mu.Unlock()
			if err != nil {
				return err
			}
			atomic.AddInt64(&stats.StructureDefinitions, 1)
		}

	case "CodeSystem":
		if l.termService != nil {
			var cs r4.CodeSystem
			if err := json.Unmarshal(data, &cs); err != nil {
				return err
			}
			l.mu.Lock()
			err = l.termService.LoadR4CodeSystem(&cs)
			l.mu.Unlock()
			if err != nil {
				return err
			}
			atomic.AddInt64(&stats.CodeSystems, 1)
		}

	case "ValueSet":
		if l.termService != nil {
			var vs r4.ValueSet
			if err := json.Unmarshal(data, &vs); err != nil {
				return err
			}
			l.mu.Lock()
			err = l.termService.LoadR4ValueSet(&vs)
			l.mu.Unlock()
			if err != nil {
				return err
			}
			atomic.AddInt64(&stats.ValueSets, 1)
		}

	case "Bundle":
		// Process bundle entries
		return l.loadBundle(data, stats)
	}

	return nil
}

// loadBundle loads resources from a Bundle.
func (l *PackageLoader) loadBundle(data []byte, stats *LoadStats) error {
	var bundle struct {
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			Resource json.RawMessage `json:"resource"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(data, &bundle); err != nil {
		return err
	}

	for _, entry := range bundle.Entry {
		if entry.Resource == nil {
			continue
		}

		var probe struct {
			ResourceType string `json:"resourceType"`
		}
		if err := json.Unmarshal(entry.Resource, &probe); err != nil {
			continue
		}

		switch probe.ResourceType {
		case "StructureDefinition":
			if l.profileService != nil {
				var sd r4.StructureDefinition
				if err := json.Unmarshal(entry.Resource, &sd); err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				l.mu.Lock()
				err := l.profileService.LoadR4StructureDefinition(&sd)
				l.mu.Unlock()
				if err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				atomic.AddInt64(&stats.StructureDefinitions, 1)
			}

		case "CodeSystem":
			if l.termService != nil {
				var cs r4.CodeSystem
				if err := json.Unmarshal(entry.Resource, &cs); err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				l.mu.Lock()
				err := l.termService.LoadR4CodeSystem(&cs)
				l.mu.Unlock()
				if err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				atomic.AddInt64(&stats.CodeSystems, 1)
			}

		case "ValueSet":
			if l.termService != nil {
				var vs r4.ValueSet
				if err := json.Unmarshal(entry.Resource, &vs); err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				l.mu.Lock()
				err := l.termService.LoadR4ValueSet(&vs)
				l.mu.Unlock()
				if err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				atomic.AddInt64(&stats.ValueSets, 1)
			}
		}
	}

	return nil
}

// mergeStats merges source stats into target.
func (l *PackageLoader) mergeStats(target, source *LoadStats) {
	atomic.AddInt64(&target.StructureDefinitions, source.StructureDefinitions)
	atomic.AddInt64(&target.CodeSystems, source.CodeSystems)
	atomic.AddInt64(&target.ValueSets, source.ValueSets)
	atomic.AddInt64(&target.Errors, source.Errors)
	target.PackagesLoaded += source.PackagesLoaded
}
