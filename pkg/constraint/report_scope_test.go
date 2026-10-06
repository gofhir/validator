package constraint

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/pkg/registry"
)

// A failure is reported once per location in a scope, and a scope of its own (a check whose
// issues are discarded) neither hides a failure from the scope it is made in nor is hidden by it.
func TestReportScope(t *testing.T) {
	c := registry.Constraint{Key: "k-1", Expression: "name.exists()"}
	other := registry.Constraint{Key: "k-1", Expression: "id.exists()"}

	outer := WithReportScope(context.Background())
	if !firstReport(outer, c, "Patient") || firstReport(outer, c, "Patient") {
		t.Fatal("want the first report of a failure, and not the second")
	}
	if !firstReport(outer, c, "Patient.contact[0]") || !firstReport(outer, other, "Patient") {
		t.Error("another location, or another expression under the same key, is another failure")
	}

	inner := WithReportScope(outer)
	if !firstReport(inner, c, "Patient") {
		t.Error("a scope of its own reports a failure its parent scope has reported")
	}
	if !firstReport(inner, c, "Patient.name[0]") || !firstReport(outer, c, "Patient.name[0]") {
		t.Error("a failure reported in a scope of its own is reported in its parent scope too")
	}

	for range 2 {
		if !firstReport(context.Background(), c, "Patient") {
			t.Error("without a scope, every failure is reported")
		}
	}
}
