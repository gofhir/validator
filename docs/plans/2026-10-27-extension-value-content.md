# #178: contenido de los valores de una extensión

Issue: https://github.com/gofhir/validator/issues/178 (encontrado en #174). Rama `fix/extension-value-content`
desde `6c0db79`.

## 1. Revisión previa (27-10-2026)

### Especificación

- extensibility.html (R4 §2.17, igual en R4B y R5): toda extensión se valida contra su definición, y un
  elemento puede tener extensiones donde sea. El valor de una extensión es un elemento, así que las
  extensiones dentro de él también se validan.
- json.html#choice: la clave de un elemento de elección es el nombre del elemento seguido del código del
  tipo con la primera letra en mayúscula (`valueIdentifier` → `Identifier`). El tipo es el código, no
  la clave pasada a minúsculas.
- json.html#primitive: las extensiones de un valor primitivo van en su hermano `_clave`.

### Lo que hace el código (`pkg/extension/extension.go`, `6c0db79`)

- `extractValueType` pasa a minúscula la primera letra del tipo (`valueIdentifier` → `identifier`).
  `validateValueContent` busca `GetByType("identifier")`, no encuentra nada y vuelve. Todo el subárbol
  que valida el contenido de un valor complejo queda inalcanzable: `validateValueContent`,
  `validateValueStructure`, `buildValidElementSets`, `isSkippableKey`, `isValidChoiceType`,
  `validateNestedElements`, `findElementType`, `validateValueElement`. En #174, las mutaciones m6, m7,
  m8 y m12 sobrevivieron por eso.
- `isTypeAllowed` compara en minúsculas, lo que esconde el defecto. Además acepta una clave mal escrita
  (`valueuri`) que json.html no reconoce.
- Las claves del valor se reconocen por el prefijo `value` y la excepción `valueSet`, ambos hardcodeados
  (`hasValue`, `valueKeys`), y el elemento por su ruta `Extension.value[x]` (`findValueDefinition`).
  Esto infringe la regla 5.
- Una subextensión declarada en una extensión compleja (`validateNestedExtensionValue`) solo revisa el
  tipo de su valor, nunca el contenido.
- La misma fase ya tiene un recorrido guiado por las definiciones para los elementos del recurso:
  `validateNode`, con `Registry.ChildrenOf` y `internal/elementvalues`. `elementvalues.Of` resuelve cada
  propiedad de un elemento de elección desde sus tipos, con el código exacto, y también la de un tipo
  que el elemento no permite (`Registry.ChoiceType`). No toma `valueSet` como valor.

### Medición contra HL7 6.10.2

Sondas: [probe/](evidence/2026-10-27-extension-value-content/probe/), con el comando en su README. Son 15
instancias de Patient con cuatro extensiones de prueba: `id-ext` (Identifier), `flag-ext` (boolean, con
contexto `Element`), `str-ext` (string) y `timing-ext` (Timing). La `nested-ext` viene de #174. Se
corrieron en R4 y R5 contra `main`. En las dos versiones las diferencias de extensiones son las mismas;
R5 solo agrega `ident-1`, que ambos informan ([cmp-r4.txt](evidence/2026-10-27-extension-value-content/probe/cmp-r4.txt),
[cmp-r5.txt](evidence/2026-10-27-extension-value-content/probe/cmp-r5.txt)).

| Caso | HL7 | gofhir |
| --- | --- | --- |
| c01 `flag-ext` con `valueString` en `maritalStatus` (control) | error de tipo | error de tipo |
| c02 la misma, dentro de `valueIdentifier.type` | error de tipo | **nada** |
| c03 URL desconocida dentro de `valueIdentifier.type` | error | **nada** |
| c04 la misma, en `valueIdentifier.assigner.identifier.type.coding[0]` | error de tipo | **nada** |
| c05 en el `valueHumanName` de una subextensión declarada | error de tipo | **nada** |
| c06 `modifierExtension` en un CodeableConcept del valor | propiedad no reconocida | elemento desconocido (igual) |
| c07 `valueHumanName` donde va Identifier, con una extensión adentro | error de tipo `HumanName` y error de la extensión de adentro | error de tipo `humanName`, **nada adentro** |
| c08 elemento desconocido `foo` en el valor | 1 error | 1 error (de la fase estructural) |
| c09 en `_system` dentro del valor | error de tipo | **nada** |
| c10 sobre el valor mismo (`valueIdentifier.extension`) | error de tipo | **nada** |
| c11 extensión válida dentro del valor (control negativo) | nada | nada |
| c12 en `_valueString` de una extensión primitiva | error de tipo | **nada** |
| c13 en `valueTiming.repeat.boundsPeriod` (elección dentro del valor) | error de tipo | **nada** |
| c14 fecha inválida en `valueIdentifier.period.start` | 1 error | 1 error (de la fase de primitivos) |
| c15 `valueBoolean: "yes"` (corrida aparte, `hl7-c15.json`) | 2 errores distintos (parseo y valor booleano) | **el mismo error 2 veces** |

