# Benchmark comparison

- Date: 2026-10-18 17:57 -03
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
                                           │   base.txt    │               head.txt               │
                                           │    sec/op     │    sec/op     vs base                │
ValidatePatientWithData                       121.0µ ± 82%   112.8µ ±  6%        ~ (p=0.143 n=10)
ValidateObservation                          100.03µ ± 20%   98.54µ ±  8%        ~ (p=0.529 n=10)
ValidatorCreation                             771.6m ± 36%   856.7m ± 14%  +11.02% (p=0.019 n=10)
ValidateRotating                              474.9µ ±  6%   433.6µ ± 10%   -8.69% (p=0.023 n=10)
ValidatorLiveHeap                             787.8m ±  8%   860.9m ±  8%   +9.28% (p=0.004 n=10)
ValidateProfiled/patient                      716.2µ ±  8%   428.6µ ± 13%  -40.16% (p=0.001 n=10)
ValidateProfiled/blood-pressure               425.2µ ±  8%   320.2µ ± 56%  -24.69% (p=0.015 n=10)
ValidateParallel                              32.09µ ± 10%   32.61µ ±  8%        ~ (p=0.796 n=10)
ValidateParallel-4                            29.20µ ±  9%   31.49µ ± 12%        ~ (p=0.190 n=10)
ValidateParallel-8                            42.62µ ±  7%   43.10µ ± 15%        ~ (p=0.684 n=10)
ValidateProfiled/patient-parallel             683.0µ ±  6%   412.5µ ±  8%  -39.61% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4           258.9µ ± 21%   195.8µ ± 10%  -24.36% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8           261.4µ ± 11%   254.6µ ± 11%        ~ (p=0.353 n=10)
ValidateProfiled/blood-pressure-parallel      433.0µ ±  4%   333.7µ ±  6%  -22.95% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4    193.9µ ± 22%   188.0µ ± 10%        ~ (p=0.247 n=10)
ValidateProfiled/blood-pressure-parallel-8    247.8µ ± 12%   249.7µ ± 17%        ~ (p=0.971 n=10)
geomean                                       531.8µ         476.0µ        -10.48%

                                           │   base.txt   │               head.txt               │
                                           │     B/op     │     B/op      vs base                │
ValidatePatientWithData                      83.77Ki ± 0%   83.71Ki ± 0%   -0.07% (p=0.000 n=10)
ValidateObservation                          73.76Ki ± 0%   73.72Ki ± 0%   -0.05% (p=0.003 n=10)
ValidatorCreation                            261.5Mi ± 0%   306.4Mi ± 0%  +17.16% (p=0.000 n=10)
ValidateRotating                             342.2Ki ± 0%   316.9Ki ± 0%   -7.40% (p=0.000 n=10)
ValidatorLiveHeap                            261.6Mi ± 0%   306.4Mi ± 0%  +17.16% (p=0.000 n=10)
ValidateProfiled/patient                     464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure              316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
ValidateParallel                             24.18Ki ± 0%   24.18Ki ± 0%        ~ (p=0.157 n=10)
ValidateParallel-4                           24.19Ki ± 0%   24.18Ki ± 0%        ~ (p=0.493 n=10)
ValidateParallel-8                           24.22Ki ± 0%   24.22Ki ± 0%        ~ (p=0.956 n=10)
ValidateProfiled/patient-parallel            464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
geomean                                      421.8Ki        360.0Ki       -14.65%

                                           │  base.txt   │               head.txt                │
                                           │  allocs/op  │  allocs/op   vs base                  │
ValidatePatientWithData                      1.707k ± 0%   1.706k ± 0%   -0.03% (p=0.033 n=10)
ValidateObservation                          1.423k ± 0%   1.422k ± 0%   -0.07% (p=0.001 n=10)
ValidatorCreation                            1.116M ± 0%   1.525M ± 0%  +36.56% (p=0.000 n=10)
ValidateRotating                             5.667k ± 0%   5.360k ± 0%   -5.42% (p=0.000 n=10)
ValidatorLiveHeap                            1.116M ± 0%   1.525M ± 0%  +36.56% (p=0.000 n=10)
ValidateProfiled/patient                     7.965k ± 0%   6.032k ± 0%  -24.27% (p=0.000 n=10)
ValidateProfiled/blood-pressure              5.068k ± 0%   4.505k ± 0%  -11.11% (p=0.000 n=10)
ValidateParallel                              384.0 ± 0%    384.0 ± 0%        ~ (p=1.000 n=10) ¹
ValidateParallel-4                            384.0 ± 0%    384.0 ± 0%        ~ (p=1.000 n=10) ¹
ValidateParallel-8                            384.0 ± 0%    384.0 ± 0%        ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel            7.965k ± 0%   6.032k ± 0%  -24.27% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          7.965k ± 0%   6.032k ± 0%  -24.27% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          7.965k ± 0%   6.032k ± 0%  -24.27% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     5.068k ± 0%   4.505k ± 0%  -11.11% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   5.068k ± 0%   4.505k ± 0%  -11.11% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   5.068k ± 0%   4.505k ± 0%  -11.11% (p=0.000 n=10)
geomean                                      5.967k        5.599k        -6.16%
¹ all samples are equal

                 │   base.txt    │               head.txt                │
                 │ first-pass-ms │ first-pass-ms  vs base                │
ValidateRotating    2.621k ± 18%    2.284k ± 10%  -12.84% (p=0.003 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │              head.txt               │
                  │    live-B    │    live-B     vs base               │
ValidatorLiveHeap   91.62Mi ± 0%   92.97Mi ± 0%  +1.48% (p=0.000 n=10)
```
