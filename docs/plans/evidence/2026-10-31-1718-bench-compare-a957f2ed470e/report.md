# Benchmark comparison

- Date: 2026-10-31 17:27 -03
- Base: `a957f2ed470e` · Head: `a957f2ed470e` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 6 · serial benchmarks: 1 CPU · parallel: 1,8 · benchtime: 1s
- Command: `BASE=a957f2e COUNT=6 CPUS=1,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt   │              head.txt               │
                                             │    sec/op    │    sec/op      vs base              │
ValidatePatientWithData                        107.8µ ±  7%    110.5µ ± 17%       ~ (p=0.310 n=6)
ValidateObservation                            89.04µ ± 33%    89.63µ ±  6%       ~ (p=0.310 n=6)
ValidatorCreation                              731.8m ±  7%    732.3m ±  4%       ~ (p=1.000 n=6)
ValidateRotating                               421.4µ ±  4%    429.7µ ± 24%       ~ (p=0.065 n=6)
ValidatorLiveHeap                              752.9m ±  5%    752.8m ±  9%       ~ (p=0.589 n=6)
ValidateProfiled/patient                       406.3µ ± 23%    404.0µ ± 10%       ~ (p=0.485 n=6)
ValidateProfiled/blood-pressure                307.7µ ± 44%    302.6µ ± 11%       ~ (p=0.818 n=6)
ValidateProfiled/extension-values              1.690m ± 12%    1.732m ± 20%       ~ (p=0.065 n=6)
ValidateParallel                               30.65µ ±  1%    31.46µ ±  6%  +2.65% (p=0.004 n=6)
ValidateParallel-8                             7.366µ ± 59%    8.509µ ± 30%       ~ (p=0.180 n=6)
ValidateProfiled/patient-parallel              390.8µ ±  7%    414.7µ ±  9%       ~ (p=0.180 n=6)
ValidateProfiled/patient-parallel-8            92.78µ ± 31%   101.24µ ± 30%       ~ (p=0.699 n=6)
ValidateProfiled/blood-pressure-parallel       292.7µ ± 12%    294.7µ ± 32%       ~ (p=0.485 n=6)
ValidateProfiled/blood-pressure-parallel-8     63.32µ ±  7%    69.38µ ±  8%  +9.57% (p=0.041 n=6)
ValidateProfiled/extension-values-parallel     1.673m ±  6%    1.703m ± 20%       ~ (p=0.394 n=6)
ValidateProfiled/extension-values-parallel-8   403.9µ ± 13%    396.3µ ± 18%       ~ (p=0.937 n=6)
geomean                                        537.3µ          553.2µ        +2.96%

                                             │   base.txt   │               head.txt               │
                                             │     B/op     │     B/op      vs base                │
ValidatePatientWithData                        86.19Ki ± 0%   86.19Ki ± 0%       ~ (p=0.844 n=6)
ValidateObservation                            72.86Ki ± 0%   73.10Ki ± 0%  +0.33% (p=0.002 n=6)
ValidatorCreation                              270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.905 n=6)
ValidateRotating                               313.2Ki ± 0%   315.5Ki ± 0%  +0.74% (p=0.002 n=6)
ValidatorLiveHeap                              270.7Mi ± 0%   270.6Mi ± 0%       ~ (p=0.258 n=6)
ValidateProfiled/patient                       315.7Ki ± 0%   315.7Ki ± 0%       ~ (p=0.545 n=6)
ValidateProfiled/blood-pressure                236.0Ki ± 0%   236.0Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              1.367Mi ± 0%   1.367Mi ± 0%       ~ (p=0.675 n=6)
ValidateParallel                               24.58Ki ± 0%   24.59Ki ± 0%       ~ (p=0.158 n=6)
ValidateParallel-8                             24.55Ki ± 0%   24.55Ki ± 0%       ~ (p=0.907 n=6)
ValidateProfiled/patient-parallel              315.7Ki ± 0%   315.7Ki ± 0%       ~ (p=1.000 n=6)
ValidateProfiled/patient-parallel-8            315.7Ki ± 0%   315.7Ki ± 0%       ~ (p=0.452 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ± 0%   236.0Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ± 0%   236.0Ki ± 0%       ~ (p=0.394 n=6)
ValidateProfiled/extension-values-parallel     1.367Mi ± 0%   1.367Mi ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel-8   1.367Mi ± 0%   1.367Mi ± 0%       ~ (p=0.472 n=6)
geomean                                        563.5Ki        563.9Ki       +0.06%
¹ all samples are equal

                                             │  base.txt   │              head.txt               │
                                             │  allocs/op  │  allocs/op   vs base                │
ValidatePatientWithData                        1.753k ± 0%   1.753k ± 0%       ~ (p=1.000 n=6)
ValidateObservation                            1.357k ± 0%   1.359k ± 0%  +0.15% (p=0.002 n=6)
ValidatorCreation                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.939 n=6)
ValidateRotating                               5.579k ± 0%   5.625k ± 0%  +0.82% (p=0.002 n=6)
ValidatorLiveHeap                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.472 n=6)
ValidateProfiled/patient                       6.141k ± 0%   6.141k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure                4.334k ± 0%   4.334k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              26.14k ± 0%   26.14k ± 0%       ~ (p=1.000 n=6) ¹
ValidateParallel                                393.0 ± 0%    393.0 ± 0%       ~ (p=1.000 n=6) ¹
ValidateParallel-8                              393.0 ± 0%    393.0 ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel              6.141k ± 0%   6.141k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel-8            6.141k ± 0%   6.141k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel       4.334k ± 0%   4.334k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     4.334k ± 0%   4.334k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel     26.14k ± 0%   26.14k ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel-8   26.14k ± 0%   26.14k ± 0%       ~ (p=1.000 n=6) ¹
geomean                                        8.642k        8.647k       +0.06%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating     2.144k ± 6%    2.182k ± 47%  ~ (p=0.485 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.99Mi ± 0%   92.98Mi ± 0%  ~ (p=0.924 n=6)
```
