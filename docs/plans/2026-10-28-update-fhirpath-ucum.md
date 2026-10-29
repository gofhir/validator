# #176: fhirpath v1.16.6 y ucum v4.3.4

Issue: gofhir/validator#176. Rama `chore/update-fhirpath-ucum`, desde `main` (`548ad1f`).

## 1. Revisión previa (28-10-2026)

**Versiones.** `go.mod` exige `github.com/gofhir/fhirpath v1.13.6` y `github.com/gofhir/ucum/v4 v4.2.0`.
Las últimas son fhirpath v1.16.6 (18-10-2026) y ucum v4.3.4 (29-10-2026). El issue nombra ucum v4.3.0,
pero fhirpath v1.16.6 ya exige ucum v4.3.0 y salieron cuatro versiones de corrección después. Con la
selección de versión mínima, un programa que use ucum v4.3.4 correría el validador con una combinación
que nunca probó, así que se sube a v4.3.4. Notas de cada versión en
[evidence/2026-10-28-update-fhirpath-ucum/](evidence/2026-10-28-update-fhirpath-ucum/).

**Medición barata.** En una copia de `main`, con solo `go get` de las dos versiones y `go mod tidy`:
`go build ./...` y `go vet ./...` pasan sin cambios de código. En la rama, `make compare-baseline
BASE=548ad1f` (`hl7diff same -heavy` sobre todo el manifiesto) da **IDÉNTICO**
(`same-deps-only.txt`). `same` compara dos refs nuestros, no contra HL7: dice que ninguna salida del
validador cambia en el manifiesto con las versiones nuevas (mientras siga su propio `htmlChecks()`), no
que sean correctas. El manifiesto casi no ejercita lo que cambia (§7, F1 y F7).

