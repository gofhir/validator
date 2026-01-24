// Package pipeline provides the validation pipeline infrastructure.
package pipeline

import (
	"sync"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pool"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/walker"
)

// Context holds all state needed during validation of a single resource.
// It is passed through all validation phases and provides shared access to
// the resource data, profiles, and accumulated results.
//
// Context instances are pooled for efficiency. Use AcquireContext() and
// Release() to manage them properly.
type Context struct {
	// Resource is the raw JSON resource being validated
	Resource []byte

	// ResourceMap is the parsed resource as a map
	ResourceMap map[string]any

	// ResourceType is the FHIR resource type (e.g., "Patient", "Observation")
	ResourceType string

	// ResourceID is the resource ID if present
	ResourceID string

	// FHIRVersion is the FHIR version being validated against
	FHIRVersion fv.FHIRVersion

	// Profiles contains URLs of profiles to validate against
	Profiles []string

	// Result accumulates validation issues
	Result *fv.Result

	// ElementIndex provides fast lookup of elements by path
	ElementIndex map[string]any

	// ResolvedProfiles caches resolved StructureDefinitions
	ResolvedProfiles map[string]any

	// BundleEntries holds parsed bundle entries for bundle validation
	BundleEntries []BundleEntry

	// CurrentPath tracks the current FHIRPath location during tree walking
	CurrentPath *pool.PathBuilder

	// TypeResolver resolves FHIR types to StructureDefinitions
	TypeResolver walker.TypeResolver

	// RootProfile is the StructureDefinition for the root resource type
	RootProfile *service.StructureDefinition

	// RootIndex is the element index for the root profile
	RootIndex *walker.ElementIndex

	// Options holds validation options
	Options *ContextOptions

	// mu protects concurrent access during parallel phase execution
	mu sync.RWMutex

	// Metadata for tracking
	metadata map[string]any
}

// BundleEntry represents a single entry in a FHIR Bundle.
type BundleEntry struct {
	FullURL  string
	Resource map[string]any
	Request  *BundleRequest
	Response *BundleResponse
	Index    int
}

// BundleRequest represents a Bundle.entry.request.
type BundleRequest struct {
	Method string
	URL    string
}

// BundleResponse represents a Bundle.entry.response.
type BundleResponse struct {
	Status   string
	Location string
}

// ContextOptions holds validation options accessible during validation.
type ContextOptions struct {
	ValidateTerminology   bool
	ValidateConstraints   bool
	ValidateReferences    bool
	ValidateExtensions    bool
	ValidateUnknownElems  bool
	ValidateMetaProfiles  bool
	RequireProfile        bool
	StrictMode            bool
	ValidateQuestionnaire bool
	MaxErrors             int
	TrackPositions        bool
	UseEle1FHIRPath       bool
}

// contextPool holds reusable Context instances.
var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			Profiles:         make([]string, 0, 4),
			ElementIndex:     make(map[string]any, 128),
			ResolvedProfiles: make(map[string]any, 8),
			BundleEntries:    make([]BundleEntry, 0, 16),
			metadata:         make(map[string]any, 8),
		}
	},
}

// AcquireContext gets a Context from the pool.
// Call Release() when done to return it to the pool.
func AcquireContext() *Context {
	ctx := contextPool.Get().(*Context)
	ctx.Reset()
	return ctx
}

// Release returns the Context to the pool.
// After calling Release, the Context should not be used.
func (c *Context) Release() {
	if c == nil {
		return
	}

	// Release the path builder if present
	if c.CurrentPath != nil {
		c.CurrentPath.Release()
		c.CurrentPath = nil
	}

	// Don't return contexts with oversized maps
	if len(c.ElementIndex) <= 1024 && len(c.ResolvedProfiles) <= 64 {
		contextPool.Put(c)
	}
}

