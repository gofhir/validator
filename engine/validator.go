// Package engine provides the main FHIR validation engine.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/phase"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/stream"
)

// Validator is the main FHIR resource validator.
// It coordinates validation phases and manages services.
type Validator struct {
	// Configuration
	version fv.FHIRVersion
	options *fv.Options

	// Services
	profileService     service.ProfileResolver
	terminologyService service.TerminologyService
	referenceResolver  service.ReferenceResolver
	fhirPathEvaluator  service.FHIRPathEvaluator

	// Pipeline
	pipe *pipeline.Pipeline

	// Metrics
	metrics *fv.Metrics

	// Worker pool for batch validation
	workerPool     chan struct{}
	workerPoolOnce sync.Once
}

// New creates a new Validator with the specified FHIR version and options.
func New(ctx context.Context, version fv.FHIRVersion, opts ...fv.Option) (*Validator, error) {
	// Apply options
	options := fv.DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	v := &Validator{
		version: version,
		options: options,
		metrics: fv.NewMetrics(),
	}

	// Build the validation pipeline
	v.buildPipeline()

	return v, nil
}

// buildPipeline constructs the validation pipeline based on options.
func (v *Validator) buildPipeline() {
	pipelineOpts := &pipeline.Options{
		ParallelExecution: v.options.ParallelPhases,
		MaxErrors:         v.options.MaxErrors,
		FailFast:          v.options.MaxErrors == 1,
		PhaseTimeout:      v.options.PhaseTimeout,
		CollectMetrics:    true,
	}

	v.pipe = pipeline.NewPipeline(pipelineOpts)

	// Add phases based on options
	v.addPhases()
}

// addPhases adds validation phases to the pipeline based on configuration.
func (v *Validator) addPhases() {
	// Structure validation (always enabled)
	v.pipe.RegisterConfig(pipeline.PhaseIDStructure, phase.StructurePhaseConfig(v.profileService))

	// Primitives validation (always enabled)
	v.pipe.RegisterConfig(pipeline.PhaseIDPrimitives, phase.PrimitivesPhaseConfig(v.profileService))

	// Cardinality validation (always enabled)
	v.pipe.RegisterConfig(pipeline.PhaseIDCardinality, phase.CardinalityPhaseConfig(v.profileService))

	// Unknown elements validation
	if v.options.ValidateUnknownElements {
		v.pipe.RegisterConfig(pipeline.PhaseIDUnknownElems, phase.UnknownElementsPhaseConfig(v.profileService))
	}

	// Fixed/Pattern validation
	v.pipe.RegisterConfig(pipeline.PhaseIDFixedPattern, phase.FixedPatternPhaseConfig(v.profileService))

	// Terminology validation
	if v.options.ValidateTerminology && v.terminologyService != nil {
		v.pipe.RegisterConfig(pipeline.PhaseIDTerminology, phase.TerminologyPhaseConfig(v.profileService, v.terminologyService))
	}

	// Reference validation
	if v.options.ValidateReferences {
		mode := phase.ReferenceValidationTypeOnly
		if v.referenceResolver != nil {
			mode = phase.ReferenceValidationResolve
		}
		v.pipe.RegisterConfig(pipeline.PhaseIDReferences, phase.ReferencesPhaseConfig(v.profileService, v.referenceResolver, mode))
	}

	// Extension validation
	if v.options.ValidateExtensions {
		v.pipe.RegisterConfig(pipeline.PhaseIDExtensions, phase.ExtensionsPhaseConfig(v.profileService, v.terminologyService))
	}

	// FHIRPath constraints validation
	if v.options.ValidateConstraints && v.fhirPathEvaluator != nil {
		v.pipe.RegisterConfig(pipeline.PhaseIDConstraints, phase.ConstraintsPhaseConfig(v.profileService, v.fhirPathEvaluator))
	}

	// Slicing validation
	v.pipe.RegisterConfig(pipeline.PhaseIDSlicing, phase.SlicingPhaseConfig(v.profileService))

	// Bundle validation (only for Bundle resources, handled internally by phase)
	v.pipe.RegisterConfig(pipeline.PhaseIDBundle, phase.BundlePhaseConfig())
}

// SetProfileService sets the profile resolution service.
func (v *Validator) SetProfileService(svc service.ProfileResolver) {
	v.profileService = svc
	// Rebuild pipeline with new service
	v.buildPipeline()
}

