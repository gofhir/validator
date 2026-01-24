package registry

import (
	"context"
	"fmt"

	fv "github.com/gofhir/validator"
)

// PackageRef represents a reference to a FHIR package.
type PackageRef struct {
	Name    string
	Version string
}

// String returns the package reference as "name@version".
func (p PackageRef) String() string {
	if p.Version == "" || p.Version == "latest" {
		return p.Name
	}
	return fmt.Sprintf("%s@%s", p.Name, p.Version)
}

// CorePackages maps FHIR versions to their core package names.
var CorePackages = map[fv.FHIRVersion]PackageRef{
	fv.R4:  {Name: "hl7.fhir.r4.core", Version: "4.0.1"},
	fv.R4B: {Name: "hl7.fhir.r4b.core", Version: "4.3.0"},
	fv.R5:  {Name: "hl7.fhir.r5.core", Version: "5.0.0"},
}

// TerminologyPackages maps FHIR versions to their terminology package names.
var TerminologyPackages = map[fv.FHIRVersion]PackageRef{
	fv.R4:  {Name: "hl7.terminology.r4", Version: "latest"},
	fv.R4B: {Name: "hl7.terminology.r4", Version: "latest"}, // R4B uses R4 terminology
	fv.R5:  {Name: "hl7.terminology.r5", Version: "latest"},
}

// ExtensionsPackages maps FHIR versions to their extensions package names.
var ExtensionsPackages = map[fv.FHIRVersion]PackageRef{
	fv.R4:  {Name: "hl7.fhir.uv.extensions.r4", Version: "latest"},
	fv.R4B: {Name: "hl7.fhir.uv.extensions.r4", Version: "latest"},
	fv.R5:  {Name: "hl7.fhir.uv.extensions.r5", Version: "latest"},
}

// Resolver determines which packages are needed for validation.
type Resolver struct {
	client *Client
}

// NewResolver creates a new package resolver.
func NewResolver(client *Client) *Resolver {
	return &Resolver{client: client}
}

// ResolveOptions configures package resolution.
type ResolveOptions struct {
	// IncludeTerminology includes the terminology package (THO).
	IncludeTerminology bool

	// IncludeExtensions includes the extensions package.
	IncludeExtensions bool

	// AdditionalPackages are extra packages to include.
	AdditionalPackages []PackageRef
}

// DefaultResolveOptions returns options that include core and terminology.
func DefaultResolveOptions() ResolveOptions {
	return ResolveOptions{
		IncludeTerminology: true,
		IncludeExtensions:  false,
	}
}

// ResolvedPackages contains the resolved package paths.
type ResolvedPackages struct {
	Core        string   // Path to core package
	Terminology string   // Path to terminology package (if requested)
	Extensions  string   // Path to extensions package (if requested)
	Additional  []string // Paths to additional packages
	Version     fv.FHIRVersion
}

// Resolve resolves and downloads all required packages for a FHIR version.
func (r *Resolver) Resolve(ctx context.Context, version fv.FHIRVersion, opts ResolveOptions) (*ResolvedPackages, error) {
	result := &ResolvedPackages{
		Version: version,
	}

	// Get core package
	coreRef, ok := CorePackages[version]
	if !ok {
		return nil, fmt.Errorf("unsupported FHIR version: %s", version)
	}

	corePath, err := r.client.GetPackage(ctx, coreRef.Name, coreRef.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get core package %s: %w", coreRef, err)
	}
	result.Core = corePath

	// Get terminology package if requested
	if opts.IncludeTerminology {
		termRef, ok := TerminologyPackages[version]
		if ok {
			termPath, err := r.client.GetPackage(ctx, termRef.Name, termRef.Version)
			if err != nil {
				// Terminology is optional - log warning but don't fail
				fmt.Printf("Warning: failed to get terminology package %s: %v\n", termRef, err)
			} else {
				result.Terminology = termPath
			}
		}
	}

	// Get extensions package if requested
	if opts.IncludeExtensions {
		extRef, ok := ExtensionsPackages[version]
		if ok {
			extPath, err := r.client.GetPackage(ctx, extRef.Name, extRef.Version)
			if err != nil {
				// Extensions are optional
				fmt.Printf("Warning: failed to get extensions package %s: %v\n", extRef, err)
			} else {
				result.Extensions = extPath
			}
		}
	}

	// Get additional packages
	for _, ref := range opts.AdditionalPackages {
		path, err := r.client.GetPackage(ctx, ref.Name, ref.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to get package %s: %w", ref, err)
		}
		result.Additional = append(result.Additional, path)
	}

	return result, nil
}

// ResolveWithDependencies resolves packages including their dependencies.
func (r *Resolver) ResolveWithDependencies(ctx context.Context, version fv.FHIRVersion, opts ResolveOptions) (*ResolvedPackages, error) {
	// First resolve main packages
	result, err := r.Resolve(ctx, version, opts)
	if err != nil {
		return nil, err
	}

	// Resolve dependencies for additional packages
	for _, path := range result.Additional {
		manifest, err := r.client.ReadManifest(path)
		if err != nil {
			continue // Skip if can't read manifest
		}

		for depName, depVersion := range manifest.Dependencies {
			// Skip core and terminology dependencies (already included)
			if isCoreDependency(depName) {
				continue
			}

			depPath, err := r.client.GetPackage(ctx, depName, depVersion)
			if err != nil {
				fmt.Printf("Warning: failed to get dependency %s@%s: %v\n", depName, depVersion, err)
				continue
			}

			// Add if not already in list
			if !contains(result.Additional, depPath) {
				result.Additional = append(result.Additional, depPath)
			}
		}
	}

	return result, nil
}

// isCoreDependency checks if a package name is a core FHIR dependency.
func isCoreDependency(name string) bool {
	corePrefixes := []string{
		"hl7.fhir.r4.core",
		"hl7.fhir.r4b.core",
		"hl7.fhir.r5.core",
		"hl7.terminology",
	}
	for _, prefix := range corePrefixes {
		if name == prefix || len(name) > len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// contains checks if a string slice contains a value.
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
