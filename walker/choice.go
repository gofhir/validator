package walker

import (
	"strings"

	"github.com/gofhir/validator/service"
)

// ChoiceTypeResult contains the result of resolving a choice type.
type ChoiceTypeResult struct {
	// IsChoice is true if this is a choice type variant
	IsChoice bool

	// BaseName is the base element name (e.g., "value" for "valueString")
	BaseName string

	// TypeName is the resolved type (e.g., "string" for "valueString")
	TypeName string

	// ChoicePath is the [x] path (e.g., "Observation.value[x]")
	ChoicePath string

	// ElementDef is the ElementDefinition for the choice element
	ElementDef *service.ElementDefinition
}

// ResolveChoiceType determines if a key is a choice type variant and resolves it.
// It checks if the key matches any known type suffix and verifies the element exists.
func ResolveChoiceType(key string, index *ElementIndex) *ChoiceTypeResult {
	return ResolveChoiceTypeWithPrefix(key, "", index)
}

// ResolveChoiceTypeWithPrefix resolves a choice type using an optional path prefix.
// The prefix is used to find nested choice types like "doseAndRate.dose[x]".
func ResolveChoiceTypeWithPrefix(key, prefix string, index *ElementIndex) *ChoiceTypeResult {
	if index == nil {
		return resolveByTypeSuffix(key)
	}

	// Check each possible type suffix
	for _, suffix := range ChoiceTypeSuffixes {
		if !strings.HasSuffix(key, suffix) {
			continue
		}

		baseName := key[:len(key)-len(suffix)]
		if baseName == "" {
			continue
		}

		choicePath := baseName + "[x]"

		// Look up the choice element definition
		// First try with prefix (for nested choice types like doseAndRate.dose[x])
		var elemDef *service.ElementDefinition
		if prefix != "" {
			prefixedBaseName := prefix + "." + baseName
			elemDef = index.GetChoiceTypeDefinition(prefixedBaseName)
			if elemDef == nil {
				elemDef = index.Get(prefix + "." + choicePath)
			}
		}

		// Fallback to direct lookup
		if elemDef == nil {
			elemDef = index.GetChoiceTypeDefinition(baseName)
		}
		if elemDef == nil {
			elemDef = index.Get(choicePath)
		}

		if elemDef != nil {
			// Verify the type is allowed
			// Check both PascalCase (suffix) and camelCase (lowerFirst) for compatibility
			typeName := suffix // Use original PascalCase for complex types
			if IsPrimitiveType(lowerFirst(suffix)) {
				typeName = lowerFirst(suffix) // Use lowercase for primitives
			}
			if isTypeAllowed(elemDef, typeName) || isTypeAllowed(elemDef, lowerFirst(suffix)) || isTypeAllowed(elemDef, suffix) {
				return &ChoiceTypeResult{
					IsChoice:   true,
					BaseName:   baseName,
					TypeName:   typeName,
					ChoicePath: choicePath,
					ElementDef: elemDef,
				}
			}
		}
	}

	return &ChoiceTypeResult{IsChoice: false}
}

// resolveByTypeSuffix resolves choice type just by analyzing the key name.
// Used when no index is available.
func resolveByTypeSuffix(key string) *ChoiceTypeResult {
	for _, suffix := range ChoiceTypeSuffixes {
		if strings.HasSuffix(key, suffix) {
			baseName := key[:len(key)-len(suffix)]
			if baseName != "" {
				// Use original PascalCase for complex types, lowercase for primitives
				typeName := suffix
				if IsPrimitiveType(lowerFirst(suffix)) {
					typeName = lowerFirst(suffix)
				}
				return &ChoiceTypeResult{
					IsChoice:   true,
					BaseName:   baseName,
					TypeName:   typeName,
					ChoicePath: baseName + "[x]",
				}
			}
		}
	}
	return &ChoiceTypeResult{IsChoice: false}
}

// isTypeAllowed checks if a type code is in the ElementDefinition's type list.
func isTypeAllowed(elemDef *service.ElementDefinition, typeCode string) bool {
	if elemDef == nil || len(elemDef.Types) == 0 {
		return false
	}

	normalizedCode := NormalizeSystemType(typeCode)

	for _, typeRef := range elemDef.Types {
		normalizedRef := NormalizeSystemType(typeRef.Code)
		if normalizedRef == normalizedCode ||
			strings.EqualFold(normalizedRef, normalizedCode) ||
			typeRef.Code == typeCode {
			return true
		}
	}
	return false
}

// GetChoiceTypeFromKey extracts the type suffix from a choice type key.
// Returns the type name (e.g., "String" for "valueString") or empty if not a choice.
func GetChoiceTypeFromKey(key string) string {
	for _, suffix := range ChoiceTypeSuffixes {
		if strings.HasSuffix(key, suffix) {
			baseName := key[:len(key)-len(suffix)]
			if baseName != "" {
				return suffix
			}
		}
	}
	return ""
}

// GetChoiceBaseName returns the base name of a choice type key.
// Returns the base (e.g., "value" for "valueString") or the original key.
func GetChoiceBaseName(key string) string {
	for _, suffix := range ChoiceTypeSuffixes {
		if strings.HasSuffix(key, suffix) {
			baseName := key[:len(key)-len(suffix)]
			if baseName != "" {
				return baseName
			}
		}
	}
	return key
}

// IsChoiceTypeKey returns true if the key appears to be a choice type variant.
func IsChoiceTypeKey(key string) bool {
	for _, suffix := range ChoiceTypeSuffixes {
		if strings.HasSuffix(key, suffix) {
			baseName := key[:len(key)-len(suffix)]
			if baseName != "" {
				return true
			}
		}
	}
	return false
}

// ValidateChoiceType validates that a choice type value matches the declared type.
func ValidateChoiceType(value any, choiceResult *ChoiceTypeResult) bool {
	if choiceResult == nil || !choiceResult.IsChoice {
		return true // Not a choice type, nothing to validate
	}

	return ValidateGoType(value, choiceResult.TypeName)
}

// GetAllowedChoiceTypes returns all allowed type codes for an element.
// This is useful for generating error messages.
func GetAllowedChoiceTypes(elemDef *service.ElementDefinition) []string {
	if elemDef == nil || len(elemDef.Types) == 0 {
		return nil
	}

	types := make([]string, len(elemDef.Types))
	for i, t := range elemDef.Types {
		types[i] = t.Code
	}
	return types
}

// FormatChoiceTypePath formats a choice type path for error messages.
// Example: "value[x]" with type "String" -> "valueString (value[x])"
func FormatChoiceTypePath(basePath, typeName string) string {
	return basePath + upperFirst(typeName) + " (" + basePath + "[x])"
}
