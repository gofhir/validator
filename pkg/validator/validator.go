// Package validator provides a FHIR resource validator.
package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/gofhir/fhirpath/funcs"

	"github.com/gofhir/validator/pkg/binding"
	"github.com/gofhir/validator/pkg/cardinality"
	"github.com/gofhir/validator/pkg/constraint"
	"github.com/gofhir/validator/pkg/extension"
	"github.com/gofhir/validator/pkg/fixedpattern"
	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/location"
	"github.com/gofhir/validator/pkg/logger"
	"github.com/gofhir/validator/pkg/primitive"
	"github.com/gofhir/validator/pkg/reference"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/slicing"
	"github.com/gofhir/validator/pkg/specs"
	"github.com/gofhir/validator/pkg/structural"
	"github.com/gofhir/validator/pkg/terminology"
)

func init() {
	// Disable FHIRPath trace() output by default.
	// The trace() function is used in some FHIR constraints (e.g., dom-3)
	// and outputs debug information that should only appear when explicitly enabled.
	funcs.SetTraceLogger(funcs.NullTraceLogger{})
}

// Validator is the main FHIR resource validator.
type Validator struct {
	registry     *registry.Registry
	termRegistry *terminology.Registry
	loader       *loader.Loader
	config       *Config

	// Phase validators (reused across validations for caching)
	structValidator       *structural.Validator
	cardValidator         *cardinality.Validator
	primValidator         *primitive.Validator
	bindValidator         *binding.Validator
	extValidator          *extension.Validator
	refValidator          *reference.Validator
	constraintValidator   *constraint.Validator
	fixedPatternValidator *fixedpattern.Validator
	slicingValidator      *slicing.Validator
}

// PackageSpec represents an additional FHIR package to load.
type PackageSpec struct {
	Name    string
	Version string
}

// Config holds the validator configuration.
type Config struct {
	FHIRVersion          string               // e.g., "4.0.1", "4.3.0", "5.0.0"
	Profiles             []string             // Additional profiles to validate against
	StrictMode           bool                 // Treat warnings as errors
	PackagePath          string               // Path to FHIR package cache
	AdditionalPackages   []PackageSpec        // Additional packages to load (e.g., US Core)
	PackageTgzPaths      []string             // Paths to local .tgz package files
	PackageURLs          []string             // URLs to remote .tgz package files
	PackageData          [][]byte             // In-memory .tgz package bytes (e.g., from //go:embed)
	ConformanceResources [][]byte             // Individual conformance resource JSON bytes (e.g., from DB)
	TerminologyProvider  terminology.Provider // Optional external terminology provider
}

// Option is a functional option for configuring the validator.
type Option func(*Config)

// WithVersion sets the FHIR version.
func WithVersion(version string) Option {
	return func(c *Config) {
		c.FHIRVersion = version
	}
}

// WithProfile adds a profile URL to validate against.
func WithProfile(profileURL string) Option {
	return func(c *Config) {
		c.Profiles = append(c.Profiles, profileURL)
	}
}

// WithStrictMode enables strict mode (warnings become errors).
func WithStrictMode(strict bool) Option {
	return func(c *Config) {
		c.StrictMode = strict
	}
}

// WithPackagePath sets the FHIR package cache path.
func WithPackagePath(path string) Option {
	return func(c *Config) {
		c.PackagePath = path
	}
}

// WithPackage adds an additional FHIR package to load (e.g., US Core, IPS).
func WithPackage(name, version string) Option {
	return func(c *Config) {
		c.AdditionalPackages = append(c.AdditionalPackages, PackageSpec{Name: name, Version: version})
	}
}

// WithPackageTgz adds a local .tgz package file to load.
func WithPackageTgz(path string) Option {
	return func(c *Config) {
		c.PackageTgzPaths = append(c.PackageTgzPaths, path)
	}
}

// WithPackageURL adds a remote .tgz package URL to load.
func WithPackageURL(url string) Option {
	return func(c *Config) {
		c.PackageURLs = append(c.PackageURLs, url)
	}
}

