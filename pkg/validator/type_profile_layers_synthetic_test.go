package validator

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/validator/v2/pkg/walker"
)

// Capas por valor de `type.profile` que ningún tipo de datos del core alcanza (#193, pieza 2, mutaciones 3b, 3h, 16a y
// 16b, 14-11-2026): un tipo de datos con un elemento que tiene un recurso, o que lo tiene solo en uno de sus perfiles,
// con dos perfiles en el elemento que lo usa. HL7 no carga tipos de datos propios, así que los valores esperados salen de
// la especificación y no de sus salidas:
//
//   - ElementDefinition.type.profile (R4 elementdefinition-definitions.html#ElementDefinition.type.profile): «If any
//     profiles are specified, then the content must conform to at least one of them». Un valor que no cumple ninguno es
//     un error (PROFILE_NO_MATCH), y se informa lo que cada perfil le exige;
//   - ElementDefinition.type.profile de un elemento que tiene un recurso: el recurso debe cumplir el perfil que su
//     elemento declara; con dos perfiles del tipo de datos que lo contiene, cada uno declara el suyo, y cada uno se exige
//     por separado (un valor que cumple solo uno cumple el elemento; uno que no cumple ninguno recibe lo de cada uno);
//   - el recurso que un perfil admite y el otro no (un elemento de elección cuyo tipo Observation solo uno permite) se halla
//     y se valida como cualquier recurso.
//
// El tipo MyThing (tipo de datos) tiene `res` (un recurso), `v[x]` (de tipo Observation o string) y `req` (un string). Sus
// perfiles th-a y th-b exigen `req`, ponen un perfil de Observation distinto a `res` (obs-p1 pide `subject`, obs-p2 pide
// `performer`), y permiten `v[x]` solo th-b (en th-a, max 0). El recurso MyHost tiene un elemento `thing` de ese tipo;
// sus perfiles host-ab y host-ba lo declaran con los dos perfiles, en los dos órdenes. Los recursos MyRes y MyResBA los
// declaran en su propia definición, que es la que camina walker.Discover (un perfil no agrega elementos que tengan
// recursos, así que mira la definición del tipo).

const (
	synNS = "http://example.org/fhir/StructureDefinition/"
)

func synSD(id, kind, typ, base, derivation string, elems ...map[string]any) []byte {
	url := synNS + id
	if derivation == "specialization" {
		url = "http://hl7.org/fhir/StructureDefinition/" + id // un tipo propio se carga como los del core (GetSDForResource)
	}
	sd := map[string]any{
		"resourceType": "StructureDefinition", "id": id, "url": url, "name": strings.ReplaceAll(id, "-", "_"),
		"status": "draft", "fhirVersion": "4.0.1", "kind": kind, "abstract": false, "type": typ,
		"baseDefinition": base, "derivation": derivation, "differential": map[string]any{"element": elems},
	}
	if derivation == "specialization" {
		// Un tipo propio trae su snapshot: no hay una definición de la que el generador lo derive elemento por elemento.
		sd["snapshot"] = map[string]any{"element": elems}
	}
	b, err := json.Marshal(sd)
	if err != nil {
		panic(err)
	}
	return b
}

func synEl(path string, kv ...any) map[string]any {
	e := map[string]any{"id": path, "path": path}
	for i := 0; i < len(kv); i += 2 {
		e[kv[i].(string)] = kv[i+1]
	}
	return e
}

// synBase es un elemento de un tipo propio, de cardinalidad mínima 0 y máxima hi, con el origen que un snapshot lleva.
func synBase(path, hi string, types []map[string]any) map[string]any {
	return synEl(path, "min", 0, "max", hi, "type", types, "base", map[string]any{"path": path, "min": 0, "max": hi})
}

func synType(code string, profiles ...string) []map[string]any {
	t := map[string]any{"code": code}
	if len(profiles) > 0 {
		urls := make([]string, len(profiles))
		for i, p := range profiles {
			urls[i] = synNS + p
		}
		t["profile"] = urls
	}
	return []map[string]any{t}
}

