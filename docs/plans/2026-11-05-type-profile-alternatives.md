# #193, incremento 1: `type.profile` con varios perfiles (un O)

Issue: [gofhir/validator#193](https://github.com/gofhir/validator/issues/193), reproducción 4. Rama
`fix/type-profile-alternatives` (desde `700d1c3`). Evidencia:
[evidence/2026-11-05-type-profile-alternatives/](evidence/2026-11-05-type-profile-alternatives/).

Decisión del usuario (05-11-2026): #193 va en tres incrementos. Este es el primero; el 2 (conjunto de definiciones
gobernantes en la fase de constraint) y el 3 (en la de cardinalidad) tienen su propio plan.

## 1. Revisión previa

### Especificación

- R4 `elementdefinition-definitions.html#ElementDefinition.type.profile`: «Identifies a profile structure or
  implementation Guide that applies to the datatype this element refers to. If any profiles are specified, then the
  content must conform to at least one of them.» El mismo texto en R4B y R5.
- Es un O: el valor cumple si cumple al menos uno de los perfiles declarados para su tipo.

### Referencia: HL7 validator_cli 7.0.4

[probe/README.md](evidence/2026-11-05-type-profile-alternatives/probe/README.md). En R4, R4B y R5, con
`Observation.value[x]` = Quantity, `type.profile` = SimpleQuantity O MoneyQuantity:

- **Cumple alguno (p1, p2):** nada.
- **No cumple ninguno (p3, p5):** un error `Validation_VAL_Profile_NoMatch` («Unable to find a match for the specified
  profile among choices: …») y **los errores de cada perfil** (de SimpleQuantity: comparator máx. 0 y `sqty-1`; de
  MoneyQuantity: `mqty-1`, `mtqy-1` en R5), cada uno marcado «validating against <perfil>».
- **Un solo perfil (p4):** como hoy en gofhir.

### gofhir hoy (`700d1c3`)

- `registry.TypeProfile` devuelve el perfil solo cuando hay exactamente uno; con varios, ninguno (`type_profile.go`:
  «A type with several profiles is left to the caller»). Ningún llamador lo resuelve:
  - `constraint/tree.go` `typeLayer` cae al tipo base;
  - `registry/instance_children.go` `childrenOf` también.
- Resultado: con varios perfiles el valor se valida solo contra su tipo base. p3 y p5 pasan sin errores.

### Pieza reutilizable

`pkg/validator/slicematch.go`, `conformer.check`: corre el pipeline completo (`validateAgainstProfile`) sobre un valor
contra un perfil, en un resultado propio, con memo y manejo de ciclos (`conformState`). Lo usa el discriminador de slice
`profile`. Es exactamente «¿el valor cumple este perfil?».

### `gofhir/server` (congelado)

No trata `type.profile` múltiple (búsqueda en `internal/` y `pkg/`). Nada que tomar.

## 2. Plan

### Alcance

- **Registro:** `TypeProfiles(ctx, node, typeCode)` devuelve todos los perfiles declarados para el tipo, cada uno con su
  definición o su motivo de no uso, como `TypeProfile`. `TypeProfile` (uno solo) no cambia de comportamiento.
- **Regla, en un solo lugar** (la fase que hoy evalúa el valor contra su `type.profile`; el implementador ubica dónde,
  pero se informa una sola vez por valor y ubicación):
  - el elemento declara dos o más perfiles para el tipo del valor;
  - se prueba el valor contra cada uno con el mismo mecanismo de `conformer` (pipeline completo, resultado propio);
  - **si cumple alguno:** no se informa nada de los perfiles;
  - **si no cumple ninguno:** un error nuevo `PROFILE_NO_MATCH` en la ubicación del valor («Unable to find a match for
    the specified profile among choices: <canónicos>», como HL7), y los errores de cada perfil en sus ubicaciones, sin
    repetir los que la validación del tipo base ya informa en la misma ubicación y con el mismo ID (la deduplicación por
    clave que ya existe, `firstReportKey`).
