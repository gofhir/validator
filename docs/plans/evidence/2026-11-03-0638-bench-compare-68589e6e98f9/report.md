# Benchmark comparison

- Date: 2026-11-03 06:55 -03
- Base: `68589e6e98f9` · Head: `68589e6e98f9` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=68589e6 COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt   │              head.txt               │
                                             │    sec/op    │    sec/op     vs base               │
ValidatePatientWithData                        108.9µ ±  7%   107.0µ ±  3%       ~ (p=0.247 n=10)
ValidateObservation                            89.41µ ± 17%   87.66µ ±  5%       ~ (p=0.436 n=10)
ValidatorCreation                              735.9m ± 62%   746.7m ±  3%       ~ (p=0.853 n=10)
ValidateRotating                               415.9µ ±  6%   407.0µ ±  2%       ~ (p=0.190 n=10)
ValidatorLiveHeap                              743.1m ±  3%   723.3m ±  2%       ~ (p=0.165 n=10)
ValidateProfiled/patient                       384.4µ ±  6%   382.8µ ±  2%       ~ (p=0.739 n=10)
ValidateProfiled/blood-pressure                292.9µ ±  4%   293.0µ ±  4%       ~ (p=0.579 n=10)
ValidateProfiled/extension-values              1.654m ±  8%   1.695m ± 15%       ~ (p=0.280 n=10)
ValidateParallel                               30.94µ ±  4%   30.62µ ± 38%       ~ (p=0.684 n=10)
ValidateParallel-4                             9.473µ ± 15%   9.937µ ± 19%       ~ (p=0.393 n=10)
ValidateParallel-8                             7.292µ ± 14%   7.101µ ± 39%       ~ (p=0.912 n=10)
ValidateProfiled/patient-parallel              382.4µ ±  7%   383.2µ ±  6%       ~ (p=0.853 n=10)
ValidateProfiled/patient-parallel-4            117.1µ ± 17%   125.7µ ± 16%       ~ (p=0.481 n=10)
ValidateProfiled/patient-parallel-8            87.07µ ± 18%   98.46µ ± 21%       ~ (p=0.063 n=10)
ValidateProfiled/blood-pressure-parallel       290.5µ ±  4%   294.5µ ± 12%       ~ (p=0.481 n=10)
ValidateProfiled/blood-pressure-parallel-4     85.59µ ±  9%   91.85µ ± 11%       ~ (p=0.105 n=10)
ValidateProfiled/blood-pressure-parallel-8     60.93µ ± 11%   65.07µ ± 36%       ~ (p=0.089 n=10)
ValidateProfiled/extension-values-parallel     1.689m ± 18%   1.699m ± 11%       ~ (p=0.739 n=10)
ValidateProfiled/extension-values-parallel-4   532.6µ ± 15%   537.1µ ± 17%       ~ (p=0.739 n=10)
ValidateProfiled/extension-values-parallel-8   409.2µ ± 23%   364.1µ ± 25%       ~ (p=0.353 n=10)
geomean                                        366.5µ         370.3µ        +1.02%

                                             │   base.txt   │               head.txt                │
                                             │     B/op     │     B/op      vs base                 │
ValidatePatientWithData                        86.21Ki ± 0%   86.21Ki ± 0%       ~ (p=0.635 n=10)
ValidateObservation                            72.81Ki ± 0%   72.81Ki ± 0%       ~ (p=0.698 n=10)
ValidatorCreation                              270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.780 n=10)
ValidateRotating                               311.1Ki ± 0%   311.1Ki ± 0%       ~ (p=0.237 n=10)
ValidatorLiveHeap                              270.6Mi ± 0%   270.5Mi ± 0%       ~ (p=0.117 n=10)
ValidateProfiled/patient                       315.5Ki ± 0%   315.5Ki ± 0%       ~ (p=1.000 n=10)
ValidateProfiled/blood-pressure                235.9Ki ± 0%   235.9Ki ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values              1.365Mi ± 0%   1.365Mi ± 0%       ~ (p=0.139 n=10)
ValidateParallel                               24.60Ki ± 0%   24.60Ki ± 0%       ~ (p=0.752 n=10)
ValidateParallel-4                             24.57Ki ± 0%   24.57Ki ± 0%       ~ (p=0.469 n=10)
ValidateParallel-8                             24.57Ki ± 0%   24.57Ki ± 0%       ~ (p=0.996 n=10)
ValidateProfiled/patient-parallel              315.5Ki ± 0%   315.5Ki ± 0%       ~ (p=1.000 n=10)
ValidateProfiled/patient-parallel-4            315.5Ki ± 0%   315.5Ki ± 0%       ~ (p=0.405 n=10)
ValidateProfiled/patient-parallel-8            315.6Ki ± 0%   315.6Ki ± 0%       ~ (p=0.539 n=10)
ValidateProfiled/blood-pressure-parallel       235.9Ki ± 0%   235.9Ki ± 0%       ~ (p=1.000 n=10)
ValidateProfiled/blood-pressure-parallel-4     235.9Ki ± 0%   235.9Ki ± 0%       ~ (p=0.425 n=10)
ValidateProfiled/blood-pressure-parallel-8     235.9Ki ± 0%   235.9Ki ± 0%       ~ (p=0.718 n=10)
ValidateProfiled/extension-values-parallel     1.365Mi ± 0%   1.365Mi ± 0%       ~ (p=1.000 n=10)
ValidateProfiled/extension-values-parallel-4   1.365Mi ± 0%   1.365Mi ± 0%       ~ (p=0.986 n=10)
ValidateProfiled/extension-values-parallel-8   1.365Mi ± 0%   1.365Mi ± 0%       ~ (p=0.988 n=10)
geomean                                        468.6Ki        468.6Ki       -0.00%
¹ all samples are equal

                                             │  base.txt   │               head.txt               │
                                             │  allocs/op  │  allocs/op   vs base                 │
ValidatePatientWithData                        1.753k ± 0%   1.753k ± 0%       ~ (p=1.000 n=10)
ValidateObservation                            1.353k ± 0%   1.353k ± 0%       ~ (p=1.000 n=10)
ValidatorCreation                              1.184M ± 0%   1.184M ± 0%  -0.00% (p=0.045 n=10)
ValidateRotating                               5.530k ± 0%   5.530k ± 0%       ~ (p=1.000 n=10)
ValidatorLiveHeap                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.116 n=10)
ValidateProfiled/patient                       6.132k ± 0%   6.132k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure                4.328k ± 0%   4.328k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values              26.05k ± 0%   26.05k ± 0%       ~ (p=1.000 n=10) ¹
ValidateParallel                                393.0 ± 0%    393.0 ± 0%       ~ (p=1.000 n=10) ¹
ValidateParallel-4                              393.0 ± 0%    393.0 ± 0%       ~ (p=1.000 n=10) ¹
ValidateParallel-8                              393.0 ± 0%    393.0 ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel              6.132k ± 0%   6.132k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-4            6.132k ± 0%   6.132k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-8            6.132k ± 0%   6.132k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel       4.328k ± 0%   4.328k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-4     4.328k ± 0%   4.328k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-8     4.328k ± 0%   4.328k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel     26.05k ± 0%   26.05k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel-4   26.05k ± 0%   26.05k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel-8   26.05k ± 0%   26.05k ± 0%       ~ (p=1.000 n=10) ¹
geomean                                        7.418k        7.418k       -0.00%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating     2.093k ± 3%    2.039k ± 11%  ~ (p=0.075 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.98Mi ± 0%  ~ (p=1.000 n=10)
```
