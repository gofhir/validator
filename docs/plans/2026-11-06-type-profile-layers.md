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