Hay dos defectos en este código, y los dos son de la fase de extensiones, no de otra librería:

1. Nunca se valida el contenido de un valor (c02 a c13). El texto del error de tipo además dice
   `humanName` (c07).
2. El formato de un valor primitivo se informa dos veces (c15): una por `validatePrimitiveExtensionValue`
   y otra por la fase de primitivos, que también recorre los valores de las extensiones (c14 lo
   muestra). Se confirma con una mutación en el paso 5.

### `gofhir/server` (congelado)

No tiene una pieza propia: valida con `github.com/gofhir/validator/v2` v2.1.0. **Se descarta**, porque
no hay nada que tomar.

### Decisión por pieza

- **Se toma** `validateNode`: el valor de una extensión se recorre como cualquier elemento, con las
  definiciones.
- **Se mejora** la verificación del tipo: el código sale de `elementvalues.Of` y se compara exacto.
- **Se descarta** el subárbol inalcanzable y los prefijos hardcodeados.

## 2. Alcance (ajustado después de la ronda adversarial, §7)

1. **Un valor de extensión se recorre como cualquier elemento.** El nodo de la extensión es la raíz de
   su definición, o la slice de una subextensión declarada en el árbol de la definición del padre. Su
   valor es el hijo que es un elemento de elección: su nombre termina en `[x]` (elementdefinition.html#id,
   igual que `elementvalues`). Un test exige que la Extension base de R4, R4B y R5 tenga exactamente uno
   (C8). No se nombra `Extension.value[x]` ni el prefijo `value`.
2. **Los valores salen de `elementvalues.Of(nodo valor, ext, registry)`.** «Tiene valor» quiere decir que
   hay un valor escrito como su elemento dice (`elementvalues.Written`); un null o un array no cuentan.
   HL7, en p06 y p08, informa «must contain a value». Un primitivo presente solo por su `_clave` sí cuenta:
   HL7 no informa nada en p07 (C5, F7). Con eso se deciden `EXTENSION_VALUE_REQUIRED` y
   `EXTENSION_VALUE_NOT_ALLOWED`; se mantienen las dos, porque HL7 también las informa junto con la
   cardinalidad (p02, p03; F3 rechazado). Se borran `hasValue`, `valueKeys`, `extractValueType` y
   `isTypeAllowed`.
3. **El tipo se compara exacto** contra los códigos de `type` del elemento. `EXTENSION_INVALID_VALUE_TYPE`
   informa el código tal cual (`HumanName`, como HL7). Un primitivo sigue igual (`string`).
4. **`elementvalues` reconoce el `_clave` de un primitivo de un tipo no permitido** (`_valueString` donde
   va boolean), con `ChoiceType`, como ya reconoce la clave sin guion bajo. HL7, en q1, informa el tipo y
   las extensiones de adentro (C4). Lo usan también otras fases (cardinalidad, binding): el cambio se
   mide con `hl7diff`.
5. **El contenido de cada valor se recorre con `validateNode`:**
   - Con los hijos de `ChildrenOf(definición, nodo valor, código)`, que respetan las restricciones de la
     extensión sobre su valor.
   - Un valor de un tipo no permitido se recorre con la definición de su tipo, no con los hijos de la
     extensión, que son de otro tipo (C1). HL7 informa las extensiones de adentro (c07).
   - Si el perfil del tipo no se resuelve (`Unresolved`), se recorre con la definición del tipo, como
     hace `reference.go` con `TypeChildrenOf`. HL7 informa «Unknown profile» y sigue (q2; C3, F5).
     Aplica igual a los elementos del recurso, porque `validateNode` es uno solo.

   Eso cubre:
   - las extensiones sobre el valor (c10) y a cualquier profundidad (c02, c04);
   - los elementos de elección dentro del valor (c13);
   - las extensiones de un primitivo, en su `_clave` (c09, c12, p10).
