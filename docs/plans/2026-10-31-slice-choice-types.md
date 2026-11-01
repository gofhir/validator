# Tipo de un valor contra las definiciones que lo gobiernan (gofhir/validator#187)

Rama `fix/slice-choice-types`, desde `a957f2e` (#183, sin mergear todavía).

## 1. Revisión previa (31-10-2026)

### El problema

Un perfil que restringe los tipos de un elemento de elección dentro de un slice
(`parameter[op].value[x] only code`) no se aplica: un miembro del slice con `valueString` pasa sin
error. Sondas (`evidence/2026-10-31-slice-choice-types/probe/`, HL7 validator_cli, R4, `cmp.txt`):

| Sonda | Restricción | Instancia | HL7 | Nosotros |
|---|---|---|---|---|
| t1 | `parameter.value[x]` code, sin slice | `valueString` | «allows for the type code but found type string» | «Unknown element 'valueString'» |
| t2 | slice `parameter:op`, `value[x]` code | `name=op`, `valueString` | el mismo error | **nada** |
| t3 | `parameter.part.value[x]` code (contentReference) | `valueString` | el mismo error | «Unknown element» |
| t4 | slice de `parameter.part`, code | `valueString` | el mismo error | **nada** |
| t5 | `Observation.value[x]` Quantity | `valueString` | el mismo error | «Unknown element» |
| t6a | t2 + fixed `abc` | `valueCode=zzz` | valor fijo | valor fijo (el slice se aplica) |
| t6b | igual | `valueString=abc` | el error de tipo | **nada** |
| t7 | slice `component:sis` (pattern en `code`), Quantity | `valueString` | el error de tipo | **nada** |
| t8 | el elemento sin cortar y el slice, los dos code | `valueString` | **un** error de tipo | «Unknown element» |
| t9 | sin cortar code\|string, slice code | `valueString` | **un** error de tipo, con los tipos del slice | **nada** |
| t10 | subextensión `extension:s.value[x]` Coding | `valueString` | el error de tipo | `EXTENSION_INVALID_VALUE_TYPE` (otro texto) |

Controles sin error en los dos validadores: t2ok (`valueCode`), t2other (otro `name`, no es miembro
del slice) y t7ok.

### Causa

- **Fase estructural:** resuelve cada clave contra el índice del perfil, que guarda un elemento por
  ruta. Los slices comparten la ruta del elemento que cortan, así que quedan fuera (`buildElementIndex`,
  `structural.go`). Una clave de elección se resuelve con `ChoiceTypeOf` sobre los tipos del perfil, no
  sobre los de la base: por eso un tipo que la base declara y el perfil no da «Unknown element» (t1, t3,
  t5, t8). El #180 dejó ese mensaje en el backlog (`2026-10-27-choice-key-case.md`, §2, fuera de
  alcance).
- **Fase de slicing:** `checkChildren` solo compara cardinalidades.
- **Fase de constraints** (`pkg/constraint/tree.go`): ya recorre, para cada valor, las definiciones que
  lo gobiernan. `governing` encuentra el slice con el mismo `slicematch.Matcher` que la fase de slicing,
  y `walk` suma el slice, los elementos que corta (`SliceOf`), el destino de una `contentReference`, el
  perfil del tipo y la definición que declara el valor. En cada capa, `checkValue` aplica fixed/pattern.
  Dentro de la fase, un mismo issue se informa una vez por ubicación (`WithReportScope`). Pero ninguna
  capa compara el tipo del valor.
- **Fase de extensiones:** compara el tipo del `value[x]` de una extensión o de una subextensión con los
  tipos de su definición (`extension.go`, `declares`) y da `EXTENSION_INVALID_VALUE_TYPE` (t10).

### La especificación (R4, R4B y R5)

- **`ElementDefinition.type`:** «The data type or resource that the value of this element is permitted
  to be». Un perfil restringe un elemento de elección eligiendo un subconjunto de esos tipos
  (profiling.html, «Restricting the types»).
- **Un slice:** es una definición más de los miembros que le corresponden, y sus reglas se aplican
  además de las del elemento cortado (profiling.html, §Slicing).
