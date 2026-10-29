# Benchmark comparison

- Date: 2026-10-28 17:00 -03
- Base: `5c0001ce50d9` · Head: `5c0001ce50d9` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=5c0001c COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt   │               head.txt               │
                                             │    sec/op    │    sec/op      vs base               │
ValidatePatientWithData                        152.3µ ± 27%    145.5µ ± 95%       ~ (p=0.631 n=10)
ValidateObservation                            130.1µ ± 36%    121.4µ ± 57%       ~ (p=0.971 n=10)
ValidatorCreation                               1.136 ± 28%     1.026 ± 20%       ~ (p=0.165 n=10)
ValidateRotating                               510.2µ ± 32%    511.5µ ± 19%       ~ (p=1.000 n=10)
ValidatorLiveHeap                               1.025 ± 42%     1.040 ± 36%       ~ (p=0.912 n=10)
ValidateProfiled/patient                       505.4µ ± 27%    467.5µ ± 92%       ~ (p=0.353 n=10)
ValidateProfiled/blood-pressure                386.1µ ± 14%    394.0µ ± 44%       ~ (p=0.796 n=10)
ValidateProfiled/extension-values              2.186m ± 10%    2.334m ± 31%       ~ (p=0.529 n=10)
ValidateParallel                               39.98µ ± 62%    38.20µ ± 70%       ~ (p=0.529 n=10)
ValidateParallel-4                             19.65µ ± 32%    21.08µ ± 31%       ~ (p=0.912 n=10)
ValidateParallel-8                             14.89µ ±  8%    16.50µ ± 41%       ~ (p=0.239 n=10)
ValidateProfiled/patient-parallel              524.0µ ± 13%    551.2µ ± 64%       ~ (p=0.739 n=10)
ValidateProfiled/patient-parallel-4            215.8µ ± 31%    241.2µ ± 34%       ~ (p=0.796 n=10)
ValidateProfiled/patient-parallel-8            181.4µ ± 29%    165.7µ ± 32%       ~ (p=0.436 n=10)
ValidateProfiled/blood-pressure-parallel       473.3µ ± 32%    405.1µ ± 91%       ~ (p=0.436 n=10)
ValidateProfiled/blood-pressure-parallel-4     195.5µ ± 25%    195.8µ ± 45%       ~ (p=0.971 n=10)
ValidateProfiled/blood-pressure-parallel-8     129.5µ ± 43%    137.9µ ± 46%       ~ (p=0.739 n=10)
ValidateProfiled/extension-values-parallel     2.309m ± 26%    2.957m ± 34%       ~ (p=0.481 n=10)
ValidateProfiled/extension-values-parallel-4   937.3µ ± 31%   1017.5µ ± 33%       ~ (p=0.481 n=10)
ValidateProfiled/extension-values-parallel-8   736.4µ ± 12%    787.5µ ± 21%       ~ (p=0.247 n=10)
geomean                                        586.3µ          595.5µ        +1.57%

                                             │   base.txt    │                head.txt                │
                                             │     B/op      │     B/op       vs base                 │
ValidatePatientWithData                        83.49Ki ±  0%   83.50Ki ±  0%       ~ (p=0.578 n=10)
ValidateObservation                            70.67Ki ±  0%   70.67Ki ±  0%       ~ (p=0.734 n=10)
ValidatorCreation                              270.6Mi ±  0%   270.6Mi ±  0%       ~ (p=0.739 n=10)
ValidateRotating                               293.5Ki ± 18%   316.8Ki ± 16%       ~ (p=0.073 n=10)
ValidatorLiveHeap                              270.6Mi ±  0%   270.6Mi ±  0%       ~ (p=0.425 n=10)
ValidateProfiled/patient                       309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.875 n=10)
ValidateProfiled/blood-pressure                230.0Ki ±  0%   230.0Ki ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/extension-values              1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.841 n=10)
ValidateParallel                               23.96Ki ±  0%   23.96Ki ±  0%       ~ (p=0.669 n=10)
ValidateParallel-4                             23.92Ki ±  0%   23.92Ki ±  0%       ~ (p=0.616 n=10)
ValidateParallel-8                             23.92Ki ±  0%   23.92Ki ±  0%       ~ (p=0.696 n=10)
ValidateProfiled/patient-parallel              309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.628 n=10)
ValidateProfiled/patient-parallel-4            309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.561 n=10)
ValidateProfiled/patient-parallel-8            309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.697 n=10)
ValidateProfiled/blood-pressure-parallel       230.0Ki ±  0%   230.0Ki ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-4     230.1Ki ±  0%   230.1Ki ±  0%       ~ (p=0.639 n=10)
ValidateProfiled/blood-pressure-parallel-8     230.1Ki ±  0%   230.1Ki ±  0%       ~ (p=0.668 n=10)
ValidateProfiled/extension-values-parallel     1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.869 n=10)
ValidateProfiled/extension-values-parallel-4   1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.493 n=10)
ValidateProfiled/extension-values-parallel-8   1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.424 n=10)
geomean                                        458.7Ki         460.4Ki        +0.38%
¹ all samples are equal

                                             │   base.txt   │               head.txt                │
                                             │  allocs/op   │  allocs/op    vs base                 │
ValidatePatientWithData                        1.706k ±  0%   1.706k ±  0%       ~ (p=1.000 n=10) ¹
ValidateObservation                            1.320k ±  0%   1.320k ±  0%       ~ (p=1.000 n=10)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.617 n=10)
ValidateRotating                               4.968k ± 17%   5.354k ± 16%       ~ (p=0.358 n=10)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.077 n=10)
ValidateProfiled/patient                       6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure                4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values              25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10) ¹
ValidateParallel                                383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateParallel-4                              383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateParallel-8                              383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel              6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-4            6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-8            6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel       4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-4     4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-8     4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel     25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel-4   25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel-8   25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10) ¹
geomean                                        7.235k         7.262k        +0.37%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    3.348k ± 22%    2.854k ± 61%  ~ (p=0.353 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.98Mi ± 0%  ~ (p=0.787 n=10)
```
