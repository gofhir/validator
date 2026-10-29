# Benchmark comparison

- Date: 2026-10-28 23:56 -03
- Base: `548ad1f61d1d` · Head: `548ad1f61d1d` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=548ad1f COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt   │               head.txt                │
                                             │    sec/op    │     sec/op      vs base               │
ValidatePatientWithData                        171.1µ ± 33%    223.5µ ±  48%       ~ (p=0.089 n=10)
ValidateObservation                            140.0µ ± 90%    177.2µ ±  94%       ~ (p=0.684 n=10)
ValidatorCreation                               1.137 ± 32%     1.369 ±  34%       ~ (p=0.684 n=10)
ValidateRotating                               673.7µ ± 50%    637.8µ ±  92%       ~ (p=0.631 n=10)
ValidatorLiveHeap                               1.833 ± 61%     1.263 ±  94%       ~ (p=0.315 n=10)
ValidateProfiled/patient                       779.3µ ± 40%    619.1µ ±  84%       ~ (p=0.971 n=10)
ValidateProfiled/blood-pressure                483.3µ ± 26%    453.3µ ±  84%       ~ (p=0.684 n=10)
ValidateProfiled/extension-values              2.689m ± 22%    2.925m ±  42%       ~ (p=0.436 n=10)
ValidateParallel                               48.01µ ± 54%    60.52µ ±  68%       ~ (p=0.481 n=10)
ValidateParallel-4                             25.73µ ± 33%    31.81µ ±  40%       ~ (p=0.684 n=10)
ValidateParallel-8                             21.32µ ± 27%    20.65µ ±  24%       ~ (p=0.739 n=10)
ValidateProfiled/patient-parallel              602.0µ ± 69%    538.5µ ±  88%       ~ (p=0.631 n=10)
ValidateProfiled/patient-parallel-4            300.3µ ± 36%    296.6µ ±  29%       ~ (p=0.971 n=10)
ValidateProfiled/patient-parallel-8            241.1µ ± 28%    197.4µ ±  38%       ~ (p=0.089 n=10)
ValidateProfiled/blood-pressure-parallel       437.8µ ± 85%    410.8µ ± 215%       ~ (p=0.631 n=10)
ValidateProfiled/blood-pressure-parallel-4     202.6µ ± 70%    239.0µ ±  31%       ~ (p=0.393 n=10)
ValidateProfiled/blood-pressure-parallel-8     172.1µ ± 31%    178.5µ ±  29%       ~ (p=0.684 n=10)
ValidateProfiled/extension-values-parallel     2.333m ± 45%    2.859m ±  59%       ~ (p=0.218 n=10)
ValidateProfiled/extension-values-parallel-4   1.170m ± 59%    1.320m ±  30%       ~ (p=0.579 n=10)
ValidateProfiled/extension-values-parallel-8   984.5µ ± 36%   1007.4µ ±  28%       ~ (p=0.684 n=10)
geomean                                        724.8µ          747.8µ         +3.17%

                                             │   base.txt    │              head.txt               │
                                             │     B/op      │     B/op      vs base               │
ValidatePatientWithData                        83.49Ki ±  0%   86.26Ki ± 0%  +3.32% (p=0.000 n=10)
ValidateObservation                            70.67Ki ±  0%   72.99Ki ± 0%  +3.28% (p=0.000 n=10)
ValidatorCreation                              270.6Mi ±  0%   270.6Mi ± 0%       ~ (p=0.239 n=10)
ValidateRotating                               310.5Ki ± 12%   299.8Ki ± 9%       ~ (p=0.839 n=10)
ValidatorLiveHeap                              270.6Mi ±  0%   270.6Mi ± 0%       ~ (p=0.957 n=10)
ValidateProfiled/patient                       309.2Ki ±  0%   315.7Ki ± 0%  +2.10% (p=0.000 n=10)
ValidateProfiled/blood-pressure                230.0Ki ±  0%   236.0Ki ± 0%  +2.58% (p=0.000 n=10)
ValidateProfiled/extension-values              1.349Mi ±  0%   1.375Mi ± 0%  +1.98% (p=0.000 n=10)
ValidateParallel                               23.97Ki ±  0%   24.61Ki ± 1%  +2.67% (p=0.000 n=10)
ValidateParallel-4                             23.93Ki ±  0%   24.57Ki ± 0%  +2.66% (p=0.000 n=10)
ValidateParallel-8                             23.92Ki ±  0%   24.56Ki ± 0%  +2.68% (p=0.000 n=10)
ValidateProfiled/patient-parallel              309.2Ki ±  0%   315.7Ki ± 0%  +2.10% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4            309.2Ki ±  0%   315.7Ki ± 0%  +2.10% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8            309.2Ki ±  0%   315.7Ki ± 0%  +2.10% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel       230.0Ki ±  0%   236.0Ki ± 0%  +2.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4     230.1Ki ±  0%   236.0Ki ± 0%  +2.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8     230.1Ki ±  0%   236.0Ki ± 0%  +2.58% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel     1.349Mi ±  0%   1.375Mi ± 0%  +1.98% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-4   1.349Mi ±  0%   1.375Mi ± 0%  +1.98% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-8   1.349Mi ±  0%   1.376Mi ± 0%  +1.97% (p=0.000 n=10)
geomean                                        460.0Ki         468.6Ki       +1.88%

                                             │   base.txt   │              head.txt               │
                                             │  allocs/op   │  allocs/op    vs base               │
ValidatePatientWithData                        1.706k ±  0%   1.754k ±  0%  +2.81% (p=0.000 n=10)
ValidateObservation                            1.320k ±  0%   1.358k ±  0%  +2.88% (p=0.000 n=10)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=1.000 n=10)
ValidateRotating                               5.257k ± 12%   5.113k ± 10%       ~ (p=0.839 n=10)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.989 n=10)
ValidateProfiled/patient                       6.010k ±  0%   6.141k ±  0%  +2.18% (p=0.000 n=10)
ValidateProfiled/blood-pressure                4.218k ±  0%   4.334k ±  0%  +2.75% (p=0.000 n=10)
ValidateProfiled/extension-values              25.53k ±  0%   26.14k ±  0%  +2.39% (p=0.000 n=10)
ValidateParallel                                383.0 ±  0%    393.0 ±  0%  +2.61% (p=0.000 n=10)
ValidateParallel-4                              383.0 ±  0%    393.0 ±  0%  +2.61% (p=0.000 n=10)
ValidateParallel-8                              383.0 ±  0%    393.0 ±  0%  +2.61% (p=0.000 n=10)
ValidateProfiled/patient-parallel              6.010k ±  0%   6.141k ±  0%  +2.18% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4            6.010k ±  0%   6.141k ±  0%  +2.18% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8            6.010k ±  0%   6.141k ±  0%  +2.18% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel       4.218k ±  0%   4.334k ±  0%  +2.75% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4     4.218k ±  0%   4.334k ±  0%  +2.75% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8     4.218k ±  0%   4.334k ±  0%  +2.75% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel     25.53k ±  0%   26.14k ±  0%  +2.39% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-4   25.53k ±  0%   26.14k ±  0%  +2.39% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-8   25.53k ±  0%   26.14k ±  0%  +2.39% (p=0.000 n=10)
geomean                                        7.256k         7.401k        +1.99%

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    3.287k ± 30%    3.542k ± 42%  ~ (p=0.796 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.97Mi ± 0%  ~ (p=0.475 n=10)
```
