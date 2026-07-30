package terminology

import (
	"context"
	"testing"
)

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

// TestUnheldIncludeVersionFallsBack covers the published corpus: 441 of its
// include/exclude entries carry a version and only 3 match the CodeSystem shipped
// alongside them — the v2 ValueSets in hl7.terminology request "2.0.0" of
// CodeSystems that same package ships at "3.0.0". Resolving nothing would make 433
// includes unexpandable, so an unheld version falls back to the one held.
func TestUnheldIncludeVersionFallsBack(t *testing.T) {
	const system = "http://terminology.hl7.org/CodeSystem/v2-0155"

	r := NewRegistry()
	held := &CodeSystem{
		URL:     system,
		Version: "3.0.0",
		Concept: []CodeSystemCode{{Code: "AL"}, {Code: "NE"}},
	}
	r.codeSystems[system] = held
	r.codeSystemsByVersion[system+"|3.0.0"] = held

	vs := &ValueSet{
		URL: "http://terminology.hl7.org/ValueSet/v2-0155",
		Compose: Compose{Include: []Include{{
			System:  system,
			Version: "2.0.0", // not loaded; the corpus asks for it anyway
		}}},
	}
	r.valueSets[vs.URL] = vs

	if valid, found := r.ValidateCode(vs.URL, system, "AL"); !found || !valid {
		t.Error("an unheld version must fall back rather than make the include unexpandable")
	}
	if r.CodeSystemVersionMatches(system, "2.0.0") {
		t.Error("the fallback must remain distinguishable from an exact match")
	}
}

// TestIncludeVersionResolvesExactVersion covers the case the corpus does not
// exercise but a loaded IG can: when the requested version is held, it is the one
// expanded from.
func TestIncludeVersionResolvesExactVersion(t *testing.T) {
	const system = "http://example.org/CodeSystem/versioned"

	r := NewRegistry()
	old := &CodeSystem{
		URL: system, Version: "1.0.0",
		Concept: []CodeSystemCode{{Code: "only-in-v1"}},
	}
	current := &CodeSystem{
		URL: system, Version: "2.0.0",
		Concept: []CodeSystemCode{{Code: "only-in-v2"}},
	}
	r.codeSystems[system] = current // unversioned lookups get the newer one
	r.codeSystemsByVersion[system+"|1.0.0"] = old
	r.codeSystemsByVersion[system+"|2.0.0"] = current

	vs := &ValueSet{
		URL: "http://example.org/ValueSet/pinned-to-v1",
		Compose: Compose{Include: []Include{{
			System:  system,
			Version: "1.0.0",
		}}},
	}
	r.valueSets[vs.URL] = vs

	codes := r.expandValueSet(vs)
	if !codes[system+"|only-in-v1"] {
		t.Error("the requested version was loaded, so its codes must be the ones expanded")
	}
	if codes[system+"|only-in-v2"] {
		t.Error("a version was pinned; codes from another version must not leak in")
	}
}

// TestCodeSystemVersionMatchesReportsFallback makes the fallback observable rather
// than silent, so a caller can tell an exact match from a substitution.
func TestCodeSystemVersionMatchesReportsFallback(t *testing.T) {
	const system = "http://example.org/CodeSystem/versioned"

	r := NewRegistry()
	held := &CodeSystem{URL: system, Version: "3.0.0"}
	r.codeSystems[system] = held
	r.codeSystemsByVersion[system+"|3.0.0"] = held

	if !r.CodeSystemVersionMatches(system, "3.0.0") {
		t.Error("the held version must report as a match")
	}
	if r.CodeSystemVersionMatches(system, "2.0.0") {
		t.Error("a version that was not loaded must not report as a match")
	}
	if !r.CodeSystemVersionMatches(system, "") {
		t.Error("no requested version is trivially a match")
	}

	// The fallback still resolves, so expansion is possible.
	if got := r.GetCodeSystemVersion(system, "2.0.0"); got != held {
		t.Error("an unheld version must fall back to the one held, not resolve nothing")
	}
}

// TestVersionedCanonicalResolvesExactly checks the accessors used by callers that
// pass a "url|version" canonical rather than a separate version.
func TestVersionedCanonicalResolvesExactly(t *testing.T) {
	const url = "http://example.org/ValueSet/v"

	r := NewRegistry()
	v1 := &ValueSet{URL: url, Version: "1.0.0"}
	v2 := &ValueSet{URL: url, Version: "2.0.0"}
	r.valueSets[url] = v2
	r.valueSetsByVersion[url+"|1.0.0"] = v1
	r.valueSetsByVersion[url+"|2.0.0"] = v2

	if got := r.GetValueSet(url + "|1.0.0"); got != v1 {
		t.Error("a versioned canonical must resolve to that exact version when loaded")
	}
	if got := r.GetValueSet(url); got != v2 {
		t.Error("an unversioned lookup resolves to the version held")
	}
	if got := r.GetValueSet(url + "|9.9.9"); got != v2 {
		t.Error("an unheld version falls back rather than resolving nothing")
	}
}

