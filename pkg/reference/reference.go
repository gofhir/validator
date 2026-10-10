// Package reference validates FHIR Reference elements.
package reference

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
	"github.com/gofhir/validator/v2/pkg/walker"
)

const typeCodeReference = "Reference"

// BundleContext holds information about a Bundle for reference validation.
type BundleContext struct {
	// FullURLIndex maps fullUrl values to their resource types.
	// e.g., "urn:uuid:abc-123" -> "Patient"
	FullURLIndex map[string]string
}

// NewBundleContext creates a BundleContext from a Bundle resource.
// It indexes all entry.fullUrl values for reference resolution.
func NewBundleContext(bundle map[string]any) *BundleContext {
	ctx := &BundleContext{
		FullURLIndex: make(map[string]string),
	}

	entries, ok := bundle["entry"].([]any)
	if !ok {
		return ctx
	}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		fullURL, _ := entryMap["fullUrl"].(string)
		if fullURL == "" {
			continue
		}

		// Get the resource type from the entry's resource
		resourceMap, ok := entryMap["resource"].(map[string]any)
		if !ok {
			continue
		}

		resourceType, _ := resourceMap["resourceType"].(string)
		ctx.FullURLIndex[fullURL] = resourceType
	}

	return ctx
}

// ContainedContext holds the index of contained resource IDs for the current
// resource scope. This enables validation that fragment references (#id)
// resolve to actual contained resources.
type ContainedContext struct {
	// IDIndex maps contained resource IDs to their resource types.
	IDIndex map[string]string
}

// NewContainedContext creates a ContainedContext by indexing all contained
// resources within the given resource data.
func NewContainedContext(resource map[string]any) *ContainedContext {
	ctx := &ContainedContext{IDIndex: make(map[string]string)}

	contained, ok := resource["contained"].([]any)
	if !ok {
		return ctx
	}

	for _, item := range contained {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id, _ := m["id"].(string)
		if id == "" {
			continue
		}

		// First occurrence wins. When two contained resources share an id the fragment is
		// ambiguous, and overwriting made the *last* one resolve — so a reference could be
		// reported as pointing at the wrong type while the real defect, the repeated id,
		// went unmentioned. Document order is the one deterministic choice available, and
		// it is what the reference resolves to.
		if _, seen := ctx.IDIndex[id]; seen {
			continue
		}

		rt, _ := m["resourceType"].(string)
		ctx.IDIndex[id] = rt
	}

	return ctx
}

// ValidateBundleFullUrls validates that fullUrl is consistent with resource.id for all entries.
// Per FHIR spec: "fullUrl SHALL NOT disagree with the id in the resource"
// This applies when fullUrl is a URL (not urn:uuid or urn:oid).
func ValidateBundleFullUrls(bundle map[string]any, result *issue.Result) {
	entries, ok := bundle["entry"].([]any)
	if !ok {
		return
	}

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		fullURL, _ := entryMap["fullUrl"].(string)
		if fullURL == "" {
			continue
		}

		// Skip URN references - they don't need to match resource.id
		if strings.HasPrefix(fullURL, "urn:uuid:") || strings.HasPrefix(fullURL, "urn:oid:") {
			continue
		}

		resourceMap, ok := entryMap["resource"].(map[string]any)
		if !ok {
			continue
		}

		resourceID, _ := resourceMap["id"].(string)
		if resourceID == "" {
			// No id to validate against
			continue
		}

		// Extract expected id from fullUrl
		expectedID := extractIDFromFullURL(fullURL)
		if expectedID == "" {
			continue
		}

		if resourceID != expectedID {
			result.AddErrorWithID(
				issue.DiagBundleFullURLMismatch,
				map[string]any{
					"fullUrl": fullURL,
					"id":      resourceID,
				},
				fmt.Sprintf("Bundle.entry[%d]", i),
			)
		}
	}
}

// extractIDFromFullURL extracts the resource id from a fullUrl.
// Examples: "http://example.org/fhir/Patient/123" -> "123",
// "http://example.org/fhir/Patient/123/_history/1" -> "123".
func extractIDFromFullURL(fullURL string) string {
	// Remove _history suffix if present
	historyIdx := strings.Index(fullURL, "/_history/")
	if historyIdx != -1 {
		fullURL = fullURL[:historyIdx]
	}

	// Extract last path segment
	lastSlash := strings.LastIndex(fullURL, "/")
	if lastSlash == -1 || lastSlash == len(fullURL)-1 {
		return ""
	}

	return fullURL[lastSlash+1:]
}

