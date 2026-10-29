Moving gofhir/validator from fhirpath v1.13.6 to v1.16.6 (gofhir/validator#176) raises allocations per validation by 2–3%. Bytes per validation rise by the same amount.

| Benchmark (`pkg/validator`) | v1.13.6 | v1.16.6 |
|---|---|---|
| `ValidatePatientWithData` | 1706 allocs, 85.5 KB | 1754 allocs, 88.3 KB |
| `ValidateObservation` | 1319 allocs, 72.4 KB | 1357 allocs, 74.7 KB |

Only `go.mod` changed between the two runs: fhirpath, and ucum from v4.2.0 to v4.3.4. The validator's code is the same. The profile puts the difference in fhirpath.

I compared the allocation profiles of the two versions (`-memprofilerate=1`, 2000 validations of a Patient). Almost all of the difference is in three places, and each seems avoidable:

1. **`eval.startClock`, about +27 allocs per validation.** Since #226, every evaluation allocates a `&clock{}` and a restore closure (`evaluator.go:170`, called from `evaluator.go:699` and `plan.go:584`), even when the expression never calls `now()`, `today()` or `timeOfDay()`. Possible fix: create the instant lazily on the first call to those functions. Another option is to keep the clock by value in the evaluation, so nothing is allocated when no clock function runs.
2. **`types.(*ObjectValue).childCandidates`, about +35 allocs.** Every child lookup from `childElement` makes a slice and concatenates `t+"."+name` and `basePath+"."+name` (`types/object.go:1868`). Possible fix: try the candidates in order without building the slice, or cache them per (type, name).
3. **`types.Integer.ToDecimal` → `decimal.NewFromInt` → `big.NewInt`, about +13 allocs,** reached from `eval.Compare`. Comparing two Integers goes through a decimal. Possible fix: a fast path for Integer–Integer, and for Integer–Decimal when the decimal fits.

Time per operation shows no significant change in our runs, but the machine was loaded, so timing is not conclusive.

**Reproduction:** in gofhir/validator, compare `go test ./pkg/validator -run '^$' -bench 'BenchmarkValidatePatientWithData$' -benchmem` with `go.mod` at fhirpath v1.13.6 and at v1.16.6. To see the three functions, add `-memprofile` and compare the two profiles with `go tool pprof -diff_base`.
