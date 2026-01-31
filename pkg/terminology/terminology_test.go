package terminology

import (
	"testing"
)

func TestBuildHierarchy(t *testing.T) {
	// Create a CodeSystem with subsumedBy properties (like v3-ActCode)
	cs := &CodeSystem{
		URL: "http://terminology.hl7.org/CodeSystem/v3-ActCode",
		Concept: []CodeSystemCode{
			{
				Code:    "_ActEncounterCode",
				Display: "ActEncounterCode",
			},
			{
				Code:    "AMB",
				Display: "ambulatory",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
			{
				Code:    "EMER",
				Display: "emergency",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
			{
				Code:    "IMP",
				Display: "inpatient encounter",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
		},
	}

	r := NewRegistry()
	hierarchy := r.buildHierarchy(cs)

	// _ActEncounterCode should have AMB, EMER, IMP as children
	children := hierarchy["_ActEncounterCode"]
	if len(children) != 3 {
		t.Errorf("Expected 3 children for _ActEncounterCode, got %d", len(children))
	}

	expected := map[string]bool{"AMB": true, "EMER": true, "IMP": true}
	for _, child := range children {
		if !expected[child] {
			t.Errorf("Unexpected child code: %s", child)
		}
	}
}

func TestApplyIsAFilter(t *testing.T) {
	// Create a CodeSystem with hierarchy
	cs := &CodeSystem{
		URL: "http://terminology.hl7.org/CodeSystem/v3-ActCode",
		Concept: []CodeSystemCode{
			{Code: "_ActEncounterCode"},
			{
				Code: "AMB",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
			{
				Code: "EMER",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
		},
	}

	r := NewRegistry()
	r.codeSystems[cs.URL] = cs

	codes := make(map[string]bool)
	r.applyIsAFilter(codes, cs, cs.URL, "_ActEncounterCode")

	// Should include AMB and EMER
	if !codes["AMB"] {
		t.Error("Expected AMB to be in codes")
	}
	if !codes["EMER"] {
		t.Error("Expected EMER to be in codes")
	}
	if !codes[cs.URL+"|AMB"] {
		t.Error("Expected system|AMB to be in codes")
	}
}

func TestExpandValueSetWithIsAFilter(t *testing.T) {
	r := NewRegistry()

	// Add CodeSystem
	cs := &CodeSystem{
		URL: "http://terminology.hl7.org/CodeSystem/v3-ActCode",
		Concept: []CodeSystemCode{
			{Code: "_ActEncounterCode"},
			{
				Code:    "AMB",
				Display: "ambulatory",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
			{
				Code:    "EMER",
				Display: "emergency",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
		},
	}
	r.codeSystems[cs.URL] = cs

	// Add ValueSet with is-a filter (like v3-ActEncounterCode)
	vs := &ValueSet{
		URL: "http://terminology.hl7.org/ValueSet/v3-ActEncounterCode",
		Compose: Compose{
			Include: []Include{
				{
					System: "http://terminology.hl7.org/CodeSystem/v3-ActCode",
					Filter: []Filter{
						{
							Property: "concept",
							Op:       "is-a",
							Value:    "_ActEncounterCode",
						},
					},
				},
			},
		},
	}
	r.valueSets[vs.URL] = vs

	// Validate AMB code
	valid, found := r.ValidateCode(vs.URL, cs.URL, "AMB")
	if !found {
		t.Error("Expected ValueSet to be found")
	}
	if !valid {
		t.Error("Expected AMB to be valid in v3-ActEncounterCode ValueSet")
	}

	// Validate EMER code
	valid, found = r.ValidateCode(vs.URL, cs.URL, "EMER")
	if !found {
		t.Error("Expected ValueSet to be found")
	}
	if !valid {
		t.Error("Expected EMER to be valid in v3-ActEncounterCode ValueSet")
	}

	// Validate invalid code
	valid, found = r.ValidateCode(vs.URL, cs.URL, "INVALID")
	if !found {
		t.Error("Expected ValueSet to be found")
	}
	if valid {
		t.Error("Expected INVALID to NOT be valid in v3-ActEncounterCode ValueSet")
	}
}

func TestNestedHierarchy(t *testing.T) {
	// Test deeper hierarchy (grandchildren)
	cs := &CodeSystem{
		URL: "http://example.org/CodeSystem/test",
		Concept: []CodeSystemCode{
			{Code: "ROOT"},
			{
				Code: "PARENT",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "ROOT"},
				},
			},
			{
				Code: "CHILD",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "PARENT"},
				},
			},
		},
	}

	r := NewRegistry()
	r.codeSystems[cs.URL] = cs

	codes := make(map[string]bool)
	r.applyIsAFilter(codes, cs, cs.URL, "ROOT")

	// Should include PARENT and CHILD (descendants)
	if !codes["PARENT"] {
		t.Error("Expected PARENT to be in codes")
	}
	if !codes["CHILD"] {
		t.Error("Expected CHILD to be in codes (grandchild of ROOT)")
	}
}
