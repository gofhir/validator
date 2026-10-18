package terminology

import (
	"context"
	"encoding/json"
	"testing"
)

// Los excludes y los códigos enumerados se deciden por la evidencia que da la edición de cada
// cláusula (valueset.html#compositions: un ValueSet son los códigos que sus includes seleccionan,
// menos los que sus excludes seleccionan; valueset.html#expansion: un código enumerado tiene que ser
// válido en su code system). Un código que la edición lista es evidencia positiva, también en un
// fragment; que no lo liste es evidencia negativa solo si la edición está cargada y completa; si no
// (no cargada, fragment, supplement, externa), la evidencia está ausente. Un exclude quita el
// código cuando lo selecciona, o cuando la evidencia está ausente y toma la edición entera: si el
// código existe en ella, lo quita; si no existe, ningún include lo selecciona. Que la evidencia tenga
// tres valores es diseño del proyecto; el álgebra es de la especificación.
func TestMembershipExcludeEvidence(t *testing.T) {
	const (
		complete = "http://example.org/cs/complete"
		fragment = "http://example.org/cs/fragment"
		supp     = "http://example.org/cs/supplemented"
		unheld   = "http://example.org/cs/unheld"
		snomed   = "http://snomed.info/sct" // externo: solo un servidor lo revisa
		base     = "http://example.org/vs/"
	)
	r := NewRegistry()
	r.codeSystems[complete] = &CodeSystem{URL: complete, Concept: []CodeSystemCode{{Code: "a"}, {Code: "b"}}}
	r.codeSystems[fragment] = &CodeSystem{URL: fragment, Content: "fragment", Concept: []CodeSystemCode{{Code: "a"}, {Code: "b"}}}
	r.codeSystems[supp] = &CodeSystem{URL: supp, Content: "supplement", Concept: []CodeSystemCode{{Code: "a"}}}
	add := func(name string, c Compose) {
		r.valueSets[base+name] = &ValueSet{URL: base + name, Compose: c}
	}
	whole := func(system string) Include { return Include{System: system} }
	listing := func(system string, codes ...string) Include {
		c := Include{System: system}
		for _, code := range codes {
			c.Concept = append(c.Concept, Concept{Code: code})
		}
		return c
	}
	excluding := func(vs string) Include { return Include{ValueSet: []string{base + vs}} }

	// C1: el fragment entero, menos un ValueSet que es el fragment entero menos b.
	add("fragment-but-b", Compose{Include: []Include{whole(fragment)}, Exclude: []Include{listing(fragment, "b")}})
	add("c1", Compose{Include: []Include{whole(fragment)}, Exclude: []Include{excluding("fragment-but-b")}})
	// C2: lo mismo con un sistema externo.
	add("snomed-but-1", Compose{Include: []Include{whole(snomed)}, Exclude: []Include{listing(snomed, "1")}})
	add("c2", Compose{Include: []Include{whole(snomed)}, Exclude: []Include{excluding("snomed-but-1")}})
	// C2b: un código listado del sistema externo, menos ese ValueSet.
	add("c2b", Compose{Include: []Include{listing(snomed, "1", "2")}, Exclude: []Include{excluding("snomed-but-1")}})
	// C3: un código enumerado que el code system cargado y completo no tiene.
	add("c3", Compose{Include: []Include{listing(complete, "a", "z")}})
	add("c3-excluded", Compose{Include: []Include{listing(complete, "z")}, Exclude: []Include{excluding("complete-but-b")}})
	add("complete-but-b", Compose{Include: []Include{whole(complete)}, Exclude: []Include{listing(complete, "b")}})
	// Un code system que no está cargado, entero, menos un ValueSet que lo toma entero menos q.
	add("unheld-but-q", Compose{Include: []Include{whole(unheld)}, Exclude: []Include{listing(unheld, "q")}})
	add("unheld-listed", Compose{Include: []Include{listing(unheld, "q", "r")}, Exclude: []Include{excluding("unheld-but-q")}})
	// Un supplement no tiene los códigos de su code system: evidencia ausente.
	add("supplement-whole", Compose{Include: []Include{whole(supp)}})
	add("supplement-listed", Compose{Include: []Include{listing(supp, "z")}})
	// El mismo ValueSet incluido y excluido: lo que incluye es lo que excluye (el memo distingue si se
	// toma que el código existe).
	add("unheld-all", Compose{Include: []Include{whole(unheld)}})
	add("samex", Compose{Include: []Include{excluding("unheld-all")}, Exclude: []Include{excluding("unheld-all")}})
	// El exclude de una edición cargada y completa quita solo los códigos que tiene.
	add("complete-whole-excluded", Compose{Include: []Include{listing(unheld, "z")}, Exclude: []Include{whole(complete)}})

	ctx := WithStrictMembership(context.Background())
	for _, tt := range []struct {
		name, vs, system, code string
		want                   Resolution
	}{
		{"C1: a code the fragment lacks is excluded if it exists", "c1", fragment, "c", Invalid},
		{"C1: b, which the fragment lists and the excluded ValueSet leaves out", "c1", fragment, "b", Valid},
		{"C2: a code of the external system is excluded if it exists", "c2", snomed, "5", Invalid},
		{"C2: 1, which the excluded ValueSet leaves out, of a system not checked here", "c2", snomed, "1", Unresolved},
		{"C2b: a listed code the excluded ValueSet holds", "c2b", snomed, "2", Invalid},
		{"C2b: the listed code it leaves out", "c2b", snomed, "1", Valid},
		{"C3: a listed code the complete code system has", "c3", complete, "a", Valid},
		{"C3: a listed code the complete code system lacks", "c3", complete, "z", Invalid},
		{"C3: a code it lacks is no member whatever is excluded", "c3-excluded", complete, "z", Invalid},
		{"a listed code the excluded ValueSet leaves out", "unheld-listed", unheld, "q", Valid},
		{"a listed code the excluded ValueSet holds", "unheld-listed", unheld, "r", Invalid},
		{"a supplement's whole system: not known", "supplement-whole", supp, "a", Unresolved},
		{"a code listed of a supplemented system", "supplement-listed", supp, "z", Valid},
		{"a complete code system excluded whole keeps another system's code", "complete-whole-excluded", unheld, "z", Valid},
		{"a ValueSet both included and excluded", "samex", unheld, "q", Invalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := r.ResolveCodeInValueSet(ctx, tt.system, tt.code, base+tt.vs, LookupOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if res.Resolution != tt.want {
				t.Errorf("got %v, want %v", res.Resolution, tt.want)
			}
		})
	}
}