- El recorrido normal del valor sigue bajo su tipo base, como hoy: no se elige un perfil para recorrer los hijos.
- Un perfil declarado que no se resuelve cuenta como no cumplido. Sondeado (p6–p8): HL7 lo informa como **error**
  `Validation_VAL_Unknown_Profile` aunque otro perfil se cumpla (p7), y con dos perfiles inexistentes informa además
  `NoMatch` (p8). gofhir hoy informa un perfil de tipo inexistente como warning `PROFILE_NOT_FOUND`: la severidad es una
  diferencia anterior, que se decide en la ronda adversarial (¿entra aquí o va a un issue?).

### Fuera del alcance

- Los incrementos 2 y 3 de #193: la definición propia de una extensión y `type.profile` bajo hijos desplegados por el
  perfil.
- `targetProfile` de Reference y canonical: es otro elemento con su propia regla.
- El texto «(validating against …)» de HL7 en cada error: hl7diff no compara textos; si se agrega, es otro cambio.

### Criterio de salida

- Tests en R4, R4B y R5 con p1–p5, con los valores esperados de las salidas HL7 guardadas:
  - presencia y ausencia de `PROFILE_NO_MATCH`;
  - los errores de cada perfil, por ID y ubicación;
  - que nada se repite.
  Los de p3 y p5 fallan antes del cambio.
- Casos que se sondean en HL7 antes de escribirlos:
  - un perfil que no se resuelve entre los declarados;
  - un tipo de elección donde solo uno de los tipos tiene varios perfiles;
  - una extensión cuyo `value[x]` declara varios perfiles;
  - un elemento repetido con un valor que cumple y otro que no.
- Mutaciones: quitar la regla, cumplir con todos en vez de alguno, no informar los errores de los perfiles, informarlos
  sin deduplicar, y tomar solo el primer perfil.
- `hl7diff run -heavy` contra `main` sin hallazgos nuevos. Inventario de `same` contado como multiconjunto: lo que cambia
  tiene su par en HL7.
- `make check` en verde. Bench del pipeline: medir el costo de las validaciones por perfil.

### Dependencias

Ninguna nueva.

### Sondas extra (05-11-2026, antes de la ronda adversarial)

[probe/README.md, «Casos extra»](evidence/2026-11-05-type-profile-alternatives/probe/README.md). Cambian el plan:

- **p8:** con todos los perfiles inexistentes, HL7 sí informa `NoMatch`; el plan decía lo contrario y se corrige arriba.
- **p12:** en un tipo de elección (Quantity con OR, o string), HL7 informa solo los errores de SimpleQuantity, sin
  `NoMatch` ni los de MoneyQuantity. Contradice p3 y la especificación («at least one of them»): parece un defecto de HL7
  al elegir el perfil en un elemento de elección. Hay que confirmarlo en el código de HL7 antes de decidir. Si lo es, se
  sigue la especificación y se reporta a HL7 con evidencia.
- p9 (elemento repetido) y p10 (extensión) se comportan como p3.

### Código de HL7 (org.hl7.fhir.core, `InstanceValidator.java`, en `883a25a16`)

- **Regla (`:8235`–`:8275`):** HL7 valida el valor contra cada perfil en una lista de mensajes propia.
  - **Cumple uno solo:** agrega los mensajes de ese perfil.
  - **No cumple ninguno:** error `VALIDATION_VAL_PROFILE_NOMATCH` (perfiles ordenados y unidos con «, ») y los errores
    de cada perfil, con « (validating against url|versión [nombre])».
  - **Cumple más de uno:** warning `VALIDATION_VAL_PROFILE_MULTIPLEMATCHES` y los mensajes de los que cumplen (p13).
  - **Perfil que no encuentra:** error `VALIDATION_VAL_UNKNOWN_PROFILE`, aunque otro se cumpla (p6–p8).
- **p12, explicado:**
  - con un tipo único, HL7 toma todos los perfiles (`:8031`);
  - con un elemento de varios tipos toma solo el primero (`:8072`, `t.getProfileList().get(0)`);
  - lo mismo hace con Element/BackboneElement (`:8085`).

  La especificación dice «at least one of them», sin distinguir. Es un **defecto de HL7**: aquí se sigue la
  especificación (todos los perfiles, también en un elemento de elección), se declara la divergencia en hl7diff con la
  sección de la especificación, y se reporta a HL7 con issue y PR de corrección (fork + fhir-test-cases), como #2828.
