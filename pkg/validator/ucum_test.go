package validator

import (
	"context"
	"testing"

	"github.com/gofhir/validator/pkg/issue"
)

func TestUCUMValidQuantityCode(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]},
		"valueQuantity": {
			"value": 70.5,
			"unit": "kg",
			"system": "http://unitsofmeasure.org",
			"code": "kg"
		}
	}`)
	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagUCUMInvalidCode) {
			t.Errorf("Unexpected UCUM error for valid code 'kg': %s", iss.Diagnostics)
		}
	}
}

func TestUCUMInvalidQuantityCode(t *testing.T) {
	v := getSharedValidator(t)

	resource := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]},
		"valueQuantity": {
			"value": 70.5,
			"unit": "invalidunit!!!",
			"system": "http://unitsofmeasure.org",
			"code": "invalidunit!!!"
		}
	}`)
	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, iss := range result.Issues {
		if iss.MessageID != string(issue.DiagUCUMInvalidCode) {
			continue
		}
		found = true
		// An error rather than a warning: the instance names UCUM as its system and the code
		// does not exist there, which is the same rule as a code absent from any other
		// CodeSystem the instance declares. The reference reports it as an error too.
		if iss.Severity != issue.SeverityError {
			t.Errorf("Expected error severity, got %s", iss.Severity)
		}
		if len(iss.Expression) == 0 {
			t.Error("Expected expression path")
		}
		t.Logf("UCUM error: %s @ %v", iss.Diagnostics, iss.Expression)
		break
	}
	if !found {
		t.Error("Expected UCUM_INVALID_CODE warning for invalid UCUM code")
	}
}

func TestUCUMNonUCUMSystemSkipped(t *testing.T) {
	v := getSharedValidator(t)

	// Quantity with non-UCUM system should not trigger UCUM validation.
	resource := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]},
		"valueQuantity": {
			"value": 70.5,
			"unit": "somecustomunit",
			"system": "http://example.org/units",
			"code": "somecustomunit"
		}
	}`)
	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagUCUMInvalidCode) {
			t.Errorf("Should not validate UCUM for non-UCUM system: %s", iss.Diagnostics)
		}
	}
}

func TestUCUMCommonClinicalCodes(t *testing.T) {
	v := getSharedValidator(t)

	codes := []string{
		"kg", "g", "mg", "ug",
		"L", "mL", "dL", "uL",
		"g/dL", "mg/dL", "mmol/L",
		"10*3/uL", "10*6/uL",
		"s", "min", "h", "d",
		"mm[Hg]", "cm[H2O]",
		"mg/kg", "mg/m2",
		"%", "/uL",
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			resource := []byte(`{
				"resourceType": "Observation",
				"status": "final",
				"code": {"coding": [{"system": "http://loinc.org", "code": "test"}]},
				"valueQuantity": {
					"value": 1,
					"system": "http://unitsofmeasure.org",
					"code": "` + code + `"
				}
			}`)
			result, err := v.Validate(context.Background(), resource)
			if err != nil {
				t.Fatal(err)
			}
			for _, iss := range result.Issues {
				if iss.MessageID == string(issue.DiagUCUMInvalidCode) {
					t.Errorf("Valid UCUM code %q flagged as invalid: %s", code, iss.Diagnostics)
				}
			}
		})
	}
}

func TestUCUMNoCodeNoWarning(t *testing.T) {
	v := getSharedValidator(t)

	// Quantity with system but no code should not trigger UCUM validation.
	resource := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]},
		"valueQuantity": {
			"value": 70.5,
			"system": "http://unitsofmeasure.org"
		}
	}`)
	result, err := v.Validate(context.Background(), resource)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagUCUMInvalidCode) {
			t.Errorf("Should not validate UCUM when code is absent: %s", iss.Diagnostics)
		}
	}
}