// Los bordes de la regla de evidencia (valueset.html#compositions y #expansion): la edición que
// nombra la cláusula, el contenido de su code system y los ciclos que pasan por un exclude.
func TestMembershipEvidenceEdges(t *testing.T) {
	const (
		versioned  = "http://example.org/cs/versioned"
		fragment   = "http://example.org/cs/fragment"
		notPresent = "http://example.org/cs/not-present"
		example    = "http://example.org/cs/example"
		unheld     = "http://example.org/cs/unheld"
		snomed     = "http://snomed.info/sct" // externo: solo un servidor lo revisa
		base       = "http://example.org/vs/"
	)
	r := NewRegistry()
	// la versión 1 tiene a y z; la 2, la más alta, b y x
	v1 := &CodeSystem{URL: versioned, Version: "1", Concept: []CodeSystemCode{{Code: "a"}, {Code: "z"}}}
	v2 := &CodeSystem{URL: versioned, Version: "2", Concept: []CodeSystemCode{{Code: "b"}, {Code: "x"}}}
	r.codeSystems[versioned] = v2
	r.codeSystemsByVersion[versioned+"|1"] = v1
	r.codeSystemsByVersion[versioned+"|2"] = v2
	r.codeSystems[fragment] = &CodeSystem{URL: fragment, Content: "fragment", Concept: []CodeSystemCode{{Code: "a"}}}
	r.codeSystems[notPresent] = &CodeSystem{URL: notPresent, Content: "not-present"}
	r.codeSystems[example] = &CodeSystem{URL: example, Content: "example", Concept: []CodeSystemCode{{Code: "a"}}}
	// un code system externo cargado como completo: sus códigos igual se deciden en un servidor
	r.codeSystems[snomed] = &CodeSystem{URL: snomed, Concept: []CodeSystemCode{{Code: "a"}}}
	add := func(name string, c Compose) {
		r.valueSets[base+name] = &ValueSet{URL: base + name, Compose: c}
	}
	listing := func(system, version string, codes ...string) Include {
		c := Include{System: system, Version: version}
		for _, code := range codes {
			c.Concept = append(c.Concept, Concept{Code: code})
		}
		return c
	}
	valueSets := func(names ...string) Include {
		var c Include
		for _, name := range names {
			c.ValueSet = append(c.ValueSet, base+name)
		}
		return c
	}
	modified := listing(versioned, "", "z")
	modified.Concept[0].ModifierExtension = []json.RawMessage{json.RawMessage(`{"url":"http://example.org/modifier"}`)}

	add("unheld-all", Compose{Include: []Include{{System: unheld}}})
	add("via", Compose{Include: []Include{valueSets("unheld-all")}})
	// C6: X|2, cargada y completa, no tiene a, así que su exclude no lo quita
	add("other-edition-excluded", Compose{Include: []Include{listing(versioned, "1", "a")}, Exclude: []Include{{System: versioned, Version: "2"}}})
	add("unheld-version", Compose{Include: []Include{listing(versioned, "9", "q")}})
	add("held-older-version", Compose{Include: []Include{listing(versioned, "1", "z")}})
	add("fragment-listed", Compose{Include: []Include{listing(fragment, "", "c")}})
	add("example-listed", Compose{Include: []Include{listing(example, "", "c")}})
	add("not-present-listed", Compose{Include: []Include{listing(notPresent, "", "c")}})
	add("not-present-whole", Compose{Include: []Include{{System: notPresent}}})
	add("not-present-excluded", Compose{Include: []Include{listing(notPresent, "", "c")}, Exclude: []Include{{System: notPresent}}})
	add("external-listed", Compose{Include: []Include{listing(snomed, "", "1")}})
	add("modified", Compose{Include: []Include{modified}})
	add("filtered-out", Compose{Include: []Include{listing(versioned, "", "x")}, Exclude: []Include{{System: versioned, Filter: []Filter{{Property: "code", Op: "regex", Value: ".*"}}}}})
	// Y = C ∪ {x}, C = Y: la menor solución es Y = C = {x}
	add("cycle-root", Compose{Include: []Include{valueSets("cycle-y", "cycle-c")}})
	add("cycle-y", Compose{Include: []Include{valueSets("cycle-c"), listing(versioned, "", "x")}})
	add("cycle-c", Compose{Include: []Include{valueSets("cycle-y")}})
	// R = U − V, V = U − R: la composición no decide si un código de U está en R
	add("cycle-r", Compose{Include: []Include{{System: unheld}}, Exclude: []Include{valueSets("cycle-v")}})
	add("cycle-v", Compose{Include: []Include{{System: unheld}}, Exclude: []Include{valueSets("cycle-r")}})

	ctx := WithStrictMembership(context.Background())
	for _, tt := range []struct {
		name, vs, system, code string
		want                   Resolution
	}{
		{"an unheld system taken whole through an included ValueSet", "via", unheld, "q", Unresolved},
		{"C6: a complete edition excluded whole keeps a code it lacks", "other-edition-excluded", versioned, "a", Valid},
		{"a code listed of an edition not held", "unheld-version", versioned, "q", Valid},
		{"a code listed of a held edition other than the latest", "held-older-version", versioned, "z", Valid},
		{"a code a fragment does not list, listed", "fragment-listed", fragment, "c", Valid},
		{"a code an example code system does not list, listed", "example-listed", example, "c", Valid},
		{"a code of a not-present code system, listed", "not-present-listed", notPresent, "c", Valid},
		{"a not-present code system taken whole", "not-present-whole", notPresent, "c", Unresolved},
		{"a listed code minus its not-present code system whole", "not-present-excluded", notPresent, "c", Invalid},
		{"a code listed of an external system held as complete", "external-listed", snomed, "1", Valid},
		{"a listed code with a modifier extension the code system lacks", "modified", versioned, "z", Unresolved},
		{"a code no include selects, with an exclude not decided", "filtered-out", versioned, "b", Invalid},
		{"a cycle whose root is decided by the fixed point", "cycle-root", versioned, "x", Valid},
		{"a cycle through excludes", "cycle-r", unheld, "q", Unresolved},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := r.ResolveCodeInValueSet(ctx, tt.system, tt.code, base+tt.vs, LookupOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if res.Resolution != tt.want {
				t.Errorf("got %v, want %v", res.Resolution, tt.want)
			}
		})
	}
}
