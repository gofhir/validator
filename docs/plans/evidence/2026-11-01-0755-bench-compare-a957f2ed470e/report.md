# Benchmark comparison

- Date: 2026-11-01 08:03 -03
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
                                             │   base.txt    │              head.txt              │
                                             │    sec/op     │    sec/op     vs base              │
ValidatePatientWithData                        109.4µ ±   2%   108.5µ ±  4%       ~ (p=0.937 n=6)
ValidateObservation                            88.94µ ±   7%   90.58µ ± 26%       ~ (p=0.394 n=6)
ValidatorCreation                              743.2m ±   3%   733.5m ±  8%       ~ (p=0.589 n=6)
ValidateRotating                               419.4µ ±   9%   417.4µ ±  9%       ~ (p=0.937 n=6)
ValidatorLiveHeap                              748.5m ±   8%   730.9m ±  1%       ~ (p=0.310 n=6)
ValidateProfiled/patient                       389.6µ ±   7%   389.0µ ± 11%       ~ (p=0.699 n=6)
ValidateProfiled/blood-pressure                290.8µ ±   7%   291.7µ ±  2%       ~ (p=0.937 n=6)
ValidateProfiled/extension-values              1.670m ±  15%   1.684m ±  8%       ~ (p=0.699 n=6)
ValidateParallel                               30.84µ ±  12%   30.52µ ±  4%       ~ (p=0.818 n=6)
ValidateParallel-8                             7.168µ ±  20%   6.991µ ± 17%       ~ (p=0.937 n=6)
ValidateProfiled/patient-parallel              383.0µ ±   7%   394.7µ ±  8%       ~ (p=0.180 n=6)
ValidateProfiled/patient-parallel-8            93.72µ ± 137%   81.69µ ± 44%       ~ (p=0.485 n=6)
ValidateProfiled/blood-pressure-parallel       291.0µ ±  85%   297.5µ ±  3%       ~ (p=0.818 n=6)
ValidateProfiled/blood-pressure-parallel-8     73.19µ ±  71%   64.15µ ± 76%       ~ (p=0.394 n=6)
ValidateProfiled/extension-values-parallel     1.718m ±  64%   1.714m ± 15%       ~ (p=0.818 n=6)
ValidateProfiled/extension-values-parallel-8   363.5µ ±  36%   395.0µ ± 31%       ~ (p=0.394 n=6)
geomean                                        535.2µ          528.8µ        -1.20%

                                             │   base.txt   │              head.txt               │
                                             │     B/op     │     B/op       vs base              │
ValidatePatientWithData                        86.20Ki ± 0%   86.21Ki ±  0%       ~ (p=0.331 n=6)
ValidateObservation                            72.86Ki ± 0%   72.88Ki ±  0%  +0.02% (p=0.002 n=6)
ValidatorCreation                              270.6Mi ± 0%   270.6Mi ±  0%       ~ (p=0.909 n=6)
ValidateRotating                               313.2Ki ± 0%   311.1Ki ± 14%  -0.68% (p=0.002 n=6)
ValidatorLiveHeap                              270.6Mi ± 0%   270.6Mi ±  0%       ~ (p=0.972 n=6)
ValidateProfiled/patient                       315.7Ki ± 0%   315.7Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/blood-pressure                236.0Ki ± 0%   236.0Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/extension-values              1.367Mi ± 0%   1.366Mi ±  0%  -0.07% (p=0.002 n=6)
ValidateParallel                               24.58Ki ± 0%   24.60Ki ±  0%  +0.07% (p=0.002 n=6)
ValidateParallel-8                             24.55Ki ± 0%   24.57Ki ±  0%  +0.07% (p=0.002 n=6)
ValidateProfiled/patient-parallel              315.7Ki ± 0%   315.7Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            315.7Ki ± 0%   315.7Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ± 0%   236.0Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ± 0%   236.0Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel     1.367Mi ± 0%   1.366Mi ±  0%  -0.07% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   1.367Mi ± 0%   1.366Mi ±  0%  -0.07% (p=0.002 n=6)
geomean                                        563.5Ki        563.3Ki        -0.04%

                                             │  base.txt   │               head.txt               │
                                             │  allocs/op  │  allocs/op    vs base                │
ValidatePatientWithData                        1.754k ± 0%   1.753k ±  0%       ~ (p=0.545 n=6)
ValidateObservation                            1.357k ± 0%   1.357k ±  0%       ~ (p=1.000 n=6) ¹
ValidatorCreation                              1.184M ± 0%   1.184M ±  0%       ~ (p=0.576 n=6)
ValidateRotating                               5.579k ± 0%   5.533k ± 14%  -0.82% (p=0.002 n=6)
ValidatorLiveHeap                              1.184M ± 0%   1.184M ±  0%       ~ (p=0.827 n=6)
ValidateProfiled/patient                       6.141k ± 0%   6.142k ±  0%  +0.02% (p=0.002 n=6)
ValidateProfiled/blood-pressure                4.334k ± 0%   4.334k ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              26.14k ± 0%   26.12k ±  0%  -0.07% (p=0.002 n=6)
ValidateParallel                                393.0 ± 0%    393.0 ±  0%       ~ (p=1.000 n=6) ¹
ValidateParallel-8                              393.0 ± 0%    393.0 ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel              6.141k ± 0%   6.142k ±  0%  +0.02% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            6.141k ± 0%   6.142k ±  0%  +0.02% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       4.334k ± 0%   4.334k ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     4.334k ± 0%   4.334k ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel     26.14k ± 0%   26.12k ±  0%  -0.07% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   26.14k ± 0%   26.12k ±  0%  -0.07% (p=0.002 n=6)
geomean                                        8.642k        8.636k        -0.06%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating     2.108k ± 8%    2.095k ± 10%  ~ (p=0.818 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.99Mi ± 0%   93.00Mi ± 0%  ~ (p=0.859 n=6)
```
