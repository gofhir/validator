package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HL7ResourceType defines a resource type to test with its expected results.
type HL7ResourceType struct {
	Name string
	// Skip indicates the resource type should be skipped (e.g., known issues)
	Skip bool
	// SkipReason explains why the resource is skipped
	SkipReason string
}

// hl7ResourceTypes lists all resource types to test against HL7 examples.
// Add new resource types here to include them in testing.
var hl7ResourceTypes = []HL7ResourceType{
	// === Clinical Resources ===
	{Name: "Patient"},
	{Name: "Observation"},
	{Name: "Condition"},
	{Name: "Procedure"},
	{Name: "Encounter"},
	{Name: "EpisodeOfCare"},
	{Name: "DiagnosticReport"},
	{Name: "AllergyIntolerance"},
	{Name: "AdverseEvent"},
	{Name: "ClinicalImpression"},
	{Name: "DetectedIssue"},
	{Name: "FamilyMemberHistory"},
	{Name: "RiskAssessment"},

	// === Care Provision ===
	{Name: "CarePlan"},
	{Name: "CareTeam"},
	{Name: "Goal"},
	{Name: "NutritionOrder"},
	{Name: "RequestGroup"},
	{Name: "VisionPrescription"},

	// === Medications ===
	{Name: "Medication"},
	{Name: "MedicationRequest"},
	{Name: "MedicationDispense"},
	{Name: "MedicationAdministration"},
	{Name: "MedicationStatement"},
	{Name: "MedicationKnowledge"},
	{Name: "Immunization"},
	{Name: "ImmunizationEvaluation"},
	{Name: "ImmunizationRecommendation"},

	// === Diagnostics ===
	{Name: "Specimen"},
	{Name: "SpecimenDefinition"},
	{Name: "BodyStructure"},
	{Name: "ImagingStudy"},
	{Name: "Media"},
	{Name: "MolecularSequence"},
	{Name: "ObservationDefinition"},

	// === Administrative ===
	{Name: "Practitioner"},
	{Name: "PractitionerRole"},
	{Name: "Organization"},
	{Name: "OrganizationAffiliation"},
	{Name: "Location"},
	{Name: "HealthcareService"},
	{Name: "Endpoint"},
	{Name: "RelatedPerson"},
	{Name: "Person"},
	{Name: "Group"},
	{Name: "Linkage"},

	// === Scheduling ===
	{Name: "Appointment"},
	{Name: "AppointmentResponse"},
	{Name: "Schedule"},
	{Name: "Slot"},

	// === Financial ===
	{Name: "Coverage"},
	{Name: "CoverageEligibilityRequest"},
	{Name: "CoverageEligibilityResponse"},
	{Name: "Claim"},
	{Name: "ClaimResponse"},
	{Name: "ExplanationOfBenefit"},
	{Name: "PaymentNotice"},
	{Name: "PaymentReconciliation"},
	{Name: "Invoice"},
	{Name: "ChargeItem"},
	{Name: "ChargeItemDefinition"},
	{Name: "InsurancePlan"},
	{Name: "Account"},
	{Name: "Contract"},
	{Name: "EnrollmentRequest"},
	{Name: "EnrollmentResponse"},

	// === Workflow ===
	{Name: "Task"},
	{Name: "ServiceRequest"},
	{Name: "Communication"},
	{Name: "CommunicationRequest"},
	{Name: "DeviceRequest"},
	{Name: "DeviceUseStatement"},
	{Name: "SupplyRequest"},
	{Name: "SupplyDelivery"},
	{Name: "GuidanceResponse"},

	// === Documents & Reports ===
	{Name: "DocumentReference"},
	{Name: "DocumentManifest"},
	{Name: "Composition"},
	{Name: "Bundle"},
	{Name: "List"},
	{Name: "Basic"},
	{Name: "Binary"},

	// === Security & Consent ===
	{Name: "AuditEvent"},
	{Name: "Consent"},
	{Name: "Flag"},
	{Name: "Provenance"},
	{Name: "VerificationResult"},

	// === Clinical Reasoning ===
	{Name: "Questionnaire"},
	{Name: "QuestionnaireResponse"},
	{Name: "PlanDefinition"},
	{Name: "ActivityDefinition"},
	{Name: "Library"},
	{Name: "Measure"},
	{Name: "MeasureReport"},
	{Name: "ResearchStudy"},
	{Name: "ResearchSubject"},
	{Name: "ResearchDefinition"},
	{Name: "ResearchElementDefinition"},
	{Name: "Evidence"},
	{Name: "EvidenceVariable"},
	{Name: "EffectEvidenceSynthesis"},
	{Name: "RiskEvidenceSynthesis"},

	// === Devices ===
	{Name: "Device"},
	{Name: "DeviceDefinition"},
	{Name: "DeviceMetric"},

	// === Substances ===
	{Name: "Substance"},
	{Name: "SubstanceSpecification"},
	{Name: "BiologicallyDerivedProduct"},

	// === Medicinal Products (R4) ===
	{Name: "MedicinalProduct"},
	{Name: "MedicinalProductAuthorization"},
	{Name: "MedicinalProductContraindication"},
	{Name: "MedicinalProductIndication"},
	{Name: "MedicinalProductIngredient"},
	{Name: "MedicinalProductInteraction"},
	{Name: "MedicinalProductManufactured"},
	{Name: "MedicinalProductPackaged"},
	{Name: "MedicinalProductPharmaceutical"},
	{Name: "MedicinalProductUndesirableEffect"},

	// === Conformance & Terminology ===
	{Name: "CapabilityStatement"},
	{Name: "StructureDefinition"},
	{Name: "StructureMap"},
	{Name: "ImplementationGuide"},
	{Name: "SearchParameter"},
	{Name: "OperationDefinition"},
	{Name: "CompartmentDefinition"},
	{Name: "GraphDefinition"},
	{Name: "ExampleScenario"},
	{Name: "CodeSystem"},
	{Name: "ValueSet"},
	{Name: "ConceptMap"},
	{Name: "NamingSystem"},
	{Name: "TerminologyCapabilities"},

	// === Messaging ===
	{Name: "MessageHeader"},
	{Name: "MessageDefinition"},
	{Name: "Subscription"},
	{Name: "OperationOutcome"},

	// === Testing ===
	{Name: "TestScript"},
	{Name: "TestReport"},

	// === Other ===
	{Name: "EventDefinition"},
	{Name: "CatalogEntry"},
}

