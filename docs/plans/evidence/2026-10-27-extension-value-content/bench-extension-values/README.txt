Medición enfocada de BenchmarkValidateProfiled/extension-values (2026-10-27), porque la de
bench-compare traía una varianza de ±50-190 % en el tiempo: la máquina tenía carga de otras sesiones
(promedio de carga 20-30 en 12 núcleos). Base 6c0db79 contra el árbol de trabajo, intercalados, 20
rondas de 2 s con un CPU; base.test se compila en un worktree de 6c0db79 con profiled_bench_test.go y
testdata/bench copiados, como scripts/bench-compare.sh.
  for i in $(seq 1 20); do base.test ... ; head.test ... ; done   (-test.bench '^BenchmarkValidateProfiled$/^extension-values$' -test.benchmem -test.cpu 1 -test.benchtime 2s -test.count 1)
  benchstat base.txt head.txt > report.txt
