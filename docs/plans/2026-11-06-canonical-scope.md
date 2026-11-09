# Pieza previa de #193: un solo Scope por posición, y memos de corridas especulativas

Rama `fix/canonical-scope`, desde `87c0b02`. La pidió el usuario el 06-11-2026, después de que la ronda 3 del plan
de la pieza 2 no convergiera ([2026-11-06-type-profile-layers.md](2026-11-06-type-profile-layers.md) §3b y §3c:
B1, C1 y C2). La pieza 2 la da por hecha.

## 1. Revisión previa (06-11-2026)

### Especificación

- FHIRPath, variables de FHIR (R4 `fhirpath.html#variables`): `%resource` es el recurso que contiene la expresión y
  `%rootResource` es el recurso que no está contenido en otro. Las dos dependen solo de **dónde está** el valor.
- Referencias en un Bundle (R4 `bundle.html#references`): una referencia se resuelve en el Bundle que contiene el
  recurso. Es decir, también depende de dónde está el recurso, no de quién lo referencia.
- Conclusión: el Scope de un recurso es función de su posición en el árbol. Dos caminos que llegan al mismo map tienen
  que dar el mismo Scope.

### gofhir en `87c0b02`: constructores de Scope

| Constructor | Bundle contenido | Contenido o no |
|---|---|---|
| `constraint.ScopeOf` (`resolver_adapter.go:165`) | **no lo normaliza**: vuelve antes de tiempo | contra `s.Resource` |
| `slicing.nestedScope` (`slicing.go:274`) | sí | `w.Kind` |
| `constraint` `validateNested` (`tree.go:252-260`) | sí | `holder.Def.Base.Path` |
| `slicematch` `valueScope` (`slicematch.go:443`) | **no** | contra `RootResource` |
| `ResolveScoped` (`validator/slicematch.go:332`) | — | `#id` hereda `Container`; fullUrl y relativa toman el Bundle donde la encuentran |
| `withRootScope` (`holder.go:221`) | — | raíz `{data, data, Container: data}` |
| `reference` `rootContext` / `nested` / `within` (`reference.go:356`, `:269`, `:307`) | sí | raíz no Bundle: `container` **nil**, donde los otros ponen la raíz |
| `checkTargetConformance` (`reference.go:854`) | **no** | el destino hereda `Container` y `Outer` de **quien referencia** (B1) |

`validator.go:868-873` normaliza un Bundle solo dentro de un chequeo de `Conforms`, y eso tapa en parte a `ScopeOf` y
a `valueScope`. La clave de `Conforms` es `{perfil, map, assume}`: el primer Scope que pregunta decide la respuesta.

### gofhir en `87c0b02`: memos que heredan las corridas especulativas

`check` (`validator/slicematch.go:290-295`) crea scopes nuevos de informe y de extensiones. Pero hereda
`holderGroups` (`validator.go:780`, clave `{def, inst}`), `slicerecord` (`validator.go:779`), los descubrimientos y
los índices de Bundles. `settle` solo invalida `memo` y `provisional`. Por eso lo que esos memos derivaron de una
respuesta provisional que después se descarta queda (C2; Codex lo comprobó con `holderGroups`).

### Referencia: HL7 validator_cli 7.0.4

Sondas en [evidence/2026-11-06-canonical-scope/probe/](evidence/2026-11-06-canonical-scope/probe/), R4. Son Bundles
`collection` con un Patient p1 cuyo perfil de destino evalúa `managingOrganization.resolve().name = 'root'`, y un Bundle
anidado b2 con otra Organization o1.

| Sonda | Qué prueba | HL7 |
|---|---|---|
| sc1, sc2 | o1 de la raíz se llama `root` y la de b2 `nested`; obs1 en la raíz y obs2 en b2 referencian `Patient/p1`; dos órdenes | nada |
| sc3, sc4 | los nombres al revés | error de perfil de destino solo en obs1: el `resolve()` de p1 se evalúa en la raíz, donde está p1 |
| sc5 | solo obs2, perfil de destino con `gender.exists()` y p1 sin `gender` | nada: HL7 **no** resuelve `Patient/p1` hacia afuera desde b2 |

Así, en HL7 el Scope del destino es el de su posición (sc3, sc4), y una referencia relativa desde un Bundle anidado no
sale de él (sc5). gofhir tampoco sale de `rc.container` en `resolveTarget` (`reference.go:417`).

Caso de B1 (`ig6/`, `probe6.tgz`): un Patient raíz contiene una Observation `o` (`subject` = `"#"`) y un Bundle `b`
(`signature.who` = `"#"`, perfil `bun-sig`) que tiene la Organization o1 llamada `root`. El Patient referencia
`Organization/o1`. sc6 y sc7 cambian el orden de los contenidos.

| Sonda | HL7 | gofhir `87c0b02` |
|---|---|---|
| sc6 (`o` primero) | error de perfil de destino **solo** en `o.subject` | error en los dos referentes |
| sc7 (`b` primero) | error **solo** en `b.signature.who` | error en los dos referentes |

- En los dos órdenes, el Patient no cumple: el `resolve()` no encuentra o1, que está en `b` y no donde está el
  Patient. Es el Scope por posición.
- Con la referencia relativa, B1 no se observa en gofhir: los dos referentes fallan en los dos órdenes. Con una
  referencia absoluta sí se observa (sc8, sc9, más abajo).
- Diferencia nueva: HL7 informa el destino que no cumple **una vez**, en el primer referente; gofhir, en cada referente.
  Esto no es del Scope: se anota como pendiente aparte.

### Casos observables de C1 y C2 (06-11-2026)

Sondas en `probe/c1/` y `probe/c2/` (perfiles, instancias, `.tgz` de gofhir y salidas de HL7 7.0.4, R4). Las corrí de
nuevo con gofhir para confirmarlas.

