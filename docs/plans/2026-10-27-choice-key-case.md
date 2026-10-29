# #180: una clave de elección con otras mayúsculas no es un elemento

Issue: [#180](https://github.com/gofhir/validator/issues/180). Rama `fix/choice-key-case`, desde `main`
(`5c0001c`). Salió de #178 (plan [2026-10-27-extension-value-content.md](2026-10-27-extension-value-content.md),
§2.10): `valueuri` en una extensión es una pérdida conocida de ese incremento.

## 1. Revisión previa (27-10-2026)

### Especificación

- **formats.html#choice** (R4 2.6.0.1.2, R5 2.1.6.0.1.2; R4B igual): «The "nnn" part of the name is
  constant, and the "[x]" is replaced with the title-cased name of the type that is actually used».
- **json.html** (R4 2.6.2, R5 2.1.6.4): «Property names are case-sensitive (though duplicates that
  differ only in case are never defined)».

Así, `valuestring`, `valueSTRING`, `valuequantity` y `effectiveDatetime` no son propiedades de
Observation: son desconocidas, igual que en HL7.

### Código actual

- `pkg/structural/structural.go:386` `findMatchingChoiceType` compara el sufijo de la clave con cada
  tipo del elemento con `strings.EqualFold`. La usan `matchChoiceType` (exclusión mutua, `:163`) y
  `resolveElementDefinition` (`:322`, también para el `_clave` en `validateShadow`).
- `pkg/primitive/primitive.go:254` tiene una copia igual, también con `EqualFold`. Su bucle
  (`:233`) además no filtra la elección por el padre, a diferencia de la estructural. Como solo usa el
  tipo, el resultado no cambia.
- `Registry.ChoiceType` (`pkg/registry/registry.go:668`) ya resuelve la clave exacta según
  formats.html#choice: el sufijo empieza con mayúscula y nombra un tipo que el registro define. La usa
  `internal/elementvalues` en las fases de extensiones, referencias, cardinalidad y contextos.
- `pkg/fixedpattern/fixedpattern.go:174` `resolveElementDef` empareja el prefijo sin mayúsculas y
  sin mirar el tipo. Es el recorrido de `fixedpattern.New`/`Validator.ValidateData`, que el pipeline
  **no usa**: la fase usa `fixedpattern.NewChecker` a través de `constraint`. Es API pública.

### gofhir/server (congelado)

No tiene validación estructural propia: depende de `gofhir/validator/v2` v2.1.0. Su extractor de
índices no lee claves de elección desde la instancia. **Descartar:** no hay nada que tomar.

### Medición (HL7 6.10.2 contra `5c0001c`)

Evidencia en [evidence/2026-10-27-choice-key-case/](evidence/2026-10-27-choice-key-case/)
(`cmp-base.txt`, comando en `README.txt`). R4, R4B y R5 dan lo mismo.

| Sonda | HL7 | Nosotros |
| --- | --- | --- |
| s01 `valuestring`, s02 `valueSTRING`, s03 `valuequantity`, s07 `effectiveDatetime`, s10 `deceasedboolean` | Unrecognized property | nada |
| s04 `valueString` y `valuestring` | Unrecognized property `valuestring` | exclusión mutua de `value[x]` |
| s05 `_valuestring` | Unrecognized property | nada |
| s06 `effectivedateTime: "bad"` | Unrecognized property | formato de dateTime en `Observation.effectivedateTime` |
| s08 `valueQuantity` (control) | nada | nada |
| s09 `value` | Unrecognized property | Unknown element (igual) |
| s11 extensión del núcleo con `valuedateTime` | Unrecognized property, y además lo que sigue de no tener valor | solo lo que sigue de no tener valor |
| s12, f02, f04 perfil con fixed/pattern y la clave en minúscula | Unrecognized property | nada: el valor pasa sin revisar |
| f06 `valueBoolean` donde el perfil permite solo string con `fixedString` | tipo no permitido y fixed | Unknown element y fixed |

f06 no es de #180: HL7 también revisa el fixed, y lo que difiere es el mensaje del tipo, igual en la
base.

## 2. Alcance (ajustado después de la ronda adversarial, §7)

La regla: una propiedad `nnn` + sufijo es un elemento de elección `nnn[x]` solo si el sufijo es
exactamente el código de un tipo con la primera letra en mayúscula (formats.html#choice; json.html:
los nombres distinguen mayúsculas). Qué tipos dan **propiedades** lo dice la definición **base** del
elemento. Un perfil que restringe los tipos no cambia los nombres: hace que el valor sea del tipo
equivocado. HL7 lo trata así (§7, A1).

1. **Una sola función para la comparación.** `registry.(*ElementDefinition).ChoiceTypeOf(suffix)`
   devuelve el código del tipo declarado cuyo nombre con inicial mayúscula es exactamente `suffix`, o
   `""`. Un tipo sin código no coincide. La usan `internal/elementvalues` (en lugar de su `declared`)
   y las fases de abajo.
