package terminology

import (
	"context"
	"fmt"
	"testing"
)

// Los ValueSets que se incluyen unos a otros se deciden como la menor solución de sus composiciones
// (valueset.html#compositions: un ValueSet son los códigos que sus includes seleccionan, menos los
// que sus excludes seleccionan), sin depender del orden en que se recorren: dos ValueSets que se
// incluyen mutuamente, y uno de ellos lista el código, tienen ambos el código. La lógica de tres
// valores (lo no decidido) es diseño del proyecto, no de la especificación.
func TestMembershipCycleOrder(t *testing.T) {
	const (
		other = "http://example.org/cs/other"
		base  = "http://example.org/vs/"
	)
	r := NewRegistry()
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}, {Code: "k"}}}
	add := func(name string, c Compose) {
		r.valueSets[base+name] = &ValueSet{URL: base + name, Compose: c}
	}
	// ca = cb ∪ {a}; cb = ca: la menor solución es ca = cb = {a}.
	add("ca", Compose{Include: []Include{{ValueSet: []string{base + "cb"}}, {System: other, Concept: []Concept{{Code: "a"}}}}})
	add("cb", Compose{Include: []Include{{ValueSet: []string{base + "ca"}}}})
	// Los mismos ValueSets con los nombres cambiados: el resultado no puede depender del nombre.
	add("cx", Compose{Include: []Include{{ValueSet: []string{base + "cy"}}}})
	add("cy", Compose{Include: []Include{{ValueSet: []string{base + "cx"}}, {System: other, Concept: []Concept{{Code: "a"}}}}})
	// m1 y m2 se incluyen mutuamente; m2 incluye además uno que no está cargado: lo que no se sabe de
	// él sube por el ciclo.
	add("m1", Compose{Include: []Include{{ValueSet: []string{base + "m2"}}, {System: other, Concept: []Concept{{Code: "a"}}}}})
	add("m2", Compose{Include: []Include{{ValueSet: []string{base + "m1"}}, {ValueSet: []string{base + "not-held"}}}})
	// Un ValueSet que solo se incluye a sí mismo: la menor solución es vacía.
	add("self", Compose{Include: []Include{{ValueSet: []string{base + "self"}}}})
	// r1 = ca − cb y r3 = cb − ca: los dos vacíos.
	add("r1", Compose{Include: []Include{{ValueSet: []string{base + "ca"}}}, Exclude: []Include{{ValueSet: []string{base + "cb"}}}})
	add("r3", Compose{Include: []Include{{ValueSet: []string{base + "cb"}}}, Exclude: []Include{{ValueSet: []string{base + "ca"}}}})
	add("r1y", Compose{Include: []Include{{ValueSet: []string{base + "cy"}}}, Exclude: []Include{{ValueSet: []string{base + "cx"}}}})
	add("r3y", Compose{Include: []Include{{ValueSet: []string{base + "cx"}}}, Exclude: []Include{{ValueSet: []string{base + "cy"}}}})

	ctx := WithStrictMembership(context.Background())
	for _, tt := range []struct {
		vs, code string
		want     Resolution
	}{
		{"ca", "a", Valid},
		{"cb", "a", Valid},
		{"cx", "a", Valid},
		{"cy", "a", Valid},
		{"r1", "a", Invalid},
		{"r3", "a", Invalid},
		{"r1y", "a", Invalid},
		{"r3y", "a", Invalid},
		// Un código que ningún ValueSet del ciclo lista: la menor solución no lo tiene, pero la lógica
		// de tres valores lo deja sin decidir, de modo conservador.
		{"cb", "k", Unresolved},
		{"m1", "a", Valid},
		{"m1", "z", Unresolved},
	} {
		// El mismo código como Coding y como el de un elemento code, cuyo sistema implica el ValueSet:
		// los sistemas de un ciclo son la unión de los de sus ValueSets.
		for _, system := range []string{other, ""} {
			t.Run(tt.vs+"/"+tt.code+"/"+system, func(t *testing.T) {
				res, err := r.ResolveCodeInValueSet(ctx, system, tt.code, base+tt.vs, LookupOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if res.Resolution != tt.want {
					t.Errorf("got %v, want %v", res.Resolution, tt.want)
				}
			})
		}
	}
}

