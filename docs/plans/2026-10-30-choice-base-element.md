# #183: el elemento base de ChoiceType bajo una contentReference y en un slice sin tipo

Issue: gofhir/validator#183. Rama `fix/choice-base-element`, desde `970e2c1`.

## 1. Revisión previa (30-10-2026)

### El problema

`Registry.ChoiceType` (#180) decide qué claves JSON de un elemento de elección son propiedades con los
tipos de su elemento **base** (formats.html#choice). `baseElement` (`pkg/registry/registry.go`) lo busca:

1. por la ruta propia en la cadena de `baseDefinition` del nodo;
2. por cada ancestro con un único tipo, primero la ruta relativa y después `base.path`, en la cadena de ese tipo;
3. por `base.path` en la cadena propia.

Los tres casos de la ronda 5 del #180 se reproducen en `970e2c1`. Los tests están en
`docs/plans/evidence/2026-10-27-choice-key-case/review-r5/`.

| Caso | Entrada | Obtenido | Esperado |
|---|---|---|---|
| 1 | Modelo lógico con raíz `Timing`; `Timing.repeat` (tipo `Timing`) declara `repeat.bounds[x]` (Coding, date); `Timing.repeat.item` → `#Timing.repeat`; perfil con `Timing.repeat.item.bounds[x]` | el paso 2 encuentra `Timing.repeat.bounds[x]` del núcleo: `boundsPeriod` → Period | Coding, date |
| 2 | Snapshot publicado cuyo slice `Patient.extension:e` no repite `type` | el slice tiene cero tipos y se salta: `valueCoding` → "" | Coding |
| 3 | `A2.item` → `http://example.org/U#U.grp`; `A2.item.value[x]` con base.path `U.grp.value[x]` | "" | date |

### La especificación (R4, R4B y R5, el mismo texto: `constraints.txt`)

- `ElementDefinition.contentReference`: «Identifies an element defined elsewhere in the definition whose
  content rules should be applied to the current element. ContentReferences bring across all the rules
  that are in the ElementDefinition for the element». Comentario: «ContentReferences can only be defined
  in specializations, not constrained types, and they cannot be changed and always reference the
  non-constrained definition». `eld-5`: con `contentReference` no hay `type`.
- Lo que hay bajo un elemento con `contentReference` es, por definición, el contenido del elemento
  referido. El elemento base de `X.item.bounds[x]` es el hijo `bounds[x]` del destino de `item`. No es lo
  que `base.path` permita adivinar.
- `ElementDefinition.base`: «Across FHIR, there is only one base definition of any element». `base.path`
  es una ruta, no una definición.
- Snapshot (structuredefinition.html): «expressed in a standalone form that can be used and interpreted
  without considering the base StructureDefinition». Ninguna regla (`sdf-*`, `eld-*`) obliga a un slice
  a repetir `type`. Un slice es una restricción del elemento que corta: si no declara tipos, sus tipos
  son los del elemento cortado.

### Medición (71 packages de `~/.fhir/packages`, sin ejemplos; scripts y salidas en `evidence/2026-10-30-choice-base-element/`)

- **Caso 2:** ningún elemento de snapshot sin `type` fuera de la raíz y de `contentReference`, ni en
  slices (`scan-typeless-and-urls.txt`). El caso no aparece en packages publicados.
- **Caso 3:** 779 `contentReference` con URL. Todas apuntan a la propia cadena de `baseDefinition`
  (669) o a la URL del tipo de la `StructureDefinition` (110, perfiles cuya base no está entre los
  packages cargados) (`scan-contentreference-targets.txt`). Ninguna apunta a otra `StructureDefinition`
  sin relación: el caso 3 queda fuera de «elsewhere in the definition», pero la corrección del caso 1 lo
  cubre sin costo extra.
- **Caso 1:** en 1.621 elementos con hijos de especializaciones, 14 tienen un tipo que no es
  `BackboneElement`/`Element`/`Base`: los modelos lógicos de CDS Hooks de `hl7.fhir.uv.tools.r4`, con
  tipo `CDSHooksElement` (`scan-specialization-children.txt`). El patrón es válido y existe. Hay además
  una `contentReference` por URL (`CDSHooksResponse.systemActions`), aunque sin elementos de elección
  bajo ella. La combinación del caso 1 no aparece en ningún package.

### Otras implementaciones

- **`gofhir/server` (congelado):** valida con esta librería (`gofhir/validator/v2 v2.1.0`), así que no
  hay nada que tomar. **Se descarta.**
- **HL7 Java (`InstanceValidator`):** al bajar por un elemento con `contentReference`, resuelve el
  destino y valida con sus hijos. Es la semántica de la definición. A confirmar con sondas en el paso 7,
  donde se pueda (las instancias de modelos lógicos no se validan directo con el CLI).

### Decisiones

- **Mejorar `baseElement`:** recorrer los ancestros del más cercano al más lejano. Un ancestro con
  `contentReference` lleva a su destino (`ElementNode.Content` o `Registry.ContentReference`) y a la ruta
  relativa desde él. Un ancestro con un único tipo sigue como hoy. Así el ancestro más cercano manda, y
  `base.path` queda solo como desempate.
- **Slice sin tipos:** usar los tipos del elemento que corta (`SliceOf`), por la definición de slice. Es
  barato, pero el caso no existe en packages publicados. Se decide en el plan si entra.
- **Se descarta que el generador de snapshots lleve el origen** (la alternativa del issue): no sirve para
  snapshots publicados, que no genera el validador, y el caso 2 es justamente uno de ellos. La base tiene
  que salir de la estructura.

## 2. Plan (rehecho tras la ronda adversarial, §3)

### La idea

La base de un elemento es **el hijo de la base de su padre**, siguiendo lo que esa base define. No se lee
del ancestro tal como quedó en el snapshot del perfil, porque los snapshots cambian lo que lo define: el
generador propio y el de HL7 reemplazan una `contentReference` por el tipo del destino cuando el
diferencial nombra hijos (`snapshot.go`, `unroll`; `sdc-valueset`, `ValueSet.expansion.contains.designation`).
Otras veces la conservan (`ValueSet.expansion.contains.contains`). La definición del padre en su cadena
(un modelo o un recurso, nunca un perfil) conserva la `contentReference` original.

### Alcance

`Registry.baseElement` (`pkg/registry/registry.go`) y una función auxiliar. `ChoiceType`, su caché por
nodo y sus consumidores (`primitive`, `fixedpattern`) no cambian.

`baseElement(node)`:

1. **Definición propia** (sin cambios): `definedIn(node.sd, node.Def.Path)`. Resuelve también los slices
   (la base de `Patient.extension:e` es `Patient.extension` del núcleo, con su tipo).
2. **El hijo de la base del padre:** `P = node.Parent`, `rel` = la ruta del nodo desde la de `P`, y
   `E = baseElement(P)` (con caché; si es nil, `P`). `childOf(E, P, rel)`:
   1. `rel` definido bajo `E` en la cadena de `E`: `definedIn(E.sd, E.Def.Path + rel)`. Un destino que es
      un slice se busca por id (`E.Def.ID + rel`) en su árbol.
   2. `E` con `contentReference`: su destino, por `Registry.ContentReference` (`#id` y `url#id`), y
      `childOf(destino, …)`. Las referencias en cadena se protegen de ciclos.
   3. Por tipo: los de `P` (un perfil los acota: `effective[x]:effectiveTiming`) o, si no tiene, los de
      `E`. Si son varios y `P` es un slice de elección, se acotan por el nombre del slice
      (`ChoiceTypeIndex`, formats.html#choice). Con un único tipo, `rel` en la cadena de ese tipo
      (resuelto por su código, que puede ser una URL) y, si no está, el mismo `childOf` sobre la raíz del
      tipo (cubre la `contentReference` interna de un tipo, `T.item.item.value[x]`).
3. **`base.path` en la cadena propia** (sin cambios), como último respaldo.

Con esto no hace falta el respaldo de `SliceOf` del plan anterior (§3, H3 de Codex): el slice tiene base
por el paso 1.

### Fuera del alcance

- **Que el generador conserve la `contentReference`** (F-H1). Va al backlog. La especificación
  («cannot be changed») y los packages de HL7 no son coherentes entre sí: la misma IG conserva y
  reemplaza. El incremento no depende de eso.
- Advertir sobre una `contentReference` a otra `StructureDefinition` sin relación (caso 3).

### Criterio de salida

- Los tres casos del issue dan lo esperado en R4, R4B y R5. El caso 1 se prueba en dos formas: solo
  diferencial (snapshot generado, con la referencia reemplazada) y snapshot que la conserva.
- Los tests existentes de `ChoiceType` pasan sin cambiar sus valores esperados.
- **Sin regresiones:** comparar la base vieja y la nueva en todos los elementos de elección del núcleo
  R4/R4B/R5 y de las IG cargadas. Toda diferencia se explica. Se lleva al repo la sonda de Fable
  (`TestAdvCompareOldNew`: 0 diferencias en 3.900 elementos con el orden del plan anterior).
- `make check` verde; `hl7diff run` 51/51 PASS; `hl7diff same` idéntico a `970e2c1`; barrido de claves
  de elección del #180 sin diferencias.
- Todas las mutaciones caen.

### Tests (paso 5), en R4, R4B y R5

Los valores esperados salen de la definición de `contentReference`, de slicing.html y de formats.html#choice:

1. Los tres casos del issue; el caso 1 en sus dos formas.
2. `contentReference` en un perfil de un recurso del núcleo (`Questionnaire.item.item.answerOption.value[x]`),
   con `base` y **sin `base`**. Solo sin `base` distingue el algoritmo nuevo del de hoy (F-H2).
3. Hijo que llega por el tipo del destino: `M.grp` de tipo `Extension`, `M.item` → `#M.grp`, perfil con
   `M.item.value[x]` (C-M2).
4. `contentReference` a un slice (`#Provenance.agent:Author`, como `provenance-relevant-history`), por
   URL con versión (`url|version#id`) y en ciclo (C-M4).
5. Slice de elección sin tipo con hijos (`Observation.value[x]:valueTiming` sin `type`,
   `…repeat.bounds[x]`) (F-H3).
6. Snapshot publicado cuyo slice no repite `type` (caso 2) y reslice sin tipos.
7. Slice con tipos más estrechos que el cortado: mandan los del slice.
8. Perfil que despliega un tipo con `contentReference` interna (`T.item.item.value[x]`).
9. Destino que no existe: sigue al respaldo, sin pánico.

Las fixtures con `baseDefinition` `…/Base` lo anotan: `Base` no existe en R4 ni en R4B, y `definedIn`
corta ahí (F-H5).

### Validación (paso 7)

- `hl7diff same` idéntico a `970e2c1`; barrido de claves de elección (#180); comparación vieja/nueva de
  bases en todos los packages cargados.
- Sonda HL7 del caso 2, si `hl7diff` admite un perfil con snapshot propio. Si no, se registra por qué.
- **Mutaciones:**
  - m1: no seguir la `contentReference` de la base del padre;
  - m2: tomar la base del padre del snapshot (`P`) en vez de su definición;
  - m3: no seguir el tipo del destino;
  - m4: sin protección de ciclos (test con tiempo límite);
  - m5: no acotar por el nombre del slice de elección;
  - m6: los tipos de `E` antes que los de `P`;
  - m7: buscar el destino por ruta y no por id;
  - m8: `rel` desde el id en vez de la ruta.
- **Costo:** una base por nodo, con caché (la recursión reusa la del padre). Se miden las asignaciones
  con el bench de ejemplos contra `970e2c1`.

### Excepciones a la regla 5 y dependencias

Ninguna: todo sale de `contentReference`, `type`, el id y los snapshots. Sin dependencias nuevas.

## 3. Ronda adversarial (30-10-2026)

Codex (`--write`) no pudo escribir en su copia (sandbox): prototipó en Python sobre los packages. Fable
probó en Go en su copia.

| # | Sev. | Hallazgo | Verificación | Resolución |
|---|---|---|---|---|
| C-H1 / F-H1 | alta | El generador reemplaza la `contentReference` por el tipo del destino; el plan nunca la ve y el caso 1 (solo diferencial) sigue mal | Confirmado en `snapshot.go` (`unroll`) y por la sonda de Fable (`Timing.repeat.item type=[Timing] cr=""`) | **Aceptado:** la base del padre sale de su definición (§2). Que el generador conserve la referencia va al backlog: HL7 publica ambas formas |
| C-M2 | media | Un hijo que el destino no declara y que llega por su tipo queda sin base | Prototipo de Codex | **Aceptado:** `childOf` sigue el tipo del destino; test 3 |
| C-M3 | media | Que un slice sin tipos herede los del cortado no tiene apoyo en el snapshot | `ElementDefinition.type` habla de diferenciales | **Aceptado:** se quita el respaldo de `SliceOf`; la base del slice sale del paso 1 |
| C-M4 | media | Un destino que es un slice pierde su identidad si se pasa a ruta | Scan de Codex: `#Provenance.agent:Author` | **Aceptado:** el destino se busca por id; test 4 con versión y ciclo |
| F-H2 | media | Los tests 2, 3 y 8 pasan hoy por `base.path` y no distinguen | Sonda QNB de Fable | **Aceptado:** variante sin `base`; caso 1 en dos formas |
| F-H3 | media | Slice de elección sin tipo: el cortado tiene varios tipos y se salta | Sonda OVT de Fable | **Aceptado:** se acota por el nombre del slice; test 5 |
| F-H4 | baja | Sin regresiones con el orden nuevo | 0 diferencias en 3.900 elementos | **Aceptado como evidencia:** la comparación pasa al repo y se repite con el algoritmo rehecho |
| F-H5 | baja | `Base` no existe en R4/R4B; `rel` desde el id sobrevive | Fable | **Aceptado:** anotado en los tests; mutación m8 |

## 4. Aprobación (30-10-2026)

El usuario aprobó el plan de §2 con los hallazgos de §3 («aprobado»).

## 5. Tests primero (30-10-2026)

`pkg/registry/choice_base_test.go`: `TestChoiceTypeBaseElement` (tests 1–9 de §2, 33 consultas por versión)
y `TestChoiceTypeBaseElementCycle` (test 4, ciclo, con tiempo límite), en R4, R4B y R5. Sobre `970e2c1`
(`evidence/…/tests-red-970e2c1.txt`) fallan los tres casos del issue (el 1 en sus dos formas) y los tests
2, 3, 4, 5 y 8. Pasan, como deben, los que protegen lo que ya funciona: caso 2 con diferencial, tests 7 y 9
y el ciclo.

**Hallazgo del generador (backlog, fuera del alcance de §2):** un perfil que solo con diferencial nombra
`M.item.value[x]`, con `M.item` → `#M.grp` y `M.grp` de tipo `Extension` sin hijos en el snapshot del
modelo, pierde el elemento («differential element M.item.value[x] is left out: no element
M.item.value[x] in the base»). `unroll` copia los hijos que el destino lista (`unrollFrom`) y no despliega
el tipo del destino. El test 3 se prueba con el snapshot que conserva la referencia.

## 6. Implementación (30-10-2026)

`pkg/registry/registry.go`:

- **`baseElement`:** paso 1 y paso 3 sin cambios. El recorrido de ancestros se reemplaza por el paso 2 de
  §2: `childOf(choiceBase(P) o P, P, rel)`. La base del padre pasa por la caché de `choiceBase`, así que
  cada nodo se calcula una vez.
- **`childOf`:** busca el hijo por id en la definición de `E`, si no es un perfil, y si no por ruta en su
  cadena. Si no está, sigue la `contentReference` de `E` y, por último, el tipo que despliega. Un mapa
  de nodos visitados corta los ciclos, incluido el de volver a la raíz del mismo tipo.
- **`unrolledType`:** toma los tipos de `P` o, si no tiene, los de `E`. Entre varios, el `sliceName`
  de un slice de elección elige uno con `ChoiceTypeIndex`. Exige un único tipo cargado.

Tests agregados en el paso 7 por las mutaciones m9 y m10 (§7):

- **Test 10:** un perfil cuya base no está cargada. El tipo de su elemento da la base.
- **Test 11:** un elemento huérfano, porque al snapshot le falta un intermedio y `ElementTree.orphan`
  lo cuelga del contenedor. Lo resuelve `base.path`, el paso 3.

## 7. Validación (30-10-2026)

Todo en `evidence/2026-10-30-choice-base-element/` salvo el bench.

- **Comparación vieja/nueva** (`compare-bases.txt`, sonda `compare-bases_test.go.txt`, se copia a
  `pkg/registry` para correrla): la base de cada elemento de elección del núcleo R4, R4B y R5 y de 24 IG
  (R4) más extensions.r5, con el `baseElement` de `970e2c1` y con el nuevo. 0 diferencias de nodo y de
  tipos. Los 8 elementos sin base lo están con los dos: sus perfiles derivan de definiciones que la
  sonda no carga (`patient-birthPlace|5.3.0-ballot-tc1`, `…/5.0/StructureDefinition/profile-Task` de
  xver). Con m9 aplicada también da 0 diferencias: ningún elemento publicado depende del paso 3. Lo
  cubre el test 11.
- **Mutaciones** (`mutations.py`, `mutations.txt`): 9 de 10 caen. m9 y m10 sobrevivían y motivaron los
  tests 11 y 10. **m8 es equivalente:** el id de un elemento es su ruta con los nombres de slice
  (elementdefinition.html#id) y el árbol cuelga cada nodo por id, así que `rel` sale igual del id y de
  la ruta. Para un id mal formado la especificación no define nada que probar.
- **`hl7diff run`** (`hl7diff-run-970e2c1.txt`): 51 grupos PASS. **`hl7diff same -heavy`**
  (`hl7diff-same-970e2c1.txt`): IDENTICAL.
- **Barrido de claves de elección del #180** (`sweep.txt`): 0 diferencias y 146 KNOWN. `970e2c1` da
  lo mismo, línea por línea (`sweep-970e2c1.txt`). Los dos KNOWN que el #180 contaba y ya no aparecen
  (`PlanDefinition-protocol-example`, `definitionCanonical`, R4 y R4B) cambiaron antes de este
  incremento.
- **Costo** (`../2026-10-30-2153-bench-compare-970e2c1b1e1a/report.md`, `COUNT=6 CPUS=1,8 make
  bench-compare BASE=970e2c1`): sin diferencias significativas en tiempo (p ≥ 0,24), B/op, allocs/op ni
  heap vivo.
- **Sonda HL7 del caso 2:** no se corrió. El caso necesita un perfil con snapshot propio sin `type` en
  el slice, y `hl7diff` no tiene grupos de perfiles con snapshot publicado a mano. Queda pendiente. El valor esperado del test sale de slicing.html: un slice deriva del elemento que
  corta, `Patient.extension`, de tipo `Extension`.

## 8. Code review

Desde el 30-10-2026 los revisores son solo Codex y el código después del plan lo escribe un
subagente Sonnet. Es decisión del usuario y está registrada en `~/.claude/CLAUDE.md`.

### Ronda 1 (30-10-2026)

Fable se cortó por un error de autenticación sin dar informe. Dejó dos sondas en su copia: concurrencia
con `-race` mientras se cargan definiciones, y un slice destino de una referencia que no repite
`value[x]`. Corridas sobre el código de §6, las dos pasan.

Codex trabajó solo con lectura, porque su sandbox no escribe en el scratchpad. No dejó hallazgos de
severidad media o mayor.

| # | Sev. | Hallazgo | Verificación | Resolución |
|---|---|---|---|---|
| C1 | baja | Que el `sliceName` de un slice de elección sin tipo elija el tipo no tiene apoyo en la especificación | profiling.html (R4 y R5) no ata el nombre de un slice a un tipo. Comparación de bases con m5 aplicada: 0 diferencias, ningún elemento publicado depende de eso | **Aceptado:** cambia la decisión de F-H3 (§3). `unrolledType` no acota por nombre: un slice sin tipos de un elemento con varios no tiene base y se queda con sus tipos. El test 5 lo documenta; el caso conforme, un slice que declara Timing, es el test 7. Sale la mutación m5 |
| C2 | baja | `qNoBase` viola sdf-8b (snapshot sin `base`) | Cierto | **Aceptado:** se anota en el test. Se mantiene porque solo sin `base` distingue la estructura de `base.path` (F-H2) |

### Ronda 2 (30-10-2026)

Codex revisó solo el cambio de la ronda 1 y no dejó hallazgos de severidad media o mayor: **el review
converge**. Las mutaciones completas se corrieron antes de la ronda: 8 de 9 caen (m5 ya no existe) y
m8 sigue siendo equivalente. La comparación de bases sigue en 0 diferencias.

| # | Sev. | Hallazgo | Resolución |
|---|---|---|---|
| C3 | baja | `ensureSlice` (`snapshot.go`) también deduce el tipo de un slice de elección por su nombre | Anterior a este incremento y fuera de su alcance: **backlog**, con el generador |
| C4 | baja | El test 5 prueba una entrada incompleta, no una regla normativa | **Aceptado:** el comentario lo dice |
| C5 | baja | El comentario de `unrolledType` dice que el slice no tiene base; los que no la tienen son sus hijos | **Aceptado:** comentario corregido |

## 9. Cierre (30-10-2026)

- **Comprobaciones, corridas con el código final:**
  - `make check` en verde.
  - `hl7diff same -heavy` contra `970e2c1`: IDENTICAL (`evidence/…/hl7diff-same-970e2c1.txt`).
  - `hl7diff run` (51 PASS) se corrió antes del cambio C1. C1 solo toca slices de elección sin tipo, y
    ningún elemento publicado ni ningún grupo depende de eso: lo muestran la comparación de bases con
    m5 y `same`, que da IDENTICAL.
- **Criterio de salida (§2):** se cumple, con dos salvedades:
  - las mutaciones dan 8 de 9 y m8 es equivalente (§7);
  - el test 3 en forma diferencial depende del generador (§5).
- **Backlog:**
  - el generador conserva la `contentReference` (F-H1);
  - el generador no despliega el tipo del destino de una referencia (§5);
  - `ensureSlice` deduce el tipo por el nombre del slice (C3);
  - aviso del caso 3;
  - sonda HL7 del caso 2 (§7).
- Sin commit ni push.