**C1, por `checkTargetConformance` y el memo de `Conforms`.** sc8 y sc9 son sc6 y sc7 con `managingOrganization`
absoluta, `http://ex.org/fhir/Organization/o1`.

| Sonda | HL7 | gofhir `87c0b02` |
|---|---|---|
| sc8 (`o` primero) | error solo en `o.subject` | error en `o.subject` y en `b.signature.who` |
| sc9 (`b` primero) | error en `b.signature.who` | **ningún error**: el destino hereda `Container=b`, encuentra o1, y el true memorizado sirve también para `o.subject` |

**C1 más una fuga de `slicerecord` (sc10).** El `generalPractitioner` del Patient raíz se divide en slices por
`resolve()`, y su slice `a` exige `type` en `vs-org`. El Patient contiene un Bundle `b` con `signature.who` = `"#"`. La
fase de referencias, que corre antes que la de bindings, evalúa al Patient con `Container=b` y graba el slice `a` en
el registro de la validación visible.
- HL7: sin errores.
- gofhir: error de perfil de destino en `contained[0].signature.who`, y un binding requerido de
  `generalPractitioner[0].type` que no corresponde.

**C2, por `holderGroups`.** obs1 (`obs-p`) apunta a p1 (`pat-t`, sin `gender`, con `link` a p2). p2 no tiene perfil y
contiene una Observation `c` con `subject` p1. `pat-t` divide `contained` en slices por perfil.

| Sonda | Orden | HL7 | gofhir |
|---|---|---|---|
| sc11 | obs1, p1, p2 | sin error en `link` | error de más en `entry[1].resource.link[0].other`: «Unable to find a profile match for Patient/p2» |
| sc12 | p1 primero | sin error en `link` | sin error en `link` |
| sc13 | p2, obs1, p1 | sin error en `link` | el mismo error de más que sc11 |
| sc14 | control: `c.subject` = p2 | — | desaparece el error de más |

**Lo que no se observa:** `ScopeOf` y `valueScope` no cambian ningún diagnóstico. `validateAgainstProfile`
(`validator.go:868`) normaliza el Bundle en cada `check`, y `within` (`slicematch/path.go:405`) da un Scope normalizado
a los recursos de las entradas.

**Pendientes aparte, que no son de esta pieza:**
- HL7 informa una vez el destino que no cumple (en el primer referente); gofhir, en cada referente (sc6 a sc9).
- En un ciclo de discriminadores por perfil (p1 y p2, `link:L` min 1), HL7 asume que se cumple y no informa nada.
  gofhir responde false e informa «link:L min 1, found 0» en cada uno. Es la política de ciclos de `Conforms`. Las sondas
  están en el scratchpad (`obs/c2cyc/`); se copian a la evidencia al abrir su issue.

### `gofhir/server` (congelado)

No valida con FHIRPath ni arma Scopes (búsqueda de `%resource` y `RootResource`: solo PATCH y la operación `$graph`). Nada que tomar.

## 2. Plan

### Alcance

1. **El Scope del destino de una referencia sale de su posición** (sc8, sc9). `refContext` guarda el `Container` y el
   `Outer` del lugar donde está su recurso raíz (`rc.root`), antes de entrar en él si es un Bundle. Con eso,
   `checkTargetConformance` arma el Scope del destino así:
   - `"#"`: el destino es `rc.root`, con el `Container` y el `Outer` de donde está `rc.root`;
   - `"#id"`: un contenido de `rc.root`, con `RootResource` = `rc.root` y el mismo `Container`;
   - relativa o fullUrl: el destino está en `rc.container`; su Scope ya es el de su posición, y no cambia.
2. **Corridas especulativas aisladas** (sc10, sc11, sc13). `check` empieza un `holderGroups` y un `slicerecord` nuevos,
   así que lo que derive de respuestas provisionales no llega a la validación visible. Se revisa qué otros memos hereda
   `check`. Los que solo dependen de los datos (descubrimientos, índices de Bundles) se comparten, porque no dependen
   de `Conforms`.
3. **Invariante «un map, un Scope».** Un test con un gancho de prueba registra el Scope de cada llamada a `Conforms`
   por `{perfil, map, assume}` y falla si una misma clave llega con dos Scopes. Corre sobre todas las sondas de esta
   pieza y sobre los tests de referencias, holders y Bundles anidados. Es la premisa de la pieza 2 (A2).

### Fuera del alcance

- Unificar los demás constructores (`ScopeOf`, `valueScope`, `nestedScope`): ninguno cambia un diagnóstico. Si el test
  del invariante encuentra un caso, entra.
- El informe por referente (sc6 a sc9) y la política de ciclos de `Conforms` (pendientes aparte, §1).

### Criterio de salida

- **Tests** con los valores de HL7 (sc8 a sc14), en R4, R4B y R5. Antes se sondean en R4B y R5. Se comprueba presencia
  y ausencia por ID, severidad y ubicación, y que el resultado no depende del orden. Las diferencias que no son de esta
  pieza (el informe por referente) se comparan solo en lo que sí es de ella.
- **El test del invariante** pasa sobre las sondas y los tests existentes de referencias, holders y Bundles anidados.
- **Mutaciones:**
  - el destino con el `Container` de quien referencia;
  - `check` con el `holderGroups` heredado;
  - `check` con el `slicerecord` heredado;
  - el gancho del invariante desactivado (el test tiene que fallar con la primera mutación).
- **Sin regresión:** `hl7diff run` contra `main` (los grupos livianos y `r4-core-examples` por separado), con el
  inventario por multiconjunto.
- **Costo:** bench del pipeline y de un Bundle con referencias con `targetProfile` y recursos que tienen otros, antes y
  después. Es lo que más llama a `check`.
- `make check` en verde.

### Dependencias

Ninguna nueva.

