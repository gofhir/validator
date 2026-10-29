package registry

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// Definiciones de TestChoiceType. Las definiciones restrictedExt y noBaseExt restringen
// Extension.value[x] a string, con y sin base (un snapshot sin base viola sdf-8b, pero HL7 igual decide
// por el tipo del objeto). La extensión nestedNoBaseExt restringe a string el value[x] de su
// subextensión s, sin base. El modelo lógico cdaObs se llama Observation, con type URL como los de CDA,
// y su value[x] declara Coding, que el Observation de FHIR no declara; cdaObsNoBase es el mismo sin base.
// El modelo lógico cdaDoc tiene Doc.obs del tipo cdaObs (por URL), y cdaDocProfile restringe
// Doc.obs.value[x]: su snapshot generado lo despliega con base.path Observation.value[x], que no está en
// su cadena (DocProfile, Doc, Base). El perfil nestedItem restringe a Coding el answerOption.value[x] de
// item.item, que es un contentReference a Questionnaire.item: solo base.path
// (Questionnaire.item.answerOption.value[x]) lleva a su base, porque el ancestro no tiene tipo.
const (
	restrictedExt = `{"resourceType":"StructureDefinition","id":"restricted","url":"http://example.org/StructureDefinition/restricted","name":"Restricted","status":"active","kind":"complex-type","abstract":false,"type":"Extension","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint","snapshot":{"element":[
 {"id":"Extension","path":"Extension","base":{"path":"Extension","min":0,"max":"*"}},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","base":{"path":"Extension.value[x]","min":0,"max":"1"},"type":[{"code":"string"}]}]}}`
	noBaseExt = `{"resourceType":"StructureDefinition","id":"nobase","url":"http://example.org/StructureDefinition/nobase","name":"NoBase","status":"active","kind":"complex-type","abstract":false,"type":"Extension","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint","snapshot":{"element":[
 {"id":"Extension","path":"Extension"},
 {"id":"Extension.value[x]","path":"Extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	nestedNoBaseExt = `{"resourceType":"StructureDefinition","id":"nested-nobase","url":"http://example.org/StructureDefinition/nested-nobase","name":"NestedNoBase","status":"active","kind":"complex-type","abstract":false,"type":"Extension","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Extension","derivation":"constraint","snapshot":{"element":[
 {"id":"Extension","path":"Extension"},
 {"id":"Extension.extension","path":"Extension.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"type":[{"code":"Extension"}]},
 {"id":"Extension.extension:s","path":"Extension.extension","sliceName":"s","type":[{"code":"Extension"}]},
 {"id":"Extension.extension:s.value[x]","path":"Extension.extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]},
 {"id":"Extension.value[x]","path":"Extension.value[x]","max":"0"}]}}`
	cdaObs = `{"resourceType":"StructureDefinition","id":"cda-obs","url":"http://example.org/cda/StructureDefinition/Observation","name":"Observation","status":"active","kind":"logical","abstract":false,"type":"http://example.org/cda/StructureDefinition/Observation","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Observation","path":"Observation","base":{"path":"Observation","min":0,"max":"*"}},
 {"id":"Observation.value[x]","path":"Observation.value[x]","min":0,"max":"1","base":{"path":"Observation.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"string"}]}]}}`
	cdaObsNoBase = `{"resourceType":"StructureDefinition","id":"cda-obs-nobase","url":"http://example.org/cda/StructureDefinition/ObservationNoBase","name":"Observation","status":"active","kind":"logical","abstract":false,"type":"http://example.org/cda/StructureDefinition/ObservationNoBase","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Observation","path":"Observation"},
 {"id":"Observation.value[x]","path":"Observation.value[x]","type":[{"code":"Coding"},{"code":"string"}]}]}}`
	cdaDoc = `{"resourceType":"StructureDefinition","id":"cda-doc","url":"http://example.org/cda/StructureDefinition/Doc","name":"Doc","status":"active","kind":"logical","abstract":false,"type":"http://example.org/cda/StructureDefinition/Doc","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Doc","path":"Doc","base":{"path":"Doc","min":0,"max":"*"}},
 {"id":"Doc.obs","path":"Doc.obs","min":0,"max":"*","base":{"path":"Doc.obs","min":0,"max":"*"},"type":[{"code":"http://example.org/cda/StructureDefinition/Observation"}]}]}}`
	nestedItem = `{"resourceType":"StructureDefinition","id":"nested-item","url":"http://example.org/StructureDefinition/nested-item","name":"NestedItem","status":"active","kind":"resource","abstract":false,"type":"Questionnaire","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Questionnaire","derivation":"constraint","differential":{"element":[
 {"id":"Questionnaire","path":"Questionnaire"},
 {"id":"Questionnaire.item.item.answerOption.value[x]","path":"Questionnaire.item.item.answerOption.value[x]","type":[{"code":"Coding"}]}]}}`
	cdaDocProfile = `{"resourceType":"StructureDefinition","id":"cda-doc-profile","url":"http://example.org/cda/StructureDefinition/DocProfile","name":"DocProfile","status":"active","kind":"logical","abstract":false,"type":"http://example.org/cda/StructureDefinition/Doc","baseDefinition":"http://example.org/cda/StructureDefinition/Doc","derivation":"constraint","differential":{"element":[
 {"id":"Doc","path":"Doc"},
 {"id":"Doc.obs.value[x]","path":"Doc.obs.value[x]","min":1}]}}`
)

// Una propiedad de un elemento de elección es un valor si nombra, con la inicial en mayúscula, un tipo
// que declara el elemento base (formats.html#choice; ElementDefinition.base): uno que el perfil no
// permite es un valor del tipo equivocado, y uno que la base no declara no es un valor (HL7 6.10.2: en
// answerOption, valueDateTime es una propiedad desconocida y value[x] falta). La base se busca por la
// cadena de definiciones (baseDefinition), no por el nombre del tipo: un modelo lógico es su propia
// base aunque se llame como un recurso.
func TestChoiceType(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		r := sharedVersion(t, version).Clone()
		resources := make([][]byte, 0, 8)
		for _, sd := range []string{restrictedExt, noBaseExt, nestedNoBaseExt, cdaObs, cdaObsNoBase, cdaDoc, cdaDocProfile, nestedItem} {
			resources = append(resources, []byte(sd))
		}
		pkg, err := loader.NewLoader("").LoadFromResources(resources)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
		node := func(url, id string) *ElementNode {
			sd := r.GetByURL(url)
			if sd == nil {
				t.Fatalf("%s: %s not loaded", version, url)
			}
			n := sd.Tree().ByID(id)
			if n == nil {
				t.Fatalf("%s: %s has no %s", version, url, id)
			}
			return n
		}
		const core, ex, cda = "http://hl7.org/fhir/StructureDefinition/", "http://example.org/StructureDefinition/", "http://example.org/cda/StructureDefinition/"
		obs := node(core+"Observation", "Observation.value[x]")
		answer := node(core+"Questionnaire", "Questionnaire.item.answerOption.value[x]")
		status := node(core+"Observation", "Observation.status")
		restricted := node(ex+"restricted", "Extension.value[x]")
		noBase := node(ex+"nobase", "Extension.value[x]")
		nested := node(ex+"nested-nobase", "Extension.extension:s.value[x]")
		logical := node(cda+"Observation", "Observation.value[x]")
		logicalNoBase := node(cda+"ObservationNoBase", "Observation.value[x]")
		if err := r.EnsureSnapshot(context.Background(), r.GetByURL(cda+"DocProfile")); err != nil {
			t.Fatal(err)
		}
		unrolled := node(cda+"DocProfile", "Doc.obs.value[x]")
		if err := r.EnsureSnapshot(context.Background(), r.GetByURL(ex+"nested-item")); err != nil {
			t.Fatal(err)
		}
		item := node(ex+"nested-item", "Questionnaire.item.item.answerOption.value[x]")
		for _, tt := range []struct {
			node      *ElementNode
			key, want string
		}{
			{obs, "valueQuantity", "Quantity"},
			{obs, "valueBoolean", "boolean"},
			{obs, "valueDateTime", "dateTime"},
			{obs, "value", ""},
			{obs, "valuestring", ""},  // the type's first letter is capitalized
			{obs, "valueSTRING", ""},  // and nothing else changes
			{obs, "valueFoo", ""},     // no such type
			{obs, "valueRequest", ""}, // not a type value[x] declares: a logical model,
			{obs, "valuePatient", ""}, // a resource,
			{obs, "valueElement", ""}, // an abstract type
			{obs, "other", ""},
			{answer, "valueDate", "date"},
			{answer, "valueDateTime", ""},
			{restricted, "valueString", "string"},
			{restricted, "valueUri", "uri"}, // the profile does not allow it; the base does
			{noBase, "valueUri", "uri"},     // the base by baseDefinition, with no base.path
			{nested, "valueString", "string"},
			{nested, "valueCoding", "Coding"}, // a sub-extension is an Extension
			{logical, "valueCoding", "Coding"},
			{logical, "valueString", "string"},
			{logical, "valueQuantity", ""}, // FHIR's Observation declares it; the model does not
			{logicalNoBase, "valueCoding", "Coding"},
			{logicalNoBase, "valueQuantity", ""},
			{unrolled, "valueCoding", "Coding"}, // the model's Observation, by its URL
			{unrolled, "valueQuantity", ""},     // not FHIR's, by its name
			{item, "valueCoding", "Coding"},
			{item, "valueDate", "date"}, // the profile does not allow it; base.path does
			{item, "valueDateTime", ""},
			{status, "statusString", ""}, // not a choice element
		} {
			if got := r.ChoiceType(tt.node, tt.key); got != tt.want {
				t.Errorf("%s: ChoiceType(%s, %q) = %q, want %q", version, tt.node.Def.ID, tt.key, got, tt.want)
			}
		}
	}
}

// La base de un elemento depende de las definiciones cargadas: un perfil cargado antes que la
// definición de la que deriva toma sus tipos propios, y los de la base cuando esta se carga.
func TestChoiceTypeAfterLoad(t *testing.T) {
	const (
		model = `{"resourceType":"StructureDefinition","id":"m","url":"http://example.org/StructureDefinition/Model","name":"Model","status":"active","kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/Model","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Model","path":"Model","base":{"path":"Model","min":0,"max":"*"}},
 {"id":"Model.value[x]","path":"Model.value[x]","min":0,"max":"1","base":{"path":"Model.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"string"}]}]}}`
		profile = `{"resourceType":"StructureDefinition","id":"p","url":"http://example.org/StructureDefinition/ModelProfile","name":"ModelProfile","status":"active","kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/Model","baseDefinition":"http://example.org/StructureDefinition/Model","derivation":"constraint","snapshot":{"element":[
 {"id":"Model","path":"Model","base":{"path":"Model","min":0,"max":"*"}},
 {"id":"Model.value[x]","path":"Model.value[x]","min":0,"max":"1","base":{"path":"Model.value[x]","min":0,"max":"1"},"type":[{"code":"string"}]}]}}`
	)
	r := sharedVersion(t, "4.0.1").Clone()
	load := func(sd string) {
		pkg, err := loader.NewLoader("").LoadFromResources([][]byte{[]byte(sd)})
		if err != nil {
			t.Fatal(err)
		}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
	}
	load(profile)
	node := r.GetByURL("http://example.org/StructureDefinition/ModelProfile").Tree().ByID("Model.value[x]")
	if got := r.ChoiceType(node, "valueCoding"); got != "" {
		t.Errorf("without the model: ChoiceType(valueCoding) = %q, want \"\"", got)
	}
	load(model)
	if got := r.ChoiceType(node, "valueCoding"); got != "Coding" {
		t.Errorf("with the model: ChoiceType(valueCoding) = %q, want Coding", got)
	}
}

// mapResolver resuelve definiciones desde un mapa por URL, como un servidor de perfiles.
type mapResolver map[string]string

func (m mapResolver) ResolveProfile(_ context.Context, url, _ string) ([]byte, error) {
	if sd, ok := m[url]; ok {
		return []byte(sd), nil
	}
	return nil, nil
}

// Un perfil de la cadena resuelto a demanda (ResolveByCanonical) cambia la base de los elementos que
// derivan de él, aunque no defina un tipo: el modelo lógico M declara Coding en value[x], Q lo
// restringe y P restringe Q. Antes de resolver Q, P no llega a M; después, sí.
func TestChoiceTypeAfterResolve(t *testing.T) {
	const (
		model = `{"resourceType":"StructureDefinition","id":"m","url":"http://example.org/StructureDefinition/M","name":"M","status":"active","kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/M","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Model","path":"Model","base":{"path":"Model","min":0,"max":"*"}},
 {"id":"Model.value[x]","path":"Model.value[x]","min":0,"max":"1","base":{"path":"Model.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"string"}]}]}}`
		q = `{"resourceType":"StructureDefinition","id":"q","url":"http://example.org/StructureDefinition/Q","name":"Q","status":"active","kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/M","baseDefinition":"http://example.org/StructureDefinition/M","derivation":"constraint","snapshot":{"element":[
 {"id":"Model","path":"Model","base":{"path":"Model","min":0,"max":"*"}},
 {"id":"Model.value[x]","path":"Model.value[x]","min":0,"max":"1","base":{"path":"Model.value[x]","min":0,"max":"1"},"type":[{"code":"string"}]}]}}`
		p = `{"resourceType":"StructureDefinition","id":"p","url":"http://example.org/StructureDefinition/P","name":"P","status":"active","kind":"logical","abstract":false,"type":"http://example.org/StructureDefinition/M","baseDefinition":"http://example.org/StructureDefinition/Q","derivation":"constraint","snapshot":{"element":[
 {"id":"Model","path":"Model","base":{"path":"Model","min":0,"max":"*"}},
 {"id":"Model.value[x]","path":"Model.value[x]","min":1,"max":"1","base":{"path":"Model.value[x]","min":0,"max":"1"},"type":[{"code":"string"}]}]}}`
	)
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		r := sharedVersion(t, version).Clone()
		pkg, err := loader.NewLoader("").LoadFromResources([][]byte{[]byte(model), []byte(p)})
		if err != nil {
			t.Fatal(err)
		}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
		r.SetResolver(mapResolver{"http://example.org/StructureDefinition/Q": q})
		node := r.GetByURL("http://example.org/StructureDefinition/P").Tree().ByID("Model.value[x]")
		if got := r.ChoiceType(node, "valueCoding"); got != "" {
			t.Errorf("%s: without Q: ChoiceType(valueCoding) = %q, want \"\"", version, got)
		}
		if r.ResolveByCanonical(context.Background(), "http://example.org/StructureDefinition/Q", "") == nil {
			t.Fatalf("%s: Q not resolved", version)
		}
		if got := r.ChoiceType(node, "valueCoding"); got != "Coding" {
			t.Errorf("%s: with Q: ChoiceType(valueCoding) = %q, want Coding", version, got)
		}
	}
}

// Una base calculada con definiciones que cambiaron mientras tanto (otra goroutine cargó o resolvió
// una) no se usa: su generación ya no es la actual. Se simula guardando la entrada de la generación
// anterior, como la dejaría una búsqueda que terminó después de la carga.
func TestChoiceTypeStaleEntry(t *testing.T) {
	r := sharedVersion(t, "4.0.1").Clone()
	node := r.GetByType("Observation").Tree().ByID("Observation.value[x]")
	r.choiceBases.Store(node, choiceBaseEntry{gen: r.choiceGen.Load() - 1})
	if got := r.ChoiceType(node, "valueQuantity"); got != "Quantity" {
		t.Errorf("ChoiceType(valueQuantity) = %q, want Quantity", got)
	}
}

// Un nodo armado a mano no tiene definición: su base son sus tipos y no se guarda, para que la caché no
// crezca con cada uno.
func TestChoiceTypeHandBuiltNode(t *testing.T) {
	r := sharedVersion(t, "4.0.1").Clone()
	def := &ElementDefinition{ID: "Extension.value[x]", Path: "Extension.value[x]", Type: []Type{{Code: "string"}}}
	for range 3 {
		if got := r.ChoiceType(&ElementNode{Def: def}, "valueString"); got != "string" {
			t.Fatalf("ChoiceType(valueString) = %q, want string", got)
		}
	}
	n := 0
	r.choiceBases.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Errorf("choiceBases holds %d entries for hand-built nodes, want 0", n)
	}
}

// base.path es una ruta, no una definición: dos modelos lógicos con la raíz Observation la comparten.
// El perfil P de A despliega A.obs, del modelo B, y su elemento Observation.obs.value[x] tiene
// base.path Observation.value[x], que también es un elemento de A. La base es la de B, el tipo de obs.
// En el modelo T, T.item.item es un contentReference a T.item: el perfil Q de Doc despliega
// Doc.t.item.item.value[x] con base.path T.item.value[x], que la ruta relativa (T.item.item.value[x]) no
// encuentra.
func TestChoiceTypeAmbiguousBasePath(t *testing.T) {
	const (
		a = `{"resourceType":"StructureDefinition","id":"a","url":"http://example.org/A","name":"A","status":"active","kind":"logical","abstract":false,"type":"http://example.org/A","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Observation","path":"Observation","base":{"path":"Observation","min":0,"max":"*"}},
 {"id":"Observation.value[x]","path":"Observation.value[x]","min":0,"max":"1","base":{"path":"Observation.value[x]","min":0,"max":"1"},"type":[{"code":"boolean"}]},
 {"id":"Observation.obs","path":"Observation.obs","min":0,"max":"1","base":{"path":"Observation.obs","min":0,"max":"1"},"type":[{"code":"http://example.org/B"}]}]}}`
		b = `{"resourceType":"StructureDefinition","id":"b","url":"http://example.org/B","name":"B","status":"active","kind":"logical","abstract":false,"type":"http://example.org/B","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Observation","path":"Observation","base":{"path":"Observation","min":0,"max":"*"}},
 {"id":"Observation.value[x]","path":"Observation.value[x]","min":0,"max":"1","base":{"path":"Observation.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"string"}]}]}}`
		p = `{"resourceType":"StructureDefinition","id":"p","url":"http://example.org/P","name":"P","status":"active","kind":"logical","abstract":false,"type":"http://example.org/A","baseDefinition":"http://example.org/A","derivation":"constraint","snapshot":{"element":[
 {"id":"Observation","path":"Observation","base":{"path":"Observation","min":0,"max":"*"}},
 {"id":"Observation.value[x]","path":"Observation.value[x]","min":0,"max":"1","base":{"path":"Observation.value[x]","min":0,"max":"1"},"type":[{"code":"boolean"}]},
 {"id":"Observation.obs","path":"Observation.obs","min":0,"max":"1","base":{"path":"Observation.obs","min":0,"max":"1"},"type":[{"code":"http://example.org/B"}]},
 {"id":"Observation.obs.value[x]","path":"Observation.obs.value[x]","min":0,"max":"1","base":{"path":"Observation.value[x]","min":0,"max":"1"},"type":[{"code":"string"}]}]}}`
		tm = `{"resourceType":"StructureDefinition","id":"t","url":"http://example.org/T","name":"T","status":"active","kind":"logical","abstract":false,"type":"http://example.org/T","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"T","path":"T","base":{"path":"T","min":0,"max":"*"}},
 {"id":"T.item","path":"T.item","min":0,"max":"*","base":{"path":"T.item","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"T.item.value[x]","path":"T.item.value[x]","min":0,"max":"1","base":{"path":"T.item.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"}]},
 {"id":"T.item.item","path":"T.item.item","min":0,"max":"*","base":{"path":"T.item.item","min":0,"max":"*"},"contentReference":"#T.item"}]}}`
		doc = `{"resourceType":"StructureDefinition","id":"doc","url":"http://example.org/Doc","name":"Doc","status":"active","kind":"logical","abstract":false,"type":"http://example.org/Doc","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Doc","path":"Doc","base":{"path":"Doc","min":0,"max":"*"}},
 {"id":"Doc.t","path":"Doc.t","min":0,"max":"1","base":{"path":"Doc.t","min":0,"max":"1"},"type":[{"code":"http://example.org/T"}]}]}}`
		q = `{"resourceType":"StructureDefinition","id":"q","url":"http://example.org/Q","name":"Q","status":"active","kind":"logical","abstract":false,"type":"http://example.org/Doc","baseDefinition":"http://example.org/Doc","derivation":"constraint","snapshot":{"element":[
 {"id":"Doc","path":"Doc","base":{"path":"Doc","min":0,"max":"*"}},
 {"id":"Doc.t","path":"Doc.t","min":0,"max":"1","base":{"path":"Doc.t","min":0,"max":"1"},"type":[{"code":"http://example.org/T"}]},
 {"id":"Doc.t.item","path":"Doc.t.item","min":0,"max":"*","base":{"path":"T.item","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"Doc.t.item.item","path":"Doc.t.item.item","min":0,"max":"*","base":{"path":"T.item.item","min":0,"max":"*"}},
 {"id":"Doc.t.item.item.value[x]","path":"Doc.t.item.item.value[x]","min":0,"max":"1","base":{"path":"T.item.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"}]}]}}`
	)
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		r := sharedVersion(t, version).Clone()
		resources := make([][]byte, 0, 6)
		for _, sd := range []string{a, b, p, tm, doc, q} {
			resources = append(resources, []byte(sd))
		}
		pkg, err := loader.NewLoader("").LoadFromResources(resources)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.LoadFromPackages([]*loader.Package{pkg}); err != nil {
			t.Fatal(err)
		}
		obs := r.GetByURL("http://example.org/P").Tree().ByID("Observation.obs.value[x]")
		item := r.GetByURL("http://example.org/Q").Tree().ByID("Doc.t.item.item.value[x]")
		for _, tt := range []struct {
			node      *ElementNode
			key, want string
		}{
			{obs, "valueCoding", "Coding"}, // B declares it; the profile does not allow it
			{obs, "valueString", "string"},
			{obs, "valueBoolean", ""}, // A's Observation.value[x], not obs's
			{item, "valueDate", "date"},
			{item, "valueCoding", "Coding"},
			{item, "valueString", ""},
		} {
			if got := r.ChoiceType(tt.node, tt.key); got != tt.want {
				t.Errorf("%s: ChoiceType(%s, %q) = %q, want %q", version, tt.node.Def.ID, tt.key, got, tt.want)
			}
		}
	}
}
