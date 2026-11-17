# #193, pieza 2: capas por valor para `type.profile` con varios perfiles

Rama `refactor/type-profile-layers`, desde `87c0b02`. Viene de la partición de
[2026-11-05-type-profile-alternatives.md](2026-11-05-type-profile-alternatives.md) §3c. La pieza 1 está mergeada:
[2026-11-05-children-precedence.md](2026-11-05-children-precedence.md), #209.

## 1. Revisión previa (06-11-2026)

### Especificación

R4 `elementdefinition-definitions.html#ElementDefinition.type.profile`: «If any profiles are specified, then the
content must conform to at least one of them.» El mismo texto en R4B y R5. Es un O, y la definición no hace excepción
para un tipo de elección.

### Referencia: HL7 validator_cli 7.0.4

Sondas p1 a p21 en `evidence/2026-11-05-type-profile-alternatives/probe/` (README con la tabla). En resumen:
- **Ninguno cumple** (p3, p5, p6, p17): `Validation_VAL_Profile_NoMatch` en el valor, y los errores de cada perfil.
- **Uno cumple** (p1, p2, p7, p16, p18): nada de los perfiles que no cumple.
- **Varios cumplen** (p13, p14, p19): el warning `Validation_VAL_Profile_MultipleMatches`.
- **Perfil que no se resuelve** (p6 a p8): `Unknown_Profile` por cada uno, y cuenta como no cumplido.
- **Repetido** (p9): se decide por valor.
- **Extensión** (p10): igual que un elemento.
- **Tipo de elección** (p12): HL7 usa **solo el primer perfil**, sin `NoMatch`. Es un defecto de HL7
  (`InstanceValidator.java:8072`, `:8085`); la especificación pide «at least one».
- **Perfil de otro tipo** (p20): HL7 recorre el Quantity contra un perfil de CodeableConcept. Es otro defecto.
- **Violación común** (p21): HL7 junta los mensajes de texto idéntico (`sqty-1`, una vez) y no los de cardinalidad,
  porque su texto nombra la definición de origen («from <perfil>», dos veces).

### gofhir en `87c0b02`

Con dos o más perfiles para el tipo del valor, `TypeProfile` devuelve vacío (`type_profile.go:26-28`) y
`MemberTree` también (`instance_children.go:118`). Entonces se valida contra el tipo base y no se informa nada de los
perfiles: ni `NoMatch` ni `TYPE_PROFILE_NOT_FOUND` (`gofhir-87c0b02-4.0.1.json`).

### Hallazgos abiertos de la ronda 3c, verificados en `87c0b02`

- **C1, en parte refutado.** `slicematch.Scope` no es solo de la fase de constraint: lo arman el slicing
  (`slicing.go:129`, `:243`, `:275`), las referencias (`reference.go:854`) y los holders (`holder.go:222`,
  `rootScope`). `%resource` sale del Scope dentro de `Conforms` (`validator/slicematch.go:290-296`). Solo la
  cardinalidad corre sin Scope y con `context.Background()` (`cardinality.go:281`). El orden de las fases está en
  `validator.go:878-972`: estructura, cardinalidad, primitivos, extensiones, constraint, slicing, referencias, binding y
  UCUM.
- **C2, confirmado.** La cardinalidad recorre `Children`, nunca un slice (`cardinality.go:276-279`). Los llamadores
  (c) comparan contra el nodo sin slice (`slicing.go:197`, `tree.go:247`). `MemberTree` solo resuelve un perfil.
- **C4, confirmado, en otra ubicación.** El memo de `Conforms` es `conformKey{profile, value uintptr, assume}`
  (`validator/slicematch.go:144-148`) y un valor que no es objeto no se comprueba (`:167-169`): los primitivos quedan
  para la pieza 3.
- **C5, medido con p21:** ver la referencia.

### Llamadores (06-11-2026)

| Uso del resultado | Llamadores |
|---|---|
| **(a)** cada hijo bajo la definición | `cardinality.go:281`, `holder.go:144`, `extension/context.go:146`, `extension/extension.go:187` y `:494` (`TypeChildrenOf`), `binding/walk.go:22`, `reference.go:540`, `ucumvalidator.go:80`, `walker/discover.go:84`, `slicing/tree.go:81` |
| **(b)** una alcanza | `extension/extension.go:416`, `slicing/tree.go:242` (solo informa), `constraint/tree.go:375` y `:379` (`typeLayer`) |
| **(c)** comparación con la base | `slicing/slicing.go:197` y `:208`, `slicing/tree.go:247` y `:248` (`MemberTree`) |

Memos por valor ya existentes: `conformState` (clave `profile, value, assume`), `firstReportKey` (texto con ruta, ID
y URL), `holderGroups` (`{def, inst}`) y `slicerecord` (`{element, parent, key, index}`, que sí cubre primitivos).

### `gofhir/server` (congelado)

No trata `type.profile` múltiple (2026-11-05-type-profile-alternatives §1). Nada que tomar.

## 2. Plan

### Alcance

- Un elemento cuyo tipo declara **dos o más** perfiles para el tipo de un valor que es un objeto (tipo de datos
  complejo). Sirve igual para el elemento sin slice y para el slice que lo declara: decide el nodo que el llamador
  tiene, con la cadena `SliceOf` de la pieza 1.
- Elección por valor, con la regla de HL7, y capas: cada fase que baja controla los hijos bajo cada perfil elegido.
- Los avisos `NoMatch` y `MultipleMatches`, y `TYPE_PROFILE_NOT_FOUND` por cada perfil que no se resuelve.

### Fuera del alcance

- Primitivos (C4, s22): pieza 3.
- Hijos que el perfil despliega (s11, s12): incremento 2.
- `targetProfile` de Reference y canonical.
- El texto «(validating against …)».

### Diseño

1. **Registro.** `InstanceChildren` suma `Layers []Layer`: cada capa tiene `SD`, `Nodes` y su perfil. Con cero o un
   perfil hay una sola capa, igual que hoy. `SD` y `Nodes` siguen siendo los de la primera capa, para los llamadores
   (b).
2. **El valor llega al registro.** `ChildrenOf` recibe, en vez de `self`, una `Instance` con el valor y `self`. El
   registro no conoce el pipeline: la elección es una interfaz (`ProfileChooser`) que se inyecta por ctx, como
   `constraint.HolderProfiles`. Sin elector, todas las capas.
3. **Elector, en `pkg/validator`.** Una sola función, compartida con `holderProfiles.Choose`:
   - un perfil que no se resuelve o que es de otro tipo cuenta como no cumplido, y no se recorre;
   - de los demás, los que `Conforms` acepta; si no acepta ninguno, todos.
   - Necesita un Scope: la cardinalidad pasa a recibir el ctx de la validación y toma el Scope raíz que la validación ya deja en
     el ctx (`rootScopeKey`, `holder.go:88`, `:217`).
4. **Memo.** Una elección por valor y validación, en `conformState`, con clave `{nodo, valor, raíz del Scope}`. La raíz
   del Scope entra en la clave porque `%resource` cambia en un recurso contenido o en una entrada de Bundle (C1).
5. **Recorrido.**
   - Los llamadores (a) iteran las capas.
   - Lo que dos capas informan con el mismo ID, severidad, ruta y diagnóstico se informa una vez, con una clave por
     valor.
   - Los llamadores (c) omiten el min/max de un hijo del slice que coincide con el del mismo hijo en **alguna** capa.
6. **Avisos** en la ubicación del valor, una vez, desde la fase de constraint con `firstReportKey`:
   - `PROFILE_NO_MATCH` (error);
   - `PROFILE_MULTIPLE_MATCHES` (warning).

   `TYPE_PROFILE_NOT_FOUND` (error) se informa por cada perfil que no se resuelve, con el dueño de la pieza 1.
7. **Divergencias que se declaran en hl7diff:**
   - p12: el tipo de elección usa todos los perfiles, como dice la especificación; se reporta a HL7 con issue y PR;
   - p20: un perfil de otro tipo no se recorre;
   - p21: con la deduplicación, la cardinalidad común a dos capas se informa una vez. HL7 la informa dos veces solo
     porque el texto nombra la definición.

### Criterio de salida