6. **Las extensiones de un elemento se leen solo si su definición las tiene como hijos.** Un
   `modifierExtension` en un CodeableConcept es un elemento desconocido, que ya informa la fase
   estructural, y no se valida como extensión. Hoy sale dos veces también en el recurso (p01); HL7 lo
   informa una vez (C6, F1).
7. **Subextensión declarada:** igual que 2 a 6, con su slice. Eso incluye sus propias subextensiones,
   que existen en el corpus: `codesystem-history`, `dosage-conditions`, `medication-manufacturingBatch`
   (F6). Se reemplazan `findNestedExtensionDef` y `validateNestedExtensionValue`; se mantiene lo que
   informan hoy.
8. **Se borra `validatePrimitiveExtensionValue`** si una mutación del paso 5 confirma que el error de c15
   ya lo informa la fase de primitivos.
9. **Se borra el subárbol inalcanzable** (§1). Los elementos desconocidos de un valor siguen siendo de la
   fase estructural (c08: una sola vez).
10. **La clave mal escrita de un elemento de elección queda para #180** (decisión del
    usuario, 27-10-2026; borrador en [issue-choice-key-case.md](evidence/2026-10-27-extension-value-content/issue-choice-key-case.md)).
    La fase estructural la acepta (`findMatchingChoiceType`, `structural.go:389`, con `EqualFold`), y
    HL7 la informa como «Unrecognized property» (p09, p13). Al comparar exacto aquí, `valueuri` deja de
    ser un valor:
    - En una extensión que exige valor, sigue informándose: «requires a value» y la cardinalidad, como
      HL7 en p09. Solo falta el «Unrecognized».
    - En una extensión de valor opcional, queda en silencio hasta ese issue. Es una pérdida conocida, con
      un test que la documenta (§5.5).

**Fuera de alcance** (al backlog del README, con su evidencia):

- Los mensajes que HL7 agrega y nosotros no: «Boolean values must be…» junto al error de tipo (c01) y el
  de parseo (c15). Es otra fase y otro criterio.
- Que HL7 valide los elementos de un valor de tipo equivocado contra el tipo declarado (c07: «family not
  allowed by Identifier»). Es una diferencia deliberada: json.html#choice dice que la clave nombra el
  tipo, y los elementos desconocidos son de la fase estructural.
- La severidad de una extensión desconocida: HL7 informa error (c03) y nosotros advertencia. Es una
  política previa, igual en el recurso que dentro de un valor.
- Dos variantes de `value[x]` (p12): HL7 toma la primera y desconoce la otra; nosotros informamos la
  exclusión mutua, la cardinalidad y el tipo. Es el backlog de #174 (deduplicar como HL7).
- Una extensión simple con subextensiones y sin valor (p11): HL7 dice «must contain a value, not
  extensions»; nosotros, solo la cardinalidad (la exención `hasNested` sigue igual).
- El contexto `Element` en la raíz de un recurso: HL7 lo acepta (p02, p06, p07 y p12 con `flag-ext` en
  Patient), nosotros informamos error de contexto. Se mide contra la especificación en otro incremento.

## 3. Excepciones a la regla 5

Ninguna nueva. Este incremento **quita** tres hardcodeos: `value`, `valueSet` y `Extension.value[x]`.
Siguen los que ya estaban, que ahora se registran (C8):

- Las claves `extension` y `modifierExtension`, que `validateNode` salta porque las valida aparte. Con el
  punto 6, solo se leen cuando la definición las tiene como hijos.
- La clave `url` de una extensión (`keyURL`).

Las dos son elementos de Element y de Extension en las definiciones; derivarlas queda en el backlog.

## 4. Criterio de salida

1. Los tests de §5 en verde en R4, R4B y R5.
2. Sondas c01 a c15, p01 a p13, q1 y q2: en cada una de las clases que este plan toca (2 a 9),
   gofhir informa lo mismo que HL7. Eso es la misma ubicación, en nuestra sintaxis de rutas, y la misma
   clase de issue, contados por fase: cada issue sale una vez. Lo que queda distinto es solo lo que §2
   deja fuera.
3. `hl7diff same -baseline 6c0db79 -heavy`: cada línea distinta se lista y se explica en la evidencia.
   Hay tres clases esperadas: issues nuevos de extensiones dentro de valores; el texto de
   `EXTENSION_INVALID_VALUE_TYPE` con un tipo complejo; y los `modifierExtension` duplicados que
   desaparecen.