- **Decisión pendiente para la aprobación:** ¿informar el warning de varias coincidencias como HL7? La especificación no lo
  pide ni lo prohíbe. Recomendación: sí, para no tener una diferencia más con la referencia; es un warning.

## 3. Ronda adversarial 1 (revisor Opus independiente, 05-11-2026)

Codex estaba sin cuota (hasta las 16:21); el usuario eligió un revisor Opus independiente para esta ronda.

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| A1 | Ya existe el O de `type.profile` para recursos contenidos: `holderProfiles.Choose` (`pkg/validator/holder.go`) elige con `conformer.Conforms` y `holderDefinitions` (`constraint/tree.go:280–313`) recorre los elegidos como capas del walk, con `HOLDER_PROFILE_NOT_FOUND` (error), `HOLDER_PROFILE_NO_MATCH` y `HOLDER_PROFILE_MULTIPLE_MATCHES`, deduplicados con `firstReportKey`. Copiar issues de corridas aparte es otro diseño y crea dos IDs para la misma regla | alta | **Aceptado, verificado en el código.** Rediseño en §2 v2 |
| A2 | `conformer` guarda solo un bool; `validateAgainstProfile` sobre un Quantity da rutas relativas al tipo (`Quantity.comparator`); con un primitivo `Conforms` siempre es falso | alta | **Aceptado.** Con el rediseño, los errores salen del walk normal en sus rutas reales; `Conforms` solo elige. Los primitivos se tratan en §2 v2 |
| A3 | La deduplicación prometida no alcanza: `firstReportKey` es de la fase de constraint, y la base usa otro espacio de claves (`firstReport`) | media-alta | **Aceptado.** Con capas en el walk, la deduplicación es la que ya usan las capas y los recursos contenidos |
| A4 | Premisa falsa: con un solo `type.profile` inexistente, gofhir da error `TYPE_PROFILE_NOT_FOUND` (`cardinality.go:126–135`), no warning | media | **Aceptado, verificado.** La severidad ya coincide con HL7; p6–p8 siguen ese caso |
| A5 | No hay un lugar único: `TypeProfile` lo consultan `typeLayer` (constraint) y `childrenOf` (cardinalidad, extensión, slicing); `typeLayer` recorre `SliceOf` y `childrenOf` no | media | **Aceptado.** §2 v2 fija las dos fases y el `SliceOf` |
| A6 | p10 (extensión) depende de la definición propia de la extensión, que es del incremento 2 | media | **Aceptado:** se verifica antes de los tests si hoy llega por `selfDefinition`; si no, p10 pasa al incremento 2 |
| A7 | p12 sin decisión; faltan sondas: los dos perfiles se cumplen, canónicos versionados, candidatos derivados entre sí, un candidato de otro tipo; costo sin medir | baja | **Aceptado.** p12 y p13 ya están (§«Código de HL7»); las demás se sondean en el paso 5; bench con un Bundle de muchos valores |

## 2 v2. Plan (después de la ronda 1)

### Diseño

El O de `type.profile` de un valor de tipo de datos se resuelve **como el de un recurso contenido**, con las mismas
piezas:

1. **Elegir.** Los perfiles declarados para el tipo del valor (en el elemento y en los elementos de los que es slice,
   `SliceOf`, como `typeLayer`) se eligen como en `holderProfiles.Choose`:
   - los que se resuelven;
   - de ellos, a los que el valor se ajusta (`conformer.Conforms`);
   - si no se ajusta a ninguno, todos;
   - un perfil de otro tipo cuenta como no cumplido.
   Es una función común de `Choose`; no se duplica.
2. **Recorrer.** Los perfiles elegidos son las **capas** del valor: la fase de constraint (`typeLayer`) y la de
   cardinalidad (`childrenOf`) recorren los hijos del valor bajo cada uno. Los errores salen en sus rutas reales, con la
   deduplicación que ya tienen las capas. Con un solo perfil el comportamiento no cambia.
