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

## Frente activo: #193, pieza 2 (capas por valor)

- Pieza 1 mergeada como #209 (`580ec8d`, 06-11-2026): precedencia unificada con cero o un perfil
  ([2026-11-05-children-precedence.md](2026-11-05-children-precedence.md)); un perfil no resuelto se informa y se sigue
  por el tipo base. #210 (extensiones entre versiones, B-D41) mergeado en #211.
- **Pieza 2** (rama `refactor/type-profile-layers`, desde `87c0b02`): capas por valor, elector del O, alcance, memo y
  deduplicación. Base: plan [2026-11-05-type-profile-alternatives.md](2026-11-05-type-profile-alternatives.md) §2 v3 y
  hallazgos §3c (C1, C2, C5 siguen abiertos). **Paso 1 en curso (06-11-2026):**
  - línea base después de la pieza 1: con dos o más perfiles, gofhir no informa nada
    (`evidence/2026-11-05-type-profile-alternatives/probe/gofhir-87c0b02-4.0.1.json`);
  - sonda p21 (C5) hecha en R4: HL7 junta los textos idénticos y no los de cardinalidad;
  - relevamiento de llamadores hecho; C1 en parte refutado, C2 y C4 confirmados.
  - Paso 2 hecho: plan [2026-11-06-type-profile-layers.md](2026-11-06-type-profile-layers.md). Ronda 1 (Codex): 4 altos y 2 medios, aceptados (A2 en parte); plan §2 v2. Ronda 2: 1 alto (B1, Scope del destino de una referencia, defecto de `main`) y 1 medio, aceptados; plan §2 v3. Ronda 3 (la última): **no converge** (C1 alto: Scopes distintos para un mismo map; C2 medio: `holderGroups` en corridas especulativas; los dos, defectos de `main`; plan §3c). Usuario: pieza previa (rama `fix/canonical-scope`, plan `2026-11-06-canonical-scope.md`): un solo constructor de Scope (C1, B1) y `holderGroups` por corrida especulativa (C2). Paso 1 en curso: sondas sc1–sc7 hechas; B1 no se observa en gofhir (los dos referentes fallan en los dos órdenes); HL7 informa un destino que no cumple una vez y gofhir en cada referente (pendiente aparte). C1 y C2 observables (sc8–sc14: resultado según el orden y distinto de HL7). Plan hecho (pasos 1 y 2): Scope del destino por su posición, corridas especulativas con `holderGroups` y `slicerecord` propios, test del invariante «un map, un Scope». Ronda 1: 2 altos y 3 medios, aceptados; plan §2 v2 (índice de posiciones por validación). Ronda 2: 1 alto y 2 medios, aceptados; plan §2 v3. Ronda 3: no converge por 1 medio (F1: qué hace producción si un recurso falta en el índice); propuesta §2 v4. Plan v4 aprobado (07-11-2026). Sondas hechas (plan §5): sc8–sc14 iguales en las tres versiones; D1 y D3 observables; D5 no, y el `slicerecord` nuevo sale del alcance. Implementado (pasos 5–6, 07-11-2026): `internal/scopeindex`, tests fallan antes (40) y pasan después, `make check` verde. hl7diff verde (61 livianos y r4-core-examples); bench sin regresión de tiempo, +0,3–1 % asignaciones (el índice). Mutaciones: 9 de 14 muertas; 6 vivas, probablemente consultas redundantes al índice (plan §7). Code review ronda 1: 1 alto, 2 medios y 1 bajo, aceptados; las consultas 4, 12 y 13 se quitan por redundantes, y la 2, la 3 y la 6 se mantienen con tests (plan §8). Correcciones hechas, `make check` verde; consulta 3 revertida por HL7 (r1b); inconsistencia de HL7 entre la validación de referencias y `resolve()` (r2, observación por redactar). Mutaciones: 14 de 16 (4 equivalente; 16, `place`, sin test). Ronda 2: 1 alto (invariante roto con `entry.resolve()` en `valueScope`) y 1 medio (Bundle raíz suelto con `External`), aceptados (plan §8). Medio corregido. El alto choca con HL7 (r3a1): el invariante «un map, un Scope» contradice a HL7, que comprueba `resolve()` con el Scope de la cadena (plan §8, «Decisión pendiente»). Usuario (08-11): seguir a HL7, invariante por camino. Invariante por camino, oráculo endurecido (mutaciones 18 y 20 muertas), `place` quitado por redundante, observación a HL7 redactada; hl7diff verde. Ronda 3 NO converge: 1 alto (el memo de `Conforms` no incluye el Scope) y 1 medio (la marca de cadena se activa al cruzar recursos que contiene un elemento), más 3 bajos (plan §8). Usuario (09-11): corregir y cerrar, sin ronda 4. La evidencia de r3 era un artefacto (perfil que corta un elemento 0..1). Con r4, HL7 comprueba `resolve()` con el Scope canónico (plan §8). Usuario (09-11): volver al invariante global. Correcciones finales hechas (invariante global, r4 con HL7, nota a HL7 «no enviar»), `make check` verde. Cerrada (09-11): 20 de 20 mutaciones muertas, hl7diff verde, bench sin regresión de tiempo, `make check` verde (plan §8). Mergeada como #215 (`16bc506`, 09-11-2026). Pieza 2: rama `refactor/type-profile-layers` desde `16bc506`; plan §2 v4 escrito y sondas p1–p21 re-verificadas en HL7 (sin errores de snapshot). Ronda 1 de la serie nueva: 2 altos y 4 medios, todos aceptados (D6 fuera del alcance); plan §2 v5. Ronda 2: 2 altos, 1 medio y 1 bajo, aceptados; plan §2 v6. Ronda 3: queda 1 medio (F1, supresión compartida entre capas), con la propuesta v7 en §3f. Plan aprobado por el usuario (v7, 14-11). Sondas hechas (155, 3 versiones, plan §5). E2: HL7 usa el propio tipo de datos como `%resource` al decidir discriminadores (defecto de HL7 según la especificación); E3: la identidad entre capas pasa a ser clave, expresión y severidad. Usuario (14-11): E2 sigue la especificación (Scope del dueño; divergencia declarada; issue a HL7). Tests escritos: 279 subtests fallan antes de implementar y 522 pasan (regresión y guardas). Implementado: de 279 fallos quedan 12 (4 casos, plan §5, «Después de la implementación»). kctx, p14/p15 y bcyc3 resueltos. D5 NO cumple la cota: el recorrido por capas es exponencial en la profundidad cuando se eligen todas las capas. Usuario (14-11): aprueba §2 v8 (un descenso por valor hijo). v8 implementado, `make check` verde; visitas de 130.800 a 2.696 con D=12 (cuadrático por las comprobaciones especulativas anidadas; un solo perfil igual a main). hl7diff verde; mutaciones: 32 de 38 muertas, el resto equivalentes; defecto de `Discover` corregido; bench sin regresión. Code review ronda 1: 3 medios aceptados (plan §8). Pieza 2 cerrada (plan §9), mergeada como #218 (`0fbaf27`, 17-11-2026). **Pieza 3 (primitivos), rama `refactor/primitive-type-profiles`: paso 6, implementación** ([plan](2026-11-15-primitive-type-profiles.md) v3, aprobado; tests §5: 527 de 1092 fallan antes de implementar; `.value` va en la pieza 3b).
