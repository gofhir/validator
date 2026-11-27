| id | file:line | exact edit | result | first killing test | duration |
|---|---|---|---|---|---|
| 01 | pkg/constraint/tree.go:~145 | otherDefinitions: chain walk n = n.SliceOf -> n = nil | KILLED | TestDescentSlicedElementAndSlice (./pkg/validator) | 65s |
| 02 | pkg/constraint/tree.go:~151 | otherDefinitions: type profile branch `if typeRoot != nil &&` -> `if false &&` | KILLED | TestDescentNamedProbes (./pkg/validator) | 60s |
| 03 | pkg/constraint/tree.go:~154 | otherDefinitions: own definition `if self != nil {` -> `if false && self != nil {` | KILLED | TestDescentOwnDefinition (./pkg/validator) | 67s |
| 04 | pkg/constraint/tree.go:~365 | walkPrimitiveExtensions: node/SliceOf children loop guarded by `false &&` | KILLED | TestDescentPrimitiveExtensionSlice (./pkg/validator) | 73s |
| 05 | pkg/constraint/tree.go:~309 | walkLayers: layer walk guard `isObject && !own &&` restored | KILLED | TestDescentNamedProbes (./pkg/validator) | 57s |
| 06 | pkg/constraint/layers.go:~82 | firstDescent: `done` never true (`done && false`) | KILLED | TestDescentEachPairIsWalkedOnce (./pkg/validator) | 59s |
| 07 | pkg/constraint/tree.go:~119 | walk: open scope always (`len(sets) > 0` -> `true`) | KILLED | TestDescentSingleDefinitionOpensNoScope (./pkg/validator) | 57s |
| 08 | pkg/constraint/tree.go:~119 | walk: never open scope with several definitions (`false && len(sets) > 0`) | KILLED | TestDescentEachPairIsWalkedOnce (./pkg/validator) | 58s |
| 09a | pkg/constraint/layers.go:~34 | note(): layer invariant identity drops severity | KILLED | TestLayersBothOrders (./pkg/validator) | 106s |
| 09a2 | pkg/constraint/report_scope.go:~109 | firstReport: reportKey sev dropped | KILLED | TestDescentDerivedProfile (./pkg/validator) | 66s |
| 09a3 | pkg/constraint/tree.go:~1057 | pendingConstraints memoKey: sev dropped | KILLED | TestDescentDerivedProfile (./pkg/validator) | 52s |
| 09b | pkg/constraint/tree.go:~1057 | pendingConstraints memoKey: location dropped (at: "") | KILLED | TestDescentArraysAndDepth (./pkg/validator) | 67s |
| 10 | pkg/constraint/tree.go:~300 | walkLayers: own-children walk moved after mergeLayers | SURVIVED |  | 109s |
| 11a | pkg/constraint/tree.go:~565 | validateNested: holderDefinitions chain h = h.SliceOf -> h = nil | KILLED | TestDescentHeldResource (./pkg/validator) | 63s |
| 11b | pkg/constraint/tree.go:~575 | validateNested: nested pair dedup disabled (`done && false`) | SURVIVED |  | 99s |
| 12a | pkg/constraint/tree.go:~821 | checkType: no dedup (`false && !firstReportKey`) | KILLED | TestDescentNamedProbes (./pkg/validator) | 60s |
| 12b | pkg/constraint/tree.go:~821 | checkType: dedup key also includes sd.URL | KILLED | TestDescentNamedProbes (./pkg/validator) | 55s |
| 13a | pkg/registry/snapshot.go:~1338 | yieldCloned: no-op (return nil first) | KILLED | TestDescentSlicedElementAndSlice (./pkg/validator) | 55s |
| 13b | pkg/registry/snapshot.go:~1351 | yieldCloned: also yields inherited keys (drop baseKeys test) | KILLED | TestDescentSeverities (./pkg/validator) | 56s |
| 14 | pkg/registry/snapshot.go:~1404 | stampConstraintSources: no-op | KILLED | TestDescentSlicedElementAndSlice (./pkg/validator) | 56s |
| 15a | pkg/constraint/report_scope.go:~180 | CollapseSeverities: collapse also layered (`&& !layered[k]` removed) | SURVIVED |  | 97s |
| 15b | pkg/constraint/report_scope.go:~174 | CollapseSeverities: returns input unchanged | KILLED | TestDescentSlicedElementAndSlice (./pkg/validator) | 52s |
| 16 | pkg/validator/testdata/definition_layers/expected.json:~7 | expected.json: first "detail": "gk" -> "gX" | KILLED | TestDescentTypeProfileInvariantFocus (./pkg/validator) | 11s |
| 17 | pkg/constraint/constraint.go:~577 | typeBelow: loop replaced by TypeOf(ResolvePath(defPath)) (full path) | KILLED | TestDescentTypeProfileInvariantFocus (./pkg/validator) | 52s |
| 18 | pkg/slicematch/slicematch.go:~230 | discriminatorsOf: ownValue ignored everywhere (no implied discriminators) | KILLED | TestDescentNamedProbes (./pkg/validator) | 52s |
| 18b | pkg/slicematch/slicematch.go:~236 | discriminatorsOf: only descendants ignored (slice itself still) | KILLED | TestDescentNamedProbes (./pkg/validator) | 56s |
| 19 | pkg/slicematch/path.go:~574 | children(): overlayFixed(profiled[:1], ...) only first type profile | SURVIVED |  | 97s |
| 20 | pkg/registry/snapshot.go:~847 | carryDeclaredValues: no-op | KILLED | TestDescentSlicedElementAndSlice (./pkg/validator) | 59s |

Baseline: all targeted tests pass unmodified (constraint, registry, slicematch, TestDescent in validator). Stages: 1) constraint/registry/slicematch, 2) -run TestDescent validator, 3) full ./pkg/validator. Patches in mutpatches/NN.diff. Files restored and verified identical after each run.
Notes: 9 run as 4 variants (09a layer-note identity without severity; 09a2 firstReport without severity; 09a3 memo without severity; 09b memo without location). 15a needed `_ = layered` to compile (first try BUILD-FAIL, rerun). 18: no implied discriminators (slice matches nothing); 18b: only descendants ignored. 09a was killed only by the full-package stage (TestLayersBothOrders), not by TestDescent.

## Survivors

- 10 (own children walked after the layers): same issue set; order is normalized and own-vs-layer evaluation only differs when a key is evaluated once under a different focus (plan punto 5). No probe has the same invariant declared in the own child and in a layer child with an outcome depending on the focus. Not an equivalent mutant; a probe with a type-profile layer whose child redeclares an own-child key with a focus-dependent expression would kill it.
- 11b (nested (resource, profile) pair revalidated): report-scope dedup (firstReport) hides the repeat, so only cost changes. Killed only by an exact-count probe: a held resource reachable through two definitions with Begin/Descent counted per (resource, profile) == 1. Existing counting tests count Descent per (value, owner) and the scope, not nested validations.
- 15a (CollapseSeverities also collapses layered reports): no test has the same constraint failing at the same location, text equal, as warning in one type-profile layer and error in another (which HL7 keeps as two). A probe with two type profiles giving the same key at different severities, expecting both issues, would kill it.
- 19 (overlayFixed with only the first type profile): no slicing probe has an element with two type profiles where the slice matches only via the second alternative with the element's own fixed/pattern child overriding. A slicematch unit test with such a slice (instance conforming to profile B only) would kill it.
