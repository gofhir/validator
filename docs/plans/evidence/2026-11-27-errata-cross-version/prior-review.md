# Revisión previa (paso 1, 26-11-2026): 10r, R3-P1 y R3-P2

Hecha mientras la rama `refactor/definition-layers-descent` esperaba su commit (PR #223); las sondas están en `probes/`.
Base del plan `docs/plans/2026-11-27-errata-cross-version.md`.

## gofhir/server

No genera snapshots: delega la validación en `validator/v2` v2.1.0. Nada que tomar (descartar).

## R3-P1: la constraint del slice se pierde (sonda `kd3`)

- `kd3`: `Patient.identifier` declara el perfil de tipo `tb2` (`gk`, `true`); el slice `s1` declara en `s1.value` la
  misma clave `gk` con otra expresión (`$this.length() < 1`).
- Con otra clave (`gk2`, sonda `kd4`), gofhir sí la informa: la pérdida es el choque de claves.
- gofhir: `unrollChildren` no encuentra hijos del elemento dividido y llama a `unroll`, que despliega desde el tipo
  heredado del slice, con el perfil `tb2`. Al fundir el diferencial, la `gk` del slice pierde contra la de `tb2`.
- HL7 (`snap/kd3.json.snap.json`): el slice `s1` queda sin perfil de tipo, sus hijos vienen de `Identifier` base y
  `s1.value` lleva solo la `gk` del slice. `tb2` sigue rigiendo como capa de `Patient.identifier`.
- Mismo choque sin slicing (sonda `kn`: `Patient.identifier` con `tb2` y `Patient.identifier.value` con `gk` y otra
  expresión): HL7 **rechaza el perfil** al generar el snapshot: «The constraint 'gk' at Patient.identifier.value has a
  different expression to the constraint 'gk' it inherits ('$this.length() < 1' vs 'true'). A constraint with the same
  key must be the same constraint». gofhir acepta el perfil y la instancia sin informes.

- Al **validar** `kn`, HL7 sigue con el snapshot roto y da Success, 0 errores: lo mismo que gofhir. El choque solo se ve con
  el comando `snapshot`.

**Conclusión:** R3-P1 solo aparece con un perfil que HL7 considera inválido, y la diferencia al validar está solo en el
caso del slice (`kd3`). El defecto de fondo es que gofhir no
detecta el choque de claves al generar el snapshot (`SNAPSHOT_CONSTRAINT_KEY_CONFLICT` en HL7:
`ProfileUtilities.updateFromDefinition`). Queda por decidir si, en el caso del slice, se sigue la forma en que HL7
despliega sus hijos (desde el tipo base, sin el perfil del elemento dividido).

## 10r

- m10r1 y m10r2 usan la misma clave (`gk`) con la misma expresión y otro texto. El slice declara dos perfiles de tipo
  (`tz-10b`, `tk-10b`; solo el segundo, por su base `tb-10b`, tiene `gk`).
- HL7 informa una vez `gk: 'gk text' (defined in …/tb-10b)`, «validating against p10r1»: desde el perfil, no desde la
  capa. Su snapshot (comando `snapshot`) tiene en `s1.value` la `gk` del slice («slice text»), así que el texto que informa
  al validar no sale de ese snapshot.
- Traza en gofhir (copia con un print en `firstReport`): la `gk` de `tb-10b` solo se evalúa en el chequeo de a qué
  alternativa se ajusta el valor (ruta `Identifier.value`, otro ámbito). En el recorrido real se informa la del slice.
- Variante `probes/m10r3` (el hijo propio con otra clave, `gk3`, que falla; la capa también fallaría): HL7 informa solo
  `gk3`, y gofhir lo mismo, en la rama y en `ba63bba`. O sea, HL7 sí valida los hijos propios del slice, y no informa los
  fallos de `tk-10b` cuando el valor se ajusta a `tz-10b`.

**Conclusión:** 10r aparece solo cuando dos definiciones comparten clave y expresión. HL7 las trata como la misma
constraint («A constraint with the same key must be the same constraint») y la nombra con el texto del perfil de tipo.
Es la misma familia que R3-P1: el choque de claves. No cambia veredictos.

## R3-P2: versión cruzada (sonda `x`)

- `rp-pat` (fhirVersion 4.0.1) validado con `-version 4.3.0` y perfil explícito: gofhir (rama y `ba63bba`) da
  `ref-1`; HL7 da 0 errores. Sin perfil explícito no se da.
- Causa: `pkg/registry/snapshot.go`, `correctSnapshot(cmp.Or(sd.errataVersion, sd.FHIRVersion, baseSD.errataVersion,
  baseSD.FHIRVersion), …)` corrige el snapshot generado con la versión del perfil (R4), pero las constraints heredadas
  vienen de la base, que es R4B. La errata de R4 no encuentra la expresión publicada de R4 y deja la `ref-1` sin corregir.
- Probado en una copia: con la versión de la base primero (`baseSD.errataVersion, baseSD.FHIRVersion, sd.errataVersion,
  sd.FHIRVersion`) la sonda da 0 errores en 4.3.0 y 4.0.1, y pasan `pkg/registry` y `pkg/validator`.
