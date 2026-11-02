# #196: dos variantes de un elemento de elección (antes #188)

Issue: gofhir/validator#196, que reemplaza a #188 (cerrado el 02-11-2026 con la evidencia de §1). Rama
`fix/choice-two-variants`, desde `e22324e`.

## 1. Revisión previa (01-11-2026)

**`gofhir/server`:** no valida por su cuenta los recursos contenidos; entrega el recurso completo al
validador (`internal/operations/message_validation.go`). Nada que tomar: **descartar**.

**Especificación:** `Resource.meta.profile` (R4, R4B, R5): «A list of profiles … that this resource claims
to conform to». La afirmación es de cada recurso, esté donde esté. Un elemento de elección es un solo elemento
(formats.html §2.6.0.1.2: «the authoring system must create a single element with a data type chosen from
among the list of permitted data types»; «Elements that have a choice of data type cannot repeat - they must
have a maximum cardinality of 1»).

**Sondas** (`evidence/2026-11-01-held-meta-profile/probe/`, `cmp-<versión>.txt`, 17 casos en R4, R4B y R5):

1. Un recurso contenido (entrada de Bundle, `contained`, `Parameters.parameter.resource`, Bundle dentro de
   Bundle, perfil por `type.profile` del elemento que lo contiene) **ya** se valida contra su
   `meta.profile` en las fases que dependen del perfil: `b-one` y `c-one` dan el error de tipo igual que
   HL7, en la ruta del recurso contenido (lo arregló #187).
2. **El parser de HL7 no mira el perfil.** Con dos variantes del mismo elemento de elección, HL7 da
   «Unrecognized property '<la segunda>'» en el objeto que las tiene, **con perfil o sin él** (`r-none-two`),
   en la raíz igual que en un recurso contenido. Decide el **orden del documento**: la primera variante es
   el valor (`r-rev`, `r-none-rev`, `b-rev`: con `valueString` antes que `valueCode`, sobra `valueCode` y
   el perfil `p-code` informa el tipo de `valueString`). No da error de cardinalidad.
3. gofhir, en cambio:
   - sin perfil, o en un recurso contenido: «Only one variant of choice type 'value[x]' is allowed, but
     found: …» más CARDINALITY_MAX en `value[x]`;
   - en la raíz con perfil: elige la variante por el perfil (H4 de #187, `baseOnlyChoiceKeys`):
     «Unknown element» en la que el perfil no admite, más CARDINALITY_MAX. `r-rev` muestra que HL7 no
     elige así.
4. Cardinalidad con dos perfiles (`evidence/…/dup/`): HL7 informa un issue por definición (la base y cada
   perfil), en la raíz y contenido; gofhir uno por perfil. No es un defecto de este frente.

**Conclusión:** lo que #188 pide (que la fase estructural lea el `meta.profile` de un recurso contenido) no
es lo que hace HL7: su parser decide con la definición base y el orden del documento, y el perfil se aplica
después, en las fases que gofhir ya corre sobre los recursos contenidos. La diferencia real es otra y vale
igual en la raíz y en un recurso contenido: **dos variantes de un elemento de elección**. A decidir con el
usuario antes del paso 2.

**json.html §2.6.2.1** (R4, R4B, R5): «The order of properties of an object is not significant in the JSON
representation». La elección de HL7 por el orden del documento no es una regla a copiar: el resultado no
debe cambiar al reordenar las claves (también la prueba metamórfica de la carta, §10 paso 7).

**Decisión del usuario (01-11-2026):** cerrar #188 con la evidencia y abrir un issue nuevo, «dos variantes de
un elemento de elección», que se trabaja ya como este incremento. Borradores:
`evidence/2026-11-01-held-meta-profile/issue-two-variants-draft.md` y `issue-188-close-comment.md`
(`#NEW` se reemplaza por el número del issue nuevo). El clasificador de auto mode bloqueó la publicación:
la corrió el usuario: #196 publicado y #188 cerrado (02-11-2026).

## 2. Plan (02-11-2026)

### Hoy (`e22324e`)

- Estructural (`pkg/structural/structural.go`, `validateElement`): `baseOnlyChoiceKeys` deja como valor una
  variante que solo la base admite **si ninguna otra variante del mismo elemento la admite el perfil**; si
  otra la admite, la primera queda «Unknown element». Por eso el resultado depende del perfil, y la raíz
  (que se valida contra el perfil) difiere de un recurso contenido (contra su tipo base).
- Estructural: con dos o más variantes que cuentan como valor, STRUCTURE_CHOICE_MUTUAL_EXCLUSION en el
  objeto, con las variantes en orden de nombre (no depende del orden del documento).
- Cardinalidad (`pkg/cardinality/cardinality.go`, `validateChildren`): cuenta cada variante como un valor
  del elemento: dos variantes dan además CARDINALITY_MAX en `value[x]`.
- Constraint (`pkg/constraint/tree.go`, `unrecognizedChoice`, la heurística `untyped` de t12 en #187): no
  compara con el perfil el tipo de una variante si otra variante del mismo elemento sí lo declara.

### Alcance

1. **Estructural sin perfil en la decisión:** toda variante que la base del elemento admite es un valor de
   su tipo (formats.html#choice), haya o no otra que el perfil admita. Se quita la condición `allowed` de
   `baseOnlyChoiceKeys`. Con dos o más, un solo STRUCTURE_CHOICE_MUTUAL_EXCLUSION en el objeto, igual en la
   raíz, en un recurso contenido y con cualquier orden de claves.
2. **Un hecho, un issue:** dos variantes son el mismo hecho que un elemento repetido (formats.html
   §2.6.0.1.2); la cardinalidad no informa CARDINALITY_MAX en un elemento de elección por
   tener varias variantes; eso lo informa la fase estructural. Regla: en un elemento de elección
   (`[x]`), las variantes presentes en un objeto son **una** aparición del elemento. Un `max` de 0 sigue
   informándose.
3. **Tipo contra el perfil, sin depender del orden:** cada variante se compara con los tipos del perfil
   (se quita `unrecognizedChoice`/`untyped`). Con `p-code`, `valueString` da STRUCTURE_TYPE_NOT_ALLOWED en
   cualquier orden; HL7 lo da solo cuando `valueString` va primero (`r-rev`), porque descarta la segunda.
4. **hl7diff:** la familia nueva `choice-variants` (regla `parent`): gofhir
   STRUCTURE_CHOICE_MUTUAL_EXCLUSION ↔ HL7 «Unrecognized property» en el objeto. Se quita del listado
   «no HL7 counterpart verified yet». HL7 da un «Unrecognized property» por cada variante descartada;
   gofhir uno por elemento: la regla `parent` compara por objeto.

### Fuera del alcance

- Que la fase estructural lea el `meta.profile` de un recurso contenido (lo que pedía #188): HL7 no lo hace.
- Un issue de cardinalidad por definición (base y cada perfil, §1.4): HL7 informa la base también; gofhir
  no. Si se quiere igualar, es otro frente.

### Diferencias declaradas con HL7

- Mensaje y cantidad: un STRUCTURE_CHOICE_MUTUAL_EXCLUSION con todas las variantes, contra un
  «Unrecognized property» por variante descartada (json.html §2.6.2.1: el orden no es significativo, no hay
  «primera» variante).
- El error de tipo de una variante que el perfil no admite sale en cualquier orden (ver 3).

### Criterio de salida

- Las sondas `r-two`, `r-rev`, `r-none-two`, `r-none-rev`, `b-two`, `b-rev`, `c-two`, `pr-two`, `bb-two`,
  `h-two`, `b-cs-two`, `b-two-profiles` dan en R4, R4B y R5 los mismos issues entre sí (salvo la ruta), sin
  CARDINALITY_MAX, y el error de tipo de `valueString` donde el perfil es `p-code`.
- Metamórfico: cada sonda con las claves de cada objeto en orden inverso da los mismos issues.
- `hl7diff run` sin grupos que fallen; `hl7diff same` contra `e22324e` solo con los cambios de arriba,
  cada uno clasificado.
- Mutaciones de los oráculos, bench sin regresión, `make check` en verde.

### Tests (paso 5), R4, R4B y R5

- `pkg/validator/choice_variants_test.go`: las sondas de arriba como casos de tabla, con los issues
  esperados de la regla (formats.html#choice, json.html §2.6.2.1) y la evidencia de HL7 (`cmp-*.txt`) para
  la ubicación y lo que no es diferencia declarada; cada caso también con las claves invertidas.
- Cardinalidad: un elemento de elección con `max` 0 en un perfil y una variante presente sigue dando
  CARDINALITY_MAX; con dos variantes y `min` 1, sin CARDINALITY_MIN.
- Los tests de #187 que fijaban t12 (`slice_choice_types_test.go`) cambian a la regla nueva: es una
  decisión de #187 que este frente revierte con evidencia (`r-rev`).

### Validación (paso 7)

Sondas regeneradas (`probe/run.sh`), hl7diff run y same, barrido de #180 (`-choice-key-sweep`), mutaciones
en una copia (sin la quita de `allowed`, sin la regla de cardinalidad, con `untyped` de vuelta, orden de las
variantes por documento), bench contra `e22324e`.

### Excepciones a la regla 5 y dependencias

Ninguna. El sufijo `[x]` es la forma en que la especificación nombra un elemento de elección
(elementdefinition.html#path), ya leído así en el código. Sin dependencias nuevas.

## 3. Ronda adversarial (02-11-2026)

Codex sin cuota hasta el 04-11-2026. **Excepción a la regla «solo Codex», decidida por el usuario el
02-11-2026:** esta ronda la hace un agente Opus independiente (`general-purpose`, `model: opus`), con el
mismo encargo, sin el razonamiento del plan, sobre la copia `scratchpad/codex196p`.
Prueba del revisor: `evidence/…/opus-r0-attack_test.go.txt`. Sondas que agregaron sus hallazgos (gen.py, 27
casos): `o-u1`, `o-u2`, `e-u1`, `e-two`, `r-three`, `o-qs`, `p-dec`, `o-timing`, `q-cr`, `o-ts-two`.

| # | Hallazgo | Verificación | Resolución |
| --- | --- | --- | --- |
| 1 alto | Una variante que existe solo como hermano `_x` cuenta para la cardinalidad (elementvalues) pero no para la exclusión estructural (que salta las claves `_`): con la regla 2, `valueString` + `_valueTime` no daría nada | Confirmado: `o-u1` y `o-u2` dan hoy solo CARDINALITY_MAX. HL7 es inconsistente: `o-u1` nada, `o-u2` «Unrecognized property '_valueTime'», `e-u1` toma `code` como tipo del valor | **Aceptado.** «Variante» se define una vez, desde `elementvalues.Of` (tipos que la base declara, con los hermanos `_x` de primitivos, json.html#primitive); la fase estructural arma la lista de MUTUAL_EXCLUSION desde ahí. `o-u1`, `o-u2`, `e-u1` dan MUTUAL_EXCLUSION; diferencia declarada con HL7 (su resultado depende del orden y del prefijo) |
| 2 alto | La familia `choice-variants` no puede funcionar: `HL7Class` devuelve la primera familia que coincide y «^Unrecognized property» ya es de `unknown-element`; el emparejamiento es 1:1 | Confirmado (`families.go`, primer `HL7NoID` que coincide); `r-three`: HL7 da dos «Unrecognized property», gofhir uno | **Aceptado.** STRUCTURE_CHOICE_MUTUAL_EXCLUSION entra en la familia `unknown-element` (regla `parent`); con tres o más variantes, el excedente de HL7 queda como diferencia declarada clasificada en `hl7diff same`, no como familia nueva |
| 3 medio | Muestra sesgada: solo `Parameters.parameter.value[x]` con code y string | Confirmado | **Aceptado.** Sondas nuevas: complejo + primitivo (`o-qs`), elemento de recurso (`p-dec`), extensión (`e-two`), dentro de un tipo de datos (`o-timing`), contentReference (`q-cr`), slices por tipo (`o-ts-two`). HL7 da «Unrecognized property» en todos, en el objeto |
| 4 medio | Regla 1 con slicing cerrado por tipo, y `slicing.checkChildren` cuenta `len(values)` | Confirmado (`pkg/slicing/slicing.go`, `checkChildren`) | **Aceptado.** `o-ts-two` esperado: MUTUAL_EXCLUSION + SLICING_NO_MATCH de `valueBoolean` (el slicing cerrado lo rechaza; es un hecho distinto); HL7 solo «Unrecognized property»: diferencia declarada. La regla 2 se implementa en una función de `internal/elementvalues` (apariciones de un elemento) que usan la cardinalidad y el slicing |
| 5 medio | La estructural (`matchChoiceType`, `byProfile`) y elementvalues deciden «variante» distinto para un tipo que el perfil declara y la base no | Por lectura (`structural.go`, `matchChoiceType`); el caso de `choice_key_case_test.go` (citizenship, R4) | **Aceptado.** El predicado de variante es el de la base (`registry.ChoiceType`) en todas las fases; test con ese caso |
| 6 medio | El metamórfico de orden y «min 1 sin CARDINALITY_MIN» ya pasan hoy: no son oráculos | Confirmado: `r-two` y `r-rev` dan lo mismo en gofhir (`cmp-4.0.1.txt`) | **Aceptado.** Se mantienen como regresión, no se cuentan como oráculos; se descarta la mutación «orden del documento». Mutaciones nuevas: sin hermanos `_` en las variantes, slicing contando N |
| 7 bajo | `max` 0: el test debe fijar el conteo del mensaje | — | **Aceptado** («found 1») |
| 8 bajo | Sin `untyped`, un test de exactamente un TYPE_NOT_ALLOWED y un MUTUAL_EXCLUSION, sin duplicados | — | **Aceptado** |
| 9 bajo | Los números de sección pueden cambiar entre versiones | — | **Aceptado:** se citan las anclas (formats.html#choice, json.html#primitive, json.html) con el texto citado |
| 10 bajo | Sin excepciones a la regla 5 | — | Sin cambio |

### Plan ajustado (sustituye lo que contradiga a §2)

- **Variante** (una definición, en `internal/elementvalues`): cada propiedad de un objeto que nombra un tipo
  que la base del elemento de elección declara, con su hermano `_x` si es primitivo, o solo el hermano.
  La usan las fases estructural, de cardinalidad y de slicing.
- **Estructural:** un STRUCTURE_CHOICE_MUTUAL_EXCLUSION por elemento con dos o más variantes, con sus
  nombres en orden; el perfil no decide (sin `allowed` ni `byProfile`); un tipo que solo el perfil declara no
  es variante (#180).
- **Apariciones:** una función de `internal/elementvalues` da las apariciones de un elemento en un objeto; en
  uno de elección, las variantes presentes son una. La usan la cardinalidad y `slicing.checkChildren`.
- **Tipo:** cada variante contra el perfil (sin `untyped`).
- **hl7diff:** STRUCTURE_CHOICE_MUTUAL_EXCLUSION en la familia `unknown-element`.
- **Criterio de salida:** además de §2, las sondas nuevas con su resultado esperado de la tabla, en R4, R4B y
  R5; mutaciones que pueden morir (hallazgo 6).

## 4. Aprobación (02-11-2026)

El usuario aprobó el plan ajustado de §3.

## 5. Tests primero (02-11-2026, Sonnet)

`pkg/validator/choice_variants_test.go` (`TestChoiceVariants`: 28 casos de §2/§3 más cardinalidad y slicing
con `max` 0 y `min` 1, en R4, R4B y R5; `TestChoiceVariantsKeyOrder`: guarda de regresión, no oráculo) y
perfiles en `pkg/validator/testdata/choice-variants/`. t12 de `slice_choice_types_test.go` pasa a la regla
nueva. En rojo antes de implementar: t12 y los casos de la regla; pasan ya `max0 one variant`,
`slice child max0, one variant`, el caso de un tipo que solo declara el perfil (#180) y el de orden inverso.

## 6. Implementación (02-11-2026, Sonnet)
Sonnet implementó §3 (`internal/elementvalues`: `variantOf`, `CompetingVariants`, `Occurrences`;
`pkg/structural`: `reportCompetingVariants`, `baseOnlyChoiceKeys` sin `allowed`; cardinalidad y slicing con
`Occurrences`; `pkg/constraint`: sin `unrecognizedChoice`/`untyped`; hl7diff: MUTUAL_EXCLUSION en
`unknown-element`). `go test ./...` verde; lint de los tests nuevos (palabras en español que `misspell` marca,
dos de gocritic) corregido; `make check` verde.

## 7. Validación (02-11-2026)

- Sondas (`probe/cmp-*.txt`; las de antes en `before/`): las 24 con dos o más variantes dan un
  MUTUAL_EXCLUSION en R4, R4B y R5, sin CARDINALITY_MAX ni «Unknown element».
- `hl7diff run -baseline e22324e` (`hl7diff-run.txt`): 51 grupos PASS. `hl7diff same -heavy`
  (`hl7diff-same-e22324e.txt`): idéntico en todos los grupos, incluidos los 5301 ejemplos de R4: el corpus no
  tenía ningún objeto con dos variantes. Por eso las sondas entran a hl7diff como grupo
  `choice-variant-probes` (R4): sin él, la familia `unknown-element` no tenía caso que la probara.
- El grupo nuevo mostró que `namesAgree` no empareja un MUTUAL en el objeto con el «Unrecognized property»
  de HL7 (exige que la ubicación de gofhir termine en la propiedad que HL7 nombra). Se corrige en hl7diff
  (Sonnet) y se declaran las diferencias de §3 en `declared-divergences.json` (decisión «#196»).
- hl7diff (Sonnet): `namesAgree` empareja un error de gofhir en el mismo elemento cuyo mensaje lista la
  propiedad que HL7 nombra tras «but found: » (sin el «_» de un hermano, json.html#primitive);
  `TestNamesAgreeChoiceVariants`. Diez diferencias declaradas «#196» (ocho STRUCTURE_TYPE_NOT_ALLOWED de la
  segunda variante; MUTUAL_EXCLUSION de `o-u1` y `e-u1`). `hl7diff run` completo (`hl7diff-run-final.txt`):
  52 grupos PASS.
- Mutaciones (`mutations.py`, `mutations-1.txt`): de 10, mueren 10.
- Bench (`evidence/2026-11-02-1146-bench-compare-e22324e6c0e9/`): asignaciones −0,35% (geomean); los tiempos
  no sirven (dispersión de ±36% a ±549%, carga de 13 a 21 por tests de otras sesiones). Se repite en el cierre.

## 8. Code review

Revisor: agente Opus independiente (excepción hasta que vuelva Codex, §3), sobre la copia `scratchpad/rev196`.

### Ronda 1 (02-11-2026)

Pruebas del revisor: `evidence/…/review-r1/`.

| # | Hallazgo | Verificación | Resolución |
| --- | --- | --- | --- |
| 1 medio | La estructural todavía acepta una clave de un tipo que solo declara el perfil (`resolveElementDefinition`, `matchChoiceType`), que `variantOf` ignora: en la raíz, `valueCode` + `valueCodeableReference` (R4, perfil `p-cr`) no da nada; contenido da «Unknown element». Regresión: la base daba MUTUAL | Confirmado (`zz3_review_test.go`: `cr-and-code` sin issues; `held-cr-and-code` UNKNOWN_ELEMENT) | Aceptado: la estructural reconoce una clave de elección solo por los tipos de la base (#180), en la raíz igual que contenido |
| 2 medio | hl7diff empareja un MUTUAL con un «Unrecognized property» en el objeto padre o hijo (regla `parent`) | Confirmado: `TestReviewMutualPairsAcrossObjects` falla (Observation ↔ Observation.code; repeat ↔ effectiveTiming) | Aceptado: por la lista de variantes, solo en la misma ubicación |
| 3 bajo | Comentario viejo en `pkg/extension/extension.go` (la cardinalidad ya no informa varias variantes) | Lectura | Aceptado |
| 4 bajo | `ListsVariant` exportada sin motivo y atada solo al texto | Lectura | Aceptado: sin exportar y atada al ID |
| 5 bajo | El package de prueba se llama `example.held` | Log de hl7diff | Aceptado: `example.choice-variants` |
| 6 bajo | `corpus.json` reformateado entero | `git diff` | Aceptado: solo el grupo nuevo |
| 7 bajo | `Occurrences` supone valores contiguos por clave | Lectura | Aceptado: documentado |

Correcciones de la ronda 1 (Sonnet): la estructural reconoce una clave de elección solo por los tipos de la
base (se quitan `matchChoiceType` y `baseOnlyChoiceKeys`; casos `cr-*` en raíz y contenido, R4); hl7diff
exige la misma ubicación cuando empareja por la lista de variantes; `listsVariant` sin exportar y atada al ID;
package `example.choice-variants`; `corpus.json` solo con el grupo nuevo; `Occurrences` documentada.
`go test ./...`, `make check` y `hl7diff run` (`hl7diff-run-r1.txt`, 52 PASS) en verde. Mutaciones
(`mutations-2.txt`; m7 reescrita para la regla nueva, m11 nueva): de 11, mueren 11.

### Ronda 2 (02-11-2026)

Revisor Opus independiente, sobre `scratchpad/rev196b`, con los hallazgos de la ronda 1 y el diff de sus
correcciones.

Resultado de la ronda 2: **converge**, sin hallazgos de severidad media o mayor. El revisor verificó las dos
correcciones con mutaciones (vuelta al tipo del perfil en `resolveElementDefinition`; regla `parent` y sin
el ID en `listsVariant`: fallan los tests) y la coherencia entre fases (estructural, primitivos y
`variantOf` deciden con `registry.ChoiceType`). Dos menores:

| # | Hallazgo | Resolución |
| --- | --- | --- |
| 1 bajo | Un snapshot sin `id` en sus elementos deja toda clave de elección desconocida | Sin cambio: sdf-16 exige `id` en cada elemento del snapshot; ahora es igual en todas las fases |
| 2 nit | `ElementDefinition.ChoiceTypeOf` quedó sin uso fuera de los tests | Es API exportada de `pkg/registry`: no se quita; su documentación aclara que una propiedad de una instancia se reconoce por `Registry.ChoiceType` |

## 9. Cierre (02-11-2026)

- `GOTOOLCHAIN=go1.25.13 make check`: `evidence/…/make-check-final.txt` (exit 0).
- El primer `hl7diff same` final mostró que el patrón `probes/c*_*.json` de `constraint-probes` también tomaba
  las sondas `cv_*` (sin su package). Se renombran a `ov_*` (sin coincidencia con otro patrón del corpus,
  comprobado) en `corpus.json` y `declared-divergences.json`.
- `hl7diff run` (`hl7diff-run-final.txt`): 52 grupos PASS. `hl7diff same -heavy` (`hl7diff-same-e22324e-final.txt`):
  idéntico a `e22324e` en todos los grupos salvo `choice-variant-probes`, el grupo nuevo, cuyo cambio es el de
  este frente.
- Barrido de #180 (`sweep-final.txt`): idéntico al de #187 (`2026-10-31-slice-choice-types/sweep-final.txt`).
- Bench (`evidence/2026-11-02-1301-bench-compare-e22324e6c0e9/`; carga de la máquina al inicio y al final en
  `bench-final.log`): asignaciones por debajo de la base (como en `2026-11-02-1146-…`, −0,35% geomean). Los
  tiempos no son medibles con la carga de otras sesiones (de 15 a 21; dispersión hasta ±268%): ninguna fila
  significativa en ningún sentido. El revisor de la ronda 2 midió el mismo número de asignaciones y bytes por
  objeto que la base en un Observation con elecciones en la raíz y en componentes. **Pendiente:** repetir el
  tiempo con la máquina libre antes del release.

**Para el commit:** mensaje con el cambio de comportamiento (dos variantes: un STRUCTURE_CHOICE_MUTUAL_EXCLUSION,
sin CARDINALITY_MAX ni STRUCTURE_UNKNOWN_ELEMENT; un tipo que solo declara el perfil es elemento desconocido
también en la raíz; error de tipo para cada variante). Closes #196.
