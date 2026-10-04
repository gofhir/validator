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
	"github.com/gofhir/validator/pkg/slicematch"
	"github.com/gofhir/validator/pkg/slicing"
	"github.com/gofhir/validator/pkg/specs"
	"github.com/gofhir/validator/pkg/structural"
	"github.com/gofhir/validator/pkg/terminology"
	"github.com/gofhir/validator/pkg/ucumvalidator"

	"github.com/gofhir/ucum/v4"
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
	ucumValidator         *ucumvalidator.Validator
}

// PackageSpec represents an additional FHIR package to load.
type PackageSpec struct {
	Name    string
	Version string
}

// ConformancePackage is a group of conformance resources tagged with their
// source FHIR package metadata. Use with WithConformancePackage so that
// Registry.GetProfilesByPackage and ValidateWithIG can correctly scope by IG.
type ConformancePackage struct {
	Name      string   // e.g., "hl7.fhir.us.core"
	Version   string   // e.g., "6.1.0"
	Resources [][]byte // FHIR conformance resource JSON bytes (StructureDefinition, ValueSet, etc.)
}

// Config holds the validator configuration.
type Config struct {
	FHIRVersion          string                   // e.g., "4.0.1", "4.3.0", "5.0.0"
	Profiles             []string                 // Additional profiles to validate against
	StrictMode           bool                     // Treat warnings as errors
	PackagePath          string                   // Path to FHIR package cache
	BasePackages         []PackageSpec            // The base packages to load instead of the version's embedded set
	AdditionalPackages   []PackageSpec            // Additional packages to load (e.g., US Core)
	PackageRegistry      string                   // Package registry to download missing packages from; empty downloads none
	PackageTgzPaths      []string                 // Paths to local .tgz package files
	PackageURLs          []string                 // URLs to remote .tgz package files
	PackageData          [][]byte                 // In-memory .tgz package bytes (e.g., from //go:embed)
	ConformanceResources [][]byte                 // Individual conformance resource JSON bytes (e.g., from DB)
	ConformancePackages  []ConformancePackage     // Conformance resources grouped by source IG package
	TerminologyProvider  terminology.Provider     // Optional external terminology provider
	TerminologyAuthority terminology.Authority    // Optional authoritative terminology port (replaces the base copy)
	ProfileResolver      registry.ProfileResolver // Optional external profile resolver for on-demand SD loading
	NoTerminology        bool                     // When true, skip all terminology/binding validation
	UnresolvedPolicy     UnresolvedPolicy         // How to report bindings no terminology source could decide
	DisplayLanguage      string                   // BCP-47 tag preferred when checking Coding.display
}

// UnresolvedPolicy decides what happens when no terminology source can decide a
// binding. Aliased from pkg/terminology, where the phase validators consume it.
type UnresolvedPolicy = terminology.UnresolvedPolicy

// Unresolved policies. See terminology.UnresolvedPolicy.
const (
	UnresolvedWarn  = terminology.UnresolvedWarn
	UnresolvedError = terminology.UnresolvedError
)

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

// WithBasePackages sets the base packages to load, from the package cache, instead of the set the
// validator embeds for its FHIR version (the core package, terminology and extensions): to validate
// against given versions of them, such as the ones another validator uses. Any package missing
// from the set is not loaded.
func WithBasePackages(packages ...PackageSpec) Option {
	return func(c *Config) {
		c.BasePackages = append(c.BasePackages, packages...)
	}
}

// WithPackageRegistry sets the package registry (such as loader.DefaultRegistry) that packages
// missing from the package cache are downloaded from, into the cache: the packages added with
// WithPackage and the packages they depend on. Without it, nothing is downloaded, and a dependency
// missing from the cache is reported and not loaded.
func WithPackageRegistry(url string) Option {
	return func(c *Config) {
		c.PackageRegistry = url
	}
}

// WithPackagePath sets the FHIR package cache path.
func WithPackagePath(path string) Option {
	return func(c *Config) {
		c.PackagePath = path
	}
}

