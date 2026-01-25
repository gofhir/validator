package loader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofhir/fhir/r4"
	"github.com/gofhir/validator/service"
)

// InMemoryProfileService implements service.ProfileResolver using in-memory storage.
// It stores pre-converted StructureDefinitions indexed by URL and Type.
type InMemoryProfileService struct {
	mu        sync.RWMutex
	byURL     map[string]*service.StructureDefinition
	byType    map[string]*service.StructureDefinition
	converter *R4Converter
}

// NewInMemoryProfileService creates a new in-memory profile service.
func NewInMemoryProfileService() *InMemoryProfileService {
	return &InMemoryProfileService{
		byURL:     make(map[string]*service.StructureDefinition),
		byType:    make(map[string]*service.StructureDefinition),
		converter: NewR4Converter(),
	}
}

// LoadR4StructureDefinition loads an R4 StructureDefinition into the service.
func (s *InMemoryProfileService) LoadR4StructureDefinition(sd *r4.StructureDefinition) error {
	if sd == nil {
		return fmt.Errorf("structure definition is nil")
	}

	converted := s.converter.ConvertStructureDefinition(sd)
	if converted == nil {
		return fmt.Errorf("failed to convert structure definition")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Index by URL
	if converted.URL != "" {
		s.byURL[converted.URL] = converted
	}

	// Index by Type - only index THE base definition for each type
	// This prevents profiles (like shareableactivitydefinition) from overwriting
	// the base type definition (like ActivityDefinition)
	if converted.Type != "" {
		switch converted.Kind {
		case "resource":
			// Only index if this is the base definition for the type
			// e.g., URL must be http://hl7.org/fhir/StructureDefinition/{Type}
			if isBaseTypeDefinition(converted.URL, converted.Type) {
				s.byType[converted.Type] = converted
			}
		case "complex-type", "primitive-type":
			// Same for complex types - only the base definition
			if isBaseTypeDefinition(converted.URL, converted.Type) {
				s.byType[converted.Type] = converted
			}
		}
	}

	return nil
}

// LoadR4StructureDefinitions loads multiple R4 StructureDefinitions.
func (s *InMemoryProfileService) LoadR4StructureDefinitions(sds []*r4.StructureDefinition) error {
	for _, sd := range sds {
		if err := s.LoadR4StructureDefinition(sd); err != nil {
			return err
		}
	}
	return nil
}

// LoadServiceStructureDefinition loads a pre-converted service.StructureDefinition.
func (s *InMemoryProfileService) LoadServiceStructureDefinition(sd *service.StructureDefinition) error {
	if sd == nil {
		return fmt.Errorf("structure definition is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if sd.URL != "" {
		s.byURL[sd.URL] = sd
	}

	// Only index THE base definition for the type
	if sd.Type != "" && (sd.Kind == "resource" || sd.Kind == "complex-type" || sd.Kind == "primitive-type") {
		if isBaseTypeDefinition(sd.URL, sd.Type) {
			s.byType[sd.Type] = sd
		}
	}

	return nil
}

// FetchStructureDefinition implements service.StructureDefinitionFetcher.
func (s *InMemoryProfileService) FetchStructureDefinition(ctx context.Context, url string) (*service.StructureDefinition, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, ok := s.byURL[url]
	if !ok {
		return nil, fmt.Errorf("structure definition not found: %s", url)
	}
	return sd, nil
}

// FetchStructureDefinitionByType implements service.StructureDefinitionByTypeFetcher.
// It searches in the following order:
// 1. By type name (for resources)
// 2. By canonical URL (for complex types and profiles)
func (s *InMemoryProfileService) FetchStructureDefinitionByType(ctx context.Context, resourceType string) (*service.StructureDefinition, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// First try by type (for resources)
	if sd, ok := s.byType[resourceType]; ok {
		return sd, nil
	}

	// Fallback: try by canonical URL (for complex types like SimpleQuantity, Dosage, etc.)
	canonicalURL := "http://hl7.org/fhir/StructureDefinition/" + resourceType
	if sd, ok := s.byURL[canonicalURL]; ok {
		return sd, nil
	}

	return nil, fmt.Errorf("structure definition not found for type: %s", resourceType)
}

// Count returns the number of loaded StructureDefinitions.
func (s *InMemoryProfileService) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byURL)
}

