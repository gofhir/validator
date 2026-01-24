package phase

import (
	"context"
	"strings"

	"github.com/gofhir/validator/service"
)

// Note: SlicingRules type and constants are defined in slicing.go

// ExtensionSliceInfo contains information about an extension slice defined in a profile.
type ExtensionSliceInfo struct {
	// SliceName is the name of the slice (e.g., "segundoApellido")
	SliceName string
	// ExtensionURL is the URL of the extension this slice expects
	ExtensionURL string
	// Profile is the profile URL that defines this extension (from type.profile)
	Profile string
	// Min is the minimum cardinality
	Min int
	// Max is the maximum cardinality ("*" for unlimited)
	Max string
	// MustSupport indicates if this extension is must-support
	MustSupport bool
}

// ExtensionSlicingInfo contains slicing information for an extension element.
type ExtensionSlicingInfo struct {
	// ElementPath is the path of the extension element (e.g., "Patient.name.family.extension")
	ElementPath string
	// Rules defines how unmatched extensions are handled
	Rules SlicingRules
	// Slices contains the defined extension slices
	Slices []ExtensionSliceInfo
}

// ProfileExtensionResolver resolves extension slicing information from profiles.
// It provides methods to look up what extensions are allowed at a given path
// according to the profile being validated against.
type ProfileExtensionResolver struct {
	profileService service.ProfileResolver
	// cache stores resolved extension slicing info by profile URL + element path
	cache map[string]*ExtensionSlicingInfo
}

// NewProfileExtensionResolver creates a new ProfileExtensionResolver.
func NewProfileExtensionResolver(profileService service.ProfileResolver) *ProfileExtensionResolver {
	return &ProfileExtensionResolver{
		profileService: profileService,
		cache:          make(map[string]*ExtensionSlicingInfo),
	}
}

// GetExtensionSlicingInfo retrieves slicing information for extensions at a given element path.
// It looks up the profile to find:
// - The slicing rules (open/closed) for the extension element
// - The defined extension slices (URLs, cardinality, etc.)
//
// Parameters:
//   - ctx: Context for cancellation
//   - profile: The StructureDefinition to look up (can be nil for base type)
//   - resourceType: The resource type (e.g., "Patient")
//   - elementPath: The path where extensions are found (e.g., "name[0]._family" for Patient.name.family.extension)
//
// Returns the slicing info or nil if no slicing is defined.
func (r *ProfileExtensionResolver) GetExtensionSlicingInfo(
	ctx context.Context,
	profile *service.StructureDefinition,
	resourceType string,
	elementPath string,
) *ExtensionSlicingInfo {
	if profile == nil || r.profileService == nil {
		return nil
	}

	// Normalize the element path for profile lookup:
	// - Remove array indices: "name[0]" -> "name"
	// - Remove primitive element prefix: "_family" -> "family"
	normalizedPath := normalizePathForProfile(elementPath)

	// Build cache key using normalized path
	cacheKey := profile.URL + "#" + resourceType + "." + normalizedPath + ".extension"
	if cached, ok := r.cache[cacheKey]; ok {
		return cached
	}

	// Build the full extension path to look for
	// e.g., "Patient.name.family.extension" for elementPath "name.family"
	var extensionPath string
	if normalizedPath == "" {
		extensionPath = resourceType + ".extension"
	} else {
		extensionPath = resourceType + "." + normalizedPath + ".extension"
	}

	// Find the extension element with slicing in the profile's snapshot
	info := r.findExtensionSlicing(profile, extensionPath)
	if info != nil {
		r.cache[cacheKey] = info
	}

	return info
}

// normalizePathForProfile normalizes a resource path for profile lookup.
// It removes:
// - Array indices: "name[0]" -> "name"
// - Primitive element prefix: "_family" -> "family" (used for extensions on primitives)
func normalizePathForProfile(path string) string {
	if path == "" {
		return ""
	}

	// Split by dots to process each segment
	segments := strings.Split(path, ".")
	normalized := make([]string, 0, len(segments))

	for _, segment := range segments {
		// Remove array indices
		if idx := strings.Index(segment, "["); idx > 0 {
			segment = segment[:idx]
		}

		// Remove primitive element prefix
		// In FHIR JSON, extensions on primitives use "_elementName" (e.g., "_family")
		if strings.HasPrefix(segment, "_") && len(segment) > 1 {
			segment = segment[1:]
		}

		if segment != "" {
			normalized = append(normalized, segment)
		}
	}

	return strings.Join(normalized, ".")
}

