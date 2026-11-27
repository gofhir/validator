# Code review r3: definition-layers descent (after the round-2 fixes)

Revisor independiente (Opus), ronda 3 (última). HL7 validator_cli 7.0.4, `-tx n/a -no-http-access`, un Java a la vez.
Arnés: `review8/probe-src/main.go` compilado de esta copia como `review8r3/pb`; `review8r2/pb` (r2) y `review8/pm` (main) para comparar.
Runner: `review8r3/run.sh` (salidas en `review8r3/out/*.r2|*.r3`). Sondas nuevas: `review8r3/probes/k` (orden) y `review8r3/probes/x` (versión cruzada).
`go test ./pkg/... -count=1`: 19/19 ok (89 s, RSS 1,6 GB; `review8r3/t-pkgs.out`). `go vet` de constraint, registry y validator: limpio.

## R2-F1: arreglado
- `probes/f` explícito (R4): r3 da `ref-1 … (defined in …/Reference)`, igual que HL7 (`review8r2/probes/f/hl7/explicit.txt`); r2 daba `…/f-rp`.
- Familia (ref-1, bdl-8, tim-9, vs-1, que-7, que-12, ras-2, con-3) en R4 y R4B: `TestReviewR2ErrataConstraints…` pasa contra las salidas de HL7.
- Sin regresiones: las 14 corridas de las sondas de r1/r2 (w1, w2 R4/R5, con y sin `-profile`, e, f, f-meta, f-mix, g, h, hc) dan lo mismo
  en r2 y r3, excepto `f`, que ahora coincide con HL7.

## Ataques a lo nuevo (lo que resistió)
- **Sello después de la errata.** Ninguna errata cambia una clave ni quita constraints; `bdl-15` (la agregada) lleva `Source` propio.
  Las erratas por `from` (R4B/R5) siguen encontrando el source publicado en el JSON de la base. Sin hallazgo.
- **Orden: capas primero (walkLayers) y propios primero (walk).** Sondas `k` contra HL7:
  - `ka` (un perfil de tipo, el hijo propio con la misma clave y otro texto): `gk text`, igual que HL7.
  - `kb` (slice y cadena SliceOf con la misma clave y otro texto): `slice text`, igual que HL7 (main daba `base text`: la rama lo arregla).
  - `kc` (perfil de tipo en el slice): `gk text`, igual que HL7. `km1` y `km2` (dos perfiles, expresiones distintas): iguales a HL7.
- **Mapa perezoso de perfiles explícitos.** No hay llamada anidada a `Validate`, la clave es privada y una lectura de un mapa nil da false: equivalente.

## CONFIRMED (anteriores a la rama: main da lo mismo, así que no son regresiones)

### R3-P1 (medium, falso negativo): el slice pierde su constraint cuando el perfil de tipo lo declara el elemento que se divide
`probes/k/kd3`: `Patient.identifier` con slicing y tipo `Identifier` con el perfil `tb2` (`gk`, `true`), y `s1.value` con `gk` («slice text», falla).
HL7 (`k/hl7/k3.txt`): `gk: 'slice text'`. gofhir (rama y main): ningún error. Con la misma expresión (`kd`), HL7 da `slice text` y gofhir `gk text`.
Causa (volcado `review8r3/dump`): el generador de snapshots de gofhir copia el perfil de tipo al slice `s1` y despliega sus hijos desde `tb2`,
y la `gk` del diferencial se pierde (en `s1.value` queda `gk[tb2 text|true]`). En el snapshot de HL7, `s1` tiene `Identifier` sin perfil y
la `gk` del slice. El problema es de `pkg/registry`, no del recorrido.

### R3-P2 (low a medium, versión cruzada): un perfil R4 validado con R4B se queda con la `ref-1` defectuosa de R4B
`probes/x`: `rp-pat` (fhirVersion 4.0.1), `-version 4.3.0`, `generalPractitioner: [{display}]`. gofhir (rama y main) da un error de `ref-1`;
HL7 da 0 errores. `correctSnapshot(cmp.Or(sd.errataVersion, …))` aplica la errata de R4 a una mezcla que trae el JSON publicado de R4B.

## SUSPECTED
Ninguno nuevo de severidad media o mayor en el diff.