// WithPackage adds an additional FHIR package to load (e.g., US Core, IPS), from the package cache.
// The packages it depends on are loaded too, transitively, in the versions it declares; see
// WithPackageRegistry for those missing from the cache.
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
//
// Resources loaded this way are tagged with PackageID "custom#0.0.0" — IG-scoped
// lookups via Registry.GetProfilesByPackage / ValidateWithIG will not find them.
// Prefer WithConformancePackage when the source IG metadata is known.
func WithConformanceResources(resources [][]byte) Option {
	return func(c *Config) {
		c.ConformanceResources = append(c.ConformanceResources, resources...)
	}
}

// WithConformancePackage loads conformance resources tagged with their source
// IG package metadata. Each StructureDefinition / ValueSet / CodeSystem will
// carry PackageID "<name>#<version>" so Registry.GetProfilesByPackage and
// ValidateWithIG can scope by IG.
//
// Use this when conformance resources originate from a known FHIR package but
// are stored outside the on-disk package cache (for example, in a database
// table populated by an IG-install pipeline).
func WithConformancePackage(name, version string, resources [][]byte) Option {
	return func(c *Config) {
		c.ConformancePackages = append(c.ConformancePackages, ConformancePackage{
			Name:      name,
			Version:   version,
			Resources: resources,
		})
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

// WithTerminologyAuthority delegates all terminology resolution to a, and skips
// loading the embedded base ValueSets/CodeSystems entirely.
//
// Use it for embedded deployments where the host already owns terminology — a
// FHIR server whose chain holds the base vocabularies, resources authored over
// its API, and any configured remote terminology server. It removes the second
// in-memory copy of the base terminology from the process, and lets binding
// validation see ValueSets and CodeSystems created after the validator was
// constructed, which a constructor-time snapshot cannot.
//
// Unlike WithTerminologyProvider — which supplements the local copies and is
// consulted only for systems that cannot be expanded locally — this replaces
// them. The authority answers every lookup, so it must be prepared to resolve
// the base vocabularies too. When both options are given, the authority wins.
func WithTerminologyAuthority(a terminology.Authority) Option {
	return func(c *Config) {
		c.TerminologyAuthority = a
	}
}

// WithUnresolvedPolicy decides how a binding that no terminology source could
// resolve is reported: UnresolvedWarn (default) accepts the code and records an
// informational issue, UnresolvedError treats it as a validation failure.
//
// The default matches the HL7 validator's -tx n/a behavior. Choose
// UnresolvedError for closed-world validation, where accepting an unchecked code
// is worse than rejecting it.
func WithUnresolvedPolicy(p UnresolvedPolicy) Option {
	return func(c *Config) {
		c.UnresolvedPolicy = p
	}
}

// WithDisplayLanguage sets the BCP-47 language preferred when checking
// Coding.display against the concept's display.
//
// Without it, display validation compares against whatever display the
// terminology source returns by default, which for the base FHIR packages is
// English. Set it when resources carry displays in another language, so a valid
// translation is not reported as a mismatch.
//
// When the terminology source cannot supply a display in the requested language,
// display validation is skipped rather than compared against a fallback.
func WithDisplayLanguage(lang string) Option {
	return func(c *Config) {
		c.DisplayLanguage = lang
	}
}

// WithNoTerminology disables all terminology/binding validation (the CLI's -no-terminology).
// It is not the HL7 Validator's "-tx n/a", which only leaves out a terminology server and still
// checks codes against the ValueSets and CodeSystems loaded, as this validator does by default.
func WithNoTerminology() Option {
	return func(c *Config) {
		c.NoTerminology = true
	}
}

// WithProfileResolver sets an external profile resolver for on-demand SD loading.
// When configured, the registry falls back to this resolver for profiles not found in memory.
// This is optional: in standalone mode (CLI), the validator pre-loads all SDs at init.
// In server mode, the server provides a resolver backed by its conformance store.
func WithProfileResolver(resolver registry.ProfileResolver) Option {
	return func(c *Config) {
		c.ProfileResolver = resolver
	}
}

// canonicalRef represents a parsed canonical reference with URL and optional version.
type canonicalRef struct {
	url     string
	version string
}

// ValidateMode represents the validation mode for the $validate operation.
// Per FHIR R4, mode affects which rules apply during validation.
type ValidateMode string

const (
	// ValidateModeNone means no specific mode — default validation rules apply.
	ValidateModeNone ValidateMode = ""

	// ValidateModeCreate validates as if the resource is being created.
	// Server-assigned fields (id, meta.versionId, meta.lastUpdated) are not required.
	ValidateModeCreate ValidateMode = "create"

	// ValidateModeUpdate validates as if the resource is being updated.
	// The id element must be present.
	ValidateModeUpdate ValidateMode = "update"

	// ValidateModeDelete validates as if the resource is being deleted.
	// Only minimal validation is performed (resourceType and id must be present).
	ValidateModeDelete ValidateMode = "delete"
)

// validateConfig holds per-call validation options.
type validateConfig struct {
	profiles          []string
	canonicalProfiles []canonicalRef
	mode              ValidateMode
	ig                string // Package ID for IG-scoped validation (e.g., "hl7.fhir.us.core#6.1.0")
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

// ValidateWithCanonicalProfile adds a canonical reference (url|version) as a profile
// to validate against for this call only. The version is used for version-aware resolution.
// If no "|" is present, behaves like ValidateWithProfile.
func ValidateWithCanonicalProfile(canonical string) ValidateOption {
	return func(c *validateConfig) {
		url, version := registry.ParseCanonical(canonical)
		c.canonicalProfiles = append(c.canonicalProfiles, canonicalRef{url: url, version: version})
	}
}

// ValidateWithIG sets the implementation guide package for this call.
// When set, profiles from the specified IG that match the resource type are automatically
// added to validation. The ig parameter is a package ID (e.g., "hl7.fhir.us.core#6.1.0").
// The IG package must have been loaded at construction time via WithPackage.
//
// See https://hl7.org/fhir/R4/resource-operation-validate.html
func ValidateWithIG(ig string) ValidateOption {
	return func(c *validateConfig) {
		c.ig = ig
	}
}

// ValidateWithMode sets the validation mode for this call.
// Per FHIR R4, mode affects which rules apply:
//   - "create": server-assigned fields (id, meta.versionId, meta.lastUpdated) are not required
//   - "update": id must be present
//   - "delete": only minimal validation is performed
//
// See https://hl7.org/fhir/R4/resource-operation-validate.html
func ValidateWithMode(mode ValidateMode) ValidateOption {
	return func(c *validateConfig) {
		c.mode = mode
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
	packages, err := loadBase(l, config)
	if err != nil {
		return nil, fmt.Errorf("failed to load FHIR packages: %w", err)
	}

	base := len(packages)

	// Load additional packages (e.g., US Core, IPS)
	for _, pkgSpec := range config.AdditionalPackages {
		version, err := dependencyVersion(l, config, pkgSpec.Name, pkgSpec.Version)
		if err != nil {
			logger.Warn("Could not load additional package %s#%s: %v", pkgSpec.Name, pkgSpec.Version, err)
			continue
		}
		pkg, err := l.OpenPackage(pkgSpec.Name, version)
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

	// Load conformance resources from memory (raw + IG-tagged).
	packages = loadConformanceResources(l, config, packages)

	// A package added once, and one core package, the version validated's; then the packages those
	// added depend on.
	packages = append(packages[:base], addedPackages(packages[:base], packages[base:], config.FHIRVersion)...)
	packages = append(packages, loadDependencies(l, config, packages, packages[base:])...)

	loadDuration := time.Since(loadStart)

	// Log loaded packages
	totalResources := 0
	for _, pkg := range packages {
		logger.Info("  Loaded %s#%s (%d resources, %d read when needed)", pkg.Name, pkg.Version, len(pkg.Resources), len(pkg.Deferred))
		totalResources += len(pkg.Resources) + len(pkg.Deferred)
	}
	afterLoadMem := getMemUsage()
	logger.Info("  Total: %d resources from %d packages in %v", totalResources, len(packages), loadDuration.Round(time.Millisecond))
	logger.Info("  Memory after load: %s (+%s)", formatBytes(afterLoadMem), formatBytes(afterLoadMem-startMem))

	// Create and populate the registry
	logger.Info("Building StructureDefinition registry...")
	registryStart := time.Now()
	reg := registry.New()
	reg.SetFHIRVersion(config.FHIRVersion)
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
	termReg.SetFHIRVersion(config.FHIRVersion)

	if config.TerminologyAuthority != nil {
		// The host owns terminology resolution, so parsing our own copy of the
		// base ValueSets/CodeSystems would be dead weight — this is where the
		// duplicate in-memory copy is avoided.
		termReg.SetAuthority(config.TerminologyAuthority)
		logger.Debug("  Terminology authority configured; base terminology not loaded")
	} else {
		if err := termReg.LoadFromPackages(packages); err != nil {
			return nil, fmt.Errorf("failed to load terminology: %w", err)
		}
		logger.Debug("  Indexed %d ValueSets, %d CodeSystems", termReg.ValueSetCount(), termReg.CodeSystemCount())

		if config.TerminologyProvider != nil {
			termReg.SetProvider(config.TerminologyProvider)
			logger.Debug("  External terminology provider configured")
		}
	}

	if config.ProfileResolver != nil {
		reg.SetResolver(config.ProfileResolver)
		logger.Debug("  External profile resolver configured")
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
	v.primValidator = primitive.New(reg)
	v.bindValidator = binding.New(reg, termReg)
	v.bindValidator.SetUnresolvedPolicy(config.UnresolvedPolicy)
	v.bindValidator.SetDisplayLanguage(config.DisplayLanguage)
	v.extValidator = extension.New(reg, termReg, v.primValidator)
	// Extension values are bound like any other element, so they are validated by
	// the same code rather than a second copy of it.
	v.extValidator.SetBindingValidator(v.bindValidator)
	// An extension is checked against the definition its url names by the phases that walk
	// definitions, as by the element that holds it.
	v.cardValidator = cardinality.New(reg, cardinality.WithDefinitions(v.extValidator))
	v.refValidator = reference.New(reg)
	// Pass termRegistry to constraint validator for memberOf() support.
	// When NoTerminology is set, pass nil to disable terminology in FHIRPath.
	constraintTermReg := termReg
	if config.NoTerminology {
		constraintTermReg = nil
	}
	// One slice matcher for the phases that tell the slice a value belongs to, so that they
	// assign it to the same one.
	matcher := slicematch.New(reg,
		slicematch.WithConformer(conformer{v: v}),
		slicematch.WithMemberChecker(memberChecker{v: v}),
	)
	v.constraintValidator = constraint.New(reg, constraintTermReg, constraint.WithMatcher(matcher),
		constraint.WithDefinitions(v.extValidator))
	v.fixedPatternValidator = fixedpattern.New(reg)
	v.slicingValidator = slicing.NewWithMatcher(reg, matcher, slicing.WithDefinitions(v.extValidator))

	v.ucumValidator = initUCUMValidator(reg)

	return v, nil
}

// initUCUMValidator creates the UCUM validator for Quantity.code validation.
func initUCUMValidator(reg *registry.Registry) *ucumvalidator.Validator {
	ucumSvc, err := ucum.New()
	if err != nil {
		logger.Warn("Could not initialize UCUM service: %v", err)
		return nil
	}
	return ucumvalidator.New(reg, ucumSvc)
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

	// Validate mode parameter
	if vc.mode != ValidateModeNone && vc.mode != ValidateModeCreate && vc.mode != ValidateModeUpdate && vc.mode != ValidateModeDelete {
		return nil, fmt.Errorf("invalid validation mode %q: must be \"create\", \"update\", or \"delete\"", vc.mode)
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

	// Mode-specific validation: delete returns early, update checks id.
	if earlyReturn := v.validateMode(vc.mode, data, resourceType, result); earlyReturn {
		result.Stats.Duration = time.Since(startTime).Nanoseconds()
		return result, nil
	}

	// Extract meta.profile if present
	metaProfiles := extractMetaProfiles(data)

	// Get core resource StructureDefinition (always validate against this)
	coreURL := registry.GetSDForResource(resourceType)
	coreSD := v.registry.GetByURL(coreURL)

	if coreSD == nil {
		result.AddError(issue.CodeStructure, fmt.Sprintf("Unknown resourceType '%s'", resourceType))
		result.Stats.Duration = time.Since(startTime).Nanoseconds()
		return result, nil
	}

	// If an IG is specified, add its profiles that match this resource type
	if vc.ig != "" {
		vc.profiles = v.resolveIGProfiles(vc.ig, resourceType, vc.profiles, result)
	}

	// Collect and resolve all profiles to validate against
	customProfiles := v.collectProfilesToValidate(vc.profiles, metaProfiles)
	resolvedProfiles, profileURLs, profilesNotFound := v.resolveProfiles(ctx, vc.canonicalProfiles, customProfiles)

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
	// Conformance checks made by slice matching share one memo for this validation.
	ctx = withConformState(ctx)
	ctx = constraint.WithReportScope(ctx)
	for _, sd := range profilesToValidate {
		v.validateAgainstProfile(ctx, data, resource, sd, nil, result)
	}

	result.Stats.Duration = time.Since(startTime).Nanoseconds()

	enrichLocations(resource, result)

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
// The value scope is nil for the resource being validated, and set for a value checked for
// conformance inside it (a slice's profile discriminator), which may be a datatype or an extension.
func (v *Validator) validateAgainstProfile(ctx context.Context, data map[string]any, rawJSON []byte, sd *registry.StructureDefinition, vs *valueScope, result *issue.Result) {
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

	// Phase 4: Binding validation (terminology) — skipped when NoTerminology is set
	if !v.config.NoTerminology {
		v.bindValidator.ValidateData(ctx, data, sd, result)
	}
	result.Stats.PhasesRun++

	// Phase 5: Extension validation
	v.extValidator.ValidateData(ctx, data, sd, result)
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
	// Build constraint options: pass Bundle data for resolve() support.
	constraintOpts := &constraint.ValidateOptions{}
	if resourceType, _ := data["resourceType"].(string); resourceType == "Bundle" {
		constraintOpts.BundleData = data
	}
	if vs != nil {
		constraintOpts = vs.constraintOptions()
	}
	// Slice matching shares what it learns about a value with the slicing phase by the value's
	// identity: both read the same parse.
	constraintOpts.Data = data
	constraintOpts.Resolver, constraintOpts.Containment = referenceResolver{}, constraint.IsContainedIn
	v.constraintValidator.Validate(ctx, rawJSON, sd, constraintOpts, result)
	result.Stats.PhasesRun++

	// Phase 8: Fixed/Pattern value validation
	v.fixedPatternValidator.ValidateData(data, sd, result)
	result.Stats.PhasesRun++

	// Phase 9: Slicing validation
	sliceOpts := slicing.Options{Resolver: referenceResolver{}, Containment: constraint.IsContainedIn}
	if vs != nil {
		sliceOpts.Scope = &vs.scope
	}
	v.slicingValidator.ValidateDataContext(ctx, data, sd, sliceOpts, result)
	result.Stats.PhasesRun++

	// Phase 10: UCUM validation (Quantity.code syntax)
	if v.ucumValidator != nil {
		v.ucumValidator.ValidateData(data, sd, result)
	}
	result.Stats.PhasesRun++
}

// enrichLocations adds each issue's line and column in the source JSON, locating every issue in
// one reading of it.
func enrichLocations(resource []byte, result *issue.Result) {
	paths := make([]string, 0, len(result.Issues))
	for i := range result.Issues {
		if is := &result.Issues[i]; len(is.Expression) > 0 && is.Location == nil {
			paths = append(paths, is.Expression[0])
		}
	}
	located := location.FindAll(resource, paths)
	result.EnrichLocations(func(expr string) *issue.Location {
		if loc := located[expr]; loc != nil {
			return &issue.Location{Line: loc.Line, Column: loc.Column}
		}
		return nil
	})
}

// validateMode applies mode-specific validation rules.
// Returns true if validation should return early (delete mode).
func (v *Validator) validateMode(mode ValidateMode, data map[string]any, resourceType string, result *issue.Result) bool {
	switch mode {
	case ValidateModeDelete:
		if _, hasID := data["id"]; !hasID {
			result.AddErrorWithID(
				issue.DiagModeDeleteRequiresID,
				map[string]any{"resourceType": resourceType},
				resourceType+".id",
			)
		}
		return true
	case ValidateModeUpdate:
		if _, hasID := data["id"]; !hasID {
			result.AddErrorWithID(
				issue.DiagModeUpdateRequiresID,
				map[string]any{"resourceType": resourceType},
				resourceType+".id",
			)
		}
	}
	return false
}

// extractMetaProfiles extracts profile URLs from resource meta.profile.
func extractMetaProfiles(data map[string]any) []string {
	meta, ok := data["meta"].(map[string]any)
	if !ok {
		return nil
	}
	profiles, ok := meta["profile"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if ps, ok := p.(string); ok {
			result = append(result, ps)
		}
	}
	return result
}

// resolveIGProfiles finds profiles from an IG package that match the resource type.
func (v *Validator) resolveIGProfiles(ig, resourceType string, profiles []string, result *issue.Result) []string {
	igProfiles := v.registry.GetProfilesByPackage(ig)
	if len(igProfiles) == 0 {
		result.AddIssue(issue.Issue{
			Severity:    issue.SeverityWarning,
			Code:        issue.CodeNotFound,
			Diagnostics: fmt.Sprintf("No profiles found for IG package '%s'", ig),
		})
		return profiles
	}
	for _, sd := range igProfiles {
		if sd.Type == resourceType {
			profiles = append(profiles, sd.URL)
		}
	}
	return profiles
}

// ValidateJSON validates a FHIR resource from a JSON string.
func (v *Validator) ValidateJSON(ctx context.Context, jsonStr string, opts ...ValidateOption) (*issue.Result, error) {
	return v.Validate(ctx, []byte(jsonStr), opts...)
}

// Registry returns the underlying registry for advanced use cases.
func (v *Validator) Registry() *registry.Registry {
	return v.registry
}

// TerminologyRegistry returns the terminology registry in use, for introspection
// and diagnostics.
//
// It is not a terminology service. Its contents depend entirely on how this
// Validator was configured — it is empty under WithTerminologyAuthority, and
// otherwise holds only what the configured packages provided. It is also owned by
// the Validator and shared with its validation phases, so callers must treat it,
// and every resource reachable through it, as read-only: mutating anything it
// returns corrupts validation for the whole process.
//
// To share one terminology source across components, have the host own it and
// pass WithTerminologyAuthority rather than reading from here.
func (v *Validator) TerminologyRegistry() *terminology.Registry {
	return v.termRegistry
}

// Config returns the validator configuration.
func (v *Validator) Config() *Config {
	return v.config
}

// Version returns the FHIR version being used.
func (v *Validator) Version() string {
	return v.config.FHIRVersion
}

// resolveProfiles resolves canonical and plain URL profiles using version-aware lookup
// with optional resolver fallback. Returns resolved SDs, their URLs, and unresolved profile strings.
func (v *Validator) resolveProfiles(ctx context.Context, canonicals []canonicalRef, plainURLs []string) (resolved []*registry.StructureDefinition, urls, notFound []string) {
	// Canonical profiles (url|version) from per-call options
	for _, cp := range canonicals {
		sd := v.registry.ResolveByCanonical(ctx, cp.url, cp.version)
		if sd != nil {
			resolved = append(resolved, sd)
			urls = append(urls, cp.url)
		} else {
			canonical := cp.url
			if cp.version != "" {
				canonical = cp.url + "|" + cp.version
			}
			notFound = append(notFound, canonical)
		}
	}

	// Plain URL profiles (may contain url|version syntax in meta.profile)
	for _, profileURL := range plainURLs {
		url, version := registry.ParseCanonical(profileURL)
		sd := v.registry.ResolveByCanonical(ctx, url, version)
		if sd != nil {
			resolved = append(resolved, sd)
			urls = append(urls, url)
		} else {
			notFound = append(notFound, profileURL)
		}
	}

	// Ensure base definition chains are resolved for all profiles
	for _, sd := range resolved {
		v.registry.ResolveBaseChain(ctx, sd)
	}

	// Generate snapshots for profiles that only have differentials
	withSnapshots := make([]*registry.StructureDefinition, 0, len(resolved))
	withSnapshotURLs := make([]string, 0, len(resolved))
	for i, sd := range resolved {
		if err := v.registry.EnsureSnapshot(ctx, sd); err != nil {
			logger.Warn("Snapshot generation failed for %s: %v", urls[i], err)
			notFound = append(notFound, urls[i])
			continue
		}
		withSnapshots = append(withSnapshots, sd)
		withSnapshotURLs = append(withSnapshotURLs, urls[i])
	}

	return withSnapshots, withSnapshotURLs, notFound
}

// collectProfilesToValidate returns the ordered list of profiles to validate against.
// Priority: 1) Per-call profiles, 2) Config profiles, 3) meta.profile, 4) core resource SD.
func (v *Validator) collectProfilesToValidate(perCallProfiles, metaProfiles []string) []string {
	profiles := make([]string, 0, len(perCallProfiles)+len(v.config.Profiles)+len(metaProfiles))

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

// loadConformanceResources loads in-memory conformance resources into the
// package list. Handles both the legacy untagged form (ConformanceResources)
// and the IG-tagged form (ConformancePackages).
func loadConformanceResources(l *loader.Loader, config *Config, packages []*loader.Package) []*loader.Package {
	if len(config.ConformanceResources) > 0 {
		pkg, err := l.LoadFromResources(config.ConformanceResources)
		if err != nil {
			logger.Warn("Could not load conformance resources: %v", err)
		} else {
			logger.Info("  Loaded %d conformance resources from memory", len(pkg.Resources))
			packages = append(packages, pkg)
		}
	}
	for _, cp := range config.ConformancePackages {
		pkg, err := l.LoadFromResourcesWithMeta(cp.Name, cp.Version, cp.Resources)
		if err != nil {
			logger.Warn("Could not load conformance package %s#%s: %v", cp.Name, cp.Version, err)
			continue
		}
		logger.Info("  Loaded %s#%s (%d conformance resources from memory)", pkg.Name, pkg.Version, len(pkg.Resources))
		packages = append(packages, pkg)
	}
	return packages
}

// loadBase loads the base packages: those config gives, from the package cache; else the set
// embedded for its FHIR version, or that set from the cache when none is embedded.
func loadBase(l *loader.Loader, config *Config) ([]*loader.Package, error) {
	if len(config.BasePackages) > 0 {
		logger.Info("  Using the base packages given for %s", config.FHIRVersion)
		return loadBasePackages(l, config.BasePackages)
	}
	if embeddedData := specs.GetPackages(config.FHIRVersion); len(embeddedData) > 0 {
		logger.Info("  Using embedded specs for %s", config.FHIRVersion)
		return l.LoadFromEmbeddedData(embeddedData)
	}
	logger.Info("  Loading specs from disk for %s", config.FHIRVersion)
	return l.LoadVersion(config.FHIRVersion)
}

// loadBasePackages loads the base packages given, from the package cache, in their order.
func loadBasePackages(l *loader.Loader, base []PackageSpec) ([]*loader.Package, error) {
	packages := make([]*loader.Package, 0, len(base))
	for _, spec := range base {
		pkg, err := l.OpenPackage(spec.Name, spec.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to load base package %s#%s: %w", spec.Name, spec.Version, err)
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}