3. **Informar** en la ubicación del valor, una vez:
   - un perfil que no se resuelve: `TYPE_PROFILE_NOT_FOUND`, error, como hoy con uno;
   - si no se ajusta a ninguno: `PROFILE_NO_MATCH`, error;
   - si se ajusta a varios: `PROFILE_MULTIPLE_MATCHES`, warning, como HL7.
   Si los IDs de recursos contenidos (`HOLDER_PROFILE_*`) sirven tal cual, se reusan en vez de crear otros.
4. **Primitivos.** `Conforms` exige un map. Para un primitivo con varios perfiles se adapta para que valide el valor
   primitivo, o el incremento se detiene y vuelve al plan; no se informa `NoMatch` por no poder comprobar.
5. **Elementos de elección (p12):** todos los perfiles del tipo, como dice la especificación. Diferencia con HL7, que toma
   solo el primero: se declara en hl7diff con la sección de la especificación y se reporta a HL7 (issue y PR).

### Límites con los incrementos 2 y 3

Este incremento agrega a las dos fases la posibilidad de que un valor tenga **varias capas por su tipo**. Los
incrementos 2 y 3 suman a ese conjunto la definición propia de una extensión bajo hijos que despliega el perfil.

### Criterio de salida (reemplaza el de §2)

- **Tests en R4, R4B y R5** con p1–p5, p13, y p6–p9 y p12 en R4. Los valores esperados salen de HL7, salvo p12, que sigue
  la especificación. Se comprueba presencia y ausencia, por ID, severidad y ubicación, y que nada se repite.
- **Antes de los tests:** p10 (A6), un primitivo con dos perfiles, canónicos versionados, candidatos derivados entre sí y
  un candidato de otro tipo, sondeados en HL7.
- **Mutaciones:**
  - quitar la elección (recorrer siempre todos);
  - elegir siempre el primero;
  - no recorrer las capas en la fase de cardinalidad;
  - no informar `NoMatch` o `MultipleMatches`;
  - ignorar `SliceOf`.
- **`hl7diff`:** `run -heavy` contra `main` sin hallazgos nuevos, con p12 declarado. Inventario de `same` como
  multiconjunto, con cada cambio emparejado con HL7.
- **Costo:** bench del pipeline y de un Bundle con muchos valores de este tipo, antes y después.
- `make check` en verde.

## 3b. Ronda adversarial 2 (revisor Opus independiente, por elección del usuario, 05-11-2026)

No converge: 2 altos y 3 medios. Los altos, verificados en el código.

| # | Hallazgo | Sev. |
|---|---|---|
| B1 | `Choose` no sirve tal cual para un tipo de datos: compara `sd.Type` con `resourceType` (todos los candidatos serían «de otro tipo»), decide sobre los resueltos (p6: con uno resuelto lo elige sin `Conforms` y no informa NoMatch; p8: sin NoMatch), al contrario de HL7 | alta |
| B2 | `ChildrenOf` tiene llamadas en 11 paquetes (binding, slicing, ucum, reference, extension, walker, holder…), no solo cardinalidad. Con un perfil, todas lo ven; con capas solo en constraint y cardinalidad, un NoMatch puede deberse a un binding y salir sin el error que lo explica | alta |
| B3 | La cardinalidad no tiene ctx (`context.Background()`, `cardinality.go:281`), ni scope ni deduplicación; `Conforms` sin `conformState` crea uno nuevo; la clave del memo no lleva scope | media |
| B4 | `walk` y `childrenOf` toman los hijos del tipo solo si el nodo no despliega los suyos, con un solo recorrido: varias capas exigen un bucle por capa, y bajo hijos desplegados es terreno de los incrementos 2/3 | media |
| B5 | Primitivos: «adaptar o parar» no es un diseño (`Conforms` y `walkPrimitiveExtensions`) | media-baja |

**Lectura:** las capas por valor tienen que respetarlas **todos** los llamadores de `ChildrenOf`, que es la maquinaria
de los incrementos 2 y 3. El incremento 1 no es independiente de ellos. Se consulta al usuario cómo seguir.

