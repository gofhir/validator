// Package specs provides embedded FHIR specification files.
//
// This package embeds the official FHIR specification JSON files for
// R4, R4B, and R5 versions. The embedded files include:
//   - profiles-resources.json: Resource StructureDefinitions
//   - profiles-types.json: DataType StructureDefinitions
//   - v3-codesystems.json: HL7 v3 CodeSystems (R4 only)
//   - valuesets.json: ValueSet definitions
//
// Usage:
//
//	fs, dir, err := specs.GetSpecsFS(fv.R4)
//	if err != nil {
//	    return err
//	}
//	data, err := fs.ReadFile(filepath.Join(dir, "profiles-resources.json"))
package specs

import (
	"embed"
	"fmt"
)

// Embedded spec files for each FHIR version
//
//go:embed r4/*.json
var R4Specs embed.FS

//go:embed r4b/*.json
var R4BSpecs embed.FS

//go:embed r5/*.json
var R5Specs embed.FS

// FHIRVersion represents a FHIR specification version.
type FHIRVersion string

const (
	R4  FHIRVersion = "R4"
	R4B FHIRVersion = "R4B"
	R5  FHIRVersion = "R5"
)

// SpecFiles contains the standard file names in each version directory.
var SpecFiles = struct {
	ProfilesResources string
	ProfilesTypes     string
	ProfilesOthers    string
	ValueSets         string
	V3CodeSystems     string
	V2Tables          string
	SearchParameters  string
	ConceptMaps       string
	DataElements      string
	Extensions        string
}{
	ProfilesResources: "profiles-resources.json",
	ProfilesTypes:     "profiles-types.json",
	ProfilesOthers:    "profiles-others.json",
	ValueSets:         "valuesets.json",
	V3CodeSystems:     "v3-codesystems.json",
	V2Tables:          "v2-tables.json",
	SearchParameters:  "search-parameters.json",
	ConceptMaps:       "conceptmaps.json",
	DataElements:      "dataelements.json",
	Extensions:        "extension-definitions.json",
}

// GetSpecsFS returns the embedded filesystem and directory name for a FHIR version.
// The returned directory name should be used as a prefix when reading files.
func GetSpecsFS(version FHIRVersion) (embed.FS, string, error) {
	switch version {
	case R4:
		return R4Specs, "r4", nil
	case R4B:
		return R4BSpecs, "r4b", nil
	case R5:
		return R5Specs, "r5", nil
	default:
		return embed.FS{}, "", fmt.Errorf("unsupported FHIR version: %s", version)
	}
}

// GetSpecsFSByString returns the embedded filesystem using a string version identifier.
// Accepts version strings like "R4", "4.0.1", "R4B", "4.3.0", "R5", "5.0.0".
func GetSpecsFSByString(version string) (embed.FS, string, error) {
	switch version {
	case "R4", "4.0", "4.0.1":
		return R4Specs, "r4", nil
	case "R4B", "4.3", "4.3.0":
		return R4BSpecs, "r4b", nil
	case "R5", "5.0", "5.0.0":
		return R5Specs, "r5", nil
	default:
		return embed.FS{}, "", fmt.Errorf("unsupported FHIR version: %s", version)
	}
}

// ListFiles returns the list of available files for a FHIR version.
func ListFiles(version FHIRVersion) ([]string, error) {
	fs, dir, err := GetSpecsFS(version)
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ReadFile reads a file from the embedded specs for a given version.
func ReadFile(version FHIRVersion, filename string) ([]byte, error) {
	fs, dir, err := GetSpecsFS(version)
	if err != nil {
		return nil, err
	}

	path := dir + "/" + filename
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	return data, nil
}

// HasFile checks if a file exists in the embedded specs for a given version.
func HasFile(version FHIRVersion, filename string) bool {
	fs, dir, err := GetSpecsFS(version)
	if err != nil {
		return false
	}

	path := dir + "/" + filename
	_, err = fs.ReadFile(path)
	return err == nil
}
