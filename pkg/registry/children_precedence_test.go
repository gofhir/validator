package registry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

const (
	quantityURL = "http://hl7.org/fhir/StructureDefinition/Quantity"
	moneyURL    = "http://hl7.org/fhir/StructureDefinition/MoneyQuantity"
	missingURL  = "https://example.org/nothing"
)

// precedenceRegistry es R4 más perfiles de Observation que declaran, cada uno, una combinación
// distinta de hijos desplegados, slices y perfiles de tipo en value[x] (ElementDefinition.type.profile,
// profiling.html#slicing: un slice restringe el elemento que divide y hereda lo que él declara).
func precedenceRegistry(t *testing.T) *Registry {
	t.Helper()
	r := loadVersion(t, "4.0.1")
	sdOf := func(id string, elements string) json.RawMessage {
		return json.RawMessage(`{"resourceType":"StructureDefinition","url":"https://example.org/` + id + `","name":"P","type":"Observation",
"kind":"resource","fhirVersion":"4.0.1","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation",
"snapshot":{"element":[{"id":"Observation","path":"Observation"},` + elements + `]}}`)
	}
	value := func(profile string, slicing bool) string {
		s := ""
		if slicing {
			s = `,"slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"}`
		}
		p := ""
		if profile != "" {
			p = `,"profile":["` + profile + `"]`
		}
		return `{"id":"Observation.value[x]","path":"Observation.value[x]","type":[{"code":"Quantity"` + p + `}]` + s + `}`
	}
	slice := func(profile string) string {
		p := ""
		if profile != "" {
			p = `,"profile":["` + profile + `"]`
		}
		return `{"id":"Observation.value[x]:vq","path":"Observation.value[x]","sliceName":"vq","type":[{"code":"Quantity"` + p + `}]}`
	}
	unit := `{"id":"Observation.value[x].unit","path":"Observation.value[x].unit","min":1}`
	defs := map[string]json.RawMessage{
		"own":        sdOf("own", value(simpleQuantity, false)+","+unit),
		"ownmissing": sdOf("ownmissing", value(missingURL, false)+","+unit),
		"unrolled":   sdOf("unrolled", value("", true)+","+unit+","+slice("")),
		"node":       sdOf("node", value(simpleQuantity, false)),
		"sliceof":    sdOf("sliceof", value(simpleQuantity, true)+","+slice("")),
		"mostspecific": sdOf("mostspecific", value(moneyURL, true)+","+slice(simpleQuantity)+","+
			`{"id":"Observation.value[x]:vq/r","path":"Observation.value[x]","sliceName":"vq/r","type":[{"code":"Quantity"}]}`),
		"mixed":     sdOf("mixed", value(simpleQuantity, true)+","+slice(missingURL)),
		"onlyslc":   sdOf("onlyslc", value("", true)+","+slice(missingURL)),
		"onlyanc":   sdOf("onlyanc", value(missingURL, true)+","+slice("")),
		"chainmiss": sdOf("chainmiss", value(missingURL, true)+","+slice(missingURL)),
		"content": sdOf("content", value(simpleQuantity, true)+","+slice("")+","+
			`{"id":"Observation.hasMember","path":"Observation.hasMember","contentReference":"#Observation.value[x]:vq"}`),
	}
	if err := r.LoadFromPackages([]*loader.Package{{Name: "prec", Version: "0.0.1", Resources: defs}}); err != nil {
		t.Fatal(err)
	}
	return r
}

func precNode(t *testing.T, r *Registry, profile, id string) *ElementNode {
	t.Helper()
	sd := r.GetByURL("https://example.org/" + profile)
	if sd == nil {
		t.Fatalf("%s not loaded", profile)
	}
	if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
		t.Fatal(err)
	}
	n := sd.Tree().ByID(id)
	if n == nil {
		t.Fatalf("%s has no %s", profile, id)
	}
	return n
}