## 2 v3. Plan: primero la maquinaria (decisión del usuario, 05-11-2026)

El incremento 1 de #193 construye las **capas de definiciones por valor** que respetan todos los llamadores, y el O de
`type.profile` es su primer uso. El incremento 2 suma a esas capas la definición propia de una extensión bajo hijos
desplegados; el 3 queda absorbido si el 2 cubre la cardinalidad (se decide al cerrar este).

### Relevamiento de llamadores (05-11-2026)

12 usos de `ChildrenOf` y `typeLayer`, sin caché:

| Semántica con varias capas | Llamadores |
|---|---|
| **(a) todas las capas:** cada hijo se controla bajo cada capa | `cardinality.go:281` (con `context.Background()`), `extension/context.go:146`, `extension/extension.go:187`, `binding/walk.go:22`, `slicing/tree.go:81`, `reference.go:540`, `constraint/tree.go:73` (`typeLayer`) |
| **(b) una alcanza** (lo que buscan no lo cambia un perfil) | `holder.go:144`, `ucumvalidator.go:80`, `walker/discover.go:84`, `extension/extension.go:421` (ya es la definición propia), `constraint/tree.go:140` (primitivos) |
| **(c) comparación con la base** | `slicing/slicing.go:194` y `slicing/tree.go:240`: omiten el min/max que la base ya controla |

### Diseño

1. **Registro.** `ChildrenOf` devuelve `Layers []Layer`; cada capa lleva `SD`, `Nodes` y su procedencia (perfil de tipo,
   definición propia, tipo base, `contentReference`).
   - Con cero o un perfil, una sola capa: igual que hoy.
   - Los campos `SD` y `Nodes` actuales quedan como la primera capa, para los llamadores (b).
   - `typeLayer` deja de duplicar la precedencia y usa lo mismo, con la cadena `SliceOf` incluida. Así se cierra la
     diferencia que hoy tiene con `childrenOf`.
2. **Elección del O.** Una interfaz inyectada por ctx elige los perfiles; el registro no conoce el pipeline. Es el mismo
   patrón de `constraint.HolderProfiles`.
   - Implementación en `pkg/validator`: una sola función de elección, compartida con `holderProfiles.Choose`, con la regla
     de HL7 (`InstanceValidator.java:8235`).
   - Sobre los perfiles **declarados**: uno que no se resuelve cuenta como no cumplido.
   - Uno de otro tipo cuenta como no cumplido y no se recorre. Es la regla actual de `Choose`; la diferencia con HL7 (p20)
     se declara en hl7diff.
   - Entre los del tipo del valor, los que `Conforms` acepta. Si no acepta ninguno, todos.
   - El tipo del valor es el `resourceType` de un recurso o el código del tipo de datos.
   - Si la regla compartida cambia lo que hoy informan los recursos contenidos (p6/p8 con un recurso), entra, porque es la
     misma regla. Se sondea en HL7 antes del test.
3. **Una elección por valor y validación.** Un memo en el ctx de la validación, con clave (nodo, valor) y el
   `conformState` compartido. La cardinalidad recibe el ctx de la validación en vez de `context.Background()`.
4. **Recorrido.** Los llamadores (a) iteran las capas.
   - Lo que dos capas informan igual (mismo ID, ruta, severidad y parámetros) se informa una vez, con una clave por valor
     dentro de la corrida del perfil.
   - Los llamadores (c) omiten el min/max de un slice que coincide con el del mismo hijo en **alguna** capa: la
     cardinalidad ya lo controla bajo esa capa.
5. **Avisos** en la ubicación del valor, una vez, desde la fase de constraint con `firstReportKey`, como
   `holderDefinitions`. Son IDs nuevos, propios de los tipos de datos:
   - `PROFILE_NO_MATCH` (error);
   - `PROFILE_MULTIPLE_MATCHES` (warning).

   `TYPE_PROFILE_NOT_FOUND` (error) se informa por cada perfil que no se resuelve, como hoy con uno.