// WithPackageData loads a FHIR package from .tgz bytes in memory.
// Useful for packages embedded in the binary via //go:embed.
func WithPackageData(data []byte) Option {
	return func(c *Config) {
		c.PackageData = append(c.PackageData, data)
	}
}

// WithConformanceResources loads individual conformance resources (JSON bytes)
// directly into the validator's registry. Each entry should be a valid JSON
// FHIR conformance resource (StructureDefinition, ValueSet, CodeSystem, etc.).
func WithConformanceResources(resources [][]byte) Option {
	return func(c *Config) {
		c.ConformanceResources = append(c.ConformanceResources, resources...)
	}
}

// WithTerminologyProvider sets an external terminology provider for validating
// codes in systems that cannot be expanded locally (e.g., SNOMED CT, LOINC).
// When configured, the validator delegates to this provider instead of silently
// accepting any code from external systems.
func WithTerminologyProvider(provider terminology.Provider) Option {
	return func(c *Config) {
		c.TerminologyProvider = provider
	}
}

// validateConfig holds per-call validation options.
type validateConfig struct {
	profiles []string
}

// ValidateOption configures a single Validate call.
type ValidateOption func(*validateConfig)

// ValidateWithProfile adds a profile URL to validate against for this call only.
// Does not modify the Validator's construction-time config.
func ValidateWithProfile(profileURL string) ValidateOption {
	return func(c *validateConfig) {
		c.profiles = append(c.profiles, profileURL)
	}
}