- **Tests** con los valores de HL7:
  - p1 a p5 y p13 en R4, R4B y R5;
  - p6 a p10, p17 a p21 en R4. Antes de escribir los tests se sondean en R4B y R5; si coinciden, entran en las tres
    versiones.
  - Se comprueba lo que aparece y lo que no, por ID, severidad y ubicación, y que nada se repite.
- **Un test por llamador (a)** en el que solo la **segunda** capa elegida tiene la regla que falla. Mata «solo la
  primera capa» en cada uno.
- **Una sonda nueva por llamador (c):** un slice cuyo hijo coincide con la segunda capa y no con la primera.
- **Un test de recurso contenido:** el mismo valor bajo dos `%resource` distintos, con un invariante del perfil que
  depende de `%resource`. Mata el memo sin la raíz del Scope.
- **Mutaciones:**
  - sin elección (todas las capas);
  - siempre el primer perfil;
  - una sola capa en cada llamador (a);
  - sin deduplicación;
  - sin `NoMatch`;
  - sin `MultipleMatches`;
  - ignorar `SliceOf`;
  - la cardinalidad con `context.Background()`;
  - el memo sin la raíz del Scope;
  - (c) comparando solo contra la primera capa.
- **Sin regresión:** `hl7diff run` contra `main` (los grupos livianos y `r4-core-examples` por separado), con p12, p20 y
  p21 declaradas. Con cero o un perfil, la salida no cambia.
- **Costo:** bench del pipeline y de un Bundle con muchos valores con varios perfiles, antes y después; sin regresión en
  asignaciones en los que tienen cero o un perfil.
- `make check` en verde.

### Dependencias

Ninguna nueva.

## 3. Ronda adversarial 1 (Codex, 06-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| A1 | `typeLayer` no es (b): da las invariantes de la raíz del perfil (`constraint/tree.go:73-75`). Con una sola capa, p3 pierde `mqty-1` (`mtqy-1` en R5) | alta | **Aceptado** (verificado en el código). La constraint itera las raíces elegidas. Test: una invariante que solo tiene el segundo perfil |
| A2 | El Scope raíz no sirve para la cardinalidad: recorre los recursos que el recurso tiene (`cardinality.go:95-101`), y `ScopeOf` cambia `Resource`, `RootResource`, `Container` y `Outer` (`resolver_adapter.go:165-176`). Además, `Conforms` memoiza sin el Scope | alta | **Aceptado en parte.** El Scope se propaga por recurso, igual que en las otras fases: la cardinalidad lo rebasa con `ScopeOf` en cada recurso que recorre. **Rechazado** meter el Scope en las claves: un map del valor está en un solo lugar del árbol, así que su recurso y su Scope quedan fijos. El riesgo real es pasar un Scope equivocado, y eso lo prueban tests con un recurso contenido, una entrada de Bundle y `resolve()` |
| A3 | Un slice que declara varios perfiles no tiene diseño: `MemberTree` rechaza más de uno (`instance_children.go:118`) y el slicing lee `member.Children` (`tree.go:248-249`) | alta | **Aceptado.** `MemberTree` pasa a devolver capas con el valor, como `ChildrenOf`, para sus cuatro llamadores (holder, binding, slicing, reference). Las restricciones del slice se comparan contra las capas que la cardinalidad controla en el nodo sin slice. Sondas: un slice hoja y un reslice con varios perfiles, sin hijos desplegados |
| A4 | Un memo propio de la elección congela respuestas provisionales de ciclos (`slicematch.go:183-208`, `:232-261`) | alta | **Aceptado.** No hay memo propio: la elección se recalcula desde `Conforms`, que ya memoiza y asienta los ciclos. El costo se mide (bench). Sondas: un perfil recursivo, un perfil con una referencia y uno con un discriminador por perfil, en los dos órdenes de los candidatos |
| A5 | La deduplicación no dice qué la aísla: las corridas de `Conforms` usan scopes de informe propios (`slicematch.go:293-294`) y podrían consumir las claves de la validación visible | media | **Aceptado.** La clave vive en el scope de informe: una corrida especulativa empieza uno nuevo. `TYPE_PROFILE_NOT_FOUND` mantiene el nodo declarante. Tests: una corrida especulativa antes de la visible, y el mismo canónico declarado en dos nodos |
| A6 | Las mediciones pueden no ver regresiones: falta p12, las versiones R4B y R5 quedan condicionadas, no hay inventario por multiconjunto, y el rendimiento no se compara con HL7 | media | **Aceptado.** Entran p12 y lo que cada versión espera, aunque difiera. El inventario de hl7diff se hace por multiconjunto. El rendimiento se compara con HL7, que es el rival de la carta §10: mismos packages, entradas y `-tx n/a`, sobre un lote grande para que el arranque de la JVM no domine |

## 2 v2. Plan (después de la ronda 1)

Cambia el diseño de §2 así:

1. **Registro.** `ChildrenOf` y `MemberTree` reciben la `Instance` (el valor y `self`) y devuelven capas. Con cero o un
   perfil, una sola capa, igual que hoy.
2. **Elector.** Igual que en §2, pero **sin memo propio**: cada llamada pregunta a `Conforms`, que memoiza y asienta los
   ciclos. Se mide su costo.
3. **Scope.** Cada fase que elige usa el Scope del recurso en que está el valor. La cardinalidad lo arma desde el Scope
   raíz y lo rebasa con `constraint.ScopeOf` en cada recurso que recorre. Las claves de los memos no cambian (A2).
4. **Capas en todas las fases que bajan,** también en la constraint: cada raíz elegida aporta sus invariantes, y la
   distinción de la pieza 1 entre la precedencia del recorrido y las raíces de invariantes se mantiene.
5. **Slices.** `MemberTree` devuelve las capas del slice. Las restricciones del slice se comparan contra las capas que la
   cardinalidad controla para el nodo sin slice.
6. **Deduplicación.** La clave (ID, severidad, ruta, diagnóstico) vive en el scope de informe de la validación. Una
   corrida especulativa de `Conforms` empieza uno nuevo.

**Criterio de salida:** el de §2, más:
- p12 y lo que cada versión espera en R4, R4B y R5, aunque difiera;
- un test de una invariante que solo tiene el segundo perfil;
- un recurso contenido, una entrada de Bundle y `resolve()`;
- un slice hoja y un reslice con varios perfiles;
- un perfil recursivo, uno con referencia y uno con discriminador por perfil, en los dos órdenes;
- una corrida especulativa antes de la visible;
- el mismo canónico en dos nodos;
- el inventario de hl7diff por multiconjunto;
- el rendimiento comparado con HL7 sobre un lote grande, con los mismos packages, entradas y `-tx n/a`.

**Mutaciones nuevas:**
- la constraint con una sola raíz;
- la cardinalidad con el Scope raíz sin rebasar;
- `MemberTree` con una sola capa;
- la deduplicación compartida con las corridas especulativas.

## 3b. Ronda adversarial 2 (Codex, 06-11-2026)

A1, A3, A5 y A6 quedan resueltos en el diseño.

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| B1 | El rechazo de A2 es falso. Si una Observation contenida y un Bundle contenido referencian `"#"`, los dos resuelven al mismo map del Patient (`reference.go:417-425`). Pero `checkTargetConformance` le copia al Scope del destino el `Container` de quien referencia (`:854-859`), y al entrar al Bundle ese `Container` cambia (`:269-288`). La clave de `Conforms` es la misma con dos entornos distintos. Codex lo comprobó con un test temporal | alta | **Aceptado.** Es un defecto que ya existe en `main`. Corrección, como paso 0 de la pieza: el Scope del destino sale de dónde está el destino, no de quien referencia. Así la premisa de A2 (un map, un Scope) vale por construcción, y las claves siguen sin el Scope. Test de regresión con esos dos referentes |
| B2 | Sin memo propio, dos fases pueden elegir distinto para un valor dentro de un ciclo: una respuesta provisional se invalida entre fases (`slicematch.go:183-190`, `:230-238`), y los informes de la fase anterior quedan en el resultado | media | **Aceptado en parte.** En la validación visible las respuestas de `Conforms` son definitivas: no hay un chequeo abierto que las vuelva provisionales. Dentro de una corrida especulativa, el chequeo que contiene la respuesta invalidada queda provisional y vuelve a correr entero, con todas sus fases (`settle`, hasta `maxSwayedChecks`). Ese límite ya existe y está documentado (`:138-143`). Se agrega un test que fuerza la invalidación entre fases y comprueba las capas elegidas y los diagnósticos finales |

