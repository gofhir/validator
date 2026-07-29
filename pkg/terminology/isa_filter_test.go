package terminology

import "testing"

// TestIsAFilterIncludesSelf verifies the FHIR R4 semantics of the "is-a" filter
// operator, which includes the concept named by the filter value:
//
//	is-a: "Includes all concept ids that have a transitive is-a relationship with
//	the concept Id provided as the value, including the provided concept itself
//	(include descendant codes and self)."
//	-- https://hl7.org/fhir/R4/codesystem-filter-operator.html
//
// The root concept here is concrete (selectable), so excluding it is
// unambiguously wrong.
func TestIsAFilterIncludesSelf(t *testing.T) {
	const system = "http://example.org/CodeSystem/animals"

	cs := &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{Code: "mammal", Display: "Mammal"},
			{
				Code:    "cat",
				Display: "Cat",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "mammal"},
				},
			},
			{
				Code:    "dog",
				Display: "Dog",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "mammal"},
				},
			},
		},
	}

	vs := &ValueSet{
		URL: "http://example.org/ValueSet/mammals",
		Compose: Compose{
			Include: []Include{{
				System: system,
				Filter: []Filter{{Property: "concept", Op: "is-a", Value: "mammal"}},
			}},
		},
	}

	r := NewRegistry()
	r.codeSystems[cs.URL] = cs
	r.valueSets[vs.URL] = vs

	// Descendants must be members.
	for _, code := range []string{"cat", "dog"} {
		valid, found := r.ValidateCode(vs.URL, system, code)
		if !found {
			t.Fatalf("ValueSet %s not found", vs.URL)
		}
		if !valid {
			t.Errorf("descendant %q should be a member of the is-a expansion", code)
		}
	}

	// The filter's own concept must be a member too. This is the assertion the
	// existing tests never made.
	valid, found := r.ValidateCode(vs.URL, system, "mammal")
	if !found {
		t.Fatalf("ValueSet %s not found", vs.URL)
	}
	if !valid {
		t.Error("is-a must include the provided concept itself: " +
			"\"mammal\" should be a member of the expansion, but was rejected")
	}
}

// TestIsAFilterExcludesNotSelectableRoot covers the v3 grouping concepts (523 of
// the 1515 is-a filters in the embedded R4 corpus): they head a hierarchy but
// carry notSelectable, so they must not be accepted as instance values even
// though is-a nominally includes self.
func TestIsAFilterExcludesNotSelectableRoot(t *testing.T) {
	const system = "http://terminology.hl7.org/CodeSystem/v3-ActCode"
	yes := true

	cs := &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{
				Code:    "_ActEncounterCode",
				Display: "ActEncounterCode",
				Property: []CodeSystemProperty{
					{Code: "notSelectable", ValueBoolean: &yes},
				},
			},
			{
				Code:    "AMB",
				Display: "ambulatory",
				Property: []CodeSystemProperty{
					{Code: "subsumedBy", ValueCode: "_ActEncounterCode"},
				},
			},
		},
	}

	vs := &ValueSet{
		URL: "http://terminology.hl7.org/ValueSet/v3-ActEncounterCode",
		Compose: Compose{
			Include: []Include{{
				System: system,
				Filter: []Filter{{Property: "concept", Op: "is-a", Value: "_ActEncounterCode"}},
			}},
		},
	}

	r := NewRegistry()
	r.codeSystems[cs.URL] = cs
	r.valueSets[vs.URL] = vs

	if valid, _ := r.ValidateCode(vs.URL, system, "AMB"); !valid {
		t.Error("selectable descendant AMB should be a member")
	}
	if valid, _ := r.ValidateCode(vs.URL, system, "_ActEncounterCode"); valid {
		t.Error("notSelectable grouping concept must not be valid as an instance value")
	}
}

// TestIsAFilterUnknownRootIsNotMinted guards the fix: adding self must not
// invent a member when the filter names a code absent from the CodeSystem.
func TestIsAFilterUnknownRootIsNotMinted(t *testing.T) {
	const system = "http://example.org/CodeSystem/animals"

	cs := &CodeSystem{
		URL:     system,
		Concept: []CodeSystemCode{{Code: "mammal", Display: "Mammal"}},
	}

	vs := &ValueSet{
		URL: "http://example.org/ValueSet/ghosts",
		Compose: Compose{
			Include: []Include{{
				System: system,
				Filter: []Filter{{Property: "concept", Op: "is-a", Value: "unicorn"}},
			}},
		},
	}

	r := NewRegistry()
	r.codeSystems[cs.URL] = cs
	r.valueSets[vs.URL] = vs

	valid, found := r.ValidateCode(vs.URL, system, "unicorn")
	if !found {
		t.Fatalf("ValueSet %s not found", vs.URL)
	}
	if valid {
		t.Error("a code absent from the CodeSystem must not become a member " +
			"just because an is-a filter names it")
	}
}
