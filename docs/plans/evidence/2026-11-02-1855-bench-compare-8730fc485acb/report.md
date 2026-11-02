# Benchmark comparison

- Date: 2026-11-02 19:04 -03
- Base: `8730fc485acb` · Head: `8730fc485acb` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 6 · serial benchmarks: 1 CPU · parallel: 1,8 · benchtime: 1s
- Command: `BASE=8730fc4 COUNT=6 CPUS=1,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │    base.txt    │              head.txt               │
                                             │     sec/op     │    sec/op      vs base              │
ValidatePatientWithData                         113.5µ ±   5%    113.5µ ± 25%       ~ (p=0.818 n=6)
ValidateObservation                             92.03µ ±  33%   100.32µ ± 19%       ~ (p=0.240 n=6)
ValidatorCreation                               789.6m ±  13%    902.2m ± 16%       ~ (p=0.394 n=6)
ValidateRotating                                426.1µ ±  20%    451.0µ ± 11%       ~ (p=0.394 n=6)
ValidatorLiveHeap                               746.9m ±  22%    797.8m ± 32%       ~ (p=0.240 n=6)
ValidateProfiled/patient                        485.9µ ± 244%    410.0µ ± 20%       ~ (p=0.180 n=6)
ValidateProfiled/blood-pressure                 342.1µ ±  62%    307.9µ ±  7%       ~ (p=0.093 n=6)
ValidateProfiled/extension-values               1.869m ±  10%    1.811m ± 12%       ~ (p=0.485 n=6)
ValidateParallel                                33.88µ ±  16%    33.75µ ± 74%       ~ (p=0.937 n=6)
ValidateParallel-8                             12.112µ ±  25%    9.980µ ± 78%       ~ (p=0.180 n=6)
ValidateProfiled/patient-parallel               417.9µ ±  41%    412.4µ ±  9%       ~ (p=0.699 n=6)
ValidateProfiled/patient-parallel-8             146.5µ ±  23%    140.8µ ± 27%       ~ (p=0.485 n=6)
ValidateProfiled/blood-pressure-parallel        317.8µ ±  11%    327.4µ ± 47%       ~ (p=0.394 n=6)
ValidateProfiled/blood-pressure-parallel-8      110.3µ ±  24%    114.0µ ± 27%       ~ (p=0.937 n=6)
ValidateProfiled/extension-values-parallel      1.852m ±   8%    1.745m ± 74%       ~ (p=0.818 n=6)
ValidateProfiled/extension-values-parallel-8    622.5µ ±  33%    534.4µ ± 75%       ~ (p=0.310 n=6)
geomean                                         641.8µ           627.3µ        -2.26%

                                             │   base.txt    │               head.txt               │
                                             │     B/op      │     B/op      vs base                │
ValidatePatientWithData                        86.22Ki ±  0%   86.22Ki ± 0%       ~ (p=0.455 n=6)
ValidateObservation                            72.82Ki ±  0%   72.85Ki ± 0%       ~ (p=0.320 n=6)
ValidatorCreation                              270.5Mi ±  0%   270.6Mi ± 0%       ~ (p=0.420 n=6)
ValidateRotating                               311.0Ki ± 21%   311.1Ki ± 0%  +0.02% (p=0.002 n=6)
ValidatorLiveHeap                              270.6Mi ±  0%   270.6Mi ± 0%       ~ (p=0.558 n=6)
ValidateProfiled/patient                       315.5Ki ±  0%   315.5Ki ± 0%       ~ (p=1.000 n=6)
ValidateProfiled/blood-pressure                235.9Ki ±  0%   235.9Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              1.365Mi ±  0%   1.365Mi ± 0%       ~ (p=0.578 n=6)
ValidateParallel                               24.61Ki ±  0%   24.60Ki ± 0%       ~ (p=0.792 n=6)
ValidateParallel-8                             24.57Ki ±  0%   24.57Ki ± 0%       ~ (p=0.398 n=6)
ValidateProfiled/patient-parallel              315.5Ki ±  0%   315.5Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel-8            315.6Ki ±  0%   315.6Ki ± 0%       ~ (p=0.513 n=6)
ValidateProfiled/blood-pressure-parallel       235.9Ki ±  0%   235.9Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     235.9Ki ±  0%   235.9Ki ± 0%       ~ (p=0.853 n=6)
ValidateProfiled/extension-values-parallel     1.365Mi ±  0%   1.365Mi ± 0%       ~ (p=1.000 n=6)
ValidateProfiled/extension-values-parallel-8   1.365Mi ±  0%   1.365Mi ± 0%       ~ (p=0.463 n=6)
geomean                                        563.1Ki         563.1Ki       +0.01%
¹ all samples are equal

                                             │   base.txt   │              head.txt               │
                                             │  allocs/op   │  allocs/op   vs base                │
ValidatePatientWithData                        1.754k ±  0%   1.754k ± 0%       ~ (p=1.000 n=6)
ValidateObservation                            1.353k ±  0%   1.353k ± 0%       ~ (p=1.000 n=6)
ValidatorCreation                              1.184M ±  0%   1.184M ± 0%       ~ (p=0.511 n=6)
ValidateRotating                               5.529k ± 21%   5.530k ± 0%  +0.02% (p=0.002 n=6)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ± 0%       ~ (p=0.853 n=6)
ValidateProfiled/patient                       6.132k ±  0%   6.132k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure                4.328k ±  0%   4.328k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              26.05k ±  0%   26.05k ± 0%       ~ (p=1.000 n=6) ¹
ValidateParallel                                393.0 ±  0%    393.0 ± 0%       ~ (p=1.000 n=6) ¹
ValidateParallel-8                              393.0 ±  0%    393.0 ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel              6.132k ±  0%   6.132k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel-8            6.132k ±  0%   6.132k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel       4.328k ±  0%   4.328k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     4.328k ±  0%   4.328k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel     26.05k ±  0%   26.05k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel-8   26.05k ±  0%   26.05k ± 0%       ~ (p=1.000 n=6) ¹
geomean                                        8.625k         8.625k       +0.00%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating    2.341k ± 19%    2.189k ± 11%  ~ (p=0.558 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   93.00Mi ± 0%   92.98Mi ± 0%  ~ (p=0.513 n=6)
```