## 3. Ronda adversarial 1 (Codex, 06-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| D1 | El arreglo 1 deja `ResolveScoped` (`validator/slicematch.go:332-335`) copiando `Container`/`Outer` de quien referencia. Desde un Bundle contenido, `#o` encuentra a la Organization hermana con `Container=Bundle`. `#` no se resuelve: `ContainedByID` busca por id, también el vacío (`resolver_adapter.go:215`) | alta | **Aceptado** (verificado en el código). La construcción del Scope del destino se comparte entre las referencias y los discriminadores. Se sondean en HL7 `resolve()` con `#` y con `#id` |
| D2 | Guardar la posición de la raíz durante el recorrido no alcanza cuando se vuelve a entrar en una corrida especulativa. `rootContext` (`reference.go:362-375`) reconstruye `rc.root` desde `WithContainingResource`, pero recibe el `Container` del Scope especulativo (`validator.go:942-949`): la posición original ya se perdió | alta | **Aceptado.** Cambia el diseño: un **índice de posiciones por validación**, con el Scope canónico de cada recurso, por su map. Se arma una vez, recorriendo el árbol, y todo constructor (destinos, `ResolveScoped`, reentradas) lo consulta en vez de heredar |
| D3 | Un `holderGroups` nuevo por corrida evita la fuga entre corridas, pero no dentro de una misma corrida: `settle` invalida respuestas sin invalidar los grupos (`holder.go:127-134`) | media | **Aceptado.** Primero, una sonda que lo provoque dentro de una corrida. Si se observa, dentro de una corrida especulativa los grupos no se memoizan. Se mide el costo |
| D4 | El gancho del invariante necesita un contrato preciso: un Bundle contenido llega en dos representaciones antes de normalizarse, y observar solo los chequeos ejecutados pierde el acierto del memo de sc9 (`slicematch.go:178`) | media | **Aceptado, simplificado por D2.** Con el índice, el Scope de un recurso es por construcción el del índice. El test comprueba que todo Scope que llega a `Conforms`, antes de mirar el memo, es igual al del índice para su recurso. Usa un callback local de la validación, sin estado global |
| D5 | sc10 no prueba por sí sola el aislamiento de `slicerecord`: su slice equivocado depende del `Container` equivocado | media | **Aceptado.** Hace falta una sonda en la que `slicerecord` se filtre con el arreglo de Scope ya aplicado, por ejemplo un ciclo de discriminador con un binding. La mutación de `slicerecord` tiene que fallar con el arreglo 1 activo. Expectativas por versión |

## 2 v2. Plan (después de la ronda 1)

1. **Índice de posiciones.** Antes de las fases, la validación recorre una vez los recursos (raíz, contenidos, entradas
   y los que otros elementos tienen) y guarda para cada map su Scope canónico:
   - `Resource` es el propio recurso;
   - `RootResource` es el recurso que no está contenido;
   - `Container` y `Outer` salen de su posición, con los Bundles normalizados.

   Va en el ctx de la validación.
2. **Los constructores consultan el índice:** `checkTargetConformance`, `ResolveScoped` (`#`, `#id`, relativa y
   fullUrl), las reentradas especulativas (`rootContext` y `valueScope`) y `ScopeOf`. Lo que no es un recurso del árbol
   (por ejemplo, un valor de tipo de datos) toma el Scope de su recurso. `#` resuelve a `RootResource`.
3. **Corridas especulativas:** un `slicerecord` nuevo, y un `holderGroups` nuevo o sin memo, según lo que muestre la
   sonda de D3.
4. **Test del invariante:** todo Scope que llega a `Conforms`, antes de mirar el memo, coincide con el del índice.

**Criterio de salida:** el de §2, más:
- sondas de HL7 con `resolve()` sobre `#` y `#id`, desde un Bundle contenido, un recurso que tiene Parameters y una
  entrada;
- la sonda de D3 y la de D5;
- expectativas por versión;
- bench de asignaciones del índice y de los holders en grafos con ciclos.

**Mutaciones nuevas:**
- `ResolveScoped` con el `Container` de quien referencia;
- una reentrada sin el índice;
- `slicerecord` heredado, con el arreglo 1 activo.

## 3b. Ronda adversarial 2 (Codex, 06-11-2026)

D1, D2, D4 y D5 quedan resueltos en el diseño.

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| E1 | Un map que falta en el índice no tiene comportamiento definido. Hay recursos destino que no están en el árbol validado: los de un Bundle externo que llega por `reference.WithContainer` (`reference.go:340`, `:374-377`, `:429-441`). Además, el descubrimiento salta los tipos de recurso desconocidos (`walker/discover.go:119-121`), mientras que el índice de Bundles acepta sus maps (`bundleref.go:76-82`) | alta | **Aceptado.** El índice distingue un valor de tipo de datos (toma el Scope de su recurso) de un recurso que falta. Entran al índice también el Bundle externo de `WithContainer`, con sus entradas, y todo map de recurso que `bundleref` indexa, sea o no de un tipo conocido. Un recurso que falta igual es un error de programa: el test del invariante falla y, en producción, no se hereda el Scope de quien llama. Tests de los dos caminos |
| E2 | El índice necesita un orden de construcción. El walker con perfiles descubre los holders antes de visitar cada recurso (`walker.go:140-159`), y elegir un holder puede llamar a `Conforms` (`holder.go:69`). Si se arma con ese walker, el índice queda a medias: por ejemplo, cuando un discriminador resuelve una entrada posterior | media | **Aceptado.** El índice se arma con un recorrido que no depende de perfiles, después de normalizar la forma (`validator.go:702`) y antes de elegir holders o llamar a `Conforms`. Después no se modifica. Tests con referencias hacia adelante y Bundles contenidos, con los Scopes esperados escritos a mano desde la especificación: comparar contra el propio índice no detecta un índice mal armado |
| E3 | D3 deja abierta una memoización que ya se sabe insegura | media | **Aceptado.** Dentro de una corrida especulativa, `holderGroups` no se memoiza, sin condiciones. La sonda de D3 verifica el arreglo. Se mide el costo |

