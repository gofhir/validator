# Benchmark comparison

- Date: 2026-10-18 18:22 -03
- Base: `74afca6243c1` · Head: `74afca6243c1` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=74afca6 COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                           │   base.txt   │               head.txt                │
                                           │    sec/op    │    sec/op      vs base                │
ValidatePatientWithData                      134.9µ ± 35%    142.7µ ± 19%        ~ (p=0.971 n=10)
ValidateObservation                          109.4µ ± 11%    120.5µ ± 17%        ~ (p=0.315 n=10)
ValidatorCreation                            891.0m ± 28%    970.1m ± 15%        ~ (p=0.529 n=10)
ValidateRotating                             532.8µ ± 28%    531.3µ ± 41%        ~ (p=0.912 n=10)
ValidatorLiveHeap                            856.8m ± 27%   1009.9m ± 30%        ~ (p=0.247 n=10)
ValidateProfiled/patient                     842.6µ ± 12%    511.2µ ± 60%  -39.33% (p=0.002 n=10)
ValidateProfiled/blood-pressure              536.3µ ± 31%    350.7µ ± 96%  -34.61% (p=0.009 n=10)
ValidateParallel                             41.61µ ± 22%    39.13µ ± 36%        ~ (p=0.684 n=10)
ValidateParallel-4                           34.49µ ± 18%    31.93µ ± 26%        ~ (p=0.631 n=10)
ValidateParallel-8                           35.39µ ± 18%    35.15µ ± 21%        ~ (p=0.912 n=10)
ValidateProfiled/patient-parallel            747.6µ ± 39%    468.4µ ± 62%  -37.34% (p=0.007 n=10)
ValidateProfiled/patient-parallel-4          300.8µ ± 89%    230.2µ ± 37%  -23.45% (p=0.019 n=10)
ValidateProfiled/patient-parallel-8          287.5µ ± 73%    251.3µ ± 10%  -12.60% (p=0.043 n=10)
ValidateProfiled/blood-pressure-parallel     482.8µ ± 49%    385.3µ ± 22%  -20.20% (p=0.001 n=10)
ValidateProfiled/blood-pressure-parallel-4   257.0µ ± 26%    226.0µ ± 29%        ~ (p=0.247 n=10)
ValidateProfiled/blood-pressure-parallel-8   248.2µ ± 14%    241.3µ ± 17%        ~ (p=0.393 n=10)
geomean                                      598.9µ          531.1µ        -11.32%

                                           │   base.txt    │               head.txt                │
                                           │     B/op      │     B/op       vs base                │
ValidatePatientWithData                      83.78Ki ±  0%   83.72Ki ±  0%   -0.07% (p=0.004 n=10)
ValidateObservation                          73.80Ki ±  0%   73.76Ki ±  0%   -0.05% (p=0.027 n=10)
ValidatorCreation                            261.5Mi ±  0%   266.0Mi ±  0%   +1.71% (p=0.000 n=10)
ValidateRotating                             342.2Ki ± 14%   316.9Ki ± 14%   -7.40% (p=0.046 n=10)
ValidatorLiveHeap                            261.6Mi ±  0%   266.0Mi ±  0%   +1.71% (p=0.000 n=10)
ValidateProfiled/patient                     464.5Ki ±  0%   308.5Ki ±  0%  -33.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure              316.5Ki ±  0%   238.3Ki ±  0%  -24.71% (p=0.000 n=10)
ValidateParallel                             24.19Ki ±  0%   24.19Ki ±  0%        ~ (p=0.565 n=10)
ValidateParallel-4                           24.19Ki ±  0%   24.19Ki ±  0%   -0.03% (p=0.019 n=10)
ValidateParallel-8                           24.20Ki ±  0%   24.20Ki ±  0%        ~ (p=1.000 n=10)
ValidateProfiled/patient-parallel            464.5Ki ±  0%   308.5Ki ±  0%  -33.58% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          464.5Ki ±  0%   308.5Ki ±  0%  -33.58% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          464.5Ki ±  0%   308.5Ki ±  0%  -33.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     316.5Ki ±  0%   238.3Ki ±  0%  -24.71% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   316.5Ki ±  0%   238.3Ki ±  0%  -24.71% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   316.5Ki ±  0%   238.3Ki ±  0%  -24.71% (p=0.000 n=10)
geomean                                      421.8Ki         353.7Ki        -16.14%

                                           │   base.txt   │                head.txt                │
                                           │  allocs/op   │  allocs/op    vs base                  │
ValidatePatientWithData                      1.707k ±  0%   1.707k ±  0%        ~ (p=0.124 n=10)
ValidateObservation                          1.423k ±  0%   1.423k ±  0%        ~ (p=0.474 n=10)
ValidatorCreation                            1.116M ±  0%   1.162M ±  0%   +4.05% (p=0.000 n=10)
ValidateRotating                             5.667k ± 14%   5.360k ± 14%   -5.42% (p=0.037 n=10)
ValidatorLiveHeap                            1.116M ±  0%   1.162M ±  0%   +4.05% (p=0.000 n=10)
ValidateProfiled/patient                     7.965k ±  0%   6.032k ±  0%  -24.27% (p=0.000 n=10)
ValidateProfiled/blood-pressure              5.068k ±  0%   4.505k ±  0%  -11.11% (p=0.000 n=10)
ValidateParallel                              384.0 ±  0%    384.0 ±  0%        ~ (p=1.000 n=10) ¹
ValidateParallel-4                            384.0 ±  0%    384.0 ±  0%        ~ (p=1.000 n=10) ¹
ValidateParallel-8                            384.0 ±  0%    384.0 ±  0%        ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel            7.965k ±  0%   6.032k ±  0%  -24.27% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          7.965k ±  0%   6.032k ±  0%  -24.27% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          7.965k ±  0%   6.032k ±  0%  -24.27% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     5.068k ±  0%   4.505k ±  0%  -11.11% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   5.068k ±  0%   4.505k ±  0%  -11.11% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   5.068k ±  0%   4.505k ±  0%  -11.11% (p=0.000 n=10)
geomean                                      5.966k         5.412k         -9.29%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    2.910k ± 13%    2.497k ± 43%  ~ (p=0.105 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │              head.txt               │
                  │    live-B    │    live-B     vs base               │
ValidatorLiveHeap   91.62Mi ± 0%   92.89Mi ± 0%  +1.39% (p=0.000 n=10)
```