// TestExpansionCacheIsScopedByVersion guards the hole that made version-aware
// resolution illusory: the expansion cache was keyed by the version-stripped URL,
// so a lookup against one version could be answered from another's expansion.
func TestExpansionCacheIsScopedByVersion(t *testing.T) {
	const system = "http://example.org/CodeSystem/cs"
	const vsURL = "http://example.org/ValueSet/vs"

	r := NewRegistry()

	csV1 := &CodeSystem{URL: system, Version: "1.0.0", Concept: []CodeSystemCode{{Code: "old"}}}
	csV2 := &CodeSystem{URL: system, Version: "2.0.0", Concept: []CodeSystemCode{{Code: "new"}}}
	r.codeSystems[system] = csV2
	r.codeSystemsByVersion[system+"|1.0.0"] = csV1
	r.codeSystemsByVersion[system+"|2.0.0"] = csV2

	vsV1 := &ValueSet{
		URL: vsURL, Version: "1.0.0",
		Compose: Compose{Include: []Include{{System: system, Version: "1.0.0"}}},
	}
	vsV2 := &ValueSet{
		URL: vsURL, Version: "2.0.0",
		Compose: Compose{Include: []Include{{System: system, Version: "2.0.0"}}},
	}
	r.valueSets[vsURL] = vsV2
	r.valueSetsByVersion[vsURL+"|1.0.0"] = vsV1
	r.valueSetsByVersion[vsURL+"|2.0.0"] = vsV2

	// Warm the cache with v1, then ask v2. A shared cache entry would answer the
	// second question with the first expansion.
	if valid, _ := r.ValidateCode(vsURL+"|1.0.0", system, "old"); !valid {
		t.Fatal(`"old" is a member of the v1 ValueSet`)
	}
	if valid, _ := r.ValidateCode(vsURL+"|2.0.0", system, "new"); !valid {
		t.Error(`"new" is a member of the v2 ValueSet; the v1 expansion must not answer for it`)
	}
	if valid, _ := r.ValidateCode(vsURL+"|2.0.0", system, "old"); valid {
		t.Error(`"old" is not in the v2 ValueSet; the cached v1 expansion leaked`)
	}
}

// TestHierarchyCacheIsScopedByVersion covers the same hazard for is-a filters: two
// versions of a CodeSystem can have different hierarchies.
func TestHierarchyCacheIsScopedByVersion(t *testing.T) {
	const system = "http://example.org/CodeSystem/tree"

	r := NewRegistry()

	// In v1, "b" is under "a". In v2 it is not.
	csV1 := &CodeSystem{
		URL: system, Version: "1.0.0",
		Concept: []CodeSystemCode{
			{Code: "a"},
			{Code: "b", Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "a"}}},
		},
	}
	csV2 := &CodeSystem{
		URL: system, Version: "2.0.0",
		Concept: []CodeSystemCode{{Code: "a"}, {Code: "b"}},
	}
	r.codeSystems[system] = csV2
	r.codeSystemsByVersion[system+"|1.0.0"] = csV1
	r.codeSystemsByVersion[system+"|2.0.0"] = csV2

	isAOver := func(csVersion string) map[string]bool {
		vs := &ValueSet{
			URL:     "http://example.org/ValueSet/isa-" + csVersion,
			Version: csVersion,
			Compose: Compose{Include: []Include{{
				System:  system,
				Version: csVersion,
				Filter:  []Filter{{Property: "concept", Op: "is-a", Value: "a"}},
			}}},
		}
		r.valueSets[vs.URL] = vs
		return r.expandValueSet(vs)
	}

	if !isAOver("1.0.0")[system+"|b"] {
		t.Fatal(`in v1, "b" is subsumed by "a"`)
	}
	if isAOver("2.0.0")[system+"|b"] {
		t.Error(`in v2, "b" is not under "a"; the v1 hierarchy was reused`)
	}
}

