# Plan v5, second series round 2

Offline review. `final.txt` is the authoritative complete run of all TestV5 tests. Counterexample tests assert that a defect reproduces: PASS does not imply v5 is defect-free. Other outputs are intermediate runs.

## Model

`probe_test.go.txt` adapts v4-r1: expression/type corrections patch raw leaves, additions remain struct-only. Load-time differential correction runs before merging. After the complete merge, correction selects the matching differential's constraint keys and type only when declared. Additions use an explicitly supplied specialization-root version (`v5RootVersion`, R4 by default; R5 for the foreign-root case). Root traversal is not implemented or claimed validated.

The temporary `placement-hook.patch` records actual successful placement IDs inside applyDifferential, including choice normalization and fallback. This gives v5 the strongest mapping available rather than assuming a naive ID/path lookup. All correction remains after the complete merge.

Reproduce by applying placement-hook.patch, copying probe_test.go.txt into pkg/registry/v5_review_probe_test.go, then running `go test ./pkg/registry -run '^TestV5' -count=1 -timeout=90s -v`. Use GOPROXY=off, GOTOOLCHAIN=local and writable workspace-local GOTMPDIR/TMPDIR/GOCACHE. Restore snapshot.go and delete the temporary test afterwards. Production source was restored byte-for-byte and temporary tests/caches removed. No network used.

## Findings

CONFIRMED: TestV5RedeclaredTypeCopiedToSliceChild. Loaded Resource R4 raw is corrected. The differential redeclares Patient.contained.id type without base.path, so loading cannot correct it. It overrides the corrected source with string. Subsequent slice-child copying during the same merge propagates string. Final correction fixes Patient.contained.id to id, while Patient.contained:mine.id remains string (its differential declares only min). This is same-profile propagation, not the previously resolved C1 inheritance from an already corrected base.

CONFIRMED: TestV5RenamedChoiceResliceLoadedDifferential. Observation.effectiveDateTime needs path normalization to correct vs-1; a reslice made during the same merge copies its uncorrected expression. Final correction fixes the source slice but leaves its reslice uncorrected.

CONFIRMED: TestV5SameKeyInheritedOwnership. R4B guide required-name has expressionErrata[0].published. Derived R4 differential redeclares that key with name.exists(). Base-first duplicate-key merge discards the differential expression, then v5's key mask rewrites the surviving inherited expression with R4 errata. The existing discard is not the finding: the new correction misattributes the surviving base text to the differential. This is ordinary guide inheritance, not the excluded type-profile/slice collision.

SUSPECTED for valid-profile reachability: TestV5AddedConstraintForeignRootCopy synthesizes logical Envelope.bundle with base.path Bundle. R4 load adds bdl-15 to its struct; raw-based unrolling under R5 Host loses it, and outer-root R5 does not restore it. Direct cloneElement also loses it. Copy loss is confirmed, but the exact model's specification conformance and an official-package reproducer were not established offline.

## Coverage and missing tests

All original v4-r1 scenarios pass with v5 expectations: D1/D2 resolved and A1/A2/B1/B4/C1/C2 retained, including inverse-core scan, stamping, metadata, clone reapplication, key order and fixed cache. Additional probes cover logical, Extension and datatype containing roots; differential constraints on cross-version unrolled children; internal contentReference; unconventional-ID fallback; slices/reslices. Placement/copy behavior is tested, not root traversal.

Add planned regressions for same-merge propagation (type and renamed-choice constraint), surviving ownership after duplicate-key merge, and foreign-root struct-only additions using a specification-valid fixture. Specify and test root traversal for logical models, Extension (stop at Extension rather than Element), datatype constraint profiles and external contentReference/type profiles, including version-pinned canonicals. A containing resource's root alone cannot identify an unrolled source's version.
