package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
)

func TestValidateWithModeCreate(t *testing.T) {
	v := getSharedValidator(t)

	// In create mode, id is not required (server assigns it)
	resource := []byte(`{"resourceType": "Patient", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeCreate))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should not have errors about missing id
	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagModeUpdateRequiresID) || iss.MessageID == string(issue.DiagModeDeleteRequiresID) {
			t.Errorf("Unexpected mode error in create mode: %s", iss.Diagnostics)
		}
	}

	t.Logf("Create mode: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidateWithModeCreateWithID(t *testing.T) {
	v := getSharedValidator(t)

	// In create mode, id is allowed but not required
	resource := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeCreate))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("Create mode with id: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidateWithModeUpdateRequiresID(t *testing.T) {
	v := getSharedValidator(t)

	// In update mode, id must be present
	resource := []byte(`{"resourceType": "Patient", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeUpdate))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	found := false
	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagModeUpdateRequiresID) {
			found = true
			if iss.Severity != issue.SeverityError {
				t.Errorf("Expected error severity, got %s", iss.Severity)
			}
			if len(iss.Expression) == 0 || iss.Expression[0] != "Patient.id" {
				t.Errorf("Expected expression [Patient.id], got %v", iss.Expression)
			}
			break
		}
	}
	if !found {
		t.Error("Expected MODE_UPDATE_REQUIRES_ID error not found")
	}
}

func TestValidateWithModeUpdateWithID(t *testing.T) {
	v := getSharedValidator(t)

	// In update mode with id present, no mode-specific error
	resource := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeUpdate))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagModeUpdateRequiresID) {
			t.Errorf("Unexpected MODE_UPDATE_REQUIRES_ID error when id is present: %s", iss.Diagnostics)
		}
	}

	t.Logf("Update mode with id: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidateWithModeDeleteRequiresID(t *testing.T) {
	v := getSharedValidator(t)

	// In delete mode, id must be present
	resource := []byte(`{"resourceType": "Patient"}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeDelete))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	found := false
	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagModeDeleteRequiresID) {
			found = true
			if iss.Severity != issue.SeverityError {
				t.Errorf("Expected error severity, got %s", iss.Severity)
			}
			if len(iss.Expression) == 0 || iss.Expression[0] != "Patient.id" {
				t.Errorf("Expected expression [Patient.id], got %v", iss.Expression)
			}
			break
		}
	}
	if !found {
		t.Error("Expected MODE_DELETE_REQUIRES_ID error not found")
	}
}

func TestValidateWithModeDeleteWithID(t *testing.T) {
	v := getSharedValidator(t)

	// In delete mode with id present, no error
	resource := []byte(`{"resourceType": "Patient", "id": "123"}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeDelete))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagModeDeleteRequiresID) {
			t.Errorf("Unexpected MODE_DELETE_REQUIRES_ID error when id is present: %s", iss.Diagnostics)
		}
	}

	// Delete mode should return early with minimal validation — no other phase errors expected
	if result.ErrorCount() > 0 {
		t.Errorf("Delete mode with valid id should have no errors, got %d", result.ErrorCount())
		for _, iss := range result.Issues {
			if iss.Severity == issue.SeverityError {
				t.Logf("  Error: %s @ %v", iss.Diagnostics, iss.Expression)
			}
		}
	}
}

func TestValidateWithModeDeleteSkipsFullValidation(t *testing.T) {
	v := getSharedValidator(t)

	// In delete mode, even invalid data should only be checked for resourceType + id
	resource := []byte(`{"resourceType": "Patient", "id": "123", "name": "invalid-not-array"}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeDelete))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Delete mode returns early — structural errors on name should not appear
	if result.ErrorCount() > 0 {
		t.Errorf("Delete mode should skip full validation, got %d errors", result.ErrorCount())
		for _, iss := range result.Issues {
			if iss.Severity == issue.SeverityError {
				t.Logf("  Error: %s @ %v", iss.Diagnostics, iss.Expression)
			}
		}
	}
}

func TestValidateWithModeInvalid(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{"resourceType": "Patient"}`)
	_, err := v.Validate(context.Background(), resource, ValidateWithMode("invalid"))
	if err == nil {
		t.Error("Expected error for invalid mode, got nil")
	}
}

func TestValidateWithModeNone(t *testing.T) {
	v := getSharedValidator(t)

	// Default mode (none) should work like normal validation
	resource := []byte(`{"resourceType": "Patient", "active": true}`)
	result, err := v.Validate(context.Background(), resource, ValidateWithMode(ValidateModeNone))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("Default mode: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}

func TestValidateWithModeCombinedWithProfile(t *testing.T) {
	v := getSharedValidator(t)

	// Mode can be combined with other ValidateOptions
	resource := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
	result, err := v.Validate(context.Background(), resource,
		ValidateWithMode(ValidateModeUpdate),
		ValidateWithProfile("http://hl7.org/fhir/StructureDefinition/Patient"),
	)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	t.Logf("Update mode with profile: Errors: %d, Warnings: %d", result.ErrorCount(), result.WarningCount())
}
