package registry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// Un slice que no declara perfiles de tipo hereda los del elemento que divide, también cuando son varios
// (profiling.html#slicing: un slice restringe el elemento que divide; ElementDefinition.type.profile: «the content must
// conform to at least one of them»). La cadena SliceOf de TypeProfilesOf da esas alternativas, y ChildrenOf elige entre
// ellas por valor. Mutación 10 (14-11-2026): sin la cadena, un snapshot que da el slice sin tipo propio perdería las
// alternativas; el generador de snapshots copia el tipo del elemento al slice, así que solo un snapshot escrito así lo
// alcanza.

// El elector chooseOnly da por cumplido solo el perfil met.
type chooseOnly struct{ met string }

func (c chooseOnly) Conforms(_ context.Context, _ Instance, _ Alternatives, profile *StructureDefinition) (met, decided bool) {
	return profile.URL == c.met, true
}

func TestTypeProfilesOfAlternativesInTheSlicedElement(t *testing.T) {
	r := loadVersion(t, "4.3.0")
	sdOf := func(id, elements string) json.RawMessage {
		return json.RawMessage(`{"resourceType":"StructureDefinition","url":"https://example.org/` + id + `","name":"P","type":"Observation",
"kind":"resource","fhirVersion":"4.3.0","derivation":"constraint","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation",
"snapshot":{"element":[{"id":"Observation","path":"Observation"},` + elements + `]}}`)
	}
	two := `"profile":["` + simpleQuantity + `","` + moneyURL + `"]`
	value := `{"id":"Observation.value[x]","path":"Observation.value[x]","type":[{"code":"Quantity",` + two + `}],
"slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"}}`
	defs := map[string]json.RawMessage{
		// el slice no dice nada de su tipo
		"altof": sdOf("altof", value+`,{"id":"Observation.value[x]:vq","path":"Observation.value[x]","sliceName":"vq"}`),
		// y un slice que declara solo uno gobierna con él: las alternativas del elemento no cuentan
		"altone": sdOf("altone", value+`,{"id":"Observation.value[x]:vq","path":"Observation.value[x]","sliceName":"vq","type":[{"code":"Quantity","profile":["`+moneyURL+`"]}]}`),
	}
	if err := r.LoadFromPackages([]*loader.Package{{Name: "alt", Version: "0.0.1", Resources: defs}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const vq, element = "Observation.value[x]:vq", "Observation.value[x]"

	t.Run("a slice with no type of its own has the alternatives of the sliced element", func(t *testing.T) {
		p := r.TypeProfilesOf(ctx, precNode(t, r, "altof", vq), "Quantity")
		if p.Governing.SD != nil {
			t.Fatalf("governing %s, want none", p.Governing.SD.URL)
		}
		if p.Alternatives.Node == nil || p.Alternatives.Node.Def.ID != element || len(p.Alternatives.Declared) != 2 {
			t.Fatalf("alternatives %+v, want the two the element %s declares", p.Alternatives, element)
		}
	})
	t.Run("a slice that declares one profile governs with it", func(t *testing.T) {
		p := r.TypeProfilesOf(ctx, precNode(t, r, "altone", vq), "Quantity")
		if p.Governing.SD == nil || p.Governing.SD.URL != moneyURL || p.Alternatives.Node != nil {
			t.Fatalf("governing %v alternatives %+v, want %s and none", sdURL(p.Governing.SD), p.Alternatives, moneyURL)
		}
	})
	t.Run("the children of a value of the slice are those of the profiles it conforms to", func(t *testing.T) {
		slice := precNode(t, r, "altof", vq)
		sd := r.GetByURL("https://example.org/altof")
		value := map[string]any{"value": 1.0}
		for _, tt := range []struct {
			name string
			ctx  context.Context
			want []string
		}{
			{"it conforms to one", WithProfileChooser(ctx, chooseOnly{met: moneyURL}), []string{moneyURL}},
			{"it conforms to the other", WithProfileChooser(ctx, chooseOnly{met: simpleQuantity}), []string{simpleQuantity}},
			{"it conforms to none: under both", WithProfileChooser(ctx, chooseOnly{met: "none"}), []string{simpleQuantity, moneyURL}},
			{"nothing chooses: under both", ctx, []string{simpleQuantity, moneyURL}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				c := r.ChildrenOf(tt.ctx, sd, slice, "Quantity", Instance{Value: value})
				var got []string
				for i := range c.NumLayers() {
					got = append(got, c.LayerAt(i).SD.URL)
				}
				if len(got) != len(tt.want) {
					t.Fatalf("layers %q, want %q", got, tt.want)
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Fatalf("layers %q, want %q", got, tt.want)
					}
				}
			})
		}
	})
}