// Reset clears the context for reuse.
func (c *Context) Reset() {
	c.Resource = nil
	c.ResourceMap = nil
	c.ResourceType = ""
	c.ResourceID = ""
	c.FHIRVersion = ""
	c.Profiles = c.Profiles[:0]
	c.Result = nil
	c.Options = nil

	// Clear maps without reallocating
	for k := range c.ElementIndex {
		delete(c.ElementIndex, k)
	}
	for k := range c.ResolvedProfiles {
		delete(c.ResolvedProfiles, k)
	}
	for k := range c.metadata {
		delete(c.metadata, k)
	}

	c.BundleEntries = c.BundleEntries[:0]

	if c.CurrentPath != nil {
		c.CurrentPath.Release()
		c.CurrentPath = nil
	}

	c.TypeResolver = nil
	c.RootProfile = nil
	c.RootIndex = nil
}

// SetMetadata stores a value in the context metadata.
// Thread-safe for use during parallel phase execution.
func (c *Context) SetMetadata(key string, value any) {
	c.mu.Lock()
	c.metadata[key] = value
	c.mu.Unlock()
}

// GetMetadata retrieves a value from the context metadata.
// Thread-safe for use during parallel phase execution.
func (c *Context) GetMetadata(key string) (any, bool) {
	c.mu.RLock()
	v, ok := c.metadata[key]
	c.mu.RUnlock()
	return v, ok
}

// AddIssue adds a validation issue to the result.
// Thread-safe for use during parallel phase execution.
func (c *Context) AddIssue(issue fv.Issue) {
	if c.Result != nil {
		c.Result.AddIssue(issue)
	}
}

// AddError is a convenience method to add an error issue.
// Thread-safe for use during parallel phase execution.
func (c *Context) AddError(code fv.IssueType, diagnostics, path string) {
	if c.Result != nil {
		c.Result.AddError(code, diagnostics, path)
	}
}

// AddWarning is a convenience method to add a warning issue.
// Thread-safe for use during parallel phase execution.
func (c *Context) AddWarning(code fv.IssueType, diagnostics, path string) {
	if c.Result != nil {
		c.Result.AddWarning(code, diagnostics, path)
	}
}

// ShouldStop returns true if validation should stop (max errors reached).
func (c *Context) ShouldStop() bool {
	if c.Options == nil || c.Options.MaxErrors <= 0 {
		return false
	}
	if c.Result == nil {
		return false
	}
	return c.Result.ErrorCount() >= c.Options.MaxErrors
}

// IsBundle returns true if the resource is a Bundle.
func (c *Context) IsBundle() bool {
	return c.ResourceType == "Bundle"
}

// GetResourceField returns a field value from the resource map.
func (c *Context) GetResourceField(field string) (any, bool) {
	if c.ResourceMap == nil {
		return nil, false
	}
	v, ok := c.ResourceMap[field]
	return v, ok
}

// GetNestedField returns a nested field value using dot notation.
// Example: GetNestedField("meta.profile") returns the profile array.
func (c *Context) GetNestedField(path string) (any, bool) {
	if c.ResourceMap == nil {
		return nil, false
	}

	current := any(c.ResourceMap)
	start := 0

	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i > start {
				key := path[start:i]
				switch m := current.(type) {
				case map[string]any:
					var ok bool
					current, ok = m[key]
					if !ok {
						return nil, false
					}
				default:
					return nil, false
				}
			}
			start = i + 1
		}
	}

	return current, true
}

// Clone creates a shallow copy of the context.
// The new context shares the same resource data but has independent result.
func (c *Context) Clone() *Context {
	clone := AcquireContext()
	clone.Resource = c.Resource
	clone.ResourceMap = c.ResourceMap
	clone.ResourceType = c.ResourceType
	clone.ResourceID = c.ResourceID
	clone.FHIRVersion = c.FHIRVersion
	clone.Profiles = append(clone.Profiles, c.Profiles...)
	clone.Options = c.Options

	// Note: Result is NOT copied - caller should set a new result if needed
	return clone
}

// NewContext creates a new Context (non-pooled).
// Prefer AcquireContext() for better performance.
func NewContext() *Context {
	return &Context{
		Profiles:         make([]string, 0, 4),
		ElementIndex:     make(map[string]any, 128),
		ResolvedProfiles: make(map[string]any, 8),
		BundleEntries:    make([]BundleEntry, 0, 16),
		metadata:         make(map[string]any, 8),
	}
}

// ReleaseContext returns a Context to the pool.
// This is a convenience function equivalent to ctx.Release().
func ReleaseContext(ctx *Context) {
	if ctx != nil {
		ctx.Release()
	}
}
