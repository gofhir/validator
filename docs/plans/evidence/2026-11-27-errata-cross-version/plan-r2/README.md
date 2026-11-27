# Ronda adversarial 2: sondas del diseño v2

No se modificó la implementación ni el plan. Las sondas usan los helpers existentes para modelar corrección al cargar, reescritura del raw de los campos cambiados y mezcla sin corrección posterior. No son una implementación futura ni una corrida de HL7. PASS significa que la sonda terminó; la salida registra contraejemplos.

La reescritura de arrays completos de constraint/type es una interpretación candidata del plan, no un requisito explícito. La pérdida de miembros JSON en esa sonda es por tanto un riesgo SUSPECTED del diseño, no un defecto confirmado de una implementación inexistente. Los contraejemplos de contexto de diferencial y de source no dependen de esa elección: los campos no modelados no participan en ellos.

Reproducción sin red desde la raíz:

```sh
cp docs/plans/evidence/2026-11-27-errata-cross-version/plan-r2/probe_test.go.txt pkg/registry/adversarial_r2_probe_test.go
mkdir -p .cache/go-review-tmp
GOCACHE="$PWD/.cache/go-review" GOTMPDIR="$PWD/.cache/go-review-tmp" GOPROXY=off GOSUMDB=off go test ./pkg/registry -run '^TestAdversarialR2$' -v -count=1
rm pkg/registry/adversarial_r2_probe_test.go
```

`output.txt` conserva la salida. Los controles confirman que bdl-15 sobrevive dos mezclas, que A2 no se reproduce y que el source se recupera en una cadena ordinaria. Los contraejemplos muestran:

- El diferencial Patient.id sin base.path todavía no puede ser identificado como Resource.id al cargar. Sobrescribe el tipo corregido de la base y queda string; la corrección posterior actual lo vuelve id.
- El diferencial Observation.effectiveDateTime con vs-1 no coincide con Observation.effective[x] hasta que la mezcla normaliza la ruta. La corrección posterior actual sí lo corrige.
- Un hijo de un perfil de Timing generado tiene el source de su base en el struct, pero no en raw. Al desplegarlo bajo Patient, cloneElement pierde el source y stampConstraintSources no lo recupera desde la base Patient.
- Una definición sin fhirVersion, cargada con fallback R5 y con snapshot publicado, sigue compartida por el clon después de SetFHIRVersion(R4), con errataVersion R5. Es un límite existente que §3.4 del plan no explicita. El comentario de SetFHIRVersion exige configurarlo antes de cargar; si ese contrato se impone como restricción, el plan debe decirlo y probarlo, en lugar de prometer un fallback distinto por validación.

El enlace plan-r1 del plan no existe en esta copia; la evidencia anterior está en codex-adversarial/ y se leyó allí.