// Una cadena de ValueSets, cada uno incluyendo el siguiente, se decide a cualquier profundidad: el
// último lista el código, así que todos lo tienen (valueset.html#compositions).
func TestMembershipDeepChain(t *testing.T) {
	const (
		other  = "http://example.org/cs/other"
		base   = "http://example.org/vs/"
		levels = 40
	)
	r := NewRegistry()
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}}}
	for i := range levels {
		c := Compose{Include: []Include{{ValueSet: []string{fmt.Sprintf("%sd%d", base, i+1)}}}}
		if i == levels-1 {
			c = Compose{Include: []Include{{System: other, Concept: []Concept{{Code: "a"}}}}}
		}
		url := fmt.Sprintf("%sd%d", base, i)
		r.valueSets[url] = &ValueSet{URL: url, Compose: c}
	}
	for _, system := range []string{other, ""} { // como Coding y como el de un elemento code
		res, err := r.ResolveCodeInValueSet(WithStrictMembership(context.Background()), system, "a", base+"d0", LookupOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Resolution != Valid {
			t.Errorf("system %q: got %v, want valid", system, res.Resolution)
		}
	}
}

// Treinta niveles de dos ValueSets que incluyen cada uno a los dos del nivel siguiente, el último
// listando el código y volviendo a la raíz: cada ValueSet se decide una vez en el recorrido, no por
// cada camino que llega a él (2^30), y el ciclo de vuelta no cambia que la raíz tenga el código. Un
// código que ninguno lista obliga a recorrerlos todos; por el ciclo queda sin decidir.
func TestMembershipCyclicDiamond(t *testing.T) {
	const (
		other  = "http://example.org/cs/other"
		base   = "http://example.org/vs/"
		levels = 30
	)
	r := NewRegistry()
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}}}
	url := func(side string, i int) string {
		if i == levels {
			return base + "end"
		}
		return fmt.Sprintf("%s%s%d", base, side, i)
	}
	r.valueSets[base+"end"] = &ValueSet{URL: base + "end", Compose: Compose{Include: []Include{
		{System: other, Concept: []Concept{{Code: "a"}}}, {ValueSet: []string{url("v", 0)}},
	}}}
	for i := range levels {
		for _, side := range []string{"v", "w"} {
			r.valueSets[url(side, i)] = &ValueSet{URL: url(side, i), Compose: Compose{Include: []Include{
				{ValueSet: []string{url("v", i+1)}}, {ValueSet: []string{url("w", i+1)}},
			}}}
		}
	}
	for code, want := range map[string]Resolution{"a": Valid, "z": Unresolved} {
		res, err := r.ResolveCodeInValueSet(WithStrictMembership(context.Background()), other, code, url("v", 0), LookupOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Resolution != want {
			t.Errorf("%s: got %v, want %v", code, res.Resolution, want)
		}
	}
}

// Un ValueSet que solo se incluye a sí mismo no tiene sistemas ni códigos (la menor solución es
// vacía): preguntado como el código de un elemento code, que no es de ningún sistema que incluya, no
// es miembro; preguntado como Coding, la lógica de tres valores lo deja sin decidir, de modo
// conservador. Las dos respuestas quedan fijadas.
func TestMembershipSelfOnly(t *testing.T) {
	const (
		other = "http://example.org/cs/other"
		base  = "http://example.org/vs/"
	)
	r := NewRegistry()
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}}}
	r.valueSets[base+"self"] = &ValueSet{URL: base + "self", Compose: Compose{Include: []Include{{ValueSet: []string{base + "self"}}}}}
	ctx := WithStrictMembership(context.Background())
	for system, want := range map[string]Resolution{"": Invalid, other: Unresolved} {
		res, err := r.ResolveCodeInValueSet(ctx, system, "a", base+"self", LookupOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Resolution != want {
			t.Errorf("system %q: got %v, want %v", system, res.Resolution, want)
		}
	}
}

// Lo que no se sabe de un ValueSet que no está cargado, dos niveles más abajo, llega a la raíz para el
// código de un elemento code: los sistemas que se conocen (aquí excluido entero) no deciden que no sea
// miembro, porque el que falta puede incluir otros.
func TestMembershipUnknownSystemsPropagate(t *testing.T) {
	const (
		other = "http://example.org/cs/other"
		base  = "http://example.org/vs/"
	)
	r := NewRegistry()
	r.codeSystems[other] = &CodeSystem{URL: other, Concept: []CodeSystemCode{{Code: "a"}}}
	add := func(name string, c Compose) {
		r.valueSets[base+name] = &ValueSet{URL: base + name, Compose: c}
	}
	add("root", Compose{Include: []Include{{ValueSet: []string{base + "n1"}}}, Exclude: []Include{{System: other}}})
	add("n1", Compose{Include: []Include{{System: other, Concept: []Concept{{Code: "a"}}}, {ValueSet: []string{base + "n2"}}}})
	add("n2", Compose{Include: []Include{{ValueSet: []string{base + "not-held"}}}})
	res, err := r.ResolveCodeInValueSet(WithStrictMembership(context.Background()), "", "z", base+"root", LookupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != Unresolved {
		t.Errorf("got %v, want unresolved", res.Resolution)
	}
}
