# Benchmark comparison

- Date: 2026-10-19 01:00 -03
- Base: `a2ac6ea6050e` · Head: `0e7b0c9416ef`
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=a2ac6ea COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                           │   base.txt   │               head.txt                │
                                           │    sec/op    │    sec/op      vs base                │
ValidatePatientWithData                      133.7µ ± 17%    134.5µ ± 14%        ~ (p=0.796 n=10)
ValidateObservation                          98.63µ ± 20%   103.07µ ± 24%        ~ (p=0.631 n=10)
ValidatorCreation                            836.7m ± 24%    942.0m ± 38%        ~ (p=0.123 n=10)
ValidateRotating                             529.6µ ± 38%    549.1µ ± 13%        ~ (p=0.739 n=10)
ValidatorLiveHeap                            902.6m ± 46%    873.2m ± 26%        ~ (p=0.971 n=10)
ValidateProfiled/patient                     786.2µ ± 57%    432.3µ ± 11%  -45.01% (p=0.000 n=10)
ValidateProfiled/blood-pressure              524.9µ ± 19%    310.9µ ± 18%  -40.76% (p=0.000 n=10)
ValidateParallel                             38.30µ ± 31%    37.34µ ± 29%        ~ (p=0.481 n=10)
ValidateParallel-4                           33.14µ ±  9%    15.58µ ± 41%  -52.99% (p=0.000 n=10)
ValidateParallel-8                           41.77µ ± 17%    12.97µ ± 52%  -68.95% (p=0.000 n=10)
ValidateProfiled/patient-parallel            731.7µ ± 31%    556.7µ ± 49%  -23.91% (p=0.015 n=10)
ValidateProfiled/patient-parallel-4          297.4µ ± 19%    208.6µ ± 50%  -29.87% (p=0.011 n=10)
ValidateProfiled/patient-parallel-8          300.4µ ± 19%    172.7µ ± 38%  -42.51% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     470.9µ ± 45%    355.3µ ± 37%  -24.56% (p=0.009 n=10)
ValidateProfiled/blood-pressure-parallel-4   241.3µ ± 17%    162.5µ ± 27%  -32.65% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   245.1µ ± 11%    127.0µ ± 23%  -48.18% (p=0.000 n=10)
geomean                                      589.0µ          419.9µ        -28.72%

                                           │   base.txt    │               head.txt               │
                                           │     B/op      │     B/op      vs base                │
ValidatePatientWithData                      83.73Ki ±  0%   83.69Ki ± 0%   -0.06% (p=0.002 n=10)
ValidateObservation                          70.88Ki ±  0%   70.84Ki ± 0%   -0.06% (p=0.041 n=10)
ValidatorCreation                            266.1Mi ±  0%   270.6Mi ± 0%   +1.68% (p=0.000 n=10)
ValidateRotating                             342.1Ki ± 15%   316.8Ki ± 0%        ~ (p=0.080 n=10)
ValidatorLiveHeap                            266.1Mi ±  0%   270.6Mi ± 0%   +1.69% (p=0.000 n=10)
ValidateProfiled/patient                     466.6Ki ±  0%   310.6Ki ± 0%  -33.43% (p=0.000 n=10)
ValidateProfiled/blood-pressure              308.5Ki ±  0%   230.3Ki ± 0%  -25.36% (p=0.000 n=10)
ValidateParallel                             24.16Ki ±  0%   24.15Ki ± 0%        ~ (p=0.099 n=10)
ValidateParallel-4                           24.16Ki ±  0%   24.12Ki ± 0%   -0.17% (p=0.000 n=10)
ValidateParallel-8                           24.18Ki ±  0%   24.12Ki ± 0%   -0.26% (p=0.000 n=10)
ValidateProfiled/patient-parallel            466.6Ki ±  0%   310.6Ki ± 0%  -33.43% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          466.6Ki ±  0%   310.6Ki ± 0%  -33.43% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          466.6Ki ±  0%   310.6Ki ± 0%  -33.43% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     308.5Ki ±  0%   230.3Ki ± 0%  -25.36% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   308.5Ki ±  0%   230.3Ki ± 0%  -25.36% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   308.5Ki ±  0%   230.3Ki ± 0%  -25.36% (p=0.000 n=10)
geomean                                      419.3Ki         351.0Ki       -16.30%

                                           │   base.txt   │               head.txt                │
                                           │  allocs/op   │  allocs/op   vs base                  │
ValidatePatientWithData                      1.707k ±  0%   1.707k ± 0%        ~ (p=0.474 n=10)
ValidateObservation                          1.321k ±  0%   1.320k ± 0%   -0.08% (p=0.011 n=10)
ValidatorCreation                            1.139M ±  0%   1.184M ± 0%   +3.92% (p=0.000 n=10)
ValidateRotating                             5.661k ± 14%   5.354k ± 0%        ~ (p=0.067 n=10)
ValidatorLiveHeap                            1.139M ±  0%   1.184M ± 0%   +3.92% (p=0.000 n=10)
ValidateProfiled/patient                     7.997k ±  0%   6.064k ± 0%  -24.17% (p=0.000 n=10)
ValidateProfiled/blood-pressure              4.782k ±  0%   4.219k ± 0%  -11.77% (p=0.000 n=10)
ValidateParallel                              384.0 ±  0%    384.0 ± 0%        ~ (p=1.000 n=10) ¹
ValidateParallel-4                            384.0 ±  0%    384.0 ± 0%        ~ (p=1.000 n=10) ¹
ValidateParallel-8                            384.0 ±  0%    384.0 ± 0%        ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel            7.997k ±  0%   6.064k ± 0%  -24.17% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          7.997k ±  0%   6.064k ± 0%  -24.17% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          7.997k ±  0%   6.064k ± 0%  -24.17% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     4.782k ±  0%   4.219k ± 0%  -11.77% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   4.782k ±  0%   4.219k ± 0%  -11.77% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   4.782k ±  0%   4.219k ± 0%  -11.77% (p=0.000 n=10)
geomean                                      5.873k         5.319k        -9.45%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    2.789k ± 44%    2.854k ± 30%  ~ (p=0.755 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │              head.txt               │
                  │    live-B    │    live-B     vs base               │
ValidatorLiveHeap   91.71Mi ± 0%   92.99Mi ± 0%  +1.40% (p=0.000 n=10)
```
