// Package reference validates FHIR Reference elements.
package reference

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/gofhir/validator/v2/internal/bundleref"
	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/internal/slicerecord"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/slicematch"
	"github.com/gofhir/validator/v2/pkg/walker"
)

const (
	typeCodeReference = "Reference"
	resourceTypeKey   = "resourceType"
	bundleType        = "Bundle"
)

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

	contained := containedOf(resource)

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
	// definitions names the definition a value declares for itself (an extension's url).
	definitions DefinitionSource
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

// nested returns the context of a resource the walk visits, from its parent's, by how it is held:
// a contained resource shares its container's (references.html#contained); an entry has its own
// contained resources, its fullUrl and its Bundle; a resource another element holds (a
// parameter's, a response's outcome) has its own contained resources, and resolves the rest where
// its holder does, from its holder's base (the fullUrl of the entry whose resource holds it: an
// entry's response outcome is not the entry's resource, bundle.html#references). A Bundle is,
// besides, where its own references resolve, then the Bundles that hold it; one that is not an
// entry's resource has no base, as the Bundle validated has none.
func (rc *refContext) nested(w *walker.ResourceContext) *refContext {
	var in refContext
	switch w.Kind {
	case walker.KindContained:
		in = *rc
		in.inContained = true
	case walker.KindHeld:
		in = refContext{ctx: rc.ctx, bundle: rc.bundle, contained: NewContainedContext(w.Data), root: w.Data,
			container: rc.container, outer: rc.outer, fullURL: rc.fullURL}
	default:
		in = refContext{ctx: rc.ctx, bundle: rc.bundle, contained: NewContainedContext(w.Data), root: w.Data,
			container: rc.container, outer: rc.outer, fullURL: w.FullURL}
		in.within(w.Container)
	}
	if w.ResourceType == bundleType {
		in.within(w.Data)
		if w.Kind != walker.KindEntry {
			in.fullURL = ""
		}
	}
	return &in
}

