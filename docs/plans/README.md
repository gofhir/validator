# Punto de reanudación

Se lee primero al empezar, al reanudar y después de cada compactación (carta, §10).

## Incremento activo

**Un solo evaluador de composiciones de ValueSet**, [2026-10-18-composition-evaluator.md](2026-10-18-composition-evaluator.md):
los **incrementos 1 y 2 de 3 están cerrados**. El 1 es el punto fijo en el modo cargado
([evidence/2026-10-18-composition-evaluator-inc1/](evidence/2026-10-18-composition-evaluator-inc1/)). El 2
son los excludes por evidencia de existencia, que retiran `coversSystem`, con C1, C2, C2b y C3, y C6 al
backlog B6; su code review convergió en dos rondas
([evidence/2026-10-18-composition-evaluator-inc2/](evidence/2026-10-18-composition-evaluator-inc2/)).
Próximo paso: el incremento 3 (el modo versión no cargada como un modo del evaluador), desde el paso 5.
Rama `refactor/composition-evaluator`, sin push.

## Frente activo: #196 cerrado (pasos 1-9, 02-11-2026), espera «commit y pr»; dos variantes de un elemento de elección (rama `fix/choice-two-variants`, desde `e22324e`)

[2026-11-01-held-meta-profile.md](2026-11-01-held-meta-profile.md): #188 cerrado con la evidencia del §1 y reemplazado por #196.
Plan con ronda adversarial (Opus, excepción mientras Codex no tiene cuota: 9 de 10 hallazgos aceptados). Code review convergió
en la ronda 2. `make check` verde; hl7diff 52 PASS con el grupo nuevo `choice-variant-probes`; mutaciones 11/11; barrido de #180
idéntico. **Pendiente:** el tiempo del bench con la máquina libre (las asignaciones bajan). Próximo: «commit y pr»; después, #189.

## Cerrado: #187 (mergeado como #195, `e22324e`, 01-11-2026); tipos de un elemento de elección en un slice (rama `fix/slice-choice-types`, desde `a957f2e`)

[2026-10-31-slice-choice-types.md](2026-10-31-slice-choice-types.md):
- Paso 1 (§1, revisión previa) hecho: sondas t1–t10 contra HL7 en `evidence/2026-10-31-slice-choice-types/probe/`.
- Paso 2 (§2, plan) escrito.
- Paso 3 (§3) hecho: 8 hallazgos de Codex, todos aceptados.
- Paso 4 (§4) hecho: plan aprobado, con la decisión de unificar el chequeo de extensiones.
- Sondas de §4 hechas. Plan rehecho con ellas (§5): un error por valor y por perfil; t12 sigue siendo
  «Unknown element»; Bundle y `contained` quedan fuera del alcance.
- Bundle y `contained` contra su `meta.profile`: publicado como gofhir/validator#188 (31-10-2026).
- Paso 5 (§6) hecho: tests en rojo donde corresponde. Paso 6 (§7) hecho: `make check` en verde. Paso 7 en curso: sondas regeneradas (coinciden con HL7); #188 mal diagnosticado, lo resuelve #187 (comentario de cierre redactado, lo publica el usuario). hl7diff run 51 PASS y same solo con el renombre; mutaciones 13/13 tras corregir 4 sobrevivientes (t19, perfil en el mensaje, sin chequeo de tipo en pkg/extension). Barrido de #180 idéntico a la base; bench sin cambios en asignaciones (costo de `checkType` a corregir en el paso 8). Paso 8: ronda 1 de Codex hecha (§8): 6 hallazgos, una regresión en extensiones; decisión del usuario: la fase de extensiones es dueña del tipo contra la definición de la extensión. Correcciones de la ronda 1 hechas; mutaciones 17/17; hl7diff 51 PASS (con EXTENSION_VALUE_NOT_ALLOWED en la familia extension-value-type); bench sin regresión. Rondas 2 y 3 hechas; la 3 no converge (3 hallazgos confirmados en el reparto del tipo entre las fases de extensiones y constraint, §8). Rediseño §9 descartado para #187 tras su ronda adversarial; decisión del usuario: dividir. #187 cierra sin regresión en extensiones (se quita validateOwnValue); frente nuevo redactado en evidence/…/next-front/ (lo publica el usuario). Mutaciones 18/18 y hl7diff 51 PASS en el alcance final; la ronda de Codex sobre el alcance falló por límite de uso de Codex (se reintenta después de las 21:59 con el mismo encargo, copia scratchpad/codex187r4, diff codex187-r4.diff); después, cierre. #188 no se cierra: se acota (borrador reescrito). Ronda 4 hecha (747 combinaciones sin pérdidas; ubicación bajo `_x` corregida en pkg/location). Paso 9 hecho (01-11-2026): make check verde, bench sin regresión tras quitar dos asignaciones (§Comparación final). Próximo: «commit y pr» de #183 y luego #187 (apilada sobre `a957f2e`); frente nuevo publicado como #193 y #188 acotado con un comentario (01-11-2026).

**#183, listo para PR:** `a957f2e` en `fix/choice-base-element`, sin push.

## Cerrado: #183, elemento base de ChoiceType (rama `fix/choice-base-element`, desde `970e2c1`)

[2026-10-30-choice-base-element.md](2026-10-30-choice-base-element.md): pasos 1 a 9 hechos (30-10-2026).
`baseElement` es recursivo con `childOf`. El review convergió en la ronda 2 (solo Codex). `make check`
está en verde y `hl7diff same` da IDENTICAL. El backlog está en §9. Commit `a957f2e`, sin push: falta el «commit y pr»
del usuario.

