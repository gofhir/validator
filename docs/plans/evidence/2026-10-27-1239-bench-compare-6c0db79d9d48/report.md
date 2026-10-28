# Benchmark comparison

- Date: 2026-10-27 13:02 -03
- Base: `6c0db79d9d48` · Head: `6c0db79d9d48` (uncommitted changes)
- `go version`: go version go1.27.2 darwin/arm64
- Machine: Apple M4 Pro, 8P+4E
- Interleaved rounds: 10 · serial benchmarks: 1 CPU · parallel: 1,4,8 · benchtime: 1s
- Command: `BASE=6c0db79 COUNT=10 CPUS=1,4,8 BENCHTIME=1s scripts/bench-compare.sh`

```
goos: darwin
goarch: arm64
pkg: github.com/gofhir/validator/v2/pkg/validator
cpu: Apple M4 Pro
                                             │   base.txt    │               head.txt                │
                                             │    sec/op     │     sec/op      vs base               │
ValidatePatientWithData                        138.9µ ± 111%    160.2µ ±  88%       ~ (p=0.529 n=10)
ValidateObservation                            126.0µ ± 169%    126.4µ ±  56%       ~ (p=0.912 n=10)
ValidatorCreation                              890.4m ± 176%   1083.1m ±  51%       ~ (p=0.315 n=10)
ValidateRotating                               535.7µ ± 156%    543.8µ ±  88%       ~ (p=0.971 n=10)
ValidatorLiveHeap                               1.019 ± 122%     1.065 ±  60%       ~ (p=0.739 n=10)
ValidateProfiled/patient                       535.2µ ± 159%    591.8µ ± 133%       ~ (p=0.912 n=10)
ValidateProfiled/blood-pressure                382.8µ ±  40%    581.4µ ± 114%       ~ (p=0.143 n=10)
ValidateProfiled/extension-values              2.456m ±  52%    3.100m ±  32%       ~ (p=0.481 n=10)
ValidateParallel                               38.31µ ±  57%    57.35µ ±  97%       ~ (p=0.075 n=10)
ValidateParallel-4                             16.98µ ± 154%    25.23µ ±  66%       ~ (p=0.315 n=10)
ValidateParallel-8                             17.63µ ±  58%    17.65µ ±  43%       ~ (p=0.912 n=10)
ValidateProfiled/patient-parallel              638.7µ ±  78%    514.9µ ±  63%       ~ (p=0.393 n=10)
ValidateProfiled/patient-parallel-4            280.0µ ±  41%    271.2µ ±  86%       ~ (p=0.971 n=10)
ValidateProfiled/patient-parallel-8            202.9µ ±  56%    200.2µ ±  72%       ~ (p=0.971 n=10)
ValidateProfiled/blood-pressure-parallel       482.6µ ±  72%    447.8µ ± 103%       ~ (p=0.684 n=10)
ValidateProfiled/blood-pressure-parallel-4     197.0µ ±  65%    199.5µ ±  88%       ~ (p=0.739 n=10)
ValidateProfiled/blood-pressure-parallel-8     152.5µ ±  61%    158.4µ ±  50%       ~ (p=0.912 n=10)
ValidateProfiled/extension-values-parallel     2.483m ±  38%    2.107m ± 196%       ~ (p=0.247 n=10)
ValidateProfiled/extension-values-parallel-4   1.129m ±  64%    1.297m ±  70%       ~ (p=0.315 n=10)
ValidateProfiled/extension-values-parallel-8   894.2µ ±  23%    802.7µ ±  94%       ~ (p=0.912 n=10)
geomean                                        617.4µ           666.6µ         +7.97%

                                             │   base.txt    │               head.txt               │
                                             │     B/op      │     B/op       vs base               │
ValidatePatientWithData                        83.48Ki ±  0%   83.50Ki ±  0%       ~ (p=0.342 n=10)
ValidateObservation                            70.68Ki ±  0%   70.68Ki ±  0%       ~ (p=0.468 n=10)
ValidatorCreation                              270.6Mi ±  0%   270.5Mi ±  0%       ~ (p=0.210 n=10)
ValidateRotating                               316.4Ki ± 17%   316.8Ki ± 14%       ~ (p=0.058 n=10)
ValidatorLiveHeap                              270.7Mi ±  0%   270.5Mi ±  0%       ~ (p=0.314 n=10)
ValidateProfiled/patient                       309.8Ki ±  0%   309.2Ki ±  0%  -0.22% (p=0.000 n=10)
ValidateProfiled/blood-pressure                230.0Ki ±  0%   230.0Ki ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/extension-values              1.342Mi ±  0%   1.349Mi ±  0%  +0.54% (p=0.000 n=10)
ValidateParallel                               23.96Ki ±  0%   23.97Ki ±  1%       ~ (p=0.985 n=10)
ValidateParallel-4                             23.92Ki ±  0%   23.93Ki ±  0%       ~ (p=0.795 n=10)
ValidateParallel-8                             23.92Ki ±  0%   23.93Ki ±  0%       ~ (p=0.240 n=10)
ValidateProfiled/patient-parallel              309.8Ki ±  0%   309.2Ki ±  0%  -0.22% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4            309.9Ki ±  0%   309.2Ki ±  0%  -0.22% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8            309.9Ki ±  0%   309.2Ki ±  0%  -0.22% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel       230.0Ki ±  0%   230.0Ki ±  0%       ~ (p=1.000 n=10)
ValidateProfiled/blood-pressure-parallel-4     230.1Ki ±  0%   230.1Ki ±  0%       ~ (p=0.476 n=10)
ValidateProfiled/blood-pressure-parallel-8     230.1Ki ±  0%   230.1Ki ±  0%       ~ (p=0.197 n=10)
ValidateProfiled/extension-values-parallel     1.342Mi ±  0%   1.349Mi ±  0%  +0.54% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-4   1.342Mi ±  0%   1.349Mi ±  0%  +0.54% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-8   1.342Mi ±  0%   1.349Mi ±  0%  +0.54% (p=0.000 n=10)
geomean                                        460.1Ki         460.4Ki        +0.07%

                                             │   base.txt   │               head.txt                │
                                             │  allocs/op   │  allocs/op    vs base                 │
ValidatePatientWithData                        1.706k ±  0%   1.706k ±  0%       ~ (p=0.777 n=10)
ValidateObservation                            1.320k ±  0%   1.320k ±  0%       ~ (p=0.628 n=10)
ValidatorCreation                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.614 n=10)
ValidateRotating                               5.351k ± 16%   5.354k ± 14%       ~ (p=0.051 n=10)
ValidatorLiveHeap                              1.184M ±  0%   1.184M ±  0%       ~ (p=0.311 n=10)
ValidateProfiled/patient                       6.043k ±  0%   6.010k ±  0%  -0.55% (p=0.000 n=10)
ValidateProfiled/blood-pressure                4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values              25.52k ±  0%   25.53k ±  0%  +0.05% (p=0.000 n=10)
ValidateParallel                                383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateParallel-4                              383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateParallel-8                              383.0 ±  0%    383.0 ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/patient-parallel              6.043k ±  0%   6.010k ±  0%  -0.55% (p=0.000 n=10)
ValidateProfiled/patient-parallel-4            6.043k ±  0%   6.010k ±  0%  -0.55% (p=0.000 n=10)
ValidateProfiled/patient-parallel-8            6.043k ±  0%   6.010k ±  0%  -0.55% (p=0.000 n=10)
ValidateProfiled/blood-pressure-parallel       4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-4     4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/blood-pressure-parallel-8     4.218k ±  0%   4.218k ±  0%       ~ (p=1.000 n=10) ¹
ValidateProfiled/extension-values-parallel     25.52k ±  0%   25.53k ±  0%  +0.05% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-4   25.52k ±  0%   25.53k ±  0%  +0.05% (p=0.000 n=10)
ValidateProfiled/extension-values-parallel-8   25.52k ±  0%   25.53k ±  0%  +0.05% (p=0.000 n=10)
geomean                                        7.270k         7.262k        -0.10%
¹ all samples are equal

                 │   base.txt    │            head.txt             │
                 │ first-pass-ms │ first-pass-ms  vs base          │
ValidateRotating   2.422k ± 266%    2.897k ± 71%  ~ (p=0.469 n=10)

                 │  base.txt   │            head.txt             │
                 │  resources  │  resources   vs base            │
ValidateRotating   4.778k ± 0%   4.778k ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                  │   base.txt   │            head.txt            │
                  │    live-B    │    live-B     vs base          │
ValidatorLiveHeap   92.98Mi ± 0%   92.98Mi ± 0%  ~ (p=0.756 n=10)
```
