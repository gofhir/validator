# Benchmark comparison

- Date: 2026-10-27 09:45 -03
- Base: `a12780150511` · Head: `a12780150511` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=a127801 COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                           │   base.txt    │               head.txt               │
                                           │    sec/op     │    sec/op      vs base               │
ValidatePatientWithData                      105.8µ ±   3%   105.9µ ±  23%       ~ (p=0.971 n=10)
ValidateObservation                          85.67µ ±  52%   86.58µ ±  51%       ~ (p=0.796 n=10)
ValidatorCreation                            764.8m ± 201%   721.4m ±   5%  -5.68% (p=0.002 n=10)
ValidateRotating                             393.6µ ±   6%   399.9µ ±   4%       ~ (p=0.684 n=10)
ValidatorLiveHeap                            755.2m ±   6%   714.2m ±   1%  -5.43% (p=0.000 n=10)
ValidateProfiled/patient                     369.7µ ±  12%   365.6µ ±   3%       ~ (p=0.436 n=10)
ValidateProfiled/blood-pressure              281.1µ ±  25%   280.7µ ±   3%       ~ (p=0.684 n=10)
ValidateParallel                             29.92µ ±   5%   30.34µ ±  10%       ~ (p=0.529 n=10)
ValidateParallel-4                           9.072µ ±   4%   8.967µ ±  13%       ~ (p=0.529 n=10)
ValidateParallel-8                           5.533µ ±  40%   5.735µ ±  48%       ~ (p=0.684 n=10)
ValidateProfiled/patient-parallel            365.5µ ±  13%   368.9µ ±   5%       ~ (p=0.912 n=10)
ValidateProfiled/patient-parallel-4          108.3µ ±  28%   108.8µ ±   2%       ~ (p=0.853 n=10)
ValidateProfiled/patient-parallel-8          70.61µ ±  42%   68.11µ ±  92%       ~ (p=0.436 n=10)
ValidateProfiled/blood-pressure-parallel     281.3µ ±   8%   279.9µ ±  36%       ~ (p=0.853 n=10)
ValidateProfiled/blood-pressure-parallel-4   83.05µ ±  12%   82.22µ ± 858%       ~ (p=0.912 n=10)
ValidateProfiled/blood-pressure-parallel-8   50.88µ ± 126%   53.41µ ± 290%       ~ (p=0.436 n=10)
geomean                                      275.0µ          274.1µ         -0.31%

                                           │   base.txt   │              head.txt               │
                                           │     B/op     │     B/op      vs base               │
ValidatePatientWithData                      83.67Ki ± 0%   83.46Ki ± 0%  -0.26% (p=0.000 n=10)
ValidateObservation                          70.81Ki ± 0%   70.60Ki ± 0%  -0.29% (p=0.000 n=10)
ValidatorCreation                            270.6Mi ± 0%   270.6Mi ± 0%  -0.02% (p=0.007 n=10)
ValidateRotating                             316.8Ki ± 0%   316.4Ki ± 0%  -0.13% (p=0.000 n=10)
ValidatorLiveHeap                            270.6Mi ± 0%   270.6Mi ± 0%       ~ (p=0.669 n=10)
ValidateProfiled/patient                     310.6Ki ± 0%   309.8Ki ± 0%  -0.25% (p=0.000 n=10)
ValidateProfiled/blood-pressure              230.3Ki ± 0%   230.0Ki ± 0%  -0.09% (p=0.000 n=10)
ValidateParallel                             24.15Ki ± 0%   23.94Ki ± 0%  -0.85% (p=0.000 n=10)
ValidateParallel-4                           24.11Ki ± 0%   23.91Ki ± 0%  -0.85% (p=0.000 n=10)
ValidateParallel-8                           24.11Ki ± 0%   23.91Ki ± 0%  -0.84% (p=0.000 n=10)
ValidateProfiled/patient-parallel            310.6Ki ± 0%   309.8Ki ± 0%  -0.25% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          310.6Ki ± 0%   309.9Ki ± 0%  -0.25% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          310.6Ki ± 0%   309.9Ki ± 0%  -0.25% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     230.2Ki ± 0%   230.0Ki ± 0%  -0.09% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   230.3Ki ± 0%   230.1Ki ± 0%  -0.09% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   230.3Ki ± 0%   230.1Ki ± 0%  -0.09% (p=0.000 n=10)
geomean                                      350.9Ki        349.9Ki       -0.29%

                                           │  base.txt   │              head.txt              │
                                           │  allocs/op  │  allocs/op   vs base               │
ValidatePatientWithData                      1.706k ± 0%   1.705k ± 0%  -0.06% (p=0.005 n=10)
ValidateObservation                          1.320k ± 0%   1.319k ± 0%  -0.08% (p=0.000 n=10)
ValidatorCreation                            1.184M ± 0%   1.184M ± 0%  -0.00% (p=0.019 n=10)
ValidateRotating                             5.354k ± 0%   5.351k ± 0%  -0.06% (p=0.000 n=10)
ValidatorLiveHeap                            1.184M ± 0%   1.184M ± 0%       ~ (p=0.393 n=10)
ValidateProfiled/patient                     6.064k ± 0%   6.043k ± 0%  -0.35% (p=0.000 n=10)
ValidateProfiled/blood-pressure              4.219k ± 0%   4.218k ± 0%  -0.02% (p=0.000 n=10)
ValidateParallel                              384.0 ± 0%    383.0 ± 0%  -0.26% (p=0.000 n=10)
ValidateParallel-4                            384.0 ± 0%    383.0 ± 0%  -0.26% (p=0.000 n=10)
ValidateParallel-8                            384.0 ± 0%    383.0 ± 0%  -0.26% (p=0.000 n=10)
ValidateProfiled/patient-parallel            6.064k ± 0%   6.043k ± 0%  -0.35% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4          6.064k ± 0%   6.043k ± 0%  -0.35% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8          6.064k ± 0%   6.043k ± 0%  -0.35% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel     4.219k ± 0%   4.218k ± 0%  -0.02% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-4   4.219k ± 0%   4.218k ± 0%  -0.02% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel-8   4.219k ± 0%   4.218k ± 0%  -0.02% (p=0.000 n=10)
geomean                                      5.319k        5.310k       -0.15%

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    1.990k ± 12%     2.016k ± 3%  ~ (p=0.971 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.98Mi ± 0%  ~ (p=0.567 n=10)
```
