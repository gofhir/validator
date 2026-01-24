package context

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"sync"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/loader"
	"github.com/gofhir/validator/registry"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/specs"
	"github.com/gofhir/validator/terminology"
)

// SpecContext holds all version-specific resources for FHIR validation.
// It provides automatic loading of StructureDefinitions and optionally
// CodeSystems/ValueSets from the embedded FHIR specification files.
type SpecContext struct {
	// Version is the FHIR version this context is configured for.
	Version fv.FHIRVersion

	// Profiles provides access to StructureDefinitions for profile resolution.
	Profiles service.ProfileResolver

	// Terminology provides code validation against CodeSystems and ValueSets.
	// This is nil if terminology loading was not enabled.
	Terminology service.TerminologyService

	// Options used to create this context.
	options Options

	// loaded indicates whether specs have been loaded.
	loaded bool

	// mu protects lazy loading operations.
	mu sync.RWMutex
}

// New creates a new SpecContext for the specified FHIR version.
// It automatically loads StructureDefinitions and optionally CodeSystems/ValueSets.
//
// By default (PackageSourceAuto), it tries to download packages from packages.fhir.org
// and falls back to embedded specs if download fails.
func New(ctx context.Context, version fv.FHIRVersion, opts Options) (*SpecContext, error) {
	sc := &SpecContext{
		Version: version,
		options: opts,
	}

	// Try registry-based loading first (unless explicitly set to embedded)
	if opts.PackageSource == PackageSourceRegistry ||
		(opts.PackageSource == PackageSourceAuto && !opts.OfflineMode) {
		err := sc.loadFromRegistry(ctx, version, opts)
		if err == nil {
			sc.loaded = true
			return sc, nil
		}
		// If registry fails and we're in auto mode, fall back to embedded
		if opts.PackageSource == PackageSourceAuto {
			fmt.Printf("Registry loading failed, falling back to embedded specs: %v\n", err)
		} else {
			return nil, fmt.Errorf("failed to load from registry: %w", err)
		}
	}

	// Load from embedded specs
	return sc.loadFromEmbedded(ctx, version, opts)
}

// loadFromRegistry loads packages from the FHIR package registry.
func (sc *SpecContext) loadFromRegistry(ctx context.Context, version fv.FHIRVersion, opts Options) error {
	// Create registry client
	clientOpts := []registry.ClientOption{}
	if opts.CacheDir != "" {
		clientOpts = append(clientOpts, registry.WithCacheDir(opts.CacheDir))
	}
	client := registry.NewClient(clientOpts...)

	// Create resolver
	resolver := registry.NewResolver(client)

	// Parse additional packages
	additionalPkgs := make([]registry.PackageRef, 0, len(opts.AdditionalPackages))
	for _, pkg := range opts.AdditionalPackages {
		ref := parsePackageRef(pkg)
		additionalPkgs = append(additionalPkgs, ref)
	}

	// Resolve packages
	resolveOpts := registry.ResolveOptions{
		IncludeTerminology: opts.LoadTerminology,
		IncludeExtensions:  false,
		AdditionalPackages: additionalPkgs,
	}

	resolved, err := resolver.Resolve(ctx, version, resolveOpts)
	if err != nil {
		return fmt.Errorf("failed to resolve packages: %w", err)
	}

	// Create services
	profileService := loader.NewInMemoryProfileService()
	var termService *terminology.InMemoryTerminologyService
	if opts.LoadTerminology {
		termService = terminology.NewInMemoryTerminologyService()
	}

	// Create loader
	pkgLoader := registry.NewPackageLoader(profileService, termService)

	// Load packages - always use LoadPackages for proper ordering
	// (CodeSystems must be loaded before ValueSets for filter expansion)
	stats, err := pkgLoader.LoadPackages(resolved)
	if err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	sc.Profiles = profileService
	if termService != nil {
		sc.Terminology = termService
	}

	_ = stats // Can be used for logging
	return nil
}

