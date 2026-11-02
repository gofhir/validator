# Benchmark comparison

- Date: 2026-11-02 11:57 -03
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
                                             │   base.txt    │               head.txt               │
                                             │    sec/op     │     sec/op      vs base              │
ValidatePatientWithData                        121.6µ ±  84%    134.6µ ± 437%       ~ (p=0.818 n=6)
ValidateObservation                            109.6µ ±  62%    128.2µ ± 332%       ~ (p=0.394 n=6)
ValidatorCreation                               1.010 ± 122%     1.104 ± 273%       ~ (p=0.394 n=6)
ValidateRotating                               470.7µ ±  95%    622.1µ ± 212%       ~ (p=0.699 n=6)
ValidatorLiveHeap                              920.6m ± 104%   1055.3m ± 189%       ~ (p=0.937 n=6)
ValidateProfiled/patient                       613.0µ ±  62%    728.1µ ±  44%       ~ (p=0.818 n=6)
ValidateProfiled/blood-pressure                459.0µ ±  49%    420.8µ ±  76%       ~ (p=1.000 n=6)
ValidateProfiled/extension-values              2.113m ±  36%    2.411m ±  90%       ~ (p=0.485 n=6)
ValidateParallel                               45.38µ ±  61%    46.49µ ±  34%       ~ (p=1.000 n=6)
ValidateParallel-8                             17.93µ ±  61%    14.63µ ±  92%       ~ (p=0.699 n=6)
ValidateProfiled/patient-parallel              512.7µ ± 305%    485.7µ ± 204%       ~ (p=0.699 n=6)
ValidateProfiled/patient-parallel-8            191.2µ ± 549%    194.8µ ±  49%       ~ (p=1.000 n=6)
ValidateProfiled/blood-pressure-parallel       370.7µ ± 465%    381.6µ ± 188%       ~ (p=0.937 n=6)
ValidateProfiled/blood-pressure-parallel-8     124.7µ ± 235%    137.4µ ±  93%       ~ (p=0.937 n=6)
ValidateProfiled/extension-values-parallel     3.168m ±  46%    2.335m ± 104%       ~ (p=0.937 n=6)
ValidateProfiled/extension-values-parallel-8   824.3µ ± 172%    931.2µ ±  57%       ~ (p=0.818 n=6)
geomean                                        808.7µ           845.2µ         +4.51%

                                             │   base.txt    │              head.txt               │
                                             │     B/op      │     B/op       vs base              │
ValidatePatientWithData                        86.24Ki ±  0%   86.24Ki ±  1%       ~ (p=1.000 n=6)
ValidateObservation                            72.92Ki ±  0%   72.89Ki ±  0%       ~ (p=0.370 n=6)
ValidatorCreation                              270.7Mi ±  0%   270.6Mi ±  0%       ~ (p=0.065 n=6)
ValidateRotating                               311.1Ki ± 21%   299.0Ki ± 17%       ~ (p=0.275 n=6)
ValidatorLiveHeap                              270.6Mi ±  0%   270.6Mi ±  0%       ~ (p=0.584 n=6)
ValidateProfiled/patient                       315.7Ki ±  0%   315.5Ki ±  0%  -0.05% (p=0.002 n=6)
ValidateProfiled/blood-pressure                236.0Ki ±  0%   235.9Ki ±  0%  -0.04% (p=0.002 n=6)
ValidateProfiled/extension-values              1.366Mi ±  0%   1.365Mi ±  0%  -0.08% (p=0.002 n=6)
ValidateParallel                               24.61Ki ±  0%   24.62Ki ±  0%       ~ (p=0.699 n=6)
ValidateParallel-8                             24.57Ki ±  0%   24.57Ki ±  0%       ~ (p=1.000 n=6)
ValidateProfiled/patient-parallel              315.7Ki ±  0%   315.5Ki ±  0%  -0.05% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            315.7Ki ±  0%   315.6Ki ±  0%  -0.05% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ±  0%   235.9Ki ±  0%  -0.04% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ±  0%   235.9Ki ±  0%  -0.04% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel     1.366Mi ±  0%   1.365Mi ±  0%  -0.08% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   1.366Mi ±  0%   1.365Mi ±  0%  -0.08% (p=0.002 n=6)
geomean                                        563.3Ki         561.8Ki        -0.28%

                                             │   base.txt   │               head.txt               │
                                             │  allocs/op   │  allocs/op    vs base                │
ValidatePatientWithData                        1.754k ±  0%   1.754k ±  0%       ~ (p=1.000 n=6)
ValidateObservation                            1.357k ±  0%   1.354k ±  0%  -0.26% (p=0.002 n=6)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.195 n=6)
ValidateRotating                               5.533k ± 21%   5.332k ± 17%       ~ (p=0.301 n=6)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.695 n=6)
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
geomean                                        8.637k         8.606k        -0.35%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating   2.600k ± 135%   3.170k ± 141%  ~ (p=0.394 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.99Mi ± 0%   93.00Mi ± 0%  ~ (p=0.805 n=6)
```
