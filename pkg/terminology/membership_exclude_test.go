package terminology

import (
	"context"
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
