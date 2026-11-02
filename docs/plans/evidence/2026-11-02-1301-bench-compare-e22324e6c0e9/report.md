# Benchmark comparison

- Date: 2026-11-02 13:13 -03
- Base: `e22324e6c0e9` · Head: `e22324e6c0e9` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 6 · serial benchmarks: 1 CPU · parallel: 1,8 · benchtime: 1s
- Command: `BASE=e22324e COUNT=6 CPUS=1,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt    │              head.txt               │
                                             │    sec/op     │    sec/op      vs base              │
ValidatePatientWithData                        194.5µ ±  43%   159.9µ ±  33%       ~ (p=0.589 n=6)
ValidateObservation                            150.1µ ± 149%   134.4µ ±  28%       ~ (p=0.240 n=6)
ValidatorCreation                               1.094 ± 156%    1.430 ±  55%       ~ (p=0.937 n=6)
ValidateRotating                               796.4µ ±  74%   751.1µ ±  45%       ~ (p=0.699 n=6)
ValidatorLiveHeap                               1.106 ± 104%    1.147 ± 102%       ~ (p=0.937 n=6)
ValidateProfiled/patient                       601.2µ ± 160%   528.7µ ±  57%       ~ (p=0.589 n=6)
ValidateProfiled/blood-pressure                399.5µ ± 234%   371.7µ ± 268%       ~ (p=0.818 n=6)
ValidateProfiled/extension-values              2.253m ± 135%   2.483m ±  86%       ~ (p=0.699 n=6)
ValidateParallel                               44.08µ ±  52%   42.71µ ± 135%       ~ (p=0.699 n=6)
ValidateParallel-8                             14.38µ ± 114%   14.11µ ± 117%       ~ (p=1.000 n=6)
ValidateProfiled/patient-parallel              718.0µ ±  49%   602.3µ ± 232%       ~ (p=0.589 n=6)
ValidateProfiled/patient-parallel-8            176.2µ ±  81%   157.3µ ±  94%       ~ (p=0.699 n=6)
ValidateProfiled/blood-pressure-parallel       411.8µ ± 106%   442.7µ ±  91%       ~ (p=1.000 n=6)
ValidateProfiled/blood-pressure-parallel-8     130.5µ ±  32%   136.0µ ±  82%       ~ (p=0.818 n=6)
ValidateProfiled/extension-values-parallel     2.622m ±  56%   2.405m ±  66%       ~ (p=0.818 n=6)
ValidateProfiled/extension-values-parallel-8   786.6µ ±  46%   954.2µ ±  61%       ~ (p=1.000 n=6)
geomean                                        882.5µ          866.9µ         -1.77%

                                             │   base.txt    │              head.txt               │
                                             │     B/op      │     B/op       vs base              │
ValidatePatientWithData                        86.28Ki ±  0%   86.28Ki ±  0%       ~ (p=0.732 n=6)
ValidateObservation                            72.95Ki ±  0%   72.88Ki ±  0%  -0.10% (p=0.004 n=6)
ValidatorCreation                              270.5Mi ±  0%   270.6Mi ±  0%       ~ (p=0.132 n=6)
ValidateRotating                               311.1Ki ± 17%   311.0Ki ± 10%       ~ (p=0.361 n=6)
ValidatorLiveHeap                              270.6Mi ±  0%   270.6Mi ±  0%       ~ (p=0.937 n=6)
ValidateProfiled/patient                       315.7Ki ±  0%   315.5Ki ±  0%  -0.05% (p=0.002 n=6)
ValidateProfiled/blood-pressure                236.0Ki ±  0%   235.9Ki ±  0%  -0.04% (p=0.002 n=6)
ValidateProfiled/extension-values              1.366Mi ±  0%   1.365Mi ±  0%  -0.08% (p=0.002 n=6)
ValidateParallel                               24.62Ki ±  0%   24.61Ki ±  0%       ~ (p=0.937 n=6)
ValidateParallel-8                             24.58Ki ±  0%   24.57Ki ±  0%       ~ (p=0.669 n=6)
ValidateProfiled/patient-parallel              315.7Ki ±  0%   315.5Ki ±  0%  -0.05% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            315.7Ki ±  0%   315.6Ki ±  0%  -0.05% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ±  0%   235.9Ki ±  0%  -0.04% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ±  0%   235.9Ki ±  0%  -0.04% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel     1.366Mi ±  0%   1.365Mi ±  0%  -0.08% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   1.366Mi ±  0%   1.365Mi ±  0%  -0.08% (p=0.002 n=6)
geomean                                        563.4Ki         563.1Ki        -0.04%

                                             │   base.txt   │               head.txt               │
                                             │  allocs/op   │  allocs/op    vs base                │
ValidatePatientWithData                        1.754k ±  0%   1.754k ±  0%       ~ (p=1.000 n=6) ¹
ValidateObservation                            1.358k ±  0%   1.354k ±  0%  -0.29% (p=0.002 n=6)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.061 n=6)
ValidateRotating                               5.533k ± 17%   5.529k ± 10%       ~ (p=0.396 n=6)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.844 n=6)
ValidateProfiled/patient                       6.142k ±  0%   6.132k ±  0%  -0.16% (p=0.002 n=6)
ValidateProfiled/blood-pressure                4.334k ±  0%   4.328k ±  0%  -0.14% (p=0.002 n=6)
ValidateProfiled/extension-values              26.12k ±  0%   26.05k ±  0%  -0.28% (p=0.002 n=6)
ValidateParallel                                393.0 ±  0%    393.0 ±  0%       ~ (p=1.000 n=6) ¹
ValidateParallel-8                              393.0 ±  0%    393.0 ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel              6.142k ±  0%   6.132k ±  0%  -0.16% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            6.142k ±  0%   6.132k ±  0%  -0.16% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       4.334k ±  0%   4.328k ±  0%  -0.14% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     4.334k ±  0%   4.328k ±  0%  -0.14% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel     26.12k ±  0%   26.05k ±  0%  -0.28% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   26.12k ±  0%   26.05k ±  0%  -0.28% (p=0.002 n=6)
geomean                                        8.637k         8.626k        -0.13%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating    3.719k ± 48%    3.090k ± 60%  ~ (p=0.394 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.98Mi ± 0%   92.95Mi ± 0%  ~ (p=0.236 n=6)
```
