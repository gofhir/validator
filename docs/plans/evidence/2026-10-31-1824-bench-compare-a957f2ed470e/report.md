# Benchmark comparison

- Date: 2026-10-31 18:40 -03
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
                                             │    base.txt    │               head.txt               │
                                             │     sec/op     │    sec/op      vs base               │
ValidatePatientWithData                         223.9µ ± 176%   220.7µ ± 243%        ~ (p=0.699 n=6)
ValidateObservation                             242.2µ ±  58%   203.0µ ± 507%        ~ (p=1.000 n=6)
ValidatorCreation                                1.533 ± 101%    1.192 ± 210%        ~ (p=0.818 n=6)
ValidateRotating                                1.129m ±  73%   1.158m ±  56%        ~ (p=0.818 n=6)
ValidatorLiveHeap                                1.424 ± 142%    2.147 ±  61%        ~ (p=0.485 n=6)
ValidateProfiled/patient                       1155.9µ ±  55%   926.1µ ±  44%        ~ (p=0.310 n=6)
ValidateProfiled/blood-pressure                1048.0µ ±  53%   602.6µ ±  54%  -42.50% (p=0.041 n=6)
ValidateProfiled/extension-values               5.045m ±  90%   2.682m ±  80%        ~ (p=0.093 n=6)
ValidateParallel                               124.44µ ±  74%   67.71µ ±  59%        ~ (p=0.310 n=6)
ValidateParallel-8                              20.99µ ± 106%   21.74µ ±  29%        ~ (p=1.000 n=6)
ValidateProfiled/patient-parallel               1.937m ±  64%   1.107m ± 140%        ~ (p=0.485 n=6)
ValidateProfiled/patient-parallel-8             254.8µ ± 103%   307.6µ ± 148%        ~ (p=0.937 n=6)
ValidateProfiled/blood-pressure-parallel        1.026m ±  76%   1.014m ± 112%        ~ (p=0.818 n=6)
ValidateProfiled/blood-pressure-parallel-8      176.5µ ± 116%   197.5µ ±  88%        ~ (p=0.699 n=6)
ValidateProfiled/extension-values-parallel      5.628m ±  96%   6.280m ±  68%        ~ (p=0.937 n=6)
ValidateProfiled/extension-values-parallel-8    1.376m ± 125%   1.246m ± 105%        ~ (p=1.000 n=6)
geomean                                         1.578m          1.371m         -13.11%

                                             │   base.txt    │              head.txt               │
                                             │     B/op      │     B/op       vs base              │
ValidatePatientWithData                        86.26Ki ±  1%   86.24Ki ±  1%       ~ (p=1.000 n=6)
ValidateObservation                            72.93Ki ±  0%   72.99Ki ±  2%       ~ (p=0.485 n=6)
ValidatorCreation                              270.6Mi ±  0%   270.5Mi ±  0%       ~ (p=0.851 n=6)
ValidateRotating                               266.8Ki ± 50%   286.7Ki ± 37%       ~ (p=0.589 n=6)
ValidatorLiveHeap                              270.6Mi ±  0%   270.7Mi ±  0%       ~ (p=0.485 n=6)
ValidateProfiled/patient                       315.7Ki ±  0%   315.6Ki ±  0%  -0.03% (p=0.002 n=6)
ValidateProfiled/blood-pressure                236.0Ki ±  0%   236.0Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/extension-values              1.367Mi ±  0%   1.366Mi ±  0%  -0.08% (p=0.002 n=6)
ValidateParallel                               24.68Ki ±  0%   24.66Ki ±  0%       ~ (p=0.818 n=6)
ValidateParallel-8                             24.56Ki ±  0%   24.58Ki ±  0%       ~ (p=0.093 n=6)
ValidateProfiled/patient-parallel              315.7Ki ±  0%   315.6Ki ±  0%  -0.02% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            315.7Ki ±  0%   315.6Ki ±  0%  -0.02% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       236.0Ki ±  0%   236.0Ki ±  0%  +0.01% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel-8     236.0Ki ±  0%   236.0Ki ±  0%       ~ (p=0.394 n=6)
ValidateProfiled/extension-values-parallel     1.367Mi ±  0%   1.366Mi ±  0%  -0.08% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   1.367Mi ±  0%   1.366Mi ±  0%  -0.08% (p=0.002 n=6)
geomean                                        558.1Ki         560.5Ki        +0.44%

                                             │   base.txt   │               head.txt               │
                                             │  allocs/op   │  allocs/op    vs base                │
ValidatePatientWithData                        1.754k ±  0%   1.754k ±  0%       ~ (p=0.924 n=6)
ValidateObservation                            1.358k ±  0%   1.358k ±  1%       ~ (p=0.879 n=6)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.784 n=6)
ValidateRotating                               4.773k ± 49%   5.128k ± 37%       ~ (p=0.589 n=6)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.658 n=6)
ValidateProfiled/patient                       6.141k ±  0%   6.139k ±  0%  -0.03% (p=0.002 n=6)
ValidateProfiled/blood-pressure                4.334k ±  0%   4.334k ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values              26.14k ±  0%   26.11k ±  0%  -0.08% (p=0.002 n=6)
ValidateParallel                                393.0 ±  0%    393.0 ±  0%       ~ (p=1.000 n=6)
ValidateParallel-8                              393.0 ±  0%    393.0 ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/patient-parallel              6.141k ±  0%   6.139k ±  0%  -0.03% (p=0.002 n=6)
ValidateProfiled/patient-parallel-8            6.141k ±  0%   6.139k ±  0%  -0.03% (p=0.002 n=6)
ValidateProfiled/blood-pressure-parallel       4.334k ±  0%   4.334k ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/blood-pressure-parallel-8     4.334k ±  0%   4.334k ±  0%       ~ (p=1.000 n=6) ¹
ValidateProfiled/extension-values-parallel     26.14k ±  0%   26.11k ±  0%  -0.08% (p=0.002 n=6)
ValidateProfiled/extension-values-parallel-8   26.14k ±  0%   26.11k ±  0%  -0.08% (p=0.002 n=6)
geomean                                        8.558k         8.595k        +0.43%
¹ all samples are equal

                 │   base.txt    │            head.txt            │
                 │ first-pass-ms │ first-pass-ms  vs base         │
ValidateRotating    8.646k ± 57%    6.092k ± 66%  ~ (p=0.240 n=6)

                 │  base.txt   │            head.txt            │
                 │  resources  │  resources   vs base           │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=6) ¹
¹ all samples are equal

                  │   base.txt   │           head.txt            │
                  │    live-B    │    live-B     vs base         │
ValidatorLiveHeap   92.97Mi ± 0%   92.99Mi ± 0%  ~ (p=0.145 n=6)
```
