package terminology

import (
	"context"
	"testing"
)

// A code is a member of a ValueSet when an include selects it and no exclude does
// (valueset-definitions.html#ValueSet.compose), each clause decided on its own: a code system held
// in full decides its codes, one not held (or partial, or external) leaves them undecided, a
// ValueSet named in a clause is asked, not copied, and a clause naming a system and ValueSets
// selects the codes of both. An undecided code is accepted as assumed, and is Unresolved where
// membership must be known.
func TestMembershipByDefinition(t *testing.T) {
	const (
		held    = "http://example.org/cs/held"
		other   = "http://example.org/cs/other"
		partial = "http://example.org/cs/partial"
		unheld  = "http://example.org/cs/unheld"
		base    = "http://example.org/vs/"
	)
	r := NewRegistry()
	r.codeSystems[held] = &CodeSystem{URL: held, Concept: []CodeSystemCode{
		{Code: "a", Property: []CodeSystemProperty{{Code: "kind", ValueCode: "x"}}},
		{Code: "b", Property: []CodeSystemProperty{{Code: "kind", ValueCode: "y"}}, Concept: []CodeSystemCode{
			{Code: "b1", Property: []CodeSystemProperty{{Code: "kind", ValueCode: "x"}}},
		}},
	}}
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}}}
	r.codeSystems[partial] = &CodeSystem{URL: partial, Content: "fragment", Concept: []CodeSystemCode{{Code: "f1"}, {Code: "f2"}}}
	add := func(name string, c Compose) {
		r.valueSets[base+name] = &ValueSet{URL: base + name, Compose: c}
	}
	add("unheld-but-q", Compose{
		Include: []Include{{System: unheld}},
		Exclude: []Include{{System: unheld, Concept: []Concept{{Code: "q"}}}},
	})
	add("nested", Compose{Include: []Include{{ValueSet: []string{base + "unheld-but-q"}}}})
	add("listed-then-whole-excluded", Compose{
		Include: []Include{{System: unheld, Concept: []Concept{{Code: "a"}, {Code: "b"}}}},
		Exclude: []Include{{System: unheld}},
	})
	add("held-all", Compose{Include: []Include{{System: held}}})
	add("excluded-by-valueset", Compose{
		Include: []Include{{System: held}},
		Exclude: []Include{{ValueSet: []string{base + "held-all"}}},
	})
	add("held-and-other", Compose{Include: []Include{{System: held}, {System: other}}})
	add("system-and-valueset", Compose{Include: []Include{{System: held, ValueSet: []string{base + "only-b"}}}})
	add("only-b", Compose{Include: []Include{{System: held, Concept: []Concept{{Code: "b"}, {Code: "b1"}}}}})
	add("kind-x-under-b", Compose{Include: []Include{{System: held, Filter: []Filter{
		{Property: "concept", Op: "is-a", Value: "b"},
		{Property: "kind", Op: "=", Value: "x"},
	}}}})
	add("partial-but-f1", Compose{
		Include: []Include{{System: partial}},
		Exclude: []Include{{System: partial, Concept: []Concept{{Code: "f1"}}}},
	})
	add("partial-whole-excluded", Compose{
		Include: []Include{{System: partial}, {System: held}},
		Exclude: []Include{{System: partial}},
	})
	add("unheld-all", Compose{Include: []Include{{System: unheld}}})
	add("excluded-whole-by-valueset", Compose{
		Include: []Include{{System: unheld}},
		Exclude: []Include{{ValueSet: []string{base + "unheld-all"}}},
	})
	add("regex-then-is-a", Compose{Include: []Include{{System: held, Filter: []Filter{
		{Property: "code", Op: "regex", Value: "b.*"},
		{Property: "concept", Op: "is-a", Value: "b"},
	}}}})
	add("is-a-then-regex", Compose{Include: []Include{{System: held, Filter: []Filter{
		{Property: "concept", Op: "is-a", Value: "b"},
		{Property: "code", Op: "regex", Value: "b.*"},
	}}}})
	add("excluded-by-undecided-filters", Compose{
		Include: []Include{{System: held}},
		Exclude: []Include{{System: held, Filter: []Filter{
			{Property: "concept", Op: "is-a", Value: "b"},
			{Property: "code", Op: "regex", Value: "z.*"},
		}}},
	})
	add("version-not-held", Compose{Include: []Include{{System: held, Version: "9"}}})
	r.valueSets[base+"expansion-only"] = &ValueSet{URL: base + "expansion-only", Expansion: &Expansion{
		Contains: []ExpansionContains{{Code: "group", Contains: []ExpansionContains{{System: held, Code: "a"}}}},
	}}
	total := 40
	r.valueSets[base+"expansion-page"] = &ValueSet{URL: base + "expansion-page", Expansion: &Expansion{
		Total: &total, Contains: []ExpansionContains{{System: held, Code: "a"}},
	}}
	r.valueSets[base+"composed-and-expanded"] = &ValueSet{URL: base + "composed-and-expanded",
		Compose:   Compose{Include: []Include{{System: held, Concept: []Concept{{Code: "a"}}}}},
		Expansion: &Expansion{Contains: []ExpansionContains{{System: held, Code: "b"}}},
	}
	loop := Compose{}
	for range 4 {
		loop.Include = append(loop.Include, Include{ValueSet: []string{base + "self-loop"}})
	}
	add("self-loop", loop)
	add("excluding-self-loop", Compose{
		Include: []Include{{System: held}},
		Exclude: []Include{{ValueSet: []string{base + "self-loop"}}},
	})
	// Diamonds: a ValueSet reached by two paths, which includes no ValueSet that includes it.
	add("held-via-a", Compose{Include: []Include{{ValueSet: []string{base + "held-all"}}}})
	add("held-via-b", Compose{Include: []Include{{ValueSet: []string{base + "held-all"}}}})
	add("diamond", Compose{Include: []Include{
		{ValueSet: []string{base + "held-via-a"}}, {ValueSet: []string{base + "held-via-b"}},
	}})
	add("unheld-via-a", Compose{Include: []Include{{ValueSet: []string{base + "unheld-all"}}}})
	add("unheld-via-b", Compose{Include: []Include{{ValueSet: []string{base + "unheld-all"}}}})
	add("excluding-a-diamond", Compose{
		Include: []Include{{System: unheld}},
		Exclude: []Include{{ValueSet: []string{base + "unheld-via-a", base + "unheld-via-b"}}},
	})
	add("cycle-a", Compose{Include: []Include{{ValueSet: []string{base + "cycle-b"}}, {System: other, Concept: []Concept{{Code: "k"}}}}})
	add("cycle-b", Compose{Include: []Include{{ValueSet: []string{base + "cycle-a"}}}})
	add("missing-nested", Compose{Include: []Include{{ValueSet: []string{base + "not-held"}}}})

	ctx := context.Background()
	strict := WithStrictMembership(ctx)
	for _, tt := range []struct {
		name, vs, system, code string
		ctx                    context.Context
		want                   Resolution
		assumed                bool
	}{
		{"a code a ValueSet excludes by name, of a system not held", "unheld-but-q", unheld, "q", ctx, Invalid, false},
		{"another code of that system", "unheld-but-q", unheld, "r", ctx, Valid, true},
		{"the same, where membership must be known", "unheld-but-q", unheld, "r", strict, Unresolved, false},
		{"a code a nested ValueSet excludes", "nested", unheld, "q", ctx, Invalid, false},
		{"a code element's code a nested ValueSet excludes", "nested", "", "q", ctx, Invalid, false},
		{"a listed code of a system excluded whole", "listed-then-whole-excluded", unheld, "a", ctx, Invalid, false},
		{"a code an excluded ValueSet holds", "excluded-by-valueset", held, "a", ctx, Invalid, false},
		{"a code element's code of either system", "held-and-other", "", "a", ctx, Valid, false},
		{"a code of the system and the ValueSet", "system-and-valueset", held, "b1", ctx, Valid, false},
		{"a code of the system, not the ValueSet", "system-and-valueset", held, "a", ctx, Invalid, false},
		{"a code every filter selects", "kind-x-under-b", held, "b1", ctx, Valid, false},
		{"a code one filter selects", "kind-x-under-b", held, "a", ctx, Invalid, false},
		{"a code a fragment lacks", "partial-but-f1", partial, "f9", ctx, Valid, true},
		{"a code a fragment has, excluded", "partial-but-f1", partial, "f1", ctx, Invalid, false},
		{"a code a fragment has", "partial-but-f1", partial, "f2", ctx, Valid, false},
		{"a code of a fragment excluded whole", "partial-whole-excluded", partial, "f2", ctx, Invalid, false},
		{"a code of the system it keeps", "partial-whole-excluded", held, "a", ctx, Valid, false},
		{"a code of a system an excluded ValueSet includes whole", "excluded-whole-by-valueset", unheld, "q", ctx, Invalid, false},
		{"a code one filter selects and another cannot be evaluated for", "regex-then-is-a", held, "b1", ctx, Valid, true},
		{"the same, the filters the other way round", "is-a-then-regex", held, "b1", ctx, Valid, true},
		{"a code the evaluated filter does not select", "is-a-then-regex", held, "a", ctx, Invalid, false},
		{"a code an exclusion with an undecided filter may take away", "excluded-by-undecided-filters", held, "b1", ctx, Valid, true},
		{"a code that exclusion cannot take away", "excluded-by-undecided-filters", held, "a", ctx, Valid, false},
		{"a code of a version not held", "version-not-held", held, "new", ctx, Valid, true},
		{"a code of the version held", "version-not-held", held, "a", ctx, Valid, false},
		{"a code an expansion contains", "expansion-only", held, "a", ctx, Valid, false},
		{"a code an expansion does not contain", "expansion-only", held, "b", ctx, Invalid, false},
		{"a code a page of an expansion does not contain", "expansion-page", held, "b", ctx, Valid, true},
		{"a code its composition selects, not its expansion", "composed-and-expanded", held, "a", ctx, Valid, false},
		{"a code its expansion contains, not its composition", "composed-and-expanded", held, "b", ctx, Invalid, false},
		{"a code an exclusion of a ValueSet including itself may take away", "excluding-self-loop", held, "a", strict, Unresolved, false},
		{"a code element's code a diamond does not hold", "diamond", "", "zz", ctx, Invalid, false},
		{"a code element's code a diamond holds", "diamond", "", "b1", ctx, Valid, false},
		{"a code of a system a diamond of ValueSets excludes whole", "excluding-a-diamond", unheld, "x", ctx, Invalid, false},
		{"a code of ValueSets that include each other", "cycle-a", other, "k", ctx, Valid, false},
		{"another system's code, through the cycle", "cycle-a", held, "k", strict, Unresolved, false},
		{"a code of a nested ValueSet not held", "missing-nested", held, "a", strict, Unresolved, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := r.ResolveCodeInValueSet(tt.ctx, tt.system, tt.code, base+tt.vs, LookupOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if res.Resolution != tt.want || res.Assumed != tt.assumed {
				t.Errorf("resolution %v assumed %v, want %v assumed %v", res.Resolution, res.Assumed, tt.want, tt.assumed)
			}
		})
	}
}

// A ValueSet read with a composition is decided by it, and keeps no expansion; one with only an
// expansion keeps it.
func TestValueSetExpansionKept(t *testing.T) {
	for _, tt := range []struct {
		name, json string
		kept       bool
	}{
		{"composed", `{"resourceType":"ValueSet","url":"u","compose":{"include":[{"system":"s"}]},"expansion":{"contains":[{"system":"s","code":"a"}]}}`, false},
		{"expansion only", `{"resourceType":"ValueSet","url":"u","expansion":{"contains":[{"system":"s","code":"a"}]}}`, true},
	} {
		vs, ok := parseValueSet([]byte(tt.json))
		if !ok {
			t.Fatalf("%s: not read", tt.name)
		}
		if kept := vs.Expansion != nil; kept != tt.kept {
			t.Errorf("%s: expansion kept %v, want %v", tt.name, kept, tt.kept)
		}
	}
}
