# #174: issues en un orden determinista

Issue: [gofhir/validator#174](https://github.com/gofhir/validator/issues/174). Rama `fix/issue-order`,
desde `a127801`. Proceso `fhir-incremento` (NUCLEO.md §10).

## 1. Revisión previa (26-10-2026)

### Especificación

`OperationOutcome.issue` (R4, R4B y R5, operationoutcome.html) no exige ningún orden. No es un error de
conformidad: es un defecto de reproducibilidad. Dos corridas sobre la misma entrada no se comparan
línea a línea, y los archivos dorados y `hl7diff` tienen que ordenar primero.

### Medición (sondas en el scratchpad; evidencia en `evidence/2026-10-26-issue-order/`)

Sonda: cada recurso se valida 8 veces y se cuentan los órdenes distintos de `Result.Issues` y los
conjuntos distintos (el mismo multiconjunto de líneas, ordenado).

| Datos (R4 4.0.1) | Recursos | Orden cambia | Conjunto cambia |
| --- | --- | --- | --- |
| `hl7.fhir.r4.examples` 4.0.1, tal cual | 4.658 | 0 | 0 |
| Los mismos, con 3 propiedades desconocidas por objeto hasta la profundidad 2 | 4.658 | 4.657 | 0 |
| Todo `testdata/` | 2.056 | 26 | 0 |

IDs cuyo orden cambia: `STRUCTURE_UNKNOWN_ELEMENT`, `STRUCTURE_VALUE_EXPECTED`,
`STRUCTURE_OBJECT_EXPECTED`, `STRUCTURE_ARRAY_EXPECTED`, `STRUCTURE_EMPTY_OBJECT` (fase estructural) y
`TYPE_INVALID_FORMAT` (fase de primitivos). Costo de la sonda: 203 s y 555–897 MB de RSS por corrida
de 4.658 recursos.

Dos casos que la sonda no cubría, probados aparte (40 validaciones):

- **El texto cambia.** `STRUCTURE_CHOICE_MUTUAL_EXCLUSION` lista las variantes en el orden del mapa:
  «found: valueString, valueInteger, valueBoolean» y sus permutaciones (8 salidas distintas).
- **El conjunto cambia: falso negativo intermitente.** Una extensión `patient-birthPlace` (solo admite
  `Address`) con `valueString` y `valueAddress`: `EXTENSION_INVALID_VALUE_TYPE` sobre `valueString`
  aparece en unas corridas y no en otras. `findValueKey` (`pkg/extension/extension.go:679`) revisa
  solo la primera clave `value*` que entrega el mapa.

### Fuentes en el código

Análisis de tipos (`go/packages`, sonda `maprange`): 50 `range` sobre mapas en `pkg/` sin tests. Los
que emiten issues o deciden qué se valida:

| Sitio | Qué recorre | Efecto |
| --- | --- | --- |
| `structural/structural.go:190` y `:198` | las claves del objeto, para la exclusión mutua | texto de las variantes y orden |
| `structural/structural.go:209` | las claves del objeto | orden de desconocidos y de forma |
| `primitive/primitive.go:186` | las claves del objeto | orden de `TYPE_INVALID_FORMAT` |
| `extension/extension.go:233`, `:549`, `:615` | las claves del valor de una extensión | orden |
| `extension/extension.go:679` (`findValueKey`) | primera clave `value*` | **conjunto** |
| `fixedpattern/fixedpattern.go:132` | las claves del objeto | orden |
| `issue/diagnostics.go:742` (`formatTemplate`) | los parámetros del mensaje | ninguno hoy (ningún valor contiene otro marcador) |

Los otros 42 no emiten: copian, cuentan o arman conjuntos. Tres devuelven la primera coincidencia de un
mapa (`resolveElementDefinition` en `structural`, `primitive` y `fixedpattern`; `matchChoiceType`):
solo cambiarían la elección si, bajo un mismo padre, el nombre de un elemento de elección fuera prefijo
de otro. Medido en los packages core: **0 casos** en R4 (655 SD), R4B (651) y R5 (307). El recorrido
por el árbol (`walker`) y las fases que lo usan no recorren mapas.

### `gofhir/server` (congelado)

`internal/validation/gofhir.go:286-303` copia `Result.Issues` en el orden que entrega el validador; no
ordena. **Descartar:** no hay nada que tomar; hereda el defecto.

### HL7 (6.10.2)

- `ValidatorUtils.removeDuplicates` quita los duplicados (mismo mensaje y ubicación) y **no ordena**
  (el `sort` está comentado). `ValidationMessage.compare` (nivel, tipo, línea, mensaje) existe pero el
  CLI no lo aplica.
- Su orden es determinista porque recorre el modelo de elementos, cuyos hijos conservan el orden del
  documento. En `snapshot-choice-merge/hl7/outcomes.json` los issues salen por línea ascendente, con
  los warnings de la raíz antes que un error de la línea 13.
- **Tomar:** que el orden sea función de la entrada. **No tomar:** su orden concreto. gofhir valida por
  fases y no puede intercalar como HL7 sin rehacer el motor, y quitar duplicados es otro cambio.

## 2. Alcance

**Dentro:**

1. Cada sitio de la tabla recorre las claves del objeto **ordenadas por nombre**. El orden es el mismo
   para la misma entrada y para cualquier reordenamiento de sus claves. Sin asignar por objeto (F1):
   las claves van a un buffer `[]string` reutilizado durante la validación, usado como pila (se agrega
   al entrar en un objeto y se trunca al salir), y un objeto con una sola clave no se ordena.
2. La lista de variantes de la exclusión mutua, ordenada.
3. La extensión con más de un `value[x]`: **cada** clave `value*`, en orden, se trata como hoy se trata la
   única: su tipo se revisa, y, si el tipo está permitido, su contenido (primitivo o complejo) se valida.
   En los dos lugares que hoy toman la primera (`extension.go:464`, extensiones de primer nivel, y
   `:797`, extensiones anidadas; C5, F2). La cardinalidad ya informa que hay más de uno
   (`CARDINALITY_MAX`) y la estructura, la exclusión mutua; lo que se valide no puede depender de cuál
   salió primero.
4. `formatTemplate` reemplaza los marcadores **en una sola pasada sobre la plantilla** (`strings.NewReplacer`):
   un valor que contiene un marcador (un código literal `{valueSet}`) se escribe tal cual y no se vuelve a
   reemplazar (C4). Ordenar los reemplazos sucesivos fijaría una salida incorrecta.

**Fuera (y por qué):**

- **El orden del documento.** El recurso se decodifica a `map[string]any` y ese orden se pierde ahí.
  Conservarlo exige otro decodificador: es un cambio de arquitectura, no de este issue. Con claves
  ordenadas, además, un reordenamiento de la entrada da la misma salida, que el orden del documento no
  da. Queda como frente propio si se quiere acercar el orden al de HL7.
- **Ordenar `Result.Issues` al final** (por línea y columna): descartado. Los issues de la misma
  ubicación siguen en el orden de emisión (la exclusión mutua de dos elementos de elección de un mismo
  objeto), así que no resuelve la causa, y mezcla las fases.
- **Quitar duplicados como HL7.** Otro comportamiento; al backlog.
- **`findValueKey` y `hasValue` reconocen el valor por el prefijo `value`** escrito en el código, no por
  el `value[x]` de la definición de la extensión, y excluyen `valueSet` a mano (`extension.go:671`,
  `:681`), que no es un tipo de `value[x]` en R4, R4B ni R5 (F3) (regla 5). Es previo; se registra como
  frente propio y no se toca aquí, salvo para recorrer todas las claves.
- **Ordenar al final por línea y columna** para acercarse al orden de HL7 (con `Location` ya calculada,
  costaría una asignación): opcional, frente propio.

## 3. Excepciones a la regla 5

Ninguna nueva. El cambio no agrega nombres de la especificación.

## 4. Criterio de salida

1. Las sondas de §1, convertidas en un test del repo con flag (como `TestDumpRegenerated`): **0 recursos
   inestables**, en los ejemplos de R4, R4B y R5 tal cual y con las propiedades desconocidas, y en
   `testdata/`.
2. Los tests de §5 en verde en R4, R4B y R5.
3. `hl7diff same -baseline a127801 -heavy`: los mismos issues (como multiconjunto) que la base, con dos
   diferencias declaradas y verificadas una por una, no excluidas por ID (C2): el texto de
   `STRUCTURE_CHOICE_MUTUAL_EXCLUSION` (variantes en orden de nombre) y los issues de las extensiones con
   más de un `value[x]` (el tipo y el contenido de cada uno). La firma de `hl7diff` incluye el texto, así
   que cada línea distinta se lista y se explica en la evidencia.
4. Mutaciones: quitar el orden en cada sitio, y volver a `findValueKey` con la primera clave, hace
   fallar al menos un test.
5. Costo: `make bench-compare BASE=a127801` sin regresión significativa (benchstat, p < 0,05) de
   asignaciones ni de bytes en los benchmarks de un recurso, y tiempo sin regresión significativa. Con
   `slices.Sorted(maps.Keys(...))` en cada sitio, Fable midió +5,3 a +6,2 % de asignaciones (p = 0,000):
   esa implementación no cumple (F1).

## 5. Tests (primero, en rojo con el código actual)

Valores esperados: el orden por nombre es la decisión de §2; los conjuntos salen de la especificación
o de HL7, no de correr el código.

1. **Desconocidos en orden.** El Patient del issue (`alpha` … `echo`): 30 validaciones, todas con los
   `STRUCTURE_UNKNOWN_ELEMENT` en orden `alpha, bravo, charlie, delta, echo`. R4, R4B y R5.
2. **Metamórfico (documental).** El mismo recurso con sus claves en 5 órdenes distintos (en la raíz y en
   un objeto anidado), con desconocidos, formas incorrectas y un primitivo inválido: la misma lista de
   issues **sin `Location`** (C1: la ubicación sale de los bytes y cambia con el orden), y cada `Location`
   apunta a la propiedad de su issue en esa entrada. Con 30 validaciones de cada orden. R4, R4B y R5.
   Como el decodificador ya borra el orden de la entrada, lo que discrimina es la repetición (F4).
3. **Texto de la exclusión mutua.** Observation con `valueString`, `valueInteger`, `valueBoolean`:
   «found: valueBoolean, valueInteger, valueString» siempre.
4. **Extensión con dos valores** (`patient-birthPlace`, que solo admite `Address`), 30 validaciones de
   cada caso, en `pkg/validator` (`pkg/extension` no tiene tests, F2):
   - `valueString` y `valueAddress`: un `EXTENSION_INVALID_VALUE_TYPE` sobre `valueString`;
   - `valueString` y `valueInteger`: uno sobre cada uno;
   - dos `valueAddress` no caben en JSON; `valueAddress` válido y `_valueString` con una extensión: el
     contenido del `valueAddress` se valida (una `Address` con un elemento desconocido lo informa);
   - una extensión anidada con dos valores (el segundo lugar, `:797`), con el mismo criterio;
   - y la plantilla con un valor que contiene `{valueSet}` (test de `formatTemplate`, C4).

   **Una corrida de HL7** (una sola instancia de Java) sobre estas instancias y la del test 3, para
   registrar qué informa.
5. **Barrido** (`TestIssueOrderSweep`, con flag): la sonda de §1 en el repo, sobre los ejemplos de las
   tres versiones (tal cual y con desconocidos, también dentro de `contained` y de las entradas de un
   Bundle) y `testdata/`. Exige las N validaciones correctas de cada recurso (un error de `Validate`
   falla el barrido, no lo saltea), y escribe cuántos recursos y cuántos issues cubrió (C3).

Los packages `hl7.fhir.r4b.examples` 4.3.0 y `hl7.fhir.r5.examples` 5.0.0 no están en la caché: se
instalan con el loader del repo (`pkg/loader`), con su SHA-256 registrado en la evidencia.

## 6. Dependencias

Ninguna. La sonda `maprange` usa `golang.org/x/tools` fuera del módulo, en el scratchpad.

## 7. Ronda adversarial (26-10-2026)

Fable (F) compiló y midió en su copia; Codex (C) revisó solo leyendo, porque su sandbox no le dejó
crear las cachés de Go. Los hallazgos de Codex que tocan el diseño (C4, C5) los verifiqué leyendo el
código.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| F1 | media | Con `slices.Sorted(maps.Keys(...))` en los 7 sitios, +5,3 a +6,2 % de asignaciones (benchstat, n = 8, p = 0,000) | **Aceptado:** buffer reutilizado como pila, sin ordenar un objeto de una clave; criterio 5 con benchstat |
| C5, F2 | media | El primer `value*` decide también qué contenido se valida (`extension.go:489-502`), en dos lugares (`:464`, `:797`); faltan casos (dos inválidos, contenido del segundo, anidadas) | **Aceptado:** §2.3 y test 4 |
| C1 | media | El metamórfico no puede exigir `Issues` idéntico: `Location` sale de los bytes | **Aceptado:** compara sin `Location` y verifica cada ubicación aparte |
| C2 | media | El criterio de `hl7diff` permitía solo una diferencia, pero el texto de la exclusión mutua también cambia | **Aceptado:** las dos diferencias se declaran y se verifican línea por línea |
| C3 | media | La sonda saltea los errores de `Validate`, no mide cobertura y no muta `contained` | **Aceptado:** test 5 |
| C4 | media | `formatTemplate` reemplaza en sucesión: un valor con `{valueSet}` se reinterpreta | **Aceptado:** una sola pasada (§2.4) y test |
| F3 | baja | `valueSet` excluido a mano en `hasValue` y `findValueKey` | **Aceptado como registro:** al frente de la regla 5 de §2 |
| F4 | baja | El metamórfico es trivial: el decodificador ya borra el orden | **Aceptado:** se marca documental; la repetición es el oráculo |
| F5 | — | Ordenar al final por línea y columna daría el orden de HL7 por una asignación | **Registrado** fuera de alcance (§2) |

Verificado sin hallazgo (Fable): la validación no usa goroutines (los `sync.Map` son cachés por URL);
`dropReported` incluye el texto en su clave, pero la fase estructural corre una vez por recurso; un
Bundle con entradas y `contained` da 7 órdenes y un solo conjunto; `hl7diff` ya ordena
(`identical.go:62`); ningún test depende de la posición de un issue después del primero; con las claves
ordenadas, la sonda sobre `testdata/` da 0 recursos inestables.


## 8. Decisión del usuario

Plan aprobado sin cambios (27-10-2026, «si»).

## 9. Registro de los pasos 5 a 7 (27-10-2026)

### Paso 5: tests primero

`pkg/validator/issue_order_test.go` (órdenes, metamórfico, exclusión mutua, extensiones con varios
valores), `pkg/issue/format_template_test.go` y, después de las mutaciones, `TestFixedValuesInNameOrder`
y `pkg/fixedpattern/issue_order_test.go`. En rojo con el código de `a127801`, por la salida que cambia
entre validaciones ([red-before.txt](evidence/2026-10-26-issue-order/red-before.txt)); en los casos de
extensiones, `EXTENSION_INVALID_VALUE_TYPE` aparece en unas corridas y no en otras.

### Paso 6: implementación

- `internal/objectkeys`: las claves de un objeto en orden de nombre, en un buffer de un `sync.Pool`
  (0 asignaciones por llamada, `TestSortedDoesNotAllocate`). Lo usan los sitios de §1.
- `pkg/extension`: `valueKeys` en lugar de `findValueKey`; `validateValueOf` valida cada valor, en los
  dos lugares.
- `pkg/issue`: `formatTemplate` en una sola pasada.

### Paso 7: validación

- **Barrido** ([sweep.txt](evidence/2026-10-26-issue-order/sweep.txt)): 26.166 validaciones de
  recursos de R4, R4B y R5, tal cual y con propiedades desconocidas, 4 veces cada una: **0
  inestables** en la rama y **13.089** en la base con el mismo test, que confirma el oráculo. Los
  packages de ejemplos de R4B y R5 se instalaron con `loader.Install`
  ([packages-sha256.txt](evidence/2026-10-26-issue-order/packages-sha256.txt)).
- **HL7 6.10.2**, una instancia de Java ([hl7/](evidence/2026-10-26-issue-order/hl7/)). Su parser
  se queda con la primera variante de un elemento de elección en el orden del documento, informa las
  demás como «Unrecognized property» y valida solo esa. gofhir no conserva el orden del documento
  (§2): informa la exclusión mutua y valida cada variante. En los casos con un valor que sobra y al
  menos uno no permitido, los dos dan error sobre el valor que sobra; difieren en el código del
  mensaje y en cuántos informan. **Diferencia deliberada**, json.html#choice no dice qué informar.
  Los elementos desconocidos: HL7 los da en orden del documento y gofhir por nombre (§2, fuera).
- **Mutaciones** ([mutations.txt](evidence/2026-10-26-issue-order/mutations.txt)): **11 de 15
  mueren**. Las primeras corridas mostraron dos huecos, cerrados con tests: dos elementos de elección
  en un objeto (m3) y `fixedpattern.Validator`, que el validador no usa (solo su `Checker`) pero es API
  pública (m5). **Sobreviven m6, m7, m8 y m12: código inalcanzable** por un defecto previo, abajo.
- **Costo** ([2026-10-27-0647-bench-compare-a12780150511](evidence/2026-10-27-0647-bench-compare-a12780150511/report.md),
  `make bench-compare BASE=a127801`): **ninguna regresión significativa** de tiempo, bytes ni
  asignaciones; media geométrica −0,29 % de bytes y −0,15 % de asignaciones. Criterio 5 cumplido.
- `go test ./...` en verde; lint con 0 issues.

### Hallazgo: un defecto previo (no de #174)

`extractValueType` pasa a minúscula la primera letra de todo tipo (`valueHumanName` → `humanName`),
así que `validateValueContent` nunca valida el contenido de un valor de extensión de tipo complejo.
Todo lo que cuelga de ahí (`validateValueStructure`, `validateValueElement`, `validateNestedElements`)
es inalcanzable con tipos complejos: por eso esas mutaciones sobreviven. Desde `Validate` se ve como un
**falso negativo**: una extensión con un tipo no permitido, o desconocida, **dentro** del valor complejo
de otra extensión no se informa (R4 y R5); la misma, en un elemento del recurso, sí. Publicado como
[#178](https://github.com/gofhir/validator/issues/178) (27-10-2026), con la reproducción verificada con el
CLI sobre `a127801`
([issue-extension-value-content.md](evidence/2026-10-26-issue-order/issue-extension-value-content.md)).
**Decisión del usuario:** #174 se cierra con m6, m7, m8 y m12 registradas como código inalcanzable; #178
es un incremento propio. El orden por nombre queda aplicado en ese código, para cuando sea alcanzable.
El test de la fase que lo encontró: [extension_phase_test.go.txt](evidence/2026-10-26-issue-order/extension_phase_test.go.txt).


## 10. Paso 8: code review, ronda 1 (27-10-2026)

Fable (F) compiló y corrió tests, mutaciones y `-race` en su copia; Codex (C) revisó solo leyendo, porque
su sandbox no le dejó crear las cachés de Go. Cada hallazgo se verificó antes de corregirlo.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| C1, F1 | alta | `TestSortedDoesNotAllocate` falla con `-race`, que el CI usa (`ci.yml:73`): `sync.Pool` descarta al azar parte de lo que recibe. Verificado: 17 de 20 corridas fallan | **Corregido:** el test pasa a `alloc_test.go` con `//go:build !race`; con `-race`, 20 de 20 en verde |
| F2 | media | Sin test para un marcador sin parámetro y una llave suelta en `formatTemplate`: dos mutaciones sobrevivían | **Corregido:** `TestFormatTemplateLeavesTheRestAsIs` (la salida es la del reemplazo sucesivo, así que ninguna plantilla cambia); mutaciones m16 y m17 |
| C2 | baja | `checkLocations` saltaba los issues sin ubicación | **Corregido:** cada elemento desconocido exige su ubicación |
| C3 | baja | Un comentario modificado quedó en inglés (`extension.go:555`) | **Corregido** |
| F3 | baja | `prealloc` en el test de `fixedpattern`, agregado después de la pasada del lint | **Corregido** |
| F4 | baja | Quitar la limpieza del buffer en `Release` no lo detecta ningún test | **Registrado;** en la ronda 2 se agregó el test (abajo) |

Verificado sin hallazgo (Fable): cada `Sorted` anidado toma otro buffer y ningún `Keys` se usa después
de `Release`; `validateValueOf` es el cuerpo anterior, así que con un solo valor nada cambia; no quedan
recorridos de mapas que emitan fuera de los de §1.

Después: mutaciones 13/17 (las 4 que sobreviven, de #178), `make check` en verde (incluye `-race
-short`, como el CI; 139 s, 2,9 GB de RSS).

## 11. Paso 8: code review, ronda 2 (27-10-2026)

Los dos revisores recibieron solo los hallazgos de la ronda 1, sus correcciones y el diff nuevo. Esta vez
los dos compilaron y corrieron `go test -race -short` en su copia. Ninguno dejó hallazgos de severidad
media o mayor: **la ronda converge**. Fable aplicó cuatro mutaciones a `formatTemplate` y las cuatro
murieron; Codex, dos, también muertas.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| F5 | baja | Con `//go:build !race`, ni `make check` ni el CI corren `TestSortedDoesNotAllocate` | **Corregido:** `make check` y el CI corren además `go test -run DoesNotAllocate ./internal/objectkeys` sin `-race` |
| F6 | baja | `checkLocations` saltaba en silencio un elemento desconocido sin `Expression` | **Corregido:** falla |
| F4 | baja | (de la ronda 1) test determinista de `Release` | **Corregido:** `TestReleaseClearsTheBuffer`; mutación m18 |

Después: mutaciones 14/18 (las 4 que sobreviven, de #178; [mutations.txt](evidence/2026-10-26-issue-order/mutations.txt)),
`make check` en verde (145 s, 2,3 GB de RSS).

## 12. Paso 9: cierre (27-10-2026)

| Criterio (§4) | Resultado | Evidencia |
| --- | --- | --- |
| 1. Barrido | 0 inestables en la rama; 13.089 en la base | [sweep.txt](evidence/2026-10-26-issue-order/sweep.txt) |
| 2. Tests de §5 en R4, R4B y R5 | Verdes | `make check` |
| 3. `hl7diff same -baseline a127801 -heavy` | **IDENTICAL** en todos los conjuntos (165 s, 0,66 GB de RSS). Las dos diferencias declaradas no aparecen en el corpus: ningún recurso tiene dos variantes de un elemento de elección ni una extensión con dos `value[x]`. Las cubren los tests de §5 | [same-a127801.md](evidence/2026-10-26-issue-order/same-a127801.md) |
| 4. Mutaciones | 14/18; las 4 que sobreviven están en el código que #178 deja inalcanzable | [mutations.txt](evidence/2026-10-26-issue-order/mutations.txt) |
| 5. Costo | Sin regresión significativa | [report.md](evidence/2026-10-27-0647-bench-compare-a12780150511/report.md) |

Pendiente fuera de este incremento: #178 (contenido de los valores complejos de una extensión), que
además hará alcanzables las mutaciones m6, m7, m8 y m12; el backlog de §2 (orden del documento o un
orden final por ubicación, deduplicar como HL7, el prefijo `value` hardcodeado).