4. `hl7diff run -jar ~/tools/validator_cli.jar -baseline 6c0db79`, en todos los grupos: cada issue nuevo
   o que desaparece coincide con lo que HL7 informa en el mismo recurso y la misma ubicación, o se
   clasifica con evidencia (bug propio, de HL7 o deliberado).
5. Mutaciones (cada una hace fallar al menos un test):
   - quitar el recorrido del contenido;
   - comparar el tipo sin distinguir mayúsculas;
   - recorrer un tipo no permitido con los hijos de la extensión;
   - no caer a la definición del tipo con un perfil no resuelto;
   - quitar el `_clave` de un primitivo, en `elementvalues` y en el recorrido;
   - saltar la subextensión declarada y sus subextensiones;
   - leer `modifierExtension` sin mirar la definición;
   - contar un null como valor.
6. Costo, con `make bench-compare BASE=6c0db79`:
   - sin regresión significativa (benchstat, p < 0,05) de asignaciones ni de tiempo en los benchmarks
     que hay hoy;
   - un benchmark nuevo, un recurso con extensiones de valores complejos (anchas y profundas), que mide
     el costo del recorrido nuevo; se informa su cifra.

   El contexto de uso ya recorre los valores de las extensiones (`context.go:145`, F4). Si el recorrido
   nuevo sube más de un 10 % el tiempo de ese benchmark, se unifican los dos recorridos antes de cerrar.

## 5. Tests (primero, en rojo con el código actual)

En `pkg/validator`, por las tres versiones. El valor esperado sale de HL7 6.10.2 (§1, §7) o de la
especificación citada:

1. Las sondas c02 a c13 y p10 como tabla. Cada caso exige el issue de la extensión de adentro, con su
   ruta; c11 exige ninguno (control negativo). Hay un caso más profundo que c04: una extensión en el
   valor de una extensión, dentro del valor de otra.
2. c07: el texto del error de tipo nombra `HumanName`, y la extensión de adentro se informa.
3. Unicidad por fase: c06, c08, c14, c15 y p01 dan un solo issue en cada ubicación.
4. p06 y p08 (null y array): «requires a value», como HL7. p07 (`_valueBoolean` solo): sin error.
5. p09: «requires a value» y la cardinalidad, como HL7. Una clave `valueuri` en una extensión de valor
   opcional no se informa: el test documenta la pérdida conocida y remite a #180 (§2.10).
6. q1: el tipo `string` no permitido y la extensión de adentro. q2: el perfil no resuelto y la extensión
   de adentro.
7. Una sub-subextensión mala dentro de una subextensión declarada.
8. Por versión, desde el package cargado (C7): un valor de un tipo que la Extension de esa versión
   permite y otro que no. Los tipos con cambios son `CodeableReference` (R4B y R5), `integer64` (R5) y
   `Meta` (R4).
9. Sin regresión en lo que hoy se informa bien: p02 y p03, una extensión sin el valor que exige, y una
   clave `valueSet` en una extensión (no es un valor: `Set` no es un tipo, así que no se toma como tal).
10. La Extension base de cada versión tiene un solo hijo de elección (C8).

Datos variados (paso 7): el corpus de `hl7diff` (US Core, mCODE, IPS, DEQM, AU, CH y CL Core, y los
ejemplos de R4 core), que tiene extensiones con valores complejos, y el barrido de #174 sobre los
ejemplos de R4, R4B y R5.

## 6. Dependencias

Ninguna nueva.

## 7. Ronda adversarial (27-10-2026)

Fable (F) y Codex (C), en paralelo, cada uno en su copia, sin mi razonamiento. Los dos corrieron sondas
en Go; HL7 lo corrí yo, una vez, sobre sus sondas y dos de Codex
([adversarial/](evidence/2026-10-27-extension-value-content/adversarial/)).