// SetTerminologyService sets the terminology service.
func (v *Validator) SetTerminologyService(svc service.TerminologyService) {
	v.terminologyService = svc
	// Rebuild pipeline with new service
	v.buildPipeline()
}

// SetReferenceResolver sets the reference resolver service.
func (v *Validator) SetReferenceResolver(svc service.ReferenceResolver) {
	v.referenceResolver = svc
	v.buildPipeline()
}

// SetFHIRPathEvaluator sets the FHIRPath evaluator for constraint validation.
func (v *Validator) SetFHIRPathEvaluator(eval service.FHIRPathEvaluator) {
	v.fhirPathEvaluator = eval
	v.buildPipeline()
}

// Validate validates a FHIR resource.
func (v *Validator) Validate(ctx context.Context, resource []byte) (*fv.Result, error) {
	start := time.Now()

	// Parse the resource
	var resourceMap map[string]any
	if err := json.Unmarshal(resource, &resourceMap); err != nil {
		result := fv.AcquireResult()
		result.AddError(fv.IssueTypeStructure, fmt.Sprintf("Invalid JSON: %v", err), "")
		v.metrics.RecordValidation(time.Since(start), false)
		return result, nil
	}

	return v.ValidateMap(ctx, resourceMap)
}

// ValidateMap validates a FHIR resource that's already been parsed to a map.
func (v *Validator) ValidateMap(ctx context.Context, resourceMap map[string]any) (*fv.Result, error) {
	start := time.Now()

	// Get resource type
	resourceType, ok := resourceMap["resourceType"].(string)
	if !ok || resourceType == "" {
		result := fv.AcquireResult()
		result.AddError(fv.IssueTypeStructure, "Resource must have a 'resourceType' element", "")
		v.metrics.RecordValidation(time.Since(start), false)
		return result, nil
	}

	// Create pipeline context
	pctx := pipeline.AcquireContext()
	pctx.ResourceType = resourceType
	pctx.ResourceMap = resourceMap
	pctx.Result = fv.AcquireResult()

	// Extract profiles from meta.profile if present
	profiles := v.extractMetaProfiles(resourceMap)
	pctx.Profiles = profiles

	// Resolve the root profile - prioritize meta.profile over base type
	if len(profiles) > 0 && v.profileService != nil {
		// Use the first declared profile as the root profile
		rootProfile, err := v.profileService.FetchStructureDefinition(ctx, profiles[0])
		if err == nil && rootProfile != nil {
			pctx.RootProfile = rootProfile
		}
	}

	// Fall back to base type if no profile resolved
	if pctx.RootProfile == nil && v.profileService != nil {
		baseProfile, err := v.profileService.FetchStructureDefinitionByType(ctx, resourceType)
		if err == nil {
			pctx.RootProfile = baseProfile
		}
	}

	// Run the pipeline
	v.pipe.Execute(ctx, pctx)

	result := pctx.Result
	pctx.Result = nil // Don't release the result with the context
	pipeline.ReleaseContext(pctx)

	v.metrics.RecordValidation(time.Since(start), result.Valid)
	return result, nil
}

// extractMetaProfiles extracts profile URLs from resource.meta.profile.
func (v *Validator) extractMetaProfiles(resourceMap map[string]any) []string {
	meta, ok := resourceMap["meta"].(map[string]any)
	if !ok {
		return nil
	}

	profileArray, ok := meta["profile"].([]any)
	if !ok {
		return nil
	}

	profiles := make([]string, 0, len(profileArray))
	for _, p := range profileArray {
		if profileURL, ok := p.(string); ok && profileURL != "" {
			profiles = append(profiles, profileURL)
		}
	}

	return profiles
}

// ValidateBatch validates multiple resources in parallel.
func (v *Validator) ValidateBatch(ctx context.Context, resources [][]byte) []*fv.Result {
	results := make([]*fv.Result, len(resources))

	// Initialize worker pool if needed
	v.workerPoolOnce.Do(func() {
		workers := v.options.WorkerCount
		if workers <= 0 {
			workers = 4
		}
		v.workerPool = make(chan struct{}, workers)
	})

	var wg sync.WaitGroup
	for i, resource := range resources {
		wg.Add(1)
		go func(idx int, res []byte) {
			defer wg.Done()

			// Acquire worker slot
			v.workerPool <- struct{}{}
			defer func() { <-v.workerPool }()

			result, err := v.Validate(ctx, res)
			if err != nil {
				result = fv.AcquireResult()
				result.AddError(fv.IssueTypeProcessing, err.Error(), "")
			}
			results[idx] = result
		}(i, resource)
	}

	wg.Wait()
	return results
}

