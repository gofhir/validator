Round 2 review evidence

1. High: profile discrimination ending in resolve() passes the chain's scope to Conforms.
Reproducer: round2-experiment_test.go (copy into pkg/validator to run TestReviewRound2ResolvedBundle).
Starting with r1b, change Composition.section discriminator to entry.resolve(), targetProfile to bundle-p.
bundle-p slices Bundle.signature by onBehalfOf.resolve() against org-t, requiring one matching signature;
org-t requires Organization.name. Also constrain Composition.event.detail to bundle-p and reference the same Bundle.
round2-invariant.txt shows the same map reaching Conforms with outer and canonical containers, false versus true
with independent conform states, and erroneous SLICING_CARDINALITY_MIN in the complete validation.
Earlier run using checkedCtx failed the invariant twice. The final diagnostic callback logs both scopes.
Fix experiment: in Matcher.valueScope, permit every resource map (including resolved definition roots) to use
ScopedResolver.ScopeOf, instead of requiring HoldsResource(e.def.Def). This preserves ResolveScoped chain semantics.
round2-terminal-fix.txt: erroneous minimum disappears; canonical probes and invariant tests pass.

2. Medium: detached root Bundle with External gets the external container as its canonical container.
Reproducer: round2-place-experiment_test.go (copy into pkg/validator to run TestReviewRound2Place).
Starting with probe/r2/r2a, extract b2, remove its entry from the external Bundle, make signature.who absolute,
and validate b2 independently using reference.WithContainer(external) and bun-tpw.
Mock conformer accepts only the patient named inner.
round2-place-input.txt: target family outer, REFERENCE_TARGET_PROFILE.
round2-place-input-mutated.txt: place is a no-op, target family inner, no issues. This kills mutation 16.
round2-external-fix.txt: with place restored, initialize the detached root's scope through inBundle when root
is a Bundle in Build's external fallback. Target is inner, no issues. Existing standalone and index tests pass.
Thus place is not observationally redundant in the current implementation; the difference exposes a Build defect.

Verification:
Original targeted tests pass. Broader place mutation targeted tests pass, demonstrating coverage gap.
All six full package test runs were attempted, but validator full suite panics opening an httptest listener
because socket binding is denied by the sandbox. Other five packages passed. Do not interpret this as full-suite success.
Final restored targeted tests: round2-restored-tests.txt.
All production files compared byte-for-byte with pre-experiment backups; temporary package tests removed.

Review context discrepancy: the local plan ends at section 7, line 314. Section 8, round-1 corrections, HL7
inconsistency, and later mutation notes requested by the user are absent in this worktree.