// URLs returns all loaded URLs.
func (s *InMemoryProfileService) URLs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	urls := make([]string, 0, len(s.byURL))
	for url := range s.byURL {
		urls = append(urls, url)
	}
	return urls
}

// Types returns all loaded resource types.
func (s *InMemoryProfileService) Types() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	types := make([]string, 0, len(s.byType))
	for t := range s.byType {
		types = append(types, t)
	}
	return types
}

// Clear removes all loaded StructureDefinitions.
func (s *InMemoryProfileService) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byURL = make(map[string]*service.StructureDefinition)
	s.byType = make(map[string]*service.StructureDefinition)
}

// isBaseTypeDefinition checks if a URL is THE base definition for its type.
// For example, http://hl7.org/fhir/StructureDefinition/Patient is the base for Patient,
// but http://hl7.org/fhir/StructureDefinition/us-core-patient is a profile, not the base.
func isBaseTypeDefinition(url, typeName string) bool {
	if typeName == "" {
		return false
	}
	expectedURL := "http://hl7.org/fhir/StructureDefinition/" + typeName
	return url == expectedURL
}

// LoadFromFile loads StructureDefinitions from a JSON file.
// Supports both single StructureDefinition and Bundle formats.
func (s *InMemoryProfileService) LoadFromFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return s.LoadFromJSON(data)
}

// LoadFromJSON loads StructureDefinitions from JSON data.
// Auto-detects Bundle vs single StructureDefinition format.
func (s *InMemoryProfileService) LoadFromJSON(data []byte) (int, error) {
	// Try to detect format
	var probe struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, fmt.Errorf("invalid JSON: %w", err)
	}

	switch probe.ResourceType {
	case "Bundle":
		return s.LoadFromBundle(data)
	case "StructureDefinition":
		var sd r4.StructureDefinition
		if err := json.Unmarshal(data, &sd); err != nil {
			return 0, fmt.Errorf("failed to parse StructureDefinition: %w", err)
		}
		if err := s.LoadR4StructureDefinition(&sd); err != nil {
			return 0, err
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported resourceType: %s", probe.ResourceType)
	}
}

// LoadFromBundle loads StructureDefinitions from a FHIR Bundle.
func (s *InMemoryProfileService) LoadFromBundle(data []byte) (int, error) {
	var bundle struct {
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			Resource json.RawMessage `json:"resource"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(data, &bundle); err != nil {
		return 0, fmt.Errorf("failed to parse Bundle: %w", err)
	}

	if bundle.ResourceType != "Bundle" {
		return 0, fmt.Errorf("expected Bundle, got %s", bundle.ResourceType)
	}

	count := 0
	for _, entry := range bundle.Entry {
		if entry.Resource == nil {
			continue
		}

		// Check if it's a StructureDefinition
		var probe struct {
			ResourceType string `json:"resourceType"`
		}
		if err := json.Unmarshal(entry.Resource, &probe); err != nil {
			continue
		}

		if probe.ResourceType != "StructureDefinition" {
			continue
		}

		var sd r4.StructureDefinition
		if err := json.Unmarshal(entry.Resource, &sd); err != nil {
			continue
		}

		if err := s.LoadR4StructureDefinition(&sd); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// LoadFromDirectory loads all StructureDefinition JSON files from a directory.
// Files must be named StructureDefinition-*.json to be loaded.
func (s *InMemoryProfileService) LoadFromDirectory(dirPath string) (int, error) {
	files, err := filepath.Glob(filepath.Join(dirPath, "StructureDefinition-*.json"))
	if err != nil {
		return 0, fmt.Errorf("failed to glob directory: %w", err)
	}

	total := 0
	for _, file := range files {
		count, err := s.LoadFromFile(file)
		if err != nil {
			// Skip files that fail to load
			continue
		}
		total += count
	}

	return total, nil
}

// LoadAllFromDirectory loads all JSON files from a directory recursively.
func (s *InMemoryProfileService) LoadAllFromDirectory(dirPath string) (int, error) {
	total := 0
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		count, err := s.LoadFromFile(path)
		if err != nil {
			return nil // Skip files that fail
		}
		total += count
		return nil
	})

	return total, err
}

// Verify interface compliance
var _ service.ProfileResolver = (*InMemoryProfileService)(nil)
