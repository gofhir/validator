package phase

import (
	"context"
	"testing"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/service"
)

// mockCodeValidator implements service.CodeValidator for testing
type mockCodeValidator struct {
	validCodes map[string]map[string]*service.ValidateCodeResult // system -> code -> result
}

func (m *mockCodeValidator) ValidateCode(ctx context.Context, system, code, valueSetURL string) (*service.ValidateCodeResult, error) {
	if m.validCodes == nil {
		return &service.ValidateCodeResult{Valid: false}, nil
	}

	// Check by system
	if systemCodes, ok := m.validCodes[system]; ok {
		if result, ok := systemCodes[code]; ok {
			return result, nil
		}
	}

	// Check by ValueSet URL
	if vsCodes, ok := m.validCodes[valueSetURL]; ok {
		if result, ok := vsCodes[code]; ok {
			return result, nil
		}
	}

	return &service.ValidateCodeResult{Valid: false}, nil
}

func (m *mockCodeValidator) ExpandValueSet(ctx context.Context, url string) (*service.ValueSetExpansion, error) {
	return nil, nil
}

func TestCodingValidationHelper_ValidateCoding_Valid(t *testing.T) {
	mock := &mockCodeValidator{
		validCodes: map[string]map[string]*service.ValidateCodeResult{
			"http://example.org/CodeSystem/test": {
				"code1": {Valid: true, Display: "Code One"},
			},
		},
	}

	helper := NewCodingValidationHelper(mock)
	ctx := context.Background()

	coding := map[string]any{
		"system":  "http://example.org/CodeSystem/test",
		"code":    "code1",
		"display": "Code One",
	}

	opts := CodingValidationOptions{
		ValueSet:         "http://example.org/ValueSet/test",
		BindingStrength:  "required",
		ValidateDisplay:  true,
		DisplayAsWarning: true,
		Phase:            "test",
	}

	result := helper.ValidateCoding(ctx, coding, "Test.element", opts)

	if !result.Valid {
		t.Errorf("Expected valid result, got invalid")
	}

	if len(result.Issues) > 0 {
		t.Errorf("Expected no issues, got %d: %v", len(result.Issues), result.Issues)
	}
}

func TestCodingValidationHelper_ValidateCoding_DisplayMismatch(t *testing.T) {
	mock := &mockCodeValidator{
		validCodes: map[string]map[string]*service.ValidateCodeResult{
			"http://example.org/CodeSystem/test": {
				"code1": {Valid: true, Display: "Code One"},
			},
		},
	}

	helper := NewCodingValidationHelper(mock)
	ctx := context.Background()

	coding := map[string]any{
		"system":  "http://example.org/CodeSystem/test",
		"code":    "code1",
		"display": "Wrong Display",
	}

	opts := CodingValidationOptions{
		ValueSet:         "http://example.org/ValueSet/test",
		BindingStrength:  "required",
		ValidateDisplay:  true,
		DisplayAsWarning: true,
		Phase:            "test",
	}

	result := helper.ValidateCoding(ctx, coding, "Test.element", opts)

	if !result.Valid {
		t.Errorf("Expected valid result (display mismatch is warning)")
	}

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue (display mismatch), got %d", len(result.Issues))
		return
	}

	if result.Issues[0].Severity != fv.SeverityWarning {
		t.Errorf("Expected warning severity for display mismatch, got %v", result.Issues[0].Severity)
	}
}

func TestCodingValidationHelper_ValidateCoding_InvalidCode(t *testing.T) {
	mock := &mockCodeValidator{
		validCodes: map[string]map[string]*service.ValidateCodeResult{},
	}

	helper := NewCodingValidationHelper(mock)
	ctx := context.Background()

	coding := map[string]any{
		"system": "http://example.org/CodeSystem/test",
		"code":   "invalid_code",
	}

	opts := CodingValidationOptions{
		ValueSet:        "http://example.org/ValueSet/test",
		BindingStrength: "required",
		Phase:           "test",
	}

	result := helper.ValidateCoding(ctx, coding, "Test.element", opts)

	if result.Valid {
		t.Errorf("Expected invalid result for non-existent code")
	}

	if len(result.Issues) == 0 {
		t.Errorf("Expected issues for invalid code")
	}
}

func TestCodingValidationHelper_ValidateCodeableConcept_OneValid(t *testing.T) {
	mock := &mockCodeValidator{
		validCodes: map[string]map[string]*service.ValidateCodeResult{
			"http://example.org/CodeSystem/test": {
				"valid_code": {Valid: true, Display: "Valid Code"},
			},
		},
	}

	helper := NewCodingValidationHelper(mock)
	ctx := context.Background()

	cc := map[string]any{
		"coding": []any{
			map[string]any{
				"system": "http://other.org/CodeSystem",
				"code":   "invalid_code",
			},
			map[string]any{
				"system":  "http://example.org/CodeSystem/test",
				"code":    "valid_code",
				"display": "Valid Code",
			},
		},
	}

	opts := CodingValidationOptions{
		ValueSet:         "http://example.org/ValueSet/test",
		BindingStrength:  "required",
		ValidateDisplay:  true,
		DisplayAsWarning: true,
		Phase:            "test",
	}

	result := helper.ValidateCodeableConcept(ctx, cc, "Test.element", opts)

	if !result.Valid {
		t.Errorf("Expected valid result when at least one coding is valid")
	}
}