## 2 v3. Plan (después de la ronda 2)

§2 v2, con estos cambios:
- **Índice (1):**
  - se arma con un recorrido independiente de los perfiles, después de normalizar la forma y antes de toda elección;
  - después no cambia;
  - es uno por validación de nivel superior, y lo heredan los chequeos;
  - incluye el Bundle externo de `WithContainer` y todo map de recurso que `bundleref` indexa.
- **Faltantes:** un valor de tipo de datos toma el Scope de su recurso. Un recurso que falta en el índice es un error
  de programa: el test del invariante lo detecta, y en producción no se hereda el Scope de quien llama.
- **Corridas especulativas (3):** un `slicerecord` nuevo y `holderGroups` sin memo.
- **Criterio de salida, además:**
  - Scopes esperados escritos a mano para referencias hacia adelante, Bundles contenidos, Parameters y un Bundle
    externo;
  - bench de un Bundle grande y plano sin `targetProfile`, y de grafos con ciclos.

## 3c. Ronda adversarial 3, la última (Codex, 06-11-2026)

E2 y E3 quedan resueltos. **No converge** por un hallazgo medio, así que se consulta al usuario.

| # | Hallazgo | Sev. | Propuesta |
|---|---|---|---|
| F1 | E1, en parte: no está definido qué hace producción cuando un recurso falta en el índice. «Error de programa» puede ser un panic, una validación abortada, una referencia sin resolver o un Scope vacío. Y como `Conforms` devuelve solo `bool` (`slicematch.go:166`), un false convertiría un defecto del índice en un perfil no cumplido | media | Que falte un recurso **aborta la validación de nivel superior** con un error interno no nulo (`Validate` ya devuelve `error`), que nombra el recurso y el sitio de la consulta. No se evalúa ni se memoiza nada con un Scope inventado. Test: se omite a propósito una entrada del índice y se verifica ese error, sin el callback del invariante |

## 2 v4. Propuesta (sin cuarta ronda, sujeta a la decisión del usuario)

§2 v3, más F1 tal como lo propone Codex: que falte un recurso en el índice aborta la validación con un error interno,
que se propaga desde la consulta hasta `Validate` sin memoizar nada, y tiene su test.

**Aprobado por el usuario (07-11-2026):** plan §2 v4, sin cuarta ronda.

## 5. Sondas antes de los tests (07-11-2026)

La tabla por sonda está en [probe/README.md](evidence/2026-11-06-canonical-scope/probe/README.md). En las tres versiones,
tanto HL7 como gofhir dan los mismos diagnósticos que en R4, y ninguna instancia cambia entre versiones.
- **sc8 a sc14:** en R4B y R5, igual que en R4.
- **D1 (`d1/`, 17 sondas).** En HL7, `resolve()` sobre `"#"` no devuelve nada en ninguna posición, y gofhir tampoco.
  Con `#id`, el invariante coincide en las tres posiciones. D1 se ve en el **destino** de `#o` desde un Bundle
  contenido: gofhir lo evalúa con el Container de quien referencia, y HL7 con el de la posición del destino.
  - d1a5, d1a6 y d1a10: gofhir da un error de perfil de destino que HL7 no da.
  - d1a9 y d1a11: HL7 da el error y gofhir no.
  - d1a12 y d1a13: lo mismo, pero en una corrida especulativa con slices de `contained`.
- **D3 (`d3/`), observable.** En d3a1 y d3a2, gofhir da un error de más en `r1.hasMember[0]`, y depende del orden: d3b,
  con el orden inverso, no lo da, y tampoco el control d3c. En una copia, quitar el memo de `holderGroups` en las
  corridas especulativas borra el error de d3 y también los de sc11 y sc13.
- **D5 (`d5/`, 12 sondas), no observable.** Con el Scope ya corregido, `slicerecord` no se filtra. El slicing visible
  corre antes que las fases de referencias y bindings, y una corrida posterior reusa sus respuestas y graba lo mismo.
  **Decisión:** el `slicerecord` nuevo por corrida **sale del alcance**, porque ningún test podría matar su mutación.
  Las diferencias de d5s1, d5s3, d5s6 y d5s7 son de la política de ciclos, que ya está pendiente aparte.

## 6. Implementación (07-11-2026)

La hizo Sonnet.
- **Índice:** `internal/scopeindex`, del puntero de cada map de recurso a su Scope. Se arma una vez en `Validate`,
  después de normalizar la forma y antes de elegir holders, con un recorrido que no depende de perfiles. Lo consultan:
  - `Conforms`, `checkTargetConformance`, `refContext.nested` y `rootContext`;
  - `ResolveScoped`, `ScopeOf`, `heldDefinitions` y `valueScope`;
  - `slicing.nestedScope` y `constraint.validateNested`.

  La normalización de Bundle de `validateAgainstProfile` sale.
- **Recurso que falta:** el índice guarda un `*MissingError` (tipo, id y sitio de la consulta). Desde ese momento
  `Conforms` no evalúa ni memoiza nada, y `Validate` devuelve `(nil, err)`. Una llamada directa a un subvalidador arma
  el índice con `Ensure` y no ve ese error.
- **Corridas especulativas:** `holderGroups` sin memo.
- **Tests:**
  - `canonical_scope_test.go`: c1, c2, d1 y d3 en R4, R4B y R5. Los valores esperados los genera
    `probe/scripts/gen_expected.py` desde las salidas de HL7. En sc8 y sc9 se exige el error del primer referente y se
    aceptan los de más, porque informar por referente es la diferencia pendiente aparte.
  - `scopeindex_test.go`: Scopes escritos a mano desde la especificación.
  - `scope_invariant_test.go`: el invariante, una entrada omitida a propósito y `Conforms` sobre un recurso ausente.
