package phase

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
	"github.com/gofhir/validator/service"
	"github.com/gofhir/validator/walker"
)

// FHIR primitive type constants.
const (
	TypeBoolean  = "boolean"
	TypeString   = "string"
	TypeInstant  = "instant"
	TypeDateTime = "dateTime"
)

// PrimitivesPhase validates FHIR primitive type formats.
// It checks that values conform to their declared primitive type patterns.
type PrimitivesPhase struct {
	profileService service.ProfileResolver
}

// NewPrimitivesPhase creates a new primitives validation phase.
func NewPrimitivesPhase(profileService service.ProfileResolver) *PrimitivesPhase {
	return &PrimitivesPhase{
		profileService: profileService,
	}
}

// Name returns the phase name.
func (p *PrimitivesPhase) Name() string {
	return "primitives"
}

// Validate performs primitive type validation.
func (p *PrimitivesPhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil {
		return issues
	}

	// Get profile for type information
	var profile *service.StructureDefinition
	if p.profileService != nil {
		var err error
		profile, err = p.profileService.FetchStructureDefinitionByType(ctx, pctx.ResourceType)
		if err != nil {
			profile = nil
		}
	}

	// Use TypeAwareTreeWalker for proper type context
	resolver := pctx.TypeResolver
	if resolver == nil {
		resolver = walker.NewDefaultTypeResolver(p.profileService)
	}

	tw := walker.NewTypeAwareTreeWalker(resolver)

	// Walk through elements and validate primitives
	err := tw.Walk(ctx, pctx.ResourceMap, profile, func(wctx *walker.WalkContext) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip non-primitive values (objects and arrays)
		if wctx.IsObject() || wctx.IsArray() {
			return nil
		}

		// Determine the type to validate
		var typeCode string

		// First, try to get type from element definition (with proper type context)
		if wctx.HasElementDef() && len(wctx.ElementDef.Types) > 0 {
			typeCode = walker.NormalizeSystemType(wctx.ElementDef.Types[0].Code)
		}

		// If we have a resolved type name from the walker, use that
		if typeCode == "" && wctx.TypeName != "" {
			typeCode = wctx.TypeName
		}

		// For choice types, use the resolved choice type
		if wctx.IsChoiceType && wctx.ChoiceTypeName != "" {
			typeCode = wctx.ChoiceTypeName
		}

		// Fallback: infer type from field name
		if typeCode == "" {
			typeCode = inferTypeFromFieldName(wctx.Key)
		}

		// Validate if it's a primitive type
		if typeCode != "" && walker.IsPrimitiveType(typeCode) {
			if err := p.validatePrimitive(typeCode, wctx.Node); err != nil {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeValue,
					err.Error(),
					wctx.Path,
					p.Name(),
				))
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		issues = append(issues, WarningIssue(
			fv.IssueTypeProcessing,
			fmt.Sprintf("Error during primitives validation: %v", err),
			pctx.ResourceType,
			p.Name(),
		))
	}

	return issues
}

// validatePrimitive validates a value against a primitive type.
func (p *PrimitivesPhase) validatePrimitive(typeCode string, value any) error {
	// Null is valid for optional elements
	if value == nil {
		return nil
	}

	switch typeCode {
	case TypeBoolean:
		return validateBoolean(value)
	case "integer":
		return validateInteger(value)
	case "integer64":
		return validateInteger64(value)
	case "unsignedInt":
		return validateUnsignedInt(value)
	case "positiveInt":
		return validatePositiveInt(value)
	case "decimal":
		return validateDecimal(value)
	case TypeString:
		return validateString(value)
	case "uri":
		return validateURI(value)
	case "url":
		return validateURL(value)
	case "canonical":
		return validateCanonical(value)
	case "code":
		return validateCode(value)
	case "id":
		return validateID(value)
	case "oid":
		return validateOID(value)
	case "uuid":
		return validateUUID(value)
	case "markdown":
		return validateMarkdown(value)
	case "base64Binary":
		return validateBase64Binary(value)
	case TypeInstant:
		return validateInstant(value)
	case "date":
		return validateDate(value)
	case TypeDateTime:
		return validateDateTime(value)
	case "time":
		return validateTime(value)
	case "xhtml":
		return validateXHTML(value)
	default:
		return nil
	}
}

// validateBoolean validates a boolean value.
func validateBoolean(value any) error {
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("value must be a boolean, got %T", value)
	}
	return nil
}

