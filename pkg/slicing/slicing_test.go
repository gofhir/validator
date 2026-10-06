package slicing

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/validator/v2/internal/testfhir"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/registry"
)

var (
	sharedSlicingValidator *Validator
	sharedRegistry         *registry.Registry
	sharedSetupOnce        sync.Once
	errSharedSetup         error
)

func getSharedSetup(t *testing.T) (*Validator, *registry.Registry) {
	t.Helper()
	sharedSetupOnce.Do(func() {
		l := loader.NewLoader("")
		packages, err := l.LoadVersion("4.0.1")
		if err != nil {
			errSharedSetup = fmt.Errorf("failed to load packages: %w", err)
			return
		}

		// Also try to load US Core if available
		usCorePkgs, _ := l.LoadPackage("hl7.fhir.us.core", "6.1.0")
		if usCorePkgs != nil {
			packages = append(packages, usCorePkgs)
		}

		reg := registry.New()
		if err := reg.LoadFromPackages(packages); err != nil {
			errSharedSetup = fmt.Errorf("failed to load registry: %w", err)
			return
		}

		sharedRegistry = reg
		sharedSlicingValidator = New(reg)
	})
	if errSharedSetup != nil {
		t.Fatalf("Shared setup failed: %v", errSharedSetup)
	}
	return sharedSlicingValidator, sharedRegistry
}

func TestSlicingValidation_OpenRules(t *testing.T) {
	validator, reg := getSharedSetup(t)

	// Test resource with extensions (open slicing allows additional extensions)
	resource := json.RawMessage(`{
		"resourceType": "Patient",
		"extension": [
			{
				"url": "http://example.org/custom-extension",
				"valueString": "custom value"
			}
		],
		"name": [{"family": "Test"}]
	}`)

	patientSD := reg.GetByType("Patient")
	if patientSD == nil {
		t.Fatal("Patient SD not found")
	}

	result := issue.NewResult()
	validator.Validate(resource, patientSD, result)

	// With open rules, custom extensions should be allowed
	if result.ErrorCount() > 0 {
		t.Errorf("Expected no errors for open slicing with custom extension, got %d errors", result.ErrorCount())
		for _, iss := range result.Issues {
			t.Logf("  Issue: %s - %s", iss.Severity, iss.Diagnostics)
		}
	}
}

