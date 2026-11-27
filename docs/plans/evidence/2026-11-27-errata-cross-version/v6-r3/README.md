# v6, second series round 3 (offline)

Final modeled v6 passes requested A1, A2, B1, B4, C1, C2, D1, D2, E1, valid E2 and E4: probes.txt. E2 now repeats the guide's identical published expression; inherited R4B text survives. The retained synthetic E3 fixture is excluded from findings because its valid-profile reachability remains unestablished.

model.patch changes applyErrata and snapshot generation. v6_review_model.go.txt supplies raw leaf patching, the pre-merge identity adapter and first-specialization root traversal. The hook corrects the placed differential's own constraints/type using the current base element's path/base.path, after yieldCloned and before mergeElements. Final correctSnapshot is removed; only additions run post-merge with the root's version. Context carries the profile version without changing existing callers. Adapted direct probes provide that version explicitly; r3Correct only adds constraints. Clone loading calls changed applyErrata.

The initial raw leaf patch replaced raw without refreshing keepPrefixedValues. cachedPrefixed then rejects the old cache by slice identity, though getters still compute correct values. registry-before-cache-refresh.txt records TestPrefixedValuesPublishedSnapshots failures. Final helper refreshes the cache when replacing raw. registry.txt records go test ./pkg/registry/... -count=1 -timeout=180s PASS (38.151 seconds), including existing and temporary tests. No existing test required the removed final expression/type correction after loading/placement correction was modeled.

v6_review_extra_test.go.txt covers root traversal with version-pinned canonicals, logical specialization, Extension/datatype contexts, version-pinned external contentReference, official nested Bundle unrolling, failed placement followed by later successful unrolling, and yieldCloned. Root fixtures load R5 ElementDefinition for choice metadata. Initial fixture lacked that metadata; corrected before final runs.

Official R4 Bundle.entry.resource and Parameters.parameter.resource constrained to Bundle both unroll 47 descendants when the differential names .type. The plan's never-unrolled assertion is false. unroll copies descendants, not the Bundle root. validator_probe_test.go.txt and nested-verdict.txt demonstrate that the nested resource's own Bundle definition still reports bdl-15 at both nested resource locations. This does not establish the excluded E3 defect in valid profiles.

mutation-no-placement-correction.txt disables only placement correction. B1 explicit type, B1 renamed choice, and both E1 same-generation copies fail. Section 6 should require this mutation instead of removing the final correction already absent in v6.

Reproduce by applying model.patch and copying each registry .go.txt to pkg/registry under its original filename; copy validator_probe_test.go.txt to pkg/validator/v6_review_probe_test.go. Use TMPDIR=/tmp GOTMPDIR=/tmp GOPROXY=off GOTOOLCHAIN=local and writable GOCACHE. Run registry suite as above, probes with go test ./pkg/registry -run '^TestV6' -count=1 -v and validator probe with go test ./pkg/validator -run '^TestV6UnrolledNestedBundleAddition$' -count=1 -v. Mutation uses empty version at v6Placed.

Production files restored byte-for-byte. Temporary Go files and build cache removed. Only this evidence directory added by review. No network used.
