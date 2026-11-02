# #189: valores primitivos bajo un elemento con contentReference

Rama `fix/content-reference-primitives`, desde `8730fc4`. Evidencia: `evidence/2026-11-02-content-reference-primitives/`.

## 1. Revisión previa

**Especificación.** Un elemento con `contentReference` no declara tipo: su contenido es el del elemento al que
apunta (elementdefinition.html#ElementDefinition.contentReference, R4, R4B y R5: «Identifies an element
defined elsewhere in the definition whose content rules should be applied to the current element»). Un hijo
de `Questionnaire.item.item` es un hijo de `Questionnaire.item`, con su tipo; un valor primitivo con el
formato equivocado es un error en cualquiera de los dos lugares (json.html#primitive; datatypes.html).

**En los packages core** (`~/.fhir/packages`, solo `specialization`): 55 elementos en 30 tipos (R4), 58 en 32
(R4B), 78 en 38 (R5); todos con la forma `#id` y un id igual a la ruta. Un perfil puede usar `url#id` y un id
con slice (R5: «slices included»); `registry.ContentReference` resuelve las dos.

**Sondas** (`probe/in/`, 9 instancias; `probe/gofhir.txt` con `main` en `8730fc4`, `probe/hl7.txt` con
validator_cli 6.10.2, `-tx n/a`, en las tres versiones):

- HL7 informa el formato del primitivo bajo la contentReference igual que en el elemento referido: un string
  vacío, un decimal y un entero escritos como string, un boolean escrito como string, un uri con espacio, y el
  `valueBoolean` de una extensión. También a dos niveles (`item.item.item`).
- gofhir solo los informa en el elemento referido, en las tres versiones.

**Sonda metamórfica** (`probe/meta/`, `probe/meta.txt`): el mismo `item` en `Questionnaire.item[0]` y en
`Questionnaire.item[0].item[0]`, 13 variantes × 3 versiones, todos los issues de gofhir con la ruta
normalizada. Iguales: código fuera de su ValueSet, operador inválido, dos variantes, propiedad desconocida,
`linkId` ausente. Distintos: **solo** los errores de la fase de primitivos (8 de 13 variantes, en las tres
versiones). Las demás fases ya siguen la contentReference: la estructural reescribe la ruta
(`contentRefPath`), cardinalidad y slicing usan `registry.ChildrenOf`, la de extensiones `ContentReference`.

**Causa.** `pkg/primitive/primitive.go`: `resolveElementDefinition` busca por la ruta de la instancia en
`idx.byPath`; el elemento con contentReference no tiene tipo, y `validateComplexElement` vuelve sin recorrer
sus hijos (`typeName == ""`).

**gofhir/server (congelado)**, `internal/fhircontext/navigate.go` (`Cursor.Child`): ante una contentReference,
sigue en la ruta referida de la misma definición.
- **Tomar:** seguir la referencia y recorrer los hijos del elemento referido.
- **Mejorar:** el server corta el string en `#` y supone que la definición es la propia, que el id es la ruta y
  que el tipo es «BackboneElement» (nombre escrito en el código). Aquí se resuelve con
  `registry.ContentReference` (id exacto; `url#id` en otra definición), sin nombres de tipo.
- **Descartar:** nada más; el resto del cursor no aplica a esta fase.

## 2. Plan

**Alcance.** La fase de primitivos sigue la contentReference de un elemento: recorre sus hijos con la
definición del elemento referido (la misma u otra, si la referencia es `url#id`), a cualquier profundidad.

**Diseño.**
- `elementIndex` guarda su StructureDefinition.
- Cuando el elemento resuelto tiene `ContentReference`, `validateComplexElement` toma su nodo
  (`sd.Tree().ByID(elemDef.ID)`), lo resuelve con `registry.ContentReference` y sigue con
  `validateElement(data, target.Def.Path, fhirPath, índice de la definición del destino, …)`.
- La definición del destino: la propia salvo `url#id` de otra; para no repetir la regla de
  `ContentReference`, el registro la devuelve (se evalúa en el plan adversarial si conviene una función que
  devuelva nodo y definición, que `childrenOf` también use).
- Una referencia que no resuelve (o un nodo sin id): no se recorren los hijos; la estructural ya informa lo que
  corresponda (`TreeIssueContentReference` al cargar).
- Cadenas: el recorrido sigue a la instancia, que es finita; una referencia a sí mismo se resuelve en cada
  nivel. Sin límite de saltos adicional (`childrenOf` lo tiene porque se encadena sin instancia).

**Fuera.**
- Pasar la fase entera a `registry.ChildrenOf` (el árbol): unificaría el recorrido con cardinalidad y slicing,
  pero es un cambio de arquitectura mayor. Candidato a frente propio.
- ele-1 sobre un string vacío (HL7 lo informa también en la raíz; gofhir no, en ningún lugar): otro frente.
- Un destino con slice: ver §3, hallazgo 3.

**Criterio de salida.**
1. Las sondas de §1: los errores de primitivos bajo la contentReference, iguales a los del elemento referido
   (ruta normalizada) en R4, R4B y R5.
2. Test metamórfico sobre **todas** las contentReferences de los packages core de las tres versiones: para
   cada elemento con contentReference y cada hijo primitivo del referido, un valor mal formado en el referido y
   bajo la contentReference da los mismos issues de primitivos (ruta normalizada). Cientos de casos por versión.
3. hl7diff: un grupo nuevo con las sondas, sin hallazgos; el resto idéntico a `8730fc4` (`hl7diff same`).
4. Mutaciones de los oráculos; bench sin asignaciones nuevas por objeto en recursos sin contentReference.

**Tests.**
- `pkg/primitive`: casos escritos a mano con la cita de elementdefinition.html (los de §1, dos niveles, raíz vs.
  anidado, contenido en un Bundle, una extensión).
- Perfil con `url#id` a otra definición y con un destino con slice (testdata propia).
- El test metamórfico del criterio 2, generado desde los StructureDefinitions (sin nombres escritos).
- Valores esperados: de HL7 (sondas) y de la regla metamórfica, no del código propio.

**Excepciones a la regla 5:** ninguna. **Dependencias nuevas:** ninguna.

## 3. Ronda adversarial (02-11-2026)

Revisor Opus independiente (excepción mientras Codex no tiene cuota, hasta el 04-11-2026), sobre una copia sin
`.git`, con el plan, la carta y las rutas. Sondas del revisor: `evidence/…/adversarial/`.

| # | Hallazgo | Verificación | Resolución |
| --- | --- | --- | --- |
| 1 alto | El plan supone que el destino es un contenedor en línea (BackboneElement): sigue con `validateElement` en su ruta. Un `url#id` de un perfil puede apuntar a un elemento de un tipo de datos; sus hijos se buscarían bajo la ruta y se saltarían | En el core los 191 destinos son BackboneElement (revisor, jq); el código no lo comprueba | Aceptado: se vuelve a entrar por `validateComplexElement` con la definición del destino, que despacha por tipo; con un tope de saltos |
| 2 alto | Cada fase resuelve la contentReference a su manera: la estructural toma el id como ruta en la misma definición (ignora `url#`; un id con slice no encuentra hijos); `ChildrenOf` usa primero los hijos propios del snapshot | Confirmado: `structural.go:548` sigue con `contentRefPath` y el índice propio | Aceptado: el orden de `ChildrenOf` (los hijos propios si el snapshot los desenrolla; si no, la referencia) y un helper del registro que devuelva definición y nodo, compartido con `childrenOf`. La estructural queda fuera (un frente a la vez): issue aparte, borrador `issue-structural-content-reference-draft.md` |
| 3 medio | El índice por ruta guarda la **última** slice, no la primera | Confirmado: `idx.byPath[elem.Path] = elem` sobrescribe | Aceptado: texto corregido; el test no depende de qué slice guarda el índice |
| 4 medio | El oráculo metamórfico compara gofhir con gofhir: no ve un defecto común a la raíz y al anidado. `_required` con una extensión `valueBoolean: "true"` no da error en ningún lugar; HL7 sí | Confirmado con HL7 6.10.2 y gofhir (`adversarial/root-ext.json`): HL7 informa el valor en los dos lugares; gofhir, solo «Unknown extension» | Aceptado: casos anclados en HL7 (una muestra por clase, en hl7diff); el metamórfico incluye variantes de elección y descendientes de tipos de datos (`code.system`); un objeto donde va un array. El `_x` es otro defecto: issue aparte, borrador `issue-primitive-extension-values-draft.md` |
| 5 medio | `Repeats()` sale del elemento con contentReference, no del destino (R5 `TestScript.test.action.assert` max 1 → destino `*`) | Lectura; el flujo del plan conserva el `resolved` externo | Aceptado: caso metamórfico con un array bajo una contentReference de max 1 |
| 6 bajo | Costo: `Tree().ByID`, `ContentReference` y `getOrBuildIndex` por objeto | Lectura | Aceptado: la resolución se hace una vez por elemento y se guarda en el índice; bench con un Questionnaire profundo y sin contentReference |
| 7 bajo | Cadenas (un destino que también es contentReference) se saltarían | Sin cadenas en el core (revisor) | Aceptado: cubierto por el 1 y el tope; test sintético |
| 8 bajo | R5 `Bundle.entry.link` → `#Bundle.link` combina un recurso contenido y una contentReference | Lectura | Aceptado: en los tests a mano de R5 |

### Plan ajustado

- **Resolución compartida:** `registry` expone una función que devuelve la definición y el nodo de destino de
  una contentReference (la regla `#id` / `url#id` de `ContentReference`); `childrenOf` la usa.
- **Fase de primitivos:** si el elemento resuelto tiene `ContentReference` y el índice no tiene hijos bajo su
  ruta, se resuelve el destino una vez (cache en el índice: ruta → destino e índice de su definición) y se
  vuelve a entrar por `validateComplexElement` con la definición del destino y su ruta; `Repeats()` sigue
  saliendo del elemento propio. Tope de saltos (`maxContentReferenceHops`).
- **Tests:** los de §2, más: destino de un tipo de datos (`url#id` en un perfil), cadena sintética, array bajo
  max 1, objeto donde va un array, R5 `Bundle.entry.link`. Metamórfico con variantes de elección y
  descendientes de tipos de datos. Una muestra por clase anclada en HL7 (grupo hl7diff).
- **Fuera:** la estructural (issue aparte) y los valores de extensiones en `_x` (issue aparte).

## 4. Aprobación

El usuario aprobó el plan ajustado (02-11-2026).

## 5. Tests (Sonnet, antes del código)

`pkg/validator/content_reference_primitives_test.go`, en R4, R4B y R5; perfiles de prueba en
`pkg/validator/testdata/content-reference/`.
- **Sondas:** las del §1, con la ubicación que da HL7 (`probe/hl7.txt`), y los controles sin error; también
  el Observation dentro de un Bundle.
- **Metamórfico:** generado desde los StructureDefinitions, sin nombres escritos. Recorre cada contentReference
  del core y cada hijo primitivo del destino: directos, variantes de elección, hijos en línea y un nivel dentro
  de un tipo de datos. Usa 7 valores mal formados, y exige que haya issues en el destino, así que la comparación
  nunca es vacía. Casos y tiempos en `metamorphic.txt`.
- **Bordes:** un objeto donde va un array y un array de objetos. R5 `TestScript.test.action.assert` (max 1,
  destino `*`) y `Bundle.entry.link`.
- **Perfiles:** `url#id` a un modelo lógico con un destino Quantity; cadenas en la misma definición y entre
  definiciones; un destino con slice; un ciclo, que termina.
- **Antes del código** (`tests-before.txt`): fallan todos salvo los controles y los perfiles válidos.

## 6. Implementación (Sonnet)

- `pkg/registry`: `ContentReferenceTarget`, que devuelve la definición y el nodo del destino.
  `ContentReference` y `childrenOf` la usan, así que la regla de `url#id` vive en un solo lugar.
  `MaxContentReferenceHops` queda exportada.
- `pkg/primitive`: el índice guarda su definición, las rutas con hijos en el snapshot y el destino de cada
  contentReference, resuelto una vez (`sync.Map`). Un elemento sin tipo con contentReference sigue su destino
  por `validateContentReference`:
  - los hijos propios van primero, como en `ChildrenOf`;
  - después sigue el destino con su definición, volviendo a entrar por `validateComplexElement`, con el tope
    de saltos;
  - `Repeats()` sigue saliendo del elemento propio.

## 7. Validación

- **hl7diff:** tres grupos nuevos, `content-reference-probes`, `-r4b` y `-r5`, con las sondas (`kr_*`) y, en
  R5, `kr5_bundle-entry-link`. Se borraron tres sondas cuyo JSON igualaba al de otra: hl7diff las deduplica.
  - La primera corrida dio 7 hallazgos por grupo: `TYPE_WRONG_JSON_TYPE` no tenía familia, porque el parser de
    HL7 informa ese error sin ID de mensaje.
  - Se agrega la familia `primitive-json-type` (`hl7NoID`: código `invalid`, texto «Error parsing JSON: the
    primitive value must be a »), con la evidencia de las sondas: HL7 lo informa en la misma ubicación y por
    el mismo valor. El test `TestPrimitiveJSONTypePairsWithHL7ParserError` lo fija.
  - Resultado: 55 grupos PASS (`hl7diff-run.txt`). `hl7diff same -heavy` sale idéntico a `8730fc4`
    salvo en los tres grupos nuevos (`hl7diff-same-8730fc4.txt`).
- **Mutaciones** (`mutations.py`):
  - `mutations-1.txt`: m5 y m7 no aplicaban; m6 era equivalente, porque solo cambiaba la clave de lectura; m9
    sobrevivía porque el texto de la familia nueva no tenía test. Se corrigieron y se agregó el test.
  - `mutations-2.txt`: de 8, mueren 8.
  - m2 (seguir la referencia aunque el snapshot desenrolle los hijos) se retiró por ser equivalente en esta
    fase: el snapshot copia los hijos del destino, y las variantes se nombran por los tipos de la base (#180).
    El orden se mantiene por coherencia con `ChildrenOf`.
- `GOTOOLCHAIN=go1.25.13 make check`: `make-check.txt` (exit 0).
- **Pendiente:** bench, en el cierre.

## 8. Code review

### Ronda 1 (02-11-2026)

Revisor Opus independiente (excepción mientras Codex no tiene cuota), sobre `scratchpad/rev189`, sin mi
razonamiento. Verificó con mutaciones y con `-race` (16 goroutines sobre un validador: limpio).

| # | Hallazgo | Verificación | Resolución |
| --- | --- | --- | --- |
| 1 medio | La rama «hijos propios primero» no tiene test, y m2 no es equivalente: con un destino que no resuelve (`url#id` a una definición no cargada) y un hijo desenrollado de tipo boolean, `"yes"` da TYPE_WRONG_JSON_TYPE; con la mutación, nada | Mutación del revisor: toda la suite pasa con `if false && …` | Aceptado: test del caso, m2 de vuelta en `mutations.py`. La conclusión del §7 sobre m2 era errónea |
| 2 bajo | Los hijos propios se buscan por ruta (`parents`), `ChildrenOf` por nodo: una slice que desenrolla hijos vale para toda la ruta | Lectura; igual resultado con un snapshot desenrollado completo, y la fase ya trabajaba por ruta | Sin cambio de código; queda registrado aquí |
| 3 bajo | `getOrBuildIndex` (código previo) hace `Load` y `Store`: dos goroutines pueden construir dos índices por URL | Lectura | Aceptado: `LoadOrStore` |
| 4 bajo | Comentarios del test en inglés | Lectura | Aceptado: en español. El doc de `ContentReferenceTarget` sigue en inglés, como el resto de `tree.go` |

Fuera de #189 (el revisor): una extensión en `_required` con `"valueInteger":"1"` no se informa ni en la raíz
ni bajo la contentReference; es el mismo defecto del borrador `issue-primitive-extension-values-draft.md`.

Correcciones de la ronda 1 (Sonnet):
- caso `Observation.missingRef` en `StructureDefinition-cref-profile.json`, con dos casos en
  `TestContentReferencePrimitivesProfiles`;
- `LoadOrStore` en `getOrBuildIndex`;
- comentarios del test en español.

`make check` en verde. Mutaciones (`mutations-3.txt`, con m2 de vuelta): de 9, mueren 9.

### Ronda 2 (02-11-2026)

Revisor Opus independiente, sobre `scratchpad/rev189b`, con los hallazgos de la ronda 1 y el diff de sus
correcciones.

| # | Hallazgo | Verificación | Resolución |
| --- | --- | --- | --- |
| 1 medio | El orden «hijos propios antes que la contentReference, en cada salto» (como `ChildrenOf`) solo se prueba cuando el destino no resuelve | Dos mutaciones sobreviven a toda la suite: A (el destino primero; los propios solo si no resuelve) y B (los propios solo en el primer salto) | Aceptado: casos `ownFirst` y `midA → midB → chainC` en el perfil de prueba; A y B como m10 y m11 |
| 2 nit | Comentarios con inglés: `primitive.go` («Build and cache:») y el test | Lectura | Aceptado |

El revisor verificó las correcciones de la ronda 1: la expectativa del caso `missingRef` sale de la especificación
(los hijos del snapshot son las restricciones que se aplican, como en `ChildrenOf` y `getChildList` de HL7), y
`LoadOrStore` no cambia nada para una definición sin URL.

Correcciones de la ronda 2 (Sonnet):
- casos `ownFirst` y `midA → midB → chainC`;
- m10 y m11 en `mutations.py`;
- comentarios en español.

Mutaciones (`mutations-4.txt`): de 11, mueren 11.

### Ronda 3 (02-11-2026)

**Converge**: sin hallazgos de severidad media o mayor. El revisor verificó que las expectativas siguen el orden
de `ChildrenOf` y que el snapshot del perfil está bien formado (eld-5). Un bajo:

| # | Hallazgo | Verificación | Resolución |
| --- | --- | --- | --- |
| 1 bajo | En el último salto de la cadena, el bucle no mira los hijos propios del nodo; `childrenOf` sí los mira antes de su tope | Test del revisor: una cadena k0…k8 con un hijo propio en k8 da `[]` en vez de TYPE_WRONG_JSON_TYPE | Aceptado: el bucle sigue el orden de `childrenOf` (hijos propios, tope, salto); test de la cadena |

Corrección (Sonnet): el bucle sigue el orden de `childrenOf` (hijos propios, tope, salto), y se agregó un test de
una cadena de `MaxContentReferenceHops` saltos (k0…k8). Se reescribieron m4 y m10 sobre el código nuevo y se
agregó m12 (el tope antes de los hijos propios). Mutaciones (`mutations-5.txt`): de 12, mueren 12.

## 9. Cierre (02-11-2026)

- `GOTOOLCHAIN=go1.25.13 make check`: `make-check-final.txt` (exit 0).
- **hl7diff:**
  - Los tres grupos nuevos dan PASS sobre el código final (`hl7diff-content-reference-probes*-final.txt`).
  - La corrida completa del §7 (`hl7diff-run.txt`) dio 55 grupos PASS; desde entonces solo cambió el bucle
    de cadenas.
  - `hl7diff same -heavy` (`hl7diff-same-8730fc4-final.txt`) sale idéntico a `8730fc4` en todos los grupos
    salvo los tres nuevos.
- **Bench** (`evidence/2026-11-02-1855-bench-compare-8730fc485acb/`; carga en `bench.log`): las asignaciones
  son iguales a la base en todos los benchmarks. Con carga de otras sesiones de 16 a 22, el tiempo no se puede
  medir.
- **Questionnaire profundo** (`bench-questionnaire/`, benchmark temporal `zz_cref_bench_test.go.txt`, que se copia a `pkg/validator` como `_test.go` para
  correrlo): con la rama hay más asignaciones que en la base, porque ahora se revisan los primitivos de los
  ítems anidados. Ese costo no supera el de los mismos ítems en la raíz (`BenchmarkValidateFlatQuestionnaire`,
  igual en base y rama), así que seguir la contentReference no agrega costo propio por objeto. Cifras en
  `bench.txt`.
- **Pendiente:** publicar los dos issues aparte, cuando el usuario lo apruebe:
  - `issue-primitive-extension-values-draft.md`, valores de extensiones en `_x`, listo;
  - `issue-structural-content-reference-draft.md`, la estructural con `url#id`; le falta la reproducción con un
    perfil.

**Para el commit:** `fix(primitive): validate primitive values under an element defined by contentReference`.
Agrega `registry.ContentReferenceTarget` y `MaxContentReferenceHops` exportada, y la familia hl7diff
`primitive-json-type`. Closes #189.
