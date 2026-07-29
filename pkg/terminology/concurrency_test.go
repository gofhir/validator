package terminology

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// stubProvider is stateless, so concurrent validation does not race on the mock
// itself the way the state-recording provider in context_test.go would.
type stubProvider struct{}

func (stubProvider) ValidateCode(context.Context, string, string) (bool, error) {
	return true, nil
}

func (stubProvider) ValidateCodeInValueSet(context.Context, string, string, string) (valid, found bool, err error) {
	return true, true, nil
}

// hierarchicalCodeSystem builds a CodeSystem whose concepts form a tree, so that
// is-a filters have a hierarchy to derive.
func hierarchicalCodeSystem(url string, children int) *CodeSystem {
	cs := &CodeSystem{
		URL:     url,
		Concept: []CodeSystemCode{{Code: "root"}},
	}
	for i := range children {
		cs.Concept = append(cs.Concept, CodeSystemCode{
			Code:     fmt.Sprintf("child-%d", i),
			Property: []CodeSystemProperty{{Code: "subsumedBy", ValueCode: "root"}},
		})
	}
	return cs
}

// TestConcurrentExpansionBuildsHierarchySafely hammers the is-a expansion path
// from many goroutines over distinct ValueSets that share one CodeSystem, so the
// hierarchy cache is read and written concurrently. Run with -race.
func TestConcurrentExpansionBuildsHierarchySafely(t *testing.T) {
	const system = "http://example.org/CodeSystem/tree"
	const valueSets = 32

	r := NewRegistry()
	r.codeSystems[system] = hierarchicalCodeSystem(system, 50)

	urls := make([]string, valueSets)
	for i := range valueSets {
		// Distinct ValueSets so the expansion cache does not short-circuit the
		// hierarchy build.
		urls[i] = fmt.Sprintf("http://example.org/ValueSet/vs-%d", i)
		r.valueSets[urls[i]] = &ValueSet{
			URL: urls[i],
			Compose: Compose{Include: []Include{{
				System: system,
				Filter: []Filter{{Property: "concept", Op: "is-a", Value: "root"}},
			}}},
		}
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range valueSets {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			for range 20 {
				if valid, found := r.ValidateCodeContext(ctx, url, system, "child-7"); !found || !valid {
					t.Errorf("expected child-7 to be a member of %s", url)
					return
				}
			}
		}(urls[i])
	}
	wg.Wait()
}

// TestConcurrentSetProviderIsSafe covers the other unsynchronised field: the
// provider was written without a lock while validation read it. Run with -race.
func TestConcurrentSetProviderIsSafe(_ *testing.T) {
	const vsURL = "http://example.org/ValueSet/sct"

	r := NewRegistry()
	r.valueSets[vsURL] = &ValueSet{
		URL:     vsURL,
		Compose: Compose{Include: []Include{{System: externalSystem}}},
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 200 {
			r.SetProvider(stubProvider{})
		}
	}()

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				r.ValidateCodeContext(ctx, vsURL, externalSystem, "73211009")
				r.ValidateCodeInCodeSystemContext(ctx, externalSystem, "73211009")
			}
		}()
	}

	wg.Wait()
}
