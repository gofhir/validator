# Sondas de la pieza previa de #193: Scope por posición y memos de corridas especulativas

HL7 validator_cli 7.0.4 con `-tx n/a`, y gofhir en `87c0b02` (rama `fix/canonical-scope`, sin cambios en `pkg/`).
Fechas: 06-11-2026 (sc1 a sc14 en R4) y 07-11-2026 (sc8 a sc14 en R4B y R5; D1, D3 y D5). Plan:
[../../2026-11-06-canonical-scope.md](../../2026-11-06-canonical-scope.md).

Estructura: cada grupo (`c1/`, `c2/`, `d1/`, `d3/`, `d5/`) tiene `ig/` (R4), `inst/`, `probe-<grupo>.tgz` (el paquete
de gofhir) y las salidas `hl7-<sondas>-<versión>-7.0.4.json` y `gofhir-87c0b02-<sondas>-<versión>.json`. En `v4.3.0/` y
`v5.0.0/` están los mismos perfiles, con solo `fhirVersion` cambiado, más su `package.json` y su `.tgz`. **Ninguna
instancia cambia entre versiones**: todos los elementos que usan (`Bundle.signature.who`/`onBehalfOf`,
`Patient.link`, `Patient.generalPractitioner`, `Observation.performer`/`hasMember`/`focus`,
`Parameters.parameter.resource`) existen igual en R4, R4B y R5. Los perfiles y las instancias de `d1/`, `d3/` y `d5/`
salen de `scripts/gen_*.py`; `scripts/run.sh` corre HL7 y gofhir en las tres versiones (el binario de gofhir, en
`GOFHIR`), y `scripts/sum.py` resume una salida.

```bash
java -jar validator_cli.jar inst/*.json -version 4.0.1 -ig ig -tx n/a -output hl7-<sondas>-4.0.1-7.0.4.json
java -jar validator_cli.jar inst/*.json -version 4.3.0 -ig v4.3.0/ig -tx n/a -output v4.3.0/hl7-<sondas>-4.3.0-7.0.4.json
java -jar validator_cli.jar inst/*.json -version 5.0.0 -ig v5.0.0/ig -tx n/a -output v5.0.0/hl7-<sondas>-5.0.0-7.0.4.json
gofhir-validator -version <v> -package-file <dir de la versión>/probe-<grupo>.tgz -tx n/a -output json inst/*.json
```

En las tablas, «R4 = R4B = R5» quiere decir que las tres versiones dan exactamente los mismos diagnósticos, y así fue
en todas las sondas de este directorio, tanto en HL7 como en gofhir. No se listan los warnings de buenas prácticas
(`dom-6`, performer, effective) ni los mensajes informativos.

## sc1 a sc7 (raíz del directorio, solo R4)

Las describe el plan, §1. gofhir `87c0b02`, R4:

| Sonda | HL7 | gofhir |
|---|---|---|
| sc1, sc2 | nada | nada |
| sc3, sc4 | error de perfil de destino solo en obs1 | igual |
| sc5 | nada | nada |
| sc6 | error solo en `o.subject` | error en `o.subject` y en `b.signature.who` |
| sc7 | error solo en `b.signature.who` | error en los dos referentes |

## c1: sc8 a sc10 (C1 y la fuga de `slicerecord` por un Container equivocado)

| Sonda | Qué prueba | HL7 (R4 = R4B = R5) | gofhir `87c0b02` (R4 = R4B = R5) |
|---|---|---|---|
| sc8 | sc6 con `managingOrganization` absoluta (`o` primero) | error solo en `contained[0].subject` | error en `o.subject` y en `b.signature.who` |
| sc9 | ídem, `b` primero | error en `contained[0].signature.who` | **ningún error** |
| sc10 | slice `a` de `generalPractitioner` por `resolve()`, Bundle contenido con `signature.who` = `"#"` | nada | error de perfil de destino en `contained[0].signature.who` y binding requerido de más en `generalPractitioner[0].type` |

## c2: sc11 a sc14 (C2, `holderGroups` que una corrida especulativa deja)

| Sonda | Qué prueba | HL7 (R4 = R4B = R5) | gofhir `87c0b02` (R4 = R4B = R5) |
|---|---|---|---|
| sc11 | obs1, p1, p2; p2 contiene `c` con `subject` p1 | `subject` de obs1 y `gender` de p1; nada en `link` | lo mismo, más «Unable to find a profile match for Patient/p2» en `entry[1].resource.link[0].other` |
| sc12 | p1 primero | sin error en `link` | sin error en `link` |
| sc13 | p2, obs1, p1 | sin error en `link` | el mismo error de más que sc11, en `entry[2]` |
| sc14 | control: `c.subject` = p2 | sin error en `link` | sin error en `link` |

## d1: `resolve()` con `#` y `#id`, y el Scope del destino de `#id` (D1)

