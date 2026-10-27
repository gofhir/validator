# Un elemento de elección de ElementDefinition es uno solo al fusionar el diferencial (#175)

Rama `fix/snapshot-fixed-choice`, desde `main` `7292a80` (con #173). Proceso: NUCLEO.md §10.

## 1. Revisión previa (25-10-2026)

### El defecto

`mergeElements` (`pkg/registry/snapshot.go`) fusiona el diferencial sobre el elemento de la base
clave JSON por clave JSON. Un elemento de elección de `ElementDefinition` (`fixed[x]`, `pattern[x]`…)
se escribe en JSON como una sola propiedad cuyo nombre lleva el tipo (`fixedString`, `fixedInteger`;
json.html#choice), y su id y extensiones en el hermano `_` (`_fixedString`; json.html#primitive).
Si el diferencial cambia el tipo, la clave de la base sobrevive junto a la del diferencial: el
elemento del snapshot queda con dos `fixed…`, y `fixed[x]` es 0..1.

Medido en `main` `7292a80` con la reproducción del issue (`WithConformanceResources`, un proceso por
corrida):

| Base (A) | Derivado (B) | Instancia | gofhir `main` | HL7 6.10.2 |
| --- | --- | --- | --- | --- |
| `fixedString: "a"` | `fixedInteger: 5` | `valueString: "a"` | error «requires 5», 12/12 procesos | error «Value is 'a' but is fixed to '5'» |
| `fixedInteger: 5` | `fixedString: "a"` | `valueInteger: 5` | **sin error**, 6/6 procesos | (por medir en el paso 7: se espera error, «fixed to 'a'») |

Antes de #173 el resultado cambiaba entre procesos. #173 elige la clave lexicográficamente menor,
así que ahora es estable, pero solo acierta cuando el tipo del diferencial ordena antes que el de la
base. En el otro caso es un falso negativo estable.

### Especificación

- R4 `elementdefinition.html`: `defaultValue[x]`, `fixed[x]`, `pattern[x]`, `minValue[x]` y
  `maxValue[x]` son 0..1. Un elemento de elección es un solo elemento (`formats.html#choice`,
  `json.html#choice`): no puede haber dos propiedades suyas.
- `profiling.html` (snapshot): el snapshot es el resultado de aplicar el diferencial a la base. Lo que
  el diferencial dice de un elemento reemplaza lo que decía la base.
- `profiling.html` §5.1.0.7–8: un derivado no puede relajar ni contradecir a su base. Cambiar el
  `fixed` de A en B es un perfil inválido. Detectarlo es validar el perfil, no generar su snapshot
  (ver §2, fuera de alcance).

### `server` y otras implementaciones

- **`gofhir/server`**: no tiene implementación propia. `internal/service/ops/snapshot/snapshot.go`
  delega en `registry.EnsureSnapshot` de esta librería (v2.1.0), así que su `$snapshot` hereda el
  defecto y lo devuelve en la salida. **Descartar**: no hay nada que tomar. Al adoptar la nueva
  versión de la librería, `server` queda corregido.
- **HL7** (`ProfileUtilities.updateFromDefinition`): el modelo de objetos tiene un solo campo
  `fixed`. `base.setFixed(derived.getFixed().copy())` reemplaza el valor entero, con su tipo y
  extensiones. Medido arriba: el snapshot efectivo de B tiene el 5 y no el "a". Al validar B como
  recurso, HL7 además informa dos errores de perfil («The fixed value has type 'integer' which is not
  valid (valid type: [string])»; «The base element has a fixed type of 'integer', so this element
  must have a fixed value of the same type, not 'string'»). gofhir no informa ninguno: no valida la
  coherencia de un perfil con su base (§2).

### Elementos de elección de ElementDefinition, por versión (snapshots de los packages core)

| Versión | Elementos `[x]` directos | Tipos de `fixed[x]` | Tipos de `minValue[x]` |
| --- | --- | --- | --- |
| R4 4.0.1 | `defaultValue`, `fixed`, `pattern`, `minValue`, `maxValue` | 50 | 9 |
| R4B 4.3.0 | los mismos | 51 | 9 |
| R5 5.0.0 | los mismos | 54 | 10 |

También `example.value[x]`, dentro de `example` (0..*). En gofhir `example` no se acumula (el
diferencial reemplaza el arreglo entero), así que no lo afecta. Si HL7 acumula ejemplos **no está
verificado** (F10), y queda fuera. Los tipos cambian entre versiones en los dos sentidos (medido en
`fixed[x]`): R4B agrega `CodeableReference` y `RatioRange` y quita `Meta`; R5 agrega `integer64`,
`Availability` y `ExtendedContactDetail`, vuelve a tener `Meta` y quita `Contributor`. Por eso salen
del package y no del código.

## 2. Alcance

**Dentro:**

1. Al fusionar, cada elemento de elección directo de `ElementDefinition` es uno solo. Una propiedad
   pertenece al elemento `<nombre>[x]` si se llama `<nombre><Tipo>` o `_<nombre><Tipo>`, con `Tipo`
   (primera letra en mayúscula, json.html#choice) entre los `type.code` que el package de **esa
   versión** declara para el elemento. Si el diferencial trae alguna, se quitan de la base todas las
   de ese elemento, de cualquier tipo y también los `_`, y quedan solo las del diferencial. El valor
   se reemplaza entero, como `setFixed` en HL7. La semántica de «diferencial solo con `_`» se fija con
   la corrida de HL7, **antes** de implementar (paso 5, C8).
2. Los nombres y los tipos salen de la definición de `ElementDefinition`, resuelta con
   `ResolveByCanonical` (pasa por el `ProfileResolver`, F2) en la versión de la `StructureDefinition`
   que se genera (`sd.FHIRVersion`, con la preferida como respaldo, F6). Se calculan en cada
   `applyDifferential`, dentro del `snapshotBuilder` y sin memo en el registro: así no hay nada que
   invalidar al cargar packages, cambiar la versión o clonar (C1, F3). El costo se mide en el paso 7.
3. Una propiedad con el prefijo de un elemento de elección y un tipo que la versión no tiene (en R4,
   `fixedInteger64`) no es una propiedad de ElementDefinition. Queda fuera del snapshot y se informa en
   las notas del snapshot con la razón, como ya pasa con un elemento del diferencial que la base no
   tiene (C4, F5).
4. Sin la definición de `ElementDefinition`, un diferencial vacío se sigue aplicando, porque no hay
   nada que fusionar. Uno no vacío falla con un error explícito, en vez de fusionar clave por clave
   (C3, F2). La ausencia no se memoriza: si la definición se carga después, el siguiente intento
   funciona. **Cambia la API para quien genera snapshots sin el core: aprobado por el usuario (§8).**

**Fuera** (cada uno con su razón):

- Informar que B es un perfil inválido (punto 2 del issue). HL7 lo hace al validar la
  `StructureDefinition` como recurso, no al generar el snapshot ni al validar instancias. gofhir no
  valida la coherencia de un perfil con su base: es un frente propio (validación de perfiles) y queda
  registrado como pendiente.
- `fixed[x]` y `pattern[x]` juntos (`eld-8`): son dos elementos distintos y se conservan los dos. Lo
  que hace HL7 se mide en la misma corrida (F10).
- `accumulateFields` (`constraint`, `mapping`, `code`, `alias`) ya estaba escrito en el código. Queda
  registrado para el eje de dinamismo.
- Se mantiene la clave menor de `extractPrefixedValues`, solo para asegurar el determinismo ante una
  entrada **hipotética**: ninguno de los 9.901 snapshots publicados escaneados tiene dos claves (F9).
- `defaultValue[x]`, `minValue[x]` y `maxValue[x]` no tienen consumidor en `pkg/`. Sus tests verifican
  el snapshot, no la validación de instancias (F7).

## 3. Excepción a la regla 5 (ADR)

El generador trabaja sobre el struct `ElementDefinition` de `pkg/registry`, que ya es el metamodelo
escrito en código (tags JSON, `cloneElement`). `ElementDefinition` se nombra **una sola vez**, en la
constante con la URL canónica de su definición, con un ADR (profiling.html: un snapshot es una lista
de ElementDefinition). Derivarlo de `StructureDefinition.snapshot.element` (C2) obliga a escribir otros
dos nombres, el del recurso y el del path, y no lo vuelve más dinámico (F4). Lo que propone C2 queda
como **verificación del ADR**: un test comprueba que en R4, R4B y R5 el tipo de
`StructureDefinition.snapshot.element` es el de la constante. Todo lo demás (los nombres `[x]` y sus
tipos) sale del package.

## 4. Criterio de salida

1. Pasan los tests unitarios y de integración del §5. Los marcados «rojo primero» fallan en `main`
   `7292a80` antes del cambio, y esa salida se registra.
2. Las dos filas de la tabla del §1 y los casos de `_` dan lo mismo que HL7 en su corrida. Basta un
   proceso: #173 ya quitó la variación entre procesos (F8).
3. **No regresión.** Esto no detecta el defecto: F1 midió 0 duplicados antes del cambio. Se regenera
   el snapshot de cada `StructureDefinition` con diferencial de los packages del manifiesto y se
   informan los intentados, los generados y los fallidos, antes y después. No puede haber fallos
   nuevos ni elementos distintos, comparando el raw completo (C5). Además,
   `hl7diff same -baseline 7292a80 -heavy` sin cambios.
4. Cada mutación del §5 muere en un test unitario o de integración, no solo en el barrido (F1).
5. `make check` en verde.

## 5. Tests y datos

**HL7: una sola corrida, antes de implementar** (tope del usuario). Va A como IG, con varias
instancias y perfiles en la misma invocación:
- el caso inverso;
- base `fixedString:"a"` con diferencial `_fixedString` (mismo tipo) y con `_fixedInteger` (otro tipo);
- base `fixedString` + `_fixedString` con diferencial solo `fixedString:"b"`;
- base `fixedString` con diferencial `patternString`.

Los valores esperados de los tests salen de esa corrida (C8, F10).

### Resultado de la referencia (26-10-2026)

**HL7 6.10.2**, una invocación sobre las 9 instancias de `testdata/snapshot-choice-merge/instances/` con los 9
perfiles de `ig/`. Salida: [`testdata/snapshot-choice-merge/hl7/outcomes.json`](../../testdata/snapshot-choice-merge/hl7/outcomes.json).
Comando (desde ese directorio, en zsh):
`java -jar validator_cli.jar ${=$(ls instances/*.json | grep -v -E -- '-ext\.json$|-extonly\.json$')} -version 4.0.1 -ig ig -tx n/a -output hl7/outcomes.json`.

| Perfil | Instancia | HL7 | Qué fija |
| --- | --- | --- | --- |
| `to-integer` (base `fixedString`) | `valueString:"a"` | «fixed to '5'» | gana el diferencial |
| `to-string` (base `fixedInteger`) | `valueInteger:5` | «fixed to 'a'» | gana el diferencial también cuando su tipo ordena después |
| `value-drops-ext` (base valor + `_`; diferencial solo el valor) | `valueString:"a"` sin extensión | sin error | el `_` de la base se pierde: el valor se reemplaza entero |
| `adds-pattern` (base `fixedString:"a"`, diferencial `patternString:"b"`) | `"a"` y `"b"` | cada una falla contra el otro | `fixed` y `pattern` se conservan los dos |
| `only-ext-same`, `only-ext-other` (diferencial solo `_`) | `valueString:"a"` | solo «Extensions count mismatch» | no distingue si el valor de la base sobrevive |

El último caso se cierra con el **fuente de HL7 6.10.2** (`ProfileUtilities.updateFromDefinition`, líneas 2779–2795):
`derived.hasFixed()` es verdadero para un primitivo con solo extensiones, y `base.setFixed(derived.getFixed().copy())`
reemplaza el valor entero. Por lo tanto, un diferencial solo con `_` deja solo el `_`. En `master` ese bloque no cambia.

Las 4 instancias con extensiones en el valor (`*-ext`, `*-extonly`) quedan fuera: HL7 6.10.2 se cae con un
`NullPointerException` en `InstanceValidator.checkFixedValue` (línea 3111) cuando el valor fijo de una
instancia con extensiones no tiene valor. Es un defecto de la referencia, que se registra aquí con su
reproducción (`only-ext-other` + `only-ext-other-ext.json`).

**Hallazgos del fuente, fuera de alcance** (pendientes, cada uno con su incremento):
- `example`: HL7 **acumula** los del diferencial que la base no tiene, y quita los marcados con la
  extensión de supresión. gofhir reemplaza el arreglo. Confirma F10.
- `defaultValue[x]`: «profiles cannot change: isModifier, defaultValue, meaningWhenMissing». HL7 ignora el
  del diferencial; gofhir lo copia. La especificación (R4, `ElementDefinition.defaultValue[x]`) dice que
  solo se define en un recurso, tipo de dato o extensión, nunca en un perfil sobre ellos.
- `minValue[x]`, `maxValue[x]` y `maxLength`: en `master` (posterior a 6.10.2), si el derivado relaja el
  límite, gana el de la base, con una advertencia.

Este incremento trata los cinco elementos de elección del mismo modo: si el diferencial trae uno, el
suyo reemplaza al de la base. Lo de `defaultValue` es otra regla y no se mezcla aquí.

**Unitarios** (`pkg/registry`), en R4, R4B y R5, con los elementos de elección y sus tipos leídos del
package de cada versión. Se compara el elemento completo (raw), no solo las claves de elección (C5):

| Caso | Base | Diferencial | Esperado | Rojo primero |
| --- | --- | --- | --- | --- |
| Cambio de tipo, en cada elemento de elección y en los dos sentidos del orden alfabético | `<n>T1` | `<n>T2` | solo `<n>T2`; el resto del elemento, intacto | sí |
| Mismo tipo | `fixedString:"a"` | `fixedString:"b"` | `"b"` | no (regresión) |
| Base con `_` | `fixedString` + `_fixedString` | `fixedInteger` | solo `fixedInteger` | sí |
| Mismo tipo, base con `_` | `fixedString` + `_fixedString` | `fixedString` | solo `fixedString` (HL7: `value-drops-ext`) | sí |
| Diferencial solo `_`, mismo tipo y otro tipo | `fixedString:"a"` | `_fixedString` / `_fixedInteger` | solo el `_` del diferencial (fuente HL7) | sí |
| Diferencial con valor y `_` | `fixedString` | `fixedInteger` + `_fixedInteger` | los dos del diferencial | sí |
| Elementos distintos | `fixedString` | `patternString` | los dos | no |
| Tipo que la versión no tiene | `fixedString` | R4: `fixedInteger64`; R5: `fixedContributor` | queda `fixedString`, con nota | sí |
| El mismo tipo, en la versión que sí lo tiene | `fixedString` | R5: `fixedInteger64`; R4: `fixedContributor` | solo el del diferencial | sí |
| Slice de tipo | `value[x]` con `fixedString` | `valueInteger` con `fixedInteger` | el slice, solo `fixedInteger`; `value[x]` conserva `fixedString` | sí |
| Cadena A→B→C | A `fixedString`, B `fixedInteger`, C `fixedBoolean` | — | C: solo `fixedBoolean` | sí |
| Orden de las claves | permutaciones del JSON | — | el mismo snapshot | no (metamórfico) |
| Sin definición, diferencial vacío | — | — | se aplica, como hoy | no |
| Sin definición, diferencial no vacío, y después se carga | — | — | error; el reintento funciona | sí |
| Definición por el `ProfileResolver` | — | — | se usa | sí |
| ADR: tipo de `StructureDefinition.snapshot.element` | R4, R4B, R5 | — | es la constante | — |

**De integración** (API pública): las filas del §1 y los casos de `_`, contra los resultados de HL7.

**Mutaciones** (en una copia; cada una tiene que morir en un test unitario o de integración):
- (m1) no quitar los `_` de la base;
- (m2) una lista fija de `fixed`/`pattern`: muere con `defaultValue`, `minValue` y `maxValue`;
- (m3) agrupar por prefijo de texto: muere con un tipo que la versión no tiene (C6, F5);
- (m4) quitar solo cuando el tipo cambia;
- (m5) los tipos de una sola versión;
- (m6) volver a la fusión clave por clave;
- (m7) quitar también la propiedad del diferencial;
- (m8) conservar la de la base y descartar la del diferencial (C5);
- (m9) no aplicarlo en la copia del slice (C7).

## 6. Dependencias nuevas

Ninguna.

## 7. Ronda adversarial (25-10-2026)

Revisores: **Fable** (F, con sondas Go en su copia: 9.901 snapshots publicados y 1.633 regenerados) y
**Codex** (C; su sandbox no le dejó compilar y solo hizo sondas Python de lectura). Lo que verifiqué
yo está marcado como «verificado».

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| F1 | alta | El barrido y `hl7diff` dan 0→0: no detectan el defecto ni m6 | **Aceptado.** Pasan a ser de no regresión, y las mutaciones tienen que morir en tests (§4.3–4) |
| C5 | alta | «Cero duplicados» no detecta un valor perdido ni el valor equivocado; `checkRegenerated` ignora los errores | **Aceptado.** Se compara el raw completo y se cuentan los intentados, generados y fallidos; se agregan m7 y m8 |
| C3, F2 | alta | El error sin definición rompe `New()` sin core (`TestApplyDifferential_EmptyDiff`, verificado) y el modo con solo resolver | **Aceptado en parte.** `ResolveByCanonical`; el diferencial vacío sigue funcionando; la ausencia no se memoriza. El error con un diferencial no vacío lo decide el usuario (§8) |
| C1, F3 | alta/media | Un memo por registro queda viejo al cargar, cambiar de versión o clonar | **Aceptado.** Se calcula en cada `applyDifferential` |
| C4, F6 | media | Las versiones también quitan tipos (verificado: R4B quita `Meta`, R5 quita `Contributor`); `CodeableReference` es de R4B, no de R5; un registro con varias versiones elige por `fhirRank` | **Aceptado.** Tabla del §1 corregida; la definición se elige según `sd.FHIRVersion`; tests en los dos sentidos |
| F5, C6 | media | `fixedInteger64` en R4 sin decidir; `maxLength` no mata a m3 | **Aceptado.** §2.3; m3 muere con el tipo que la versión no tiene |
| C2 / F4 | media | C2: derivar el tipo de `StructureDefinition.snapshot.element` (verificado: es `ElementDefinition` en las 3). F4: eso agrega dos nombres sin ganar dinamismo | **Resuelto con los dos.** Una constante con ADR; lo de C2 es el test del ADR (§3) |
| C7, F7a | media | Faltan las rutas de slice, reslice y cadena de perfiles | **Aceptado** (slice de tipo, cadena A→B→C, m9) |
| C8 | media | La semántica del `_` solo se supone | **Aceptado.** La corrida de HL7 va antes de implementar |
| F7 | baja | Cuatro tests ya pasan en `main`; `defaultValue/minValue/maxValue` no tienen consumidor | **Aceptado.** Columna «rojo primero»; anotado en §2 |
| F8 | baja | N = 12 procesos ya no mide nada después de #173 | **Aceptado.** Un proceso |
| F9 | baja | La clave menor protege de un caso no observado (0/9.901) | **Aceptado.** Se la llama «hipotética» |
| F10 | baja | No está verificado que HL7 reemplace `example` ni que conserve `fixed` y `pattern` juntos | **Aceptado.** `example`, fuera de alcance como no verificado; `fixed` + `pattern` entra en la corrida de HL7 |

## 8. Decisiones del usuario

1. **Sin la definición de `ElementDefinition` y con un diferencial no vacío:** ¿error (recomendado,
   porque hoy, si cambia un tipo, fusiona mal sin avisar) o fusión clave por clave con una nota en el
   snapshot? El validador y `server` siempre cargan el core, así que solo afecta a quien use
   `pkg/registry` sin él.

**Aprobado por el usuario (26-10-2026), con las recomendaciones:** error explícito en §8.1.

## 9. Registro del incremento

### Pasos 5 y 6: tests primero e implementación (26-10-2026)

- Tests: `pkg/registry/choice_merge_test.go` (unitarios, en R4, R4B y R5, con los elementos de elección
  y sus tipos leídos del package de cada versión) y `pkg/validator/choice_merge_test.go` (integración
  con los perfiles e instancias de `testdata/snapshot-choice-merge`, contra la salida de HL7).
- Rojo primero, sobre `main` `7292a80`:
  [`evidence/2026-10-26-snapshot-choice-merge/red-before.txt`](evidence/2026-10-26-snapshot-choice-merge/red-before.txt).
  Fallan los casos marcados «rojo primero», en las tres versiones; los de regresión (mismo tipo,
  `fixed` + `pattern`, orden de claves) pasan. En integración, fallan el caso inverso (sin error) y
  `value-drops-ext` (la extensión de la base sobrevivía).
- Implementación: `pkg/registry/choice_index.go` (índice de propiedades por elemento de elección,
  desde la definición de `ElementDefinition` de la versión del perfil) y `mergeElements`, que
  reemplaza el elemento entero. ADR-001 (`docs/adr/ADR-001-elementdefinition-metamodel.md`).

### Paso 7: validación (26-10-2026)

- **Barrido de no regresión** (`TestDumpRegenerated`, solo con `-regenerated-dump`):
  [`regenerated-sweep.txt`](evidence/2026-10-26-snapshot-choice-merge/regenerated-sweep.txt). El
  oráculo se probó con una mutación.
- **`hl7diff same -baseline 7292a80 -heavy`**:
  [`hl7diff-same.txt`](evidence/2026-10-26-snapshot-choice-merge/hl7diff-same.txt).
- **Mutaciones** (`mutations.py`, `run-mutations.sh`):
  [`mutations.txt`](evidence/2026-10-26-snapshot-choice-merge/mutations.txt). m5 (los tipos de una sola
  versión) sobrevivió en la primera corrida, porque todos los registros de los tests eran de una sola
  versión. Se agregó `TestMergeChoiceUsesTheProfilesVersion` (un registro con la definición de R4 y la
  de R5) y se repitieron las 13.
- **Diferencias con HL7 que no son de este incremento** (pendientes, a clasificar con evidencia):
  - Un tipo que el perfil no permite (`to-integer-integer-5`, `to-string-string-a`): HL7 dice «allows
    for the type X but found type Y»; gofhir, `STRUCTURE_UNKNOWN_ELEMENT`. Las dos informan error.
  - Un valor fijo sin valor, solo con extensiones (`only-ext-*-plain`): las dos informan la extensión
    que falta; gofhir además, `FIXED_VALUE_EXTRA` («value present, but the fixed value has none»).
  - HL7 6.10.2 se cae (`NullPointerException`, `InstanceValidator.checkFixedValue:3111`) al validar
    una instancia con extensiones contra un valor fijo sin valor (`only-ext-other` +
    `only-ext-other-ext.json`). Defecto de la referencia: issue por redactar para `hapifhir/org.hl7.fhir.core`.
- **Costo** (`make bench-compare BASE=7292a80`):
  [`evidence/2026-10-26-1104-bench-compare-7292a809d6c3/report.md`](evidence/2026-10-26-1104-bench-compare-7292a809d6c3/report.md).
  Bytes y asignaciones por operación iguales en todos los benchmarks. En tiempo no hay ninguna
  diferencia significativa, pero la dispersión fue de ±30–180 %, así que la corrida no permite
  afirmar nada sobre el tiempo. Además, esos benchmarks validan contra perfiles con snapshot
  publicado y casi no generan snapshots. La ruta nueva la mide el barrido: 4.224 snapshots en 15,0 s
  (base) y 15,9 s (rama), una sola corrida de cada uno.
- **Costo de `choiceIndexFor`** (Fable, en el code review, sonda en R4): 13,7 µs, 26 KB y 196
  asignaciones por llamada, una por snapshot generado; `generateSnapshot` de un perfil de un elemento
  tarda 260 µs. Despreciable en perfiles reales; no se agrega un memo, que exigiría invalidarlo (C1 y F3
  de la ronda adversarial).

### Paso 8: code review, ronda 1 (26-10-2026)

Revisores: **Fable** (F; compiló y corrió tests y sondas en su copia) y **Codex** (C; su sandbox no le
dejó crear la caché de Go: solo hizo sondas Python). Cada hallazgo se verificó con un test que falla en
el código anterior a la corrección (`TestMergeChoiceElementDefinitionResolution`,
`TestMergeChoiceDefinitionFailureIsNotPermanent`, `TestMergeChoiceDropsTheBasesForeignProperty`).

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| C2 | alta | `ResolveByCanonical` devuelve nil también cuando el resolver falla (`registry.go:598`): se caía a la definición de otra versión y ese snapshot quedaba guardado | **Corregido.** Primero el registro (la versión exacta y, si no está, la preferida); el resolver solo si no hay ninguna cargada, una vez, sin versión y sin respaldo. Test: el resolver falla una vez, el primer intento falla y el segundo genera |
| F1 | media | Con `fhirVersion` 4.0.0 y un resolver, se lo consultaba en cada snapshot (5 perfiles, 5 llamadas) | **Corregido** con lo mismo. Test: con la definición cargada, el resolver no se consulta |
| C1 | media | Una propiedad de la base con un tipo que la versión no declara sobrevivía al reemplazo, y la clave menor podía elegirla | **Corregido:** se quita con el elemento reemplazado. Test con una base armada a mano |
| C3, F3 | baja | Un `permanentError` de la definición de ElementDefinition se propagaba con `%w` y quedaba permanente en el perfil, contra lo que dice el ADR | **Corregido:** se envuelve con `%v`. Test: definición sin snapshot, después la completa, el mismo perfil se genera |
| F5 | baja | La integración fijaba 5 de las 9 instancias con salida de HL7; US Core 5.0.1 no está en la caché | **Corregido:** las 4 restantes, con lo que HL7 y gofhir comparten (la diferencia queda en los pendientes); el barrido usa US Core 6.1.0 |
| F6, C4 | baja | `godot`, `unnamedResult`, `prealloc`, mensaje de error duplicado, campo sin comentario, comentarios nuevos en inglés | **Corregido** (`errNoElementDefinition`; comentarios en español; lint con 0 issues) |
| F4 | baja | Costo de `choiceIndexFor` | **Registrado** arriba |
| F2 | baja | Una definición de ElementDefinition sin snapshot da un ciclo en cada intento | **Rechazado.** Los packages core la publican con snapshot, el error lo dice y no es permanente (test de C3) |

Mutaciones después de las correcciones: m1, m4, m5 y m12 se reescribieron para el código nuevo, y se
agregaron m14 (el error de la definición vuelve a ser permanente), m15 (la base conserva la propiedad
de otra versión) y m16 (el resolver antes que el registro).

### Paso 8: code review, ronda 2 (26-10-2026)

Codex compiló y corrió los tests esta vez (cachés de Go dentro de su copia). Fable no encontró nada de
severidad media o mayor; Codex encontró C5, así que la ronda no converge y hay una tercera (la última).

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| C5 | alta | La corrección de C2 dejó sin consultar al resolver cuando había otra release cargada: un perfil R5 en un registro R4 usaba los tipos de R4 (`fixedInteger64` quedaba fuera y sobrevivía el `fixedString` de la base) | **Corregido.** En memoria solo vale la exacta o una de la misma release (`versionorder.SameRelease`, el mismo criterio de `fhirRank`). Si no está y hay resolver, se le pide la versión y, si no la tiene, la última de la misma release; nunca otra release. Sin resolver, la preferida aunque sea de otra release (como antes). Test `TestMergeChoiceAsksTheResolverForTheProfilesRelease`, rojo con el código de la ronda 1 |
| F7 | baja | El mismo caso que C5 | Resuelto con C5. Recordar la falta por versión mientras viva el registro: **rechazado**, exige invalidarlo al cargar y clonar, y solo pesa con perfiles de otra release y un resolver que no la tiene |
| F8 | baja | «resolver fails once» no ataba el primer error a su causa | **Corregido:** exige `errNoElementDefinition` y las dos llamadas al resolver; nuevo subtest «same release from the resolver» |
| F9 | baja | La lista de guías estaba duplicada y divergía (US Core 6.1.0 frente a 5.0.1) | **Corregido:** `regenerationGuides`, compartida; `TestRegeneratedSnapshotsAcrossGuides` pasa con US Core 6.1.0 (59 perfiles, 0 diferencias) |

Después de las correcciones: mutaciones 18/18 (m5, m12 y m16 reescritas; nuevas m17, otra release en
memoria, y m18, sin el reintento de la misma release), `make check` en verde, barrido 4.283 perfiles
sin diferencias con la base, `hl7diff same` idéntico en 49 grupos.

### Paso 8: code review, ronda 3 (26-10-2026): converge

Fable compiló y sondeó (versiones vacías, perfil y registro sin `fhirVersion`, «4.0», resolver que
ignora la versión, `-race`): **ningún hallazgo de severidad media o mayor**. Codex no pudo escribir en
su copia (sandbox) y no corrió nada; su sospecha sobre la ruta del resolver sin la versión exacta la
cubrió la sonda de Fable con un resolver que ignora la versión. Tope de 3 rondas alcanzado, convergido.

| Id | Sev. | Hallazgo | Resolución |
| --- | --- | --- | --- |
| F10 | baja | Una definición sin `fhirVersion` se rechazaba solo con resolver, a diferencia de `fhirRank` | **Corregido:** `sameRelease` la acepta, como `fhirRank`. Test `TestMergeChoiceDefinitionWithoutFHIRVersion`, rojo antes |
| F11 | baja | Sin la release en memoria ni en el resolver, se consulta al resolver en cada snapshot (1–2 llamadas por perfil y por intento) | **Registrado:** consecuencia de rechazar F7; solo pasa con perfiles de una release que nadie entrega, y esos perfiles fallan |
| F12 | baja | Sin test: aceptar del resolver otra release, y la rama sin resolver | **Corregido:** `TestMergeChoiceOtherRelease` (mutaciones m19 y m20) |
| F13 | trivial | `version == ""` muerto; nombre del subtest | **Corregido** |

Después: mutaciones 21/21 (m17 reescrita; nuevas m19, m20 y m21), `make check` en verde con 0 issues,
barrido 4.283 perfiles sin diferencias con la base, `hl7diff same` idéntico en 49 grupos. ADR-001:
«Consecuencias» dice qué fallas no son permanentes y qué release se usa.

### Paso 9: cierre (26-10-2026)

Incremento cerrado sin commit. Pendientes que no son de #175 (observados contra HL7 6.10.2, a
priorizar como frentes propios):

- HL7 acumula `example` del diferencial; gofhir los reemplaza.
- HL7 ignora `defaultValue[x]` en el diferencial; gofhir lo copia.
- `master` de HL7: en `minValue[x]`, `maxValue[x]` y `maxLength` gana el límite más estricto.
- Tipo no permitido por el perfil: HL7 lo informa como error de tipo; gofhir como
  `STRUCTURE_UNKNOWN_ELEMENT`.
- Valor fijo que es solo extensión: gofhir agrega `FIXED_VALUE_EXTRA`, HL7 no.
- Defecto de HL7 6.10.2: NPE en `InstanceValidator.checkFixedValue` con instancias que traen
  extensiones en un valor fijo; redactar el issue para `hapifhir/org.hl7.fhir.core` (lo publica el
  usuario).