func sdURL(sd *StructureDefinition) string {
	if sd == nil {
		return "none"
	}
	return sd.URL
}

func hasChild(c InstanceChildren, path string) bool {
	for _, n := range c.Nodes {
		if n.Def.Path == path {
			return true
		}
	}
	return false
}

// Qué definición gobierna los hijos de un valor, en orden de precedencia (#193, pieza 1): los
// hijos desplegados del nodo o del elemento que divide, el destino de un contentReference, el perfil
// de tipo resuelto más específico del nodo y de su cadena SliceOf, la definición que el valor declara
// de sí mismo y la del tipo. Un perfil que no se resuelve nunca detiene el descenso: si el propio nodo lo
// declara y ninguno resuelve, se informa en Unresolved y los hijos son los del tipo. HL7 validator_cli
// 7.0.4 (R4, R4B y R5) valida el valor contra el tipo base y además informa el perfil desconocido:
// docs/plans/evidence/2026-11-05-children-precedence/probe/review2/ (s21, s23 y s24).
func TestChildrenOfPrecedence(t *testing.T) {
	r := precedenceRegistry(t)
	ctx := context.Background()
	vq := "Observation.value[x]:vq"
	selfMoney := func(string) *StructureDefinition { return r.GetByURL(moneyURL) }
	for _, tt := range []struct {
		name           string
		profile, id    string
		self           func(string) *StructureDefinition
		wantSD         string
		wantChild      string
		wantUnresolved string
	}{
		{"own children come before the node's profile", "own", "Observation.value[x]", nil,
			"https://example.org/own", "Observation.value[x].unit", ""},
		{"own children come before an unresolved profile", "ownmissing", "Observation.value[x]", nil,
			"https://example.org/ownmissing", "Observation.value[x].unit", ""},
		{"the unrolled children of the element a slice slices", "unrolled", vq, nil,
			"https://example.org/unrolled", "Observation.value[x].unit", ""},
		{"the node's profile", "node", "Observation.value[x]", nil, simpleQuantity, "Quantity.comparator", ""},
		{"the node's profile before what the instance declares", "node", "Observation.value[x]", selfMoney,
			simpleQuantity, "Quantity.comparator", ""},
		{"the profile of the element a slice slices", "sliceof", vq, nil, simpleQuantity, "Quantity.comparator", ""},
		{"the most specific resolved profile in the chain", "mostspecific", "Observation.value[x]:vq/r", nil,
			simpleQuantity, "Quantity.comparator", ""},
		{"a resolved ancestor over an unresolved slice", "mixed", vq, nil, simpleQuantity, "Quantity.comparator", ""},
		{"an unresolved profile with none resolved: reported, and the type's children", "onlyslc", vq, nil,
			quantityURL, "Quantity.comparator", missingURL},
		{"an unresolved profile only on an ancestor does not stop the descent", "onlyanc", vq, nil,
			quantityURL, "Quantity.comparator", ""},
		{"an unresolved profile of the unsliced element: reported, and the type's children", "onlyanc", "Observation.value[x]", nil,
			quantityURL, "Quantity.comparator", missingURL},
		{"the type of a contentReference's target, not the chain of its slice", "content", "Observation.hasMember", nil,
			quantityURL, "Quantity.comparator", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := r.ChildrenOf(ctx, r.GetByURL("https://example.org/"+tt.profile), precNode(t, r, tt.profile, tt.id), "Quantity", tt.self)
			if c.SD == nil || c.SD.URL != tt.wantSD || c.Unresolved.Canonical != tt.wantUnresolved {
				t.Fatalf("definition %s (unresolved %q), want %s (unresolved %q)", sdURL(c.SD), c.Unresolved.Canonical, tt.wantSD, tt.wantUnresolved)
			}
			if !hasChild(c, tt.wantChild) {
				t.Errorf("no %s among the children", tt.wantChild)
			}
		})
	}
}
