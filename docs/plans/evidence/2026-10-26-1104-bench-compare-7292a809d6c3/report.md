# Benchmark comparison

- Date: 2026-10-26 11:22 -03
- Base: `7292a809d6c3` · Head: `7292a809d6c3` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=7292a80 COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                           │   base.txt    │               head.txt                │
                                           │    sec/op     │     sec/op      vs base               │
ValidatePatientWithData                      119.9µ ±  41%    119.6µ ±  58%       ~ (p=0.912 n=10)
ValidateObservation                          103.1µ ±  36%    101.8µ ±  29%       ~ (p=0.796 n=10)
ValidatorCreation                            977.2m ±  32%   1002.6m ± 148%       ~ (p=0.971 n=10)
ValidateRotating                             455.2µ ±  48%    491.4µ ±  85%       ~ (p=0.579 n=10)
ValidatorLiveHeap                            868.4m ±  29%    842.2m ±  84%       ~ (p=0.853 n=10)
ValidateProfiled/patient                     420.2µ ±  41%    424.8µ ±  50%       ~ (p=0.912 n=10)
ValidateProfiled/blood-pressure              315.8µ ±  69%    313.5µ ±  27%       ~ (p=0.912 n=10)
ValidateParallel                             33.39µ ±  38%    36.17µ ± 107%       ~ (p=0.631 n=10)
ValidateParallel-4                           14.75µ ±  79%    15.11µ ± 178%       ~ (p=0.853 n=10)
ValidateParallel-8                           12.66µ ±  56%    12.81µ ± 185%       ~ (p=0.739 n=10)
ValidateProfiled/patient-parallel            436.9µ ±  21%    453.7µ ±  55%       ~ (p=0.529 n=10)
ValidateProfiled/patient-parallel-4          158.2µ ±  53%    181.6µ ±  58%       ~ (p=0.315 n=10)
ValidateProfiled/patient-parallel-8          125.4µ ± 108%    204.2µ ±  53%       ~ (p=0.218 n=10)
ValidateProfiled/blood-pressure-parallel     340.8µ ±  15%    354.7µ ±  69%       ~ (p=0.684 n=10)
ValidateProfiled/blood-pressure-parallel-4   124.3µ ± 121%    145.7µ ±  55%       ~ (p=0.912 n=10)
ValidateProfiled/blood-pressure-parallel-8   102.7µ ± 112%    137.6µ ±  54%       ~ (p=0.436 n=10)
geomean                                      374.4µ           406.9µ         +8.68%

                                           │   base.txt   │              head.txt               │
                                           │     B/op     │     B/op      vs base               │
ValidatePatientWithData                      83.70Ki ± 0%   83.70Ki ± 0%       ~ (p=0.726 n=10)
ValidateObservation                          70.83Ki ± 0%   70.87Ki ± 0%       ~ (p=0.755 n=10)
ValidatorCreation                            270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.617 n=10)
ValidateRotating                             316.8Ki ± 0%   316.8Ki ± 0%       ~ (p=1.000 n=10)
ValidatorLiveHeap                            270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.564 n=10)
ValidateProfiled/patient                     310.6Ki ± 0%   310.6Ki ± 0%       ~ (p=0.465 n=10)
ValidateProfiled/blood-pressure              230.3Ki ± 0%   230.3Ki ± 0%       ~ (p=0.303 n=10)
ValidateParallel                             24.15Ki ± 0%   24.15Ki ± 0%       ~ (p=0.752 n=10)
ValidateParallel-4                           24.12Ki ± 0%   24.12Ki ± 0%       ~ (p=0.382 n=10)
ValidateParallel-8                           24.12Ki ± 0%   24.12Ki ± 0%       ~ (p=0.868 n=10)
ValidateProfiled/patient-parallel            310.6Ki ± 0%   310.6Ki ± 0%       ~ (p=0.843 n=10)
ValidateProfiled/patient-parallel-4          310.6Ki ± 0%   310.6Ki ± 0%       ~ (p=0.088 n=10)
ValidateProfiled/patient-parallel-8          310.6Ki ± 0%   310.6Ki ± 0%       ~ (p=0.748 n=10)
ValidateProfiled/blood-pressure-parallel     230.3Ki ± 0%   230.3Ki ± 0%       ~ (p=0.350 n=10)
ValidateProfiled/blood-pressure-parallel-4   230.3Ki ± 0%   230.3Ki ± 0%       ~ (p=0.250 n=10)
ValidateProfiled/blood-pressure-parallel-8   230.3Ki ± 0%   230.3Ki ± 0%       ~ (p=0.285 n=10)
geomean                                      351.0Ki        351.0Ki       +0.01%

                                           │  base.txt   │               head.txt               │
                                           │  allocs/op  │  allocs/op   vs base                 │
ValidatePatientWithData                      1.707k ± 0%   1.707k ± 0%       ~ (p=1.000 n=10)
ValidateObservation                          1.320k ± 0%   1.321k ± 0%       ~ (p=0.370 n=10)
ValidatorCreation                            1.184M ± 0%   1.184M ± 0%       ~ (p=0.838 n=10)
ValidateRotating                             5.354k ± 0%   5.354k ± 0%       ~ (p=0.526 n=10)
ValidatorLiveHeap                            1.184M ± 0%   1.184M ± 0%       ~ (p=0.857 n=10)
ValidateProfiled/patient                     6.064k ± 0%   6.064k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure              4.219k ± 0%   4.219k ± 0%       ~ (p=1.000 n=10) ¹
ValidateParallel                              384.0 ± 0%    384.0 ± 0%       ~ (p=1.000 n=10)
ValidateParallel-4                            384.0 ± 0%    384.0 ± 0%       ~ (p=1.000 n=10) ¹
ValidateParallel-8                            384.0 ± 0%    384.0 ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel            6.064k ± 0%   6.064k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-4          6.064k ± 0%   6.064k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-8          6.064k ± 0%   6.064k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel     4.219k ± 0%   4.219k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-4   4.219k ± 0%   4.219k ± 0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-8   4.219k ± 0%   4.219k ± 0%       ~ (p=1.000 n=10) ¹
geomean                                      5.319k        5.319k       +0.00%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    2.513k ± 18%    2.972k ± 33%  ~ (p=0.529 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.98Mi ± 0%  ~ (p=0.751 n=10)
```
