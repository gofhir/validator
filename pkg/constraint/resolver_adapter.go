package constraint

import (
	"context"
	"encoding/json"
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
	if reference == "" {
		return nil, nil
	}

	// Fragment reference (#id) — look in contained resources of each entry.
	if strings.HasPrefix(reference, "#") {
		return r.resolveContained(reference[1:])
	}

	// Full URL or relative reference — search Bundle entries by fullUrl.
	return r.resolveByFullURL(reference)
}

// resolveByFullURL searches Bundle entries for a matching fullUrl.
func (r *fhirpathResolver) resolveByFullURL(reference string) ([]byte, error) {
	entries, ok := r.bundleData["entry"].([]any)
	if !ok {
		return nil, nil
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

		// Direct match.
		if fullURL == reference {
			return marshalResource(entryMap)
		}

		// Relative reference match: "Patient/123" matches "http://example.org/fhir/Patient/123".
		if strings.HasSuffix(fullURL, "/"+reference) {
			return marshalResource(entryMap)
		}
	}

	return nil, nil
}

// resolveContained searches for a contained resource by id across all Bundle entries.
func (r *fhirpathResolver) resolveContained(id string) ([]byte, error) {
	entries, ok := r.bundleData["entry"].([]any)
	if !ok {
		return nil, nil
	}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		resource, ok := entryMap["resource"].(map[string]any)
		if !ok {
			continue
		}
		if result := findContainedByID(resource, id); result != nil {
			return result, nil
		}
	}

	return nil, nil
}

// findContainedByID searches a resource's contained array for a resource with the given id.
func findContainedByID(resource map[string]any, id string) []byte {
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
			data, err := json.Marshal(res)
			if err != nil {
				continue
			}
			return data
		}
	}
	return nil
}

// marshalResource extracts and marshals the "resource" field from a Bundle entry.
func marshalResource(entry map[string]any) ([]byte, error) {
	resource, ok := entry["resource"].(map[string]any)
	if !ok {
		return nil, nil
	}
	return json.Marshal(resource)
}
