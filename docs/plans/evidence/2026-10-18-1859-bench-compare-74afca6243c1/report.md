# Benchmark comparison

- Date: 2026-10-18 19:16 -03
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
                                           │    base.txt    │               head.txt               │
                                           │     sec/op     │    sec/op     vs base                │
ValidatePatientWithData                       122.6µ ±  23%   121.2µ ± 19%        ~ (p=0.971 n=10)
ValidateObservation                           110.4µ ±  14%   103.2µ ± 15%        ~ (p=0.529 n=10)
ValidatorCreation                             851.8m ±  92%   855.0m ± 24%        ~ (p=0.971 n=10)
ValidateRotating                              511.1µ ±  21%   424.0µ ± 10%  -17.04% (p=0.003 n=10)
ValidatorLiveHeap                             810.3m ±  29%   874.1m ± 13%        ~ (p=0.684 n=10)
ValidateProfiled/patient                      711.1µ ±  17%   441.2µ ± 14%  -37.95% (p=0.001 n=10)
ValidateProfiled/blood-pressure               451.7µ ±  39%   333.3µ ± 25%  -26.22% (p=0.002 n=10)
ValidateParallel                              34.95µ ±  12%   33.81µ ± 10%        ~ (p=0.481 n=10)
ValidateParallel-4                            31.48µ ±  13%   11.94µ ± 17%  -62.07% (p=0.000 n=10)
ValidateParallel-8                            42.41µ ±  15%   10.31µ ± 23%  -75.68% (p=0.000 n=10)
ValidateProfiled/patient-parallel             740.8µ ± 175%   464.8µ ± 19%  -37.25% (p=0.001 n=10)
ValidateProfiled/patient-parallel-4           269.3µ ±  55%   150.7µ ± 27%  -44.04% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8           312.7µ ±  26%   120.8µ ± 34%  -61.37% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel      492.4µ ±  71%   337.3µ ± 36%  -31.50% (p=0.015 n=10)
ValidateProfiled/blood-pressure-parallel-4    207.6µ ±  16%   122.4µ ± 43%  -41.07% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   236.68µ ±   8%   90.49µ ± 53%  -61.77% (p=0.000 n=10)
geomean                                       565.0µ          359.2µ        -36.44%

                                           │   base.txt   │               head.txt               │
                                           │     B/op     │     B/op      vs base                │
ValidatePatientWithData                      83.75Ki ± 0%   83.71Ki ± 0%   -0.06% (p=0.003 n=10)
ValidateObservation                          73.79Ki ± 0%   73.74Ki ± 0%        ~ (p=0.071 n=10)
ValidatorCreation                            261.6Mi ± 0%   266.0Mi ± 0%   +1.70% (p=0.000 n=10)
ValidateRotating                             342.2Ki ± 0%   316.9Ki ± 0%   -7.40% (p=0.000 n=10)
ValidatorLiveHeap                            261.6Mi ± 0%   266.0Mi ± 0%   +1.71% (p=0.000 n=10)
ValidateProfiled/patient                     464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure              316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
ValidateParallel                             24.18Ki ± 0%   24.18Ki ± 0%        ~ (p=0.492 n=10)
ValidateParallel-4                           24.19Ki ± 0%   24.15Ki ± 0%   -0.16% (p=0.000 n=10)
ValidateParallel-8                           24.21Ki ± 0%   24.15Ki ± 0%   -0.27% (p=0.000 n=10)
ValidateProfiled/patient-parallel            464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          464.5Ki ± 0%   308.5Ki ± 0%  -33.58% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   316.5Ki ± 0%   238.3Ki ± 0%  -24.71% (p=0.000 n=10)
geomean                                      421.8Ki        353.6Ki       -16.16%

                                           │  base.txt   │               head.txt                │
                                           │  allocs/op  │  allocs/op   vs base                  │
ValidatePatientWithData                      1.707k ± 0%   1.706k ± 0%   -0.06% (p=0.011 n=10)
ValidateObservation                          1.423k ± 0%   1.423k ± 0%   -0.04% (p=0.033 n=10)
ValidatorCreation                            1.116M ± 0%   1.162M ± 0%   +4.05% (p=0.000 n=10)
ValidateRotating                             5.667k ± 0%   5.360k ± 0%   -5.42% (p=0.000 n=10)
ValidatorLiveHeap                            1.116M ± 0%   1.162M ± 0%   +4.05% (p=0.000 n=10)
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
geomean                                      5.966k        5.412k        -9.29%
¹ all samples are equal

                 │   base.txt    │               head.txt               │
                 │ first-pass-ms │ first-pass-ms  vs base               │
ValidateRotating    2.589k ± 10%    2.338k ± 10%  -9.68% (p=0.007 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │              head.txt               │
                  │    live-B    │    live-B     vs base               │
ValidatorLiveHeap   91.62Mi ± 0%   92.91Mi ± 0%  +1.41% (p=0.000 n=10)
```
