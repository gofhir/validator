package context

import "time"

// PackageSource defines where to load FHIR packages from.
type PackageSource int

const (
	// PackageSourceAuto tries registry first, falls back to embedded.
	PackageSourceAuto PackageSource = iota

	// PackageSourceRegistry downloads packages from packages.fhir.org.
	PackageSourceRegistry

	// PackageSourceEmbedded uses only embedded specs (no network).
	PackageSourceEmbedded
)

// Options configures the SpecContext loading behavior.
type Options struct {
	// LoadTerminology enables loading of CodeSystems and ValueSets.
	// When false, only StructureDefinitions are loaded.
	LoadTerminology bool

	// TerminologyOptions configures the terminology service.
	TerminologyOptions TerminologyOptions

	// PackageSource determines where to load packages from.
	// Default is PackageSourceAuto.
	PackageSource PackageSource

	// AdditionalPackages lists extra packages to load (e.g., IGs).
	// Format: "package.name" or "package.name@version"
	AdditionalPackages []string

	// CacheDir is the directory for caching downloaded packages.
	// Default is ~/.fhir/packages
	CacheDir string

	// OfflineMode prevents any network requests.
	// Only works if packages are already cached.
	OfflineMode bool

	// ParallelLoading enables parallel file loading for faster startup.
	ParallelLoading bool

	// Workers is the number of parallel workers for loading.
	// Default is 4.
	Workers int
}

// TerminologyOptions configures the terminology service behavior.
type TerminologyOptions struct {
	// ExternalServer is the URL of an external terminology server (e.g., tx.fhir.org).
	// Leave empty to use only local (embedded) terminology.
	ExternalServer string

	// ExternalSystems lists CodeSystem URLs that should be validated
	// against the external server (e.g., http://loinc.org, http://snomed.info/sct).
	ExternalSystems []string

	// CachePath is the directory for persistent terminology cache.
	// Leave empty for in-memory cache only.
	CachePath string

	// CacheTTL is the time-to-live for cached terminology lookups.
	// Default is 24 hours.
	CacheTTL time.Duration

	// Timeout for external terminology server calls.
	// Default is 10 seconds.
	Timeout time.Duration
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		LoadTerminology: true, // Load terminology by default
		TerminologyOptions: TerminologyOptions{
			CacheTTL: 24 * time.Hour,
			Timeout:  10 * time.Second,
		},
		PackageSource:   PackageSourceAuto,
		ParallelLoading: true,
		Workers:         4,
	}
}

// DefaultTerminologyOptions returns TerminologyOptions with sensible defaults.
func DefaultTerminologyOptions() TerminologyOptions {
	return TerminologyOptions{
		CacheTTL: 24 * time.Hour,
		Timeout:  10 * time.Second,
	}
}

// WithTerminology returns Options with terminology loading enabled.
func WithTerminology() Options {
	opts := DefaultOptions()
	opts.LoadTerminology = true
	return opts
}

// WithExternalTerminology returns Options with external terminology server configured.
func WithExternalTerminology(serverURL string, systems ...string) Options {
	opts := DefaultOptions()
	opts.LoadTerminology = true
	opts.TerminologyOptions.ExternalServer = serverURL
	opts.TerminologyOptions.ExternalSystems = systems
	return opts
}

// WithPackages returns Options with additional packages to load.
func WithPackages(packages ...string) Options {
	opts := DefaultOptions()
	opts.AdditionalPackages = packages
	return opts
}

// WithOffline returns Options that only use cached/embedded packages.
func WithOffline() Options {
	opts := DefaultOptions()
	opts.OfflineMode = true
	opts.PackageSource = PackageSourceEmbedded
	return opts
}

// WithRegistry returns Options that download from the package registry.
func WithRegistry() Options {
	opts := DefaultOptions()
	opts.PackageSource = PackageSourceRegistry
	return opts
}

// WithEmbedded returns Options that only use embedded specs.
func WithEmbedded() Options {
	opts := DefaultOptions()
	opts.PackageSource = PackageSourceEmbedded
	return opts
}

// WithCacheDir returns Options with a custom cache directory.
func WithCacheDir(dir string) Options {
	opts := DefaultOptions()
	opts.CacheDir = dir
	return opts
}