- **Pieza 3, primitivos: cerrada el 16-11-2026, mergeada como #219 (`ba63bba`, 17-11-2026)** (rama `refactor/primitive-type-profiles`). El registro
  está en el plan, §7 y §8.
  - La ronda 3 no convergió: dejó T1 (primitivo sin valor) y T2 (perfil no resuelto), confirmados con c16. El usuario
    decidió corregir y cerrar, sin ronda 4.
  - Una rama sin evidencia de la corrección se quitó por c17, medido en HL7.
  - La severidad en `reportKey` es una excepción a C1, aprobada por el usuario.
  - Mutaciones de la corrección: 8 de 10 muertas. Viven T2d (sin sonda) y T1c2 (equivalente por razonamiento).
  - hl7diff: 0 hallazgos en los 61 grupos livianos y en los 5.301 ejemplos del core.
  - Bench sin regresión de tiempo, con asignaciones -1 %. `make check` en verde.
  - **Incremento activo: límites del valor** (`maxLength`, `minValue[x]` y `maxValue[x]` en cualquier elemento; hoy no se
    aplican en ninguno), rama `fix/element-value-limits`: paso 1 en curso
    ([revisión previa](evidence/2026-11-17-element-value-limits/prior-review.md)). Usuario (17-11): separar; la pieza 3b
    (`<type>.value`) va después y reusa estas comprobaciones. HL7 7.0.4 aplica las tres en R4, R4B y R5. Sondas por tipo hechas
    (65 instancias, iguales en las tres versiones). Defectos de HL7: `maxValue` de date, dateTime e instant nunca falla;
    `maxLength` no se comprueba en id, markdown ni base64Binary. Plan v1 escrito ([plan](2026-11-17-element-value-limits.md)); sondas ampliadas a 83
    (Duration, capas de slice, arreglos, elección). Paso 3: ronda adversarial 1 sin hacer: Codex llegó a su límite de uso (se puede reintentar el 19-11-2026, 14:10);
    ronda 1 hecha con un revisor Opus independiente (18-11-2026): 5 altos, 6 medios y 2 bajos, aceptados (A1 con alcance menor); plan §3, v2. Usuario (18-11): D1 a D4 como se recomendó. Ronda 2 (revisor Opus): no converge, 4 altos y 7 medios confirmados (plan §4). Causa común: el recorrido no baja por todas las definiciones que gobiernan un valor, y eso ya afecta hoy a `fixed[x]` y `pattern[x]`. Usuario (18-11): pieza previa, y este incremento queda en pausa (queda 1 ronda). **Incremento activo: descenso por todas las definiciones (resto de #193)**, rama `refactor/definition-layers-descent`, pasos 1 y 2 hechos ([revisión previa](evidence/2026-11-18-definition-layers-descent/prior-review.md), [plan v1](2026-11-18-definition-layers-descent.md)). Usuario (18-11): binding en la pieza siguiente. Tres rondas del plan (revisor Opus) sin converger: el usuario eligió la opción A: prototipo acotado hecho (plan §6); plan v4 aprobado (19-11-2026). **Paso 5 hecho** (plan §7): `definition_layers_test.go` con 308 sondas × 3 versiones, esperados de HL7 (`tests/run_hl7.sh`, `mk_testdata.sh`); 543 subtests fallan antes de implementar. Gancho `internal/walkprobe` (sin emisores aún). **Paso 6 hecho (25-11-2026, plan §8–§9): `TestDescent*` en verde. Paso 7 hecho (25-11-2026, plan §10–§12): los defectos A (texto «(defined in …)») y B (reslice) y tres más del texto se corrigieron con tests de HL7; sin cambios de veredicto en los datos reales; el texto distinto coincide con HL7; costo +0,5 % de tiempo y RSS igual; mutaciones 34/34, y en el código final sobrevive la 28; `make check` en verde. Paso 8, ronda 1 (25-11-2026, revisor Opus independiente): F1–F4 confirmados por Opus con HL7 (texto «(defined in …)» en hijos de un perfil de tipo, en la definición propia de una extensión y con un perfil explícito; slicing sin discriminador). Usuario (25-11): F3, seguir a HL7 (sin discriminador no se asigna ningún slice, `sliceMatches`). Correcciones hechas (plan §13; `make check` verde, mutaciones F, L, B, P, E y R1–R4 muertas, D y K equivalentes); Ronda 2 (revisor Opus): F1–F6 corregidos; nuevo R2-F1 (medio, confirmado por Opus: con un perfil explícito, una constraint con errata en la raíz de un perfil de tipo nombra el perfil equivocado, porque el sello va antes de `correctSnapshot`). Mutaciones antes de la ronda 2: 50/55; sobreviven 10 (otra vez), 28 y K (equivalentes en los paquetes oficiales). Correcciones de la ronda 2 hechas (plan §14): R2-F1 (el sello va después de `correctSnapshot`; siete constraints con errata, en R4 y R4B), mapa de perfiles explícitos perezoso y, por decisión del usuario (25-11), las capas antes que los hijos propios, como HL7 (sondas m10b y m10c; punto 5 corregido). `make check` verde. Ronda 3 (25-11, revisor Opus): converge (plan §15); R2-F1 corregido; sin hallazgos medios nuevos. Pendientes anteriores a la rama: R3-P1 (falso aceptado: se pierde la constraint de un slice cuyo elemento dividido declara un perfil de tipo, generador de snapshots) y R3-P2 (versión cruzada R4/R4B, `ref-1`). Mutaciones completas: 52/57 mueren, sobreviven 28 y K (equivalentes) y 10r, divergencia de texto con HL7. Usuario (26-11): se cierra y 10r pasa al próximo incremento. **Paso 9 hecho (26-11-2026, plan §16): `make check` verde; incremento cerrado, sin commit.** Próximo: commit y PR si el usuario los pide; después, un incremento con 10r, R3-P1 y R3-P2, y los cuatro defectos de mhd en orden (scopeindex, `fullUrl`, #221, #222). **Próximo incremento, paso 1 hecho (27-11-2026):** borrador en el scratchpad (`p1/prior-review.md`), pasa al repo con el plan. R3-P2: corrección de errata con la versión de la base, probada en una copia. R3-P1 y 10r: choque de claves de invariantes; R3-P1 fuera de alcance por decisión del usuario (perfil inválido para HL7). Sigue el paso 2 (plan) tras el commit de la rama actual. Pendientes: S2, S5 y el aviso informativo de HL7 en un slicing abierto (gofhir nunca lo emite).** La pieza siguiente junta binding y cardinalidad. Decisiones: D1 (tipos de maxLength), D2 (conteo), D3 (comparador), D4 (fecha parcial). Después, el incremento 2 (s11, s12) y s16.
- Cola de issues: #199, #200, #202, #205, #206, #207, #208 (array vacío de un primitivo en R5: reemplaza ADR-003; HL7
  lo aceptó en hapifhir/org.hl7.fhir.core#2825). Pendiente: issue y PR a HL7 del defecto de varios perfiles en un tipo
  de elección (solo el primero, `InstanceValidator.java:8072`, `:8085`).

## Cerrado: #192, un Coding sin system en un CodeableConcept no pertenece al value set, mergeado como #204 (`700d1c3`, 05-11-2026)

[2026-11-05-coding-no-system-binding.md](2026-11-05-coding-no-system-binding.md),
[evidencia](evidence/2026-11-05-coding-no-system/).
- Paso 1 hecho (05-11-2026): HL7 7.0.4 nunca da por miembro a un Coding sin system de un CodeableConcept (required: error;
  extensible: warning), en R4, R4B y R5; gofhir lo acepta si el código coincide (`codingInValueSet`). Coding suelto:
  coinciden.
- Pasos 2 y 3 hechos: Codex dejó 5 medios y 1 bajo (plan §3); A2 y A5 medidos en HL7 (q11–q16).
- Paso 4 hecho: aprobado (05-11-2026). Sondas A1/A2 hechas (plan §5): HL7 rechaza en perfil, slice y extensión; con un
  value set inexistente no rechaza. Regla: `Invalid` solo si `ResolvesValueSet`.
- Pasos 5-7 hechos (plan §6-7): tests fallan antes y pasan después; mutaciones 6/6; corpus: solo se agregan 9 errores y
  188 warnings de los dos IDs, todos con par en HL7; `make check` verde. Paso 8: ronda 1 con 1 medio y 2 bajos, ronda 2 converge (2 bajos, corregidos); mutaciones 8/8. Paso 9 hecho: corpus y
  `make check` en verde (plan §9). Sin commit.
- Pendiente aparte: falso positivo `STRUCTURE_TYPE_NOT_ALLOWED` en `ov_e-two.json`, regresión posterior a v2.2.0 (plan §7).
- Pendiente aparte: value set inexistente, gofhir `information` vs HL7 warning `Terminology_TX_ValueSet_NotFound` (issue).
- Mergeado como #204 (`700d1c3`). Issues del cierre: #205, #206, #207 (plan §9).

## Cerrado: #191, IssueType `code-invalid` para un código que su CodeSystem no define, mergeado como #203 (`dd3ea6c`, 05-11-2026)

[2026-11-05-code-invalid-issue-type.md](2026-11-05-code-invalid-issue-type.md),
[evidencia](evidence/2026-11-05-code-invalid/).
- Paso 1 hecho (05-11-2026): HL7 7.0.4 da `code-invalid` para `CODE_NOT_IN_CODESYSTEM` y, con servidor, para un UCUM
  inválido (`UCUM_INVALID_CODE`), en R4, R4B y R5. hl7diff no compara IssueType: unas 100 combinaciones difieren (frente
  aparte, fuera del alcance).
- Pasos 2 y 3 hechos: plan escrito; ronda adversarial de Codex con 4 hallazgos (3 medios), registrados en el plan §3.
- Pasos 4-7 hechos: aprobado; tests y fix; `hl7diff same -expect` IDENTICAL (92 líneas reescritas); mutaciones 7/7;
  `make check` verde (plan §5-7).
- Pasos 8 y 9 hechos (05-11-2026): Codex convergió en la ronda 2 (ronda 1: un medio en `-expect`, corregido); mutaciones
  10/10; `same -expect` IDENTICAL; `make check` verde (plan §8-9). Sin commit.
- Frente aparte publicado como [#202](https://github.com/gofhir/validator/issues/202) (IssueType frente a HL7; hl7diff no lo
  compara).
- Mergeado como #203 (`dd3ea6c`).

## Cerrado: #190, mergeado como #201 (`169de67`, 05-11-2026), con D4 revisado por la referencia HL7 7.0.4; un array JSON vacío se reporta según el json.html de cada release

[2026-11-02-empty-array.md](2026-11-02-empty-array.md), [ADR-003](../adr/ADR-003-empty-primitive-arrays-by-release.md).
- Revisiones:
  - ronda adversarial (Opus): 11 aceptados;
  - code review: converge en la ronda 2.
- Verificación:
  - `make check` verde;
  - hl7diff 58 PASS;
  - mutaciones 15/15;
  - asignaciones iguales.
- **Reapertura (03-11-2026), commiteada en #201 (05-11-2026):** la referencia HL7 era la 6.10.2; ahora es la 7.0.4
  (`~/tools/validator_cli.jar` → 7.0.4; la 6.10.2 queda como `validator_cli-6.10.2.jar`). La 7.0.4 reporta ele-1
  en el objeto que contiene el array vacío. Con eso:
  - D4 revisado: el holder ya no se registra; plan §10;
  - hl7diff con la 7.0.4: 58 PASS; `same` sin cambios fuera de los grupos nuevos y de `Patient.link`;
  - mutaciones 15/15; `make check` verde; revisión de Opus: converge.
  - Mergeado como #201 (`169de67`, 05-11-2026).
- **Pendiente del usuario**, borradores en
  [evidence/2026-11-02-empty-array/drafts/](evidence/2026-11-02-empty-array/drafts/):
  - ~~comentario de corrección en #190~~ (erróneo, ver arriba);
  - issue publicado: [hapifhir/org.hl7.fhir.core#2824](https://github.com/hapifhir/org.hl7.fhir.core/issues/2824) (04-11-2026; relacionado #2806). El fix y los
    casos están publicados (04-11-2026): PR [hapifhir/org.hl7.fhir.core#2825](https://github.com/hapifhir/org.hl7.fhir.core/pull/2825) y [FHIR/fhir-test-cases#337](https://github.com/FHIR/fhir-test-cases/pull/337):
    - fix en `/Volumes/dev/hl7/core-ea`, rama `fix-json-empty-primitive-array` (`547376cfb`), en las copias de services y r5 de
      `JsonParser` (el validador 7.x usa la de services); revisión independiente aplicada;
    - casos en `/Volumes/dev/hl7/fhir-test-cases-ea`, rama `fix-json-empty-primitive-array-cases` (`e96eed9`), 8 casos;
    - cuerpos de PR en `/Volumes/dev/hl7/logs/pr-empty-array-*.md`;
    - `ValidationTests` completo: 1153 tests; solo cambian los 6 casos nuevos de R3/R4/R4B, que fallan en master y
      pasan con el fix. Las 4 fallas de supplements son previas a este cambio.
  - issue de paths bajo `_x`.
  - NPE de HL7 en `validateValueSetExpansion`: issue publicado [hapifhir/org.hl7.fhir.core#2828](https://github.com/hapifhir/org.hl7.fhir.core/issues/2828) (05-11-2026); borrador en
    [evidence/2026-11-04-hl7-expansion-npe/drafts/](evidence/2026-11-04-hl7-expansion-npe/drafts/); estado en
    [2026-09-28-hl7-validator-npe-upstream-fix.md](2026-09-28-hl7-validator-npe-upstream-fix.md). Fix en curso en
    `/Volumes/dev/hl7/core-npe` (rama `fix-expansion-npe`, commit `883a25a16`, en el fork): tests 4/4, mutaciones 4/4, lote R4 sin caída, Codex convergió en la ronda 1 (solo hallazgos bajos, corregidos); PR [hapifhir/org.hl7.fhir.core#2829](https://github.com/hapifhir/org.hl7.fhir.core/pull/2829) abierto (05-11-2026); tras una release con el fix, quitar las 4 exclusiones del corpus.

Después: #191.

## Cerrado: #189 (PR #198, `68589e6`): primitivos bajo una contentReference

[2026-11-02-content-reference-primitives.md](2026-11-02-content-reference-primitives.md). Issues nuevos: #199 (`_x`)
y #200 (estructural con `url#id`). **Pendiente:** el tiempo del bench con la máquina libre, antes del release.

## Cerrado: #196 (PR #197, `8730fc4`): dos variantes de un elemento de elección (rama `fix/choice-two-variants`, desde `e22324e`)

[2026-11-01-held-meta-profile.md](2026-11-01-held-meta-profile.md): #188 cerrado con la evidencia del §1 y reemplazado por #196.
Plan con ronda adversarial (Opus, excepción mientras Codex no tiene cuota: 9 de 10 hallazgos aceptados). Code review convergió
en la ronda 2. `make check` verde; hl7diff 52 PASS con el grupo nuevo `choice-variant-probes`; mutaciones 11/11; barrido de #180
idéntico. **Pendiente:** el tiempo del bench con la máquina libre, antes del release (las asignaciones bajan).

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

- **Divergencias de la puerta de mhd** (24-11-2026, [evidencia](evidence/2026-11-24-mhd-gate-divergences/README.md)): 242 en
  19 grupos contra HL7 7.0.4. Dos están reproducidas en `ba63bba`: un error interno de `scopeindex` con `resourceType` numérico, y
  una entrada sin `fullUrl` que gofhir acepta. Falta triar el resto y revisar `key()` del harness de mhd. El valueCode en
  `value[x] only string` de MHD#288 ya estaba corregido por #195 ([evidencia](evidence/2026-11-24-choice-type-code-as-string/README.md)).
- **#221** (25-11-2026, desde mhd): las claves JSON duplicadas no se reportan. HL7 7.0.6 da error («is a duplicate and will be
  ignored»); `encoding/json` se queda con el último valor. **#222:** gofhir acepta UTF-8 inválido y surrogates sueltos. HL7 también
  los acepta, así que es una brecha contra la especificación (RFC 8259 §8.1), no una divergencia. Las dos reproducidas en
  `ba63bba` con el CLI (0 errores). Al corregirlas, avisarle a la sesión de mhd para que repita la puerta del 011.

## Decisiones pendientes del usuario

- Ninguna por ahora. Después del evaluador viene el backlog B1 (la caída de versión de
  `GetCodeSystemVersion`), cuyo impacto se mide antes de decidir.