func TestCodingValidationHelper_ValidateCodeableConcept_NoneValid(t *testing.T) {
	mock := &mockCodeValidator{
		validCodes: map[string]map[string]*service.ValidateCodeResult{},
	}

	helper := NewCodingValidationHelper(mock)
	ctx := context.Background()

	cc := map[string]any{
		"coding": []any{
			map[string]any{
				"system": "http://example.org/CodeSystem/test",
				"code":   "invalid_code1",
			},
			map[string]any{
				"system": "http://example.org/CodeSystem/test",
				"code":   "invalid_code2",
			},
		},
	}

	opts := CodingValidationOptions{
		ValueSet:        "http://example.org/ValueSet/test",
		BindingStrength: "required",
		Phase:           "test",
	}

	result := helper.ValidateCodeableConcept(ctx, cc, "Test.element", opts)

	if result.Valid {
		t.Errorf("Expected invalid result when no coding is valid")
	}

	// Check for error issue
	hasError := false
	for _, issue := range result.Issues {
		if issue.Severity == fv.SeverityError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Errorf("Expected error issue for required binding with no valid codes")
	}
}

func TestCodingValidationHelper_ValidateCodeableConcept_DisplayMismatchReported(t *testing.T) {
	mock := &mockCodeValidator{
		validCodes: map[string]map[string]*service.ValidateCodeResult{
			"http://example.org/CodeSystem/test": {
				"code1": {Valid: true, Display: "Correct Display"},
			},
		},
	}

	helper := NewCodingValidationHelper(mock)
	ctx := context.Background()

	cc := map[string]any{
		"coding": []any{
			map[string]any{
				"system":  "http://example.org/CodeSystem/test",
				"code":    "code1",
				"display": "Wrong Display",
			},
		},
	}

	opts := CodingValidationOptions{
		ValueSet:         "http://example.org/ValueSet/test",
		BindingStrength:  "required",
		ValidateDisplay:  true,
		DisplayAsWarning: true,
		Phase:            "test",
	}

	result := helper.ValidateCodeableConcept(ctx, cc, "Test.element", opts)

	// Valid because code is valid, but should have display mismatch warning
	if !result.Valid {
		t.Errorf("Expected valid result (code is valid)")
	}

	// Check for warning issue
	hasWarning := false
	for _, issue := range result.Issues {
		if issue.Severity == fv.SeverityWarning {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("Expected warning issue for display mismatch")
	}
}

func TestCodingValidationHelper_BindingStrengthSeverity(t *testing.T) {
	tests := []struct {
		strength         string
		expectedSeverity fv.IssueSeverity
	}{
		{"required", fv.SeverityError},
		{"extensible", fv.SeverityWarning},
		{"preferred", fv.SeverityInformation},
		{"example", fv.SeverityInformation},
	}

	helper := NewCodingValidationHelper(nil)

	for _, tt := range tests {
		t.Run(tt.strength, func(t *testing.T) {
			severity := helper.getSeverityForStrength(tt.strength)
			if severity != tt.expectedSeverity {
				t.Errorf("For strength %q, expected severity %v, got %v",
					tt.strength, tt.expectedSeverity, severity)
			}
		})
	}
}

func TestCodingValidationHelper_NilTerminologyService(t *testing.T) {
	helper := NewCodingValidationHelper(nil)
	ctx := context.Background()

	coding := map[string]any{
		"system": "http://example.org/CodeSystem/test",
		"code":   "code1",
	}

	opts := DefaultCodingValidationOptions("test")
	opts.ValueSet = "http://example.org/ValueSet/test"

	result := helper.ValidateCoding(ctx, coding, "Test.element", opts)

	// Should return valid with no issues when no terminology service
	if !result.Valid {
		t.Errorf("Expected valid result with nil terminology service")
	}
}

func TestDefaultCodingValidationOptions(t *testing.T) {
	opts := DefaultCodingValidationOptions("test-phase")

	if opts.Phase != "test-phase" {
		t.Errorf("Expected phase 'test-phase', got %q", opts.Phase)
	}

	if !opts.ValidateDisplay {
		t.Errorf("Expected ValidateDisplay to be true by default")
	}

	if !opts.DisplayAsWarning {
		t.Errorf("Expected DisplayAsWarning to be true by default")
	}

	if opts.ValueSet != "" {
		t.Errorf("Expected empty ValueSet by default")
	}
}