// Reference format patterns.
var (
	// Relative reference: ResourceType/id or ResourceType/id/_history/vid.
	relativeRefPattern = regexp.MustCompile(`^[A-Za-z]+/[A-Za-z0-9\-.]+(?:/_history/[A-Za-z0-9\-.]+)?$`)

	// Absolute URL reference (with optional _history/vid).
	absoluteRefPattern = regexp.MustCompile(`^https?://\S+/[A-Za-z]+/[A-Za-z0-9\-.]+(?:/_history/[A-Za-z0-9\-.]+)?$`)

	// Fragment reference (contained resource).
	// "#" alone is a contained resource's reference to the resource that contains it
	// (references.html#contained); ref-1 reports it made from any other resource.
	fragmentRefPattern = regexp.MustCompile(`^#[A-Za-z0-9\-.]*$`)

	// URN reference patterns.
	// Note: urn:uuid accepts any non-empty suffix to match HL7 validator behavior.
	// HL7 validator does NOT validate UUID format (RFC 4122) - it only checks if
	// the reference exists in the Bundle. Invalid UUIDs get "not in bundle" warning.
	urnUUIDPattern = regexp.MustCompile(`^urn:uuid:.+$`)
	urnOIDPattern  = regexp.MustCompile(`^urn:oid:[012](\.[1-9]\d*)+$`)
)

// Validator validates Reference elements.
type Validator struct {
	registry  *registry.Registry
	walker    *walker.Walker
	conformer slicematch.Conformer
}

// Option configures a Validator.
type Option func(*Validator)

// WithTargets checks the target a reference resolves to against the profiles its element allows
// (ElementDefinition.type.targetProfile: "the content must conform to at least one of them"):
// conformer decides whether it conforms. Without it, only the target's type is checked.
func WithTargets(conformer slicematch.Conformer) Option {
	return func(v *Validator) { v.conformer = conformer }
}

// New creates a new reference Validator.
func New(reg *registry.Registry, opts ...Option) *Validator {
	v := &Validator{
		registry: reg,
		walker:   walker.New(reg),
	}
	for _, o := range opts {
		o(v)
	}
	return v
}

// refContext is what a reference is resolved within: the Bundle's index, the contained resources
// of the resource that makes it (root), and the Bundle its targets are entries of (container).
type refContext struct {
	ctx       context.Context
	bundle    *BundleContext
	contained *ContainedContext
	root      map[string]any
	// container is the Bundle whose entry the resource that makes the reference is (or is
	// contained in): its targets are entries of it (bundle.html#references).
	container map[string]any
	// outer are the Bundles that hold container, the innermost first: where its targets' resolve()
	// looks after it.
	outer []map[string]any
	// fullURL is the fullUrl of that entry, the base of a relative reference.
	fullURL string
	// inContained is set when the referring resource is one root contains: "#" names root.
	inContained bool
}

// nested returns the context of a resource the walk visits, from its parent's: an entry has its
// own contained resources, its fullUrl and its Bundle; a contained resource shares its
// container's.
func (rc *refContext) nested(w *walker.ResourceContext) *refContext {
	if !w.IsBundleEntry {
		in := *rc
		in.inContained = true
		return &in
	}
	outer := rc.outer
	if !sameMap(w.Container, rc.container) {
		// An entry of a Bundle that is an entry of rc's.
		outer = append([]map[string]any{rc.container}, rc.outer...)
	}
	return &refContext{ctx: rc.ctx, bundle: rc.bundle, contained: NewContainedContext(w.Data), root: w.Data,
		container: w.Container, outer: outer, fullURL: w.FullURL}
}

type containerKey struct{}

type containingKey struct{}

type holdingKey struct{}

// WithHoldingResource returns a context in which the value validated, a datatype, is one resource
// holds: its references resolve as resource's do ("#id" among its contained resources, a relative
// reference from its entry).
func WithHoldingResource(ctx context.Context, resource map[string]any) context.Context {
	return context.WithValue(ctx, holdingKey{}, resource)
}

// WithContainingResource returns a context in which the resource validated is one contained in
// container: its fragment references ("#id") resolve among container's contained resources
// (references.html#contained).
func WithContainingResource(ctx context.Context, container map[string]any) context.Context {
	return context.WithValue(ctx, containingKey{}, container)
}

// WithContainer returns a context in which references resolve among the entries of container, a
// Bundle: the one the resource validated is an entry of. Outer are the Bundles that hold it, the
// innermost first, where the targets' resolve() looks after it.
func WithContainer(ctx context.Context, container map[string]any, outer ...map[string]any) context.Context {
	return context.WithValue(ctx, containerKey{}, containers{container: container, outer: outer})
}

// containers are what WithContainer puts in a context.
type containers struct {
	container map[string]any
	outer     []map[string]any
}

