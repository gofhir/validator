package reslang

import (
	"context"
	"testing"

	"github.com/gofhir/validator/v2/pkg/walker"
)

// A nested resource is in its own language, or else in the language of the resource that holds
// it, whatever holds it: a contained resource, an entry, a parameter's resource.
func TestWalkInheritsTheHoldersLanguage(t *testing.T) {
	ctx := With(context.Background(), map[string]any{"language": "de"})
	w := NewWalk(ctx, "Parameters")
	for _, tt := range []struct {
		rc   *walker.ResourceContext
		want string
	}{
		{&walker.ResourceContext{Data: map[string]any{}, FHIRPath: "Parameters.parameter[0].resource", ParentPath: "Parameters", Kind: walker.KindHeld}, "de"},
		{&walker.ResourceContext{Data: map[string]any{"language": "fr"}, FHIRPath: "Parameters.parameter[1].resource", ParentPath: "Parameters", Kind: walker.KindHeld}, "fr"},
		{&walker.ResourceContext{Data: map[string]any{}, FHIRPath: "Parameters.parameter[1].resource.contained[0]", ParentPath: "Parameters.parameter[1].resource", Kind: walker.KindContained}, "fr"},
	} {
		if got := Of(w.Context(ctx, tt.rc)); got != tt.want {
			t.Errorf("%s: language %q, want %q", tt.rc.FHIRPath, got, tt.want)
		}
	}
}
