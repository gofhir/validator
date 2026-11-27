| id | file:line | edit | result | first failing test | stage |
|---|---|---|---|---|---|
| 01 | pkg/constraint/tree.go:146 | otherDefinitions chain walk n = n.SliceOf -> n = nil | KILLED | TestDescentSlicedElementAndSlice | 2 |
| 02 | pkg/constraint/tree.go:152 | otherDefinitions type-profile branch guarded by false && | KILLED | TestDescentNamedProbes | 2 |
| 03 | pkg/constraint/tree.go:155 | otherDefinitions own definition guarded by false && | KILLED | TestDescentOwnDefinition | 2 |
| 04 | pkg/constraint/tree.go:366 | walkPrimitiveExtensions children loop guarded by false && | KILLED | TestDescentPrimitiveExtensionSlice | 2 |
| 05 | pkg/constraint/tree.go:310 | walkLayers layer walk guard !own restored | KILLED | TestDescentNamedProbes | 2 |
| 06 | pkg/constraint/layers.go:82 | firstDescent done never true | KILLED | TestDescentEachPairIsWalkedOnce | 2 |
| 07 | pkg/constraint/tree.go:120 | walk opens scope always (len(sets)>0 -> true) | KILLED | TestDescentSingleDefinitionOpensNoScope | 2 |
| 08 | pkg/constraint/tree.go:120 | walk never opens scope with several definitions | KILLED | TestDescentEachPairIsWalkedOnce | 2 |
| 09a | pkg/constraint/layers.go:34 | layers note(): identity drops severity | KILLED | TestMutationProbeSeveritiesAcrossLayersAreKept | 2 |
| 09b | pkg/constraint/tree.go:1092 | pendingConstraints memoKey location dropped (at: "") | KILLED | TestDescentArraysAndDepth | 2 |
| 10 | pkg/constraint/tree.go:301 | walkLayers own-children walk moved after mergeLayers | KILLED | TestMutationProbeOwnChildBeforeLayers | 2 |
| 11a | pkg/constraint/tree.go:570 | validateNested holder chain h = h.SliceOf -> h = nil | KILLED | TestDescentHeldResource | 2 |
| 11b | pkg/constraint/tree.go:580 | validateNested nested pair dedup disabled | KILLED | TestHeldResourceIsValidatedOncePerProfile | 2 |
| 12a | pkg/constraint/tree.go:853 | checkType no dedup (false &&) | KILLED | TestDescentNamedProbes | 2 |
| 12b | pkg/constraint/tree.go:853 | checkType dedup key also includes sd.URL | KILLED | TestDescentNamedProbes | 2 |
| 13a | pkg/registry/snapshot.go:1337 | yieldCloned no-op | KILLED | TestDescentSlicedElementAndSlice | 2 |
| 13b | pkg/registry/snapshot.go:1351 | yieldCloned also yields inherited keys | KILLED | TestDescentSeverities | 2 |
| 14 | pkg/registry/snapshot.go:1404 | stampConstraintSources no-op | KILLED | TestDescentDerivedProfile | 2 |
| 15a | pkg/constraint/report_scope.go:198 | CollapseSeverities collapses layered too | KILLED | TestMutationProbeSeveritiesAcrossLayersAreKept | 2 |
| 15b | pkg/constraint/report_scope.go:173 | CollapseSeverities returns input | KILLED | TestDescentSlicedElementAndSlice | 2 |
| 16 | pkg/validator/testdata/definition_layers/expected.json:7 | expected.json first detail gk -> gX | KILLED | TestDescentTypeProfileInvariantFocus | 2 |
| 17 | pkg/constraint/constraint.go:583 | typeBelow uses full path TypeOf(ResolvePath(defPath)) | KILLED | TestDescentTypeProfileInvariantFocus | 2 |
| 18 | pkg/slicematch/slicematch.go:236 | discriminatorsOf ownValue ignored everywhere | KILLED | TestDescentNamedProbes | 2 |
| 18b | pkg/slicematch/slicematch.go:236 | discriminatorsOf ownValue ignored on descendants only | KILLED | TestDescentNamedProbes | 2 |
| 19 | pkg/slicematch/path.go:574 | children() overlayFixed only first type profile | KILLED | TestExtensionURLThroughSeveralProfilesWithOwnChildren | 1 |
| 20 | pkg/registry/snapshot.go:846 | carryDeclaredValues no-op | KILLED | TestDescentSlicedElementAndSlice | 2 |
| 21 | pkg/constraint/constraint.go:662 | reportedSource returns c.Source always | KILLED | TestDescentSlicedElementAndSlice | 2 |
| 22 | pkg/constraint/constraint.go:667 | reportedSource without publishedSource rule | KILLED | TestConstraintDefinedIn | 3 |
| 23 | pkg/constraint/constraint.go:672 | reportedSource without own-constraint-of-non-validated-def gets owner.URL rule | KILLED | TestDescentSlicedElementAndSlice | 2 |
| 24 | pkg/registry/snapshot.go:1404 | stampConstraintSources stamps own constraints with the profile URL | KILLED | TestDescentOwnDefinition | 2 |
| 25 | pkg/constraint/tree.go:564 | validateNested without the divided check | KILLED | TestHeldResourceIsValidatedOncePerProfile | 2 |
| 26 | pkg/constraint/tree.go:584 | walkprobe.Nested event never emitted | KILLED | TestHeldResourceIsValidatedOncePerProfile | 2 |
| 09a2 | pkg/constraint/report_scope.go:109 | firstReport reportKey sev dropped | KILLED | TestDescentDerivedProfile | 2 |
| 09a3 | pkg/constraint/tree.go:1092 | pendingConstraints memoKey sev dropped | KILLED | TestDescentDerivedProfile | 2 |

Stages: 1 = constraint/registry/slicematch; 2 = -run filter on ./pkg/validator; 3 = full ./pkg/validator. Baseline all green. Patches in mutpatches2/NN.diff. Adaptations: 15a needs `_ = layered` to compile (first try BUILD-FAIL, rerun); 24 adds a profileURL parameter to stampConstraintSources (caller passes sd.URL) and uses it as last fallback of cmp.Or; 10 re-created by moving the walkOwn block after mergeLayers; line numbers are the current code's.

Survivors: none
