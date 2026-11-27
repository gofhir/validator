Review round 1 evidence (offline)

- targeted.txt: requested ErrataCrossVersion suite, unmodified production.
- probes.txt: failing regression probes against reviewed production.
- ancestry-old-postmerge.txt: ancestry probe passes when the former post-merge correctSnapshot call replaces the new rootVersion/addConstraints call. This mutation isolates the loss of bdl-15.
- leaf-skip-unset.txt: leaf probe passes when patchRaw skips unset expressions. This is diagnostic, not a proposed complete fix.
- concurrent.txt: concurrent EnsureSnapshot probe for a shared differential that needs placed correction.
- race-targeted.txt: requested suite with the Go race detector.
- errata-diff.txt: reviewed errata.go diff, including newly added English comments.
- review_round1_test.go.txt: reproducer, temporarily copy into pkg/registry/review_round1_test.go and run go test -count=1 -run TestReviewRound1 ./pkg/registry.

All commands used GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local, with GOCACHE and GOTMPDIR under .review. No production fix was retained.