- **Antes del cambio** fallan 40 subtests (sc9, sc10, sc11, sc13, d1a5, d1a6, d1a9 a d1a13, d3a1 y d3a2, en las tres
  versiones). Después pasan todos. `make check` verde, sin cambios en los dorados.

hl7diff contra `main` (07-11-2026): 61 grupos livianos PASS, sin hallazgos; `r4-core-examples` PASS, sin hallazgos.

Bench contra `main` (07-11-2026, `-count=6`, benchstat): sin diferencia de tiempo significativa. Las asignaciones
suben un 0,3 % a 1 % (Patient: de 1.753 a 1.759 por validación; Batch: de 2.429 a 2.454), que es el costo del índice.
`BenchmarkValidateProfiled` solo existe en la rama, así que no se compara. Falta un bench con `targetProfile` y
holders en grafos con ciclos. Salidas en el scratchpad (`scope-impl/bench-*.txt`, `benchstat.txt`).

## 7. Mutaciones (07-11-2026)

14 mutaciones en una copia: 8 muertas con los tests dirigidos y una más (la 9) con la corrida ampliada.
- **Muertas:**
  - 1: el destino con el `Container` de quien referencia;
  - 5: `holderGroups` heredado;
  - 7: tipos desconocidos fuera del índice;
  - 8: heredar ante un faltante;
  - 9: `Validate` ignora el `MissingError`;
  - 10: sin índice en el ctx;
  - 11: callback del invariante desactivado;
  - 14: `referenceResolver.ScopeOf` sin el índice.
- **Vivas:**
  - 2: `ResolveScoped` `#id` sin el índice;
  - 3: relativa y fullUrl sin el índice;
  - 4: `Conforms` no reemplaza el Scope;
  - 6: sin el Bundle externo de `WithContainer`;
  - 12: `slicing.nestedScope` sin el índice;
  - 13: `constraint.validateNested` sin el índice.

  En todos los tests, el Scope de la consulta vieja ya es el del índice, y la sonda de C1 (§1) tampoco encontró
  diferencias observables en esos constructores. Hipótesis: las consultas 2, 3, 4, 6, 12 y 13 son **redundantes**; el
  test del invariante prueba que los constructores viejos dan el Scope canónico en todas las entradas que se ejercitan.
  Se lleva al code review la pregunta de si se quitan, y se deja el invariante como guarda, o si se buscan entradas
  que las maten.

## 8. Code review

### Ronda 1 (Codex, 07-11-2026)

Reproducciones en la copia de revisión (`.review/review_round1_test.go`, `review_cost_test.go`, `reproductions.txt`).

| Hallazgo | Sev. | Resolución |
|---|---|---|
| Una validación de referencias independiente (`reference.go:524`, con `WithContainingResource`) arma con `Ensure` un índice solo del recurso dado. Una Observation contenida con `subject="#p"` hacia un Patient hermano con perfil de destino da **0 chequeos y 0 issues**; con el recurso que la contiene en el índice vuelve el error | alta | **Aceptado** (reproducido por Codex). `Ensure` indexa también el recurso que contiene y el que tiene. Test de regresión; el valor esperado es el de HL7 al validar el recurso que contiene |
| Indexar los contenidos es cuadrático (`scopeindex.go:152`): 2.000, 4.000 y 8.000 contenidos tardan 33, 131 y 513 ms | media | **Aceptado.** Las identidades de los contenidos se calculan una vez por recurso que los tiene. Bench de regresión |
| La relajación de sc8 y sc9 acepta errores de perfil de destino sin relación (`canonical_scope_test.go:122`) | media | **Aceptado.** Se acepta solo el referente de más conocido, con el mismo destino y perfil, y la multiplicidad esperada |
| Comentarios nuevos en inglés | baja | Aceptado |

Mutaciones vivas de §7, según Codex:
- **2:** brecha real, pero solo con traza del resolutor, sin un diagnóstico de punta a punta. Se mantiene, con un test
  del resolutor.
- **3:** brecha real. Un discriminador de Composition con `entry.resolve().signature.onBehalfOf.resolve()` pasa hoy, y
  con la mutación aparece `SLICING_CARDINALITY_MIN`. Se mantiene, con una sonda en HL7 y un test.
- **6:** brecha real. Ya la mata el test del contexto externo, que no estaba en el filtro de la corrida de mutaciones.
  Se mantiene.
- **4, 12 y 13:** redundantes. Se quitan, y el test del invariante queda como guarda. De la 4 se mantiene la detección
  del recurso que falta.

### Resolución de la ronda 1 (07-11-2026)

- **Alta:** `scopeindex.Ensure` recibe el recurso que contiene (`WithContainingResource`) o tiene (`WithHoldingResource`) y lo indexa con lo que tiene. Test `TestStandaloneReferenceContexts` (containing y External); valor de HL7 7.0.4 en `probe/r1/` (r1a, R4, R4B y R5): un `REFERENCE_TARGET_PROFILE` en `DiagnosticReport.contained[0].subject`.
- **Media (costo):** las identidades de los contenidos se calculan una vez por recurso que los tiene. Construir el índice de 2.000, 4.000 y 8.000 contenidos pasó de 11, 44 y 171 ms a 0,26, 0,47 y 1,06 ms (`TestBuildContainedGrowsLinearly`, `BenchmarkBuildContained`).
- **Media (test):** sc8 y sc9 aceptan solo el error de más conocido (`extraReferrers`), una vez.
- **Consultas redundantes quitadas:** 4 (`Conforms` ya no reemplaza el Scope; sigue detectando el recurso que falta y llamando al callback del invariante), 12 y 13.
- **Consulta 3, hallazgo:** la sonda r1b (Composition con discriminador `entry.resolve().signature.onBehalfOf.resolve()`) da en HL7 un `SLICING_CARDINALITY_MIN`: una cadena de `resolve()` sigue en el Bundle donde encontró la entry, no en el que ella sea. `main` coincidía con HL7; el índice no (daba cero errores). Se devolvió a `ResolveScoped` el Scope de la entry en el Bundle donde se la encontró. Las consultas 2 (`#id`) y 6 (`WithContainer`) se mantienen, con `TestCanonicalScopeResolveScopedContainedID` y `TestStandaloneReferenceContexts/External`.

