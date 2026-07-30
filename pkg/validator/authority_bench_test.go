package validator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/validator/pkg/terminology"
)

// benchAuthority answers instantly, so the benchmark isolates the structural cost
// of routing every coded element through the port. A real chain adds its own
// per-lookup latency on top; GoFHIR Server measured ~1.1 µs warm against an
// in-memory layer, which is additive per coded element.
type benchAuthority struct{}

func (benchAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{
		Resolution:       terminology.Valid,
		SystemInValueSet: terminology.MembershipIncluded,
	}, nil
}

func (benchAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Valid}, nil
}

func (benchAuthority) Supports(context.Context, string) bool { return true }

// bulkBundle builds a Bundle of Patients carrying several bound elements each, so
// the benchmark reflects validation over many coded elements rather than a single
// lookup.
func bulkBundle(entries int) []byte {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Bundle","type":"collection","entry":[`)
	for i := range entries {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"resource":{
			"resourceType":"Patient",
			"id":"p%d",
			"gender":"female",
			"maritalStatus":{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/v3-MaritalStatus","code":"M"}]},
			"communication":[{"language":{"coding":[{"system":"urn:ietf:bcp:47","code":"en"}]}}],
			"identifier":[{"use":"official","system":"http://example.org/mrn","value":"%d"}],
			"telecom":[{"system":"phone","use":"home","value":"555-000%d"}],
			"address":[{"use":"home","type":"physical","country":"CL"}]
		}}`, i, i, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

const bundleEntries = 25

// Measured 2026-07-29, Apple M4 Pro, 25-entry Bundle, -benchtime 20x:
//
//	BenchmarkValidateBundleLocalTerminology      10.03 ms/op   2.26 MB   43701 allocs
//	BenchmarkValidateBundleWithAuthority          9.80 ms/op   2.25 MB   43644 allocs
//	BenchmarkValidateBundleAuthorityUnresolved   14.36 ms/op   6.34 MB  223748 allocs
//
// Routing every binding lookup through the port does not regress throughput — it
// is marginally faster, because skipping the base terminology also skips expanding
// ValueSets and maintaining the expansion cache, which costs more than the extra
// indirection.
//
// The third case is a degraded configuration, not the expected one: an authority
// that resolves nothing makes the validator emit a diagnostic per coded element,
// and the cost is those issues rather than the lookups.
//
// These use an in-process authority, so a real chain's latency is additive. At the
// ~1.1 µs/lookup GoFHIR Server measured against its in-memory layer, and roughly
// 125 coded elements in this Bundle, that is ~137 µs on top of 9.8 ms — about 1.4%.

// BenchmarkValidateBundleLocalTerminology is the baseline: binding lookups resolve
// against the in-process copies.
func BenchmarkValidateBundleLocalTerminology(b *testing.B) {
	v, err := New(WithVersion("4.0.1"))
	if err != nil {
		b.Skipf("cannot create validator: %v", err)
	}
	bundle := bulkBundle(bundleEntries)

	b.ResetTimer()
	for b.Loop() {
		if _, err := v.Validate(context.Background(), bundle); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidateBundleWithAuthority is the same work with every binding lookup
// routed through the Authority port instead of a local map.
func BenchmarkValidateBundleWithAuthority(b *testing.B) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(benchAuthority{}))
	if err != nil {
		b.Skipf("cannot create validator: %v", err)
	}
	bundle := bulkBundle(bundleEntries)

	b.ResetTimer()
	for b.Loop() {
		if _, err := v.Validate(context.Background(), bundle); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidateBundleAuthorityUnresolved measures the pattern the negative
// cache exists for: a canonical the authority cannot resolve, hit once per coded
// element across the whole Bundle.
func BenchmarkValidateBundleAuthorityUnresolved(b *testing.B) {
	v, err := New(WithVersion("4.0.1"), WithTerminologyAuthority(unresolvedAuthority{}))
	if err != nil {
		b.Skipf("cannot create validator: %v", err)
	}
	bundle := bulkBundle(bundleEntries)

	b.ResetTimer()
	for b.Loop() {
		if _, err := v.Validate(context.Background(), bundle); err != nil {
			b.Fatal(err)
		}
	}
}

type unresolvedAuthority struct{}

func (unresolvedAuthority) ResolveCodeInValueSet(_ context.Context, _, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Unresolved}, nil
}

func (unresolvedAuthority) ResolveCodeInCodeSystem(_ context.Context, _, _ string, _ terminology.LookupOptions) (terminology.CodeResult, error) {
	return terminology.CodeResult{Resolution: terminology.Unresolved}, nil
}

func (unresolvedAuthority) Supports(context.Context, string) bool { return true }