**`htmlChecks()`.** fhirpath v1.15.0 implementa `htmlChecks()` y pide las reglas de HTML a un
`eval.HTMLChecker` que da quien evalúa (`Context.SetHTMLChecker`, `fhirpath.WithHTMLChecker`): las reglas
son de FHIR, no del lenguaje. El validador, en `pkg/constraint/htmlchecks.go`, registra en un `init` una
función global con el mismo nombre (`funcs.Register`), que pisa la de la librería. Eso rodea la librería
(carta, principio 10): se deja de sobrescribir su función y se le da el verificador por su punto de
extensión. Especificación: R4 [fhirpath.html#functions](https://hl7.org/fhir/R4/fhirpath.html#functions),
«When invoked on an xhtml element returns true if the rules around HTML usage are met, and false if they
are not. The return value is undefined (null) on any other kind of element»; R5
([fhirpath.html#functions](https://hl7.org/fhir/R5/fhirpath.html#functions)) dice «a single xhtml
element» y da vacío también en una colección de elementos xhtml. Reglas de HTML: R4 [narrative.html#xhtml](https://hl7.org/fhir/R4/narrative.html#xhtml).

Diferencias entre las dos funciones, que el cambio vuelve visibles:

- La de la librería da vacío si la entrada no tiene exactamente un ítem (como dice el texto); la del
  validador toma el primero de varios.
- La de la librería solo acepta un `String` de tipo `xhtml` o `string`; la del validador también un
  objeto con `value`.
- Sin verificador, la de la librería da error: todo contexto que evalúa invariantes tiene que darlo.

**`gofhir/server`** (congelado, fhirpath v1.14.0) no usa `htmlChecks()`: nada que tomar.

**Cambios de fhirpath que pueden mover invariantes** (v1.14.0 a v1.16.6): `=` entre valores complejos,
`~`, el tipo de `Resource.id`/`Extension.url`, errores con argumentos de varios ítems, `memberOf()`,
el análisis estático, la lectura de un elemento de elección solo por los sufijos del modelo (v1.16.6) y
el parser SLL (v1.16.4, rendimiento). `hl7diff same` no muestra ninguno en el manifiesto.

**Cambios de ucum** (v4.3.0 a v4.3.4): Multiply y Divide en las unidades de los operandos; clave canónica
redondeada una vez; error fuera del rango de float64; anotaciones con espacios o llaves anidadas
rechazadas (UCUM §2.1: una anotación es de caracteres ASCII 33–126 sin llaves); claves de caché copiadas.
`hl7diff same` no muestra ninguno.

**Defectos de fhirpath que siguen abiertos:** el de `children()`/`descendants()` (propiedades que el
modelo no define; borrador de #180, lo publica el usuario) no lo corrige v1.16.6. Queda fuera.

## 2. Alcance

1. `go.mod`: fhirpath v1.16.6 y ucum v4.3.4.
2. `pkg/constraint/htmlchecks.go`: la lógica actual pasa a un tipo que implementa `eval.HTMLChecker`,
   que `constraint.go` da al contexto de evaluación (`evalCtx.SetHTMLChecker`). Se quita el `init` que
   registra la función global. No cambian las reglas de HTML (no es este incremento).
3. Texto del issue: ucum v4.3.4 en vez de v4.3.0 (comentario que publica el usuario).
4. CHANGELOG de v2.2.0 (lo genera release-please con el mensaje del commit; la nota va ahí): el validador ya no registra `htmlChecks()` como función global de fhirpath; un
   programa que evalúe FHIRPath por su cuenta sin verificador obtiene el error de la librería. Corregir
   `docs/VALIDATION-GAPS.md` y `docs/content/docs/error-reference/constraints*.md`, que describen la
   función propia.
5. Sondas nuevas en el manifiesto de `hl7diff`, en R4, R4B y R5 (§4).

**Fuera:** el release v2.2.0 (paso aparte, después del merge); #183; el defecto de fhirpath de
`children()`; las listas de elementos y atributos de HTML (F4: issue aparte); la divergencia de `txt-1`
con un `div` solo de espacios (F3, corregida en §10: anterior; se clasifica, no se cambia).

## 3. Criterio de salida

- `make check` en verde.
- Tests nuevos (§4) en verde en R4, R4B y R5, con sus valores esperados tomados de HL7.
- `hl7diff same` contra `548ad1f`: idéntico, o cada diferencia clasificada con evidencia. Mide que no
  hay regresión, no conformidad.
- `hl7diff run` con las sondas nuevas: PASS, con cada diferencia clasificada (las de UCUM y `txt-2`,
  con la regla de la especificación verificada campo a campo, porque HL7 no es oráculo ahí: §7).
- Barrido de claves de elección (#180): 0 diferencias; los KNOWN, iguales o menos.
- `make bench-compare BASE=548ad1f`: sin regresión significativa.
- Mutaciones de §5: todas caen.

## 4. Tests (primero, en rojo donde corresponde)

**Narrativa, contra HL7 6.10.2** (máx. 2 Java simultáneos), en R4, R4B y R5, como sondas del manifiesto
y como tests de `pkg/validator`. Cada caso cita narrative.html#xhtml. El oráculo de HL7 para un elemento o
atributo ilegal es `XHTML_XHTML_Element_Illegal` / `XHTML_XHTML_Attribute_Illegal`, no `txt-1` (F1):
el valor esperado es «un error en esa ruta», y la correspondencia de mensajes se fija antes de escribir
los tests.

1. Narrativa válida: ningún issue en `text.div`.
2. Con `<script>`: error en `text.div`.
3. Con un atributo de evento (`onclick`): error.
4. Un `div` solo con espacios: error de `txt-2` según su texto («SHALL have some non-whitespace
   content»); HL7 no lo informa (F3): diferencia clasificada como defecto de la referencia.
5. Un recurso contenido con narrativa inválida: el error en su ruta.
6. `"div": ""` (F5): según el texto, no tiene contenido; el valor esperado sale de HL7 y de `txt-2`.

**`htmlChecks()`, tests de `pkg/constraint`** (fhirpath.html#functions; sin HL7, lo dice el texto), uno
por entrada (Codex 1): el `div` tipado de Narrative, un `string` de otro elemento, un `code`, una cadena
vacía, la colección vacía y dos `div`. Lo que la librería haga distinto del texto se reporta en
gofhir/fhirpath, no se rodea.

**El verificador llega** (F6, Codex 2): un test de la API de `pkg/constraint` comprueba, con un
verificador falso, que `evaluateOn` lo da (se llama, y su `true` y su `false` se ven en el resultado); y
que un error de evaluación no se confunde con un `false`. Es el único camino que crea contextos
(`constraint.go:381-420`).

**UCUM** (F2, Codex 3): HL7 corre con `-tx n/a` y no valida la sintaxis de UCUM, y el UCUM de Java
acepta `mg{a b}` y `mg{a}{b}` (medido con `UcumEssenceService` del jar de HL7). El valor esperado sale de
UCUM §2.1 (una anotación es de caracteres ASCII 33-126 sin llaves): `mg{a b}` y `mg{a{b}}` inválidos,
`mg{ab}` válido, en una `Quantity` con `system` UCUM, en las tres versiones. La divergencia de Java ya
está clasificada en gofhir/ucum (`internal/engine/javaannotation_test.go`); se registra aquí y se
propone reportarla en `org.fhir.ucum`.

## 5. Mutaciones (antes del review)

Cada una se corre sola:

- m1: `evaluateOn` no da el verificador → falla el test del verificador y el 1 (la librería da error,
  que no es un `false`; Codex 2).
- m2: el verificador siempre responde `true` → fallan 2-4.
- m3: vuelve el `funcs.Register` global y no se da verificador → falla el test del verificador.
- m4: el verificador da error → falla 1.
- m5: `go.mod` pide ucum v4.3.0 (la mínima que exige fhirpath; v4.2.0 no se puede elegir, Codex 5) →
  se verifica con `go list -m` qué versión queda; si queda v4.3.0, fallan los tests de UCUM.
- m6: `ucumvalidator` no llama a `ucum.Validate` → fallan los tests de UCUM.

## 6. Excepciones a la regla 5, dependencias

Ninguna excepción nueva. Las reglas de HTML ya existen y no cambian, pero son listas escritas a mano sin
ADR que difieren del xpath de `txt-1` de R4/R4B (F4); R5 quita ese xpath. Queda como issue aparte con
esa evidencia. Sin dependencias nuevas: solo
suben de versión dos que ya están.

## 7. Ronda adversarial del plan (28-10-2026)

Fable y Codex, sin mi razonamiento. Codex leyó el código pero no corrió nada; las citas que importan se
verificaron.

| Hallazgo | Resolución |
|---|---|
| F1 (alta): `hl7diff same` compara dos refs nuestros; HL7 no informa `txt-1` sino `XHTML_XHTML_*` | **Aceptado.** §1 y §3 lo dicen; los tests usan los mensajes de HL7 (§4) |
| F2 (alta) y Codex 3 (alta): HL7 no es oráculo de UCUM (`-tx n/a`, `run.go:422`; el UCUM de Java acepta `mg{a b}`) | **Aceptado y verificado** (`declared-divergences.json` B-D22; `javaannotation_test.go` en ucum). El valor esperado sale de UCUM §2.1 |
| F3 (alta): `div` solo con espacios: damos `txt-1`/`txt-2` y HL7 nada (ActivityDefinition-blood-tubes-supply, -heart-valve-replacement, EventDefinition-example, Questionnaire-zika) | **Aceptado; la premisa resultó falsa** (§10): los `div` son solo espacios, pero solo se verificó eso; HL7 sí da `txt-2` en los cuatro ejemplos, y lo que sobra es nuestro `txt-1`, la divergencia de §8. Test 4 |
| F4 (media): las listas de HTML difieren del xpath de `txt-1` | **Aceptado como issue aparte;** fuera de alcance (no cambia aquí) |
| F5 (media): `"div": ""` cambia de resultado con la función de la librería | **Aceptado:** test 6 |
| F6 (baja): un solo camino crea contextos | **Aceptado:** test del verificador sobre `evaluateOn` |
| F7 (baja): R4B y R5 casi sin corpus | **Aceptado:** las sondas entran al manifiesto en los tres grupos |
| F8 (baja): CHANGELOG y documentos que describen la función propia | **Aceptado:** alcance 4 |
| Codex 1 (alta): el test 6 contradice la librería, que acepta `string` | **Aceptado:** un test por tipo de entrada; lo que difiera del texto se reporta en fhirpath |
| Codex 2 (alta): m1 puede sobrevivir, porque el error de evaluación también da `txt-1` | **Aceptado:** el test del verificador distingue `false` de error; m1 debe tumbar el test 1 |
| Codex 4 (media): las sondas no están en el manifiesto | **Aceptado:** alcance 5 |
| Codex 5 (media): m5 no puede elegir v4.2.0 | **Aceptado:** m5 pide v4.3.0 y verifica el grafo; m6 nueva |

## 8. Tests e implementación (pasos 5 y 6, 28-10-2026)

**Sondas en HL7 6.10.2** (`hl7diff run -group narrative-ucum-probes,…-r4b,…-r5`; salidas de HL7 y
nuestras en `evidence/…/probes-hl7/`):

| Sonda | HL7 (las tres versiones) | Nosotros (base y rama) |
|---|---|---|
| `nv_valid` | nada | nada |
| `nv_script`, `nv_onclick` | `txt-1` y `XHTML_XHTML_Element_Illegal`/`_Attribute_Illegal` | `txt-1` y `txt-2` |
| `nv_blank` | `txt-2` | `txt-1` y `txt-2` |
| `nv_empty` (`"div": ""`) | `txt-1`, `txt-2`, `ele-1`, valor vacío y error de parseo | `txt-1` y `txt-2` |
| `nv_contained` | `txt-1` y `XHTML_*` en `contained[0].text.div`; `CONTAINED_ORPHAN_DOM3` | `txt-1` y `txt-2` ahí; `dom-3` |
| `uq_space` (`mg{a b}`) | solo avisos (`-tx n/a`) | base: nada; rama: `UCUM_INVALID_CODE` |
| `uq_nested` (`mg{a{b}}`) | solo avisos | `UCUM_INVALID_CODE` en las dos |
| `uq_plain` (`mg{ab}`) | solo avisos | nada |

El único cambio de la rama es `uq_space`, que cumple UCUM §2.1: divergencia declarada (B-D22). La
corrida con las sondas da un solo hallazgo, ese (`run-probes-before-divergence.txt`).

**Divergencias anteriores, fuera de alcance** (la base da lo mismo): `txt-1` y `txt-2` tienen la misma
expresión, `htmlChecks()`, en R4, R4B y R5, así que un `false` hace fallar las dos; HL7 las separa por
su texto (elementos ilegales: `txt-1`; sin contenido: `txt-2`). Con la expresión publicada no hay
forma de separarlas sin escribir a mano a qué invariante pertenece cada regla. Es una limitación de la
especificación: se propone una observación a HL7 (que cada invariante tenga su expresión) y queda en el
backlog. Lo mismo con `nv_empty`, donde HL7 da además `ele-1` y el valor vacío. El test de
`pkg/validator` pide el error en la ruta, no cuál de las dos.

**`htmlChecks()` sobre un `string`** (`TestHTMLChecksInput`): fhirpath v1.16.6 responde `true` sobre un
elemento `string` y sobre un literal; R4 dice «undefined (null) on any other kind of element» y R5,
vacío. Defecto de la librería: pérdida conocida con su test, que falla cuando la librería lo corrija;
gofhir/fhirpath#248. No toca al validador: `txt-1` y `txt-2`
se evalúan en `Narrative.div`, y la función propia anterior hacía lo mismo.

**Implementación:** `narrativeHTML` implementa `eval.HTMLChecker` con las mismas reglas
(`validateXHTML`); `New` lo pone en el campo `html` y `evaluateOn` lo da a cada contexto. Se quitó el
`init` con `funcs.Register`. Documentos corregidos: `docs/VALIDATION-GAPS.md` y
`docs/content/docs/error-reference/constraints{,.es}.md`.

## 9. Validación (paso 7, 28 y 29-10-2026)

Reportes en `evidence/2026-10-28-update-fhirpath-ucum/` y `evidence/2026-10-28-2334-bench-compare-548ad1f61d1d/`.

- `make check`: verde.
- `hl7diff same` contra `548ad1f` (`hl7diff-same-548ad1f.txt`): solo cambian los tres grupos de sondas
  nuevos, en 3 líneas cada uno: `uq_space` da `UCUM_INVALID_CODE` (v4.3.3, UCUM §2.1) y el mensaje de
  `uq_nested` cambia de posición y de texto. El resto del manifiesto, igual.
- `hl7diff run` (`hl7diff-run-548ad1f.txt`): PASS en los 51 grupos livianos, con la divergencia B-D22
  de `uq_space`.
- Barrido de claves de elección (`sweep-after.txt`): 0 diferencias; KNOWN baja de 148 a 146. Las dos
  que dejan de serlo son `PlanDefinition-protocol-example` `action[0].definitionCanonical` en R4 y R4B:
  fhirpath v1.16.6 lee un elemento de elección solo por los sufijos del modelo (#227), así que `dom-3`
  ya no cuenta la clave mal escrita. 109 s y 948 MB de RSS.
- Mutaciones (`mutations.txt`, `mutations.py`): caen las 6; m5 resuelve ucum v4.3.0 (`go list -m`).
- Costo (`bench-compare`): +2,0 a +2,8 % de asignaciones y +2,1 a +3,3 % de bytes por validación; el
  tiempo no cambia de forma significativa (máquina con carga 28-37 por otras sesiones: no concluyente).
  Todo el aumento es de las dependencias: `main` con solo el `go.mod` nuevo da las mismas cifras que la
  rama (1754 contra 1706 asignaciones en `ValidatePatientWithData`). El perfil de asignaciones
  (`allocs-diff-main-vs-deps.txt`) lo pone en fhirpath: `eval.startClock` (un reloj por evaluación,
  #226), `ObjectValue.childCandidates` (#227) e `Integer.ToDecimal` en `Compare`. Son evitables en la
  librería: issue redactado (`issue-fhirpath-allocs-draft.md`), que publica el usuario. No se rodea aquí.

## 10. Code review, ronda 1 (29-10-2026)

Fable (con tests y mutaciones en su copia) y Codex (lectura; sus sospechas se verificaron aquí).

| Hallazgo | Estado |
|---|---|
| Codex 1 (media): la auditoría `TestAuditEngineExpressionGaps` (con `FPAUDIT=1`) evalúa sin verificador y anota `htmlChecks()` como hueco del motor | **Confirmado** (1 clase de falla, «none was given»). Corregido: la auditoría da un verificador propio que acepta todo; 0 clases |
| Codex 2 (media): el caso `known` de `TestHTMLChecksInput` aceptaba cualquier resultado no vacío | **Confirmado.** Corregido: solo acepta el desvío documentado (un `true`) |
| Codex 3 (media): `memberOf()` de varios ítems cambia (vacío, sin preguntar a la terminología) y nada lo cubre | **Aceptado.** `TestMemberOfSeveralItems`, en las tres versiones (R5 fhirpath.html#functions) |
| Codex 4 y Fable 1 (bajas): §1 citaba a R4 con el texto de R5 | **Corregido** |
| Fable 2 (baja): §7 F3 contradice §8: HL7 sí da `txt-2` en el `div` en blanco del corpus | **Confirmado** en `r4-core-examples` (blood-tubes-supply, heart-valve-replacement, EventDefinition-example, Questionnaire-zika: `txt-2`). F3 corregida |
| Fable 3 (baja): quitar el `init` cambia el comportamiento de un programa que evalúe FHIRPath por su cuenta | **Aceptado:** va en el mensaje del commit, para el CHANGELOG |

Fable, además: un solo camino evalúa FHIRPath (`evaluateOn`), los subcontextos de la librería copian el
verificador, `-race` sin carreras, y sus cuatro mutaciones caen. La ronda no converge: Codex dejó tres
medios, ya corregidos.

## 11. Code review, ronda 2 (29-10-2026)

Con los hallazgos de la ronda 1, lo cambiado y su diff. **Converge.** Codex: sin hallazgos. Fable
verificó con mutaciones que las cuatro correcciones tienen dientes (la auditoría sin verificador vuelve
a dar la clase «none was given»; `known` obsoleto, quitado o con otro valor hace fallar
`TestHTMLChecksInput`; un `memberOf()` de un ítem hace fallar `TestMemberOfSeveralItems`) y dejó un
hallazgo bajo: `auditHTML` había quedado entre el comentario de `TestAuditEngineExpressionGaps` y la
función. Corregido.

## 12. Cierre (paso 9, 29-10-2026)

`make check` en verde sobre el estado final. Comparaciones en verde (§9). Sin ADR nuevo: no hay
excepción nueva a la regla 5. Reportes a otras librerías: gofhir/fhirpath#248 (`htmlChecks()` sobre un
`string`, publicado); el costo de asignaciones de fhirpath v1.16.6 (redactado,
`evidence/…/issue-fhirpath-allocs-draft.md`, lo publica el usuario). Backlog: las dos invariantes de
narrativa con la misma expresión (observación a HL7); las listas de HTML sin ADR (F4, issue aparte).
El mensaje del commit lleva la nota del CHANGELOG (§2.4).
