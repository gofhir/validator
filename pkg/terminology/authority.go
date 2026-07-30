package terminology

import "context"

// unknownLabel is the String() result for an out-of-range enum value.
const unknownLabel = "unknown"

// Resolution is the outcome of a terminology decision.
//
// Unresolved and Invalid are deliberately distinct. A backend that cannot decide
// — no terminology server configured, canonical unknown to its chain — reports
// Unresolved, and the caller applies its unresolved policy. Only Invalid means
// the code was checked and rejected.
type Resolution int

const (
	// Unresolved reports that neither local copies nor any configured backend
	// could decide. Never treat it as a validation failure.
	Unresolved Resolution = iota
	// Valid reports that the code is a member.
	Valid
	// Invalid reports that the code was checked and is not a member.
	Invalid
)

// String implements fmt.Stringer.
func (r Resolution) String() string {
	switch r {
	case Unresolved:
		return "unresolved"
	case Valid:
		return "valid"
	case Invalid:
		return "invalid"
	default:
		return unknownLabel
	}
}

// Membership is a tri-state answer about a system's presence in a ValueSet.
//
// Tri-state rather than a bool because "the ValueSet does not declare this
// system" and "I could not determine whether it does" lead to different binding
// outcomes: the first legitimizes a code from another system under an extensible
// binding, the second legitimizes nothing.
type Membership int

const (
	// MembershipUnknown reports that presence could not be determined, for
	// instance when only a remote backend could answer. Callers must not infer
	// presence or absence.
	MembershipUnknown Membership = iota
	// MembershipIncluded reports that the system is among the ValueSet's
	// declared systems.
	MembershipIncluded
	// MembershipExcluded reports that the system is not among them.
	MembershipExcluded
)

// String implements fmt.Stringer.
func (m Membership) String() string {
	switch m {
	case MembershipUnknown:
		return unknownLabel
	case MembershipIncluded:
		return "included"
	case MembershipExcluded:
		return "excluded"
	default:
		return "invalid"
	}
}

// UnresolvedPolicy decides what a caller does when terminology cannot be
// resolved — Resolution is Unresolved rather than Valid or Invalid.
//
// It exists so that "accept what we could not check" is a stated policy rather
// than a value baked into a return type. Before it, the answer was hardcoded in
// two places and could not be changed by an operator who needed closed-world
// validation.
type UnresolvedPolicy int

const (
	// UnresolvedWarn accepts the code and records an informational issue. Default,
	// and equivalent to the HL7 validator's -tx n/a: without a terminology source
	// there is nothing to check against, and failing every coded element would be
	// worse than saying so.
	UnresolvedWarn UnresolvedPolicy = iota
	// UnresolvedError treats an unresolvable binding as a validation failure. For
	// deployments that would rather reject data than accept it unchecked.
	UnresolvedError
)

// String implements fmt.Stringer.
func (p UnresolvedPolicy) String() string {
	switch p {
	case UnresolvedWarn:
		return "warn"
	case UnresolvedError:
		return "error"
	default:
		return unknownLabel
	}
}

// LookupOptions carries request-scoped preferences. Fields are preferences, not
// guarantees: the result reports what was actually honored.
type LookupOptions struct {
	// DisplayLanguage is a BCP-47 language tag. Empty means no preference.
	DisplayLanguage string

	// SystemVersion is the CodeSystem version the code was authored against,
	// taken from Coding.version. Empty means no preference, in which case the
	// backend resolves whichever version it considers current.
	//
	// A code can be valid in one version of a CodeSystem and absent from another,
	// so a Coding that declares a version is asking to be checked against that
	// version specifically.
	SystemVersion string
}

// CodeResult answers a single coded-element question.
type CodeResult struct {
	Resolution Resolution

	// Display is the concept's display name, empty when the backend has none.
	Display string

	// DisplayLanguageHonored reports whether Display is in the language
	// requested via LookupOptions.DisplayLanguage. When false — including when
	// the backend ignores the option entirely — callers must not treat a display
	// mismatch as an error, because the comparison would be against a fallback
	// language.
	DisplayLanguageHonored bool

	// SystemInValueSet reports whether the queried system is among the
	// ValueSet's declared systems, so callers can apply extensible binding
	// semantics without holding a local copy of the ValueSet. Meaningful only
	// for ResolveCodeInValueSet.
	SystemInValueSet Membership

	// Message is an optional backend diagnostic, suitable for surfacing in the
	// resulting issue.
	Message string
}

// Authority is the terminology port for hosts that own terminology resolution,
// including ValueSets and CodeSystems authored after the validator was built and
// any configured remote terminology server.
//
// Every method takes a context: implementations may perform network I/O, and
// callers impose deadlines and cancellation.
//
// Resolution and error are distinct. Return Unresolved for "cannot decide";
// reserve error for genuine failures — backend unreachable, circuit open, query
// failed. An implementation that returns an error for "I don't know about this
// canonical" forces callers to classify error strings, which is what this
// contract exists to avoid.
//
// Method names are deliberately Resolve* rather than ValidateCode*: a single
// type must be able to implement both Authority and the narrower Provider during
// migration, and Go permits only one method per name per type.
type Authority interface {
	// ResolveCodeInValueSet decides whether code (in system) is a member of the
	// ValueSet identified by valueSetURL.
	ResolveCodeInValueSet(ctx context.Context, system, code, valueSetURL string, opts LookupOptions) (CodeResult, error)

	// ResolveCodeInCodeSystem decides whether code exists in system, regardless
	// of any ValueSet binding.
	ResolveCodeInCodeSystem(ctx context.Context, system, code string, opts LookupOptions) (CodeResult, error)

	// Supports reports whether anything in this authority's chain might decide
	// the given canonical URL. It is a short-circuit hint, not a guarantee: a
	// chain ending at a remote server may answer true and still fail to resolve.
	// False means "do not bother asking".
	Supports(ctx context.Context, url string) bool
}