| Id | Sev. | Hallazgo | Decisión |
| --- | --- | --- | --- |
| C6, F1 | alta | Un `modifierExtension` en un tipo que no lo tiene se informa dos veces, y el recorrido nuevo lo llevaría a c06 | **Aceptado** (§2.6). Medido: hoy pasa también en el recurso (p01) |
| F2 | alta | La fase estructural acepta `valueuri` (`EqualFold`); al comparar exacto aquí, nadie lo informa | **Aceptado como issue aparte** (decisión del usuario; §2.10). HL7: «Unrecognized property» (p09, p13) |
| C1 | alta | Con un tipo no permitido, `ChildrenOf` da los hijos del tipo permitido | **Aceptado** (§2.5) |
| C3, F5 | alta, media | Con un perfil de tipo no resuelto no hay hijos y el valor no se recorre | **Aceptado** (§2.5). HL7 sigue con el tipo base (q2) |
| C4 | alta | `elementvalues` no reconoce `_valueString` de un tipo no permitido | **Aceptado** (§2.4). HL7 lo informa (q1) |
| C2, F8 | alta, baja | El recorrido no entra a las slices (de `value[x]` o de otro elemento) | **Rechazado:** una slice restringe su elemento, no agrega hijos (profiling.html). Esta fase solo busca propiedades de extensión y claves de elección, que son las del elemento base; lo que una slice restringe lo validan las fases de slicing y binding. Se registra en §2 que la restricción de una slice no se usa aquí |
| C5, F7 | media, baja | Null, array y `_clave` sin valor: ¿cuentan como valor? | **Aceptado** (§2.2), con lo que HL7 hace en p06, p07 y p08 |
| C7 | media | Las sondas usan los mismos tipos en todas las versiones | **Aceptado** (§5.8) |
| C8 | media | El criterio `[x]` y los nombres que siguen hardcodeados | **Aceptado:** test de unicidad (§5.10) y registro (§3) |
| C9 | media | Faltan profundidad, multiplicidad por fase y umbral de costo | **Aceptado** (§4.2, §4.6, §5.1, §5.3). `contentReference`: `ChildrenOf` ya la resuelve; en el paso 5 se mide qué tipos de datos la usan y, si hay alguno, se agrega su caso |
| F3 | media | `EXTENSION_VALUE_REQUIRED`/`NOT_ALLOWED` duplican la cardinalidad | **Rechazado:** HL7 informa las dos cosas (p02: «must contain a value» y «minimum required»; p03: el tipo y «max allowed») |
| F4 | media | El contexto de uso ya recorre los valores: dos recorridos | **Aceptado como medición** (§4.6): se unifican si el costo pasa el umbral. Son dos responsabilidades distintas |
| F6 | media | Las sub-subextensiones existen en el corpus | **Aceptado** (§2.7) |

Medido además: HL7 no informa el contexto `Element` en la raíz de un recurso, y nosotros sí (al backlog,
§2).

## 8. Decisión del usuario (27-10-2026)

«separemos en otro issue»: la clave de un elemento de elección escrita con otras mayúsculas
(`findMatchingChoiceType`) va en un issue propio, publicado como #180. El resto del plan queda como está. Aprobado: «si».

## 9. Registro de los pasos 5 a 7 (27-10-2026)

### Paso 5: tests primero

- `pkg/validator/extension_value_test.go`: `TestExtensionValueContent`, con 36 casos (algunos solo en ciertas versiones) por las tres
  versiones, y `TestExtensionHasOneChoiceElement` (§5.10). Las definiciones están en
  `pkg/validator/testdata/extension-value/`.
- Los valores esperados salen de HL7 6.10.2 sobre las mismas entradas. El flag
  `-ext-value-dump=<dir>` las escribe ([tests-hl7/](evidence/2026-10-27-extension-value-content/tests-hl7/),
  con su comando).
- Contra el código de `6c0db79` el test sale en rojo ([red-before.txt](evidence/2026-10-27-extension-value-content/red-before.txt)).

Correcciones que pidió HL7 antes de cerrar los tests:

- **Meta está en R4 y R5, no solo en R4.** Solo R4B lo quitó de `Extension.value[x]`; el supuesto del
  plan (§5.8) estaba mal. Medido en los packages y en HL7 ([meta-r5.json](evidence/2026-10-27-extension-value-content/tests-hl7/meta-r5.json)).
- **`valueuri` en una extensión de valor opcional no queda en silencio.** Falla `ext-1` (HL7 informa
  lo mismo), así que la pérdida conocida de #180 se reduce al «Unrecognized property». §2.10 decía
  «en silencio».
- **p11 (sin valor y con subextensiones) lo informa la cardinalidad mínima de `value[x]`**, no `ext-1`,
  como HL7. Se mantiene la exención `hasNested` (§2, fuera de alcance) y el caso la fija.