**Cerrado:** reglas de la narrativa ([2026-10-29-narrative-rules.md](2026-10-29-narrative-rules.md)), mergeado
como #185 (`970e2c1`, 30-10-2026). Borradores que publica el usuario: issue de Go de sustitutos
(`go-issue-surrogate-draft.md`) y observación a HL7 (`hl7-observation-txt-draft.md`).

**Cerrado:** #176, fhirpath v1.16.6 y ucum v4.3.4 ([2026-10-28-update-fhirpath-ucum.md](2026-10-28-update-fhirpath-ucum.md)),
mergeado como #184 (`0a0f891`); release v2.2.0 (`836a626`).

**Cerrado:** #180, clave de elección con otras mayúsculas
([2026-10-27-choice-key-case.md](2026-10-27-choice-key-case.md)), mergeado como #182 (`548ad1f`,
28-10-2026), tras 5 rondas de review. Los tres casos de resolución de la base de la ronda 5 están en
gofhir/validator#183. Backlog: issue de fhirpath de `children()` (lo publica el usuario).
HL7: tope de 2 instancias de Java simultáneas (no de corridas).

**Cerrado:** #178, contenido de los valores de una extensión
([2026-10-27-extension-value-content.md](2026-10-27-extension-value-content.md)), mergeado como #181
(`5c0001c`, 27-10-2026). Su backlog: doble informe valor/cardinalidad (con la deduplicación de #174)
y el testdata compartido entre `pkg/extension` y `pkg/validator` (F4).

**Cerrado:** #174, orden determinista de los issues ([2026-10-26-issue-order.md](2026-10-26-issue-order.md)),
mergeado como #179 (`6c0db79`, 27-10-2026). Su backlog: orden del documento o un orden final por
ubicación, y deduplicar como HL7.

**Cerrado:** #175, un elemento de elección de ElementDefinition al fusionar
([2026-10-25-snapshot-choice-merge.md](2026-10-25-snapshot-choice-merge.md)), mergeado como #177
(`a127801`, 26-10-2026). Sus pendientes fuera de alcance están al final del plan (§9, paso 9),
incluido el issue del NPE de HL7.

## Otro frente: camino caliente de la validación (rama `perf/registry-lockfree`)

[2026-10-18-validation-hot-path.md](2026-10-18-validation-hot-path.md): **incrementos A y B cerrados**
(§9 y §11). A extrae `fixed[x]`/`pattern[x]` una vez por elemento de snapshot; B publica los tipos
del registro para que `GetByType` e `IsDomainResource` no tomen el lock. Resultados idénticos a la base
(`make compare-baseline`), mutaciones A 9/9 y B 11/11, benchmarks en
[evidence/2026-10-18-1859-bench-compare-74afca6243c1/](evidence/2026-10-18-1859-bench-compare-74afca6243c1/).
Defectos previos publicados: #174 (orden de los issues) y #175 (dos `fixed[x]` en un snapshot).
Rebasada sobre `main` (`a2ac6ea`) y validada contra esa base: `make check`, `make compare-baseline`
idéntico, mutaciones 20/20, benchmarks en
[evidence/2026-10-19-0043-bench-compare-0e7b0c9416ef/](evidence/2026-10-19-0043-bench-compare-0e7b0c9416ef/).
PR [#173](https://github.com/gofhir/validator/pull/173); los pendientes de su revisión, resueltos el 25-10-2026
(lint con `go1.25.13` y sin `ignore-rules`; `hl7diff same` se mantiene, documentado frente a
`run -identical`). Próximo paso: push y merge, con el verbo del usuario; después #175, #174 y #176.
Worktree aparte (`go-validator-wt-registry`); no tocar el checkout principal.

## Último incremento cerrado

**Contexto `Element` en la raíz de un recurso** (18-10-2026): se mantiene B-D2, de
[2026-09-27-profile-false-negatives.md](2026-09-27-profile-false-negatives.md). Un contexto element
es un element id (defining-extensions.html) y Resource no deriva de Element (R4, R5), así que
`Element` no nombra la raíz de un recurso. HL7 6.10.x y Firely
(`ExtensionContextValidator.validateElementContext`) lo toman como comodín; la especificación no.
Ningún ejemplo del corpus lo usa en la raíz de un recurso. Sin cambios de código: las extensiones de
prueba `ext-coding*` de `acme.unknowncs` pasan de contexto `Element` a `Basic`, para que sus probes
(`ux_r29b_*xc*`) midan el binding y no el contexto.

Antes: severidad de un code system desconocido, PR 1 a PR 3 (#164, #166, #167),
[2026-10-16-unknown-codesystem-severity.md](2026-10-16-unknown-codesystem-severity.md).

## Próximos frentes (a elegir)

- Migrar a golangci-lint v2.14 o posterior con Go 1.27: decidir qué hacer con los 28 issues nuevos
  de goconst y gocritic. Mientras tanto, `make lint` y `make check` fijan las versiones de CI
  (Go 1.25, golangci-lint v2.11.3), con las que hay 0 issues (rama `perf/registry-lockfree`).
- Brechas anotadas: `ACR` y `umls` bajo un Coding con binding required (HL7: warning, gofhir:
  error); `memberOf` de FHIRPath sin `Coding.version`; `VALUESET_CIRCULAR_REFERENCE`, que HL7
  reporta y gofhir no; el perfil de prueba `fp-fix` de `acme.fixedpattern`, con el differential
  desordenado.

## Decisiones pendientes del usuario

- Ninguna por ahora. Después del evaluador viene el backlog B1 (la caída de versión de
  `GetCodeSystemVersion`), cuyo impacto se mide antes de decidir.