### Correcciones de la ronda 1 (07-11-2026, Sonnet)

- **Alto:** `scopeindex.Ensure` indexa también el recurso que contiene o tiene. HL7 en r1a (`probe/r1/`, R4, R4B y R5)
  da un `REFERENCE_TARGET_PROFILE` en `DiagnosticReport.contained[0].subject`. Test `TestStandaloneReferenceContexts`.
- **Costo:** con 2.000, 4.000 y 8.000 contenidos, un build pasa de 11, 44 y 171 ms a 0,26, 0,47 y 1,06 ms. Quedan
  `TestBuildContainedGrowsLinearly` y `BenchmarkBuildContained`.
- **sc8 y sc9:** se acepta solo el referente de más conocido, una vez (`TestCompareScopeAllowsOnlyTheKnownExtraReferrer`).
- Comentarios traducidos.
- **Consultas:** salen la 4, la 12 y la 13. La 2 queda con un test del resolutor y la 6 con `TestStandaloneReferenceContexts/External`.
- **Consulta 3, revertida por evidencia de HL7:** en r1b, una Composition con el discriminador
  `entry.resolve().signature.onBehalfOf.resolve()`, HL7 da `SLICING_CARDINALITY_MIN` en las tres versiones. `main`
  coincidía con HL7, y la consulta al índice lo perdía. En una cadena de `resolve()` de FHIRPath, HL7 sigue en el
  Bundle donde encontró la entrada, aunque esa entrada sea un Bundle. `ResolveScoped` vuelve a eso, con test.

### Una inconsistencia de HL7 (07-11-2026, sondas `probe/r2/`, R4)

Un Bundle b2, entrada de b1, con `signature.who` = `Patient/p`. Hay un Patient p en b2 (`inner`) y otro en b1
(`outer`), y `bun-tpw` exige un destino `pat-inner`.

| Sonda | Pacientes | HL7 |
|---|---|---|
| r2a | `inner` en b2, `outer` en b1 | nada: resuelve en b2 |
| r2c | solo `outer`, en b1 | nada: no sale de b2 |
| r2d | `zzz` en b2, `outer` en b1 | error de perfil de destino: resuelve en b2 |

En la **validación de referencias**, las referencias propias de un Bundle anidado se resuelven en él mismo (r2a,
r2d), como dice el índice. En una **cadena de `resolve()` de FHIRPath** (r1b), en el Bundle que lo tiene. gofhir sigue
a HL7 en los dos caminos. La especificación (`bundle.html#references`) no distingue los dos casos. Se redacta una
observación a HL7. Para la pieza 2, el test del invariante sigue de guarda: un Scope que llega a `Conforms` distinto
del índice lo hace fallar.

Mutaciones completas después de las correcciones (07-11-2026): 14 de 16 muertas.
- **4** (`Conforms` reemplaza el Scope): equivalente. Los que llaman ya pasan el Scope del índice, y el invariante lo
  comprueba.
- **16** (`refContext.place` no consulta el índice): queda viva. Para verla habría que validar un recurso por separado,
  con `WithContainer` hacia un Bundle A, mientras el recurso está en una entrada de un Bundle B. Se lleva a la ronda 2
  la pregunta de si `place` hace falta.

### Ronda 2 (Codex, 07-11-2026)

La evidencia está en la copia de revisión, en `.review/round2-*`.

| Hallazgo | Sev. | Resolución |
|---|---|---|
| Un discriminador que termina en `entry.resolve()` (`slicematch.valueScope`) pasa a `Conforms` el Scope de la cadena, porque la raíz de la definición resuelta no cumple `HoldsResource`. El invariante se rompe: el mismo map llega con dos Containers, uno da falso y el otro verdadero, y aparece un `SLICING_CARDINALITY_MIN` | alta | **Aceptado**, porque el invariante falla (reproducido). `valueScope` canoniza todo map de recurso con `ScopeOf`, también la raíz resuelta. La cadena conserva su Scope para los pasos siguientes. El valor esperado lo da HL7 con la sonda r3a |
| Un Bundle raíz suelto validado con `External` (`scopeindex.Build`) recibe el Container externo como canónico. `place` pisa el contexto correcto y elige el Patient de afuera | media | **Aceptado** (reproducido; mata la mutación 16). En el caso de reserva, si la raíz es un Bundle se la trata con `inBundle`. El valor esperado lo da HL7 con la sonda r3b (`-bundle` no aplica: se valida b2 solo, con el Bundle externo como contexto, si el CLI lo permite; si no, se registra como un caso sin referencia) |

Quitar la consulta 4 fue prematuro. La canonización vuelve en el límite de `valueScope`, no en `Conforms`.

### Correcciones de la ronda 2 (07-11-2026)

- **Media (r3b):** `scopeindex.Build`, en el caso con contenedor externo, trata la raíz con `inBundle` si es un Bundle.
  `TestStandaloneDetachedBundleResolvesInItself` falla sin la corrección (destino `outer`) y pasa con ella (destino
  `inner`, sin errores). HL7 7.0.4 (`probe/r3/`, r3b, R4, R4B y R5): ningún error al validar el Bundle solo; el CLI no
  puede expresar el Bundle externo, y el caso se apoya en `bundle.html#references` y en r2a y r2d.
