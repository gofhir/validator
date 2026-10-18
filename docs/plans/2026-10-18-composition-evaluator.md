# Plan: un solo evaluador de composiciones de ValueSet

Estado: incremento 1 de 3 cerrado, 18-10-2026; sigue el incremento 2. Base: `main` en eabdce4.

## Objetivo

Hoy hay dos evaluadores de composición en `pkg/terminology`:

- `membership.go` decide si un código está en un ValueSet con las versiones cargadas;
- `version_membership.go` (PR 3, #167) decide lo mismo para un `Coding.version` que no está cargado.

Repiten el álgebra (unión de includes, intersección de las partes de una cláusula, excludes, value
sets anidados, ciclos, memo) con reglas que ya divergen. Por ejemplo, un ciclo da `unknown` en
`membership.go` pero se decide como punto fijo en `version_membership.go`, y `lockedDate` solo lo
mira el segundo. El objetivo es **un solo evaluador**, donde la versión del Coding no cargada sea un
modo de la parte de sistema de una cláusula, y **ningún cambio de resultado con las versiones
cargadas**, salvo los que este plan nombra y justifica.

## Revisión previa (paso 1)

| Fuente | Qué hace | Decisión |
| --- | --- | --- |
| Especificación R4/R4B/R5, valueset.html#compositions y valueset-definitions.html (`compose.include`, `.version`, `.valueSet`, `.exclude`, `lockedDate`) | Unión de includes menos excludes; la parte de sistema y los value sets de una cláusula se intersectan; `version` fija la edición del sistema (`*`, todas); `lockedDate` fija la versión de lo que no la nombra | Fuente de las reglas |
| `membership.go` | Recursión con `deciding`/`decided` (un ciclo es `unknown` y queda memoizado) y tope de profundidad 16. La parte de sistema expande con `clauseCodes`: el CodeSystem cargado, con caída a otra versión si la pedida no está, marcando la expansión como incompleta. Un exclude del sistema completo se detecta aparte (`coversSystem`) | **Mejorar**: el álgebra pasa al evaluador común; la expansión por CodeSystem cargado (`inSystem`, `clauseCodes`) se **toma** tal cual |
| `version_membership.go` | Punto fijo monótono con lista de trabajo, por (url, exists, locked); dentro de un exclude se supone que el código existe; `systemClause` decide por enumeración, versión, `*` y `lockedDate` | **Tomar**: el punto fijo y el supuesto de existencia pasan a ser el mecanismo común; `systemClause` queda como la parte de sistema del modo "versión no cargada" |
| `gofhir/server` (congelado), `internal/terminology/valueset/compiler.go` | Expande la composición a un conjunto de códigos (necesita el CodeSystem entero); `include.version` fija la edición; no conoce versiones de Coding no cargadas, `lockedDate` ni ciclos | **Descartar** como modelo; se toma solo la idea de que `include.version` fija la edición, que `clauseCodes` ya aplica |
| HL7 6.10.2 (`ValueSetValidator`), Firely (`LocalTerminologyService`) | HL7 expande y valida contra la versión cargada; Firely ignora `Coding.version` | Referencias de comparación, no de diseño (ver PR 3) |

Medido en 11.815 ValueSets de core R4, THO r4 7.4.0, extensions r4, US Core 6.1.0, VSAC 0.22.0 e
IPS 2.0.0:

- **`include.version`:** 916 ValueSets. La caída a la versión cargada en `clauseCodes` tiene que
  seguir igual: el modo "versión no cargada" lo dispara solo la versión **del Coding**, nunca la del
  include.
- **`lockedDate`:** 1 ValueSet (`example-extensional`, core R4), con todos sus includes versionados,
  así que no le afecta.
- **Ciclos:** ninguno (ronda 8 del PR 3).

## Diseño (ajustado tras las dos rondas adversariales)

Tres incrementos, en este orden. Cada uno se entrega por separado, con su diferencial y sus cambios
nombrados.

### Incremento 1: punto fijo en el modo cargado

- `membership.of` y `composition` pasan al punto fijo monótono por (`*ValueSet` resuelto,
  existencia, lock), en lugar de `deciding`/`decided` y el tope de profundidad 16 (H3, H10).
- `coversSystem`, `includesSystem` e `includesWholeSystem` **se mantienen**: retirarlos antes del
  incremento 2 cambiaría un caso acíclico que hoy da Out (Codex #5).
- **Cambios nombrados:**
  - el resultado en un ciclo ya no depende del orden (`r1`/`r3`, H3);
  - una cadena de más de 16 ValueSets se decide.

  No hay casos así en el corpus.

### Incremento 2: excludes por evidencia de existencia

- Cada cláusula se evalúa por sistema **y edición**. La edición es la que nombra la cláusula: su
  `version`, `*` para todas, o ninguna, en cuyo caso es la cargada o la que fija `lockedDate`.
- La evidencia sobre el código tiene tres valores: **positiva**, **negativa** o **ausente** (Codex
  #3).
  - Un código que la edición lista es evidencia **positiva**, también en un `fragment` (el código
    existe). Que no lo liste es evidencia **negativa** solo si la edición está cargada y completa. Un
    `supplement` (que no trae los códigos de su sistema), un `fragment` o `example` que no lo lista, un
    `not-present`, un sistema externo o una edición que no está cargada dan evidencia **ausente**.
- Un exclude quita el código cuando lo selecciona con evidencia positiva, o cuando la evidencia está
  ausente y el exclude toma la edición entera (el supuesto de existencia).
- Esto reemplaza a `coversSystem`, `includesSystem` e `includesWholeSystem`.
- **C3** (include `X concept [z]`, X cargado y completo sin `z`): el algoritmo de expansión exige
  comprobar que los códigos enumerados son válidos (valueset.html#expansion), y `z` no lo es. Pasa de
  In a **Out**. Es un cambio nombrado, con su fila.
- **C6** (excludes por edición): por la regla de evidencia, un exclude de una edición cargada y
  completa que no tiene el código ya no lo quita. Por ejemplo, X\|2 entero sin `a` no quita `a`. Lo
  que queda de C6 se va al backlog **B6**: la pertenencia por edición cuando se mezclan cláusulas con y
  sin versión, que la especificación no define. Medición: 0 excludes con una versión distinta a la del
  include en los 8 paquetes medidos (decisión del usuario, 18-10-2026).
- **C1, C2 y C2b:** pasan de Unknown a Out. Son cambios nombrados.
- **Impacto medido de C3:** 2 ValueSets de THO 7.4.0 enumeran códigos que su CodeSystem cargado y
  completo no tiene. `image-reference-type` enumera DICOM `130401`, `130488` y `130489`;
  `citation-artifact-classifier` enumera `cds-artifact`, `dataset-unpublished` e
  `interactive-form`. Esos códigos dejan de ser miembros.

### Incremento 3: el modo versión no cargada como un modo del evaluador

- `VersionMembership` pasa a ser un envoltorio del evaluador común, con la misma firma.
  `systemClause` queda como la parte de sistema cuando la versión **del Coding** no está cargada.
  Una versión de `include.version` que no está cargada sigue siendo la del modo cargado, con su caída
  (ver el backlog B1).
- Decisiones de H4:
  - **C4:** un include de otro sistema con `modifierExtension` es Out en los dos modos.
  - **C5:** un include vacío es Out.
  - **ValueSet anidado que solo tiene expansión:** `expansionCodes` en el modo cargado, Unresolved en
    el modo versión.
- `answerCache` agrega a su clave la versión no cargada (H10).
- `lockedDate` deja incompleta, en el modo cargado, una cláusula sin versión (H6). La versión de los
  ValueSets anidados bajo `lockedDate` queda en el backlog (B2).

## Backlog de defectos que la unificación expone pero no crea (cada uno, un incremento propio para corregirlo)

- **B1 (Codex #1):** la caída de `GetCodeSystemVersion` toma como ciertos los positivos de otra
  edición. Por la especificación, otra edición no prueba pertenencia. De esa caída dependen los
  ValueSets v2 que piden 2.0.0 cuando THO trae 3.0.0. Se mide su impacto y se decide con el usuario.
  Es el **próximo incremento**.
- **B2 (Codex #2):** `lockedDate` fija también la versión de los ValueSets anidados.
- **B3 (Codex #6):** el caché de negativos (`unresolved`) no se invalida con `SetAuthority` ni
  `SetProvider`.
- **B4 (Codex #9):** en R5, una expansión paginada con `next` y sin `total` da Out para un código de
  otra página.
- **B6:** la pertenencia por edición cuando se mezclan cláusulas con y sin versión (lo que queda de
  C6).
- **B5 (Codex #8, H8):** ADR para `externalSystems`, cuyo efecto es mayor que una copia `complete`: un
  `fragment` de un URI ordinario sí se consulta.

## Fuera de alcance

- `systemsOf`, `collectSystems` y su tope de profundidad.
- `expansionCodes` para ValueSets sin composición, salvo la decisión del incremento 3.
- La evaluación de filtros.
- `memberOf` de FHIRPath con `Coding.version`.
- `VALUESET_CIRCULAR_REFERENCE`.
- `compose.inactive` (H11: `inactive=false` aparece en `ihe.formatcode.fhir#1.4.0`, se mide en su
  propio incremento).

## Criterio de salida (por incremento)

- **Incremento 1:** salida idéntica a `main` en todos los corpus, salvo los cambios nombrados, que no
  aparecen en el corpus. El recorrido de la composición ya no tiene tope de profundidad ni `decided`;
  `deciding` queda como la marca de "en curso" del recorrido en profundidad. Los cambios nombrados
  valen también para un elemento `code`: `collectSystems` toma los sistemas de un ciclo como la unión
  y no tiene tope (ronda 1, hallazgo 1).
- **Incremento 2:** idéntica a `main`, salvo C1, C2, C2b, C3 y la parte de C6 que decide la
  evidencia, cada uno con su fila. El diferencial muestra, como cambios esperados, solo los de los dos
  ValueSets de C3. Ya no quedan `coversSystem`, `includesSystem` ni `includesWholeSystem`.
- **Incremento 3:** los mismos resultados que el PR 3 en el modo versión no cargada, y un solo
  recorrido de la composición. Ya no queda el `versionWalk` aparte.
- En los tres: `go test -race`, `go vet` y lint en verde, y benchmarks de tiempo y asignaciones en
  caché fría (`BenchmarkMembershipNested`), caliente (`...Warm`) y con un ciclo
  (`BenchmarkMembershipCyclicDiamond`), sin regresión medible frente a `main` (Codex #10). Contar las
  aristas recorridas exigiría instrumentar el código de producción: se descarta, y el benchmark del
  ciclo mide el costo del punto fijo.

## Tests (paso 5) y datos (paso 7)

- **Tabla de álgebra:** cada fila cita su sección de valueset.html o, si es diseño del proyecto (la
  lógica de tres valores, la existencia), lo dice.
  - Las filas cubren: unión, intersección, varios `valueSet` en una cláusula, exclude directo y por
    ValueSet, sistema completo, edición (C6), `*`, `lockedDate`, `modifierExtension` (C4), include
    vacío (C5), `supplement`, ciclo en los dos órdenes (metamórfico) y diamante.
  - Los valores esperados salen del álgebra, no del código.
- **Corrida en tres versiones:** ValueSets reales de los core r4, r4b y r5 (H7).
- **Consumidores:** una fila para `slicematch` en modo estricto y una para `memberOf` de FHIRPath (H9).
- **Oráculo diferencial:** un comando del repo que corre corpusrun de HEAD y de `main` sobre los
  ejemplos del core R4 y siete grupos de IG, archivo por archivo (severidad, messageId, expression y
  diagnóstico), más los 49 grupos de hl7diff.
  - Publica cuántas veces se pasa por cada rama cambiada (H2).
- **Mutaciones:**
  - **Del evaluador:** invertir el exclude; quitar la intersección; quitar el supuesto de existencia;
    quitar la caída de `clauseCodes`; quitar `locked` o la existencia de la clave; perder el
    reencolado de dependientes; romper la clave por versión; quitar la protección de `derivedGen`.
    Cada una tiene que hacer fallar un **test**.
  - **Del arnés del diferencial (Codex #7):** omitir un archivo, duplicar un issue, cambiar una
    severidad, una expression o un diagnóstico. Cada una tiene que hacer fallar el **diferencial**.

## Excepciones a la regla 5

Ninguna nueva. Existentes y listadas: `filterOps`, semántica de prosa (valueset-filter-operator), y
`externalSystems`, con su ADR pendiente en el backlog B5.

## Dependencias nuevas

Ninguna.

## Ronda adversarial (paso 3)

Revisor independiente, 18-10-2026. Sondas en el scratchpad (`adv-eval/`).

| # | Hallazgo | Resolución |
| --- | --- | --- |
| H1 | El supuesto de existencia en los excludes cambia resultados en el modo cargado. C1, C2 y C2b pasan de Unknown a Out. C3 (include `X concept [z]`, X `complete` sin `z`, exclude un VS = X entero menos `b`) pasa de In a Out | **Aceptado.** El supuesto de existencia se aplica solo donde la existencia del código **no se conoce**: un CodeSystem que no está cargado, uno parcial o externo, o la versión no cargada. Donde el CodeSystem está cargado y completo, la existencia la dice el CodeSystem, así que C3 sigue siendo In (`z` no es un código de X: valueset.html, compose.exclude). C1, C2 y C2b pasan a Out y se nombran como mejora: si el código existe, el VS lo excluye; si no existe, el include no lo selecciona. Cada caso es una fila de la tabla |
| H2 | El diferencial no ejercita lo que cambia: 0 excludes de un sistema completo, 2 excludes por valueSet, 0 ciclos en 63 paquetes | **Aceptado.** El comando diferencial cuenta y publica cuántas veces se pasa por cada rama cambiada. Cada rama tiene al menos un caso sintético. Las mutaciones "quitar el supuesto de existencia" e "invertir el exclude" tienen que hacer fallar los tests, no el corpus |
| H3 | Los ciclos cambian más de lo dicho: hoy el resultado depende del orden (`r1`: assumed; `r3`: Invalid) y una cadena de más de 16 da Unknown | **Aceptado.** Se nombran los dos cambios: el resultado deja de depender del orden y la profundidad deja de tener tope. Se agrega un test metamórfico: intercambiar dos ValueSets equivalentes no cambia la respuesta |
| H4 | Divergencias sin resolver: C4 (`modifierExtension` en un include de otro sistema), C5 (include vacío), un VS anidado que solo tiene expansión, C6 (exclude del sistema completo con otra versión) | **Aceptado**, con una decisión y una fila por caso. **C4:** un include de otro sistema no selecciona X, tenga o no un modificador; queda Out en los dos modos, y el modo versión se alinea. **C5:** el include vacío no selecciona nada (vsd-1 lo prohíbe): Out en los dos modos. **VS anidado sin composición:** el modo cargado sigue con `expansionCodes`; el modo versión queda Unresolved, porque la expansión no es de esa versión. **C6:** en el modo cargado, el exclude del sistema completo quita el código sea cual sea su versión, como hoy (el código del Coding es el cargado); en el modo versión se compara la versión, como hoy |
| H5 | El criterio de salida pide que no quede `maxValueSetDepth`, pero `collectSystems` lo usa | **Aceptado.** `collectSystems`, el tipo `walk` y su tope quedan fuera del criterio y del alcance |
| H6 | `lockedDate` en el modo cargado: nada asegura que la versión cargada sea la que regía en esa fecha | **Aceptado, cumplir la especificación.** Una cláusula sin versión bajo `lockedDate` queda incompleta en el modo cargado, igual que un `include.version` que no está cargado (`clauseCodes`, `complete=false`). Impacto medido: ninguno (1 ValueSet, con todos sus includes versionados) |
| H7 | `include.valueSet`: la definición dice union en R4 y R4B, e intersection en R5; la narrativa de R4 (4.9.5, 4.9.8) dice intersección. La corrida en tres versiones con structs sintéticos no prueba nada | **Aceptado.** Se registra la contradicción y la decisión: intersección, por la narrativa de R4 y la corrección de R5 (el comportamiento de hoy). Se agrega una fila con varios `valueSet` en una cláusula (`action-participant-role`, core R4). La corrida en tres versiones carga ValueSets reales de los core r4, r4b y r5 |
| H8 | Regla 5: la parte de sistema depende de `externalSystems` (11 URIs en el código) y de `filterOps` | **Aceptado como registro.** Son excepciones que ya existen, no nuevas, y quedan listadas aquí. `filterOps` es semántica de prosa (valueset-filter-operator), una excepción legítima. `externalSystems` solo cambia algo cuando un paquete trae una copia `complete` de uno de esos sistemas. Revisarlo es otro incremento, anotado en el backlog |
| H9 | Los cambios de Unknown a Out llegan a `slicematch` (modo estricto) y a `memberOf` de FHIRPath | **Aceptado.** El diferencial ya compara todos los issues, incluidos los de slicing y los de invariantes. Se agrega una fila de test por consumidor |
| H10 | Claves del caché: hay que usar el `*ValueSet` resuelto, no la URL ("X" y "X\|1" resuelven al mismo); el modo lo decide quien llama | **Aceptado.** Los nodos del punto fijo se identifican por el `*ValueSet` resuelto más (existencia, lock). `answerCache` agrega la versión no cargada a su clave. Un test pregunta la misma terna con dos versiones sobre un mismo registry. La caída silenciosa de `GetValueSetVersion` para un `include.valueSet` versionado queda fuera de alcance |
| H11 | `compose.inactive` y los `modifierExtension` de `Compose` y del ValueSet no se miran | **Aceptado, fuera de alcance**, con la medición: 2 ValueSets usan `inactive`, ambos con `true` (el comportamiento permisivo de hoy) |
| H12 | Falta un criterio de rendimiento | **Aceptado.** Se agrega un benchmark que cuenta las decisiones por llamada: la clausura más grande de VSAC, más uno sintético con muchos ValueSets que entran a uno solo y un ciclo. Criterio: no peor que `main` en la clausura real |
| H13 | Mezcla tres cambios con riesgos distintos | **Aceptado: tres incrementos**, cada uno con su diferencial. **1)** Punto fijo en el modo cargado (cambios nombrados en H3). **2)** Exclude por el supuesto de existencia (H1, C3 decidido). **3)** Integrar el modo versión no cargada, con `systemClause` como parte de sistema (H4, H6, H10). La lógica de tres valores y la existencia son diseño del proyecto, no de la especificación: la tabla separa qué filas citan la especificación y cuáles citan el diseño |

## Segunda ronda adversarial (Codex, 18-10-2026)

Codex no pudo ejecutar casos, porque el sandbox no le dejó crear el scratchpad; sus hallazgos salen
de leer el código.

| # | Hallazgo | Resolución |
| --- | --- | --- |
| 1 | Los positivos de otra edición (la caída de versión, `lockedDate`) se toman como ciertos | Aceptado → backlog **B1** (próximo incremento) |
| 2 | `lockedDate` debe fijar también los ValueSets anidados | Aceptado → backlog **B2** |
| 3 | C3 se mantenía In por compatibilidad; el algoritmo de expansión exige validar los códigos enumerados; `partial()` no cubre `supplement` | **Aceptado**: C3 pasa a Out en el incremento 2; la evidencia tiene tres valores por edición, y `supplement` da evidencia ausente |
| 4 | C6 da un Out falso (X\|1 incluido, X\|2 excluido entero) | **Aceptado**: la edición se aplica a includes y excludes |
| 5 | Quitar `coversSystem` en el incremento 1 rompe un caso acíclico; el diseño y el criterio contradecían las resoluciones | **Aceptado**: `coversSystem` se retira en el incremento 2; el diseño y el criterio se reescribieron por incremento |
| 6 | El caché de negativos no se invalida con un cambio de autoridad | Aceptado → backlog **B3** (no es de la composición) |
| 7 | Un diferencial inoperante pasaría el criterio | **Aceptado**: se agregan mutaciones del arnés |
| 8 | El efecto de `externalSystems` es mayor que el dicho; necesita un ADR | Aceptado → backlog **B5**; se corrigió la afirmación |
| 9 | Una expansión paginada de R5 (`next`) da un Out falso | Aceptado → backlog **B4** |
| 10 | Contar decisiones no mide el costo | **Aceptado**: el benchmark mide tiempo, asignaciones y aristas recorridas, en caché fría y caliente |

Sólido según Codex: la intersección de `include.valueSet` (H7), las claves por `*ValueSet` (H10) y
que slicing y `memberOf` usen pertenencia estricta.

## Code review del incremento 1 (paso 8)

| Ronda | Hallazgo | Resolución |
| --- | --- | --- |
| 1 | (media) Los cambios nombrados no valían para un elemento `code`: `collectSystems` tenía tope 16 y daba Unknown ante un ciclo | Corregido: los sistemas de un ciclo son la unión, sin tope (el mapa `walked` evita repetir). Filas de elemento `code` en `membership_fixpoint_test.go`, que fallaban antes |
| 1 | (baja) El criterio de salida decía que ya no quedaba `deciding`, y el benchmark era solo de caché fría | Se corrigió el texto del criterio. Se agregaron los benchmarks de caché caliente y de ciclo |
| 1 | (baja) `m.value[vs] = answerUnknown` sobraba (mutante equivalente) | Eliminada |
| 1 | (baja) Comentarios desactualizados (`answerUnknown`, `maxValueSetDepth`) y un `nolint` en inglés | Corregidos, en español |
| 1 | (baja) Mutantes de hl7diff que sobrevivían (la falla fuera de la firma, `count = 1`, el resumen de cobertura reimplementado en el test) | Casos nuevos: una falla cambiada y un duplicado solo en la base. `summarize` se testea directamente. Las cuatro mutaciones se detectan |
| 1 | (baja) `readCoverBlocks`: la clave sin el archivo, el sufijo sin anclar, el 0/0 silencioso. `funcRanges` sin el receptor. `slices.Sorted(maps.Keys)` | Corregidos: la clave es archivo más posición, el sufijo `"/"+file`, falla sin bloques, el método lleva su receptor |
| 1 | (baja) La evidencia del benchmark no demuestra igualdad (p≈0,2) | El registro dice "sin regresión medible", con las corridas y la carga de la máquina |
| 2 | Los siete de la ronda 1, verificados como corregidos (con mutaciones) | — |
| 2 | (baja) N1: un ValueSet que solo se incluye a sí mismo responde distinto como elemento `code` (Out) y como Coding (Unknown) | Fijado con `TestMembershipSelfOnly`: Out es la menor solución; Unknown es lo conservador de los tres valores |
| 2 | (baja) N2: ningún test fijaba que lo no sabido de un ValueSet anidado ausente suba en `collectSystems` | `TestMembershipUnknownSystemsPropagate`; la mutación ahora se detecta |
| 2 | (sugerencia) N3: la clave de `readCoverBlocks` con dos rutas que terminan igual | Agregado al perfil del test; la mutación se detecta |

**Convergencia:** la ronda 2 no dejó hallazgos de severidad media o mayor. El incremento 1 está cerrado.

## Validación del incremento 2 (pasos 5 a 7)

- **Tests primero:** `TestMembershipExcludeEvidence` (`membership_exclude_test.go`), con los valores
  esperados según el álgebra de composiciones y la regla de evidencia. Antes de implementar, 7
  filas fallaban: justo C1, C2, C2b, C3 y el supplement. Al implementarse, la fixture del ciclo de
  `membership_test.go` tuvo que listar `k` en su code system `other`: por C3, un código enumerado que
  el sistema cargado no tiene deja de ser miembro.
- **Mutaciones** (a mano, en una copia de `membership.go`; cuántos tests fallan):

  | Mutación | Fallas |
  | --- | --- |
  | sin el supuesto de existencia en el exclude | 9 |
  | el ValueSet anidado sin el supuesto de existencia | 8 |
  | el exclude ignorado | 29 |
  | sin C3 (`editionLacks` siempre falso) | 3 |
  | el supplement tomado como evidencia negativa | 2 |
  | la raíz decidida con existencia | 9 |
  | la clave del memo sin `exists` | 0 → 2, con la fila `samex` (el mismo ValueSet incluido y excluido) |

- **Corpus** (`hl7diff run -identical -cover pkg/terminology/membership.go`, contra `origin/main`):
  los 48 grupos livianos y `r4-core-examples`, idénticos issue por issue. Los cambios nombrados no
  cambian ningún resultado del corpus: ningún ejemplo usa los dos ValueSets de THO que toca C3
  (`editionLacks` sí se ejecuta: 782 y 284 veces). Evidencia:
  [evidence/2026-10-18-composition-evaluator-inc2/](evidence/2026-10-18-composition-evaluator-inc2/).
- Lint (golangci-lint v2.11.3 con Go 1.25.13) sin issues; `go test -race -short ./...` en verde.

## Code review del incremento 2 (paso 8)

| Ronda | Hallazgo | Resolución |
| --- | --- | --- |
| 1 | Ninguna respuesta incorrecta ni dependiente del orden. El supuesto de existencia vale cuando include y exclude hablan de la misma edición; con ediciones mezcladas es B6 | — |
| 1 | (media) Sin fila para un sistema no cargado tomado entero a través de un ValueSet incluido (mutación: el anidado siempre con existencia) | Fila en `TestMembershipEvidenceEdges`; la mutación se detecta |
| 1 | (media) C6 sin fila: X\|2 cargada y completa, excluida entera, no quita el `a` que no tiene | Fila; la mutación (el supuesto aunque la cláusula dé Out) se detecta |
| 1 | (media) La regla de evidencia ausente sin filas: una edición no cargada, un fragment | Filas, más `example` y `not-present`; las mutaciones (sin el chequeo de versión, sin `partial()`) se detectan |
| 1 | (baja) `GetCodeSystem` en lugar de la edición que nombra la cláusula; la raíz sin releer tras el punto fijo (incremento 1); sin el corte cuando ningún include selecciona; sin `isExternalSystem` en `editionLacks` | Una fila por cada una; las cuatro mutaciones se detectan |
| 1 | (sugerencia) El orden entre el modifier y C3: un código enumerado con modifierExtension que el sistema completo no tiene | Se mantiene Unknown: un modifier no entendido hace desconocido lo que la cláusula significa. Fijado con una fila; invertir el orden se detecta |
| 1 | (baja) Faltaba un ciclo que pasa por excludes (R = U − V, V = U − R) | Fila: Unresolved, la menor solución en el orden de Kleene |
| 1 | (baja) La rama del supplement solo cuenta cuando una cláusula nombra la URL del supplement | Comentario en `contentSupplement` |
| 1 | (estilo) El comentario de `selects` mezclaba inglés y español | En español |

## Registro

| Paso | Estado |
| --- | --- |
| 1 Revisión previa | hecho (arriba) |
| 2 Plan | este documento |
| 3 Ronda adversarial | hecho: 13 hallazgos más 10 de Codex, aceptados o pasados al backlog |
| 4 Aprobación | aprobado por el usuario (18-10-2026): unificación con los cambios nombrados, grupo B al backlog, B1 como próximo incremento |