## 2 v3. Plan (después de la ronda 2)

§2 v2, más:

0. **Paso 0: Scope canónico del destino de una referencia.** `checkTargetConformance` arma el Scope del destino desde el
   recurso al que pertenece el destino: el raíz, un contenido, una entrada de Bundle o un recurso que otro tiene.
   Ya no hereda el `Container` ni el `Outer` de quien referencia. Antes del cambio se sondea en HL7 un `resolve()` desde
   el destino con los dos referentes de B1, para tomar de ahí el valor esperado.
7. **Elección dentro de ciclos:** se apoya en la invalidación y en el nuevo chequeo que `settle` ya hace. No se agrega
   un mecanismo propio.

**Criterio de salida:** el de §2 v2, más el test de regresión de B1 (con el valor de HL7) y el de B2.
**Mutaciones nuevas:** el Scope del destino con el `Container` de quien referencia.

## 3c. Ronda adversarial 3, la última (Codex, 06-11-2026)

**No converge:** 1 alto y 1 medio. Por la regla de tres rondas, se consulta al usuario.

| # | Hallazgo | Sev. |
|---|---|---|
| C1 | B1 sigue incompleto: el paso 0 corrige los destinos, pero otros recorridos arman Scopes distintos para el mismo map. `constraint.ScopeOf` vuelve antes de tiempo en un recurso contenido (`resolver_adapter.go:166-167`) y no normaliza un Bundle contenido. `nestedScope` (`slicing.go:275-280`) y la constraint (`constraint/tree.go:252-259`) sí lo normalizan. Experimento de Codex: para el mismo Bundle contenido, `Container` vale `Patient` por `ScopeOf` y `Bundle` por el slicing. `heldDefinitions` usa `ScopeOf` (`holder.go:93`). Propuesta: un solo constructor de Scope para los recorridos de recursos, los holders y los destinos de referencias | alta |
| C2 | La nueva corrida de B2 no renueva `holderGroups`: las corridas especulativas heredan el memo (`holder.go:116-134`), y `settle` invalida las respuestas de `Conforms` pero no los grupos. Experimento: los grupos siguen siendo `[named-patient]` después de que el discriminador pasa a falso; con un memo nuevo dan `[]`. Propuesta: un memo de grupos nuevo por corrida especulativa, o invalidarlos junto con las respuestas provisionales | media |

Los dos son defectos que ya están en `main`, independientes de las capas.

**Decisión del usuario (06-11-2026):** pieza previa. Un incremento propio corrige antes C1, B1 y C2: un solo
constructor de Scope y un memo de `holderGroups` nuevo por corrida especulativa. Tiene su plan y sus sondas en HL7. La
pieza 2 lo da por hecho y vuelve a la ronda adversarial con un diff más chico.

## 2 v4. Plan (después de la pieza previa, 14-11-2026)

