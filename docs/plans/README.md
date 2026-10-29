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

## Frente activo: #180, clave de elección con otras mayúsculas (rama `fix/choice-key-case`)

[2026-10-27-choice-key-case.md](2026-10-27-choice-key-case.md): paso 1 hecho (las sondas en R4, R4B y R5
confirman que HL7 marca la clave como desconocida y nosotros no; evidencia en
[evidence/2026-10-27-choice-key-case/](evidence/2026-10-27-choice-key-case/)). Paso 2 hecho (plan §2-§6).
Paso 3 hecho (plan §7). Paso 4: aprobado el 27-10-2026 (§8). Paso 5 en curso (plan §9): tests escritos y en rojo;
el barrido metamórfico encontró que FHIRPath navega propiedades que el modelo no define (`dom-3`, 148 claves),
anterior a #180: issue para fhirpath aprobado (lo publica el usuario), y en el barrido queda como KNOWN (plan §9).
Paso 6 hecho (plan §10): tests en verde, barrido con 0 diferencias y 148 KNOWN. Paso 7: las sondas
mostraron que falta la cardinalidad mínima de q1 (`answerOption.value[x]`); vuelta al paso 2 (plan §11).
Aprobado el 28-10-2026: en `elementvalues`, solo un tipo del elemento base es un valor (plan §11). Ya
implementado. Paso 7 (plan §12): sondas iguales a HL7, barrido 0 diferencias, `hl7diff` PASS (solo
2 falsos positivos `ref-1` menos), mutaciones 10/10 y 16/16, bench sin cambios en asignaciones, `make check` verde. Paso 8: ronda 1 con 3 hallazgos medios aceptados y corregidos (plan §13). Ronda 2 (plan §14): un defecto de diseño, porque `ChoiceType` resuelve la base por el nombre de la ruta
(un modelo lógico con el nombre de un tipo core toma sus tipos). Rediseño aprobado e implementado (plan §14.1): la base se busca por la cadena baseDefinition.
`go test ./...` en verde. Validación del rediseño en verde (plan §14.2). La ronda 3 no convergió (plan §15): la caché ya está
corregida, y queda abierto el respaldo por nombre de `base.path` (Fable 1). El usuario aprobó corregirlo y una 4.ª ronda
(plan §15.1): la búsqueda por nombre se reemplazó por el recorrido de los tipos de los ancestros. Ronda 4 (plan §16): Fable sin
hallazgos; Codex dejó 2 medios (`base.path` ambiguo), confirmados y corregidos; validación en verde.
Ronda 5 (plan §17): tres casos confirmados de la misma familia (resolución de la base: 1 medio de Codex con entrada dudosa, 2 bajos de Fable), ninguno en ejemplos ni hl7diff. Por la condición de corte van a un issue aparte (borrador listo; lo publica el usuario) y #180 pasa al paso 9. `make check` en verde al cierre (28-10-2026). Próximo paso: «commit y pr» del usuario; luego #176 y release v2.2.0.
HL7: tope de 2 instancias de Java simultáneas (no de corridas).
Después: #176 (fhirpath v1.16.6, ucum v4.3.0) y el release v2.2.0.

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