// New creates a new Validator with the given options.
func New(opts ...Option) (*Validator, error) {
	startTime := time.Now()
	startMem := getMemUsage()

	config := &Config{
		FHIRVersion: "4.0.1", // Default to R4
	}

	for _, opt := range opts {
		opt(config)
	}

	logger.Info("Initializing FHIR Validator v%s", config.FHIRVersion)
	logger.Info("  Memory at start: %s", formatBytes(startMem))

	l := loader.NewLoader(config.PackagePath)
	logger.Debug("Package cache: %s", l.BasePath())

	// Load packages for the specified FHIR version (embedded-first, fallback to disk)
	logger.Info("Loading FHIR packages...")
	loadStart := time.Now()
	var packages []*loader.Package //nolint:prealloc // assigned from branch, not built by appending
	var err error
	if embeddedData := specs.GetPackages(config.FHIRVersion); len(embeddedData) > 0 {
		logger.Info("  Using embedded specs for %s", config.FHIRVersion)
		packages, err = l.LoadFromEmbeddedData(embeddedData)
	} else {
		logger.Info("  Loading specs from disk for %s", config.FHIRVersion)
		packages, err = l.LoadVersion(config.FHIRVersion)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load FHIR packages: %w", err)
	}

	// Load additional packages (e.g., US Core, IPS)
	for _, pkgSpec := range config.AdditionalPackages {
		pkg, err := l.LoadPackage(pkgSpec.Name, pkgSpec.Version)
		if err != nil {
			logger.Warn("Could not load additional package %s#%s: %v", pkgSpec.Name, pkgSpec.Version, err)
			continue
		}
		packages = append(packages, pkg)
	}

	// Load packages from local .tgz files
	for _, tgzPath := range config.PackageTgzPaths {
		pkg, err := l.LoadFromTgz(tgzPath)
		if err != nil {
			logger.Warn("Could not load package from tgz %s: %v", tgzPath, err)
			continue
		}
		logger.Info("  Loaded package from tgz: %s#%s", pkg.Name, pkg.Version)
		packages = append(packages, pkg)
	}

	// Load packages from remote URLs
	for _, url := range config.PackageURLs {
		pkg, err := l.LoadFromURL(url)
		if err != nil {
			logger.Warn("Could not load package from URL %s: %v", url, err)
			continue
		}
		logger.Info("  Loaded package from URL: %s#%s", pkg.Name, pkg.Version)
		packages = append(packages, pkg)
	}

	// Load packages from in-memory .tgz data (e.g., //go:embed)
	for i, data := range config.PackageData {
		pkg, err := l.LoadFromTgzData(data)
		if err != nil {
			logger.Warn("Could not load package from memory data[%d]: %v", i, err)
			continue
		}
		logger.Info("  Loaded package from memory: %s#%s", pkg.Name, pkg.Version)
		packages = append(packages, pkg)
	}

	// Load individual conformance resources from memory (e.g., from database)
	if len(config.ConformanceResources) > 0 {
		pkg, err := l.LoadFromResources(config.ConformanceResources)
		if err != nil {
			logger.Warn("Could not load conformance resources: %v", err)
		} else {
			logger.Info("  Loaded %d conformance resources from memory", len(pkg.Resources))
			packages = append(packages, pkg)
		}
	}

	loadDuration := time.Since(loadStart)

	// Log loaded packages
	totalResources := 0
	for _, pkg := range packages {
		logger.Info("  Loaded %s#%s (%d resources)", pkg.Name, pkg.Version, len(pkg.Resources))
		totalResources += len(pkg.Resources)
	}
	afterLoadMem := getMemUsage()
	logger.Info("  Total: %d resources from %d packages in %v", totalResources, len(packages), loadDuration.Round(time.Millisecond))
	logger.Info("  Memory after load: %s (+%s)", formatBytes(afterLoadMem), formatBytes(afterLoadMem-startMem))

	// Create and populate the registry
	logger.Info("Building StructureDefinition registry...")
	registryStart := time.Now()
	reg := registry.New()
	if err := reg.LoadFromPackages(packages); err != nil {
		return nil, fmt.Errorf("failed to load StructureDefinitions: %w", err)
	}
	registryDuration := time.Since(registryStart)
	afterRegistryMem := getMemUsage()

	logger.Info("  Indexed %d StructureDefinitions, %d types in %v", reg.Count(), reg.TypeCount(), registryDuration.Round(time.Millisecond))
	logger.Info("  Memory after registry: %s (+%s)", formatBytes(afterRegistryMem), formatBytes(afterRegistryMem-afterLoadMem))

	// Create and populate the terminology registry
	logger.Debug("Building terminology registry...")
	termReg := terminology.NewRegistry()
	if err := termReg.LoadFromPackages(packages); err != nil {
		return nil, fmt.Errorf("failed to load terminology: %w", err)
	}
	logger.Debug("  Indexed %d ValueSets, %d CodeSystems", termReg.ValueSetCount(), termReg.CodeSystemCount())

	if config.TerminologyProvider != nil {
		termReg.SetProvider(config.TerminologyProvider)
		logger.Debug("  External terminology provider configured")
	}

	totalDuration := time.Since(startTime)
	totalMemUsed := getMemUsage() - startMem
	logger.Info("Validator ready in %v (total memory: %s)", totalDuration.Round(time.Millisecond), formatBytes(totalMemUsed))

	// Create phase validators (reused across validations for caching)
	v := &Validator{
		registry:     reg,
		termRegistry: termReg,
		loader:       l,
		config:       config,
	}

	// Initialize phase validators
	v.structValidator = structural.New(reg)
	v.cardValidator = cardinality.New(reg)
	v.primValidator = primitive.New(reg)
	v.bindValidator = binding.New(reg, termReg)
	v.extValidator = extension.New(reg, termReg, v.primValidator)
	v.refValidator = reference.New(reg)
	v.constraintValidator = constraint.New(reg)
	v.fixedPatternValidator = fixedpattern.New(reg)
	v.slicingValidator = slicing.New(reg)

	return v, nil
}