- **Un valor de elección:** su tipo lo da el nombre de la propiedad (formats.html#choice). El #183
  decide si una clave es un valor del elemento con los tipos de su base (`ChoiceType`).

Falta citar el texto exacto de cada versión (paso 3).

### Otras implementaciones

- **`gofhir/server`** delega en `validator/v2` v2.1.0: no hay nada que tomar.
- **HL7** (validator_cli de `hl7diff`) compara el tipo con cada definición que gobierna el valor y
  informa una sola vez, con los tipos de la definición más estrecha (t8, t9). El mismo texto vale para
  extensiones (t10).

## 2. Plan

### La regla

Un valor de un elemento de elección es del tipo que nombra su propiedad (formats.html#choice). Cada
definición que lo gobierna y declara tipos tiene que permitir ese tipo (`ElementDefinition.type`). Si
no lo permite, es un error de tipo, y cada valor se informa una sola vez.

### Alcance

1. **Fase de constraints:** en cada capa de `walk` en la que el valor es de elección y la definición
   declara tipos, si `cv.TypeCode` no está entre sus códigos, se informa un diagnóstico nuevo,
   `STRUCTURE_TYPE_NOT_ALLOWED` («The profile '{profile}' allows the type(s) {types} but found type
   {type}»), en la ubicación del valor. Se informa una vez por valor, aunque varias capas lo rechacen,
   con los tipos de la capa más estrecha. Cubre t2, t4, t6b, t7 y t9.
2. **Fase estructural:** una clave que `ChoiceType` reconoce como valor del elemento, porque la base
   declara su tipo, se resuelve como ese valor aunque el perfil no admita el tipo. Deja de ser
   «Unknown element»: el error de tipo lo da el punto 1 (t1, t3, t5, t8). La forma y el contenido del
   valor se validan con su tipo, como cualquier otro.
3. **Extensiones:** `EXTENSION_INVALID_VALUE_TYPE` se reemplaza por la regla general (t10), para que
   haya un solo chequeo y un solo mensaje. **Decisión del usuario:** este punto amplía el alcance; la
   alternativa es conservarlo y saltar en el punto 1 los valores de una extensión.

### Fuera del alcance

- Los tipos de elementos que no son de elección: `type.profile`, `targetProfile`.
- La expresión de la ubicación: HL7 escribe `value.ofType(string)`, nosotros `valueString`. Es una
  diferencia ya declarada.

### Criterio de salida

- t1–t10 en R4, R4B y R5 dan un error de tipo por valor, con los tipos de la definición más estrecha.
  Los controles no dan ninguno.
- `make check` en verde.
- `hl7diff run` sin diferencias nuevas. Las que cambian («Unknown element» → tipo equivocado) se
  comparan con HL7 issue por issue.
- `hl7diff same` contra la base: toda diferencia explicada.
- Barrido de claves de elección del #180 sin diferencias nuevas.
- Las mutaciones caen.

### Tests (paso 5), en R4, R4B y R5

1. t1–t10 como tests de `pkg/validator`, con los perfiles solo con diferencial. Los valores esperados
   salen de las sondas de HL7.
2. Variante de t2 con el slice por tipo de `value[x]`.
3. Reslice (`parameter:op/r`).
4. Un valor de un slice que no es de elección: sin cambios.
5. Un tipo que ni la base declara: sigue siendo «Unknown element» (#180).
6. Bundle y `contained`: la regla se aplica en cada recurso.

### Validación (paso 7)

- `hl7diff`, más un grupo nuevo con las sondas t1–t10 si el formato lo admite.
- **Mutaciones:**
  - no comparar en las capas de slice;
  - comparar con los tipos del perfil sin cortar;
  - informar en cada capa (duplicados);
  - quitar la resolución estructural del punto 2.
- **Costo:** el bench contra la base. Es una comparación por valor de elección y por capa.

### Excepciones a la regla 5 y dependencias

Ninguna: los tipos salen de `ElementDefinition.type` y el tipo del valor de su propiedad. Sin
dependencias nuevas.

## 3. Ronda adversarial (31-10-2026, solo Codex)

Codex leyó el código sin correrlo. Su informe completo está en
`evidence/2026-10-31-slice-choice-types/codex-adversarial.txt`.

| # | Sev. | Hallazgo | Resolución |
|---|---|---|---|
| H1 | alta | Un valor que solo trae `_valueString` (sin valor) no pasa por `walk`: lo atiende `checkValueless` (`tree.go:155`, `:485`) | **Aceptado:** la regla corre en los dos caminos; test con `_valueString` en t2 |
| H2 | alta | Reemplazar `EXTENSION_INVALID_VALUE_TYPE` quita el chequeo a quien llama `extension.Validator.ValidateData` (API pública, sin fase de constraints) | **Aceptado:** el punto 3 pasa a decisión del usuario, con la recomendación de conservar el chequeo de extensiones (§4) |
| H3 | media | «La capa más estrecha» no está definida cuando los tipos de dos perfiles no se contienen | **Aceptado:** una vez por valor, con la primera definición que lo rechaza en el orden de `walk` (el slice antes que el elemento cortado; los perfiles en el orden declarado). Una sonda HL7 con dos perfiles disjuntos fija el valor esperado antes del test |
| H4 | media | La exclusión mutua de variantes (`matchChoiceType`, `structural.go:171`) y los `_key` también reconocen claves con los tipos del perfil | **Aceptado:** el reconocimiento por la base es el mismo en la resolución, los `_key` y la exclusión mutua. Test con `valueCode` y `valueString` juntos, también en un Bundle y en `contained` |
| H5 | media | Faltan oráculos de lo que pasa después del error de tipo: fixed/pattern distinto, primitivo mal formado, binding, cardinalidad, discriminador por tipo | **Aceptado:** sondas HL7 con esas combinaciones; los conjuntos de issues esperados salen de ellas |
| H6 | media | Las tres versiones están afirmadas sin evidencia: sondas solo R4, citas pendientes | **Aceptado:** sondas en R4B y R5; citas por versión de `ElementDefinition.type`, slicing y choice |
| H7 | media | `hl7diff` empareja por familia y ubicación: una mutación de los tipos o del perfil del mensaje puede pasar; `cmp.py` acepta un resultado que falta | **Aceptado:** grupo de sondas obligatorio, mutaciones del contenido del mensaje; `cmp.py` falla si falta una sonda |
| H8 | baja | El criterio de salida incluye t6a, que es un error de valor fijo | **Aceptado:** cada sonda con su resultado esperado exacto |

## 4. Aprobación (31-10-2026)

El usuario aprobó el plan, con §3, y decidió **unificar** el chequeo de extensiones (punto 3).

Así queda resuelto H2:
- un diagnóstico nuevo, `STRUCTURE_TYPE_NOT_ALLOWED`, con el texto de HL7, reemplaza a
  `EXTENSION_INVALID_VALUE_TYPE`;
- en la cadena de validación, la regla corre una sola vez, en la fase de constraints. Esa fase recorre
  también la definición de la extensión y los slices de sus subextensiones;
- `extension.Validator` tiene una opción para no comparar tipos, y el validador la usa;
- usado por su cuenta (`extension.Validator.ValidateData`, API pública), sigue comparando con el mismo
  diagnóstico.

Antes de los tests, las sondas de §3 se corren en HL7 y fijan los valores esperados:
- H1: un valor que solo trae `_valueString`;
- H3: dos perfiles con tipos disjuntos;
- H4: dos variantes juntas, también en Bundle y en `contained`;
- H5: tipo equivocado combinado con fixed, con un primitivo mal formado, con binding, con cardinalidad y
  con un discriminador por tipo;
- H6: las tres versiones, R4, R4B y R5.

## 5. Plan rehecho con las sondas de §4 (31-10-2026)

Sondas: `probe/gen.py` y `probe/run.sh`; comparación en `probe/cmp-<versión>.txt`. R4, R4B y R5 dan lo
mismo, salvo una `ele-1` extra de HL7 en R4B (t11), que no tiene que ver con esta regla. Las sondas
cambian cuatro decisiones del plan:

| Sonda | HL7 | Cambio en el plan |
|---|---|---|
| t13: dos perfiles con tipos que no se contienen | un error por perfil («The type of element valueInteger is not known… Valid types at this point are code, string» y «…code, boolean») | H3: el error se informa una vez por valor **y por perfil**. Dentro de un perfil manda la primera capa que lo rechaza en el orden de `walk` (el slice antes que el elemento cortado, t9). Entre perfiles no se deduplica: los parámetros del issue difieren |
| t12: `valueCode` y `valueString` juntos, el perfil solo admite code | «Unrecognized property 'valueString'», sin error de tipo | H4: la variante que el perfil admite es el valor. Una variante que el perfil no admite es «Unknown element» si hay otra que sí admite; solo si no la hay se reconoce por la base y da el error de tipo. El error de cardinalidad máxima que damos además es anterior a esto: queda fuera |
| t14, t15, t11 | el error de tipo se suma al de valor fijo (t14), al de JSON mal formado (t15) y a `ele-1` (t11) | Los tests exigen los dos issues |
| t16: binding requerido en el slice | el error de tipo más dos de binding | La fase de binding no aplica un binding a un tipo que el elemento no admite (`binding/walk.go:146`, decisión anterior). La diferencia se declara; el test exige solo el error de tipo |
| t17: `min 1` en el `value[x]` del slice | solo el error de tipo | El test exige que no aparezca la cardinalidad mínima |
| t12b y t12c: Bundle y `contained` | el error de tipo | **Fuera del alcance:** hoy no validamos un recurso de un Bundle ni uno de `contained` contra su `meta.profile`, aunque su perfil no tenga slices (control en `probe/README.txt`). Issue aparte: gofhir/validator#188 |

**Texto del mensaje:** el diagnóstico nuevo lleva siempre los tipos admitidos y el tipo encontrado. HL7 usa
tres textos:
- «The Profile '…' definition allows for the type …» (t1–t9);
- «The type of element … is not known… Valid types at this point are …», cuando el perfil sin cortar
  admite varios tipos (t13);
- «The Extension '…' definition allows for the types […]…», en el `value[x]` de una extensión, informado
  en la extensión (t10b).

Las diferencias de texto y de ubicación se declaran después de ver cómo las agrupa `hl7diff` (paso 7).

**Tests (paso 5):** t1–t18 en R4, R4B y R5, con los issues esperados de `cmp-<versión>.txt`, salvo t12b y
t12c (fuera del alcance) y las diferencias declaradas arriba.

## 6. Tests primero (31-10-2026)

Sonnet escribió `pkg/validator/slice_choice_types_test.go` (`TestSliceChoiceTypes`, 24 casos, en R4, R4B y
R5) y `testdata/slice-choice-types/` (los perfiles de las sondas). En la verificación aparecieron cuatro
casos (t1, t3, t5, t8) que exigían «Unknown element» en `want` en lugar de `absent`: corregidos, junto
con nombres que no describían el caso.

Sobre `a957f2e` (`evidence/…/tests-red-a957f2e.txt`), 17 casos fallan en cada versión: falta
`STRUCTURE_TYPE_NOT_ALLOWED`, o aparece `STRUCTURE_UNKNOWN_ELEMENT` o `EXTENSION_INVALID_VALUE_TYPE`. Pasan
los controles (t2ok, t2other, t7ok, t18, t18ok) y t6a y t12, cuyo comportamiento esperado no cambia.

## 7. Implementación (31-10-2026, Sonnet)

- `pkg/constraint/tree.go`: `walk` aplica `checkType` en cada capa del slice y en cada destino de
  `contentReference` (también `checkValueless`). Un valor cuyo tipo no admite una definición que declara
  tipos da `STRUCTURE_TYPE_NOT_ALLOWED` («The profile '{profile}' allows the type(s) {types} but found type
  {type}»), una vez por valor y por perfil (`typeCheckSeen`); un valor de tipo equivocado toma sus hijos de
  su propio tipo (`wrongType`).
- `pkg/structural/structural.go`: el nombre de la propiedad se reconoce contra la base
  (`registry.ChoiceType` sobre `sd.Tree().ByID`), así que una variante que el perfil no admite deja de ser
  «Unknown element» (t1, t3, t5, t8) salvo que otra variante del mismo valor sí la admita (t12).
- `pkg/extension`: con la decisión «Unificar» (§4) desaparece `EXTENSION_INVALID_VALUE_TYPE`; la fase de
  extensiones deja el tipo a la de constraint (`WithoutValueTypeCheck`) y, sola, emite el diagnóstico nuevo
  con el URL de la extensión como perfil.
- `hl7diff`: la familia `extension-value-type` agrupa ahora `STRUCTURE_TYPE_NOT_ALLOWED`.
- Tests existentes ajustados al ID y texto nuevos; en `extension_value_test.go` la ruta de un error en una
  extensión de un primitivo pasa de `._family.extension…` a `.family.extension…` (se verifica contra HL7 en
  el paso 7).

`make check` en verde. Para el paso 8: la condición `sd.Kind == kindResource` de `untyped` en
`walkChildren` (caso t12) es una heurística; se revisa.

**Sondas con el binario nuevo (paso 7, 31-10-2026):** `cmp-<versión>.txt` regenerados (lado gofhir; HL7 no
cambia). El error de tipo aparece donde HL7 lo da en t1–t11 y t13–t17, en R4, R4B y R5. Quedan las
diferencias declaradas en §5 (texto y ubicación, t12, t13 con otro mensaje, binding de t16).

**#188 estaba mal diagnosticado.** t12b y t12c dan ahora el error de tipo, igual que HL7. La causa de
#188 no era que se ignorara el `meta.profile` de un recurso contenido: las fases de cardinalidad,
constraint y slicing ya validan cada recurso contenido contra su `meta.profile`
(`WalkAllWithProfilesContext`). Con el binario de `a957f2e`, un `Parameters` de perfil `p-t6` dentro de un
Bundle ya da el error de valor fijo. Lo que faltaba en t12b, t12c y en el control con `p-t1` era la regla
de tipos misma, que este incremento agrega; con el binario nuevo el control da el error en los dos
recursos. La línea `validator.go:705` (perfiles solo del recurso raíz) solo afecta a la fase estructural,
que ya no decide el tipo. Se propone cerrar #188 como resuelto por #187 (comentario redactado en
`evidence/…/issue-188-comment-draft.md`).

**hl7diff (paso 7, 31-10-2026):**
- `hl7diff run -baseline a957f2e` (`evidence/…/hl7diff-run.txt`): 51 grupos PASS, sin hallazgos.
- `hl7diff same -baseline a957f2e -heavy` (`evidence/…/hl7diff-same-a957f2e.txt`): 49 grupos idénticos; en
  extension-probes, reference-walk-probes y deqm-probes cambian 4 líneas, todas del renombre
  `EXTENSION_INVALID_VALUE_TYPE` → `STRUCTURE_TYPE_NOT_ALLOWED`, con la misma ubicación.
- Ruta de un error en una extensión de un primitivo: HL7 informa en `Patient.name[0].family.extension[0]`
  y `Patient.birthDate.extension[0]` (sin `_`), como la ruta nueva. La fase de extensiones sigue
  informando `EXTENSION_UNKNOWN` en `Patient._birthDate.extension[0]` (anterior a #187): backlog.

**Mutaciones, primera corrida** (`evidence/…/mutations.py`, `mutations-1.txt`): de 15, sobreviven 4.
- m1 (sin tipo en los slices del propio `value[x]`): ningún test cortaba un `value[x]` con un slice que
  recibiera un valor de otro tipo. Sonda t19: `value[x]` cortado con discriminador `exists` sobre `system`,
  slice `q` de Quantity, valor Coding con system. R4 profiling.html §5.1.0.11 no exige el discriminador
  `type` para cortar un elemento de elección; HL7 no genera el snapshot («Type slicing with
  slicing.discriminator.type != 'type'»), así que el valor esperado sale de la especificación. Se agrega
  t19 (y t19ok, t19none) al test.
- m3 (sin tipo en el destino de un `contentReference`): inalcanzable, un elemento con `contentReference`
  nunca tiene ruta `[x]`. Se quita esa llamada.
- m6 (perfil vacío en el mensaje): el test no comparaba el perfil. `notAllowed` exige el texto completo.
- m15 (la fase de extensiones sola, sin el error de tipo): el único llamador pasaba siempre la opción que
  lo apagaba. Se quita el chequeo de tipo de `pkg/extension` y la opción: la regla vive en la fase de
  constraint y `extension.New` vuelve a su firma.

**Mutaciones, segunda corrida** (`mutations-2.txt`), después de las correcciones de la primera: sin m3
(el código ya no existe) ni las dos de la opción de extensiones; nueva m14 (la fase de constraint no
compara el tipo del valor de una extensión). De 13, mueren 13.

**Barrido de claves de elección (#180)** (`sweep.txt`, con los recursos y claves que cubrió por versión): sin
diferencias fuera de las KNOWN, idéntico línea por línea a `a957f2e` (`sweep-a957f2e.txt`).

**Bench** (`evidence/2026-10-31-1718-bench-compare-a957f2ed470e/`, `COUNT=6 CPUS=1,8 make bench-compare
BASE=a957f2e`): asignaciones y memoria viva sin cambios; en tiempo, el informe marca dos benchmarks
paralelos como significativos con dispersión alta. Costo evitable encontrado: `checkType` arma la lista
de tipos antes de saber si el tipo está admitido (una asignación por valor de elección); se corrige con
el lote del paso 8 y se repite el bench.

## 8. Code review (solo Codex)

### Ronda 1 (31-10-2026)

Codex, sobre una copia, con reproducciones (`evidence/…/codex-r1/`). Cada hallazgo verificado sobre el
código real:

| # | Hallazgo | Sev. | Verificación | Resolución |
|---|---|---|---|---|
| 1 | Un perfil que despliega los hijos de `Extension` (p. ej. `Patient.extension.id max 0`): la caminata usa esos hijos y no la definición propia de la extensión; un `valueString` en una extensión que solo admite boolean no da error | alta | confirmado (R4, R4B, R5); en `a957f2e` la fase de extensiones lo informaba: **regresión** | Aceptado. Decisión del usuario: la fase de extensiones vuelve a ser dueña del tipo del valor contra la definición de la extensión, a toda profundidad; la de constraint compara solo con las definiciones del perfil y omite las que son la definición propia de un valor (`DefinitionSource`) |
| 2 | Un `Parameters` con `p-t1` y `valueCode` + `valueString` dentro de un Bundle: ni «Unknown element» ni error de tipo | media | confirmado, igual en `a957f2e` | Fuera de alcance: es el defecto real de #188, más acotado (la fase estructural no lee el `meta.profile` de un recurso contenido). Se reescribe el comentario de #188 para acotarlo en vez de cerrarlo |
| 3 | `pkg/extension` usado solo (API pública) ya no compara el tipo | media | confirmado | Aceptado; lo resuelve la decisión de #1 |
| 4 | Ningún test cubre un slice del propio elemento de elección en un primitivo sin valor (`checkValueless`) | media | confirmado con la mutación | Aceptado: test unitario en `pkg/constraint` |
| 5 | `checkType` asigna la lista de tipos aun cuando el tipo está admitido | baja | confirmado (1 asignación por valor) | Aceptado |
| 6 | Comentarios nuevos de `tree.go` en inglés | baja | confirmado | Aceptado |

Con la decisión de #1, la ruta de lo que la fase de extensiones informa dentro de un primitivo pasa de
`._family` a `.family`, como HL7 (sonda de §7).

**Correcciones de la ronda 1** (Sonnet): la fase de extensiones vuelve a informar el tipo del valor de una
extensión contra su definición; la de constraint lleva en sus opciones la definición propia del valor
cuyos hijos recorre (`opts.own`) y no compara con ella; `checkType` solo arma la lista de tipos al
informar; la ruta de lo que la fase de extensiones informa dentro de un primitivo pierde el guion bajo
(cambian también, por la misma regla, `held_resources_test.go` z8 y `json_shape_test.go` d4 y d5);
tests nuevos: `TestExtensionTypeUnderInlineChildren`, `pkg/extension/standalone_type_test.go`,
`pkg/constraint/valueless_choice_slice_test.go`.

Con las extensiones fuera de la fase de constraint, m9 y m10 sobrevivieron: las mataban tests de
extensiones. Sondas nuevas (`probe/`, t20, t21, t21one, en R4, R4B y R5):
- t20 (m10): el perfil despliega los hijos de un `value[x]` de Quantity; un `valueCodeableConcept` se
  recorre con los hijos de CodeableConcept. HL7 da, como nosotros, el error de tipo y `ext-1` en
  `coding[0].extension[0]`.
- t21 (m9): un perfil de Timing (tipo de datos, por `type.profile`) que restringe `bounds[x]` a Period,
  con `boundsPeriod` y `boundsDuration`. HL7: «Unrecognized property 'boundsDuration'». Nosotros: la
  exclusión mutua de la base (la fase estructural lee la definición base de un tipo de datos, no su
  perfil) y el error de tipo de `t-t21`. **Diferencia declarada.** Con una sola variante (t21one), HL7 da
  el mismo error de tipo. Backlog: la fase estructural no lee el perfil de un tipo de datos.

**Mutaciones, cuarta corrida** (`mutations-4.txt`), después de las correcciones de la ronda 1 y de t20/t21:
m1–m13 actualizadas a la firma nueva de `checkType`; m14–m18 nuevas (definición propia, fase de
extensiones, ruta sin guion bajo, `checkType` que rechaza todo). De 17, mueren 17.

**hl7diff después de la ronda 1:**
- `hl7diff same` (`hl7diff-same-a957f2e-r1.txt`): además del renombre, cambian las rutas de lo que la fase
  de extensiones informa dentro de un primitivo (`Patient._birthDate.extension[0]` →
  `Patient.birthDate.extension[0]`, también `EXTENSION_UNKNOWN`), como las informa HL7. En deqm, P4_prof
  da ahora dos `STRUCTURE_TYPE_NOT_ALLOWED`, uno por la definición de la extensión y otro por el slice
  del perfil `indv-measurereport-deqm`: HL7 da los mismos dos (`Extension_EXT_Type` y
  `Extension_PROF_Type`).
- `hl7diff run` (`hl7diff-run-r1.txt`): 50 PASS, 1 FAIL en extension-probes: con la ruta ya igual a la de
  HL7, `xd10` (valor en una extensión compleja, dentro de un primitivo) queda comparado, y HL7 informa ese
  caso como «definition allows for the types [] but found type string» (`Extension_EXT_Type`). Se agrega
  `EXTENSION_VALUE_NOT_ALLOWED` a la familia `extension-value-type` (antes «no HL7 counterpart verified
  yet»); extension-probes pasa (`hl7diff-run-r1-ext.txt`).
- Bench (`evidence/2026-10-31-1824-bench-compare-a957f2ed470e/`): asignaciones algo menores que la base;
  tiempos sin cambio significativo atribuible (máquina cargada, dispersión alta).

### Ronda 2 (31-10-2026)

El primer intento de Codex corrió sin poder escribir ni compilar (sin hallazgos verificados); se relanzó
con escritura. Reproducciones en `evidence/…/codex-r2/`, las dos confirmadas en R4, R4B y R5:

| # | Hallazgo | Sev. | Verificación | Resolución |
|---|---|---|---|---|
| A | `opts.own` omite toda la definición propia: una restricción más profunda (`Extension.value[x].repeat.bounds[x]` de una extensión con valor Timing) no la revisa nadie | alta | confirmado; en `a957f2e` tampoco se informaba (no es regresión), pero lo introdujo el `own` de la ronda 1 | Aceptado: la fase de constraint omite solo lo que compara la fase de extensiones (el valor de la extensión y de sus subextensiones) |
| B | Una subextensión declarada en un slice con url absoluta no se compara con su propia definición | alta | confirmado; igual en `a957f2e` | Aceptado: es la regla del hallazgo 1 de la ronda 1 a otra profundidad |

**Correcciones de la ronda 2** (Sonnet): A, `judgedByExtension` (`pkg/constraint/tree.go`): la fase de
constraint omite solo el elemento de elección hijo de la raíz de la definición propia o de un elemento de
su tipo (siguiendo `SliceOf`), lo que compara `pkg/extension`; todo lo más profundo lo revisa. B,
`validateOwnValue` (`pkg/extension`): una subextensión declarada con url absoluta compara además su valor
con su propia definición. Tests: `TestExtensionValueDescendantRestriction`,
`TestDeclaredSubExtensionOwnDefinition` (un issue por definición, R4/R4B/R5). Mutaciones
(`mutations-5.txt`): m19 y m20 nuevas; de 19, mueren 19.

### Ronda 3 (31-10-2026): no converge

Reproducciones en `evidence/…/codex-r3/`, las tres confirmadas en R4, R4B y R5:

| # | Hallazgo | Sev. |
|---|---|---|
| 1 | `judgedByExtension` omite también una restricción inline de la definición propia sobre una extensión dentro del valor (`Extension.value[x].extension:x.value[x]`), que `pkg/extension` no compara (la compara contra la definición de la extensión interna) | alta |
| 2 | Un slice del `value[x]` de la definición propia (discriminador `exists`) queda sin revisar: `judgedByExtension` sigue `SliceOf` y `validateExtensionValue` compara solo el elemento sin cortar | alta |
| 3 | `validateOwnValue` recorre otra vez las extensiones dentro del valor: el mismo error dos veces | media |

Tope de 3 rondas alcanzado. Diagnóstico: el reparto del tipo entre dos fases (extensiones: la definición
propia; constraint: el resto, con `own`/`judgedByExtension` para no duplicar) produce un caso borde nuevo
en cada ronda. Se consulta al usuario (fhir-incremento, paso 8).

## 9. Rediseño: la fase de constraint, única dueña (31-10-2026, vuelta al paso 2)

**Decisión del usuario** (después de la ronda 3): el tipo de un valor contra **toda** definición que lo
gobierna lo compara solo la fase de constraint; la definición propia de una extensión (la que nombra su
url, extensibility.html) es una definición gobernante más.

**Causa raíz.** En `walk`, los hijos de un valor salen de **un** árbol: el del elemento en el perfil
(`governedChildren(node)`, si el perfil los despliega) o, si no, el del tipo (`typeLayer`: `type.profile`,
la definición propia o el tipo base). El otro árbol gobierna solo la raíz del valor (`v.layer(root…)`).
Así, bajo un perfil que despliega los hijos, nada de la definición propia de la extensión (ni de un
`type.profile`) se aplica más abajo: el tipo (ronda 1, #1), los slices del `value[x]` (ronda 3, #2), las
restricciones inline (ronda 3, #1), y también invariantes y valores fijos.

**Diseño.**
1. Un valor lo gobiernan uno o más árboles: el **primario** (el elemento, sus slices y su
   `contentReference`, como hoy) y, cuando existe y es distinto del árbol del que salen los hijos, un
   **secundario**: la definición propia del valor (`selfDefinition`) o el `type.profile` del elemento
   (`TypeProfile`). El tipo base sin perfil no se agrega: el snapshot del perfil ya lo contiene.
2. En cada hijo, el nodo del árbol secundario es el hijo del mismo nombre (`Name()`); su slice, si lo
   hay, lo elige `governing` con su propia definición, y su cadena `SliceOf` se recorre como la del
   primario: `checkType` y `layer` por capa. La recursión lleva los nodos secundarios hacia abajo.
3. `checkType` informa una vez por valor **y por definición** (la clave incluye `sd.URL`): si el perfil y
   la definición de la extensión rechazan el mismo valor, son dos issues, como HL7 (deqm P4_prof:
   `Extension_PROF_Type` y `Extension_EXT_Type`). Las invariantes se siguen deduplicando por clave
   (`seen`).
4. Se quitan `opts.own`, `judgedByExtension`, `validateOwnValue` y el chequeo de tipo de `pkg/extension`
   (que conserva sus demás reglas: valor requerido o prohibido, subextensiones, contexto).
   **Cambio de API declarado:** `pkg/extension` usado solo deja de comparar el tipo del valor; la regla
   vive en la fase de constraint. Va en el CHANGELOG.
5. Sin cambios: la fase estructural (reconocimiento por la base, `untyped` para t12), la ruta sin guion
   bajo, la familia de hl7diff.

**Tests (paso 5):** los de §6 y §8 siguen; se agregan como tests las reproducciones de las rondas 1–3
(`codex-r1/`, `codex-r2/`, `codex-r3/`), con el valor esperado de la especificación y, donde HL7 genera el
snapshot, sondado en HL7 (R4, R4B, R5); y un caso de `type.profile` bajo un perfil que despliega el
elemento (un tipo de datos perfilado cuyas restricciones de hijos deben aplicarse). Mutaciones: las de
§7–§8 que sigan aplicando, más quitar el árbol secundario, no bajarlo a los hijos, y la clave sin
`sd.URL`.

**Costo:** el árbol secundario solo existe para valores con definición propia o `type.profile` y con
hijos desplegados por el perfil; se mide con el bench.

### Ronda adversarial de §9 (31-10-2026, solo Codex; `evidence/…/codex-plan9/`)

| Hallazgo | Sev. | Estado |
|---|---|---|
| Dos árboles no alcanzan: perfil + definición E de la extensión + definición F de una extensión anidada gobiernan a la vez; hace falta un **conjunto** de definiciones gobernantes, con su procedencia | alta | verificado en el código (`tree.go:76–85`) |
| `type.profile` con varios perfiles es un O (elementdefinition, R4/R4B/R5); `TypeProfile` hoy no devuelve ninguno | alta | verificado; **anterior a #187** |
| Emparejar por `Name()` falla con un snapshot publicado que nombra `valueQuantity` donde el otro dice `value[x]` | alta | verificado en parte |
| Recorrer solo los hijos del primario pierde las restricciones del secundario cuando el primario no despliega hijos: hace falta la unión | alta | omisión del diseño |
| Quitar `validateOwnValue` quita también valor requerido/prohibido de una subextensión declarada con url absoluta (agregado en esta rama); la fase de cardinalidad tampoco aplica la definición propia bajo hijos desplegados (`instance_children.go:36–38`) | alta | verificado; respecto de `a957f2e` no es pérdida |
| `seen` deduplica invariantes por clave y expresión sin severidad: un `x-1` error en el secundario se pierde si el primario lo tiene como warning | alta | verificado; anterior, pero el diseño lo agrava |
| Dos perfiles que recorren la misma extensión informan dos veces el rechazo de su definición: el tipo necesita la deduplicación compartida que ya usan fixed/pattern | media | verificado |
| `contentReference` y ciclos en el árbol secundario sin regla | media | hipótesis |
| Los tests y el bench del plan no cubren la arquitectura nueva | media | aceptado |

**Lectura:** el rediseño correcto es un cambio de arquitectura de la caminata (conjunto de definiciones
gobernantes por valor, unión de hijos, identidad de elementos de elección), que también toca la fase de
cardinalidad y `type.profile` múltiple. Excede #187. Se consulta el alcance al usuario.

### Decisión de alcance (31-10-2026)

El usuario elige dividir. #187 cierra con lo validado; las extensiones quedan **sin regresión respecto de
`a957f2e`**: la fase de extensiones compara el tipo del valor con la definición de la extensión (ID
nuevo) y la de constraint omite solo lo que esa fase compara (`judgedByExtension`, que revisa más que
`a957f2e`: las restricciones más profundas del valor, hallazgo A de la ronda 2). Se quita
`validateOwnValue` (hallazgo B de la ronda 2 y duplicado #3 de la ronda 3). Quedan para un frente nuevo,
todas ausentes también en `a957f2e`: hallazgos 1 y 2 de la ronda 3, B de la ronda 2, `type.profile`
múltiple, y la fase de cardinalidad bajo hijos desplegados. Borrador del issue y reproducciones:
`evidence/…/next-front/`. El rediseño de §9 no se implementa en #187. Antes de cerrar, una ronda más de
Codex sobre este alcance.

**Mutaciones en el alcance final** (`mutations-6.txt`, sin m20, cuyo código ya no existe): de 18, mueren 18.

**hl7diff en el alcance final:** `hl7diff run -baseline a957f2e` (`hl7diff-run-final.txt`): 51 grupos
PASS. `hl7diff same -heavy` (`hl7diff-same-a957f2e-final.txt`): los mismos grupos y líneas que después de
la ronda 1 (renombre del ID, rutas sin guion bajo dentro de un primitivo, y en deqm P4_prof los dos
issues que HL7 también da).

### Ronda 4, sobre el alcance final (31-10-2026)

Primer intento: límite de uso de Codex. Segundo: el sandbox no dejó compilar; revisión estática con un
hallazgo bajo, confirmado y corregido (comentario nuevo en inglés en `pkg/structural/structural.go:15`).
Barrido de #180 en el código final (`sweep-final.txt`): idéntico a `a957f2e`. Tercer intento, con
ejecución, en curso.

Tercer intento, con ejecución (`evidence/…/codex-r4/`): 747 combinaciones caso/versión comparadas con
`a957f2e`, sin pérdidas semánticas ni falsos positivos en los controles válidos, sin duplicados. Un
hallazgo bajo, confirmado: con la ruta sin guion bajo, un issue dentro del `_x` de un primitivo pierde su
ubicación en la fuente (`Location` nil; `a957f2e` daba línea y columna), porque `pkg/location` busca la
clave literal. Se corrige en `pkg/location`: si la propiedad no está y `_`+propiedad sí, la ruta sigue por
ella (json.html#primitive), en la misma pasada.

**Corrección de la ronda 4** (Sonnet, `pkg/location/location.go`): en la misma pasada, una propiedad `_x`
se recorre como el segmento `x` y lo que contiene se anota aparte (`shadow`); al terminar, cubre solo las
claves sin coincidencia literal (la clave literal gana: `Patient.birthDate` sigue en el valor). Un
primitivo que existe solo como `_x` se ubica ahí. Tests: `TestFindUnderPrimitiveSibling` (pkg/location),
`TestPrimitiveSiblingIssueLocation` (R4/R4B/R5). Mutaciones: sin anotar `shadow` (mueren los dos tests) y
`shadow` por encima de la clave literal (muere el de pkg/location).

### Comparación final y rendimiento (01-11-2026)

`hl7diff same -heavy` (`hl7diff-same-a957f2e-final.txt`): las dos «pérdidas de ubicación» que marcó el
clasificador son corrimientos del emparejamiento por línea ordenada (P5_cehrt_ok_prof y
P6_unknown_ext_str_prof ganan una línea antes); cada línea de la base reaparece en head con su ubicación.

Bench (`evidence/2026-11-01-0717-bench-compare-a957f2ed470e/`): ValidateRotating +47 asignaciones/op. Un
perfil exacto (`-memprofilerate 1`, los 4778 recursos) lo atribuye entero a la concatenación
`sdPath+"."+choiceBaseName` de `matchChoiceType`, que ahora también llama `baseOnlyChoiceKeys`; el scanner
de `pkg/location` suma ~0,002/op. Correcciones: `matchChoiceType` compara el prefijo sin concatenar (quita
también el costo que ya tenía la base), y el mapa `shadow` de `pkg/location` se crea solo cuando hay una
propiedad `_x` que ubicar. Bench final: `evidence/2026-11-01-0755-bench-compare-a957f2ed470e/` (sin cambios
de tiempo significativos; asignaciones de ValidateRotating por debajo de la base).

## Cierre (01-11-2026)

- `GOTOOLCHAIN=go1.25.13 make check`: `evidence/…/make-check-final.txt` (exit 0).
- Sondas contra HL7: `probe/cmp-*.txt`; diferencias declaradas en §7.
- hl7diff: `hl7diff-run-final.txt` y `hl7diff-same-a957f2e-final.txt`; barrido de #180: `sweep-final.txt`.
- Mutaciones: `mutations-6.txt`; ronda 4 de Codex: `codex-r4/`.
- Fuera de #187, publicados el 01-11-2026: el frente nuevo como gofhir/validator#193
  (`next-front/issue-governing-definitions-draft.md`) y el comentario que acota #188 (`issue-188-comment-draft.md`).
- CHANGELOG: lo genera release-please; el commit debe anotar el reemplazo de EXTENSION_INVALID_VALUE_TYPE
  por STRUCTURE_TYPE_NOT_ALLOWED y las rutas sin guion bajo dentro de un primitivo.