// testHL7Examples validates HL7 examples for a specific resource type.
func testHL7Examples(t *testing.T, v *Validator, resourceType string) (passed, failed int, failedFiles []string) {
	examplesDir := "../../testdata/hl7-examples"
	files, err := filepath.Glob(filepath.Join(examplesDir, resourceType+"-*.json"))
	if err != nil {
		t.Fatalf("Failed to glob %s files: %v", resourceType, err)
	}

	if len(files) == 0 {
		t.Skipf("No %s examples found", resourceType)
	}

	t.Logf("Found %d %s examples", len(files), resourceType)

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}

			result, err := v.Validate(context.Background(), data)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}

			// Log all errors
			for _, issue := range result.Issues {
				if issue.Severity == "error" {
					t.Logf("  [%s] %s @ %v", issue.Severity, issue.Diagnostics, issue.Expression)
				}
			}

			// HL7 examples should have 0 errors (warnings are OK)
			if result.ErrorCount() > 0 {
				failed++
				failedFiles = append(failedFiles, name)
				t.Errorf("Expected 0 errors for official HL7 example, got %d", result.ErrorCount())
			} else {
				passed++
			}
		})
	}

	return passed, failed, failedFiles
}

// TestHL7ExamplesAll runs validation tests for all configured resource types.
func TestHL7ExamplesAll(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}

	var totalPassed, totalFailed int
	var allFailedFiles []string

	for _, rt := range hl7ResourceTypes {
		t.Run(rt.Name, func(t *testing.T) {
			if rt.Skip {
				t.Skipf("Skipping %s: %s", rt.Name, rt.SkipReason)
			}

			passed, failed, failedFiles := testHL7Examples(t, v, rt.Name)

			totalPassed += passed
			totalFailed += failed
			for _, f := range failedFiles {
				allFailedFiles = append(allFailedFiles, rt.Name+"/"+f)
			}

			t.Logf("=== %s: %d/%d ===", rt.Name, passed, passed+failed)
		})
	}

	t.Logf("\n========================================")
	t.Logf("TOTAL: %d/%d passed", totalPassed, totalPassed+totalFailed)
	if len(allFailedFiles) > 0 {
		t.Logf("Failed files: %s", strings.Join(allFailedFiles, ", "))
	}
	t.Logf("========================================")
}

// Individual test functions for CI/CD that need per-resource granularity.
// These use the shared testHL7Examples helper.

func TestHL7PatientExamples(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}
	passed, failed, failedFiles := testHL7Examples(t, v, "Patient")
	t.Logf("Patient: %d/%d", passed, passed+failed)
	if len(failedFiles) > 0 {
		t.Logf("Failed: %s", strings.Join(failedFiles, ", "))
	}
}

func TestHL7ObservationExamples(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}
	passed, failed, failedFiles := testHL7Examples(t, v, "Observation")
	t.Logf("Observation: %d/%d", passed, passed+failed)
	if len(failedFiles) > 0 {
		t.Logf("Failed: %s", strings.Join(failedFiles, ", "))
	}
}

func TestHL7BundleExamples(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}
	passed, failed, failedFiles := testHL7Examples(t, v, "Bundle")
	t.Logf("Bundle: %d/%d", passed, passed+failed)
	if len(failedFiles) > 0 {
		t.Logf("Failed: %s", strings.Join(failedFiles, ", "))
	}
}

func TestHL7QuestionnaireExamples(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}
	passed, failed, failedFiles := testHL7Examples(t, v, "Questionnaire")
	t.Logf("Questionnaire: %d/%d", passed, passed+failed)
	if len(failedFiles) > 0 {
		t.Logf("Failed: %s", strings.Join(failedFiles, ", "))
	}
}

func TestHL7PlanDefinitionExamples(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Skipf("Cannot create validator: %v", err)
	}
	passed, failed, failedFiles := testHL7Examples(t, v, "PlanDefinition")
	t.Logf("PlanDefinition: %d/%d", passed, passed+failed)
	if len(failedFiles) > 0 {
		t.Logf("Failed: %s", strings.Join(failedFiles, ", "))
	}
}