func syntheticDefinitions() [][]byte {
	hl7 := "http://hl7.org/fhir/StructureDefinition/"
	thing := func(id string, resProfile string, v map[string]any) []byte {
		return synSD(id, "complex-type", "MyThing", hl7+"MyThing", "constraint",
			synEl("MyThing.res", "type", synType("Observation", resProfile)),
			v,
			synEl("MyThing.req", "min", 1))
	}
	// Un recurso propio con el elemento `thing` de tipo MyThing, con los perfiles que se digan en su definición.
	resource := func(rt string, profiles ...string) []byte {
		return synSD(rt, "resource", rt, hl7+"DomainResource", "specialization",
			synBase(rt, "*", nil),
			synBase(rt+".id", "1", synType("http://hl7.org/fhirpath/System.String")),
			synBase(rt+".meta", "1", synType("Meta")),
			synBase(rt+".implicitRules", "1", synType("uri")),
			synBase(rt+".language", "1", synType("code")),
			synBase(rt+".text", "1", synType("Narrative")),
			synBase(rt+".contained", "*", synType("Resource")),
			synBase(rt+".extension", "*", synType("Extension")),
			synBase(rt+".modifierExtension", "*", synType("Extension")),
			synBase(rt+".thing", "1", synType("MyThing", profiles...)))
	}
	host := func(id string, profiles ...string) []byte {
		return synSD(id, "resource", "MyHost", hl7+"MyHost", "constraint", synEl("MyHost.thing", "type", synType("MyThing", profiles...)))
	}
	return [][]byte{
		synSD("MyThing", "complex-type", "MyThing", hl7+"Element", "specialization",
			synBase("MyThing", "*", nil),
			synBase("MyThing.id", "1", synType("http://hl7.org/fhirpath/System.String")),
			synBase("MyThing.extension", "*", synType("Extension")),
			synBase("MyThing.res", "1", synType("Resource")),
			synBase("MyThing.v[x]", "1", []map[string]any{{"code": "Observation"}, {"code": "string"}}),
			synBase("MyThing.req", "1", synType("string"))),
		thing("th-a", "obs-p1", synEl("MyThing.v[x]", "max", "0")),
		thing("th-b", "obs-p2", synEl("MyThing.v[x]", "type", []map[string]any{{"code": "Observation"}})),
		synSD("obs-p1", "resource", "Observation", hl7+"Observation", "constraint", synEl("Observation.subject", "min", 1)),
		synSD("obs-p2", "resource", "Observation", hl7+"Observation", "constraint", synEl("Observation.performer", "min", 1)),
		resource("MyHost"),
		host("host-ab", "th-a", "th-b"),
		host("host-ba", "th-b", "th-a"),
		resource("MyRes", "th-a", "th-b"),
		resource("MyResBA", "th-b", "th-a"),
	}
}

// synObs es una Observation válida salvo por lo que se le quite o ponga.
func synObs(extra map[string]any) map[string]any {
	o := map[string]any{"resourceType": "Observation", "id": "r", "status": "final", "code": map[string]any{"text": "x"}}
	for k, v := range extra {
		o[k] = v
	}
	return o
}

// synRes es un recurso de tipo resourceType con su `thing`, y el perfil que se diga.
func synRes(resourceType, profile string, thing map[string]any) []byte {
	r := map[string]any{"resourceType": resourceType, "id": "m1", "thing": thing}
	if profile != "" {
		r["meta"] = map[string]any{"profile": []string{synNS + profile}}
	}
	b, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return b
}