Rama `refactor/type-profile-layers`, desde `16bc506` (#215, la pieza previa).

**Sondas re-verificadas (14-11-2026).** HL7 7.0.4 vuelve a correr p1 a p21 en R4 con el log de consola guardado. No hay
mensajes de snapshot («does not repeat», «snapshot»). Las 21 salidas son iguales, issue por issue, a las guardadas
(`hl7-*-4.0.1-7.0.4.json`). Es la lección de r3 en la pieza previa: un perfil de sonda que HL7 no puede armar invalida su
salida.

**Qué resolvió la pieza previa (#215):**
- **El paso 0, B1 y C1:** todo map de recurso tiene un Scope canónico en `internal/scopeindex`, y
  `TestScopeInvariantOnTheProbes` exige que `Conforms` reciba ese Scope. La premisa de A2 («un map, un Scope») queda
  garantizada y protegida por un test, así que la clave de `Conforms` sigue sin el Scope.
- **C2:** las corridas especulativas usan `holderGroups` sin memo.

**Cambios respecto de §2 v3:**
1. **Sin el paso 0:** ya está en `main`.
2. **Scope para elegir (reemplaza el punto 3 de §2 v2):** la fase que elige pide el Scope a `scopeindex.Resolve` para el
   recurso que contiene el valor, sin armarlo. Para eso la cardinalidad deja `context.Background()`
   (`cardinality.go:281`, `:292`) y recibe el ctx de la validación, que lleva el índice. Un recurso que falta en el índice
   hace abortar la validación, como en las otras fases.
3. **Ciclos:** igual que §2 v3, punto 7. `settle` vuelve a correr el chequeo especulativo entero, ahora con `holderGroups`
   nuevos.

El resto de §2 v2 y v3 sigue igual: registro con capas en `ChildrenOf` y `MemberTree`, elector sin memo propio,
capas en todas las fases que bajan, incluida la constraint (A1), slices (A3), deduplicación en el scope de informe
(A5), avisos y divergencias declaradas (p12, p20 y p21).

**Criterio de salida:** el de §2 v2 y v3, salvo el test de regresión de B1, que ya está en #215. Además:
- el test del invariante de alcance corre también sobre las sondas de esta pieza;
- una sonda donde la elección depende de `%resource` en un recurso contenido y en una entrada de Bundle, con el valor
  de HL7.

**Mutaciones:** las de §2, v2 y v3, salvo la del Scope del destino (ya está en #215). Además:
- la cardinalidad con `context.Background()`, es decir, sin índice;
- el elector con el Scope del recorrido en vez del índice.

**Ronda adversarial:** el usuario decidió el 06-11-2026 que la pieza 2 vuelve a la ronda adversarial después de la
pieza previa. Es una serie nueva (ronda 1 de 3) sobre §2 v4.

## 3d. Serie nueva, ronda adversarial 1 (Codex, 14-11-2026)

Todos los hallazgos se verificaron en el código.

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| D1 | El Scope de un tipo de datos no está protegido. El índice no tiene tipos de datos (`scopeindex.go:13`), el invariante solo mira recursos (`validator/slicematch.go:183`), y UCUM y la recursión de extensiones no saben a qué recurso pertenece el valor (`ucumvalidator.go:80`, `extension.go:186`). Pasar el ctx no alcanza | alta | **Aceptado.** La `Instance` del registro lleva el recurso que contiene el valor, y cada llamador (a) lo propaga, también en las corridas especulativas. El elector pide a `scopeindex.Resolve` el Scope de ese recurso. El invariante se extiende a los tipos de datos: el Scope que llega para un valor que no es recurso es el del índice para su dueño. Test por llamador con alternativas que dependen de `%resource` |
| D2 | Las capas en los holders exigen cambiar cómo se guardan y cómo se consumen. `collectHolders` pisa `groups[mapKey(m)]` (`holder.go:179`) y `heldDefinitions` pasa esa lista única a `Choose` como alternativas (`:109-119`). Iterar capas pierde los requisitos anteriores, y concatenar convierte grupos independientes en un solo O | alta | **Aceptado.** Los grupos se guardan por recurso y por capa de origen, y cada grupo se exige por separado. Test: un fallo exclusivo de cada grupo, en los dos órdenes de capas |
| D3 | La supresión que ya existe contradice la clave de deduplicación: `pendingConstraints` suprime por clave y expresión (`constraint/tree.go:600`), y `firstReport` suma la ubicación (`report_scope.go:46`). Ninguna mira severidad ni texto, así que una invariante de igual clave y expresión con otra severidad se pierde antes de deduplicar | media (Codex: alta) | **Aceptado.** Una sola identidad para las capas: clave, expresión, severidad y texto, la misma en `pendingConstraints`, en `firstReport` y en la deduplicación. Baja a media porque dos perfiles con la misma clave y expresión y distinta severidad es raro, pero es posible. Tests: candidatos en orden inverso, severidad o texto distintos y aislamiento especulativo |
| D4 | Sin alternativas usables no hay comportamiento definido: si todos los perfiles son de otro tipo o no se resuelven, no queda capa, mientras que hoy un perfil no resuelto baja por la definición propia o la base (`instance_children.go:65-89`) | media | **Aceptado.** Sin perfiles usables, una sola capa, la de hoy: la definición propia o la del tipo (la regla de la pieza 1, como HL7). Test: contenido base inválido con dos perfiles que no se resuelven |
| D5 | Los límites del memo no acotan el recorrido por capas: `maxSwayedChecks` cuenta chequeos de conformidad, no recorridos de descendientes que se repiten por capas superpuestas | media | **Aceptado.** Bench de escala (profundidad × alternativas × repeticiones), con el conteo de visitas y chequeos. Cota declarada: lineal en el número de capas elegidas por valor; si supera esa cota, el plan vuelve al paso 2 |
| D6 | `type.profile` puede nombrar un ImplementationGuide («the type SHALL conform to at least one profile defined in the implementation guide», R4 y R5) o un StructureDefinition contenido | media | **Aceptado como fuera del alcance.** Se sondea en HL7 qué hace hoy con un canónico de IG. Se mantiene la conducta actual (perfil no resuelto) y se abre un issue con esa evidencia. Se registra como pérdida conocida, con un test que la documenta |

## 2 v5. Plan (después de la ronda 1 de la serie nueva)

§2 v4, más:
1. **Instance con dueño (D1):** `Instance{Value, Self, Owner}`, donde `Owner` es el map del recurso que contiene el valor.
   Lo propagan todos los llamadores (a) y las corridas especulativas. El elector y `Conforms` usan el Scope del índice
   para `Owner`, y el invariante cubre los tipos de datos.
2. **Holders por capa (D2):** `groups` pasa a ser una lista de grupos por recurso, cada uno con su capa de origen, y cada
   grupo se exige por separado.
3. **Una identidad de informe (D3):** clave, expresión, severidad y texto, compartida por `pendingConstraints`,
   `firstReport` y la deduplicación entre capas.
4. **Reserva sin perfiles usables (D4):** una capa, la definición propia o la del tipo, como en la pieza 1.
5. **Costo (D5):** bench de escala con conteo de visitas y chequeos, y cota lineal en las capas elegidas.
6. **IG como perfil (D6):** fuera del alcance. Sonda en HL7, issue y test de la pérdida conocida.

**Criterio de salida y mutaciones:** los de §2 v4, más los tests de D1 a D6. Mutaciones nuevas:
- `Owner` no se propaga en un llamador (a) o en una corrida especulativa;
- grupos de holders concatenados o pisados;
- la identidad de informe sin severidad o sin texto;
- sin la reserva de D4.

## 3e. Serie nueva, ronda adversarial 2 (Codex, 14-11-2026)

D2, D4 y D5 están resueltos. Siguen abiertos D1 y D3. Los puntos E2 y E3 se verificaron en el código.

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| E1 | `Owner` no llega a los llamadores del árbol de slicing: `ChildrenOf` y `MemberTree` en `slicing/tree.go:230`, `:247` y `slicing.go:195`, `:199`, `:210` no reciben el dueño | alta | **Aceptado.** `Owner` pasa por `checkMember`, `checkChildren` y `checkChildValue`, para el nodo sin slice y para el slice. Las raíces especulativas de tipo de datos toman el dueño de quien las lanza. En el límite de un recurso que otro tiene, el dueño pasa a ser ese recurso. Tests con alternativas que dependen de `%resource` |
| E2 | Extender el invariante a los tipos de datos choca con `resolve()`. `valueScope` canoniza los recursos pero deja a los tipos de datos con el Scope de la cadena (`slicematch.go:438-450`, verificado) | alta | **Aceptado.** Un tipo de datos se comprueba con el Scope canónico de su `Owner`, y la cadena conserva el suyo para la navegación. Es la regla que HL7 sigue para los recursos (r4). Antes de adoptarla para los tipos de datos se sondea en HL7: un discriminador por perfil que termina en un tipo de datos bajo un Bundle anidado resuelto, en R4, R4B y R5. Si HL7 hace otra cosa, el plan vuelve al paso 2. Tests en los dos órdenes de llegada, también a través de un recurso que otro tiene |
| E3 | La identidad compartida no llega a la agregación final: `dropReported` usa `issueKey`, sin la expresión (`validator.go:1048`, verificado). Además, sumar el texto en `pendingConstraints` cambia la supresión con un solo perfil, contra el criterio de que la salida no cambie | media | **Aceptado.** `pendingConstraints` y `firstReport` no cambian. La identidad nueva (clave, expresión, severidad y texto `human`) se usa solo para juntar informes entre capas del mismo valor, y la agregación la conserva. La política de cardinalidad por perfil raíz (`validator.go:1039`) se mantiene. Antes de cambiar una salida se sondea en HL7 (R4, R4B y R5) una invariante con el mismo texto y otra severidad entre capas, y ele-1 heredado por dos capas |
| E4 | D6 omite el StructureDefinition contenido | baja | Aceptado: entra en la evidencia de la pérdida |

## 2 v6. Plan (después de la ronda 2 de la serie nueva)

§2 v5, más:
1. **Owner en el slicing (E1):** el dueño pasa por `checkMember`, `checkChildren` y `checkChildValue`. En el límite de un
   recurso que otro tiene, el dueño cambia.
2. **Scope de un tipo de datos (E2):** el Scope canónico de su `Owner`, según la sonda de HL7. La cadena de `resolve()`
   conserva el suyo para la navegación.
3. **Identidad entre capas (E3):** solo para juntar informes entre capas. `pendingConstraints` y `firstReport` quedan
   como están, y `dropReported` conserva la identidad. Sondas de HL7 antes de cambiar una salida.
4. **D6 (E4):** el StructureDefinition contenido entra en la evidencia de la pérdida.

**Mutaciones nuevas:**
- `Owner` sin propagar en el slicing;
- un tipo de datos con el Scope de la cadena;
- `dropReported` sin la identidad entre capas.

## 3f. Serie nueva, ronda adversarial 3, la última (Codex, 14-11-2026)

E1, E2, E4 y lo que quedaba de D1 están resueltos en el diseño, sujetos a las sondas de HL7 y a los tests de `Owner`.
No hay hallazgos nuevos de severidad media o mayor. Queda uno:

| # | Hallazgo | Sev. |
|---|---|---|
| F1 | D3 y E3 siguen abiertos. La supresión actúa antes de la identidad nueva: `pendingConstraints` (`constraint/tree.go:603`) y `firstReport` (`report_scope.go:46`) comparten `seen` entre las definiciones que gobiernan y la raíz del perfil de tipo (`tree.go:54`, `:75`). Si dos capas fallan una invariante con la misma clave y expresión pero distinta severidad o texto, la segunda se pierde antes de juntar: un warning puede tapar un error | media |

**Propuesta (v7):** la supresión actual se mantiene dentro de cada capa, pero cada capa elegida tiene su propio estado
de evaluación e informe, también para los descendientes. Las definiciones que gobiernan y son comunes a todas las capas
se evalúan una vez, antes de las capas. Después se juntan los diagnósticos con la identidad nueva, que `dropReported`
conserva. Tests:
- las dos alternativas fallan;
- orden inverso de los candidatos;
- severidad o texto distintos;
- ele-1 heredado idéntico;
- regresión con un solo perfil.

La serie no converge por un hallazgo medio con una corrección concreta. Se consulta al usuario (tope de 3 rondas).

**Decisión del usuario (14-11-2026): aprobar v7.**
- El plan aprobado es §2 v4 a v6, con la corrección de F1 (§3f).
- Sigue el paso 5: primero las sondas de HL7, con su log de consola revisado por errores de snapshot. Después los tests,
  que tienen que fallar antes de la implementación.

## 5. Sondas del paso 5 (14-11-2026)

Evidencia en `evidence/2026-11-14-type-profile-layers/probe/`: 155 instancias en R4, R4B y R5, con los logs de consola
limpios de errores de snapshot. Generador `gen.py`; la tabla y los hallazgos están en el README. p1 a p21 coinciden en
las tres versiones, salvo `mtqy-1` en R5.

**Cambian el plan:**
- **E2 (sonda 11).** Al decidir un discriminador por perfil, con o sin `resolve()`, HL7 evalúa las invariantes del
  perfil de un tipo de datos con **el propio tipo de datos** como `%resource` y `%rootResource`. Con `type.profile`
  simple, usa el recurso dueño. Pasa igual en las tres versiones. La especificación (R4 `fhirpath.html`, verificado
  14-11-2026) dice: «%resource // the resource that contains the original node that is in %context». El Scope canónico
  del dueño (§2 v6, punto 2) cumple la especificación, y HL7 no: es un defecto de HL7 en la evaluación de
  discriminadores. Por la regla de E2 se consulta al usuario.
- **E3/F1 (sonda 12).** HL7 junta las invariantes de dos capas con la misma clave y expresión aunque el texto difiera:
  un solo mensaje, con el texto de la primera capa declarada. Con severidad distinta no las junta. ele-1 heredado sale
  una vez (dos veces en R4B, porque el core R4B trae dos textos de ele-1). La especificación no dice nada sobre cómo se
  informa. Se sigue a HL7: la identidad entre capas es clave, expresión y severidad (sin el texto), y el mensaje lleva el
  texto de la primera capa declarada. El aislamiento por capa de F1 se mantiene, porque sin él la severidad distinta
  se pierde.

**Confirman el plan:**
- D4: un `Unknown_Profile` por perfil, un `NoMatch` y los errores de contenido base una vez.
- D6/E4: un ImplementationGuide o un StructureDefinition contenido dan `Unknown_Profile` en HL7. Es la pérdida
  conocida.
- A4: el resultado es igual en los dos órdenes.
- D2: un `REF_CantMatchChoice` por capa.
- A5: se informa por nodo.
- Criterio v4: `%resource` es el recurso que contiene el valor.
- (c): el mínimo del slice se informa aunque se elija otra capa.

**Decisión del usuario (14-11-2026): E2 sigue la especificación.** Al decidir discriminadores, un tipo de datos se
evalúa con el Scope canónico de su recurso dueño.
- **Divergencia declarada en hl7diff:** las filas `s` y `h` de la sonda 11.
- **Valor esperado de esas filas:** el que la especificación da, igual a la columna `t` (el recurso dueño); no el de
  HL7.
- **HL7:** se redactan un issue y un PR con la evidencia; los publica el usuario.

### Tests (paso 5, 14-11-2026, Sonnet)

`pkg/validator/type_profile_layers_test.go`: 19 tests `TestLayers*` y `TestScopeInvariantOnTheTypeProfileLayerProbes`,
con los datos de prueba en `testdata/type_profile_layers/`. Las sondas por llamador están en
`evidence/…/probe-callers/`, y los valores esperados los genera `evidence/…/tests/gen_expected.py` desde las salidas de
HL7.
- **Antes de implementar:** fallan 279 subtests y pasan 522. Los que pasan son la regresión con un solo perfil, los
  controles por llamador, el orden y el invariante. No sobra ningún error de gofhir: solo faltan.
- **E2:** gofhir ya sigue la especificación en las filas `s` y `h` desde #215, así que esos tests son guardas y no fallan.
  En `h`, el dueño del tipo de datos es el destino resuelto (o2), no o1: lo que dice la especificación.
- **Valores que salen de la especificación y no de HL7:**
  - `kucum`: HL7 con `-tx n/a` solo da un warning;
  - las filas `s` y `h` de E2.
- **Huecos:**
  - `walker/discover.go` no tiene una regla exclusiva de capa, porque ningún tipo de datos de R4 tiene recursos;
  - B2 (`bcyc*`) es débil, porque HL7 no informa nada;
  - D5 queda como esqueleto: los contadores los agrega quien implementa.

## 6. Implementación — diseño concreto (14-11-2026, Sonnet)

**Registro (`pkg/registry`).**
- `Instance{Value, Self, Owner}`: el valor, la definición que declara para sí y el mapa del recurso que lo contiene (nil: el
  de la validación, que el elector toma del Scope raíz del ctx). `ChildrenOf` recibe `Instance` en vez de `self`.
- `Layer{SD, Nodes}` y `InstanceChildren{SD, Nodes, More []Layer, Unresolved, AllUnresolved}`: `SD` y `Nodes` son los de la
  primera capa (llamadores (b)), `More` las siguientes; `NumLayers()` y `LayerAt(i)` evitan reservar memoria con cero o un
  perfil. `MemberLayers` es el `MemberTree` de los llamadores (c): las capas de un slice sin hijos propios.
- `typeProfilesOf` acepta una declaración de varios perfiles (`TypeProfiles.Alternatives`): el primer nodo de la cadena
  `SliceOf` con algún perfil usable (que se resuelve y es del tipo del valor) gobierna; sin ninguno usable queda la primera
  declaración múltiple, solo para informar `NoMatch` (D4). Los que no se resuelven van a `Unresolved`.
- Elector: `ProfileChooser` (interfaz) se inyecta en ctx (`WithProfileChooser`); `ChooseTypeProfiles` pregunta por cada
  candidato (sin elector o sin valor: todos, `Decided` false) y devuelve `Chosen` (los que cumplen, o todos) y `Met`. Sin
  memo propio: `Conforms` memoiza. Primitivos fuera (pieza 3).
- La implementación (`pkg/validator`, `typeChooser`) toma el Scope de `Owner` con `scopeindex.Resolve` (sin dueño, el de
  `rootScopeKey`) y llama a `conformer.Conforms`.

**Cómo llegan las capas a cada llamador.**
- (a) cardinalidad, extensión (`extension.go` y `context.go`), binding, referencias, UCUM, slicing (`tree.go`), holders y
  `walker/discover.go` iteran las capas con `NumLayers`/`LayerAt`. Para lo que informan, con una clave por valor, a partir de
  la segunda capa se descarta lo que repite ID, severidad, ruta y diagnóstico de lo informado por las anteriores
  (`issue.Result.DropRepeated`). `Owner` se propaga como un parámetro `owner` por cada recorrido: el recurso de la raíz que
  se camina (cada recurso que otro tiene es su propio dueño; el slicing lo toma de `scope.Resource`, en `checkMember`,
  `checkChildren` y `checkChildValue`).
- Holders: `groups` pasa a `map[uintptr][][]string` (un grupo por capa u origen, sin repetir), y cada grupo se elige y se
  exige por separado.
- (b) siguen con la primera capa. (c) `checkChildren` compara contra las capas de la base: omite el min/max del hijo del
  slice si **alguna** capa lo tiene igual.
- Constraint: `walk` pide `ChooseTypeProfiles` para el valor. Con una capa de tipo se comporta como hoy. Con varias evalúa,
  una vez y antes, las definiciones comunes (cadena del nodo, contentReference, definición propia) y después cada capa con
  su propio estado (F1): una copia de `seen`, un `reportScope` hijo (lee el del padre, escribe el suyo) y un `issue.Result`
  propio, con la identidad de cada invariante (ruta, clave, expresión, severidad). Después se juntan: lo de la misma
  identidad en otra capa se descarta (el texto queda el de la primera capa declarada), y lo que ya informó el padre antes de
  las capas no se repite. `NoMatch` (error) y `MultipleMatches` (warning) se informan en el valor, una vez, con
  `firstReportKey`.

**Avisos.** `TYPE_PROFILE_NOT_FOUND` por cada perfil sin resolver del propio nodo (cardinalidad) o de la cadena (slicing);
`PROFILE_NO_MATCH` y `PROFILE_MULTIPLE_MATCHES` son IDs nuevos en `pkg/issue`.

**D5.** Dos contadores solo de pruebas en ctx, en `pkg/validator` (`probeKey`, `conformProbe`, sin API exportada): las
elecciones que se piden al elector («visit») y las comprobaciones de conformidad que corren de verdad, sin memo («check»);
el bench los informa con `b.ReportMetric` (`visits/op`, `checks/op`).

**Desvíos del diseño, hallados al implementar.**
- `Instance.Self` es una `registry.DefinitionSource` (interfaz) y no una función: con una función, pasar la `Instance` al elector
  hacía que cada llamada creara un cierre en el heap (+4 % de asignaciones con cero o un perfil).
- El elector comprueba cada candidato con `conformer{profileOnly: true}`: sin las fases estructural, de primitivos y de UCUM,
  que juzgan lo que el valor tiene sea cual sea el perfil y se informan una vez (sondas d4c y kucum: HL7 no cuenta esos errores
  como un perfil no cumplido). La clave del memo de `Conforms` lleva ese modo.
- `MemberLayers` suma a las capas del slice las de los perfiles que el propio slice declara, también cuando tiene hijos propios
  (sonda reslc); con un solo perfil sigue como `MemberTree`.
- La cardinalidad, con varias capas, informa primero los conteos de los hijos propios de cada capa y después baja a los valores
  (el orden en que HL7 los informa, del que depende la comparación de los tests con la ubicación de un hijo).

### Después de la implementación (14-11-2026)

Quedan 12 subtests en 4 casos, cada uno en tres versiones. Los verifiqué:
- **p14 y p15:** son primitivos (`valueString`) y pertenecen a la pieza 3. Salen de este grupo y pasan a los tests de la
  pieza 3, con su evidencia.
- **kctx: brecha real de la implementación.** HL7 informa el contexto de extensión, el mínimo de `unit` y `NoMatch`;
  gofhir solo el contexto. `Conforms` sobre un tipo de datos no evalúa el contexto de extensión, porque le falta la
  ubicación del valor, y entonces da por cumplida la primera capa. Se corrige: la comprobación de un tipo de datos
  recibe su ubicación.
- **bcyc3: la sonda no sirve para B2, por una diferencia con HL7 que ya está en `main`.** HL7 no informa el perfil de
  destino de `Identifier.assigner` hacia un contenido que no cumple, ni con un solo perfil de tipo (bcyc5) ni con el
  `targetProfile` puesto directamente en el perfil de Observation (bcyc7). gofhir en `main` y en la rama sí lo informa.
  La evidencia está en `probe-callers/hl7-target-in-datatype/`. El valor esperado de bcyc3 pasa a ser el de la
  especificación: `NoMatch` con los errores de cada capa, que es lo que gofhir da. La diferencia con HL7 queda pendiente
  aparte, para minimizarla y reportarla.
- **D5:** los chequeos crecen lineales en componentes, alternativas y profundidad (2/20/200, 20/40/80, 2/4/8/16). Las
  visitas crecen más que lineales con la profundidad (12, 34, 108, 376), y hay que explicar por qué.

**Resultado de D5 (14-11-2026): NO se cumple la cota.** `BenchmarkTypeProfileLayersScale/depth-all-layers=D`: dos perfiles de
Identifier que se declaran entre sí en `assigner.identifier`, con un valor que cumple los dos en cada nivel (todas las capas
elegidas). Los chequeos de conformidad siguen lineales (2D: 2, 4, 8, 16, 24), pero las visitas al elector (12, 56, 400, 8000,
130800 para D = 1, 2, 4, 8, 12) y el tiempo (0,12 ms, 0,23 ms, 0,93 ms, 16 ms, 262 ms) crecen como 2^D. La causa: cada llamador de la
categoría (a) baja por los hijos de cada capa, y cuando dos capas declaran el mismo hijo con las mismas alternativas, el
valor hijo se camina una vez por capa en cada nivel. Con una sola capa elegida por nivel (`depth=D`) el crecimiento es
cuadrático, no exponencial (cada comprobación de un nivel camina el subárbol de debajo). Sigue: volver al paso 2 (el
recorrido por capas debe caminar cada valor hijo una vez, con la unión de las capas que sus padres le dan).

## 2 v8. Cada valor se recorre una sola vez (14-11-2026, aprobado por el usuario)

**El problema.** Cada llamador (a) baja por los hijos de cada capa elegida. Si dos capas declaran el mismo hijo, el valor
hijo se recorre una vez por capa en cada nivel, y eso da 2^D (D5).

**Cambio de diseño:**
- **Agrupar por valor hijo.** Para cada valor padre, los hijos de todas las capas elegidas se agrupan por valor hijo
  (identidad del map o de la instancia). Cada valor hijo queda con la lista de pares (capa, nodo hijo) que lo
  gobiernan.
- **Chequeos propios de cada par:** conteos de cardinalidad, invariantes con el estado por capa de F1, binding y min/max
  de slice. Corren una vez por par, sin bajar.
- **Un solo descenso por valor hijo,** con la unión de los nodos que lo gobiernan. Es lo mismo que la constraint ya hace
  con varias definiciones que gobiernan (`layer()` con `seen`). La elección de perfiles del hijo se calcula una vez por
  valor, porque `Conforms` memoiza por (perfil, valor).
- **Cota:**
  - Las visitas son lineales en (valores × nodos que los gobiernan).
  - Con una sola capa, el costo cuadrático en la profundidad viene de las comprobaciones especulativas anidadas: cada
    `Conforms` recorre su subárbol una vez por (perfil, valor). Ese costo no cambia, se mide contra `main` y se declara.

**Tests y mutaciones nuevas:**
- el bench `depth-all-layers` pasa a exigir visitas lineales en D;
- un test donde las dos capas declaran el mismo hijo con reglas distintas: las dos reglas se informan y el hijo se
  recorre una vez (contador);
- la mutación «un descenso por capa» tiene que fallar.

### Implementación de v8 (14-11-2026)

- `registry.Descended` (en `instance_children.go`): por valor, las capas (definición y primer elemento) bajo las que ya se lo
  recorrió. `Fresh(valor, capas)` deja solo las nuevas y las anota; `Below(capas)` da el registro que se comparte con todo
  el subárbol desde el primer valor con varias capas (nil antes: una sola capa no cambia nada, ni el orden de los informes).
  Se usó en cardinalidad, binding, referencias, UCUM, slicing, holders, discover y extensión (valores y contextos): cada
  par (capa, hijo) corre sus chequeos propios, y el descenso es uno por valor con la unión de las capas.
  Un registro por llamada de `walk` no alcanza: dos capas del padre con hijos distintos (`idc.assigner`, `idd.assigner`)
  son dos recorridos del mismo valor, así que el registro se comparte hacia abajo.
- Constraint: `descended` en `constraintEvalOpts` (`firstDescent`, `layers.go`), activo desde el primer valor con varias
  capas; guarda los tres `walkChildren` de `walk` y `walkLayers`.
- Medida (`depth-all-layers=D`, visitas al elector, D = 1, 2, 4, 8, 12): 12, 56, 400, 8000, 130800 antes; 12, 56, 264, 1160,
  2696 después. Tiempo en D = 12: 262 ms antes, 6,3 ms después. Las comprobaciones siguen en 2D. El costo es cuadrático en D
  (cota de 24·D²), no lineal: cada comprobación especulativa de un nivel camina el subárbol de debajo, y eso solo ocurre al
  elegir entre dos o más perfiles (no hay equivalente en `main`).
- Cadena de un solo perfil (`ids` se declara a sí mismo; `main` y rama, ms/op en D = 8, 16, 32, 64): 0,46, 0,40, 0,89, 2,45
  en `main` y 0,49, 0,40, 0,95, 2,46 en la rama: iguales, sin elección y sin crecimiento cuadrático.
- Tests: `TestLayersAllLayersChosenAtEveryLevelIsPolynomial` (visitas ≤ 24·D², comprobaciones = 2D, cociente D = 12 / D = 8
  ≤ 4) y `TestLayersSameChildWithDifferentRulesIsReportedByBothAndWalkedOnce` (las dos reglas, 632 visitas con v8 y 1896 con el
  recorrido por capa; tope 900). Con `Fresh` y `firstDescent` desactivados (recorrido por capa) los dos fallan; restaurados, pasan.

## 7. Validación (14-11-2026)

- **hl7diff contra `main`:** 61 grupos livianos PASS y `r4-core-examples` PASS. El corpus de hl7diff no tiene valores
  con varios perfiles de tipo, así que esto prueba que no hay regresión con cero o un perfil.
- **Las divergencias declaradas** (p12, p20, p21 y las filas `s` y `h` de E2) se comparan en `type_profile_layers_test.go`
  contra las salidas de HL7, y cada excepción tiene su cita. No pasan por hl7diff, porque sus sondas no están en su corpus.

**Mutaciones (14-11-2026, en una copia): 26 de 38 muertas.**
- **Equivalentes en la práctica:**
  - 3g: UCUM depende solo del elemento Quantity, igual en todas las capas;
  - 15b: `definedIn` solo con la primera capa, y `DropRepeated` saca los repetidos.
- **Huecos del oráculo, que se cierran con tests nuevos:**
  - 3c y 3d, extensiones: hace falta un valor que no cumple ningún perfil, o los cumple los dos, y una regla de
    extensión que solo tiene B;
  - 13a, 13b y 13c, `Owner` en el slicing: un miembro de slice dentro de un contenido o de una entrada, con una
    invariante sobre `%resource`;
  - 14, contextos de extensión de un tipo de datos con `vs.scope`: un contexto que depende de `%resource`, en un
    discriminador, dentro de un recurso anidado;
  - 5b, `RepeatKey` sin severidad: dos capas con el mismo ID, ubicación y texto, desde una fase que no es constraint, con
    severidades distintas;
  - 10, `SliceOf`: un slice sin `type.profile` propio, cuyo elemento cortado declara dos perfiles.
- **Inalcanzables con los tipos del core:** 3b, 16a, 16b y 3h. Hacen falta un elemento con capas que tenga recursos o
  que descubra perfiles, y ningún tipo de datos de R4, R4B o R5 los tiene. Se prueban con un tipo de datos sintético.
  Como HL7 no carga tipos de datos propios, el valor esperado sale de la especificación («at least one», y cada grupo de
  holders exigido por separado).

**Mutaciones, segunda pasada (14-11-2026): los huecos, cerrados o probados equivalentes.**
- **Tests nuevos.** 5b: `pkg/issue/drop_repeated_test.go` (la severidad entra en `RepeatKey`). 10: `pkg/registry/type_profile_chain_test.go`
  (un slice sin tipo propio hereda los varios perfiles del elemento que divide; el generador de snapshots copia el tipo al slice, así
  que solo un snapshot escrito a mano lo alcanza). 3b, 16a y 16b: `TestLayersSyntheticDatatypeThatHoldsAResource`, con un tipo de datos
  sintético (`MyThing`, con `res` que tiene un recurso, `v[x]` y `req`) y dos perfiles; el valor esperado sale de la especificación
  (`type.profile`: «at least one»; cada grupo de holders se exige por separado), porque HL7 no carga tipos de datos propios. 3h:
  `TestLayersSyntheticDiscoverFindsAResourceOnlyTheSecondLayerHolds` (el recurso de `v[x]` solo lo tiene la segunda capa).
- **Sondas HL7 nuevas** (`evidence/2026-11-14-type-profile-layers/probe-gaps/`, 7.0.4, R4, R4B y R5, sin errores de snapshot; las
  carga `tests/mk_testdata.sh`): `gtctx`, `gtval` y `gtok` (3c y 3d: un Timing con dos perfiles que restringen `bounds[x]` a tipos
  distintos, sin cumplir ninguno, con una extensión en `boundsRange`; HL7 informa el contexto y el valor una vez por capa, y gofhir
  una vez en total) y `s13a*`, `s13b*`, `s10*` (13a a 13c y 10: contenido y entrada de Bundle, «amended» y «final» en cada orden;
  HL7 elige por el `%resource` del dueño en el slice, en un hijo de un miembro de slice y en el elemento que el slice divide).
  Todas coinciden con gofhir. Clasificación nueva: `type-not-allowed` (`Extension_PROF_Type`, `Extension_EXT_Type`).
- **Equivalentes en la práctica, medido con instrumentación en una copia** (no hay test que las mate sin cambiar el código):
  - 14 (`vs.scope` en vez de `vs.at.scope`): `conformer.check` recibe el mismo `Scope` que `typeChooser.Conforms` pone en
    `datatypeAt`; 0 diferencias en 2293 llamadas de la suite de `pkg/validator`;
  - 13a, 13b y 13c (el `Owner` del slicing): la elección entre perfiles se memoiza por valor y perfil (`conformState`), y la fase de
    cardinalidad, que corre antes, la hace con el dueño correcto; pasar `nil` da las mismas capas en 4268 llamadas (`checkMember`,
    `checkChildValue`: gobierno y base), también en las sondas nuevas de contenido y de entrada;
  - 3c y 3d (extensión, una sola capa): lo que la fase lee de los nodos de cada capa (id, ruta, ruta base, tipos, hijos) es igual
    en las capas de un mismo valor, y los valores salen de los tipos base; 0 diferencias en las visitas con varias capas.
  Si se simplifica el código, esos tres parámetros (`Owner` del slicing, `vs.scope` y las capas siguientes de la fase de extensiones)
  son los candidatos; mientras estén, los tests de arriba cubren lo que se puede observar.
- **Defecto hallado (no corregido).** `walker.Discover` halla dos veces el recurso que tienen dos capas elegidas en el mismo elemento, y se
  valida dos veces (cada informe sale doble). Solo se alcanza con un tipo de datos con un elemento que tiene un recurso, con los
  perfiles de tipo declarados en la definición del tipo del recurso (un perfil no suma elementos que tengan recursos); el core no
  tiene ninguno. El test está escrito y en `t.Skip` (`TestLayersSyntheticDiscoverFindsAResourceBothLayersHoldOnce`) hasta que se
  corrija (deduplicar por identidad del mapa en `Discover.found`).
- **Defecto corregido (14-11-2026).** `walk` deduplica por identidad del mapa los recursos que halla cuando el nodo tiene más de una capa (sin conjunto con una sola); se quitó el `t.Skip` y el test pasa. Benchmarks antes/después: mismas allocs y bytes por operación.

**Bench contra `main` (14-11-2026, `-count=6`):** el tiempo no cambia de forma significativa. Las corridas de `main` tuvieron
mucha varianza, por carga simultánea de la máquina. Las asignaciones suben entre 0,1 % y 0,4 % con cero o un perfil
(Patient: de 1.759 a 1.760; Batch: de 2.455 a 2.464). Las salidas están en el scratchpad, `p2-impl/bench-*.txt`.

## 8. Code review

### Ronda 1 (Codex, 14-11-2026)

Los reproductores están en la copia de revisión: `.review/round1_test.go` y `round1-*.txt`.

| Hallazgo | Sev. | Resolución |
|---|---|---|
| El descenso compartido suprime errores distintos de perfiles que no se resuelven. `Fresh` vuelve antes de informar `AllUnresolved` (`cardinality.go:136`): con dos alternativas de Identifier que declaran perfiles distintos y faltantes en `assigner`, solo se informa `missing-a` | media | **Aceptado** (reproducido). Los no resueltos se informan antes de filtrar las capas ya recorridas. Test con el valor de HL7: una sonda en las tres versiones, en la que D4 da un `Unknown_Profile` por perfil |
| `Discover` sigue duplicando a través de capas intermedias: el conjunto de identidades es de una sola invocación (`discover.go:93`). Dos perfiles envoltorio llegan por invocaciones distintas a la misma Observation | media | **Aceptado** (reproducido). La deduplicación por identidad vale para todo el descubrimiento. Test sintético con el valor que da la especificación |
| (a) Las mutaciones 13a a 13c (Owner en el slicing) **no son equivalentes**: el memo caliente las tapa, y es una dependencia oculta del orden de las fases. Con el slicing corrido en frío, s13ac1 y s13bc1 dan un `SLICING_CARDINALITY_MIN` de más | media | **Aceptado.** El código actual está bien; se agregan tests del slicing en frío, con el memo vacío, que matan las tres mutaciones |
| (b) 3c y 3d: no hay entrada que las mate. Son equivalentes para snapshots válidos, por inferencia y sin prueba | — | Se registra como equivalencia razonada. La 14 es equivalente por construcción (`slicematch.go:219`, `:392`) |

### Correcciones de la ronda 1 (14-11-2026)

Salidas en el scratchpad `p2-fix1/` (`unk-before.txt`, `unk-after.txt`, `discover-before.txt`, `discover-after.txt`, `mutant-*.txt`, `make-check.txt`).

1. **Cardinalidad.** `validateNode` (`pkg/cardinality/cardinality.go`) informa los perfiles que no se resuelven (`AllUnresolved`) antes de quitar las capas ya recorridas (`Fresh`); con un registro `done`, lo que ya se informó al caminar el valor por otra capa se descarta con `DropRepeated(0, before)`. Sonda nueva `unk` en `probe-gaps` (`gen.py`: `id-ua`, `id-ub`, `obs-unk`) corrida con HL7 7.0.4 en R4, R4B y R5 (sin errores de snapshot en las consolas): un `Unknown_Profile` por perfil, y `profile-no-match` en el identifier. `gen_expected.py` ahora quita de HL7 el sufijo «(validating against ...)» del detalle del perfil faltante. Test: `TestLayersUnresolvedProfilesOfEveryLayerAreReported`; antes de la corrección faltaba `missing-b` en las tres versiones.
2. **`walker.Discover`.** El conjunto de identidades pasó de cada invocación a `Discovery.held`, de todo el descubrimiento: nace con el primer valor de varias capas (con una sola capa por todo el camino no se asigna nada) y conserva el orden del primer hallazgo. Test sintético `TestLayersDiscoverResourceReachedThroughNestedLayersOnce` (dos perfiles de MyWrap que declaran perfiles de MyThing distintos para `inner`; ElementDefinition.type.profile: «at least one of them», y la instancia tiene un solo `res`): antes daba 2 recursos, ahora 1.
3. **Slicing en frío.** `TestLayersSlicingColdHasTheOwner` (s13ac1, s13bc1, control s10c1, en R4, R4B y R5) corre el slicing con un memo de conformidad vacío. En un worktree de prueba aparte (ya eliminado), con Owner quitado en `checkMember` (13a), en `checkChildValue` (13b) y en la propagación a `checkChildren` desde `checkMember` (13c) el test falla; sin mutación pasa. Sobreviven tres variantes que no se pedían (la propagación en `slicing.go` líneas 202 y 221, y `walkChildren` de `tree.go`): no hay entrada de las sondas que las distinga, igual que 3c y 3d.

**Mutaciones completas antes de la ronda 2 (14-11-2026): 41 de 47 muertas.**
- **Vivas y conocidas como equivalentes:** 3c, 3d, 3g, 14 y 15b.
- **Vivas y nuevas:**
  - 17a: `childrenOf` sin la reserva de la definición del tipo cuando ninguna alternativa se puede usar;
  - 18b: `walkLayer` descarta los contextos de extensión que devuelve `extensionsOf`.

  Las dos van a la ronda 2.

### Ronda 2 (Codex, 14-11-2026)

Las tres correcciones de la ronda 1 están bien y completas. Sin defectos nuevos de producción. Quedan dos huecos de los
tests, de severidad media:

| Hallazgo | Resolución |
|---|---|
| **17a es un hueco real.** Con dos perfiles de Annotation que no se pueden usar en `Observation.note` y una nota sin `text`, el código da `CARDINALITY_MIN` en `note[0].text`, y la mutación lo calla | **Aceptado.** Test con el valor de HL7 |
| **Las tres variantes del Owner en el slicing son huecos reales:** `slicing.go:202`, `:221` y `walkChildren`. Hay alternativas de Coding más hondas que dependen de `%resource.status`, en un contenido `final` bajo un recurso `amended` | **Aceptado.** Tests en frío con esas entradas. Valor esperado: el de la especificación, el Scope del dueño (decisión E2). HL7 toma el propio tipo de datos como `%resource` al decidir un discriminador, así que solo se usa su salida donde no hay discriminador |
| 18b ya lo matan `TestExtensionContextOfUse` y `TestSubExtensionInItsExtensionContext` | Se suman al filtro de las mutaciones |

Codex no pudo correr la suite completa del validador, porque su sandbox no permite abrir un puerto.

### Correcciones de la ronda 2 (14-11-2026)

Salidas en el scratchpad `p2-fix2/` (`make-check.txt`). Solo tests y evidencia; sin cambios de producción.

1. **17a.** Sonda `un17` en `probe-gaps` (`gen.py`: `obs-un17`, dos perfiles de Annotation que no existen en `Observation.note`, y una
   nota sin `text`), corrida con HL7 7.0.4 en R4, R4B y R5 (consolas sin errores de snapshot): `CARDINALITY_MIN` en `note[0].text`
   y un `Unknown_Profile` por perfil. `TestLayersNoUsableProfile` suma `un17`. Con la mutación 17a el test falla en las tres versiones.
2. **Owner del slicing más hondo.** Sondas `dpinc1`, `dpprc1` y `dpwkc1` (Observation de arriba «amended», contenido «final»;
   alternativas de Coding en línea, en un perfil de CodeableConcept, y dos perfiles de CodeableConcept con un slice `coding:c`
   obligatorio de código distinto). `TestLayersSlicingColdHasTheOwner` las corre en frío: el slicing no informa nada bajo
   `contained[0]` (valor de la especificación y decisión E2; HL7 tampoco informa nada, así que no hay divergencia). Mutaciones en
   un worktree de prueba (ya eliminado): `slicing.go:202` muere en `dpinc1`, `:221` en `dpprc1` y `walkChildren` en `dpwkc1`,
   en las tres versiones. Con las cuatro, las 4 mutantes de la ronda 2 mueren.
3. 18b: `TestExtensionContextOfUse` y `TestSubExtensionInItsExtensionContext` se suman al filtro de las mutaciones.

**Mutaciones antes de la ronda 3 (14-11-2026).** Desde la corrida completa anterior (41 de 47) solo cambiaron tests y datos de
prueba, no el código de producción. Agregar tests no puede revivir una mutación muerta, así que esa corrida sigue
valiendo y solo se repiten las vivas:
- 17a y las tres variantes del slicing mueren con las sondas `un17`, `dpinc1`, `dpprc1` y `dpwkc1`;
- 18b muere con `TestExtensionContextOfUse` y `TestSubExtensionInItsExtensionContext`, que ahora están en el filtro.

Quedan vivas solo las equivalentes conocidas: 3c, 3d, 3g, 14 y 15b.

### Ronda 3, la última (Codex, 14-11-2026)

Sin defectos de producción de severidad media o mayor. Los reproductores de las rondas 1 y 2 pasan. Queda un hueco de
los tests, de severidad media:
- **15b no es equivalente.** Con dos capas de CodeableConcept compartidas, donde solo la segunda asigna un perfil de
  Coding que exige `display`, la mutación de `definedIn` (solo la primera capa) agrega un `SLICING_CARDINALITY_MIN`.
  `DropRepeated` no lo saca, porque el ID es otro. Codex dejó el reproductor en `.review/round3_test.go`.
- **3c y 3d:** siguen sin poder distinguirse. Su equivalencia es razonada, no probada.

Por la regla de tres rondas, se consulta al usuario.

**Decisión del usuario (15-11-2026): agregar el test de 15b y cerrar, sin cuarta ronda.**

### Corrección de la ronda 3 (15-11-2026)

Solo test y evidencia; sin cambios de producción. Salida en el scratchpad `p2-fix3/` (`make-check.txt`).

1. **15b deja de ser equivalente.** Sonda `d3` en `probe-gaps` (`gen.py`: `obs-d3`, dos perfiles de CodeableConcept que fallan
   una invariante de la raíz, compartidos por `component.code` y por el slice `component:s.code`; solo el segundo asigna un
   perfil de Coding con `display` obligatorio). El patrón del slice es por `coding`, porque HL7 no evalúa un discriminador por
   `text`. HL7 7.0.4 en R4, R4B y R5 (consolas sin errores de snapshot): el no-match de perfil, la invariante y un solo mínimo
   de `display`, sin el del slice. Los valores esperados salen de `mk_testdata.sh` y `gen_expected.py`; `d3` corre en
   `TestLayersSliceChildAgainstTheLayers`.
2. **Mutación 15b** (`definedIn` con `c.SD == sd`, en un worktree de prueba ya eliminado): `d3` falla en las tres versiones por
   un `SLICING_CARDINALITY_MIN` de más. Con producción, pasa.
3. Con esto, 15b se cuenta como muerta. Quedan vivas solo 3c, 3d, 3g y 14 (equivalencia razonada, no probada para 3c y 3d).

## 9. Cierre (15-11-2026)

Todo sobre el código final:
- `GOTOOLCHAIN=go1.25.13 make check` pasa.
- hl7diff contra `origin/main`:
  - grupos livianos: sin hallazgos;
  - `r4-core-examples`, 5.301 archivos: sin hallazgos ni errores sin explicar.
- Mutaciones: mueren todas, salvo 3c, 3d, 3g y 14, que son equivalentes por razonamiento.
- Code review: tres rondas de Codex. Ninguna dejó defectos de producción sin resolver, y los huecos de los tests quedaron cerrados.

**Pendiente fuera de esta pieza:**
- Issue a HL7 por E2. Borrador en `evidence/2026-11-14-type-profile-layers/hl7-issue-discriminator-resource.md`, sin publicar.
- La diferencia de HL7 con el perfil del destino de `assigner` hacia un recurso contenido (sondas bcyc5 y bcyc7).
- D6: un ImplementationGuide o un StructureDefinition contenido como perfil.
- Pieza 3: primitivos (p14, p15 y s22).
