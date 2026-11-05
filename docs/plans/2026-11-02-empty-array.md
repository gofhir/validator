# #190: un array JSON vacío se acepta

Rama `fix/empty-array`, desde `68589e6`. Proceso: skill `fhir-incremento` (carta §10). Revisor independiente:
Opus (excepción mientras Codex no tiene cuota, hasta el 04-11-2026 03:52).

## 1. Revisión previa

### Especificación

- **R4 y R4B**, json.html §2.6.2.1 «Comparison with XML»: «Just as in XML, JSON objects and arrays are never empty,
  and properties never have null values (except for a special case documented below). Omit a property if it is
  empty». La regla vale para todo array: de un elemento complejo, de un primitivo y el `_x` de un primitivo.
- **R5**: esa frase ya no está en json.html. Quedan:
  - formats.html: «FHIR elements can never be empty. If an element is present in the resource, it SHALL have either
    a value, child elements as defined for its type, or 1 or more extensions»;
  - json.html §2.1.6.4.2: «If the length of a JSON "element" array is different from the length of its JSON
    "_element" array, implementations SHALL infer null values in the suffix of the shorter array. If an array has
    only null elements, implementations SHOULD omit it entirely». La primera frase está solo en R5 (`grep` sobre las
    tres copias: R4 0, R4B 0, R5 1).

  Ninguna dice en R5, con todas sus letras, que un array vacío sea inválido. Además, R5 tolera un `_x` más corto
  que sus valores (se infieren nulls) y solo recomienda omitir uno de puros nulls (SHOULD). HL7 reporta igual los
  arrays vacíos de complejos en R5.
- Textos bajados en las tres versiones con `curl`; la cita sale de esas copias, no de memoria.

### `gofhir/server` (congelado, `4592d561`)

No valida arrays vacíos: delega en el validador. Solo evita escribir `tag: []` al armar un recurso
(`internal/service/golden_readonly.go:107`). **Descartar:** no hay lógica que tomar.

### HL7 validator_cli 6.10.2