// childProfile is a Patient profile with three slices: name:social (given required, family at
// most one); contact:f, whose optional period requires start (the shape of Bundle.entry.request,
// D1); and extension:x, whose value[x] is required (D1b).
const childProfile = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/child","name":"Child",
"type":"Patient","kind":"resource","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient",
"snapshot":{"element":[
 {"id":"Patient","path":"Patient","min":0,"max":"*"},
 {"id":"Patient.name","path":"Patient.name","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"use"}],"rules":"open"},"type":[{"code":"HumanName"}]},
 {"id":"Patient.name:social","path":"Patient.name","sliceName":"social","min":0,"max":"*","type":[{"code":"HumanName"}]},
 {"id":"Patient.name:social.use","path":"Patient.name.use","min":1,"max":"1","fixedCode":"usual","type":[{"code":"code"}]},
 {"id":"Patient.name:social.family","path":"Patient.name.family","min":0,"max":"1","type":[{"code":"string"}]},
 {"id":"Patient.name:social.given","path":"Patient.name.given","min":1,"max":"*","type":[{"code":"string"}]},
 {"id":"Patient.name:social.prefix","path":"Patient.name.prefix","min":0,"max":"1","type":[{"code":"string"}]},
 {"id":"Patient.contact","path":"Patient.contact","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"gender"}],"rules":"open"},"type":[{"code":"BackboneElement"}]},
 {"id":"Patient.contact:f","path":"Patient.contact","sliceName":"f","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
 {"id":"Patient.contact:f.gender","path":"Patient.contact.gender","min":1,"max":"1","fixedCode":"female","type":[{"code":"code"}]},
 {"id":"Patient.contact:f.period","path":"Patient.contact.period","min":0,"max":"1","type":[{"code":"Period"}]},
 {"id":"Patient.contact:f.period.start","path":"Patient.contact.period.start","min":1,"max":"1","type":[{"code":"dateTime"}]},
 {"id":"Patient.extension","path":"Patient.extension","min":0,"max":"*","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]},
 {"id":"Patient.extension:x","path":"Patient.extension","sliceName":"x","min":0,"max":"1","type":[{"code":"Extension"}]},
 {"id":"Patient.extension:x.url","path":"Patient.extension.url","min":1,"max":"1","fixedUri":"https://example.org/x","type":[{"code":"uri"}]},
 {"id":"Patient.extension:x.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","type":[{"code":"string"},{"code":"Quantity"}]},
 {"id":"Patient.extension:y","path":"Patient.extension","sliceName":"y","min":0,"max":"1","type":[{"code":"Extension","profile":["https://example.org/fhir/StructureDefinition/y"]}]}
]}}`

// extensionY is the definition of the extension:y slice, which the profile does not unroll: its
// value[x] is required by the extension's own StructureDefinition.
const extensionY = `{"resourceType":"StructureDefinition","url":"https://example.org/fhir/StructureDefinition/y","name":"Y",
"type":"Extension","kind":"complex-type","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension",
"snapshot":{"element":[
 {"id":"Extension","path":"Extension","min":0,"max":"*"},
 {"id":"Extension.url","path":"Extension.url","min":1,"max":"1","fixedUri":"https://example.org/y","type":[{"code":"uri"}]},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"code"}]}
]}}`

// TestSliceChildCardinality checks the elements inside matched slice instances: a required
// child, a maximum, and (D1) a required child of an optional element is required only where that
// element is present.
func TestSliceChildCardinality(t *testing.T) {
	l := loader.NewLoader("")
	p, err := l.LoadFromResources([][]byte{[]byte(childProfile), []byte(extensionY)})
	if err != nil {
		t.Fatal(err)
	}
	reg := testfhir.Registry(t, "4.0.1")
	if err := reg.LoadFromPackages([]*loader.Package{p}); err != nil {
		t.Fatal(err)
	}
	sd := reg.GetByURL("https://example.org/fhir/StructureDefinition/child")
	v := New(reg)

	for _, tt := range []struct {
		name, resource string
		want           []string // "<ID> @ <location>"
	}{
		{"missing required child", `{"resourceType":"Patient","name":[{"use":"usual","family":"Garcia"}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.name[0].given"}},
		{"required child present", `{"resourceType":"Patient","name":[{"use":"usual","family":"Garcia","given":["Maria"]}]}`, nil},
		{"unmatched element is not checked", `{"resourceType":"Patient","name":[{"use":"official","family":"Garcia"}]}`, nil},
		{"maximum exceeded", `{"resourceType":"Patient","name":[{"use":"usual","prefix":["Dr","Prof"],"given":["Maria"]}]}`,
			[]string{"SLICING_CARDINALITY_MAX @ Patient.name[0].prefix"}},
		// HumanName.family is max 1 itself: the cardinality phase reports it, once.
		{"a maximum the base already sets", `{"resourceType":"Patient","name":[{"use":"usual","family":["Garcia","Lopez"],"given":["Maria"]}]}`, nil},
		{"optional parent absent (D1)", `{"resourceType":"Patient","contact":[{"gender":"female"}]}`, nil},
		{"optional parent present, required child missing", `{"resourceType":"Patient","contact":[{"gender":"female","period":{"end":"2020"}}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.contact[0].period.start"}},
		{"optional parent present, required child present", `{"resourceType":"Patient","contact":[{"gender":"female","period":{"start":"2019"}}]}`, nil},
		{"choice present under a typed name (D1b)", `{"resourceType":"Patient","extension":[{"url":"https://example.org/x","valueQuantity":{"value":1}}]}`, nil},
		{"choice missing", `{"resourceType":"Patient","extension":[{"url":"https://example.org/x"}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.extension[0].value[x]"}},
		{"a slice without children is defined by its type's one profile", `{"resourceType":"Patient","extension":[{"url":"https://example.org/y"}]}`,
			[]string{"SLICING_CARDINALITY_MIN @ Patient.extension[0].value[x]"}},
		{"primitive present through its extensions", `{"resourceType":"Patient","name":[{"use":"usual","_given":[{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"masked"}]}]}]}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var res map[string]any
			if err := json.Unmarshal([]byte(tt.resource), &res); err != nil {
				t.Fatal(err)
			}
			result := issue.NewResult()
			v.ValidateData(res, sd, result)
			var got []string
			for _, is := range result.Issues {
				if is.Severity == issue.SeverityError {
					got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("errors %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValueDiscriminator_FollowsTypeProfileChain reproduces the FALP/SUSHI bug where
// a slice declares its expected url implicitly via type[0].profile pointing to an
// Extension StructureDefinition whose Extension.url.fixedUri carries the canonical URL.
// HAPI/HL7 IG Publisher follow this chain; we must too.
//
// FHIR R4 §profiling.html#discriminator: when type.profile is present, that profile
// contributes constraints to the slice — including Extension.url.fixedUri.
// TestValueDiscriminator_TypeProfileChain_MultipleProfiles verifies ANY-of semantics
// when slice.Type[0].Profile carries multiple URLs — match should succeed if the
// instance's url matches the fixedUri of ANY referenced profile.
// TestSlicingValidation_TypeProfileChain_NoCardinalityError end-to-end reproduction
// of the FALP/paito-fhir-server v0.13.0 regression: an ActivityDefinition profile
// declares `extension:category 1..1` via type.profile, the instance carries that
// extension by URL, and the validator must NOT emit SLICING_CARDINALITY_MIN.
func TestSlicingValidation_TypeProfileChain_NoCardinalityError(t *testing.T) {
	const extURL = "https://falp.cl/fhir/ctms/StructureDefinition/falp-activity-category-ext"
	const profileURL = "https://falp.cl/fhir/ctms/StructureDefinition/falp-activity-definition"

	extSD := []byte(`{
		"resourceType": "StructureDefinition",
		"url": "` + extURL + `",
		"name": "FalpActivityCategoryExt",
		"type": "Extension",
		"kind": "complex-type",
		"abstract": false,
		"derivation": "constraint",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
		"snapshot": {
			"element": [
				{"id": "Extension", "path": "Extension"},
				{"id": "Extension.url", "path": "Extension.url", "fixedUri": "` + extURL + `"},
				{"id": "Extension.value[x]", "path": "Extension.value[x]", "type": [{"code": "code"}]}
			]
		}
	}`)

	// Profile slices ActivityDefinition.extension by url with one slice "category"
	// whose only constraint is type.profile → extSD. SUSHI does NOT emit a
	// child Extension.url with fixedUri here — that's the whole point of the bug.
	activityProfile := []byte(`{
		"resourceType": "StructureDefinition",
		"url": "` + profileURL + `",
		"name": "FalpActivityDefinition",
		"type": "ActivityDefinition",
		"kind": "resource",
		"abstract": false,
		"derivation": "constraint",
		"baseDefinition": "http://hl7.org/fhir/StructureDefinition/ActivityDefinition",
		"snapshot": {
			"element": [
				{"id": "ActivityDefinition", "path": "ActivityDefinition"},
				{
					"id": "ActivityDefinition.extension",
					"path": "ActivityDefinition.extension",
					"slicing": {
						"discriminator": [{"type": "value", "path": "url"}],
						"rules": "open"
					},
					"min": 0, "max": "*"
				},
				{
					"id": "ActivityDefinition.extension:category",
					"path": "ActivityDefinition.extension",
					"sliceName": "category",
					"min": 1, "max": "1",
					"type": [{"code": "Extension", "profile": ["` + extURL + `"]}]
				}
			]
		}
	}`)

	l := loader.NewLoader("")
	pkg, err := l.LoadFromResources([][]byte{extSD, activityProfile})
	if err != nil {
		t.Fatalf("LoadFromResources: %v", err)
	}
	reg := registry.New()
	if err := reg.LoadFromPackages([]*loader.Package{pkg}); err != nil {
		t.Fatalf("LoadFromPackages: %v", err)
	}
	profileSD := reg.GetByURL(profileURL)
	if profileSD == nil {
		t.Fatalf("activity profile not loaded into registry")
	}
	v := New(reg)

	resource := map[string]any{
		"resourceType": "ActivityDefinition",
		"status":       "active",
		"extension": []any{
			map[string]any{
				"url":       extURL,
				"valueCode": "local-lab",
			},
		},
	}

	result := issue.NewResult()
	v.ValidateData(resource, profileSD, result)

	for _, iss := range result.Issues {
		if iss.MessageID == string(issue.DiagSlicingCardinalityMin) {
			t.Errorf("unexpected SLICING_CARDINALITY_MIN issue (regression): %s @ %v",
				iss.Diagnostics, iss.Expression)
		}
		if iss.MessageID == string(issue.DiagSlicingNoMatch) {
			t.Errorf("unexpected SLICING_NO_MATCH issue: %s @ %v",
				iss.Diagnostics, iss.Expression)
		}
	}
}