// checkSynthetic valida el recurso de tipo resourceType con su thing, en R4, R4B y R5, y compara sus errores, como
// «ID @ ubicación», con want (en el que «R.» es el tipo del recurso).
func checkSynthetic(t *testing.T, resourceType, profile string, thing map[string]any, want []string) {
	t.Helper()
	for _, version := range layerVersions {
		v := syntheticValidator(t, version)
		res, err := v.Validate(context.Background(), synRes(resourceType, profile, thing))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, is := range res.Issues {
			if is.Severity == "error" || is.Severity == "fatal" {
				got = append(got, is.MessageID+" @ "+strings.Join(is.Expression, ","))
			}
		}
		slices.Sort(got)
		exp := make([]string, 0, len(want))
		for _, w := range want {
			exp = append(exp, strings.ReplaceAll(w, "R.", resourceType+"."))
		}
		slices.Sort(exp)
		if !slices.Equal(got, exp) {
			t.Errorf("%s: got %q, want %q", version, got, exp)
		}
	}
}

const (
	synNoMatch = "PROFILE_NO_MATCH @ R.thing"
	synReqMin  = "CARDINALITY_MIN @ R.thing.req"
)

// Mutaciones 3b, 16a y 16b: los grupos de holders de cada capa, por separado y de las dos capas.
func TestLayersSyntheticDatatypeThatHoldsAResource(t *testing.T) {
	subject := map[string]any{"subject": map[string]any{"display": "s"}}
	performer := map[string]any{"performer": []any{map[string]any{"display": "p"}}}
	both := map[string]any{"subject": subject["subject"], "performer": performer["performer"]}
	for _, tt := range []struct {
		name  string
		thing map[string]any
		want  []string
	}{
		// Un valor que cumple uno de los dos perfiles cumple el elemento: no hay informe, y el recurso que tiene solo
		// debe cumplir el perfil de la capa que se elige.
		{"both layers met: the resource meets each holder profile", map[string]any{"req": "r", "res": synObs(both)}, nil},
		{"a layer met alone: the resource meets only its holder profile", map[string]any{"req": "r", "res": synObs(subject)}, nil},
		{"the other layer met alone", map[string]any{"req": "r", "res": synObs(performer)}, nil},
		// Un valor que no cumple ninguno (le falta req): cada capa le exige lo suyo al recurso, por separado, sea cual sea
		// el orden de los perfiles.
		{"no layer met: the resource lacks what the second holder profile asks", map[string]any{"res": synObs(subject)},
			[]string{synNoMatch, synReqMin, "CARDINALITY_MIN @ R.thing.res.performer"}},
		{"no layer met: the resource lacks what the first holder profile asks", map[string]any{"res": synObs(performer)},
			[]string{synNoMatch, synReqMin, "CARDINALITY_MIN @ R.thing.res.subject"}},
		{"no layer met: the resource lacks both", map[string]any{"res": synObs(nil)},
			[]string{synNoMatch, synReqMin, "CARDINALITY_MIN @ R.thing.res.performer", "CARDINALITY_MIN @ R.thing.res.subject"}},
	} {
		for _, profile := range []string{"host-ab", "host-ba"} {
			t.Run(profile+"/"+tt.name, func(t *testing.T) {
				checkSynthetic(t, "MyHost", profile, tt.thing, tt.want)
			})
		}
	}
}

// Mutación 3h: un recurso que solo una capa admite se halla y se valida. El perfil th-a no permite v[x] (max 0, y el elemento no
// tiene recursos), el th-b lo permite como Observation; el valor no cumple ninguna (le falta req), y la Observation de th-b,
// que declara el perfil obs-p1 y no tiene `subject`, se valida contra él (solo el paso por los recursos lo hace). El max
// de un elemento de elección se ubica en su ruta de definición, como la fase de cardinalidad lo hace siempre.
func TestLayersSyntheticDiscoverFindsAResourceOnlyTheSecondLayerHolds(t *testing.T) {
	held := synObs(map[string]any{"meta": map[string]any{"profile": []string{synNS + "obs-p1"}}})
	thing := map[string]any{"vObservation": held}
	want := []string{synNoMatch, synReqMin, "CARDINALITY_MAX @ R.thing.v[x]", "CARDINALITY_MIN @ R.thing.vObservation.subject"}
	for _, rt := range []string{"MyRes", "MyResBA"} {
		t.Run(rt, func(t *testing.T) { checkSynthetic(t, rt, "", thing, want) })
	}
}

