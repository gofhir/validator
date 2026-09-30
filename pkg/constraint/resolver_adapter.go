package constraint

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
)

// fhirpathResolver adapts Bundle data to the eval.Resolver interface
// required by the fhirpath engine for resolve() evaluation.
type fhirpathResolver struct {
	bundleData map[string]any
}

// Resolve resolves a FHIR reference to the target resource JSON.
// It searches Bundle entries by fullUrl and contained resources by fragment ID.
func (r *fhirpathResolver) Resolve(_ context.Context, reference string) ([]byte, error) {
	res, ok := ResolveReference(r.bundleData, reference)
	if !ok {
		return nil, nil
	}
	return json.Marshal(res)
}

// ResolveReference finds the resource a reference names inside the resource being validated,
// root: a fragment reference (#id) among the contained resources of root or of any Bundle entry,
// and any other reference among the Bundle entries (see ResolveInBundle).
func ResolveReference(root map[string]any, reference string) (map[string]any, bool) {
	if reference == "" || root == nil {
		return nil, false
	}
	if id, ok := strings.CutPrefix(reference, "#"); ok {
		if res := findContainedByID(root, id); res != nil {
			return res, true
		}
		for _, entry := range bundleEntries(root) {
			if resource, ok := entry["resource"].(map[string]any); ok {
				if res := findContainedByID(resource, id); res != nil {
					return res, true
				}
			}
		}
		return nil, false
	}
	return ResolveInBundle(root, reference)
}

// ResolveInBundle finds a Bundle entry's resource by fullUrl, or by a relative reference that ends
// it ("Patient/123" for "http://example.org/fhir/Patient/123"). Any other resource has no entries.
func ResolveInBundle(bundle map[string]any, reference string) (map[string]any, bool) {
	for _, entry := range bundleEntries(bundle) {
		fullURL, _ := entry["fullUrl"].(string)
		if fullURL == "" {
			continue
		}
		if fullURL == reference || strings.HasSuffix(fullURL, "/"+reference) {
			if resource, ok := entry["resource"].(map[string]any); ok {
				return resource, true
			}
		}
	}
	return nil, false
}

// ContainedByID returns the contained resource of resource with this id: the target of the
// fragment reference "#id" made from within it (references.html#contained).
func ContainedByID(resource map[string]any, id string) (map[string]any, bool) {
	res := findContainedByID(resource, id)
	return res, res != nil
}

// IsContainedIn reports whether resource is one of container's contained resources (the same
// value, not an equal one).
func IsContainedIn(container, resource map[string]any) bool {
	contained, _ := container["contained"].([]any)
	for _, item := range contained {
		if m, ok := item.(map[string]any); ok && sameMap(m, resource) {
			return true
		}
	}
	return false
}

func sameMap(a, b map[string]any) bool {
	return len(a) == len(b) && reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// bundleEntries returns the entries of a Bundle, or none for any other resource.
func bundleEntries(root map[string]any) []map[string]any {
	entries, _ := root["entry"].([]any)
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// findContainedByID searches a resource's contained array for a resource with the given id.
func findContainedByID(resource map[string]any, id string) map[string]any {
	contained, ok := resource["contained"].([]any)
	if !ok {
		return nil
	}
	for _, item := range contained {
		res, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if resID, _ := res["id"].(string); resID == id {
			return res
		}
	}
	return nil
}
