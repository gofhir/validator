# Benchmark comparison

- Date: 2026-11-01 07:25 -03
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
ValidatePatientWithData                        106.9µ ±  5%   109.5µ ±  54%       ~ (p=0.240 n=6)
ValidateObservation                            86.87µ ±  4%   89.02µ ±  17%       ~ (p=0.065 n=6)
ValidatorCreation                              719.2m ±  3%   741.2m ±  10%  +3.06% (p=0.041 n=6)
ValidateRotating                               418.2µ ±  6%   418.6µ ±  10%       ~ (p=0.937 n=6)
ValidatorLiveHeap                              718.8m ±  2%   725.7m ±   6%       ~ (p=0.394 n=6)
ValidateProfiled/patient                       378.5µ ±  5%   375.7µ ±   4%       ~ (p=0.310 n=6)
ValidateProfiled/blood-pressure                287.0µ ±  6%   290.5µ ±   3%       ~ (p=0.589 n=6)
ValidateProfiled/extension-values              1.635m ±  4%   1.639m ±   8%       ~ (p=0.818 n=6)
ValidateParallel                               30.99µ ±  3%   30.38µ ±   4%       ~ (p=0.699 n=6)
ValidateParallel-8                             6.493µ ± 41%   5.789µ ±  56%       ~ (p=0.394 n=6)
ValidateProfiled/patient-parallel              374.2µ ±  6%   374.0µ ±   9%       ~ (p=0.699 n=6)
ValidateProfiled/patient-parallel-8            74.95µ ± 17%   70.50µ ±  11%       ~ (p=0.093 n=6)
ValidateProfiled/blood-pressure-parallel       288.1µ ± 27%   285.0µ ±   7%       ~ (p=0.818 n=6)
ValidateProfiled/blood-pressure-parallel-8     55.82µ ± 19%   51.37µ ± 104%       ~ (p=0.699 n=6)
ValidateProfiled/extension-values-parallel     1.649m ± 14%   1.636m ±   8%       ~ (p=0.589 n=6)
ValidateProfiled/extension-values-parallel-8   309.0µ ± 30%   328.1µ ±  33%       ~ (p=0.485 n=6)
geomean                                        502.4µ         498.0µ         -0.88%

                                             │   base.txt   │              head.txt              │
                                             │     B/op     │     B/op      vs base              │
ValidatePatientWithData                        86.19Ki ± 0%   86.25Ki ± 0%  +0.07% (p=0.002 n=6)
ValidateObservation                            72.86Ki ± 0%   72.92Ki ± 0%  +0.09% (p=0.002 n=6)
ValidatorCreation                              270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.937 n=6)
ValidateRotating                               313.2Ki ± 0%   315.4Ki ± 0%  +0.71% (p=0.002 n=6)
ValidatorLiveHeap                              270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.377 n=6)
ValidateProfiled/patient                       315.7Ki ± 0%   315.7Ki ± 0%  +0.02% (p=0.002 n=6)
ValidateProfiled/blood-pressure                236.0Ki ± 0%   236.0Ki ± 0%  +0.03% (p=0.002 n=6)
ValidateProfiled/extension-values              1.367Mi ± 0%   1.366Mi ± 0%  -0.07% (p=0.002 n=6)
ValidateParallel                               24.58Ki ± 0%   24.65Ki ± 0%  +0.26% (p=0.002 n=6)
ValidateParallel-8                             24.55Ki ± 0%   24.61Ki ± 0%  +0.25% (p=0.002 n=6)
ValidateProfiled/patient-parallel              315.7Ki ± 0%   315.7Ki ± 0%  +0.02% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            315.7Ki ± 0%   315.8Ki ± 0%  +0.02% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ± 0%   236.0Ki ± 0%  +0.03% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ± 0%   236.1Ki ± 0%  +0.03% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel     1.367Mi ± 0%   1.366Mi ± 0%  -0.07% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   1.367Mi ± 0%   1.366Mi ± 0%  -0.07% (p=0.002 n=6)
geomean                                        563.5Ki        564.0Ki       +0.08%

                                             │  base.txt   │             head.txt              │
                                             │  allocs/op  │  allocs/op   vs base              │
ValidatePatientWithData                        1.753k ± 0%   1.754k ± 0%  +0.06% (p=0.013 n=6)
ValidateObservation                            1.357k ± 0%   1.358k ± 0%  +0.07% (p=0.002 n=6)
ValidatorCreation                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.290 n=6)
ValidateRotating                               5.579k ± 0%   5.626k ± 0%  +0.84% (p=0.002 n=6)
ValidatorLiveHeap                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.152 n=6)
ValidateProfiled/patient                       6.141k ± 0%   6.143k ± 0%  +0.03% (p=0.002 n=6)
ValidateProfiled/blood-pressure                4.334k ± 0%   4.335k ± 0%  +0.02% (p=0.002 n=6)
ValidateProfiled/extension-values              26.14k ± 0%   26.12k ± 0%  -0.07% (p=0.002 n=6)
ValidateParallel                                393.0 ± 0%    394.0 ± 0%  +0.25% (p=0.002 n=6)
ValidateParallel-8                              393.0 ± 0%    394.0 ± 0%  +0.25% (p=0.002 n=6)
ValidateProfiled/patient-parallel              6.141k ± 0%   6.143k ± 0%  +0.03% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            6.141k ± 0%   6.143k ± 0%  +0.03% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       4.334k ± 0%   4.335k ± 0%  +0.02% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     4.334k ± 0%   4.335k ± 0%  +0.02% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel     26.14k ± 0%   26.12k ± 0%  -0.07% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   26.14k ± 0%   26.12k ± 0%  -0.07% (p=0.002 n=6)
geomean                                        8.642k        8.649k       +0.09%

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating     2.077k ± 5%     2.089k ± 2%  ~ (p=0.725 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.98Mi ± 0%   92.98Mi ± 0%  ~ (p=0.515 n=6)
```