Perfiles: `obs-pf-id` (`pfid-1`: `performer.resolve().ofType(Organization).name = id`), `obs-sub-hash` (`subh-1`:
`subject.resolve().ofType(Patient).name.family = id`), `bun-sig-id` (`bsig-1`, con `signature.who`), `bun-sig-org`
(`bsig-2`, con `signature.onBehalfOf`), `org-rel` (`orel-1`: `partOf.resolve().name = 'top'`), `bun-tp`
(`signature.onBehalfOf` con `targetProfile` `org-rel`) y `pat-cs` (`Patient.contained` en slices por perfil; el slice
`B`, `min` 1, es un Bundle `bun-tp`). Cada `#o` tiene un homónimo en otro lugar, con otro `name`, así que el
`resolve()` solo da el `name` esperado si busca en el recurso correcto.

| Sonda | Qué prueba | HL7 (R4 = R4B = R5) | gofhir `87c0b02` (R4 = R4B = R5) |
|---|---|---|---|
| d1a1, d1a2 | Patient P contiene `o` y un Bundle `bb` (dos órdenes). `bb.signature.onBehalfOf` = `#o` (el `o` de P), `who` = `"#"`. La entrada x1 tiene su propio `o` y `performer` = `#o`. La entrada `cq` contiene una Observation con `subject` = `"#"` | `bsig-1` y `subh-1` fallan; `bsig-2` y `pfid-1` se cumplen | igual |
| d1b1, d1b2 | Lo mismo en `Parameters.parameter.resource` (x1, x2, cq, cq2; dos órdenes) | `subh-1` falla en cq y cq2; `pfid-1` se cumple | igual |
| d1c1, d1c2 | Lo mismo como entradas de un Bundle (dos órdenes) | `subh-1` falla en cq y cq2; `pfid-1` se cumple | igual |
| d1a3, d1a4 | P en la raíz contiene `o` (`partOf` = `Organization/o2`) y `b` (`bun-tp`, `onBehalfOf` = `#o`); o2 está solo en `b` | error de perfil de destino en `b.signature.onBehalfOf` | igual |
| d1a5, d1a6 | P es una entrada de un Bundle raíz con o2 `top`; el o2 de `b` es `nested` (dos órdenes) | nada: `o` resuelve `partOf` donde está P | **error de perfil de destino de más** en `onBehalfOf` |
| d1a7, d1a8 | Los nombres al revés | error de perfil de destino | igual |
| d1a9 | d1a3 con `partOf` absoluta | error de perfil de destino | **nada**: encuentra el o2 de `b` |
| d1a10 | d1a5 con `partOf` absoluta | nada | **error de más** |
| d1a11 | d1a7 con `partOf` absoluta | error de perfil de destino | **nada** |
| d1a12 | d1a10 con `b` sin perfil, y P (`pat-cs`) que lo pone en el slice `B`: una corrida especulativa | nada | **error** «`contained:B` min 1, found 0» |
| d1a13 | d1a11 con P `pat-cs` | error «Slice 'Patient.contained:B': a matching slice is required» | **nada** |

Lo que muestran:
- El `resolve()` de FHIRPath sobre `"#"` no devuelve nada en HL7, en ninguna posición: `bsig-1` y `subh-1` fallan
  siempre. Así que `"#"` no depende del Scope en un invariante. Solo es observable cuando se sigue como referencia
  (sc6 a sc9). gofhir hace lo mismo.
- `#id` en un invariante resuelve en los contenidos del recurso que hace la referencia, en las tres posiciones y en los
  dos órdenes. gofhir coincide.
- D1 sí se observa en el **destino** de `#o` desde un Bundle contenido. En HL7, `o` resuelve sus referencias donde
  está P (d1a5 y d1a9 a d1a13). gofhir lo evalúa con el Container `b`, el de quien referencia: con una referencia
  absoluta encuentra el o2 de `b` (d1a9 a d1a11), y con una relativa no encuentra ninguno (d1a5 a d1a8). Pasa igual
  en la fase de referencias y en una corrida especulativa por discriminador (d1a12, d1a13).
- Un discriminador `resolve()` desde un elemento del propio Bundle contenido (el camino de `ResolveScoped`) no se
  puede armar: Bundle no tiene un `Reference` que se repita, y HL7 se niega a hacer slices de `Bundle.signature`
  («Attempt to a slice an element that does not repeat»). d1a12 y d1a13 llegan al mismo destino por otra corrida.

## d3: `holderGroups` reusado dentro de una corrida especulativa (D3)

Perfiles: `pat-t` y `obs-p` de c2, `pat-any` (Patient sin restricciones), `obs-za` (`subject` → `pat-t` o
`pat-any`; `performer` → solo `pat-t`), `obs-zb` (lo mismo, al revés) y `obs-ra`/`obs-rb` (`hasMember` →
`obs-za`/`obs-zb`). La raíz `r1` lleva a una sola corrida especulativa, `z` contra `obs-z?`. En esa corrida, la
referencia a p1 (que no cumple `pat-t`, pero sí `pat-any`) calcula `holderGroups(pat-t, p2)` con `c` en el slice `S`.
Lo hace por una respuesta provisional que depende de p1. p1 falla, y después, dentro de la misma corrida, la referencia
a p2 reusa esos grupos.

