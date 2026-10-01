package validator

import (
	"context"
	"strings"
	"testing"
)

// Constraints are evaluated with a FHIRPath model from the loaded definitions, so the engine
// knows an id is a string: dom-3 computes '#' + id, which without the model failed for an id of
// four digits, read as a year (the R4 examples PlanDefinition-KDN5 and RequestGroup-kdn5-example).
func TestConstraintsUseTheModelTypes(t *testing.T) {
	v := getSharedValidator(t)
	res, err := v.Validate(context.Background(), []byte(`{"resourceType":"PlanDefinition","id":"p",
"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},
"contained":[{"resourceType":"ActivityDefinition","id":"1111","status":"active"}],
"status":"active","action":[{"definitionCanonical":"#1111"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range res.Issues {
		if strings.Contains(is.Diagnostics, "dom-3") || strings.HasPrefix(is.MessageID, "CONSTRAINT_EVAL") {
			t.Errorf("%s %s: %s", is.Severity, is.MessageID, is.Diagnostics)
		}
	}
}
