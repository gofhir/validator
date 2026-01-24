package phase

import (
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pool"
	"github.com/gofhir/validator/service"
)

// ElementWalker provides utilities for walking FHIR resource elements.
type ElementWalker struct {
	resource map[string]any
	profile  *service.StructureDefinition
}

// NewElementWalker creates a new walker for a resource.
func NewElementWalker(resource map[string]any, profile *service.StructureDefinition) *ElementWalker {
	return &ElementWalker{
		resource: resource,
		profile:  profile,
	}
}

// WalkFunc is called for each element during tree walking.
// path is the FHIRPath to the element.
// value is the element value.
// def is the ElementDefinition if found in the profile.
// Return false to stop walking.
type WalkFunc func(path string, value any, def *service.ElementDefinition) bool

// Walk traverses all elements in the resource.
func (w *ElementWalker) Walk(fn WalkFunc) {
	if w.resource == nil {
		return
	}
	w.walkMap("", w.resource, fn)
}

func (w *ElementWalker) walkMap(basePath string, m map[string]any, fn WalkFunc) {
	for key, value := range m {
		path := joinPath(basePath, key)
		def := w.findElementDef(path)

		if !fn(path, value, def) {
			return
		}

		// Recurse into nested structures
		switch v := value.(type) {
		case map[string]any:
			w.walkMap(path, v, fn)
		case []any:
			for i, item := range v {
				itemPath := pool.BuildPath(func(b *pool.PathBuilder) {
					b.WriteString(path)
					b.AppendIndex(i)
				})
				if itemMap, ok := item.(map[string]any); ok {
					if !fn(itemPath, item, def) {
						return
					}
					w.walkMap(itemPath, itemMap, fn)
				} else {
					fn(itemPath, item, def)
				}
			}
		}
	}
}

func (w *ElementWalker) findElementDef(path string) *service.ElementDefinition {
	if w.profile == nil {
		return nil
	}

	// Try exact match first
	for i := range w.profile.Snapshot {
		if w.profile.Snapshot[i].Path == path {
			return &w.profile.Snapshot[i]
		}
	}

	// Try without array indices
	cleanPath := removeArrayIndices(path)
	for i := range w.profile.Snapshot {
		if w.profile.Snapshot[i].Path == cleanPath {
			return &w.profile.Snapshot[i]
		}
	}

	return nil
}

// joinPath joins path segments.
func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

// removeArrayIndices removes [n] from a path.
func removeArrayIndices(path string) string {
	result := make([]byte, 0, len(path))
	inBracket := false
	for i := 0; i < len(path); i++ {
		if path[i] == '[' {
			inBracket = true
		} else if path[i] == ']' {
			inBracket = false
		} else if !inBracket {
			result = append(result, path[i])
		}
	}
	return string(result)
}

// GetResourceType extracts the resourceType from a resource map.
func GetResourceType(resource map[string]any) string {
	if rt, ok := resource["resourceType"].(string); ok {
		return rt
	}
	return ""
}

// GetResourceID extracts the id from a resource map.
func GetResourceID(resource map[string]any) string {
	if id, ok := resource["id"].(string); ok {
		return id
	}
	return ""
}