- **Alta (r3a), SIN APLICAR:** la sonda `probe/r3/` (r3a1: Composition con discriminador `entry.resolve()` contra
  `bundle-p`, que parte `Bundle.signature` por `onBehalfOf.resolve()` contra `org-t`, con la Organization con `name`)
  da en HL7, en R4, R4B y R5, un `SLICING_CARDINALITY_MIN` en la Composition. gofhir sin el cambio en `valueScope` lo
  reproduce; con la canonización de la raíz resuelta con `ScopeOf` desaparece, y se aparta de HL7. HL7 comprueba el
  perfil del destino de `resolve()` con el Scope de la cadena. Se detuvo a la espera de decisión.

### Decisión pendiente tras la ronda 2 (07-11-2026)

La corrección del hallazgo alto (`valueScope` canoniza con `ScopeOf`) se aparta de HL7 en r3a1, en R4, R4B y R5:
HL7 da `SLICING_CARDINALITY_MIN` y la corrección no da nada. HL7 comprueba el perfil del destino de `resolve()` con el
Scope de la cadena. Se revirtió, y gofhir coincide con HL7. La corrección quedó en `scope-fix2/slicematch.fixed.go`.

El invariante «un map, un Scope» del plan v4 contradice a HL7. `TestScopeInvariantOnTheProbes` cubre c1, c2, d1 y d3,
pero no r1 ni r3; con r3 fallaría. Queda para el usuario elegir entre:
- seguir a HL7, con un invariante por camino (Scope del índice para referencias y holders, Scope de la cadena para
  `resolve()`) y una observación a HL7 sobre r2 frente a r1b y r3a;
- o seguir el invariante y apartarse de HL7, con divergencia documentada.

El hallazgo medio (r3b) está corregido, con test que falla antes y pasa después. `make check` está verde.

**Decisión del usuario (08-11-2026): seguir a HL7.**
- Se mantiene el código actual.
- El invariante pasa a ser por camino. Toda llamada a `Conforms` que no viene de una cadena de `resolve()` lleva el
  Scope del índice. La que viene de una cadena lleva el Scope de la cadena, como en HL7 (r1b y r3a).
- Las sondas r1 y r3 entran en el test del invariante.
- Se redacta una observación a HL7 sobre la inconsistencia (r2 frente a r1b y r3a).

### Invariante por camino (08-11-2026)

Implementa la decisión «seguir a HL7». No cambia el comportamiento de producción.
- `profileConforms` (`pkg/slicematch`) marca el contexto de cada consulta con `WithChain(ctx, e.in != nil)`: `e.in`
  no es nil exactamente cuando el valor lo alcanzó una cadena de `resolve()`. `conformer.Conforms` lee la marca
  (`slicematch.InChain`), la baja para las consultas anidadas y la pasa al callback del invariante.
- Esas dos funciones son lo único exportado que se agrega. Hacen falta porque el callback vive en `validator` y
  `Conforms` no puede cambiar de firma ni recibir otro campo sin tocar `Scope`; el índice no puede ser el portador
  (`scopeindex` importa `slicematch`).
- El test: consulta sin cadena, el Scope pedido es igual al del índice. Consulta de cadena, exenta, pero para un mismo
  map el Scope de la cadena debe ser siempre el mismo. `TestScopeInvariantOnTheProbes` cubre c1, c2, d1, d3, r1 y r3.
- No vacuo: con el invariante global anterior falla en r3a1 y r3a2 (12 consultas, `Bundle/nested`, contenedor
  `r3a1` frente a `nested`); con una mutación que hace que un camino sin cadena use el Scope de la cadena fallan 198.
- Observación para HL7: `evidence/2026-11-06-canonical-scope/hl7-observation-resolve-scope.md` (borrador, sin publicar).

Mutaciones completas antes de la ronda 3 (08-11-2026): 17 de 20 muertas.
- **16** (`place` sin efecto): vive. Desde la corrección de r3b, `place` podría ser redundante.
- **18 y 20** (la marca de cadena siempre en verdadero, o sin limpiar en las consultas anidadas): viven. Son huecos
  del oráculo (el test del invariante), no del código. El oráculo se endurece con autotests negativos antes de la
  ronda 3.

Oráculo endurecido y `place` retirado (08-11-2026, antes de la ronda 3).
- Mutaciones 18 y 20 ahora mueren. Tests nuevos en `scope_invariant_test.go`: `TestOnlyResolveChainsAreMarkedAsChains`
  (en c1, c2, d1 y d3 ninguna consulta de cadena; en r1 y r3 exactamente las esperadas, r3a1 y r3a2 con 2 de 2),
  `TestScopeOracleFlagsAWrongScope` (autotest negativo del oráculo: Scope equivocado sin cadena se marca; Scopes
  distintos de cadena para un mismo map, también) y `TestNestedQueryUnderAChainIsNotAChain` (una consulta anidada
  bajo una de cadena no lleva la marca). El oráculo se factorizó en `scopeOracle`.
- Mutación 16: `place` era redundante. Instrumentado para avisar cuando cambiaba container u outer, no cambió nunca en
  toda la suite de `pkg/` ni en cuatro variantes de r3a1 (destino solo en el Bundle exterior, en ambos, tres niveles
  de anidación, referencia relativa), en R4 y R5. La razón: el contenedor de una consulta de cadena es el Bundle donde
  se halló la entrada, que es el padre del recurso en el índice, y para un Bundle `rootContext` ya fija su propio
  contenedor. Se quitaron `place` y `placeResource`; el Scope del índice llega por `nested` y `rootContext`.

hl7diff contra `main` después de las correcciones de la ronda 2 (08-11-2026): pasan los 61 grupos livianos y
`r4-core-examples`.

### Ronda 3 (Codex, 08-11-2026): no converge

Evidencia en la copia de revisión: `.review/round3-notes.md`.