2. **Fase estructural:** `findMatchingChoiceType` pasa a ser `ChoiceTypeOf`. Una clave que no
   coincide es un elemento desconocido, también como `_clave`. La exclusión mutua cuenta solo las
   variantes, y la búsqueda previa salta las claves que ya son un elemento exacto (`byPath`).
3. **Fase primitiva:** resuelve la elección del **padre** (como la estructural, #59) con los tipos de
   la definición base del tipo del recurso (`GetByType(sd.Type)`, como `baseTypes` en #178). Así:
   - no revisa el formato de una clave que no es propiedad (s06, q1);
   - sí lo revisa en un valor de un tipo que el perfil no permite, como HL7 (a1, a1b).
4. **`Registry.ChoiceType`** solo acepta tipos de datos: una definición de `kind` `primitive-type` o
   `complex-type` (datatypes.html; StructureDefinition.kind). Un modelo lógico (`valueRequest`) o un
   recurso (`valuePatient`) no nombra un valor: no se recorre ni se cuenta su cardinalidad (q3, q4,
   q6). Tampoco un tipo abstracto (`StructureDefinition.abstract`): HL7 informa `valueElement`,
   `valueBackboneElement` y `valueDataType` (R5) solo como desconocidos, y nosotros les agregamos
   `ele-1` (paso 5). En los packages, Element y BackboneElement son abstractos en R4, R4B y R5;
   DataType en R4B y R5; PrimitiveType y BackboneType en R5. `derivation` no cuenta:
   SimpleQuantity y MoneyQuantity tienen `type` Quantity, y `GetByType` no las alcanza por nombre.
5. **`fixedpattern.Validator` (deprecado, sin uso en el pipeline):** `resolveElementDef` usa
   `ChoiceTypeOf` en lugar del prefijo sin mayúsculas ni tipo. Retirar la API queda para v3, en un
   issue aparte. **Decisión del usuario** (§8).
6. **#178:** el caso `valueuri` de `TestExtensionValueContent` deja de ser pérdida conocida y exige el
   elemento desconocido.

**Fuera de alcance:**

- El mensaje de un valor cuyo tipo restringe un perfil: HL7 dice «allows for the type X but found
  type Y» y nosotros «Unknown element» (f06, a1). Es anterior a #180; va al backlog.
- `registry/prefixed_values.go` lee `fixed…`/`pattern…` de las definiciones, no de las instancias.

## 3. Excepciones a la regla 5

Ninguna. La regla sale de formats.html#choice. Los tipos salen de `ElementDefinition.type` de la
definición base, y la clase de tipo, de `StructureDefinition.kind` y `abstract` del package cargado.

## 4. Criterio de salida

1. Las sondas `probe/`, `probe-fp/` y `adversarial/` dan lo que da HL7 en las tres versiones. Cuentan
   el elemento desconocido, la exclusión mutua, el formato y la cardinalidad; los mensajes de §2
   «fuera de alcance» no. Hay casos por versión: `asNeededBoolean` es válido en R4 y R4B y desconocido
   en R5.
2. El caso `valueuri` de #178 exige `STRUCTURE_UNKNOWN_ELEMENT`.
3. `hl7diff same` contra `5c0001c`: solo diferencias explicadas por este cambio. `hl7diff run`: PASS
   en los 49 grupos, sin falsos positivos nuevos.
4. Metamórfico sobre los ejemplos oficiales R4, R4B y R5: cambiar las mayúsculas del sufijo de una
   clave de elección da los issues de borrar esa clave, más el elemento desconocido en ella.
5. Mutaciones: comparar sin mayúsculas en `ChoiceTypeOf`; quitar el filtro por el padre o la base en
   la fase primitiva; quitar el filtro de `kind` o el de `abstract` en `ChoiceType`; volver al prefijo en `fixedpattern`;
   no saltar `byPath` en la búsqueda previa. Cada una tiene que hacer caer algún test.
6. Costo: `make bench-compare BASE=5c0001c` sin regresión significativa.
7. `make check` en verde.

## 5. Tests

- `pkg/validator/choice_key_case_test.go` (nuevo): las sondas como casos en las tres versiones, con
  valores esperados de HL7 (`hl7-*.json`, `hl7-fp.json`, `adversarial/`), citando formats.html#choice.
  Incluye:
  - controles válidos (`valueQuantity`, `effectiveDateTime`, `valueString`, `valueCodeableConcept`);
  - elecciones en BackboneElement y por contentReference (`Questionnaire.item.item`);
  - contained y Bundle;
  - el `_clave` de un primitivo;
  - una extensión;
  - un elemento no elección que empieza igual (`ConceptMap...dependsOn.valueSet` en R5);
  - `asNeededBoolean` por versión.
- Los perfiles de las sondas van a `pkg/validator/testdata/choice-key-case/`.
- `pkg/structural/structural_test.go` `TestFindMatchingChoiceType` congela la comparación sin
  mayúsculas: se reescribe desde formats.html#choice. Solo `Boolean`, `DateTime` y `CodeableConcept`
  resuelven; las variantes en minúscula dan `""`.
- Test de unidad de `ChoiceTypeOf` (incluido un tipo sin código) y de `ChoiceType` con `kind`.
- El metamórfico del §4.4, como test con los packages de ejemplos.

## 6. Dependencias

Ninguna nueva.

## 7. Ronda adversarial (27-10-2026)

Fable y Codex en paralelo, sin mi razonamiento. Codex revisó solo leyendo el código, porque su sandbox
no le dejó compilar. Sondas en HL7 6.10.2:
[evidence/2026-10-27-choice-key-case/adversarial/](evidence/2026-10-27-choice-key-case/adversarial/).

| Hallazgo | Resolución |
| --- | --- |
| Codex A1 / Fable 2 (alta/media): la fase primitiva no filtra por el padre y sí cambia el resultado | **Aceptado, con otra corrección.** q1 (`answerOption.valueDateTime: "bad"`, R4 y R5): HL7 da solo desconocido, nosotros además el formato. Pero a1 (perfil: dateTime en la raíz, Quantity en component) muestra que HL7 sí revisa el formato del valor de un tipo que el perfil no permite. Filtrar solo por el padre arreglaría q1 y haría retroceder a1. Se resuelve con el padre y los tipos de la definición base (§2.3), que reproduce q1, a1 y a1b. |
| Codex A2 (alta): `ChoiceType` exige que el tipo esté en el registro | **Aceptado en parte.** Las fases estructural y primitiva comparan contra los tipos declarados con `ChoiceTypeOf` (§2.1), sin el registro. `ChoiceType` sigue para `elementvalues`, que necesita reconocer un tipo no declarado (#178). Un registro parcial sin los tipos del núcleo no puede validar nada, así que no es un caso a cubrir. |
| Fable 4 (media): una cuarta copia de la regla; `ChoiceType` acepta modelos lógicos | **Aceptado.** Una sola `ChoiceTypeOf` (§2.1). q3/q6 (`valueRequest`): HL7 da solo desconocido, nosotros tres `CARDINALITY_MIN` y `ele-1`. q4 (`valuePatient`): nosotros `ele-1`. Se filtra por `kind` (§2.4). |
| Codex A3 / Fable 3 (media): el metamórfico «ningún otro issue» es falso (s11, s12) | **Aceptado:** la relación es la de borrar la clave más el desconocido (§4.4). |
| Codex A4 / Fable 6 (media/baja): fixed/pattern medido solo en R4; faltan BackboneElement, contained/Bundle y casos por versión; no hay `go-fp.json` | **Aceptado:** los tests corren las sondas en las tres versiones (§5). `go-fp.json` agregado y el sesgo de `cmp.py` declarado en `README.txt`. q5 confirma `asNeededBoolean` por versión. |
| Codex A5 / Fable 1 (media): `TestFindMatchingChoiceType` congela el defecto | **Aceptado** (§5). |
| Codex A6 / Fable 8 (media): `fixedpattern.Validator` ya está deprecado | **Aceptado:** corrección en este incremento y retiro en un issue aparte (§2.5). Un comentario no reemplaza cumplir la especificación. Lo decide el usuario. |
| Fable 5 (baja): `ToUpper(code[:1])` con un tipo sin código | **Aceptado:** `ChoiceTypeOf` lo guarda (§2.1). |
| Fable 7 (baja): la búsqueda previa de la exclusión mutua no consulta `byPath` | **Aceptado** (§2.2), con el caso ConceptMap R5 (`valueSet` + `valueCode`). |

## 8. Decisión del usuario (27-10-2026)

Aprobó el alcance del §2, con la corrección de `fixedpattern.Validator` (§2.5) en este incremento.

## 9. Tests primero (28-10-2026)

- `TestChoiceKeyCase` (`pkg/validator/choice_key_case_test.go`): los casos de §5, con los valores
  esperados de HL7 6.10.2 (`tests-hl7/`). Los tipos abstractos se agregaron al medir que nuestro
  `ele-1` sobre `valueElement` no lo informa HL7 (§2.4).
- `TestChoiceType` corre en R4, R4B y R5, con modelo lógico, recurso y los tipos abstractos de cada
  versión. `TestChoiceTypeOf` (registry) reemplaza a `TestFindMatchingChoiceType` (structural), que
  exigía la comparación sin mayúsculas.
- #178: los dos casos `valueuri` exigen el elemento desconocido; HL7 lo informa en ambos
  (`evidence/2026-10-27-extension-value-content/adversarial/hl7-adv-r4.json`).
- `TestChoiceKeySweep` (metamórfico, §4.4, solo con `-choice-key-sweep=<archivo>`): en cada recurso de
  los ejemplos oficiales altera las mayúsculas de hasta tres claves de elección (también en
  `contained` y en las entradas de un Bundle) y exige los issues de borrar la clave más el elemento
  desconocido. Las claves salen de los tipos de `value[x]` en el snapshot, no del validador.
- Rojo antes de implementar: `red-before.txt` (TestChoiceKeyCase y TestExtensionValueContent);
  `sweep-before.txt.xz` (comprimido; `xz -dk` lo descomprime), 2736 claves (R4 1113, R4B 819, R5 804) en 73 s y 1,09 GB de RSS, todas distintas.
  `sweepcls.py` las clasifica (`sweep-before-classes.txt`): 2588 solo por el elemento desconocido
  que falta (#180); 148 además porque borrar la clave da `dom-3` y alterarla no.

**Hallazgo fuera de #180: FHIRPath navega propiedades que el modelo no define.** Con
`fooBar` en lugar de `productReference` en CarePlan-f202, HL7 informa la propiedad desconocida y
`dom-3`, como al borrarla; nosotros solo la propiedad desconocida (`dom3/`). Una sonda con
fhirpath v1.13.6 y nuestro modelo da `children()` con `fooBar` aunque `ChildElements("Patient")` no la
lista; v1.16.6 conserva el mismo código (`inDefinitionOrder`). Es anterior a #180 y no depende de las
mayúsculas. Issue redactado para la librería: `dom3/issue-fhirpath-unknown-children.md`; se publica
con la aprobación del usuario: aprobada el 28-10-2026; el clasificador de auto mode bloqueó
`gh issue create`, así que lo publica el usuario. Sin duplicados (búsquedas "children descendants model",
"unknown property", "ChildElements", "undeclared"; el más cercano, #219, es otro caso). **Decisión del
usuario (28-10-2026):** el barrido escribe como KNOWN, sin fallar, solo la diferencia en que la clave
alterada no da los `dom-3` que da la borrada y contiene una referencia local (`#id`); cualquier otra
diferencia falla. No se rodea en el validador.

## 10. Implementación (28-10-2026)

- `registry.ChoiceTypeIndex(types, suffix)` y `(*ElementDefinition).ChoiceTypeOf(suffix)`: la única
  regla de §2.1. `elementvalues` usa `ChoiceTypeIndex` en lugar de su `declared`, porque necesita la
  posición del tipo, no solo el código.
- `Registry.ChoiceType`: solo un tipo de datos concreto (`kind` primitive-type o complex-type, no
  `abstract`), §2.4. Constantes `KindPrimitiveType` y `KindComplexType`.
- Fase estructural: `ChoiceTypeOf` en lugar de `findMatchingChoiceType` (borrada); la búsqueda previa
  de la exclusión mutua salta las claves que `byPath` resuelve.
- Fase primitiva: cada índice guarda el de la definición base de su tipo (`GetByType(sd.Type)`; el
  propio si no es un perfil). La elección se resuelve solo entre los hijos del padre y con los
  tipos de la base (§2.3).
- `fixedpattern.resolveElementDef`: busca `<padre>.<base>[x]` para cada prefijo de la clave y exige
  `ChoiceTypeOf`, sin recorrer el índice ni comparar sin mayúsculas (§2.5). Test nuevo
  `TestValidateChoiceKey`: con el código anterior caen los 6 casos de mayúsculas.
- Barrido después (`sweep-after.txt`): 0 diferencias y 148 KNOWN, las mismas 148 claves de antes.
  Dos de ellas son un canónico local (`definitionCanonical` = `#procedure`), no un `reference`;
  HL7 se comporta igual (`dom3/canonical/`), así que la clase KNOWN cuenta cualquier valor `"#id"`.
- `go test ./...` en verde.

## 11. Validación: vuelta al paso 2 (28-10-2026)

Sondas con el binario nuevo (`after/`; `after/cmp-probe.txt`): `probe/` en las tres versiones,
`probe-fp/`, a1 y a1b dan los errores de HL7, salvo el mensaje de tipo de §2 «fuera de alcance». En a1b,
HL7 da el formato y «allows for the type Quantity but found type dateTime».

**Diferencia:** en q1/q2 (R4 y R5), HL7 informa además
`Questionnaire.item.answerOption.value[x]: minimum required = 1, but only found 0`, y nosotros no; la
línea base tampoco (`adversarial/go-r4.json`). La ronda adversarial (§7, Codex A1 / Fable 2) lo
registró mal: HL7 no da solo el desconocido.

Causa: `elementvalues.choiceProperties` toma como valor del elemento, con el tipo equivocado, toda
clave cuyo sufijo `Registry.ChoiceType` reconoce como tipo de datos (#178). HL7 usa la misma regla que
para los nombres de propiedad: es valor solo un tipo que declara el elemento **base**
(`ElementDefinition.base.path` en la definición del tipo). `Extension.value[x]` declara todos los
tipos en la base, así que el caso de #178 (`valueUri` en una extensión de string) no cambia;
`answerOption.value[x]` declara seis, y `valueDateTime` no es un valor.

Cambio propuesto al §2, pendiente de la aprobación del usuario: en `elementvalues`, la clave cuenta
como valor de un tipo no permitido solo si su sufijo nombra (`ChoiceTypeIndex`) un tipo del elemento
base, resuelto por `ElementDefinition.base.path`. Así se reemplaza `Types.ChoiceType`, y
`Registry.ChoiceType` queda sin uso fuera de sus tests. Como lo usan todas las fases que recorren el
árbol, hacen falta `hl7diff same` y `run` completos.

**Decisión del usuario (28-10-2026):** el cambio entra en #180.

Tests primero:
- `TestChoiceKeyCase`: HL7 da `minimum required = 1` en cuatro casos (`tests-hl7/hl7-cases.txt`), y
  ahora los cuatro lo exigen: q1, la extensión con `valueRequest`, la extensión mal escrita y el
  `initial` bajo un `contentReference`. Antes de cambiar el código, solo falla q1.
- `TestChoiceType` (registry), con la nueva firma `ChoiceType(def, key)`: los tipos de la base
  (`answerOption`: `valueDate` sí, `valueDateTime` no), un perfil que restringe (`valueUri` en
  `Extension.value[x]` de string sí) y un elemento sin `base` (solo sus tipos).
- `TestOf` (elementvalues): un tipo que la base no declara y una clave con otras mayúsculas no son
  valores.

Implementación: `Registry.ChoiceType(def, key)` busca el sufijo con `ChoiceTypeIndex` entre los tipos
del elemento `def.Base.Path` (`GetElementDefinition`); si no hay base, entre los de `def`. Ya no
necesita el filtro de `kind`/`abstract` de §2.4: un elemento base declara solo tipos de datos concretos
(medido: `Extension.value[x]` declara 50, 51 y 54 tipos, ninguno abstracto ni lógico). Lo usan
`elementvalues` (`Types.ChoiceType`) y `slicematch.keyType`.

Tests anteriores que cambian:
- `TestCardinalityOnTheTree`, `deceasedString` con `deceased[x]` obligatorio: el caso suponía la regla
  de #178. HL7 da «Unrecognized property 'deceasedString'» y `Patient.deceased[x]: minimum required =
  1` (`deceased/`), así que ahora exige `CARDINALITY_MIN`.
- `TestSliceChildrenAgainstTheBase`: el snapshot escrito a mano no tenía `base`, que todo elemento de
  snapshot lleva; se le agrega `"base":{"path":"Extension.value[x]"}`.

Resultado: `go test ./...` en verde. Las sondas (`after/counts.txt`) dan el mismo número de errores
que HL7 en los 55 archivos.

**Cambio al §2.2:** se quita el salto de `byPath` en la búsqueda previa de la exclusión mutua. Con la
comparación exacta, ningún elemento de R4, R4B ni R5 coincide con una variante de una elección
hermana (barrido de los snapshots de especialización de los tres packages core: ninguno). El salto era
inalcanzable y ninguna mutación podía detectarlo. El caso R5 de `dependsOn` (`valueSet` junto a
`valueCode`) sigue en `TestChoiceKeyCase`.

## 12. Validación (28-10-2026)

- **Sondas** (`after/counts.txt`): los 55 archivos dan el mismo número de errores que HL7.
- **Metamórfico** (`sweep-after.txt`): 0 diferencias y 148 KNOWN (`dom-3`, issue de fhirpath, §9).
- **`hl7diff same` contra `5c0001c`** (`hl7diff-same-5c0001c.txt`; multiconjunto en
  `hl7diff-same-5c0001c-multiset.txt`). Cambian 2 líneas, ambas en `reference-walk-probes`: dejamos de
  dar `ref-1` en `Observation.valueReference` y en `Patient.multipleBirthReference` (R4). Ninguno de esos
  tipos está en la base de su elección, así que no son valores. `r4-core-examples`, 5301 archivos,
  sale idéntico.
- **`hl7diff run`** (`hl7diff-run-5c0001c.txt`): PASS en los 48 grupos livianos. `reference-walk-probes`
  da «unexplained errors removed: 2», los dos `ref-1` que HL7 no da. Como `r4-core-examples` sale
  idéntico en `same`, su resultado contra HL7 no cambia.
- **Mutaciones** (`mutations.txt`, `run-mutations.sh`): las de #180 y las de #178.
  - #180: caen las 10. `m2` se reescribió para que compile.
  - `m10` (`slicematch` solo con los tipos declarados) sobrevivía; se agregó `TestKeyType`.
  - #178: caen 16. `m12` ya no se aplica: el filtro `baseTypes` de `extension.go` era una segunda
    copia de la regla que ahora aplica `elementvalues` (la mutación sobrevivía), y se borró.
- **Costo** (`bench-compare-5c0001c.txt`): asignaciones y bytes por operación idénticos a la base en
  todos los benchmarks. El tiempo no cambia de forma significativa (todo `~`, varianza de ±25 % a
  ±590 % con la máquina cargada).
- `make check` en verde (lint con 0 issues).

## 13. Review, ronda 1 (28-10-2026)

Revisores: Fable, que verificó cada hallazgo con tests en su copia, y Codex. Codex no pudo escribir en
el scratchpad, revisó leyendo, y cada sospecha suya se confirmó con un test.

| Hallazgo | Resolución |
|---|---|
| Codex 1 / Fable 2 (media): `fixedpattern` usaba los tipos del perfil; `valueBoolean` con `value[x]` solo string y `fixedString` ya no se comparaba con el fijo (HL7 da el tipo y el fijo, f06) | **Aceptado.** Regresión de §2.5. `resolveElementDef` usa `Registry.ChoiceType` (tipos de la base). `TestValidateChoiceKey` agrega el caso; antes del arreglo cae en las tres versiones. Mutación `m6b`. |
| Codex 2 (media): un tipo que declara el perfil y no la base (`patient-citizenship` 5.2.0 con `CodeableReference` en R4) se tomaba como valor: `EXTENSION_VALUE_NOT_ALLOWED` nuevo, y `CARDINALITY_MAX` que ya estaba en la base | **Aceptado.** HL7 da solo la propiedad desconocida (`review-r1/citizenship/`). En `elementvalues`, la base decide si la clave es valor; los tipos del perfil solo dan el orden. Caso nuevo en `TestChoiceKeyCase` (R4), mutación `m11`. Corrige también el falso positivo de cardinalidad que ya estaba. |
| Fable 1 (media): con un snapshot sin `base`, `ChoiceType` usaba solo los tipos del perfil; regresión respecto de #178 (`valueUri` dejaba de ser un valor del tipo equivocado) | **Aceptado.** Sin `base`, la base es el elemento de la misma ruta en la definición del tipo; si tampoco está, los tipos propios. Casos en `TestChoiceType` y en `TestChoiceKeyCase` (`nobase-ext`); mutación `m12`. Fable midió que ningún snapshot core de R4, R4B o R5 tiene una elección sin `base`. |
| Fable 3 (baja): `slicematch.keyType` repetía la regla | **Aceptado.** Ahora llama a `Registry.ChoiceType`. |
| Fable 4 (baja): `"primitive-type"`/`"complex-type"` escritos en otros paquetes | **Fuera de alcance.** Es código anterior que este cambio no toca; va al backlog. |
| Codex (baja): comentarios nuevos en inglés | **Aceptado.** NUCLEO pide comentarios en español: se tradujeron los comentarios nuevos de registry, primitive y structural. Fable consideraba válido seguir el idioma de cada archivo; gana la carta. |

Después de la ronda 1: `make check` en verde. Mutaciones (`mutations.txt`): caen las 13 de #180 (`m6b`,
`m11` y `m12` son nuevas; `m9` y `m5` de #178 tienen nuevos fragmentos) y las 16 de #178 que siguen
aplicándose. `hl7diff same` da las mismas 2 líneas de §12. El barrido sigue con 0 diferencias y 148 KNOWN.

## 14. Review, ronda 2 (28-10-2026)

Los hallazgos de la ronda 1 están corregidos; lo confirman los dos revisores, y Fable con sus tests.
Hallazgos nuevos:

| Hallazgo | Estado |
|---|---|
| Fable 1 (media, confirmado con test): `ChoiceType` resuelve la base con `GetElementDefinition(path)`, que busca por el nombre de la raíz. Un modelo lógico con `type` URL y ruta `Observation` (estilo CDA) cuyo `base.path` es su propia ruta toma los tipos de `Observation.value[x]` del core: pierde `valueCoding` y acepta `valueQuantity`. Afecta snapshots válidos. | Abierto. Defecto de diseño de §11: un nombre de tipo no identifica la definición. |
| Codex 1 (media): lo mismo por el respaldo `def.Path` sin `base` | Abierto; misma causa. |
| Codex 2 / Fable 2 (media/baja): sin `base`, el slice de una subextensión (`Extension.extension.value[x]`) no encuentra la base. HL7 informa el tipo equivocado. | Abierto. Esos snapshots violan sdf-8b («All snapshot elements must have a base definition», R4 y R5). HL7 igual decide por el tipo del objeto (`nobase/`: «allows for the types [string] but found type uri»). |
| Fable 3 (baja, rendimiento): un `RLock` de `GetElementDefinition` por clave de elección; `Of` en paralelo tarda 2x (micro), sin diferencia de punta a punta | Abierto. |

### 14.1 Rediseño aprobado (28-10-2026)

El usuario aprobó corregirlo dentro de #180. La base deja de buscarse por el nombre de la raíz de la
ruta (`GetElementDefinition`) y se busca por la cadena de definiciones:

- `ElementNode` guarda la `StructureDefinition` de cuyo snapshot sale (`buildTree`).
- `Registry.ChoiceType(node, key)`: el elemento base es el de `base.path` (o la ruta propia, si el
  snapshot no la trae) en la primera definición de la cadena `baseDefinition` que no es un perfil y
  lo tiene. Un perfil restringe y no define, así que se salta; un modelo lógico es su propia base.
- Sin base en la cadena (snapshot sin `base`, sdf-8b), se usa el elemento del mismo nombre en el tipo
  del padre: `Extension.value[x]` para el `value[x]` de una subextensión, como hace HL7
  (`review-r2/nested`: «allows for the type string but found type Coding»).
- Si `base.path` no está en la cadena, se busca en la definición del tipo que la ruta nombra: un
  perfil que despliega un tipo de dato (`Observation.effective[x]:effectiveTiming.repeat.bounds[x]`,
  base `Timing.repeat.bounds[x]`) lo toma de Timing. Lo encontró la mutación `m4`, que sobrevivía:
  ni la cadena ni el padre (`repeat`, de tipo Element) llegaban a la base, y `boundsPeriod` contra un
  perfil que deja solo Duration con min 1 daba `SLICING_CARDINALITY_MIN`. HL7 da el tipo equivocado y
  no el mínimo (`review-r2/timing`). La búsqueda por nombre solo ocurre si la cadena no tiene la ruta,
  así que no vuelve la colisión del modelo lógico.
- El resultado se guarda por nodo en `Registry.choiceBases` (un `sync.Map`, que se lee sin lock) y se
  vacía cuando cambian las definiciones (`refreshDerivedUnlocked`). Esto saca el `RLock` por clave
  (Fable 3).
- Cambian los llamadores: `elementvalues.Types.ChoiceType` recibe el nodo; `fixedpattern` recibe el
  árbol del snapshot; `primitive` guarda los elementos de elección como nodos y deja su índice
  `base`, que buscaba la base por `GetByType(sd.Type)`; `slicematch.keyType` pasa el nodo.

Tests: `pkg/registry/choice_type_test.go`. `TestChoiceType` cubre el modelo lógico con y sin `base`,
la subextensión sin `base` y el snapshot sin `base`, en las tres versiones. `TestChoiceTypeAfterLoad`
cubre un perfil cargado antes que su base. `TestChoiceKeyCase` suma la subextensión de punta a punta
(testdata `StructureDefinition-nested-nobase.json`). `TestKeyType` toma sus nodos de árboles reales.
`TestChoiceKeyCase` suma también los casos de Timing (testdata `StructureDefinition-timing-bounds.json`).
Mutaciones nuevas: `m4b` (`base.path` no se busca en su tipo), `m12` (sin el respaldo del padre), `m13` (la base dentro del perfil), `m14` (la
base por nombre de tipo) y `m15` (la caché no se vacía al cargar).

### 14.2 Validación del rediseño

`make check` en verde. Mutaciones (`mutations.txt`): caen las 17 de #180 y las 16 de #178 que se
aplican. `m2` se reescribió porque no compilaba. `m4` (ignorar `base.path`) sobrevivía y llevó a la
búsqueda en el tipo que nombra la ruta (§14.1); ahora cae. `hl7diff same` contra `5c0001c` da las
mismas 2 líneas de §12. El barrido sigue con 0 diferencias y 148 KNOWN. Bench
(`bench-compare-5c0001c.txt`): ningún cambio significativo, y asignaciones iguales.

En curso: ronda 3, la última del tope.

## 15. Review, ronda 3 (28-10-2026): no converge

Codex no pudo escribir en el scratchpad y revisó leyendo; Fable verificó con tests en su copia. Los
hallazgos de la ronda 2 están corregidos: modelo lógico, subextensión sin `base` y lock por clave.

| Hallazgo | Estado |
|---|---|
| Codex 1 (media): un perfil intermedio resuelto a demanda (`ResolveByCanonical`) no vaciaba la caché, porque solo se vacía si cambia un tipo | **Confirmado y corregido.** Cada entrada guarda la generación de las definiciones (`choiceGen`, que suben `indexUnlocked` y cada snapshot generado) y vale solo si sigue siendo la actual. Test: `TestChoiceTypeAfterResolve`, en las tres versiones. |
| Codex 2 / Fable (media): una búsqueda en curso podía guardar una base vieja después de una carga | **Corregido** con la misma generación. Test: `TestChoiceTypeStaleEntry`. `-race` sin carreras. |
| Fable 1 (media, confirmado con test): el respaldo por nombre de §14.1 vuelve a colisionar. Un modelo lógico `Doc` con `Doc.obs` del tipo lógico `Observation` (type URL) y un perfil de `Doc` que despliega `Doc.obs.value[x]` con `base.path` = `Observation.value[x]` toman el `Observation` de FHIR | **Abierto.** Propuesta de Fable: antes del nombre, el tipo del padre por código, que también resuelve las URL. |
| Fable 2 (baja): la caché crece con los nodos armados a mano (`fixedpattern`, elementos sin id) | **Abierto.** No guardar cuando el nodo no tiene definición. |

Por el tope de 3 rondas, el incremento se detiene y espera la decisión del usuario.

### 15.1 Decisión del usuario y corrección (28-10-2026)

El usuario aprobó corregir Fable 1 dentro de #180 y hacer una cuarta ronda, por encima del tope.

- Se quita la búsqueda por nombre de tipo. Si `base.path` (o la ruta propia) no está en la cadena
  `baseDefinition`, se sube por los ancestros del nodo. En el primero cuyo único tipo, resuelto por
  su código (puede ser una URL), define la ruta relativa, se toma el elemento:
  `Timing.repeat.bounds[x]`, `Extension.value[x]` (subextensión), o el `Observation` lógico de
  `Doc.obs` por su URL. El respaldo por el padre de §14.1 queda dentro de este recorrido.
- `base.path` sigue siendo necesario: en un perfil que despliega `item.item` (un `contentReference`,
  sin tipo), solo `base.path` lleva a `Questionnaire.item.answerOption.value[x]`. HL7 da
  `valueDate` como tipo equivocado y `valueDateTime` como propiedad desconocida (`review-r3/item`).
- Un nodo sin definición (armado a mano) no se guarda en la caché (Fable 2).

Tests: `TestChoiceType` suma `Doc.obs.value[x]` y `item.item.answerOption.value[x]`;
`TestChoiceTypeHandBuiltNode` es nuevo. Mutaciones reescritas o nuevas: `m4` (sin `base.path`),
`m4b` (sin ancestros), `m12` (solo el padre), `m14` (cadena desde el nombre del tipo), `m15` (la
generación no sube al indexar), `m16` (se usa una entrada vieja) y `m17` (se guarda el nodo armado a
mano).

Validación (28-10-2026): `make check` en verde. Mutaciones (`mutations.txt`): caen las 19 de #180 y
las 16 de #178 que se aplican. `hl7diff same` da las mismas 2 líneas de §12. El barrido da 0
diferencias y 148 KNOWN. Bench: sin cambios de tiempo; los únicos significativos son de memoria y
menores a 0,1 %. En curso: ronda 4.

## 16. Review, ronda 4 (adicional, aprobada por el usuario; 28-10-2026)

Fable, que verificó con tests: los hallazgos de la ronda 3 están corregidos. No encontró elección
equivocada en el recorrido de ancestros (reslices, subextensiones anidadas, `baseDefinition` con
versión) ni subidas de generación faltantes. `-race` sin carreras, también con 8 goroutines y cargas
concurrentes. Sin hallazgos.

Codex leyó el código pero no pudo correr nada; las dos sospechas se confirmaron con tests:

| Hallazgo | Estado |
|---|---|
| Codex 1 (media): `base.path` es una ruta, no una definición. Dos modelos lógicos A y B tienen la raíz `Observation`; el perfil P de A despliega `Observation.obs` (tipo B) con `base.path` `Observation.value[x]`, que la cadena de P encontraba en A | **Confirmado y corregido.** Ahora manda la estructura: primero la ruta propia en la cadena, luego los ancestros con tipo (ruta relativa y, si no, `base.path` en ese tipo) y, por último, `base.path` en la cadena propia. Test: `TestChoiceTypeAmbiguousBasePath`, en las tres versiones. |
| Codex 2 (media): bajo un `contentReference` de un tipo desplegado (`Doc.t.item.item.value[x]`, base `T.item.value[x]`), la ruta relativa `T.item.item.value[x]` no existe | **Confirmado y corregido:** en el ancestro se prueba también `base.path`. Mismo test. |

Mutaciones nuevas: `m4c` (`base.path` no se prueba en el tipo del ancestro) y `m4d` (`base.path` antes
que la estructura).

Validación después de la ronda 4 (28-10-2026): `make check` en verde. Mutaciones (`mutations.txt`):
caen las 21 de #180 y las 16 de #178 que se aplican. `hl7diff same` da las mismas 2 líneas de §12. El
barrido da 0 diferencias y 148 KNOWN. Las sondas de Fable de las rondas 2 a 4 pasan con el orden
nuevo, incluidos los reslices. La ronda 4 no converge en sentido estricto: dejó dos hallazgos medios,
ya corregidos. Se consulta al usuario si hace falta una 5.ª ronda.

## 17. Review, ronda 5 (aprobada por el usuario; 28-10-2026)

Acotada al nuevo orden de `baseElement`. Condición de corte acordada: si la ronda deja otro hallazgo
de la misma familia (resolución del elemento base), no se corrige en #180; se abre un issue aparte con
su test y su evidencia, y #180 se cierra con lo que ya cumple.

Resultado (evidencia en `evidence/2026-10-27-choice-key-case/review-r5/`, con los tests y su salida):

| Hallazgo | Estado |
|---|---|
| Codex (media): un elemento de un modelo lógico expandido por `contentReference` (`Timing.repeat.item.bounds[x]`, base `Timing.repeat.bounds[x]`) bajo un ancestro de tipo `Timing`: el `base.path` del paso 2 se busca en el `Timing` del núcleo antes que en la cadena propia, donde está el origen real. La entrada es dudosa (un elemento de tipo `Timing` con hijos que `Timing` no tiene) | **Confirmado** en las tres versiones; misma familia, va al issue aparte |
| Fable (baja): snapshot publicado en el que el slice `Patient.extension:e` no trae `type`; el recorrido de ancestros lo salta y no llega a `Extension` | **Confirmado** (R4, R5); misma familia, issue aparte. Con differential el generador copia el tipo y pasa |
| Fable (baja): `contentReference` a otro StructureDefinition (`U#U.grp`): la base no está en la cadena propia ni en el tipo de un ancestro | **Confirmado**; misma familia, issue aparte |

Fable no encontró ningún caso en que el orden elija la base equivocada con definiciones válidas, y su barrido
de R4 y R5 no encuentra en el núcleo elementos de elección bajo un ancestro de un solo tipo que no sea
BackboneElement/Element: los pasos 1 y 2 no compiten en los paquetes base. Ninguno de los tres casos
aparece en el barrido de ejemplos ni en `hl7diff`. Según la condición de corte, #180 pasa al cierre
(paso 9) y los tres casos se registran en un issue aparte, redactado en
`scratchpad/i180/issue-base-resolution.md`, que publica el usuario.
