package constraint

import (
	"encoding/json"
	"testing"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
)

// El ElementDefinition.type de un slice restringe los valores del slice (profiling.html), y un
// primitivo sin valor es su objeto "_key" (json.html#primitive): el slice de un elemento de elección
// que solo admite string rechaza un boolean que no tiene más que su "_key", aunque el elemento que
// parte admita ambos.
func TestValuelessPrimitiveOfChoiceSliceTypeNotAllowed(t *testing.T) {
	v := &Validator{}
	sd := &registry.StructureDefinition{URL: "http://example.org/p"}
	base := &registry.ElementNode{Def: &registry.ElementDefinition{Path: "X.value[x]", Type: []registry.Type{{Code: "string"}, {Code: "boolean"}}}}
	node := &registry.ElementNode{Def: &registry.ElementDefinition{Path: "X.value[x]", Type: []registry.Type{{Code: "string"}}}, SliceOf: base}
	result := issue.NewResult()
	v.checkValueless(sd, node, "boolean", json.RawMessage(`{"id":"v"}`), "X.valueBoolean", &constraintEvalOpts{}, result)
	if result.ErrorCount() != 1 {
		t.Fatalf("got %d errors; want 1: %+v", result.ErrorCount(), result.Issues)
	}
	if is := result.Issues[0]; is.MessageID != string(issue.DiagStructureTypeNotAllowed) || len(is.Expression) != 1 || is.Expression[0] != "X.valueBoolean" {
		t.Fatalf("unexpected issue %+v", is)
	}
}
