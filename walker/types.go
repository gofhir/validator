package walker

// FHIR type constants for common types.
const (
	TypeString = "string"
)

// SystemTypeMapping maps FHIRPath system types to FHIR primitive types.
// These are used in StructureDefinitions when defining element types
// for primitive elements like id, string, instant, etc.
var SystemTypeMapping = map[string]string{
	"http://hl7.org/fhirpath/System.String":   TypeString,
	"http://hl7.org/fhirpath/System.Boolean":  "boolean",
	"http://hl7.org/fhirpath/System.Integer":  "integer",
	"http://hl7.org/fhirpath/System.Decimal":  "decimal",
	"http://hl7.org/fhirpath/System.DateTime": "dateTime",
	"http://hl7.org/fhirpath/System.Time":     "time",
	"http://hl7.org/fhirpath/System.Date":     "date",
}

// FHIRPrimitiveTypes contains all FHIR primitive type codes.
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

// FHIRComplexTypes contains FHIR complex data types (not resources).
// These types have their own StructureDefinitions that need to be loaded
// for proper validation.
var FHIRComplexTypes = map[string]bool{
	"Address":             true,
	"Age":                 true,
	"Annotation":          true,
	"Attachment":          true,
	"BackboneElement":     true,
	"CodeableConcept":     true,
	"CodeableReference":   true,
	"Coding":              true,
	"ContactDetail":       true,
	"ContactPoint":        true,
	"Contributor":         true,
	"Count":               true,
	"DataRequirement":     true,
	"Distance":            true,
	"Dosage":              true,
	"Duration":            true,
	"Element":             true,
	"ElementDefinition":   true,
	"Expression":          true,
	"Extension":           true,
	"HumanName":           true,
	"Identifier":          true,
	"MarketingStatus":     true,
	"Meta":                true,
	"Money":               true,
	"MoneyQuantity":       true,
	"Narrative":           true,
	"ParameterDefinition": true,
	"Period":              true,
	"Population":          true,
	"ProdCharacteristic":  true,
	"ProductShelfLife":    true,
	"Quantity":            true,
	"Range":               true,
	"Ratio":               true,
	"RatioRange":          true,
	"Reference":           true,
	"RelatedArtifact":     true,
	"SampledData":         true,
	"Signature":           true,
	"SimpleQuantity":      true,
	"SubstanceAmount":     true,
	"Timing":              true,
	"TriggerDefinition":   true,
	"UsageContext":        true,
}

// ChoiceTypeSuffixes contains all valid suffixes for choice types (value[x]).
// When encountering valueString, valueCodeableConcept, etc., we need to
// check if the base element value[x] allows that type.
var ChoiceTypeSuffixes = []string{
	// Primitives
	"String",
	"Boolean",
	"Integer",
	"Integer64",
	"Decimal",
	"DateTime",
	"Date",
	"Time",
	"Instant",
	"Uri",
	"Url",
	"Canonical",
	"Code",
	"Id",
	"Markdown",
	"Base64Binary",
	"Oid",
	"Uuid",
	"PositiveInt",
	"UnsignedInt",

	// Complex types
	"Address",
	"Age",
	"Annotation",
	"Attachment",
	"CodeableConcept",
	"CodeableReference",
	"Coding",
	"ContactDetail",
	"ContactPoint",
	"Contributor",
	"Count",
	"DataRequirement",
	"Distance",
	"Dosage",
	"Duration",
	"Expression",
	"HumanName",
	"Identifier",
	"Meta",
	"Money",
	"MoneyQuantity",
	"Narrative",
	"ParameterDefinition",
	"Period",
	"Quantity",
	"Range",
	"Ratio",
	"RatioRange",
	"Reference",
	"RelatedArtifact",
	"SampledData",
	"Signature",
	"SimpleQuantity",
	"Timing",
	"TriggerDefinition",
	"UsageContext",
}

// IsPrimitiveType returns true if the type code is a FHIR primitive type.
func IsPrimitiveType(typeCode string) bool {
	return FHIRPrimitiveTypes[typeCode]
}

// IsComplexType returns true if the type code is a FHIR complex type.
func IsComplexType(typeCode string) bool {
	return FHIRComplexTypes[typeCode]
}

// IsSystemType returns true if the type is a FHIRPath system type URL.
func IsSystemType(typeCode string) bool {
	_, ok := SystemTypeMapping[typeCode]
	return ok
}

// InlineElementTypes contains types whose children are defined inline in the
// parent's StructureDefinition. We should NOT switch type context for these.
var InlineElementTypes = map[string]bool{
	"BackboneElement": true,
	"Element":         true,
}

// isInlineElementType returns true if the type has inline element definitions.
// For these types, children are defined in the parent SD, not in the type's own SD.
func isInlineElementType(typeName string) bool {
	return InlineElementTypes[typeName]
}

// NormalizeSystemType converts a FHIRPath system type URL to a FHIR primitive type.
// If the type is not a system type, it returns the original type.
func NormalizeSystemType(typeCode string) string {
	if normalized, ok := SystemTypeMapping[typeCode]; ok {
		return normalized
	}
	return typeCode
}

// GetGoTypeForFHIRType returns the expected Go type for a FHIR type.
// This is used for type validation.
func GetGoTypeForFHIRType(fhirType string) string {
	// Normalize system types first
	fhirType = NormalizeSystemType(fhirType)

	switch fhirType {
	case "boolean":
		return "bool"

	case "integer", "integer64", "unsignedInt", "positiveInt":
		return "float64" // JSON numbers are float64 in Go

	case "decimal":
		return "float64"

	case TypeString, "uri", "url", "canonical", "code", "id", "oid", "uuid",
		"markdown", "base64Binary", "xhtml":
		return TypeString

	case "date", "dateTime", "time", "instant":
		return TypeString // Date/time types are strings in JSON

	case "BackboneElement", "Element":
		return "map[string]any"

	default:
		// Complex types and resources are objects
		if IsComplexType(fhirType) {
			return "map[string]any"
		}
		// Assume it's a resource type
		return "map[string]any"
	}
}

// ValidateGoType checks if a Go value matches the expected type for a FHIR type.
// Returns true if the types match, false otherwise.
func ValidateGoType(value any, fhirType string) bool {
	// Normalize system types
	fhirType = NormalizeSystemType(fhirType)

	switch fhirType {
	case "boolean":
		_, ok := value.(bool)
		return ok

	case "integer", "integer64", "unsignedInt", "positiveInt":
		f, ok := value.(float64)
		if !ok {
			return false
		}
		// Check if it's actually an integer
		return f == float64(int64(f))

	case "decimal":
		_, ok := value.(float64)
		return ok

	case TypeString, "uri", "url", "canonical", "code", "id", "oid", "uuid",
		"markdown", "base64Binary", "xhtml", "date", "dateTime", "time", "instant":
		_, ok := value.(string)
		return ok

	case "BackboneElement", "Element":
		_, ok := value.(map[string]any)
		return ok

	default:
		// Complex types should be objects
		if IsComplexType(fhirType) {
			_, ok := value.(map[string]any)
			return ok
		}
		// Resource types should also be objects
		_, ok := value.(map[string]any)
		return ok
	}
}

// GetActualGoType returns a string description of the Go type of a value.
func GetActualGoType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}
