# Benchmark comparison

- Date: 2026-10-28 14:05 -03
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
                                             │   base.txt    │               head.txt                │
                                             │    sec/op     │     sec/op      vs base               │
ValidatePatientWithData                        138.6µ ±  74%    131.6µ ±  65%       ~ (p=0.481 n=10)
ValidateObservation                            100.2µ ±  55%    108.9µ ±  85%       ~ (p=0.912 n=10)
ValidatorCreation                               1.033 ±  85%     1.118 ±  43%       ~ (p=0.684 n=10)
ValidateRotating                               504.6µ ±  20%    531.1µ ±  26%       ~ (p=0.481 n=10)
ValidatorLiveHeap                               1.154 ±  34%     1.023 ±  24%       ~ (p=0.631 n=10)
ValidateProfiled/patient                       554.9µ ±  28%    561.6µ ± 427%       ~ (p=0.912 n=10)
ValidateProfiled/blood-pressure                381.2µ ±  84%    404.3µ ± 566%       ~ (p=0.739 n=10)
ValidateProfiled/extension-values              2.713m ±  71%    2.457m ± 254%       ~ (p=0.436 n=10)
ValidateParallel                               42.13µ ±  86%    46.74µ ± 343%       ~ (p=0.393 n=10)
ValidateParallel-4                             21.05µ ± 134%    18.89µ ± 256%       ~ (p=0.912 n=10)
ValidateParallel-8                             14.66µ ±  64%    15.25µ ± 233%       ~ (p=1.000 n=10)
ValidateProfiled/patient-parallel              456.2µ ±  56%    712.8µ ±  77%       ~ (p=0.190 n=10)
ValidateProfiled/patient-parallel-4            215.2µ ± 593%    294.6µ ±  47%       ~ (p=0.631 n=10)
ValidateProfiled/patient-parallel-8            161.4µ ±  48%    178.3µ ±  26%       ~ (p=0.684 n=10)
ValidateProfiled/blood-pressure-parallel       371.4µ ± 107%    370.7µ ±  40%       ~ (p=0.971 n=10)
ValidateProfiled/blood-pressure-parallel-4     165.2µ ±  35%    182.2µ ±  62%       ~ (p=0.684 n=10)
ValidateProfiled/blood-pressure-parallel-8     115.0µ ± 237%    133.0µ ±  64%       ~ (p=0.684 n=10)
ValidateProfiled/extension-values-parallel     1.942m ± 383%    2.024m ±  89%       ~ (p=0.912 n=10)
ValidateProfiled/extension-values-parallel-4   808.9µ ± 102%   1089.0µ ±  66%       ~ (p=0.315 n=10)
ValidateProfiled/extension-values-parallel-8   691.6µ ±  40%    720.8µ ± 263%       ~ (p=0.971 n=10)
geomean                                        554.6µ           598.6µ         +7.95%

                                             │   base.txt    │               head.txt               │
                                             │     B/op      │     B/op       vs base               │
ValidatePatientWithData                        83.52Ki ±  0%   83.46Ki ±  0%       ~ (p=0.362 n=10)
ValidateObservation                            70.63Ki ±  0%   70.66Ki ±  0%       ~ (p=0.578 n=10)
ValidatorCreation                              270.6Mi ±  0%   270.6Mi ±  0%       ~ (p=0.699 n=10)
ValidateRotating                               316.8Ki ± 14%   316.8Ki ± 17%       ~ (p=0.476 n=10)
ValidatorLiveHeap                              270.6Mi ±  0%   270.6Mi ±  0%       ~ (p=0.085 n=10)
ValidateProfiled/patient                       309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.736 n=10)
ValidateProfiled/blood-pressure                230.0Ki ±  0%   230.0Ki ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/extension-values              1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.809 n=10)
ValidateParallel                               23.96Ki ±  0%   23.96Ki ±  1%       ~ (p=0.752 n=10)
ValidateParallel-4                             23.92Ki ±  0%   23.93Ki ±  0%       ~ (p=0.209 n=10)
ValidateParallel-8                             23.92Ki ±  0%   23.92Ki ±  0%       ~ (p=0.794 n=10)
ValidateProfiled/patient-parallel              309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/patient-parallel-4            309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.926 n=10)
ValidateProfiled/patient-parallel-8            309.2Ki ±  0%   309.2Ki ±  0%       ~ (p=0.157 n=10)
ValidateProfiled/blood-pressure-parallel       230.0Ki ±  0%   230.0Ki ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/blood-pressure-parallel-4     230.1Ki ±  0%   230.1Ki ±  0%       ~ (p=0.776 n=10)
ValidateProfiled/blood-pressure-parallel-8     230.1Ki ±  0%   230.1Ki ±  0%       ~ (p=0.616 n=10)
ValidateProfiled/extension-values-parallel     1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.872 n=10)
ValidateProfiled/extension-values-parallel-4   1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.403 n=10)
ValidateProfiled/extension-values-parallel-8   1.349Mi ±  0%   1.349Mi ±  0%       ~ (p=0.929 n=10)
geomean                                        460.4Ki         460.4Ki        +0.00%

                                             │   base.txt   │               head.txt                │
                                             │  allocs/op   │  allocs/op    vs base                 │
ValidatePatientWithData                        1.706k ±  0%   1.706k ±  0%       ~ (p=0.650 n=10)
ValidateObservation                            1.319k ±  0%   1.319k ±  0%       ~ (p=0.741 n=10)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.668 n=10)
ValidateRotating                               5.354k ± 14%   5.354k ± 16%       ~ (p=0.444 n=10)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.087 n=10)
ValidateProfiled/patient                       6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure                4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values              25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10)
ValidateParallel                                383.0 ±  0%    383.0 ±  0%       ~ (p=0.474 n=10)
ValidateParallel-4                              383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateParallel-8                              383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel              6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-4            6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel-8            6.010k ±  0%   6.010k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel       4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-4     4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-8     4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel     25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/extension-values-parallel-4   25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel-8   25.53k ±  0%   25.53k ±  0%       ~ (p=1.000 n=10)
geomean                                        7.262k         7.262k        -0.00%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating    2.521k ± 61%    2.604k ± 62%  ~ (p=0.796 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.99Mi ± 0%  ~ (p=0.572 n=10)
```