El issue cita 7.0.3, pero el jar es la 6.10.2 (`hl7-*.log`, el mismo error que en #189). Con la 6.10.2 tampoco
sale ele-1 sobre `type`: la tabla del issue está mal en eso. Comentario de corrección en §Pendientes.

**Sondas**: `evidence/2026-11-02-empty-array/probe/`, `in/` con 25 instancias y `in2/` con 8, en R4, R4B y R5.
Resúmenes en `hl7.txt`, `hl7-2.txt`, `gofhir.txt` y `gofhir-2.txt` (`sum.py`); gofhir sobre `main` en `68589e6`.
Las tres versiones dan lo mismo, salvo R5 en e37 (Organization ya no tiene `telecom`) y bdl-3a en e21.

| Sonda | HL7 6.10.2 | gofhir `68589e6` |
|---|---|---|
| e01 `name: []` | `Array cannot be empty` en `Patient.name` | nada |
| e02 `given: []`, e03 `_given: []`, e04 los dos, e15 `_given: []` con valores, e19 `line: []` en una extensión | **nada** | nada |
| e05 `gender: []`, e06 `maritalStatus: []`, e12 `deceasedBoolean: []` (no se repiten) | solo la forma («must be a simple value / an Object, not an Array») | solo la forma |
| e07 `type: {coding: []}`, e25 `maritalStatus: {coding: []}`, e18 `meta: {tag: []}` | el array vacío en `….coding` / `….tag`, **sin ele-1 en el padre** | ele-1 en el padre |
| e08 `extension`, e09 `modifierExtension`, e10 `contained`, e14 `Bundle.entry`, e22 dos, e23 extensión de un elemento, e29 extensión de una extensión | el array vacío en cada uno | nada |
| e13 `Composition.author: []` (1..*) | el array vacío **y** la cardinalidad mínima | solo la cardinalidad |
| e20 bajo `contained`, e21 bajo `Bundle.entry.resource`, e37 en un `contained` | el array vacío en el recurso anidado | nada |
| e24 `_birthDate: {extension: []}`, e27 `_given: [{extension: []}]` | `Patient.birthDate.extension`, `Patient.name[0].given.extension` | nada |
| e31 `Questionnaire.item[0].item: []` (contentReference) | el array vacío y que-1 | solo que-1 |
| e35 `valueCodeableConcept: {coding: []}` | `Observation.value[x].coding`, sin ele-1 | ele-1 en `valueCodeableConcept` |
| e11 `foo: []` (desconocido) | solo «Unrecognized property» | solo el desconocido |
| e16 `name: [[]]`, e36 `given: [null]` | errores del parser, otro tema | otro tema |
| e17 control | nada | nada |

**Dónde lo decide HL7** (`probe/hl7-jsonparser-6.10.2.txt`, `javap` sobre el jar): `ARRAY_CANNOT_BE_EMPTY` sale
solo en `JsonParser.parseChildComplex`, y se omite cuando `Property.canBeEmpty()`. Eso es verdad cuando el
elemento tiene la extensión de herramientas `json-empty-behavior` con un valor distinto de `absent`.
`parseChildPrimitive` no tiene el chequeo, y por eso no se reportan `given: []` ni `_given: []`. Lo mismo en
`master` de `hapifhir/org.hl7.fhir.core` (02-11-2026): el bucle de `parseChildPrimitive` va hasta
`max(arrC(arr1), arrC(arr2))` y con un array vacío no corre. Extractos: `probe/hl7-jsonparser-6.10.2-parseChildComplex.txt`
y `probe/hl7-property-canBeEmpty-6.10.2.txt`. Todas las sondas, con `-tx n/a`.

**Tercera tanda** (`in3/`, `hl7-3.txt`, `gofhir-3.txt`), por la ronda adversarial:
- Un objeto que solo tiene un array vacío cuenta como ausente para las demás invariantes, en HL7 y en gofhir. f01:
  ext-1 se dispara en las dos. f02: obs-6 no se dispara en ninguna. f03: pat-1 se dispara en las dos.
- f03 `contact[0].name: {given: []}`: HL7 no reporta nada en `name` (ni ele-1 ni objeto vacío); gofhir reporta
  ele-1. El array vacío de un primitivo también cuenta como contenido escrito. (Con la 6.10.2. La 7.0.4 sí reporta
  ele-1 en `contact[0].name`: ver §10.)
- f05 `name: {extension: []}` (un objeto donde va un array): HL7 da la forma y el array vacío en
  `Patient.name.extension`, sin índice. f04 `name: {given: []}`: solo la forma.
- f07 y f08 (omitir en vez de `[]`): `Object must have some content` en el objeto, y las mismas invariantes.

- `json-empty-behavior`: ningún package de `~/.fhir/packages` la usa (búsqueda sobre los
  `StructureDefinition-*.json`). Es para modelos lógicos del IG de herramientas. **Fuera del alcance.**
- **Arrays vacíos en los ejemplos oficiales:** 0 de 5307 (R4), 0 de 2841 (R4B) y 0 de 2823 (R5). No se espera
  ningún cambio en el corpus; los datos variados del §7 salen de mutaciones.

### Código actual

- `pkg/structural/shape.go` revisa la forma de cada valor de un array y no tiene nada que revisar en uno vacío.
- `Normalize` (con `ctx.edits`) ya juzga un objeto vacío sobre la fuente (`STRUCTURE_EMPTY_OBJECT`). Además
  registra el objeto que lo contiene en `holders`, y `ContentJudged` y `HadContent` le quitan solo ele-1
  (`pkg/constraint/tree.go:444`). Es el mismo mecanismo que hace falta para e07, e18, e25 y e35.
- **Paths bajo `_x`:** la fase estructural nombra `Patient._birthDate.foo` (e26), donde HL7 nombra
  `Patient.birthDate` y las otras fases de gofhir `Patient.birthDate.extension[0]`. Es anterior a #190 y no es de
  #199 (#199 es la fase de primitivos). Se redacta un issue aparte (§Pendientes).

## 2. Plan

### Decisiones

- **D1.** Un array vacío de un elemento que se repite se reporta en el propio elemento, sin índice:
  - diagnóstico nuevo `STRUCTURE_ARRAY_EMPTY`, error;
  - código `structure`, como sus hermanos (`STRUCTURE_EMPTY_OBJECT` y los de forma). HL7 usa `invalid`; hl7diff
    empareja por ID, no por código;
  - plantilla: `'{element}' is an empty array: a property with no values must be omitted`.
- **D2 (decidida en el paso 4, 02-11-2026: «según cada versión»; [ADR-003](../adr/ADR-003-empty-primitive-arrays-by-release.md)).**
  Complejos y recursos, en todas las versiones. Primitivos (`given: []`) y `_x` (`_given: []`), solo en STU3, R4 y
  R4B, cuyo json.html dice «arrays are never empty». En R5, y en cualquier versión fuera de la lista, no se reportan.
  Lo que sigue es el texto original de D2, que vale para R4 y R4B.
  Vale para todo elemento que se repite: complejo, recurso (`contained`, `Bundle.entry.resource` anidado),
  primitivo (`given: []`) y el `_x` de un primitivo que se repite (`_given: []`).
  - Para primitivos y `_x`, HL7 no lo reporta (`parseChildPrimitive` no lo revisa). Gana la especificación
    (R4/R4B json.html §2.6.2.1). Va como divergencia declarada en `declared-divergences.json` y como issue
    redactado para `hapifhir/org.hl7.fhir.core` (principio 10).
  - **Decisión del usuario en el paso 4:** si D2 incluye R5, donde el texto ya no lo dice explícito (§1). Se
    recomienda que sí: el formato es el mismo y HL7 lo aplica igual a los complejos en R5.
- **D3.** Un elemento que no se repite escrito `[]` deja solo el error de forma que ya existe (paridad e05, e06,
  e12, e28). Un elemento desconocido escrito `[]` deja solo el desconocido (e11).
- **D4** (revisado el 03-11-2026, §10; antes seguía a la 6.10.2). Un array vacío no es contenido: el objeto que lo
  contiene, de un complejo o de un primitivo, recibe ele-1 si no tiene otro contenido (paridad con la 7.0.4 en
  e07, e18, e25, e35, f01, f02 y f03). No se registra en `edits.holders`. Las demás
  invariantes siguen igual (que-1 en e31). La cardinalidad cuenta 0 y se reporta además (paridad e13).
- **D5.** Se reporta solo en `Normalize` (con `ctx.edits`), como `STRUCTURE_EMPTY_OBJECT`; la fase estructural
  sin `edits` no lo repite. El array no se borra del recurso (no hay `apply`): para FHIRPath, `[]` ya es vacío.
- **D7.** Un array vacío en el mismo objeto como `given` y como `_given` (e04) da dos issues en el mismo path, uno
  por propiedad JSON (`{element}` distinto). Se fija en la tabla.
- **D8.** El chequeo va antes de la compuerta `primitiveKind` de `checkShape`: depende solo de `Repeats()`. Hoy no
  cambia nada en el core (0 elementos con varios tipos sin `[x]` ni contentReference, según el revisor).
- **D6.** Bajo `_x` (e24, e27), el path es el que la fase estructural ya da a lo que hay ahí
  (`Patient._birthDate.extension`), coherente con los demás issues de la fase. La diferencia con HL7
  (`Patient.birthDate.extension`) es anterior y se resuelve en el issue aparte de §1. hl7diff empareja e24
  (`ComparableGoLocation` reescribe `._b` → `.b`) pero no e27: HL7 da `Patient.name[0].given.extension`, sin índice
  en `given`. e27 se declara divergencia en los dos lados, con referencia a ese issue.

### Alcance

**Dentro:**
- D1–D6.
- `pkg/issue` (diagnóstico).
- `pkg/structural/shape.go` (el chequeo en `checkShape` y `checkShadowShape`).
- La familia de hl7diff (`empty-array`: `STRUCTURE_ARRAY_EMPTY` ↔ `ARRAY_CANNOT_BE_EMPTY`) y las divergencias.

**Fuera:**
- `json-empty-behavior`.
- `[[]]` y `[null]` (e16, e36).
- Los paths bajo `_x` (issue aparte).
- #199.

### Criterio de salida

- Las sondas e01–e37 dan lo que da HL7 en las tres versiones, salvo las divergencias declaradas (D2 primitivos,
  D6 si aplica).
- El oráculo metamórfico del §7 sin diferencias.
- hl7diff `run` en PASS con los grupos nuevos; `same` sin cambios fuera de ellos.
- Mutaciones: todas mueren.
- `make check` en verde.
- Asignaciones del bench iguales a la base: el chequeo es `len(items) == 0` sobre un array que ya se recorre.

### Tests y datos (§7)

1. **Tabla a mano** (`pkg/structural` o `pkg/validator`), R4, R4B y R5, con la cita de json.html en un comentario.
   Los valores esperados salen de las sondas de HL7 (`probe/hl7*.txt`); los de D2 para primitivos, de la
   especificación.
2. **Metamórfico** sobre los ejemplos oficiales de las tres versiones (`hl7.fhir.r4.examples`, `r4b`, `r5`), con
   una muestra fija y determinista de varios cientos de recursos por versión:
   - **(a) agregar:** en un objeto al azar, una propiedad `[]` de un hijo que se repite y no está, sacado del
     snapshot. Se espera exactamente un `STRUCTURE_ARRAY_EMPTY` nuevo en ese path, y ningún otro issue cambia
     (en particular, ningún ele-1 nuevo). Para uno con `min ≥ 1`, también la cardinalidad.
   - **(b) vaciar:** un array existente pasa a `[]`. Se espera el mismo issue en ese path. Los demás cambios
     posibles (cardinalidad, invariantes que leen ese hijo) se miden contra la misma instancia con la propiedad
     omitida, que según la especificación es lo equivalente («Omit a property if it is empty»): con `[]` debe dar
     lo de la omisión más un `STRUCTURE_ARRAY_EMPTY`. Excepción (A2): si con la omisión el objeto queda `{}`, la
     omisión da `STRUCTURE_EMPTY_OBJECT` en el objeto y `[]` no (paridad f07/f08 contra f03/e07).
   - Se compara por (ID, severidad, expresión), sin `Location`: agregar una propiedad corre las líneas y columnas
     (A3). Una aserción aparte: el issue nuevo tiene ubicación, también bajo `_x`.
   - El generador resuelve el tipo real del objeto (BackboneElement, destino de una contentReference, valor de una
     elección, el `Element` de un `_x`) y no muta objetos que ya son `{}` (A10).
   - **(c) control:** las instancias sin mutar no cambian respecto de `main`.
3. **Entre librerías:** grupos nuevos de hl7diff (`empty-array-probes`, `-r4b`, `-r5`) con las sondas, más una
   muestra de 30 mutaciones (b) por versión generadas por el oráculo y escritas como archivos de prueba.
4. **Mutaciones de los oráculos** (en una copia):
   - el chequeo apagado;
   - solo complejos (sin D2);
   - reportar también cuando no se repite (rompe D3);
   - sin registrar el holder (rompe D4);
   - path con índice `[0]`;
   - reportar sin `edits` (duplicado, rompe D5);
   - la familia de hl7diff con el ID de HL7 mal escrito;
   - el ID nuevo fuera del `switch` de `Normalize` (`normalize.go:104`), que lo descartaría (A11);
   - el chequeo detrás de la compuerta `primitiveKind` (D8).

### Excepciones a la regla 5 y dependencias

Ninguna: no se nombra ningún elemento ni tipo; «se repite» sale de `ElementDefinition.max` (`Repeats()`). Sin
dependencias nuevas.

## 3. Ronda adversarial (Opus, 02-11-2026)

Revisor independiente sobre una copia sin `.git`, sin mi razonamiento. 11 hallazgos; verifiqué con sondas
(tercera tanda) o leyendo el código los que se podían medir.

| # | Sev. | Hallazgo | Resolución |
|---|---|---|---|
| A1 | alta | R5 tolera un `_x` más corto o de puros nulls («SHALL infer null values», solo en R5); D2 para `_x` en R5 no tiene base, y un cambio por versión sale de prosa, no de un `StructureDefinition` (ADR) | **Aceptado.** Verificado (`grep`). Cita completa en §1; D2 queda abierta para el paso 4; el issue para HL7 espera esa decisión |
| A2 | alta | El oráculo (b) es falso cuando el array vaciado era el único contenido: la omisión da `STRUCTURE_EMPTY_OBJECT` | **Aceptado.** Verificado (f07/f08). Excepción en (b) |
| A3 | media | Los issues llevan línea y columna; agregar una propiedad las corre | **Aceptado.** Comparación sin `Location` |
| A4 | media | e27 no se empareja en hl7diff (índice en `given`) | **Aceptado.** Divergencia en los dos lados (D6) |
| A5 | media | e04 da dos issues en el mismo path | **Aceptado.** D7 |
| A6 | media | No se sondeó una invariante sobre el objeto que solo tiene `[]` | **Aceptado.** Sondas f01–f03: misma conducta en HL7 y gofhir; f03 además mostraba, con la 6.10.2, que un array vacío de primitivo contaba como contenido. Con la 7.0.4 no cuenta: D4 revisado (§10) |
| A7 | baja | El chequeo puede quedar detrás de la compuerta `primitiveKind` | **Aceptado.** D8 y una mutación |
| A8 | baja | `name: {extension: []}` sin sondear | **Aceptado.** Sondas f04/f05 a la tabla |
| A9 | baja | Evidencia de HL7 escasa; ¿7.x ya lo corrige? | **Aceptado.** Extractos de bytecode guardados; `master` tampoco lo revisa (§1) |
| A10 | baja | El generador metamórfico está poco especificado | **Aceptado.** §Tests 2 |
| A11 | baja | El ID nuevo debe entrar al `switch` de `Normalize` | **Aceptado.** Mutación |

Confirmado correcto por el revisor: las citas de R4/R4B/R5, que `holders` solo llega a ele-1 (sin efecto en slicing,
patrones, `Written` ni `apply`), que `contained` y `Bundle.entry.resource` se recorren dentro de `Normalize`, D3,
D5 y la regla 5 (`Repeats()` usa `Base.Max`).

## 4. Aprobación (02-11-2026)

El usuario aprobó el plan ajustado, con D2 «según cada versión» (ADR-003).

## 5-6. Tests e implementación (Sonnet, 02-11-2026)

- Tests primero: `pkg/validator/empty_array_test.go` sobre las sondas `testdata/m12-slice-scoping/probes/ea_*.json`
  (37), en 4.0.1, 4.3.0 y 5.0.0, con los valores de HL7 y de ADR-003. En rojo antes del cambio
  ([tests-before.txt](evidence/2026-11-02-empty-array/tests-before.txt)).
- Producción:
  - `pkg/structural/shape.go`: `checkEmptyArray`, antes de la compuerta `primitiveKind` (D8);
    `emptyPrimitiveArrayReleases` y `emptyPrimitiveArraysForbidden` (ADR-003); el recorrido de items extraído a
    `checkItems` (gocognit).
  - `normalize.go`: el ID en el `switch`.
  - `pkg/issue`: `STRUCTURE_ARRAY_EMPTY`.
  - `Registry.FHIRVersion()`.
- hl7diff:
  - familia `empty-array` con `hl7NoID`: HL7 no trae ID de mensaje para este error;
  - grupos `empty-array-probes`, `-r4b` y `-r5`;
  - 8 divergencias declaradas (ADR-003 y e27, D6);
  - test `TestEmptyArrayPairsWithHL7ParserError`.
- En `corpus.json`, los globs `probes/e*_*.json` pasan a `probes/e[0-9]*_*.json`, para que las `ea_*` no caigan en
  `core-probes` y `r5-probes`. Los grupos no pierden ningún archivo.

## 7. Validación con datos variados

- **Metamórfico** (`pkg/validator/empty_array_metamorphic_test.go`): sobre los ejemplos oficiales, agrega `[]`,
  vacía arrays y vacía `_x`. Cuenta casos e incluye una prueba de que falla con defectos inyectados
  ([metamorphic.txt](evidence/2026-11-02-empty-array/metamorphic.txt)).
- **hl7diff:** [hl7diff-run.txt](evidence/2026-11-02-empty-array/hl7diff-run.txt) y
  [hl7diff-same.txt](evidence/2026-11-02-empty-array/hl7diff-same.txt). En `same`, fuera de los grupos nuevos, la
  única diferencia es un issue correcto en `bw_contained_profile.json` (`Patient.link: []`), que HL7 también
  reporta.
- **Incidente:** la primera corrida de hl7diff (`run` y `same`) compiló el árbol mientras el subagente tenía una
  mutación temporal en `shape.go`, y salieron paths con `[0]`. Se repitió sobre el árbol limpio, y los subagentes
  pasaron a inyectar defectos solo en copias.
- **Mutaciones** ([mutations.py](evidence/2026-11-02-empty-array/mutations.py)):
  - [mutations-1.txt](evidence/2026-11-02-empty-array/mutations-1.txt): 11/13; sobrevivían D5 (m8) y D8 (m10).
  - Con los tests de la ronda 1 de review:
    - [mutations-2.txt](evidence/2026-11-02-empty-array/mutations-2.txt) (m2 y m3 sin aplicar por un texto viejo);
    - [mutations-3.txt](evidence/2026-11-02-empty-array/mutations-3.txt): **15/15**.
- **Bench:** [2026-11-03-0638-bench-compare-68589e6e98f9/](evidence/2026-11-03-0638-bench-compare-68589e6e98f9/).
  Asignaciones iguales a la base. Los tiempos no cuentan: la máquina estaba ocupada.

## 8. Code review (Opus independiente; excepción mientras Codex no tiene cuota)

| Ronda | Hallazgos | Resolución |
|---|---|---|
| 1 | F1 media: D5 y D8 sin test que las mate. F2 baja: la regla por versión sin tests, y DSTU2 también dice «never empty». F3 baja: el metamórfico casi no vacía `_x`. F4 y F6 informativos. F5 baja: idioma de los comentarios | Todos los accionables aceptados: tests en `pkg/structural/empty_array_test.go`; `"1.0"` en la lista, con test de tabla; mínimo de casos `_x` (12/12/17); comentarios en español |
| 2 | G1 baja: los ballots no están en la lista. G2 baja: margen del mínimo `_x` | G1: verificado que los ballots tienen la frase; documentado en ADR-003 (gofhir no trae sus cores). G2: sin cambio. **Converge** |

## 9. Cierre (03-11-2026)

- `make check` en verde ([make-check-final.txt](evidence/2026-11-02-empty-array/make-check-final.txt)).
- hl7diff `run`: 58 PASS, 0 FAIL. `same`: solo los grupos nuevos y la línea correcta de `bw_contained_profile.json`.
- Mutaciones 15/15. Asignaciones iguales a la base.
- ADR-003 aceptado.

## 10. Reapertura (03-11-2026): la referencia HL7 era la 6.10.2; la última es la 7.0.4

El usuario preguntó por la última versión. La referencia de hl7diff y de las sondas era validator_cli 6.10.2;
la 7.0.0 salió el 06-10-2026 y la 7.0.4 el 02-11-2026. `~/tools/validator_cli.jar` pasa a ser un enlace a la
7.0.4; la 6.10.2 queda como `~/tools/validator_cli-6.10.2.jar`.

- hl7diff `run` con la 7.0.4 ([hl7diff-run-7.0.4.txt](evidence/2026-11-02-empty-array/hl7diff-run-7.0.4.txt)):
  - 55 grupos PASS;
  - fallan solo `empty-array-probes*`, y todo por la misma causa: la 7.0.4 reporta ele-1 en el objeto que contiene
    el array vacío (e07, e18, e25, e35, f01, f02, f03), en las tres versiones. Pasa también con un primitivo (f03,
    `contact[0].name: {given: []}`). En R4B, la 7.0.4 lo reporta dos veces, una por cada texto de ele-1.
- #190 tenía razón con la 7.0.3 (ele-1 en `type`). El comentario de corrección queda descartado.
- **D4 revisado:** un array vacío no es contenido. El objeto que lo contiene se juzga como si la propiedad no
  estuviera: `checkEmptyArray` no lo registra en `edits.holders`, y ele-1 se evalúa como siempre. D5 (reportar solo
  en `Normalize`) no cambia.
- En R4B, la 7.0.4 da dos ele-1 en un tipo de datos (uno por cada texto de ele-1) y uno en un BackboneElement
  ([probe/hl7-7.0.4/out-r4b-holders.txt](evidence/2026-11-02-empty-array/probe/hl7-7.0.4/out-r4b-holders.txt),
  `ValueSet.compose` con `include: []`). Las sondas de Go fijan el número exacto en los tipos de datos; el
  metamórfico acepta 1 o 2 en R4B.
- Verificación del D4 revisado:
  - tests dirigidos ([tests-7.0.4.txt](evidence/2026-11-02-empty-array/tests-7.0.4.txt));
  - hl7diff de los tres grupos PASS ([hl7diff-empty-array-7.0.4.txt](evidence/2026-11-02-empty-array/hl7diff-empty-array-7.0.4.txt));
  - `same` sobre todo el manifiesto ([hl7diff-same-d4.txt](evidence/2026-11-02-empty-array/hl7diff-same-d4.txt)):
    los grupos pesados son idénticos, y fuera de los grupos nuevos solo cambia `Patient.link` en
    `bw_contained_profile.json`;
  - mutaciones 15/15 ([mutations-4.txt](evidence/2026-11-02-empty-array/mutations-4.txt));
  - `make check` verde ([make-check-7.0.4.txt](evidence/2026-11-02-empty-array/make-check-7.0.4.txt)).
- Ronda de revisión (Opus) del D4 revisado: sin hallazgos de severidad media o mayor; converge. Hallazgos bajos:
  - evidencia de R4B en un BackboneElement: agregada;
  - la fila A6 y el texto de f03: anotados;
  - el `same` vacío: el archivo se copió mientras se generaba; en el worktree tiene 367 líneas;
  - las sondas de Go no son exhaustivas: es anterior a este cambio, y lo cubre hl7diff.
- El fix de HL7 sigue vigente: la 7.0.4 tampoco reporta `given: []` ni `_given: []`. En 7.x, el validador usa
  `org.hl7.fhir.services.elementmodel.JsonParser`. Se preparó un PR con el fix en las dos copias y casos en
  fhir-test-cases (`/Volumes/dev/hl7/logs/pr-empty-array-*.md`), con una revisión independiente (Opus) cuyos
  hallazgos se aplicaron: copia de r5, un issue por propiedad, dos casos más y la asimetría con R5 explicada.

## Pendientes

- ~~Comentario en #190~~: descartado, era erróneo (§10).
- Borrador de issue para `hapifhir/org.hl7.fhir.core`: `parseChildPrimitive` no reporta un array vacío.
- Borrador de issue: paths de la fase estructural bajo `_x` (`Patient._birthDate.foo`).