| Hallazgo | Sev. |
|---|---|
| El memo de `Conforms` (`conformKey`, `validator/slicematch.go:207`) no incluye el Scope. En r3a1, `Bundle/nested` contra `bundle-p` da verdadero y verdadero con el Scope canónico primero y el de la cadena después, y falso y falso en el orden inverso. Con un discriminador por `resource` en el Bundle exterior desaparece el error de mínimo de la Composition. Es consecuencia de la decisión de seguir a HL7: dos Scopes por map exigen una clave que incluya el Scope | alta |
| `e.in != nil` no identifica bien las cadenas de `resolve()` (`slicematch.go:504`): `walker.within` también pone `in` al cruzar un elemento con un recurso. Un discriminador por `resource` queda marcado como cadena, y el oráculo informa conflictos falsos | media |
| Sin `place`, el test del Bundle suelto ya no detecta que falta `inBundle`. Hay que afirmar el Container y el Outer indexados | baja |
| La observación a HL7 generaliza a R4B y R5, pero r2 solo se midió en R4 | baja |
| Un comentario en inglés (`validator/slicematch.go:405`) y comentarios que todavía afirman el invariante global | baja |

Llegamos al tope de 3 rondas. Se consulta al usuario.

**Decisión del usuario (09-11-2026): corregir y cerrar, sin cuarta ronda.** Los 5 hallazgos se corrigen, cada uno con
un test que falla antes y pasa después. Después: mutaciones completas, hl7diff, bench y `make check`.

### La evidencia de r3 era un artefacto (09-11-2026)

En `probe/r3/ig`, `bundle-p` corta `Bundle.signature`, que en R4 es 0..1. HL7 no puede armar el snapshot («Attempt to
slice an element that does not repeat»), así que el `SLICING_CARDINALITY_MIN` de r3a1 no dice nada sobre el Scope.
Con `bundle-p` bien armado (`probe/r4/`, generador `scripts/gen_r4.py`, HL7 7.0.4 en R4, R4B y R5):

| Sonda | HL7 | gofhir actual |
|---|---|---|
| r4c (r3a1 sin perfil exterior) | sin errores | `section:B` min (**diverge**) |
| r4a (r3a1 con discriminador `resource` exterior) | sin errores | sin errores, solo por el orden del memo |
| r4b (r3a2 con perfil exterior) | 3 errores | los mismos 3 |

HL7 comprueba el perfil del destino de `resolve()` con el Scope canónico. Los pasos de FHIRPath de la cadena siguen en
el Bundle donde se encontró la entrada (r1b). La decisión del 08-11 se tomó sobre evidencia falsa, así que se vuelve a
consultar al usuario.

**Decisión del usuario (09-11-2026): volver al invariante global** («un map, un Scope» en `Conforms`).
- `valueScope` canoniza la raíz resuelta con `ScopeOf`, y la cadena conserva su Scope para los pasos de FHIRPath (r1b).
- Salen la marca de cadena e `internal/chainmark`.
- r4 queda como test con los valores de HL7.
- Se corrigen los hallazgos bajos y se reescribe la observación a HL7.
- Cierre sin cuarta ronda.

### Correcciones finales (09-11-2026)

- `valueScope` (`pkg/slicematch/slicematch.go`) canoniza con `ScopeOf` todo recurso que llega a `Conforms`, también la
  raíz de una definición que `resolve()` alcanzó. La cadena conserva su Scope (`ResolveScoped`, r1b) solo para los pasos
  siguientes de FHIRPath. Salieron `internal/chainmark`, el campo `chain` de `end` y `state`, y el parámetro `chain` del
  oráculo (`scopeCheck`).
- Datos de prueba: el grupo r3 se reemplazó por r4 (r4a, r4b, r4c) en `testdata/canonical_scope`,
  `gen_expected.py` y los grupos del invariante; los valores salen de `probe/r4/` (HL7 7.0.4, R4, R4B y R5). El
  perfil de `TestStandaloneDetachedBundleResolvesInItself` pasó a `testdata/canonical_scope/r2/ig`. `probe/r3/` queda
  como evidencia con una línea en su README que dice que es un artefacto.
- r4c falla antes de la corrección en las tres versiones (`section:B` mínimo) y pasa después; el invariante fallaba
  en `Bundle/nested` de r4a, r4b y r4c.
- El memo no lleva Scope: un map, un Scope. `TestResolveChainAndCanonicalArrivalAgreeInBothOrders` comprueba que r4a da
  verdadero (HL7: sin errores) tanto si el Bundle anidado llega primero con el Scope canónico como por la cadena.
- `TestStandaloneDetachedBundleResolvesInItself` afirma el Container y el Outer indexados de la raíz suelta: sin
  `inBundle` en `scopeindex.Build` falla (`container Bundle/b1, outer []`).
- Observación a HL7 reescrita: r2a, r2c y r2d dan lo mismo en R4B y R5. Con la evidencia correcta no queda
  inconsistencia (la validación de referencias resuelve en el Bundle que tiene el recurso; `resolve()` es FHIRPath y
  conserva el contexto de su expresión): la nota queda marcada como **no enviar**.
- Comentarios: el de `validator/slicematch.go` pasó a español y todos describen el invariante global.

### Verificación final (09-11-2026)

- **Mutaciones:** 20 de 20 muertas. Las dos sobre `refContext.nested` y `rootContext` se adaptaron, porque esas funciones
  ya no consultan el índice; el invariante global protege su salida en `Conforms`.
- **hl7diff contra `main`:** pasan los 61 grupos livianos y `r4-core-examples`.
- **Bench contra `main`** (`-count=6`): sin diferencia de tiempo significativa. Las asignaciones suben entre 0,2 % y 1 %.
  Un build del índice con 2.000, 4.000 y 8.000 contenidos tarda unos 0,34, 0,75 y 1,8 ms.
- **`make check`:** verde (scratchpad, `scope-fix6/make-check.txt`).
- Las notas de las rondas de Codex se copiaron a `evidence/2026-11-06-canonical-scope/review/`.