// GetMetaProfiles extracts profile URLs from meta.profile.
func GetMetaProfiles(resource map[string]any) []string {
	meta, ok := resource["meta"].(map[string]any)
	if !ok {
		return nil
	}
	profiles, ok := meta["profile"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if s, ok := p.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// IsExtensionElement returns true if the key is an extension element.
func IsExtensionElement(key string) bool {
	return key == "extension" || key == "modifierExtension"
}

// IsPrimitiveExtension returns true if this is a primitive extension (_fieldName).
func IsPrimitiveExtension(key string) bool {
	return strings.HasPrefix(key, "_")
}

// GetPrimitiveExtensionKey returns the primitive extension key for a field.
func GetPrimitiveExtensionKey(field string) string {
	return "_" + field
}

// GetFieldFromPrimitiveExtension extracts the field name from _fieldName.
func GetFieldFromPrimitiveExtension(key string) string {
	if strings.HasPrefix(key, "_") {
		return key[1:]
	}
	return key
}

// BuildElementIndex creates a map from path to ElementDefinition for fast lookup.
func BuildElementIndex(snapshot []service.ElementDefinition) map[string]*service.ElementDefinition {
	index := make(map[string]*service.ElementDefinition, len(snapshot))
	for i := range snapshot {
		index[snapshot[i].Path] = &snapshot[i]
	}
	return index
}

// FHIRPrimitiveTypes lists all FHIR primitive types.
var FHIRPrimitiveTypes = map[string]bool{
	"boolean":      true,
	"integer":      true,
	"integer64":    true,
	"string":       true,
	"decimal":      true,
	"uri":          true,
	"url":          true,
	"canonical":    true,
	"base64Binary": true,
	"instant":      true,
	"date":         true,
	"dateTime":     true,
	"time":         true,
	"code":         true,
	"oid":          true,
	"id":           true,
	"markdown":     true,
	"unsignedInt":  true,
	"positiveInt":  true,
	"uuid":         true,
	"xhtml":        true,
}

// IsPrimitiveType returns true if the type code is a FHIR primitive type.
func IsPrimitiveType(typeCode string) bool {
	return FHIRPrimitiveTypes[typeCode]
}

// FHIRComplexTypes lists common FHIR complex types.
var FHIRComplexTypes = map[string]bool{
	"Address":          true,
	"Age":              true,
	"Annotation":       true,
	"Attachment":       true,
	"CodeableConcept":  true,
	"CodeableReference": true,
	"Coding":           true,
	"ContactDetail":    true,
	"ContactPoint":     true,
	"Contributor":      true,
	"Count":            true,
	"DataRequirement":  true,
	"Distance":         true,
	"Dosage":           true,
	"Duration":         true,
	"Expression":       true,
	"Extension":        true,
	"HumanName":        true,
	"Identifier":       true,
	"Meta":             true,
	"Money":            true,
	"Narrative":        true,
	"ParameterDefinition": true,
	"Period":           true,
	"Quantity":         true,
	"Range":            true,
	"Ratio":            true,
	"RatioRange":       true,
	"Reference":        true,
	"RelatedArtifact":  true,
	"SampledData":      true,
	"Signature":        true,
	"Timing":           true,
	"TriggerDefinition": true,
	"UsageContext":     true,
}

// IsComplexType returns true if the type code is a FHIR complex type.
func IsComplexType(typeCode string) bool {
	return FHIRComplexTypes[typeCode]
}

// BaseIssue creates a base issue with common fields set.
func BaseIssue(severity fv.IssueSeverity, code fv.IssueType, diagnostics, path, phase string) fv.Issue {
	return fv.Issue{
		Severity:    severity,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  []string{path},
		Phase:       phase,
	}
}

// ErrorIssue creates an error issue.
func ErrorIssue(code fv.IssueType, diagnostics, path, phase string) fv.Issue {
	return BaseIssue(fv.SeverityError, code, diagnostics, path, phase)
}

// WarningIssue creates a warning issue.
func WarningIssue(code fv.IssueType, diagnostics, path, phase string) fv.Issue {
	return BaseIssue(fv.SeverityWarning, code, diagnostics, path, phase)
}

// InformationIssue creates an informational issue.
func InformationIssue(code fv.IssueType, diagnostics, path, phase string) fv.Issue {
	return BaseIssue(fv.SeverityInformation, code, diagnostics, path, phase)
}

// DisplayMismatchIssue creates a warning when the display doesn't match the expected value.
// According to FHIR: "If both code and display are provided, the display SHALL match the display
// for the code in the code system (though systems MAY use a different display when presenting
// the concept to users)."
func DisplayMismatchIssue(code, providedDisplay, expectedDisplay, path, phase string) fv.Issue {
	return fv.Issue{
		Severity: fv.SeverityWarning,
		Code:     fv.IssueTypeCodeInvalid,
		Diagnostics: fmt.Sprintf(
			"Display value '%s' for code '%s' does not match the expected display '%s' from the CodeSystem. "+
				"According to FHIR, when both code and display are provided, the display SHOULD match the display defined in the CodeSystem.",
			providedDisplay, code, expectedDisplay),
		Expression: []string{path},
		Phase:      phase,
	}
}

// ValidateID validates a FHIR id value.
// FHIR ids must match pattern: [A-Za-z0-9\-\.]{1,64}
func ValidateID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.') {
			return false
		}
	}
	return true
}