### Paso 6: implementación

- `pkg/extension/extension.go`: lo que planea §2 (puntos 1 a 9).
  - `validateNode` cae a `TypeChildrenOf` con un perfil no resuelto, y lee las extensiones solo donde
    la definición las tiene (`validateDeclaredExtensions`).
  - Un valor se valida con `validateExtension` y `validateExtensionValue`, sobre el árbol de la
    definición, a cualquier profundidad de subextensiones declaradas (`declaredSlice`).
  - Se borraron el subárbol inalcanzable, `validatePrimitiveExtensionValue` y los prefijos hardcodeados.
- `internal/elementvalues`: el `_key` de un primitivo de un tipo no declarado (§2.4).
- **Hallazgo durante la implementación:** el caso «Meta value, R4B» falló. En R4B, Meta es un tipo,
  pero la Extension base no lo permite en `value[x]`. HL7 distingue dos casos:
  - un tipo que la Extension base permite y el perfil restringe: error de tipo;
  - un tipo que la base no permite: «Unrecognized property» y «must contain a value».

  La fase estructural ya informa el segundo como elemento desconocido. La fase de extensiones no lo
  cuenta como valor: toma los tipos del elemento valor en la definición del tipo de la extensión
  (`baseTypes`).
- **c15:** sin `validatePrimitiveExtensionValue`, el error del booleano lo informa una vez la fase de
  primitivos (`TYPE_WRONG_JSON_TYPE`); el caso lo exige.

### Paso 7: validación

- **Unicidad:** ningún caso informa un issue dos veces (lo verifica cada caso del test).
- **`hl7diff same -baseline 6c0db79 -heavy`**
  ([salida](evidence/2026-10-27-extension-value-content/hl7diff-same-6c0db79.txt),
  [diferencia como multiconjunto](evidence/2026-10-27-extension-value-content/hl7diff-same-6c0db79-multiset.txt),
  con [msdiff.py](evidence/2026-10-27-extension-value-content/msdiff.py)). Hay dos clases, las dos
  explicadas:
  - El texto de `EXTENSION_INVALID_VALUE_TYPE` con un tipo complejo (`reference` → `Reference`), en
    `reference-walk-probes`. Es la clase declarada en §4.3.
  - 12 `EXTENSION_VALUE_REQUIRED` que desaparecen en `r4-core-examples`: `codesystem-concept-comments`
    con su valor presente solo por `_valueString`, que trae una extensión `translation` (como p07). HL7
    no los informa. Es un falso positivo que se va, y ahora se valida además la `translation` de
    adentro.

  No aparece ningún issue nuevo: el corpus no tiene extensiones inválidas dentro de un valor.
- **`hl7diff run -baseline 6c0db79`** contra HL7: PASS en los 48 grupos livianos y en
  `r4-core-examples`, sin hallazgos. En `r4-core-examples`, «unexplained errors removed: 12»: HL7
  confirma que los 12 eran falsos positivos
  ([livianos](evidence/2026-10-27-extension-value-content/hl7diff-run-6c0db79.txt),
  [r4-core](evidence/2026-10-27-extension-value-content/hl7diff-run-6c0db79-r4-core.txt),
  [reporte](evidence/2026-10-27-extension-value-content/hl7diff-run-6c0db79-r4-core-report.md)).
- **Barrido de #174** (ejemplos de R4, R4B y R5, tal cual y con propiedades desconocidas, y
  `testdata/`): 0 inestables y ningún error de `Validate`
  ([sweep.txt](evidence/2026-10-27-extension-value-content/sweep.txt)). R4 tiene 12 issues menos (los
  de arriba). `testdata/` tiene un recurso más, el del benchmark nuevo.
- **Mutaciones:** 14/14 muertas ([mutations.txt](evidence/2026-10-27-extension-value-content/mutations.txt),
  [mutations.py](evidence/2026-10-27-extension-value-content/mutations.py)).
  - m11 y m13 (un valor no escrito cuenta, o se recorre) son equivalentes en el pipeline: la fase
    estructural ya saca del objeto lo que no está escrito como su elemento dice. Se matan con un test de
    la fase, que es API pública (`pkg/extension/value_test.go`, `TestUnwrittenValueIsNoValue`), con
    los valores esperados de HL7 ([unwritten/](evidence/2026-10-27-extension-value-content/tests-hl7/unwritten/)).
  - m14 (la exención `hasNested`) la mata el caso p11.
  - La primera corrida dio «killed» en todo porque un test fallaba sin mutar. `run-mutations.sh` ahora
    verifica primero la base y se detiene si falla.
  - Equivalente, sin mutación: comparar el tipo sin distinguir mayúsculas. Ahora el código viene de
    `elementvalues`, siempre exacto.
