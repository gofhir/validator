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

## Frente activo: reglas de la narrativa (rama `fix/narrative-rules`, desde `836a626`)

[2026-10-29-narrative-rules.md](2026-10-29-narrative-rules.md): pasos 1-4 hechos (plan aprobado el
29-10-2026: XHTML estricto con `encoding/xml`, ADR-002 con la regla de HTML 4.0). Pasos 5-7 hechos (§5-§7):
tests nuevos, `htmlchecks.go` reescrito, ADR-002, `make narrative-corpus`; sondas `nx_*` (36) contra HL7,
divergencias B-D39 y B-D40 declaradas; `hl7diff same` idéntico fuera de las sondas, `hl7diff run` PASS en
los grupos de narrativa; `make check` verde; costo medido y aceptado. Paso 8 (§8): converge en la ronda 3
(ronda 2: lectura con `RawToken` por el hallazgo X6); mutaciones 26/26; `hl7diff run` 51/51 PASS y
`hl7diff same` idéntico fuera de las sondas de narrativa. Próximo: paso 9, cierre (commit y PR cuando el
usuario lo pida). Borradores
que publica el usuario: issue de Go de sustitutos (`go-issue-surrogate-draft.md`) y observación a HL7
(`hl7-observation-txt-draft.md`).

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