// within makes bundle, a Bundle, the one rc's references resolve in, then the Bundle they resolved
// in before and those that hold it.
func (rc *refContext) within(bundle map[string]any) {
	if bundle == nil || sameMap(bundle, rc.container) {
		return
	}
	if t, _ := rc.container[resourceTypeKey].(string); t == bundleType {
		rc.outer = append([]map[string]any{rc.container}, rc.outer...)
	}
	rc.container = bundle
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
		rc.fullURL = bundleref.IndexOf(ctx, in.container).FullURLOf(holder)
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

// WithIndexes returns ctx with a new store of the entry indexes references are resolved with,
// for one validation: its conformance checks share it. A store keys Bundles by address, so it
// must not outlive the validation.
func WithIndexes(ctx context.Context) context.Context {
	return bundleref.WithIndexes(ctx)
}

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
		for _, c := range containedOf(rc.root) {
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
	return bundleref.IndexOf(rc.ctx, rc.container).Matches(ref, rc.fullURL)
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
	ctx = bundleref.EnsureIndexes(ctx)
	rc := rootContext(ctx, resource, bundleCtx)

	// The root resource, over the definitions (walk).
	root := sd.Tree().Root()
	if root == nil {
		return
	}
	v.walk(ctx, sd, root, "", resource, resourceType, rc, result)

	// Walk every resource it holds (contained resources, Bundle entries, the resources other
	// elements hold), each against the profiles its meta.profile declares and its type's
	// definition. A reference that several of them find wrong is reported once.
	nested := issue.GetPooledResult()
	defer issue.ReleaseResult(nested)
	// Each resource's context, by its path (refContext.nested). The walk visits a resource before
	// those it holds.
	contexts := map[string]*refContext{resourceType: rc}
	v.walker.WalkAllWithProfilesContext(ctx, resource, resourceType, resourceType, func(w *walker.ResourceContext) bool {
		if w.FHIRPath == resourceType {
			return true // the root, walked above
		}
		parent := contexts[w.ParentPath]
		if parent == nil {
			parent = rc
		}
		here, seen := contexts[w.FHIRPath]
		if !seen {
			here = parent.nested(w)
			contexts[w.FHIRPath] = here
		}
		if r := w.SD.Tree().Root(); r != nil {
			v.walk(ctx, w.SD, r, "", w.Data, w.FHIRPath, here, nested)
		}
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

// walk checks the references among the values of node's children in one instance of node, inst,
// held at fhirPath, each under the definition that governs it: the slice it matched
// (slicerecord.Slices), or the element. It descends into each value under that definition
// (Registry.ChildrenOf: a contentReference, the profile its type declares, a backbone element's
// children, the definition an extension's url names, or else Extension's), a primitive's extensions
// included. A resource an element holds is checked by the resource walk.
func (v *Validator) walk(ctx context.Context, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, inst map[string]any, fhirPath string, rc *refContext, result *issue.Result) {
	c := v.registry.ChildrenOf(ctx, sd, node, typeCode, v.selfDefinitions(ctx, inst))
	if c.Unresolved.Canonical != "" {
		// The profile its type declares cannot be used (the cardinality phase reports it): the value
		// is still one of its type, whose definition governs it, as the HL7 validator checks it.
		c = v.registry.TypeChildrenOf(node, typeCode)
	}
	matched := slicerecord.In(ctx)
	for _, child := range c.Nodes {
		for _, cv := range elementvalues.Of(child, inst, v.registry) {
			if !elementvalues.Written(child, cv, v.registry) || (cv.TypeCode != "" && !v.declares(child.Def, cv.TypeCode)) {
				continue // the structural phase reports it, and HL7 checks nothing in it
			}
			gov := matched.Of(child, inst, cv.Key, cv.Index)
			path := elementvalues.ItemPath(child, cv, fhirPath)
			if cv.TypeCode == typeCodeReference {
				v.validateReference(cv.Value, gov.Def, path, rc, result)
			}
			if v.registry.HoldsResource(child.Def) {
				continue
			}
			m, ok := cv.Value.(map[string]any)
			if !ok {
				// A primitive's id and extensions are its "_key" sibling (json.html#primitive).
				if m = cv.Ext; m == nil {
					continue
				}
			}
			nsd, next := v.registry.MemberTree(ctx, c.SD, gov)
			v.walk(ctx, nsd, next, cv.TypeCode, m, path, rc, result)
		}
	}
}

// declares reports whether def's element, as its base defines it (ElementDefinition.base), has
// typeCode among its types: a choice property naming another type is not the element's (the
// structural phase reports it unknown, as HL7 does), while a profile narrowing the types leaves the
// property the element's.
func (v *Validator) declares(def *registry.ElementDefinition, typeCode string) bool {
	types := def.Type
	if def.Base != nil && def.Base.Path != "" {
		if base := v.registry.GetElementDefinition(def.Base.Path); base != nil {
			types = base.Type
		}
	}
	return len(types) == 0 || slices.ContainsFunc(types, func(t registry.Type) bool { return t.Code == typeCode })
}

// DefinitionSource names the definition a value declares for itself, which governs the value
// wherever it is used: an extension's url names the definition the extension conforms to
// (extensibility.html). It returns nil for a value that declares none.
type DefinitionSource interface {
	DefinitionOf(ctx context.Context, typeCode string, value map[string]any) *registry.StructureDefinition
}

// WithDefinitions sets the source of the definitions values declare for themselves. Without it, an
// extension is checked against the definitions of the element that holds it only.
func WithDefinitions(d DefinitionSource) Option {
	return func(v *Validator) { v.definitions = d }
}

// selfDefinitions returns, for an instance, the definition it declares for itself for a type.
func (v *Validator) selfDefinitions(ctx context.Context, inst map[string]any) func(string) *registry.StructureDefinition {
	if v.definitions == nil {
		return nil
	}
	return func(typeCode string) *registry.StructureDefinition {
		return v.definitions.DefinitionOf(ctx, typeCode, inst)
	}
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

	checkFragment(refStr, fhirPath, rc, result)

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
	// Reference.type names the target's type, which must be one the element allows too, whatever the
	// target is found to be (reported above).
	if refType != "" && refType != targetType {
		v.validateTargetProfile(refType, nil, refStr, elemDef, fhirPath, rc, result)
	}

	v.validateAggregation(refStr, elemDef, fhirPath, rc, result)
}

// checkFragment checks a fragment reference resolves (references.html#contained): "#id", to a
// resource the referring one contains (ref-1: "SHALL have a contained resource if a local reference
// is provided"); "#", to the resource that contains the referring one, which must be contained.
func checkFragment(refStr, fhirPath string, rc *refContext, result *issue.Result) {
	id, ok := strings.CutPrefix(refStr, "#")
	switch {
	case !ok:
	case id == "":
		if !rc.inContained {
			result.AddErrorWithID(issue.DiagReferenceContainerNotFound, nil, fhirPath+".reference")
		}
	case rc.contained != nil:
		if _, found := rc.contained.IDIndex[id]; !found {
			result.AddErrorWithID(issue.DiagReferenceContainedNotFound, map[string]any{"id": id}, fhirPath+".reference")
		}
	}
}

// validateAggregation checks a reference against the aggregation modes its element allows
// (ElementDefinition.type.aggregation), by where its target is found: "contained", a resource the
// referring one contains ("#id"), or, from a contained resource, its container ("#"); "bundled",
// an entry of the Bundle it is in; "referenced", a resource resolved outside it, so any literal
// reference that is no fragment. Without modes, any
// reference is allowed. A reference allowed by none of them is reported, as the HL7 validator
// reports a bundled or contained reference not found.
func (v *Validator) validateAggregation(refStr string, elemDef *registry.ElementDefinition, fhirPath string, rc *refContext, result *issue.Result) {
	modes := v.getAggregationModes(elemDef)
	if len(modes) == 0 {
		return
	}
	fragment := strings.HasPrefix(refStr, "#")
	for _, mode := range modes {
		switch mode {
		case "contained":
			if _, found := resolveTarget(refStr, rc); fragment && found {
				return
			}
		case "bundled":
			if _, found := resolveTarget(refStr, rc); !fragment && found {
				return
			}
		case "referenced":
			if !fragment {
				return
			}
		}
	}
	result.AddErrorWithID(
		issue.DiagReferenceAggregationMode,
		map[string]any{
			"reference": refStr,
			"allowed":   strings.Join(modes, ", "),
		},
		fhirPath+".reference",
	)
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

// containedOf returns resource's contained resources: its array, or the one resource written where
// the array goes, which the HL7 validator also takes as the array's one item.
func containedOf(resource map[string]any) []any {
	switch c := resource["contained"].(type) {
	case []any:
		return c
	case map[string]any:
		return []any{c}
	}
	return nil
}