| Sonda | Qué prueba | HL7 (R4 = R4B = R5) | gofhir `87c0b02` (R4 = R4B = R5) |
|---|---|---|---|
| d3a1 | `z.subject` → p1 (tolerante), `z.performer` → p2 (`pat-t`) | nada | **error de más** en `r1.hasMember[0]`: «Unable to find a profile match for Observation/z among choices: …obs-za» |
| d3a2 | d3a1 con las entradas en el orden inverso | nada | el mismo error de más |
| d3b1, d3b2 | p2 antes que p1 (`obs-zb`) | nada | nada |
| d3c1 | control: `c.subject` = p2 | nada | nada |

El error depende del orden de las referencias dentro de `z` (d3a contra d3b), no del orden de las entradas. Ningún
recurso visible se valida contra `pat-t`, así que la diferencia sale entera de la corrida especulativa. Lo verifiqué
con dos mutaciones en una copia, sobre R4:
- `holderGroups` sin memo: desaparece el error de d3a y también el de sc11 y sc13.
- un memo nuevo en cada `check`: desaparecen los mismos errores.

Con un memo por `check`, el reuso que se ve acá ya no se da. Quedaría el reuso dentro del marco de un mismo `check`,
y ese no lo pude provocar. Los grupos de un `(def, inst)` se calculan en el marco del recurso que los tiene, y una
respuesta provisional en la que se apoyan depende de ese marco o de uno exterior. Esos marcos se cierran después que
el suyo, así que la respuesta no se invalida mientras el marco sigue usando los grupos.

## d5: fuga de `slicerecord` sin un Container equivocado (D5)

Perfiles: `pat-l1` (`Patient.link` en slices por `other.resolve()`; slice `L` con `targetProfile` `pat-b` y `type`
con binding requerido a `vs-refer`, que solo tiene `refer`), `pat-b` (`link.other` → `pat-l1`), `pat-c`, `pat-l2`
/`pat-b2` (lo mismo con `gender` 1..1 y `generalPractitioner` → `prac-c`), `pat-b3` y `obs-l1` (`subject` →
`pat-l1`). Todas las instancias usan `type` = `seealso`, así que un valor en `L` da error de binding.

| Sonda | Qué prueba | HL7 (R4 = R4B = R5) | gofhir `87c0b02` (R4 = R4B = R5) |
|---|---|---|---|
| d5s1a, d5s1b | ciclo de discriminador: R (`pat-l1`) → R2 → R | binding de `link[0].type` (dos errores: «System URI could not be determined» y «not found in the value set») | nada |
| d5s2 | control sin ciclo (R2 sin `link`) | los mismos dos errores | un error de binding |
| d5s3a, d5s3b | d5s1 más `link[1]` → R3 (`pat-c`), que vuelve a R en la fase de referencias | binding de `link[0]` y de `link[1]` | nada |
| d5s4a, d5s4b, d5s5 | `pat-l2`: el destino del discriminador necesita `gender`; `generalPractitioner` se comprueba en la fase de referencias | nada | nada |
| d5s6a, d5s6b | R con dos perfiles (`pat-l1`, `pat-l2`) | binding de `link[0]` | nada |
| d5s7a, d5s7b | una Observation (`obs-l1`) comprueba R contra `pat-l1` antes o después de que R se valide | perfil de destino en `o.subject` y binding de `link[0]` | nada |

**La fuga no se observa.** Para detectarla compilé una copia en la que `check` empieza con un `slicerecord` nuevo.
Sobre d5 da las mismas salidas que `87c0b02`, y sobre sc10 hace desaparecer el binding de más, así que el detector sí
la ve cuando existe. Hay una razón. El slicing visible corre antes que las fases de referencias y de bindings. Toda
respuesta de un discriminador en la validación visible queda en el memo, porque está en la profundidad 0. Una corrida
especulativa posterior, sobre el mismo perfil y el mismo map, reusa ese memo y graba lo mismo. Una anterior la
sobrescribe el slicing visible. Con el Scope por posición, el `resolve()` del discriminador da el mismo destino.

Las diferencias de la tabla son otra cosa. HL7 asume que un ciclo de discriminadores se cumple y pone el valor en `L`.
gofhir responde false. Es la política de ciclos de `Conforms` que el plan (§1) anota como pendiente aparte. La
diferencia de d5s2 (HL7 informa además «System URI could not be determined» en un `code`) tampoco es de esta pieza. El
mejor candidato para una sonda de D5 sigue siendo d5s1 (o d5s7): un ciclo de discriminadores con un binding en el
slice.

## r3 es un artefacto

`r3/` se conserva solo como evidencia de un error. Su `bundle-p` corta `Bundle.signature`, que en R4 es 0..1, así que
HL7 no puede armar el snapshot («Attempt to slice an element that does not repeat») y el `SLICING_CARDINALITY_MIN` de
r3a1 no dice nada sobre el Scope. No se usa como prueba ni como valor esperado. Lo sustituye `r4/` (perfil bien armado,
HL7 7.0.4 en R4, R4B y R5, generador `scripts/gen_r4.py`).