// Metrics returns the validator's metrics.
func (v *Validator) Metrics() *fv.Metrics {
	return v.metrics
}

// Version returns the FHIR version this validator is configured for.
func (v *Validator) Version() fv.FHIRVersion {
	return v.version
}

// Options returns the validator's options.
func (v *Validator) Options() *fv.Options {
	return v.options
}

// Close releases resources held by the validator.
func (v *Validator) Close() error {
	// Nothing to clean up currently
	return nil
}

// ValidateWithProfiles validates a resource against specific profiles.
func (v *Validator) ValidateWithProfiles(ctx context.Context, resource []byte, profiles ...string) (*fv.Result, error) {
	start := time.Now()

	// Parse the resource
	var resourceMap map[string]any
	if err := json.Unmarshal(resource, &resourceMap); err != nil {
		result := fv.AcquireResult()
		result.AddError(fv.IssueTypeStructure, fmt.Sprintf("Invalid JSON: %v", err), "")
		v.metrics.RecordValidation(time.Since(start), false)
		return result, nil
	}

	// Get resource type
	resourceType, ok := resourceMap["resourceType"].(string)
	if !ok || resourceType == "" {
		result := fv.AcquireResult()
		result.AddError(fv.IssueTypeStructure, "Resource must have a 'resourceType' element", "")
		v.metrics.RecordValidation(time.Since(start), false)
		return result, nil
	}

	// Create pipeline context with profiles
	pctx := pipeline.AcquireContext()
	pctx.ResourceType = resourceType
	pctx.ResourceMap = resourceMap
	pctx.Result = fv.AcquireResult()
	pctx.Profiles = profiles

	// Run the pipeline
	v.pipe.Execute(ctx, pctx)

	result := pctx.Result
	pctx.Result = nil
	pipeline.ReleaseContext(pctx)

	v.metrics.RecordValidation(time.Since(start), result.Valid)
	return result, nil
}

// QuickValidate performs fast validation with minimal checks.
// This is useful for initial screening of resources.
func (v *Validator) QuickValidate(ctx context.Context, resource []byte) (*fv.Result, error) {
	// Parse the resource
	var resourceMap map[string]any
	if err := json.Unmarshal(resource, &resourceMap); err != nil {
		result := fv.AcquireResult()
		result.AddError(fv.IssueTypeStructure, fmt.Sprintf("Invalid JSON: %v", err), "")
		return result, nil
	}

	result := fv.AcquireResult()

	// Check resourceType
	resourceType, ok := resourceMap["resourceType"].(string)
	if !ok || resourceType == "" {
		result.AddError(fv.IssueTypeStructure, "Resource must have a 'resourceType' element", "")
		return result, nil
	}

	// Check id format if present
	if id, ok := resourceMap["id"].(string); ok {
		if !phase.ValidateID(id) {
			result.AddError(fv.IssueTypeValue, fmt.Sprintf("Invalid id format: '%s'", id), "id")
		}
	}

	return result, nil
}

// ValidateBundleStream validates a bundle from an io.Reader in a streaming fashion.
// This is useful for large bundles that shouldn't be loaded entirely into memory.
// Results are emitted as entries are processed, in order.
func (v *Validator) ValidateBundleStream(ctx context.Context, r io.Reader) <-chan *stream.EntryResult {
	sv := stream.NewBundleValidator(v.Validate).
		WithWorkerCount(v.options.WorkerCount).
		WithBufferSize(100)

	return sv.ValidateStream(ctx, r)
}

// ValidateBundleStreamParallel validates bundle entries in parallel while preserving order.
// This provides better performance for large bundles with many entries.
func (v *Validator) ValidateBundleStreamParallel(ctx context.Context, r io.Reader) <-chan *stream.EntryResult {
	sv := stream.NewBundleValidator(v.Validate).
		WithWorkerCount(v.options.WorkerCount).
		WithBufferSize(100)

	return sv.ValidateStreamParallel(ctx, r)
}

// AggregateBundleResults collects all results from a streaming bundle validation.
func AggregateBundleResults(results <-chan *stream.EntryResult) *stream.BundleStreamResult {
	return stream.Aggregate(results)
}
