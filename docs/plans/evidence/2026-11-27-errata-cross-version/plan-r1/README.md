# Sondas de revisión adversarial

No se modificó la implementación. La sonda reproduce el paso 3 del plan mediante los helpers existentes de
merge, corrección y sellado. No constituye una ejecución de una implementación futura ni una corrida nueva de HL7.
La comparación de addConstraints usa los snapshots de los tres packages núcleo embebidos.

Reproducción sin red, desde la raíz del repositorio:

```sh
cp docs/plans/evidence/2026-11-27-errata-cross-version/codex-adversarial/probe_test.go.txt pkg/registry/adversarial_plan_probe_test.go
mkdir -p .cache/go-review-tmp
GOCACHE="$PWD/.cache/go-review" GOTMPDIR="$PWD/.cache/go-review-tmp" GOPROXY=off GOSUMDB=off go test ./pkg/registry/... -run '^TestAdversarialPlanProbe$' -v -count=1
rm pkg/registry/adversarial_plan_probe_test.go
```

`output.txt` conserva la salida. El PASS indica que las sondas terminaron; los valores registrados exponen
los problemas del algoritmo propuesto. No son tests de aceptación.
