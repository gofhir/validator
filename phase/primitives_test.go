package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
)

func TestPrimitivesPhase_Name(t *testing.T) {
	phase := NewPrimitivesPhase(nil)
	if phase.Name() != "primitives" {
		t.Errorf("Name() = %q; want %q", phase.Name(), "primitives")
	}
}

func TestValidateBoolean(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid true", true, false},
		{"valid false", false, false},
		{"invalid string", "true", true},
		{"invalid number", 1.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBoolean(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBoolean(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInteger(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid zero", float64(0), false},
		{"valid positive", float64(42), false},
		{"valid negative", float64(-42), false},
		{"valid max", float64(2147483647), false},
		{"valid min", float64(-2147483648), false},
		{"invalid over max", float64(2147483648), true},
		{"invalid under min", float64(-2147483649), true},
		{"invalid decimal", 42.5, true},
		{"invalid string", "42", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInteger(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInteger(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUnsignedInt(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid zero", float64(0), false},
		{"valid positive", float64(42), false},
		{"invalid negative", float64(-1), true},
		{"invalid string", "42", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnsignedInt(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUnsignedInt(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid one", float64(1), false},
		{"valid positive", float64(42), false},
		{"invalid zero", float64(0), true},
		{"invalid negative", float64(-1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositiveInt(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePositiveInt(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDecimal(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid float", 42.5, false},
		{"valid int as float", float64(42), false},
		{"valid string decimal", "42.5", false},
		{"valid string negative", "-42.5", false},
		{"valid scientific", "1.5e10", false},
		{"invalid string", "not a number", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecimal(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDecimal(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateString(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid string", "hello", false},
		{"valid empty", "", false},
		{"valid unicode", "日本語", false},
		{"invalid leading space", " hello", true},
		{"invalid trailing space", "hello ", true},
		{"invalid not string", 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateString(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateString(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURI(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid urn", "urn:oid:1.2.3.4", false},
		{"valid relative", "Patient/123", false},
		{"invalid empty", "", true},
		{"invalid with space", "http://example.com/with space", true},
		{"invalid not string", 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURI(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURI(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCode(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid simple", "active", false},
		{"valid with space", "final report", false},
		{"valid hyphen", "in-progress", false},
		{"invalid empty", "", true},
		{"invalid leading space", " active", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCode(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid simple", "patient-123", false},
		{"valid uuid-like", "a1b2c3d4-e5f6-7890-abcd-ef1234567890", false},
		{"valid with dot", "example.id", false},
		{"invalid too long", "this-id-is-way-too-long-and-exceeds-the-maximum-length-of-64-characters-allowed", true},
		{"invalid chars", "invalid@id", true},
		{"invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateID(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateID(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateOID(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid oid", "urn:oid:1.2.3.4.5", false},
		{"valid oid zero", "urn:oid:0.1.2", false},
		{"invalid no prefix", "1.2.3.4.5", true},
		{"invalid wrong prefix", "oid:1.2.3.4", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOID(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOID(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid uuid", "urn:uuid:a1b2c3d4-e5f6-7890-abcd-ef1234567890", false},
		{"valid uuid upper", "urn:uuid:A1B2C3D4-E5F6-7890-ABCD-EF1234567890", false},
		{"invalid no prefix", "a1b2c3d4-e5f6-7890-abcd-ef1234567890", true},
		{"invalid wrong format", "urn:uuid:not-a-uuid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUUID(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUUID(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBase64Binary(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid base64", "SGVsbG8gV29ybGQ=", false},
		{"valid empty", "", false},
		{"invalid base64", "not valid base64!!!", true},
		{"invalid not string", 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBase64Binary(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBase64Binary(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDate(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid full date", "2024-01-15", false},
		{"valid year-month", "2024-01", false},
		{"valid year only", "2024", false},
		{"invalid format", "01/15/2024", true},
		{"invalid month", "2024-13-01", true},
		{"invalid day", "2024-01-32", true},
		{"invalid not string", 20240115, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDate(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDateTime(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid date only", "2024-01-15", false},
		{"valid with time", "2024-01-15T10:30:00Z", false},
		{"valid with offset", "2024-01-15T10:30:00+05:00", false},
		{"valid with millis", "2024-01-15T10:30:00.123Z", false},
		{"valid year only", "2024", false},
		{"invalid format", "2024/01/15", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDateTime(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDateTime(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTime(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid time", "10:30:00", false},
		{"valid with millis", "10:30:00.123", false},
		{"valid midnight", "00:00:00", false},
		{"invalid format", "10:30", true},
		{"invalid hour", "25:00:00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTime(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTime(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInstant(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid instant UTC", "2024-01-15T10:30:00Z", false},
		{"valid instant offset", "2024-01-15T10:30:00+05:00", false},
		{"valid with millis", "2024-01-15T10:30:00.123Z", false},
		{"invalid no timezone", "2024-01-15T10:30:00", true},
		{"invalid date only", "2024-01-15", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstant(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInstant(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateXHTML(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid xhtml", "<div xmlns=\"http://www.w3.org/1999/xhtml\">Hello</div>", false},
		{"valid div only", "<div>Hello</div>", false},
		{"invalid no div", "<p>Hello</p>", true},
		{"invalid not string", 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateXHTML(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateXHTML(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestPrimitivesPhase_Validate(t *testing.T) {
	phase := NewPrimitivesPhase(nil)

	pctx := pipeline.NewContext()
	pctx.ResourceType = "Patient"
	pctx.ResourceMap = map[string]any{
		"resourceType": "Patient",
		"id":           "invalid@id", // Invalid ID
		"active":       true,
		"birthDate":    "invalid-date", // Invalid date
	}

	issues := phase.Validate(context.Background(), pctx)

	// Should have issues for invalid id and birthDate
	if len(issues) < 2 {
		t.Errorf("Expected at least 2 issues, got %d", len(issues))
	}

	hasIDIssue := false
	hasDateIssue := false
	for _, issue := range issues {
		if len(issue.Expression) > 0 {
			expr := issue.Expression[0]
			// Check for both old and new path formats
			if expr == "id" || expr == "Patient.id" {
				hasIDIssue = true
			}
			if expr == "birthDate" || expr == "Patient.birthDate" {
				hasDateIssue = true
			}
		}
	}

	if !hasIDIssue {
		t.Error("Expected issue for invalid id")
	}
	if !hasDateIssue {
		t.Error("Expected issue for invalid birthDate")
	}
}

func TestPrimitivesPhase_ValidResource(t *testing.T) {
	phase := NewPrimitivesPhase(nil)

	pctx := pipeline.NewContext()
	pctx.ResourceType = "Patient"
	pctx.ResourceMap = map[string]any{
		"resourceType": "Patient",
		"id":           "patient-123",
		"active":       true,
		"birthDate":    "1990-01-15",
	}

	issues := phase.Validate(context.Background(), pctx)

	if len(issues) > 0 {
		t.Errorf("Expected no issues for valid resource, got %d: %v", len(issues), issues)
	}
}

func TestInferTypeFromFieldName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"id", "id"},
		{"url", "uri"},
		{"birthDate", "date"},
		{"active", "boolean"},
		{"status", "code"},
		{"Patient.id", "id"},
		{"Patient.active", "boolean"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := inferTypeFromFieldName(tt.path)
			if got != tt.expected {
				t.Errorf("inferTypeFromFieldName(%q) = %q; want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func BenchmarkPrimitivesPhase_Validate(b *testing.B) {
	phase := NewPrimitivesPhase(nil)

	pctx := pipeline.NewContext()
	pctx.ResourceType = "Patient"
	pctx.ResourceMap = map[string]any{
		"resourceType": "Patient",
		"id":           "patient-123",
		"active":       true,
		"birthDate":    "1990-01-15",
		"gender":       "male",
		"name": []any{
			map[string]any{
				"family": "Smith",
				"given":  []any{"John"},
			},
		},
	}
	pctx.Result = fv.NewResult()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		phase.Validate(context.Background(), pctx)
	}
}