6. **Primitivos (p14–p16).** `Conforms` acepta un primitivo: valida el valor y su `_key` contra la definición del perfil
   del tipo primitivo, con las mismas fases que hoy aplican a un primitivo con un perfil. La capa del primitivo la usa
   `walkPrimitiveExtensions`.
7. **Tipo de elección (p12):** todos los perfiles, como dice la especificación. La diferencia se declara en hl7diff y se
   reporta a HL7 (issue y PR).

### Fuera del alcance

- Hijos que el perfil despliega: si el nodo tiene hijos propios, las capas no se aplican. Es el incremento 2.
- `targetProfile` de Reference y canonical.
- El texto «(validating against …)».

### Criterio de salida

- **Tests en R4, R4B y R5** con p1–p5 y p13; en R4, p6–p9, p12 y p14–p20, con los valores esperados de HL7. Las
  excepciones son p12 y p20, declaradas. Se comprueba presencia y ausencia, por ID, severidad y ubicación, y que nada se
  repite.
- **Un test por cada llamador (a)** en el que solo la **segunda** capa elegida tiene la regla que falla: cardinalidad,
  binding, slicing, referencia, contexto de extensión, extensiones declaradas y constraint. Mata la mutación «solo la
  primera capa» en cada uno.
- **Mutaciones:**
  - sin elección;
  - siempre el primer perfil;
  - una capa por llamador (a);
  - sin deduplicación entre capas;
  - sin `NoMatch`;
  - sin `MultipleMatches`;
  - ignorar `SliceOf`;
  - cardinalidad con `context.Background()`.
- **Sin regresión:** la suite completa y `hl7diff run -heavy` contra `main`, con p12 y p20 declarados. Inventario de
  `same` como multiconjunto, con cada cambio emparejado con HL7. Con un perfil o ninguno, la salida no cambia
  (`same -identical` en los grupos sin `type.profile` múltiple).
- **Costo:** bench del pipeline y de un Bundle con muchos valores de este tipo, antes y después; sin regresión en los
  que no tienen varios perfiles.
- `make check` en verde.

## 3c. Ronda adversarial 3, la última (revisor Opus independiente, por elección del usuario, 05-11-2026)

No converge: 2 altos y 3 medios. Los altos, verificados. Por la regla de tres rondas, el frente se detiene y se consulta
al usuario.

| # | Hallazgo | Sev. |
|---|---|---|
| C1 | El scope de `Conforms` no llega al elector: `slicematch` importa `registry` (verificado, `go list`); la cardinalidad corre antes que la constraint sin scope ni la cadena de `heldDefinitions`; el memo (nodo, valor) congela la primera respuesta, y en recursos contenidos o entradas de Bundle `%resource` y `resolve()` se evalúan mal | alta |
| C2 | Los perfiles declarados en un slice quedan fuera: la cardinalidad y los llamadores (c) reciben el elemento sin slice (`cardinality.go:277`); `MemberTree` (`instance_children.go:99`, verificado) solo resuelve un perfil único y lo usan holder, binding, slicing y reference; cada fase elegiría con una clave distinta | alta |
| C3 | «Con un perfil, la salida no cambia» es falso: `typeLayer` y `childrenOf` difieren con un perfil que no se resuelve; con la precedencia unificada, la suite da 0 fallos en 26 paquetes (nada fija la salida actual) y ningún test usa `DiagTypeProfileNotFound` | media |
| C4 | Primitivos: la clave del memo es el puntero del map (`slicematch.go:146–150`) | media |
| C5 | Deduplicación entre capas sin sonda (p21: una violación común a los dos perfiles con NoMatch; HL7 probablemente la informa dos veces) | media |
| — | `ChildrenOf` no recibe el valor (cambia la firma de 12 llamadores); qué fase informa `TYPE_PROFILE_NOT_FOUND` con varios perfiles | baja |

**Partición que propone el revisor:**
1. unificar la precedencia (`typeLayer`, `ChildrenOf`, `Unresolved`, `MemberTree`) con cero o un perfil, con tests que fijen
   lo que hoy coincide con HL7;
2. capas, elector, scope, memo y deduplicación;
3. primitivos.
