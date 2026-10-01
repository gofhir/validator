package terminology

import (
	"context"
	"errors"
	"testing"
)

// A ValueSet that includes a code system this registry cannot expand accepts any of its codes on
// the local path (fail-open). Those answers are marked Assumed, and under WithStrictMembership they
// are Unresolved; answers that were checked are not affected.
func TestAssumedMembership(t *testing.T) {
	const (
		local    = "http://example.org/CodeSystem/local"
		external = "http://snomed.info/sct"
		vsURL    = "http://example.org/ValueSet/mixed"
	)
	newRegistry := func() *Registry {
		r := NewRegistry()
		r.codeSystems[local] = &CodeSystem{URL: local, Concept: []CodeSystemCode{{Code: "a"}}}
		r.valueSets[vsURL] = &ValueSet{URL: vsURL, Compose: Compose{Include: []Include{
			{System: local},
			{System: external, Filter: []Filter{{Property: "concept", Op: "is-a", Value: "404684003"}}},
		}}}
		return r
	}
	ctx := context.Background()
	strict := WithStrictMembership(ctx)

	for _, tt := range []struct {
		name         string
		provider     Provider
		system, code string
		ctx          context.Context
		want         Resolution
		assumed      bool
	}{
		{"unexpandable system, any code", nil, external, "not-a-code", ctx, Valid, true},
		{"unexpandable system, strict", nil, external, "not-a-code", strict, Unresolved, false},
		{"a code of an expanded system", nil, local, "a", strict, Valid, false},
		{"a code outside an expanded system", nil, local, "z", strict, Invalid, false},
		{"a provider that answers", &hostProvider{knownSystems: map[string]map[string]bool{external: {"22298006": true}}}, external, "22298006", strict, Valid, false},
		{"a provider that fails falls back to the wildcard", &hostProvider{err: errors.New("circuit open")}, external, "22298006", strict, Unresolved, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newRegistry()
			if tt.provider != nil {
				r.SetProvider(tt.provider)
			}
			res, err := r.ResolveCodeInValueSet(tt.ctx, tt.system, tt.code, vsURL, LookupOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if res.Resolution != tt.want || res.Assumed != tt.assumed {
				t.Errorf("resolution %v assumed %v, want %v assumed %v", res.Resolution, res.Assumed, tt.want, tt.assumed)
			}
		})
	}
}