// validateInteger validates a FHIR integer (-2147483648 to 2147483647).
func validateInteger(value any) error {
	f, ok := value.(float64)
	if !ok {
		return fmt.Errorf("value must be a number, got %T", value)
	}
	if f != float64(int32(f)) {
		return fmt.Errorf("value must be a 32-bit integer, got %v", f)
	}
	i := int64(f)
	if i < -2147483648 || i > 2147483647 {
		return fmt.Errorf("integer out of range [-2147483648, 2147483647]: %v", i)
	}
	return nil
}

// validateInteger64 validates a FHIR integer64.
func validateInteger64(value any) error {
	switch v := value.(type) {
	case float64:
		if v != float64(int64(v)) {
			return fmt.Errorf("value must be an integer, got %v", v)
		}
	case string:
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return fmt.Errorf("invalid integer64 string: %v", v)
		}
	default:
		return fmt.Errorf("integer64 must be a number or string, got %T", value)
	}
	return nil
}

// validateUnsignedInt validates a FHIR unsignedInt (0 to 2147483647).
func validateUnsignedInt(value any) error {
	f, ok := value.(float64)
	if !ok {
		return fmt.Errorf("value must be a number, got %T", value)
	}
	if f != float64(int32(f)) || f < 0 {
		return fmt.Errorf("value must be a non-negative 32-bit integer, got %v", f)
	}
	i := int64(f)
	if i < 0 || i > 2147483647 {
		return fmt.Errorf("unsignedInt out of range [0, 2147483647]: %v", i)
	}
	return nil
}

// validatePositiveInt validates a FHIR positiveInt (1 to 2147483647).
func validatePositiveInt(value any) error {
	f, ok := value.(float64)
	if !ok {
		return fmt.Errorf("value must be a number, got %T", value)
	}
	if f != float64(int32(f)) || f < 1 {
		return fmt.Errorf("value must be a positive 32-bit integer, got %v", f)
	}
	i := int64(f)
	if i < 1 || i > 2147483647 {
		return fmt.Errorf("positiveInt out of range [1, 2147483647]: %v", i)
	}
	return nil
}

// validateDecimal validates a FHIR decimal.
func validateDecimal(value any) error {
	switch v := value.(type) {
	case float64:
		return nil
	case string:
		if !decimalRegex.MatchString(v) {
			return fmt.Errorf("invalid decimal format: %s", v)
		}
	default:
		return fmt.Errorf("decimal must be a number or string, got %T", value)
	}
	return nil
}

// validateString validates a FHIR string.
func validateString(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("string contains invalid UTF-8")
	}
	// FHIR strings cannot have leading/trailing whitespace
	if s != "" && (s[0] == ' ' || s[0] == '\t' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		return fmt.Errorf("string cannot have leading or trailing whitespace")
	}
	return nil
}

// validateURI validates a FHIR uri.
func validateURI(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if s == "" {
		return fmt.Errorf("uri cannot be empty")
	}
	// Basic URI validation - must not contain spaces
	if strings.ContainsAny(s, " \t\n\r") {
		return fmt.Errorf("uri cannot contain whitespace")
	}
	return nil
}

// validateURL validates a FHIR url.
func validateURL(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !urlRegex.MatchString(s) {
		return fmt.Errorf("invalid url format: %s", s)
	}
	return nil
}

// validateCanonical validates a FHIR canonical URL.
func validateCanonical(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !canonicalRegex.MatchString(s) {
		return fmt.Errorf("invalid canonical format: %s", s)
	}
	return nil
}

// validateCode validates a FHIR code.
func validateCode(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !codeRegex.MatchString(s) {
		return fmt.Errorf("invalid code format: %s", s)
	}
	return nil
}

// validateID validates a FHIR id.
func validateID(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !idRegex.MatchString(s) {
		return fmt.Errorf("invalid id format: %s (must match [A-Za-z0-9\\-\\.]{1,64})", s)
	}
	return nil
}

// validateOID validates a FHIR oid.
func validateOID(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !oidRegex.MatchString(s) {
		return fmt.Errorf("invalid oid format: %s (must be urn:oid:...)", s)
	}
	return nil
}

// validateUUID validates a FHIR uuid.
func validateUUID(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !uuidRegex.MatchString(s) {
		return fmt.Errorf("invalid uuid format: %s (must be urn:uuid:...)", s)
	}
	return nil
}

