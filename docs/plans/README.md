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

## Frente activo: #176, fhirpath v1.16.6 y ucum v4.3.4 (rama `chore/update-fhirpath-ucum`)

[2026-10-28-update-fhirpath-ucum.md](2026-10-28-update-fhirpath-ucum.md): paso 1 hecho (con solo las
dependencias nuevas, `hl7diff same` contra `548ad1f` da idéntico; el validador pisa el `htmlChecks()` de
fhirpath con una función global). Paso 2 hecho (plan §2-§6). Paso 3 hecho (plan §7: 12 hallazgos, todos aceptados; HL7 no es oráculo de UCUM ni del `div` vacío). Paso 4: aprobado el 28-10-2026. Pasos 5 y 6 hechos (plan §8): sondas de HL7 en las tres versiones, tests en verde, `htmlChecks()` por el verificador de la librería; dos defectos de otros (fhirpath responde sobre un string; txt-1/txt-2 con la misma expresión) registrados. Paso 7 hecho (plan §9): todo en verde, mutaciones 6/6; +2-3 % de asignaciones, todo de fhirpath (issue redactado). Issue gofhir/fhirpath#248 publicado. Paso 8: ronda 1 (plan §10) con 3 medios de Codex, corregidos; mutaciones 6/6 de nuevo. Ronda 2 (plan §11): converge. Paso 9 hecho (plan §12), `make check` verde. Próximo: «commit y pr» del usuario; luego release v2.2.0.
Después: release v2.2.0.

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