// getMemUsage returns the current memory allocation in bytes.
func getMemUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// formatBytes formats bytes as human-readable string.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Validate validates a FHIR resource and returns the validation result.
// According to the FHIR specification, when a resource declares multiple profiles
// in meta.profile, it MUST be valid against ALL of them.
// Optional ValidateOption parameters allow per-call configuration (e.g., ValidateWithProfile).
func (v *Validator) Validate(ctx context.Context, resource []byte, opts ...ValidateOption) (*issue.Result, error) {
	startTime := time.Now()

	// Apply per-call options
	var vc validateConfig
	for _, opt := range opts {
		opt(&vc)
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := issue.NewResult()
	result.Stats = &issue.Stats{
		ResourceSize: len(resource),
	}

	// Parse JSON once - this parsed data will be shared across all validation phases
	var data map[string]any
	if err := json.Unmarshal(resource, &data); err != nil {
		result.AddError(issue.CodeStructure, fmt.Sprintf("Invalid JSON: %v", err))
		result.Stats.Duration = time.Since(startTime).Nanoseconds()
		return result, nil
	}

	// Extract resourceType and meta from parsed data
	resourceType, _ := data["resourceType"].(string)
	result.Stats.ResourceType = resourceType

	if resourceType == "" {
		result.AddError(issue.CodeStructure, "Missing 'resourceType' property")
		result.Stats.Duration = time.Since(startTime).Nanoseconds()
		return result, nil
	}

	// Extract meta.profile if present
	var metaProfiles []string
	if meta, ok := data["meta"].(map[string]any); ok {
		if profiles, ok := meta["profile"].([]any); ok {
			for _, p := range profiles {
				if ps, ok := p.(string); ok {
					metaProfiles = append(metaProfiles, ps)
				}
			}
		}
	}

	// Get core resource StructureDefinition (always validate against this)
	coreURL := registry.GetSDForResource(resourceType)
	coreSD := v.registry.GetByURL(coreURL)

	if coreSD == nil {
		result.AddError(issue.CodeStructure, fmt.Sprintf("Unknown resourceType '%s'", resourceType))
		result.Stats.Duration = time.Since(startTime).Nanoseconds()
		return result, nil
	}

	// Collect all profiles to validate against (metaProfiles already extracted above)
	customProfiles := v.collectProfilesToValidate(vc.profiles, metaProfiles)

	// Resolve profiles from registry
	var resolvedProfiles []*registry.StructureDefinition
	var profileURLs []string
	var profilesNotFound []string

	for _, profileURL := range customProfiles {
		sd := v.registry.GetByURL(profileURL)
		if sd != nil {
			resolvedProfiles = append(resolvedProfiles, sd)
			profileURLs = append(profileURLs, profileURL)
		} else {
			profilesNotFound = append(profilesNotFound, profileURL)
		}
	}

	// Emit warnings for profiles not found
	for _, notFound := range profilesNotFound {
		result.AddIssue(issue.Issue{
			Severity:    issue.SeverityWarning,
			Code:        issue.CodeNotFound,
			Diagnostics: fmt.Sprintf("Profile '%s' not found in registry", notFound),
		})
	}

	// Determine which profiles to validate against
	// If custom profiles found, validate against all of them
	// If no custom profiles, validate against core only
	var profilesToValidate []*registry.StructureDefinition
	var profileURLsToValidate []string

	if len(resolvedProfiles) > 0 {
		profilesToValidate = resolvedProfiles
		profileURLsToValidate = profileURLs
		result.Stats.IsCustomProfile = true
	} else {
		profilesToValidate = []*registry.StructureDefinition{coreSD}
		profileURLsToValidate = []string{coreURL}
		result.Stats.IsCustomProfile = false
	}

	// Store first profile URL for stats (backward compatibility)
	result.Stats.ProfileURL = profileURLsToValidate[0]

	// Log validation info
	logger.Info("Validating %s (%s, %d bytes) against %d profile(s)",
		resourceType,
		formatBytes(uint64(len(resource))),
		len(resource),
		len(profilesToValidate),
	)
	for _, url := range profileURLsToValidate {
		logger.Debug("  Profile: %s", url)
	}

	// Emit informational issue about profiles being validated
	if len(profileURLsToValidate) > 1 {
		result.AddIssue(issue.Issue{
			Severity:    issue.SeverityInformation,
			Code:        issue.CodeInformational,
			Diagnostics: fmt.Sprintf("Validating against %d profiles: %v", len(profileURLsToValidate), profileURLsToValidate),
		})
	}

	// Validate against ALL profiles
	// According to FHIR spec, resource must be valid against all claimed profiles
	// Pass parsed data to avoid re-parsing JSON in each phase
	for i, sd := range profilesToValidate {
		profileURL := profileURLsToValidate[i]
		v.validateAgainstProfile(data, resource, sd, profileURL, result)
	}

	result.Stats.Duration = time.Since(startTime).Nanoseconds()

	// Enrich issues with line/column information from source JSON
	result.EnrichLocations(func(expr string) *issue.Location {
		if loc := location.Find(resource, expr); loc != nil {
			return &issue.Location{Line: loc.Line, Column: loc.Column}
		}
		return nil
	})

	logger.Info("Validated %s in %.3fms: %d errors, %d warnings",
		resourceType,
		result.Stats.DurationMs(),
		result.ErrorCount(),
		result.WarningCount(),
	)

	return result, nil
}

// ValidateAgainstProfile runs all validation phases against a single profile.
// Data is the pre-parsed JSON map, rawJSON is kept for phases that need raw bytes (constraint/fhirpath).
func (v *Validator) validateAgainstProfile(data map[string]any, rawJSON []byte, sd *registry.StructureDefinition, _ string, result *issue.Result) {
	// Phase 1: Structural validation (uses cached element indexes)
	structResult := v.structValidator.ValidateData(data, sd)
	result.Merge(structResult)
	issue.ReleaseResult(structResult)
	result.Stats.PhasesRun++

	// Phase 2: Cardinality validation
	cardResult := v.cardValidator.ValidateData(data, sd)
	result.Merge(cardResult)
	issue.ReleaseResult(cardResult)
	result.Stats.PhasesRun++

	// Phase 3: Primitive type validation (uses cached regex)
	primResult := v.primValidator.ValidateData(data, sd)
	result.Merge(primResult)
	issue.ReleaseResult(primResult)
	result.Stats.PhasesRun++

	// Phase 4: Binding validation (terminology)
	v.bindValidator.ValidateData(data, sd, result)
	result.Stats.PhasesRun++

	// Phase 5: Extension validation
	v.extValidator.ValidateData(data, sd, result)
	result.Stats.PhasesRun++

	// Phase 6: Reference validation
	// For Bundles, create a BundleContext to validate urn:uuid references
	var bundleCtx *reference.BundleContext
	if resourceType, _ := data["resourceType"].(string); resourceType == "Bundle" {
		bundleCtx = reference.NewBundleContext(data)
		// Validate Bundle-specific rules: fullUrl must be consistent with resource.id
		reference.ValidateBundleFullUrls(data, result)
	}
	v.refValidator.ValidateDataWithBundle(data, sd, bundleCtx, result)
	result.Stats.PhasesRun++

	// Phase 7: Constraint validation (FHIRPath, uses cached expressions)
	// Note: constraint validation needs raw bytes for FHIRPath evaluation
	v.constraintValidator.Validate(rawJSON, sd, result)
	result.Stats.PhasesRun++

	// Phase 8: Fixed/Pattern value validation
	v.fixedPatternValidator.ValidateData(data, sd, result)
	result.Stats.PhasesRun++

	// Phase 9: Slicing validation
	v.slicingValidator.ValidateData(data, sd, result)
	result.Stats.PhasesRun++
}

// ValidateJSON validates a FHIR resource from a JSON string.
func (v *Validator) ValidateJSON(ctx context.Context, jsonStr string, opts ...ValidateOption) (*issue.Result, error) {
	return v.Validate(ctx, []byte(jsonStr), opts...)
}

// Registry returns the underlying registry for advanced use cases.
func (v *Validator) Registry() *registry.Registry {
	return v.registry
}

// Config returns the validator configuration.
func (v *Validator) Config() *Config {
	return v.config
}

// Version returns the FHIR version being used.
func (v *Validator) Version() string {
	return v.config.FHIRVersion
}

// collectProfilesToValidate returns the ordered list of profiles to validate against.
// Priority: 1) Per-call profiles, 2) Config profiles, 3) meta.profile, 4) core resource SD.
func (v *Validator) collectProfilesToValidate(perCallProfiles, metaProfiles []string) []string {
	var profiles []string

	// 1. Per-call profiles take highest priority
	profiles = append(profiles, perCallProfiles...)

	// 2. Configured profiles
	profiles = append(profiles, v.config.Profiles...)

	// 3. Profiles from meta.profile
	profiles = append(profiles, metaProfiles...)

	// 4. Core resource type as fallback (added at validation time if needed)
	// Not added here to allow detecting if all custom profiles failed

	return profiles
}