// validateMarkdown validates a FHIR markdown.
func validateMarkdown(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("markdown contains invalid UTF-8")
	}
	return nil
}

// validateBase64Binary validates a FHIR base64Binary.
func validateBase64Binary(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if _, err := base64.StdEncoding.DecodeString(s); err != nil {
		return fmt.Errorf("invalid base64 encoding: %v", err)
	}
	return nil
}

// validateInstant validates a FHIR instant.
func validateInstant(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !instantRegex.MatchString(s) {
		return fmt.Errorf("invalid instant format: %s", s)
	}
	return nil
}

// validateDate validates a FHIR date.
func validateDate(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !dateRegex.MatchString(s) {
		return fmt.Errorf("invalid date format: %s (expected YYYY, YYYY-MM, or YYYY-MM-DD)", s)
	}
	return nil
}

// validateDateTime validates a FHIR dateTime.
func validateDateTime(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !dateTimeRegex.MatchString(s) {
		return fmt.Errorf("invalid dateTime format: %s", s)
	}
	return nil
}

// validateTime validates a FHIR time.
func validateTime(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	if !timeRegex.MatchString(s) {
		return fmt.Errorf("invalid time format: %s (expected HH:MM:SS or HH:MM:SS.sss)", s)
	}
	return nil
}

// validateXHTML validates a FHIR xhtml.
func validateXHTML(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("value must be a string, got %T", value)
	}
	// Basic check - must start with div element
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "<div") {
		return fmt.Errorf("xhtml must start with <div> element")
	}
	return nil
}

// inferTypeFromFieldName infers the FHIR type from common field names.
func inferTypeFromFieldName(field string) string {
	// Extract the last segment if path contains dots
	if idx := strings.LastIndex(field, "."); idx != -1 {
		field = field[idx+1:]
	}

	// Remove array index if present
	if idx := strings.Index(field, "["); idx != -1 {
		field = field[:idx]
	}

	// Common field name to type mappings
	switch field {
	case "id":
		return "id"
	case "url", "implicitRules":
		return "uri"
	case "version", "name", "title", "description", "comment", "text":
		return TypeString
	case "status", "code", "language", "gender", "use":
		return "code"
	case "active", "experimental":
		return "boolean"
	case "date", "birthDate", "deceasedDate":
		return "date"
	case "instant", "issued", "recorded":
		return "instant"
	case "dateTime", "effectiveDateTime", "onsetDateTime":
		return "dateTime"
	case "time":
		return "time"
	default:
		return ""
	}
}

// Compiled regex patterns for validation
var (
	decimalRegex   = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?$`)
	urlRegex       = regexp.MustCompile(`^\S+$`)
	canonicalRegex = regexp.MustCompile(`^\S+(\|\S+)?$`)
	codeRegex      = regexp.MustCompile(`^\S+(\s\S+)*$`)
	idRegex        = regexp.MustCompile(`^[A-Za-z0-9\-.]{1,64}$`)
	oidRegex       = regexp.MustCompile(`^urn:oid:[012](\.(0|[1-9]\d*))+$`)
	uuidRegex      = regexp.MustCompile(`^urn:uuid:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	instantRegex   = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[012])-(0[1-9]|[12]\d|3[01])T([01]\d|2[0-3]):[0-5]\d:([0-5]\d|60)(\.\d+)?(Z|[+-]((0\d|1[0-3]):[0-5]\d|14:00))$`)
	dateRegex      = regexp.MustCompile(`^(\d{4})(-(0[1-9]|1[012])(-(0[1-9]|[12]\d|3[01]))?)?$`)
	dateTimeRegex  = regexp.MustCompile(`^(\d{4})(-(0[1-9]|1[012])(-(0[1-9]|[12]\d|3[01])(T([01]\d|2[0-3]):[0-5]\d:([0-5]\d|60)(\.\d+)?(Z|[+-]((0\d|1[0-3]):[0-5]\d|14:00))?)?)?)?$`)
	timeRegex      = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d:([0-5]\d|60)(\.\d+)?$`)
)

// PrimitivesPhaseConfig returns the standard configuration for the primitives phase.
func PrimitivesPhaseConfig(profileService service.ProfileResolver) *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewPrimitivesPhase(profileService),
		Priority: pipeline.PriorityEarly,
		Parallel: true,
		Required: true,
		Enabled:  true,
	}
}
