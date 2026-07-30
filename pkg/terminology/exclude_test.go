package terminology

import "testing"

// Exclusions (compose.exclude) remove codes from the expansion. 412 of the 3237
// ValueSets in the embedded R4 corpus carry exclusions, so ignoring them
// over-accepts codes.
// https://hl7.org/fhir/R4/valueset-definitions.html#ValueSet.compose.exclude

func TestExcludeExplicitConcept(t *testing.T) {
	const system = "http://example.org/CodeSystem/colors"

	r := NewRegistry()
	r.codeSystems[system] = &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{Code: "red"}, {Code: "green"}, {Code: "blue"},
		},
	}
	vs := &ValueSet{
		URL: "http://example.org/ValueSet/not-blue",
		Compose: Compose{
			Include: []Include{{System: system}},
			Exclude: []Include{{
				System:  system,
				Concept: []Concept{{Code: "blue"}},
			}},
		},
	}
	r.valueSets[vs.URL] = vs

	if valid, _ := r.ValidateCode(vs.URL, system, "red"); !valid {
		t.Error("red should remain a member")
	}
	if valid, _ := r.ValidateCode(vs.URL, system, "blue"); valid {
		t.Error("blue is excluded and must not be a member")
	}
}

func TestExcludeByFilter(t *testing.T) {
	const system = "http://example.org/CodeSystem/animals"

	r := NewRegistry()
	r.codeSystems[system] = &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{Code: "animal"},
			{Code: "mammal", Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "animal"}}},
			{Code: "cat", Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "mammal"}}},
			{Code: "bird", Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "animal"}}},
		},
	}
	vs := &ValueSet{
		URL: "http://example.org/ValueSet/non-mammals",
		Compose: Compose{
			Include: []Include{{
				System: system,
				Filter: []Filter{{Property: "concept", Op: "is-a", Value: "animal"}},
			}},
			Exclude: []Include{{
				System: system,
				Filter: []Filter{{Property: "concept", Op: "is-a", Value: "mammal"}},
			}},
		},
	}
	r.valueSets[vs.URL] = vs

	if valid, _ := r.ValidateCode(vs.URL, system, "bird"); !valid {
		t.Error("bird should remain a member")
	}
	for _, code := range []string{"mammal", "cat"} {
		if valid, _ := r.ValidateCode(vs.URL, system, code); valid {
			t.Errorf("%q is excluded by the is-a filter and must not be a member", code)
		}
	}
}

// TestExcludeKeepsBareCodeFromOtherSystem covers the subtle case: the expansion
// carries both "code" and "system|code" keys, the bare one so that primitive
// `code` elements can be validated. Excluding one system's code must not
// invalidate the bare code while another system still contributes it.
func TestExcludeKeepsBareCodeFromOtherSystem(t *testing.T) {
	const sysA = "http://example.org/CodeSystem/a"
	const sysB = "http://example.org/CodeSystem/b"

	r := NewRegistry()
	r.codeSystems[sysA] = &CodeSystem{URL: sysA, Concept: []CodeSystemCode{{Code: "other"}}}
	r.codeSystems[sysB] = &CodeSystem{URL: sysB, Concept: []CodeSystemCode{{Code: "other"}}}

	vs := &ValueSet{
		URL: "http://example.org/ValueSet/other-from-b-only",
		Compose: Compose{
			Include: []Include{{System: sysA}, {System: sysB}},
			Exclude: []Include{{System: sysA, Concept: []Concept{{Code: "other"}}}},
		},
	}
	r.valueSets[vs.URL] = vs

	if valid, _ := r.ValidateCode(vs.URL, sysA, "other"); valid {
		t.Error("sysA|other is excluded and must not be a member")
	}
	if valid, _ := r.ValidateCode(vs.URL, sysB, "other"); !valid {
		t.Error("sysB|other was never excluded and must remain a member")
	}
	// A primitive `code` element carries no system: "other" is still reachable
	// through sysB, so it must still validate.
	if valid, _ := r.ValidateCode(vs.URL, "", "other"); !valid {
		t.Error("bare code \"other\" must remain valid: sysB still contributes it")
	}
}

func TestExcludeNestedValueSet(t *testing.T) {
	const system = "http://example.org/CodeSystem/colors"

	r := NewRegistry()
	r.codeSystems[system] = &CodeSystem{
		URL:     system,
		Concept: []CodeSystemCode{{Code: "red"}, {Code: "green"}, {Code: "blue"}},
	}
	warm := &ValueSet{
		URL: "http://example.org/ValueSet/warm",
		Compose: Compose{
			Include: []Include{{System: system, Concept: []Concept{{Code: "red"}}}},
		},
	}
	r.valueSets[warm.URL] = warm

	vs := &ValueSet{
		URL: "http://example.org/ValueSet/cool",
		Compose: Compose{
			Include: []Include{{System: system}},
			Exclude: []Include{{ValueSet: []string{warm.URL}}},
		},
	}
	r.valueSets[vs.URL] = vs

	if valid, _ := r.ValidateCode(vs.URL, system, "blue"); !valid {
		t.Error("blue should remain a member")
	}
	if valid, _ := r.ValidateCode(vs.URL, system, "red"); valid {
		t.Error("red is excluded via the nested ValueSet and must not be a member")
	}
}
