package terminology

import "testing"

// Filter operators beyond is-a and =, as used by the R4 base corpus:
// descendent-of (17 includes), is-not-a (1) and not-in (1).
// https://hl7.org/fhir/R4/codesystem-filter-operator.html

// mediaTypeCodeSystem mirrors the shape of v3-mediaType, where the base corpus
// uses descendent-of over a nested hierarchy.
func mediaTypeCodeSystem(url string) *CodeSystem {
	return &CodeSystem{
		URL: url,
		Concept: []CodeSystemCode{
			{
				Code: "image",
				Concept: []CodeSystemCode{
					{Code: "image/png"},
					{Code: "image/jpeg", Concept: []CodeSystemCode{{Code: "image/jpeg2000"}}},
				},
			},
			{Code: "audio", Concept: []CodeSystemCode{{Code: "audio/mpeg"}}},
		},
	}
}

func expandFilter(t *testing.T, cs *CodeSystem, op, property, value string) map[string]bool {
	t.Helper()
	r := NewRegistry()
	r.codeSystems[cs.URL] = cs
	vs := &ValueSet{
		URL: "http://example.org/ValueSet/under-test",
		Compose: Compose{Include: []Include{{
			System: cs.URL,
			Filter: []Filter{{Property: property, Op: op, Value: value}},
		}}},
	}
	r.valueSets[vs.URL] = vs
	return r.expandValueSet(vs)
}

func TestDescendantOfExcludesSelf(t *testing.T) {
	const system = "http://terminology.hl7.org/CodeSystem/v3-mediaType"
	codes := expandFilter(t, mediaTypeCodeSystem(system), "descendent-of", "concept", "image")

	for _, want := range []string{"image/png", "image/jpeg", "image/jpeg2000"} {
		if !codes[system+"|"+want] {
			t.Errorf("%q should be a member: descendent-of includes transitive descendants", want)
		}
	}
	if codes[system+"|image"] {
		t.Error(`"image" must not be a member: descendent-of excludes the provided concept`)
	}
	if codes[system+"|audio/mpeg"] {
		t.Error(`"audio/mpeg" is not under "image"`)
	}
}

// TestDescendantOfAndIsAOnlyDifferBySelf pins the distinction that was previously
// collapsed: the two operators produced identical expansions because is-a never
// added the concept itself.
func TestDescendantOfAndIsAOnlyDifferBySelf(t *testing.T) {
	const system = "http://terminology.hl7.org/CodeSystem/v3-mediaType"

	isA := expandFilter(t, mediaTypeCodeSystem(system), "is-a", "concept", "image")
	descOf := expandFilter(t, mediaTypeCodeSystem(system), "descendent-of", "concept", "image")

	if !isA[system+"|image"] {
		t.Error("is-a must include the provided concept")
	}
	if descOf[system+"|image"] {
		t.Error("descendent-of must exclude the provided concept")
	}

	// Every other key must match.
	for key := range isA {
		if key == "image" || key == system+"|image" {
			continue
		}
		if !descOf[key] {
			t.Errorf("key %q present under is-a but missing under descendent-of", key)
		}
	}
	for key := range descOf {
		if !isA[key] {
			t.Errorf("key %q present under descendent-of but missing under is-a", key)
		}
	}
}

// TestIsNotAExcludesTheClosure mirrors patient-contactrelationship, which includes
// v2-0131 filtered by is-not-a "O".
func TestIsNotAExcludesTheClosure(t *testing.T) {
	const system = "http://terminology.hl7.org/CodeSystem/v2-0131"

	cs := &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{Code: "C"},
			{Code: "E"},
			{Code: "O", Concept: []CodeSystemCode{{Code: "O-sub"}}},
		},
	}

	codes := expandFilter(t, cs, "is-not-a", "concept", "O")

	for _, want := range []string{"C", "E"} {
		if !codes[system+"|"+want] {
			t.Errorf("%q should be a member: it is outside O's is-a closure", want)
		}
	}
	if codes[system+"|O"] {
		t.Error(`"O" is the filter's own concept and must be excluded`)
	}
	if codes[system+"|O-sub"] {
		t.Error(`"O-sub" is a descendant of "O" and must be excluded`)
	}
}

// obligationCodeSystem mirrors the only not-in use in the base corpus: the
// obligation CodeSystem, filtered to exclude concepts flagged not-selectable.
func obligationCodeSystem(url string) *CodeSystem {
	yes := true
	return &CodeSystem{
		URL: url,
		Concept: []CodeSystemCode{
			{Code: "SHALL:populate"},
			{Code: "SHALL:handle"},
			{
				Code:     "grouping",
				Property: []CodeSystemProperty{{Code: "not-selectable", ValueBoolean: &yes}},
			},
		},
	}
}

func TestNotInFiltersByPropertyValue(t *testing.T) {
	const system = "http://hl7.org/fhir/CodeSystem/obligation"
	codes := expandFilter(t, obligationCodeSystem(system), "not-in", "not-selectable", "true")

	for _, want := range []string{"SHALL:populate", "SHALL:handle"} {
		if !codes[system+"|"+want] {
			t.Errorf("%q should be a member: it does not carry not-selectable=true", want)
		}
	}
	if codes[system+"|grouping"] {
		t.Error(`"grouping" carries not-selectable=true and must be excluded by not-in`)
	}
}

// TestInIsTheDualOfNotIn also documents the treatment of a missing property: a
// concept without the property is not "in" the list.
func TestInIsTheDualOfNotIn(t *testing.T) {
	const system = "http://hl7.org/fhir/CodeSystem/obligation"
	codes := expandFilter(t, obligationCodeSystem(system), "in", "not-selectable", "true")

	if !codes[system+"|grouping"] {
		t.Error(`"grouping" carries not-selectable=true and must be a member under in`)
	}
	for _, notWanted := range []string{"SHALL:populate", "SHALL:handle"} {
		if codes[system+"|"+notWanted] {
			t.Errorf("%q lacks the property entirely, so it is not in the list", notWanted)
		}
	}
}

func TestNotInAcceptsMultipleValues(t *testing.T) {
	const system = "http://example.org/CodeSystem/statuses"
	cs := &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{Code: "a", Property: []CodeSystemProperty{{Code: "status", ValueCode: "draft"}}},
			{Code: "b", Property: []CodeSystemProperty{{Code: "status", ValueCode: "retired"}}},
			{Code: "c", Property: []CodeSystemProperty{{Code: "status", ValueCode: "active"}}},
		},
	}

	codes := expandFilter(t, cs, "not-in", "status", "draft, retired")

	if !codes[system+"|c"] {
		t.Error(`"c" is active, which is not in the list`)
	}
	for _, excluded := range []string{"a", "b"} {
		if codes[system+"|"+excluded] {
			t.Errorf("%q has a status in the comma-separated list and must be excluded", excluded)
		}
	}
}

// TestDescendantsOfSurvivesCyclicHierarchy guards the walk: a malformed
// subsumedBy chain must not hang the expansion.
func TestDescendantsOfSurvivesCyclicHierarchy(t *testing.T) {
	const system = "http://example.org/CodeSystem/cyclic"
	cs := &CodeSystem{
		URL: system,
		Concept: []CodeSystemCode{
			{Code: "a", Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "b"}}},
			{Code: "b", Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "a"}}},
		},
	}

	codes := expandFilter(t, cs, "descendent-of", "concept", "a")
	if !codes[system+"|b"] {
		t.Error(`"b" is subsumed by "a" and should be a member`)
	}
}