func sameMap(a, b map[string]any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// rootContext is the context references in resource, the resource validated, resolve in: the
// Bundle it is, or else the Bundle the context names, with the fullUrl of resource's entry in it.
func rootContext(ctx context.Context, resource map[string]any, bundleCtx *BundleContext) *refContext {
	rc := &refContext{ctx: ctx, bundle: bundleCtx, contained: NewContainedContext(resource), root: resource}
	// A resource checked as one its container contains resolves as the container does: "#id"
	// among its contained resources, "#" the container, a relative reference from the container's
	// entry.
	holder := resource
	if c, ok := ctx.Value(containingKey{}).(map[string]any); ok && c != nil {
		rc.contained, rc.root, rc.inContained, holder = NewContainedContext(c), c, true, c
	} else if h, ok := ctx.Value(holdingKey{}).(map[string]any); ok && h != nil {
		rc.contained, rc.root, holder = NewContainedContext(h), h, h
	}
	in, _ := ctx.Value(containerKey{}).(containers)
	if rt, _ := resource["resourceType"].(string); rt == "Bundle" {
		rc.container, rc.outer = resource, in.outer
		if in.container != nil && !sameMap(in.container, resource) {
			if t, _ := in.container["resourceType"].(string); t == "Bundle" {
				rc.outer = append([]map[string]any{in.container}, in.outer...)
			}
		}
		return rc
	}
	if in.container != nil {
		rc.container, rc.outer = in.container, in.outer
		rc.fullURL = indexOf(ctx, in.container).fullURLOf[reflect.ValueOf(holder).Pointer()]
	}
	return rc
}

// urnType is the type of the entry a URN reference names, in the Bundle whose entry the referring
// resource is, and whether one does; or, without that Bundle, in the BundleContext the caller gave.
func (rc *refContext) urnType(ref string) (string, bool) {
	if rc.container == nil {
		if rc.bundle == nil {
			return "", false
		}
		t, ok := rc.bundle.FullURLIndex[ref]
		return t, ok
	}
	matches := entryMatches(ref, rc)
	if len(matches) != 1 {
		return "", len(matches) > 0 // several: no type, the reference is ambiguous
	}
	t, _ := matches[0]["resourceType"].(string)
	return t, true
}

// entryIndex indexes a Bundle's entries: the resources by their entry's fullUrl, and the fullUrl
// of each resource's entry.
type entryIndex struct {
	byFullURL map[string][]map[string]any
	fullURLOf map[uintptr]string
}

// indexes holds the entry indexes of the Bundles one validation resolves references in, by
// Bundle; each is built once (withIndexes).
type indexes struct {
	mu       sync.Mutex
	byBundle map[uintptr]*entryIndex
}

type indexesKey struct{}

// WithIndexes returns ctx with a new store of the entry indexes references are resolved with,
// for one validation: its conformance checks share it. A store keys Bundles by address, so it
// must not outlive the validation.
func WithIndexes(ctx context.Context) context.Context {
	return context.WithValue(ctx, indexesKey{}, &indexes{byBundle: map[uintptr]*entryIndex{}})
}

// withIndexes returns ctx with a store of entry indexes, unless it has one.
func withIndexes(ctx context.Context) context.Context {
	if _, ok := ctx.Value(indexesKey{}).(*indexes); ok {
		return ctx
	}
	return WithIndexes(ctx)
}

// indexOf returns the index of bundle's entries, from ctx's store when it has one.
func indexOf(ctx context.Context, bundle map[string]any) *entryIndex {
	store, _ := ctx.Value(indexesKey{}).(*indexes)
	key := reflect.ValueOf(bundle).Pointer()
	if store != nil {
		store.mu.Lock()
		defer store.mu.Unlock()
		if idx, ok := store.byBundle[key]; ok {
			return idx
		}
	}
	list, _ := bundle["entry"].([]any)
	idx := &entryIndex{byFullURL: make(map[string][]map[string]any, len(list)), fullURLOf: make(map[uintptr]string, len(list))}
	for _, e := range list {
		m, _ := e.(map[string]any)
		r, ok := m["resource"].(map[string]any)
		if !ok {
			continue
		}
		u, _ := m["fullUrl"].(string)
		idx.byFullURL[u] = append(idx.byFullURL[u], r)
		idx.fullURLOf[reflect.ValueOf(r).Pointer()] = u
	}
	if store != nil {
		store.byBundle[key] = idx
	}
	return idx
}

// restfulURL matches a RESTful fullUrl: a base, then the resource's type and id
// (references.html#literal), the type and id being the last two segments.
var restfulURL = regexp.MustCompile(`^(.+/)[A-Z][A-Za-z]+/[A-Za-z0-9\-.]{1,64}$`)

// resolveTarget finds the resource ref names, made from within rc (bundle.html#references): "#id",
// a resource the referring resource contains; an absolute reference, the entry whose fullUrl it
// is; a relative one ([type]/[id]), the entry whose fullUrl is the referring entry's base followed
// by it, which needs that fullUrl to be RESTful. A version (/_history/x) is removed before
// matching the fullUrl, then matched against the resource's meta.versionId. Only the Bundle whose
// entry the referring resource is is searched: the specification gives a reference no meaning
// in a Bundle that holds it. Several matches are ambiguous ("it is ambiguous which is correct"):
// the reference does not resolve.
func resolveTarget(ref string, rc *refContext) (map[string]any, bool) {
	if id, ok := strings.CutPrefix(ref, "#"); ok {
		if id == "" {
			return rc.root, rc.inContained
		}
		contained, _ := rc.root["contained"].([]any)
		for _, c := range contained {
			if m, ok := c.(map[string]any); ok && m["id"] == id {
				return m, true
			}
		}
		return nil, false
	}
	if matches := entryMatches(ref, rc); len(matches) == 1 {
		return matches[0], true
	}
	return nil, false
}

// entryMatches are the entries of rc's Bundle a literal reference other than "#id" matches, as
// resolveTarget matches them.
func entryMatches(ref string, rc *refContext) []map[string]any {
	if rc.container == nil {
		return nil
	}
	version := ""
	if i := strings.Index(ref, "/_history/"); i >= 0 {
		ref, version = ref[:i], ref[i+len("/_history/"):]
	}
	target := ref
	if !strings.Contains(ref, ":") {
		m := restfulURL.FindStringSubmatch(rc.fullURL)
		if m == nil {
			return nil
		}
		target = m[1] + ref
	}
	var out []map[string]any
	for _, r := range indexOf(rc.ctx, rc.container).byFullURL[target] {
		if version != "" {
			if meta, _ := r["meta"].(map[string]any); meta == nil || meta["versionId"] != version {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// Validate validates all Reference elements in a resource.
//
// Deprecated: Use ValidateData for better performance when JSON is already parsed.
func (v *Validator) Validate(resourceData json.RawMessage, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	var resource map[string]any
	if err := json.Unmarshal(resourceData, &resource); err != nil {
		return
	}

	v.ValidateData(resource, sd, result)
}

// ValidateData validates all Reference elements in a pre-parsed FHIR resource.
// This is the preferred method when JSON has already been parsed to avoid redundant parsing.
func (v *Validator) ValidateData(resource map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	v.ValidateDataWithBundle(resource, sd, nil, result)
}

// ValidateDataWithBundle validates all Reference elements in a pre-parsed FHIR resource. A Bundle's
// references resolve among its entries (bundle.html#references); bundleCtx gives the types of the
// URN references of a resource that is not a Bundle and has no Bundle in its context
// (WithContainer), and may be nil.
func (v *Validator) ValidateDataWithBundle(resource map[string]any, sd *registry.StructureDefinition, bundleCtx *BundleContext, result *issue.Result) {
	v.ValidateDataWithBundleContext(context.Background(), resource, sd, bundleCtx, result)
}

// ValidateDataWithBundleContext is ValidateDataWithBundle, resolving the profiles nested resources
// declare with ctx.
func (v *Validator) ValidateDataWithBundleContext(ctx context.Context, resource map[string]any, sd *registry.StructureDefinition, bundleCtx *BundleContext, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType := sd.RootName(resource)
	if resourceType == "" {
		return
	}

	// Build contained resource index for the root resource
	ctx = withIndexes(ctx)
	rc := rootContext(ctx, resource, bundleCtx)

	// Validate references in root resource
	v.validateElementWithPaths(resource, sd, resourceType, resourceType, rc, result)

	// Walk all nested resources (contained + Bundle entries) using the generic walker, each against
	// the profiles its meta.profile declares and its type's definition. A reference that several
	// of them find wrong is reported once.
	nested := issue.GetPooledResult()
	defer issue.ReleaseResult(nested)
	// Each resource's context, by its path: an entry has its own contained resources and fullUrl,
	// and its Bundle first; a contained resource shares its container's (references.html#contained).
	// The walk visits a resource before those it holds.
	contexts := map[string]*refContext{resourceType: rc}
	v.walker.WalkWithProfilesContext(ctx, resource, resourceType, resourceType, func(ctx *walker.ResourceContext) bool {
		// Skip root resource (already validated above)
		if ctx.FHIRPath == resourceType {
			return true
		}
		parent := contexts[ctx.ParentPath]
		if parent == nil {
			parent = rc
		}
		here, seen := contexts[ctx.FHIRPath]
		if !seen {
			here = parent.nested(ctx)
			contexts[ctx.FHIRPath] = here
		}

		// Validate references in the nested resource
		// Use ResourceType for SD lookup, FHIRPath for error reporting
		v.validateElementWithPaths(ctx.Data, ctx.SD, ctx.ResourceType, ctx.FHIRPath, here, nested)
		return true
	})
	seen := make(map[string]bool, len(nested.Issues))
	for _, is := range nested.Issues {
		k := string(is.Severity) + "\x00" + is.MessageID + "\x00" + strings.Join(is.Expression, ",") + "\x00" + is.Diagnostics
		if !seen[k] {
			seen[k] = true
			result.AddIssue(is)
		}
	}
}

// ValidateElementWithPaths validates references with separate paths for SD lookup and error reporting.
// SdPath is used to look up ElementDefinitions in the StructureDefinition.
// FhirPath is used for error reporting (e.g., "Bundle.entry[0].resource.subject").
func (v *Validator) validateElementWithPaths(data map[string]any, sd *registry.StructureDefinition, sdPath, fhirPath string, rc *refContext, result *issue.Result) {
	// In the order of the keys, not the map's: a target's check depends on the checks made before.
	for _, key := range slices.Sorted(maps.Keys(data)) {
		value := data[key]
		if key == "resourceType" {
			continue
		}

		elementSDPath := fmt.Sprintf("%s.%s", sdPath, key)
		elementFhirPath := fmt.Sprintf("%s.%s", fhirPath, key)

		// Find the ElementDefinition for this path using SD path
		elemDef := v.findElementDef(sd, elementSDPath)
		if elemDef == nil {
			continue
		}

		// Check if this element is a Reference type
		if v.isReferenceType(elemDef) {
			v.validateReference(value, elemDef, elementFhirPath, rc, result)
		}

		// BackboneElement children are defined in the parent resource SD (e.g., Patient.link.other),
		// not in BackboneElement's own SD. Use validateElementWithPaths to look them up correctly.
		isBackbone := v.isBackboneType(elemDef)

		// Recurse into complex types
		switch val := value.(type) {
		case map[string]any:
			if isBackbone {
				v.validateElementWithPaths(val, sd, elementSDPath, elementFhirPath, rc, result)
			} else {
				v.validateComplexElement(val, elemDef, elementFhirPath, rc, result)
			}
		case []any:
			v.validateArrayElement(val, sd, elemDef, elementSDPath, elementFhirPath, isBackbone, rc, result)
		}
	}
}

// validateArrayElement validates references within array elements.
func (v *Validator) validateArrayElement(items []any, sd *registry.StructureDefinition, elemDef *registry.ElementDefinition, elementSDPath, elementFhirPath string, isBackbone bool, rc *refContext, result *issue.Result) {
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", elementFhirPath, i)
		mapItem, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if v.isReferenceType(elemDef) {
			v.validateReference(mapItem, elemDef, itemPath, rc, result)
		}
		if isBackbone {
			v.validateElementWithPaths(mapItem, sd, elementSDPath, itemPath, rc, result)
		} else {
			v.validateComplexElement(mapItem, elemDef, itemPath, rc, result)
		}
	}
}

// validateComplexElement validates references within a complex element.
func (v *Validator) validateComplexElement(data map[string]any, parentDef *registry.ElementDefinition, basePath string, rc *refContext, result *issue.Result) {
	if len(parentDef.Type) == 0 {
		return
	}

	typeName := parentDef.Type[0].Code
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil || typeSD.Snapshot == nil {
		return
	}

	for _, key := range slices.Sorted(maps.Keys(data)) {
		value := data[key]
		elementPath := fmt.Sprintf("%s.%s", basePath, key)
		typePath := fmt.Sprintf("%s.%s", typeName, key)

		var elemDef *registry.ElementDefinition
		for i := range typeSD.Snapshot.Element {
			if typeSD.Snapshot.Element[i].Path == typePath {
				elemDef = &typeSD.Snapshot.Element[i]
				break
			}
		}

		if elemDef == nil {
			continue
		}

		if v.isReferenceType(elemDef) {
			v.validateReference(value, elemDef, elementPath, rc, result)
		}

		switch val := value.(type) {
		case map[string]any:
			v.validateComplexElement(val, elemDef, elementPath, rc, result)
		case []any:
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", elementPath, i)
				if mapItem, ok := item.(map[string]any); ok {
					if v.isReferenceType(elemDef) {
						v.validateReference(mapItem, elemDef, itemPath, rc, result)
					}
					v.validateComplexElement(mapItem, elemDef, itemPath, rc, result)
				}
			}
		}
	}
}

// isBackboneType checks if an element is of type BackboneElement.
// BackboneElement children are defined in the parent resource's SD, not in BackboneElement's own SD.
func (v *Validator) isBackboneType(elemDef *registry.ElementDefinition) bool {
	for _, t := range elemDef.Type {
		if t.Code == "BackboneElement" {
			return true
		}
	}
	return false
}

// isReferenceType checks if an element is of type Reference.
func (v *Validator) isReferenceType(elemDef *registry.ElementDefinition) bool {
	for _, t := range elemDef.Type {
		if t.Code == typeCodeReference {
			return true
		}
	}
	return false
}

// validateReference validates a single Reference value.
func (v *Validator) validateReference(value any, elemDef *registry.ElementDefinition, fhirPath string, rc *refContext, result *issue.Result) {
	refMap, ok := value.(map[string]any)
	if !ok {
		return
	}

	// Get reference string
	refStr, _ := refMap["reference"].(string)
	refType, _ := refMap["type"].(string)

	// If no reference string, check if it's a logical reference (identifier only)
	if refStr == "" {
		if refMap["identifier"] != nil {
			// Logical reference - valid, no further validation needed
			return
		}
		// No reference and no identifier - might be display only which is allowed
		if refMap["display"] != nil {
			return
		}
		// Empty reference is allowed per FHIR spec
		return
	}

	// Validate reference format
	if !v.isValidReferenceFormat(refStr) {
		result.AddErrorWithID(
			issue.DiagReferenceInvalidFormat,
			map[string]any{
				"reference": refStr,
			},
			fhirPath+".reference",
		)
		return
	}

	target, targetType := v.targetOf(refStr, rc)

	// Reference.type and the target "SHALL be consistent" (Reference.type).
	if refType != "" && targetType != "" && refType != targetType {
		result.AddErrorWithID(
			issue.DiagReferenceTypeMismatch,
			map[string]any{
				"type":      refType,
				"reference": targetType,
			},
			fhirPath,
		)
	}

	// Validate fragment references against contained resources.
	// Per FHIR spec (ref-1): "SHALL have a contained resource if a local reference is provided"
	if strings.HasPrefix(refStr, "#") && refStr != "#" {
		fragmentID := refStr[1:]
		if rc.contained != nil {
			if _, found := rc.contained.IDIndex[fragmentID]; !found {
				result.AddErrorWithID(
					issue.DiagReferenceContainedNotFound,
					map[string]any{
						"id": fragmentID,
					},
					fhirPath+".reference",
				)
			}
		}
	}

	// Validate URN references exist within Bundle context.
	// Per FHIR spec and HL7 validator behavior:
	// - urn:uuid and urn:oid references SHOULD resolve within the Bundle (warning if not found)
	// - Absolute URLs (http/https) are allowed to reference external resources (no warning)
	if rc.container != nil || rc.bundle != nil {
		if strings.HasPrefix(refStr, "urn:uuid:") || strings.HasPrefix(refStr, "urn:oid:") {
			if _, found := rc.urnType(refStr); !found {
				result.AddWarningWithID(
					issue.DiagReferenceNotInBundle,
					map[string]any{
						"reference": refStr,
					},
					fhirPath,
				)
			}
		}
	}

	// Several entries the reference matches: "it is ambiguous which is correct", and an
	// application "MAY return an error" (bundle.html#references), as the HL7 validator does.
	if !strings.HasPrefix(refStr, "#") && len(entryMatches(refStr, rc)) > 1 {
		result.AddErrorWithID(issue.DiagReferenceMultipleMatches, map[string]any{"reference": refStr}, fhirPath)
	}

	// Validate targetProfile - check if reference target type is allowed.
	// This validates structural conformance based on the StructureDefinition.
	v.validateTargetProfile(targetType, target, refStr, elemDef, fhirPath, rc, result)

	// Validate aggregation mode constraints.
	// Per FHIR spec, ElementDefinition.type.aggregation restricts how references should be resolved:
	// - "contained": reference must be a fragment (#id) to a contained resource
	// - "referenced": reference must be a literal URL (relative or absolute)
	// - "bundled": reference must resolve within the Bundle
	v.validateAggregation(refStr, elemDef, fhirPath, result)
}

// validateAggregation validates that the reference format matches the allowed aggregation modes.
// When aggregation is empty, any reference format is allowed (default FHIR behavior).
func (v *Validator) validateAggregation(refStr string, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
	modes := v.getAggregationModes(elemDef)
	if len(modes) == 0 {
		return // No aggregation constraints — any format allowed
	}

	isFragment := strings.HasPrefix(refStr, "#")
	isURN := strings.HasPrefix(refStr, "urn:uuid:") || strings.HasPrefix(refStr, "urn:oid:")

	allowed := false
	for _, mode := range modes {
		switch mode {
		case "contained":
			if isFragment {
				allowed = true
			}
		case "bundled":
			// URN references and fragment references can resolve in a Bundle
			if isURN {
				allowed = true
			}
		case "referenced":
			// Literal URL references (relative or absolute, not fragment, not URN)
			if !isFragment && !isURN {
				allowed = true
			}
		}
	}

	if !allowed {
		result.AddErrorWithID(
			issue.DiagReferenceAggregationMode,
			map[string]any{
				"reference": refStr,
				"allowed":   strings.Join(modes, ", "),
			},
			fhirPath+".reference",
		)
	}
}

// getAggregationModes extracts aggregation modes from all Reference types in an ElementDefinition.
func (v *Validator) getAggregationModes(elemDef *registry.ElementDefinition) []string {
	var modes []string
	seen := make(map[string]bool)
	for _, t := range elemDef.Type {
		if t.Code == typeCodeReference {
			for _, a := range t.Aggregation {
				if !seen[a] {
					seen[a] = true
					modes = append(modes, a)
				}
			}
		}
	}
	return modes
}

// targetOf is the resource refStr resolves to within rc, or nil, and the target's type: the
// resolved resource's, else the one its literal names ([type]/[id]), or for a URN the type the
// caller's BundleContext gives.
func (v *Validator) targetOf(refStr string, rc *refContext) (target map[string]any, targetType string) {
	target, resolved := resolveTarget(refStr, rc)
	if t, _ := target["resourceType"].(string); resolved && t != "" {
		return target, t
	}
	targetType = v.extractResourceType(refStr)
	if targetType == "" && (strings.HasPrefix(refStr, "urn:uuid:") || strings.HasPrefix(refStr, "urn:oid:")) {
		targetType, _ = rc.urnType(refStr)
	}
	return nil, targetType
}

// validateTargetProfile validates that the reference target type matches allowed targetProfiles.
// Per FHIR spec, ElementDefinition.type[].targetProfile restricts which resource types
// can be referenced. If no targetProfile is specified, any resource type is allowed.
func (v *Validator) validateTargetProfile(targetType string, target map[string]any, refStr string, elemDef *registry.ElementDefinition, fhirPath string, rc *refContext, result *issue.Result) {
	if targetType == "" {
		return // the type cannot be determined
	}

	// Get all targetProfiles from all Reference types in the element definition
	allowedProfiles := v.getTargetProfiles(elemDef)

	// If no targetProfiles specified, any type is allowed (Reference(Any))
	if len(allowedProfiles) == 0 {
		return
	}

	// Check if the target's type matches any of the allowed profiles
	if !v.typeMatchesProfiles(targetType, allowedProfiles) {
		// Build list of allowed types for error message
		allowedTypes := v.extractTypesFromProfiles(allowedProfiles)
		result.AddErrorWithID(
			issue.DiagReferenceInvalidTarget,
			map[string]any{
				"type":    targetType,
				"allowed": strings.Join(allowedTypes, ", "),
			},
			fhirPath, // the Reference, as the HL7 validator reports it (Reference_REF_BadTargetType)
		)
		return
	}
	if target != nil {
		v.checkTargetConformance(refStr, target, targetType, allowedProfiles, fhirPath, rc, result)
	}
}

// checkTargetConformance checks the target refStr resolves to, of type targetType, against the
// profiles of its type among allowed: it must conform to at least one of them
// (ElementDefinition.type.targetProfile). A target that does not resolve is not checked; nor one
// whose type's own definition is allowed, which its type and its own validation answer. The
// failure is reported at the Reference, as the HL7 validator reports it
// (Reference_REF_CantMatchChoice).
func (v *Validator) checkTargetConformance(refStr string, target map[string]any, targetType string, allowed []string, fhirPath string, rc *refContext, result *issue.Result) {
	if v.conformer == nil || rc == nil {
		return
	}
	var candidates []*registry.StructureDefinition
	var names []string
	for _, p := range allowed {
		sd, _ := v.registry.ResolveCanonical(p)
		if sd == nil || (sd.Type != targetType && !v.registry.IsSubtype(targetType, sd.Type)) {
			continue
		}
		if sd.Derivation != registry.DerivationConstraint {
			// The definition of its type or of a type it derives from (Resource, DomainResource):
			// any resource of the type conforms to it here.
			return
		}
		candidates = append(candidates, sd)
		names = append(names, p)
	}
	if len(candidates) == 0 {
		return
	}
	// The target is its own %resource; a contained one has the resource that contains it as its
	// %rootResource (fhirpath.html#variables). Its own references resolve in the Bundle it is in.
	scope := slicematch.Scope{Resource: target, RootResource: target, Container: rc.container, Outer: rc.outer}
	if strings.HasPrefix(refStr, "#") {
		scope.RootResource = rc.root
	}
	for _, sd := range candidates {
		if v.conformer.Conforms(rc.ctx, target, sd, scope) {
			return
		}
	}
	result.AddErrorWithID(issue.DiagReferenceTargetProfile,
		map[string]any{"reference": refStr, "profiles": strings.Join(names, ", ")}, fhirPath)
}

// getTargetProfiles extracts all targetProfile URLs from Reference types in an ElementDefinition.
func (v *Validator) getTargetProfiles(elemDef *registry.ElementDefinition) []string {
	var profiles []string
	for _, t := range elemDef.Type {
		if t.Code == typeCodeReference {
			profiles = append(profiles, t.TargetProfile...)
		}
	}
	return profiles
}

// typeMatchesProfiles checks if a resource type matches any of the allowed profile URLs.
// Profile URLs are in the format: http://hl7.org/fhir/StructureDefinition/[ResourceType]
func (v *Validator) typeMatchesProfiles(resourceType string, profiles []string) bool {
	for _, profile := range profiles {
		// Extract resource type from profile URL
		profileType := v.extractTypeFromProfile(profile)
		if profileType == resourceType {
			return true
		}
		// Reference(Resource) should allow any resource type
		if profileType == "Resource" {
			return true
		}
	}
	return false
}

// extractTypeFromProfile returns the resource type a targetProfile constrains: the type of the
// definition the canonical names, the version it pins or the one an unversioned canonical resolves
// to (references.html#canonical).
func (v *Validator) extractTypeFromProfile(profile string) string {
	if sd, _ := v.registry.ResolveCanonical(profile); sd != nil {
		return sd.Type
	}

	// Fallback: extract last path segment
	profileURL, _ := registry.ParseCanonical(profile)
	parts := strings.Split(profileURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// extractTypesFromProfiles extracts resource type names from profile URLs for error messages.
func (v *Validator) extractTypesFromProfiles(profiles []string) []string {
	seen := make(map[string]bool)
	var types []string
	for _, profile := range profiles {
		t := v.extractTypeFromProfile(profile)
		if t != "" && !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	return types
}

// isValidReferenceFormat checks if a reference string has a valid format.
func (v *Validator) isValidReferenceFormat(ref string) bool {
	if ref == "" {
		return true // Empty is allowed
	}

	// Check various valid formats
	if relativeRefPattern.MatchString(ref) {
		return true
	}
	if absoluteRefPattern.MatchString(ref) {
		return true
	}
	if fragmentRefPattern.MatchString(ref) {
		return true
	}
	if urnUUIDPattern.MatchString(ref) {
		return true
	}
	if urnOIDPattern.MatchString(ref) {
		return true
	}

	return false
}

// extractResourceType extracts the resource type from a reference string.
// It validates the extracted type against the registry to ensure it's a valid FHIR resource.
func (v *Validator) extractResourceType(ref string) string {
	// Fragment reference.
	if strings.HasPrefix(ref, "#") {
		return "" // Can't determine type from fragment
	}

	// URN reference.
	if strings.HasPrefix(ref, "urn:") {
		return "" // Can't determine type from URN
	}

	// Remove _history suffix if present (e.g., "Procedure/example/_history/1" -> "Procedure/example")
	ref = strings.Split(ref, "/_history/")[0]

	// Relative reference: ResourceType/id.
	parts := strings.Split(ref, "/")
	if len(parts) >= 2 {
		// For relative: first part is type
		candidate := parts[0]
		if v.registry.IsResourceType(candidate) {
			return candidate
		}
	}

	// Absolute URL: extract ResourceType from path.
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		// Find the last valid resource type in the path.
		for i := len(parts) - 2; i >= 0; i-- {
			candidate := parts[i]
			if v.registry.IsResourceType(candidate) {
				return candidate
			}
		}
	}

	return ""
}

// findElementDef finds an ElementDefinition by path in the StructureDefinition.
func (v *Validator) findElementDef(sd *registry.StructureDefinition, path string) *registry.ElementDefinition {
	if sd == nil || sd.Snapshot == nil {
		return nil
	}

	for i := range sd.Snapshot.Element {
		if sd.Snapshot.Element[i].Path == path {
			return &sd.Snapshot.Element[i]
		}
	}
	return nil
}
