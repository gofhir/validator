# Benchmark comparison

- Date: 2026-10-30 22:02 -03
- Base: `970e2c1b1e1a` · Head: `970e2c1b1e1a` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 6 · serial benchmarks: 1 CPU · parallel: 1,8 · benchtime: 1s
- Command: `BASE=970e2c1 COUNT=6 CPUS=1,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt    │              head.txt               │
                                             │    sec/op     │    sec/op      vs base              │
ValidatePatientWithData                        109.3µ ±   6%    109.0µ ±  9%       ~ (p=1.000 n=6)
ValidateObservation                            90.80µ ±   5%    89.61µ ±  4%       ~ (p=0.394 n=6)
ValidatorCreation                              746.3m ±   4%    740.8m ±  3%       ~ (p=0.937 n=6)
ValidateRotating                               416.1µ ±   5%    424.0µ ±  7%       ~ (p=0.937 n=6)
ValidatorLiveHeap                              729.3m ±   5%    735.1m ±  4%       ~ (p=0.818 n=6)
ValidateProfiled/patient                       387.1µ ±   7%    407.8µ ± 10%       ~ (p=0.310 n=6)
ValidateProfiled/blood-pressure                299.3µ ±   7%    297.1µ ± 11%       ~ (p=0.937 n=6)
ValidateProfiled/extension-values              1.681m ±   8%    1.709m ±  6%       ~ (p=0.818 n=6)
ValidateParallel                               31.03µ ±   4%    31.51µ ±  6%       ~ (p=0.818 n=6)
ValidateParallel-8                             8.527µ ±   9%    8.722µ ± 28%       ~ (p=0.699 n=6)
ValidateProfiled/patient-parallel              389.7µ ±  67%    395.2µ ± 24%       ~ (p=0.937 n=6)
ValidateProfiled/patient-parallel-8            97.45µ ± 243%   107.07µ ± 44%       ~ (p=0.699 n=6)
ValidateProfiled/blood-pressure-parallel       298.7µ ± 135%    303.9µ ± 10%       ~ (p=0.937 n=6)
ValidateProfiled/blood-pressure-parallel-8     76.15µ ± 178%    80.48µ ± 22%       ~ (p=0.589 n=6)
ValidateProfiled/extension-values-parallel     1.712m ± 109%    1.710m ± 10%       ~ (p=1.000 n=6)
ValidateProfiled/extension-values-parallel-8   429.6µ ±  31%    458.2µ ± 18%       ~ (p=0.699 n=6)
geomean                                        551.7µ           563.7µ        +2.19%

                                             │   base.txt   │               head.txt               │
                                             │     B/op     │     B/op      vs base                │
ValidatePatientWithData                        86.19Ki ± 0%   86.19Ki ± 0%       ~ (p=0.974 n=6)
ValidateObservation                            72.86Ki ± 0%   72.86Ki ± 0%       ~ (p=0.972 n=6)
ValidatorCreation                              270.6Mi ± 0%   270.5Mi ± 0%       ~ (p=0.394 n=6)
ValidateRotating                               313.2Ki ± 0%   313.2Ki ± 0%       ~ (p=1.000 n=6)
ValidatorLiveHeap                              270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.909 n=6)
ValidateProfiled/patient                       315.7Ki ± 0%   315.7Ki ± 0%       ~ (p=1.000 n=6)
ValidateProfiled/blood-pressure                236.0Ki ± 0%   236.0Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              1.367Mi ± 0%   1.367Mi ± 0%       ~ (p=0.636 n=6)
ValidateParallel                               24.58Ki ± 0%   24.59Ki ± 0%       ~ (p=0.768 n=6)
ValidateParallel-8                             24.55Ki ± 0%   24.55Ki ± 0%       ~ (p=0.602 n=6)
ValidateProfiled/patient-parallel              315.7Ki ± 0%   315.7Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel-8            315.7Ki ± 0%   315.7Ki ± 0%       ~ (p=0.994 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ± 0%   236.0Ki ± 0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ± 0%   236.0Ki ± 0%       ~ (p=0.281 n=6)
ValidateProfiled/extension-values-parallel     1.367Mi ± 0%   1.367Mi ± 0%       ~ (p=0.545 n=6)
ValidateProfiled/extension-values-parallel-8   1.367Mi ± 0%   1.367Mi ± 0%       ~ (p=0.416 n=6)
geomean                                        563.5Ki        563.5Ki       -0.00%
¹ all samples are equal

                                             │  base.txt   │              head.txt               │
                                             │  allocs/op  │  allocs/op   vs base                │
ValidatePatientWithData                        1.753k ± 0%   1.753k ± 0%       ~ (p=1.000 n=6)
ValidateObservation                            1.357k ± 0%   1.357k ± 0%       ~ (p=1.000 n=6) ¹
ValidatorCreation                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.515 n=6)
ValidateRotating                               5.579k ± 0%   5.579k ± 0%       ~ (p=1.000 n=6) ¹
ValidatorLiveHeap                              1.184M ± 0%   1.184M ± 0%       ~ (p=0.913 n=6)
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
geomean                                        8.642k        8.642k       -0.00%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating     2.077k ± 4%    2.168k ± 10%  ~ (p=0.240 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.99Mi ± 0%   92.98Mi ± 0%  ~ (p=0.892 n=6)
```