- **Costo:** caso nuevo `BenchmarkValidateProfiled/extension-values`
  (`testdata/bench/extension-values.json`: 15 extensiones de US Core y del núcleo, con valores
  complejos y extensiones adentro). `scripts/bench-compare.sh` lo exige en las dos salidas.
- **`make bench-compare BASE=6c0db79`** ([reporte](evidence/2026-10-27-1239-bench-compare-6c0db79d9d48/report.md)):
  - Asignaciones: sin regresión significativa salvo `extension-values`, con +0,54 % de bytes y
    +0,05 % de asignaciones. `patient` baja (-0,22 % y -0,55 %).
  - Tiempo: ningún cambio significativo, pero con una varianza de ±50 a ±190 %, porque la máquina tenía
    carga de otras sesiones (promedio de carga 20-30 en 12 núcleos).
  - Para el umbral de §4.6 se midió aparte solo `extension-values`, intercalado, en 20 rondas
    ([bench-extension-values/](evidence/2026-10-27-extension-value-content/bench-extension-values/)):
    +2,6 %, sin significancia (p = 0,242, ±5 a ±11 %). La estimación queda bajo el 10 %, pero con esa
    varianza el intervalo no lo asegura (revisión 1, F3). Con eso el recorrido no se unifica con el de
    los contextos (F4).

## 10. Code review (paso 8)

### Ronda 1 (27-10-2026): Codex y Fable, sobre el diff contra 6c0db79

| Hallazgo | Severidad | Resolución |
| --- | --- | --- |
| Codex C1: `walkTargets` (`context.go`) no recorre con la definición del tipo el valor cuyo perfil no se resuelve; el contexto de una extensión adentro no se revisa | media | **Confirmado** con una sonda (R4, R4B y R5). HL7 informa el error de contexto ([review1-c1/](evidence/2026-10-27-extension-value-content/tests-hl7/review1-c1/)). Corregido con el mismo fallback a `TypeChildrenOf`; caso nuevo «context inside a value whose type profile is not resolved» y mutación m15. |
| Fable F1: las reglas max 0 y min > 0 del valor de una subextensión declarada no tienen test | media | **Confirmado**: las mutaciones de Fable (m16 y m17) sobrevivían. Se agrega `req-part-ext` y dos casos con la salida de HL7 ([review1-f1/](evidence/2026-10-27-extension-value-content/tests-hl7/review1-f1/)), en `Patient.extension[0].extension[0]` como HL7. La fase de cardinalidad informa además `SLICING_CARDINALITY_MAX/MIN` en `.value[x]`; ese doble informe ya existía en 6c0db79 en la raíz y en el min de la subextensión (sonda sobre la base), y queda en el backlog de deduplicación (#174). |
| Fable F2: `baseTypes` llama dos veces a `Tree()`, que toma un mutex | baja | Aceptado: una sola llamada. |
| Fable F3: el umbral del 10 % no está demostrado | baja | Aceptado: §9 lo dice así. |
| Fable F4: `pkg/extension` lee el testdata de `pkg/validator` y enumera sus snapshots a mano | baja | Al backlog: funciona, y moverlo no cambia lo que se valida. |

### Ronda 2 (27-10-2026): Codex y Fable, sobre el diff de las correcciones de la ronda 1

Converge: ninguno deja hallazgos de severidad media o mayor.

- **Codex:** revisó solo por inspección, porque su sandbox no le permitió compilar. La duda que deja (si el
  test cae sin la corrección de C1) la responde m15.
- **Fable:** quitar el fallback de C1 hace caer el caso nuevo en las tres versiones; saltar el valor de
  una subextensión declarada hace caer los dos casos de F1. No hay pérdidas ni issues duplicados.
- **Observación baja, sin acción:** con el perfil sin resolver, el fallback no busca la definición
  propia por `url`. Es lo mismo que hace `validateNode` y no cambia los contextos observables.

Mutaciones antes de la ronda: 17/17 ([mutations.txt](evidence/2026-10-27-extension-value-content/mutations.txt)).