// loadFromEmbedded loads specs from embedded files.
func (sc *SpecContext) loadFromEmbedded(_ context.Context, version fv.FHIRVersion, opts Options) (*SpecContext, error) {
	// Convert fv.FHIRVersion to specs.FHIRVersion
	specsVersion, err := toSpecsVersion(version)
	if err != nil {
		return nil, err
	}

	// Get embedded specs filesystem
	specsFS, dir, err := specs.GetSpecsFS(specsVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get specs for %s: %w", version, err)
	}

	// Load profiles (StructureDefinitions)
	profileService := loader.NewInMemoryProfileService()

	// Load profiles-resources.json
	resourcesData, err := specsFS.ReadFile(filepath.Join(dir, specs.SpecFiles.ProfilesResources))
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles-resources.json: %w", err)
	}
	if _, loadErr := profileService.LoadFromJSON(resourcesData); loadErr != nil {
		return nil, fmt.Errorf("failed to load profiles-resources.json: %w", loadErr)
	}

	// Load profiles-types.json
	typesData, err := specsFS.ReadFile(filepath.Join(dir, specs.SpecFiles.ProfilesTypes))
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles-types.json: %w", err)
	}
	if _, err := profileService.LoadFromJSON(typesData); err != nil {
		return nil, fmt.Errorf("failed to load profiles-types.json: %w", err)
	}

	sc.Profiles = profileService

	// Load terminology if enabled
	if opts.LoadTerminology {
		termService, err := loadTerminology(specsFS, dir, opts.TerminologyOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to load terminology: %w", err)
		}
		sc.Terminology = termService
	}

	sc.loaded = true
	return sc, nil
}

// parsePackageRef parses a package reference string like "name@version".
func parsePackageRef(s string) registry.PackageRef {
	parts := splitAtSign(s)
	if len(parts) == 2 {
		return registry.PackageRef{Name: parts[0], Version: parts[1]}
	}
	return registry.PackageRef{Name: s, Version: "latest"}
}

// splitAtSign splits a string at the last @ sign.
func splitAtSign(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// LoadIG loads an Implementation Guide from a directory into this context.
// The IG's StructureDefinitions are added to the profile resolver.
func (sc *SpecContext) LoadIG(dirPath string) (int, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	profileService, ok := sc.Profiles.(*loader.InMemoryProfileService)
	if !ok {
		return 0, fmt.Errorf("cannot load IG: profile service does not support dynamic loading")
	}

	return profileService.LoadFromDirectory(dirPath)
}

// LoadIGFromBytes loads an Implementation Guide from a Bundle JSON byte slice.
func (sc *SpecContext) LoadIGFromBytes(data []byte) (int, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	profileService, ok := sc.Profiles.(*loader.InMemoryProfileService)
	if !ok {
		return 0, fmt.Errorf("cannot load IG: profile service does not support dynamic loading")
	}

	return profileService.LoadFromJSON(data)
}

// IsLoaded returns true if specs have been loaded.
func (sc *SpecContext) IsLoaded() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.loaded
}

// HasTerminology returns true if terminology service is available.
func (sc *SpecContext) HasTerminology() bool {
	return sc.Terminology != nil
}

// toSpecsVersion converts fv.FHIRVersion to specs.FHIRVersion.
func toSpecsVersion(version fv.FHIRVersion) (specs.FHIRVersion, error) {
	switch version {
	case fv.R4:
		return specs.R4, nil
	case fv.R4B:
		return specs.R4B, nil
	case fv.R5:
		return specs.R5, nil
	default:
		return "", fmt.Errorf("unsupported FHIR version: %s", version)
	}
}

// loadTerminology creates a terminology service from embedded specs.
func loadTerminology(specsFS embed.FS, dir string, opts TerminologyOptions) (service.TerminologyService, error) {
	// Configure cache
	cacheTTL := opts.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = terminology.DefaultCacheTTL
	}

	cacheConfig := terminology.CacheConfig{
		ShardCount: terminology.DefaultShardCount,
		TTL:        cacheTTL,
	}

	// Create cached terminology service
	cachedService := terminology.NewCachedTerminologyService(cacheConfig)

	// Load from embedded specs
	stats, err := cachedService.Inner().LoadFromFS(specsFS, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to load terminology from embedded specs: %w", err)
	}

	// Log stats for debugging (can be removed or made conditional)
	_ = stats // CodeSystemsLoaded, ValueSetsLoaded, Errors

	return cachedService, nil
}