// findExtensionSlicing finds extension slicing information in a profile snapshot.
func (r *ProfileExtensionResolver) findExtensionSlicing(
	profile *service.StructureDefinition,
	extensionPath string,
) *ExtensionSlicingInfo {
	var info *ExtensionSlicingInfo

	// First pass: find the base extension element with slicing definition
	for _, elem := range profile.Snapshot {
		// Match the extension element path (may include slice name in ID)
		if elem.Path == extensionPath && elem.Slicing != nil {
			info = &ExtensionSlicingInfo{
				ElementPath: extensionPath,
				Rules:       SlicingRules(elem.Slicing.Rules),
				Slices:      []ExtensionSliceInfo{},
			}
			break
		}
	}

	if info == nil {
		// No slicing defined for this extension element
		return nil
	}

	// Second pass: collect all slices
	for _, elem := range profile.Snapshot {
		if elem.Path != extensionPath {
			continue
		}

		// Skip the base element (no slice name)
		if elem.SliceName == "" {
			continue
		}

		slice := ExtensionSliceInfo{
			SliceName:   elem.SliceName,
			Min:         elem.Min,
			Max:         elem.Max,
			MustSupport: elem.MustSupport,
		}

		// Extract the extension URL from the type.profile
		if len(elem.Types) > 0 && elem.Types[0].Code == "Extension" {
			if len(elem.Types[0].Profile) > 0 {
				slice.Profile = elem.Types[0].Profile[0]
				slice.ExtensionURL = elem.Types[0].Profile[0]
			}
		}

		// Also check for fixed URL in child elements
		// The URL might be defined in a child element like "Patient.name.family.extension:segundoApellido.url"
		sliceURLPath := extensionPath + ":" + elem.SliceName + ".url"
		// Check element ID pattern
		for _, urlElem := range profile.Snapshot {
			if strings.HasPrefix(urlElem.ID, elem.ID) && strings.HasSuffix(urlElem.Path, ".url") {
				if urlElem.Fixed != nil {
					if fixedURL, ok := urlElem.Fixed.(string); ok {
						slice.ExtensionURL = fixedURL
					}
				}
			}
			// Also check ID pattern for fixed URL
			if urlElem.ID == sliceURLPath || strings.Contains(urlElem.ID, ":"+elem.SliceName+".url") {
				if urlElem.Fixed != nil {
					if fixedURL, ok := urlElem.Fixed.(string); ok {
						slice.ExtensionURL = fixedURL
					}
				}
			}
		}

		if slice.ExtensionURL != "" {
			info.Slices = append(info.Slices, slice)
		}
	}

	return info
}

// FindMatchingSlice finds a slice that matches the given extension URL.
// Returns the matching slice info or nil if no match found.
func (r *ProfileExtensionResolver) FindMatchingSlice(
	info *ExtensionSlicingInfo,
	extensionURL string,
) *ExtensionSliceInfo {
	if info == nil {
		return nil
	}

	for i := range info.Slices {
		if info.Slices[i].ExtensionURL == extensionURL {
			return &info.Slices[i]
		}
	}

	return nil
}

// IsExtensionAllowed checks if an extension is allowed according to slicing rules.
// Returns:
//   - allowed: true if the extension is allowed
//   - defined: true if the extension matches a defined slice
//   - severity: the issue severity if not allowed (error for closed, warning for open)
func (r *ProfileExtensionResolver) IsExtensionAllowed(
	info *ExtensionSlicingInfo,
	extensionURL string,
) (allowed bool, defined bool, severity IssueSeverityHint) {
	if info == nil {
		// No slicing defined - extension is allowed but not defined
		return true, false, SeverityHintNone
	}

	// Check if extension matches a defined slice
	matchingSlice := r.FindMatchingSlice(info, extensionURL)
	if matchingSlice != nil {
		return true, true, SeverityHintNone
	}

	// Extension doesn't match any defined slice
	switch info.Rules {
	case SlicingRulesClosed:
		// Closed slicing - extension not allowed
		return false, false, SeverityHintError
	case SlicingRulesOpen, SlicingRulesOpenAtEnd:
		// Open slicing - extension allowed but generates warning
		return true, false, SeverityHintWarning
	default:
		// Default to open
		return true, false, SeverityHintWarning
	}
}

// IssueSeverityHint indicates the severity for validation issues.
type IssueSeverityHint int

const (
	SeverityHintNone IssueSeverityHint = iota
	SeverityHintWarning
	SeverityHintError
)

// ClearCache clears the internal cache.
func (r *ProfileExtensionResolver) ClearCache() {
	r.cache = make(map[string]*ExtensionSlicingInfo)
}