// Un recurso que dos capas elegidas tienen en el mismo elemento se halla una vez (walker.Discover): se valida una vez, y
// cada informe sale una vez. Por ahora lo halla dos veces, y sale cada informe dos veces: un recurso con los perfiles de
// tipo en su propia definición (los de un perfil no los camina Discover) y un tipo de datos con un elemento que tiene un
// recurso no existen en el core, así que no se alcanza con R4, R4B ni R5; se informó el 14-11-2026 y espera su arreglo.
func TestLayersSyntheticDiscoverFindsAResourceBothLayersHoldOnce(t *testing.T) {
	for _, rt := range []string{"MyRes", "MyResBA"} {
		t.Run(rt, func(t *testing.T) {
			checkSynthetic(t, rt, "", map[string]any{"res": synObs(map[string]any{"subject": map[string]any{"display": "s"}})},
				[]string{synNoMatch, synReqMin, "CARDINALITY_MIN @ R.thing.res.performer"})
		})
	}
}

func syntheticValidator(t testing.TB, version string) *Validator {
	t.Helper()
	defs := syntheticDefinitions()
	for i := range defs {
		defs[i] = []byte(strings.ReplaceAll(string(defs[i]), "4.0.1", version))
	}
	v, err := New(WithVersion(version), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Un recurso que alcanzan dos recorridos de capas distintas, por invocaciones distintas de walker.Discover (revisión, ronda
// 1): MyWrap tiene dos perfiles (wrap-a y wrap-b) que declaran, cada uno, otro perfil de MyThing para `inner`, y MyThing tiene
// `res`, un recurso. El recurso es uno solo en la instancia: ElementDefinition.type.profile (R4
// elementdefinition-definitions.html#ElementDefinition.type.profile) dice que el contenido debe cumplir al menos uno de los
// perfiles, no que haya un contenido por perfil, y una instancia JSON tiene un solo valor en `res`. Se descubre una vez, y
// en el mismo lugar, aunque cada capa de MyWrap baje por su propio camino.
func TestLayersDiscoverResourceReachedThroughNestedLayersOnce(t *testing.T) {
	hl7 := "http://hl7.org/fhir/StructureDefinition/"
	defs := append(syntheticDefinitions(),
		synSD("MyWrap", "complex-type", "MyWrap", hl7+"Element", "specialization",
			synBase("MyWrap", "*", nil), synBase("MyWrap.inner", "1", synType("MyThing"))),
		synSD("wrap-a", "complex-type", "MyWrap", hl7+"MyWrap", "constraint", synEl("MyWrap.inner", "type", synType("MyThing", "th-a"))),
		synSD("wrap-b", "complex-type", "MyWrap", hl7+"MyWrap", "constraint", synEl("MyWrap.inner", "type", synType("MyThing", "th-b"))),
		synSD("MyNested", "resource", "MyNested", hl7+"DomainResource", "specialization",
			synBase("MyNested", "*", nil), synBase("MyNested.wrap", "1", synType("MyWrap", "wrap-a", "wrap-b"))))
	v, err := New(WithVersion("4.0.1"), WithConformanceResources(defs))
	if err != nil {
		t.Fatal(err)
	}
	obs := synObs(nil)
	data := map[string]any{"resourceType": "MyNested", "wrap": map[string]any{"inner": map[string]any{"res": obs}}}
	d := walker.Discover(context.Background(), v.registry, v.registry.GetByType("MyNested"), data, "MyNested")
	if len(d.Resources) != 1 {
		t.Fatalf("se descubrieron %d recursos, quiero 1", len(d.Resources))
	}
	if got := d.Resources[0].Path; got != "MyNested.wrap.inner.res" {
		t.Errorf("ubicación %q, quiero MyNested.wrap.inner.res", got)
	}
}