// TestCodingVersionSelectsTheCodeSystem covers Coding.version: a code can exist in
// one version of a CodeSystem and be absent from another, so a Coding that declares
// a version must be checked against that version.
func TestCodingVersionSelectsTheCodeSystem(t *testing.T) {
	const system = "http://example.org/CodeSystem/lab-status"

	r := NewRegistry()
	v1 := &CodeSystem{
		URL: system, Version: "1.0.0",
		Concept: []CodeSystemCode{{Code: "pending", Display: "Pending"}},
	}
	v2 := &CodeSystem{
		URL: system, Version: "2.0.0",
		Concept: []CodeSystemCode{{Code: "waiting", Display: "Waiting"}},
	}
	r.codeSystems[system] = v2
	r.codeSystemsByVersion[system+"|1.0.0"] = v1
	r.codeSystemsByVersion[system+"|2.0.0"] = v2

	ctx := context.Background()

	// "pending" exists only in 1.0.0.
	res, err := r.ResolveCodeInCodeSystem(ctx, system, "pending", LookupOptions{SystemVersion: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != Valid {
		t.Errorf(`"pending" is in 1.0.0; got %v`, res.Resolution)
	}
	if res.Display != "Pending" {
		t.Errorf("display = %q, want %q: the display must come from the requested version",
			res.Display, "Pending")
	}

	res, err = r.ResolveCodeInCodeSystem(ctx, system, "pending", LookupOptions{SystemVersion: "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != Invalid {
		t.Errorf(`"pending" was removed in 2.0.0, so it must be Invalid there; got %v`, res.Resolution)
	}

	// Without a declared version, the current one answers.
	res, err = r.ResolveCodeInCodeSystem(ctx, system, "waiting", LookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != Valid {
		t.Errorf(`"waiting" is in the current version; got %v`, res.Resolution)
	}
}

// TestCodingVersionDoesNotBreakExternalSystems guards the plumbing: the versioned
// canonical must not leak into the external-system check or into provider calls,
// which are about the system itself.
func TestCodingVersionDoesNotBreakExternalSystems(t *testing.T) {
	r := NewRegistry()
	seen := ""
	r.SetProvider(&recordingSystemProvider{onValidate: func(system string) { seen = system }})

	_, _ = r.ResolveCodeInCodeSystem(context.Background(), externalSystem, "73211009",
		LookupOptions{SystemVersion: "20240301"})

	if seen != externalSystem {
		t.Errorf("provider saw system %q, want %q: the version must not be appended", seen, externalSystem)
	}
}

type recordingSystemProvider struct {
	onValidate func(system string)
}

func (p *recordingSystemProvider) ValidateCode(_ context.Context, system, _ string) (bool, error) {
	p.onValidate(system)
	return true, nil
}

func (p *recordingSystemProvider) ValidateCodeInValueSet(_ context.Context, _, _, _ string) (valid, found bool, err error) {
	return true, true, nil
}

// TestSystemInValueSetIsVersionAware covers the last place that answered from the
// wrong version. Which systems a ValueSet declares can change between versions, and
// Membership selects the extensible diagnostic — including whether it is a warning
// or informational, which decides pass/fail under strict mode.
func TestSystemInValueSetIsVersionAware(t *testing.T) {
	const vsURL = "http://example.org/ValueSet/vs"
	const systemA = "http://example.org/CodeSystem/a"
	const systemB = "http://example.org/CodeSystem/b"

	r := NewRegistry()

	// v1 declares only A; v2 added B.
	v1 := &ValueSet{
		URL: vsURL, Version: "1.0.0",
		Compose: Compose{Include: []Include{{System: systemA}}},
	}
	v2 := &ValueSet{
		URL: vsURL, Version: "2.0.0",
		Compose: Compose{Include: []Include{{System: systemA}, {System: systemB}}},
	}
	r.valueSets[vsURL] = v2
	r.valueSetsByVersion[vsURL+"|1.0.0"] = v1
	r.valueSetsByVersion[vsURL+"|2.0.0"] = v2

	if r.IsSystemInValueSet(vsURL+"|1.0.0", systemB) {
		t.Error("v1.0.0 never declared system B; answering from v2 would report a " +
			"permitted extension as an expected-system miss")
	}
	if !r.IsSystemInValueSet(vsURL+"|2.0.0", systemB) {
		t.Error("v2.0.0 declares system B")
	}
	if !r.IsSystemInValueSet(vsURL, systemB) {
		t.Error("an unversioned lookup uses the version held, which declares B")
	}

	// The Membership reported to binding validation follows.
	if got := r.localMembership(vsURL+"|1.0.0", systemB); got != MembershipExcluded {
		t.Errorf("localMembership = %v, want %v for a system the pinned version does not declare",
			got, MembershipExcluded)
	}
	if got := r.localMembership(vsURL+"|2.0.0", systemB); got != MembershipIncluded {
		t.Errorf("localMembership = %v, want %v", got, MembershipIncluded)
	}
}
