# #193, pieza 3: perfiles de tipo en un valor primitivo

Rama `refactor/primitive-type-profiles`, desde `a69c03d` (pieza 2, PR #218).

## 1. Revisión previa (15-11-2026)

Notas completas: [evidence/2026-11-15-primitive-type-profiles/prior-review.md](evidence/2026-11-15-primitive-type-profiles/prior-review.md).

- **Especificación** (R4, R4B y R5, `ElementDefinition.type.profile`): «If any profiles are specified, then the content
  must conform to at least one of them». No hace excepción con los primitivos.
- **Qué pasa hoy:** un primitivo no recibe ningún `type.profile`, tenga 0, 1, 2 o más perfiles. Lo midió un test en una
  copia, con 17 casos en R4:
  - con un perfil (constraint en la raíz, `maxLength`, `fixed`, `string.extension` con min 1) no informa nada;
  - un perfil sin resolver no da `TYPE_PROFILE_NOT_FOUND`;
  - p14, p15, p16 y s22 no informan nada.
- **Dónde quedan fuera los primitivos:**
  - `registry/type_profile.go:122` e `instance_children.go:280`, por `IsPrimitiveTypeCode`;
  - `validator/slicematch.go:238` y `:262`: `Conforms` exige un map, y el memo usa su puntero (C4);
  - las guardas `Kind != primitive-type` en `constraint/tree.go` (líneas 84, 187 y 243) y en `cardinality.go:236`.

  Solo `walkPrimitiveExtensions` usa el perfil, y solo para los hijos de `_x`.
- **HL7 7.0.4** da lo mismo en R4, R4B y R5:

  | Sonda | Resultado de HL7 | Ubicación |
  |---|---|---|
  | p14 | warning `MultipleMatches` | `Observation.value.ofType(string)` |
  | p15 | error `NoMatch`, más `short-1` y `upper-1` | `Observation.value.ofType(string)` |
  | p16 | nada | — |
  | s22 | error `SLICING_CANNOT_BE_EVALUATED` | `Patient.name[0].given[0]` |
- **`gofhir/server`** no tiene lógica propia para esto: delega en el validador (v2.1.0). No hay nada que tomar.
- **Alcance:** entra también el caso de un solo perfil. Dos o más perfiles sin uno sería incoherente, y hoy ese caso no
  cumple la especificación. La regla del repo es que la especificación gana sobre el comportamiento actual. Antes del
  plan se mide con HL7 qué hace con un perfil.

### Sondas de HL7 (15-11-2026)

Están en [evidence/2026-11-15-primitive-type-profiles/probe/](evidence/2026-11-15-primitive-type-profiles/probe/README.md): 147
instancias por versión con HL7 7.0.4, en R4, R4B y R5, sin errores de carga.
- **Un perfil:** HL7 lo aplica.
  - **Raíz del perfil:** sus constraints se evalúan sobre el valor. El mensaje es `<perfil>#<clave>` y sale en la
    ubicación del valor.
  - **`string.id` y `string.extension`:** se evalúan sobre `_x`. Dan `Validation_VAL_Profile_Minimum`,
    `Validation_VAL_Profile_Minimum_SLICE` y las constraints de `id`.
- **`<tipo>.value`:** HL7 no aplica nada de lo que el perfil declara ahí: `maxLength`, `fixed`, `pattern`, `minValue`,
  un binding ni una constraint. Tampoco lo usa para elegir entre perfiles.
- **Perfil sin resolver:** `Validation_VAL_Unknown_Profile`. Si el otro perfil cumple, es lo único que informa. Si el
  otro no cumple, informa además `NoMatch` y la constraint del que no cumple.
- **Dos perfiles:** cuando cumplen los dos, informa `MultipleMatches`; cuando cumple uno, nada; cuando no cumple
  ninguno, `NoMatch` y las constraints de cada perfil. En un arreglo elige elemento por elemento, en `given[i]`.
- **Valor ausente** (solo `_x`, o `_x` más largo que el arreglo): la raíz se evalúa igual, y un resultado vacío no
  cumple.
- **Misma clave en dos perfiles:** cuando la expresión es la misma, informa una sola línea. Cuando la expresión es
  otra, una línea por perfil. Es la regla de la pieza 2.
- **Tipo de elección con varios tipos:** HL7 aplica solo el primer perfil. Es la divergencia p12, ya declarada. Un slice
  por tipo sí elige.
- **`%resource` y `%rootResource`:** son el recurso contenido y la raíz.
- **Versiones:** no hay diferencias, salvo el texto de `ele-1` en R4B para un valor sin valor ni extensión.

**Decisión del usuario (15-11-2026):** las restricciones de `<tipo>.value` siguen la especificación, pero en una pieza
aparte, la 3b, inmediatamente después de esta. Irán como divergencia declarada, con su issue para HL7. La pieza 3
cubre lo que HL7 permite verificar.

## 2. Plan

### Alcance

A un valor primitivo con uno o más perfiles de tipo se le aplican las mismas reglas que a un tipo de datos (piezas 1
y 2):
1. **Capas del primitivo.** `typeProfilesOf` y `MemberLayers` dejan de excluir los primitivos
   (`type_profile.go:122`, `instance_children.go:280`). Con eso, un primitivo recibe `Governing`, `Alternatives` y
   `Unresolved`, igual que un tipo de datos.
2. **Lo que se aplica de cada capa:**
   - las constraints de la raíz del perfil, evaluadas sobre el valor (`$this` es el valor, y uno ausente da vacío,
     que no cumple);
   - los hijos `id` y `extension` del perfil, evaluados sobre `_x`: cardinalidad, slicing de extensiones, constraints
     y extensiones (incluidos sus contextos).

   No se aplica `<tipo>.value`; queda para la pieza 3b.
3. **Elección entre dos o más perfiles:** `Conforms` acepta un primitivo en modo profileOnly, con lo mismo del punto 2.
   Informa `PROFILE_NO_MATCH`, `PROFILE_MULTIPLE_MATCHES` y `TYPE_PROFILE_NOT_FOUND`, igual que en la pieza 2.
4. **Identidad de un primitivo:** un escalar no tiene puntero, así que la clave del memo y de la deduplicación pasa a
   ser su posición: el map dueño, la propiedad y el índice (C4). Un valor que solo tiene `_x` usa la misma posición.
5. **Slicing:** un slice de primitivos con un perfil que no se resuelve informa `SLICING_CANNOT_BE_EVALUATED` (s22). Un
   slice por tipo elige entre los perfiles.
6. **Sin perfiles no cambia nada:** con cero perfiles el camino es el de hoy, y se mide con un bench.

### Cambio arquitectónico (explicado antes de implementar)

`registry.Instance{Value map[string]any, Self, Owner}` solo representa objetos. Se le agrega la forma primitiva:
- el escalar;
- el map de `_x`;
- la posición (dueño, propiedad, índice).

Con eso, `ChooseTypeProfiles`, `ProfileChooser.Conforms` y el memo trabajan con primitivos sin una segunda API. El
alternativo era armar un map sintético para el primitivo, como hace hoy la cardinalidad. Se descarta porque una
constraint de la raíz (`length() <= 3`) tiene que correr sobre el escalar, no sobre un objeto.

En las fases:
- `constraint/tree.go` levanta la guarda `Kind != primitive-type` (líneas 84 y 187) solo para las capas de perfil. La
  raíz del tipo base de un primitivo sigue sin evaluarse, porque `ele-1` sobre un valor la cubre `checkValueless`.
- `walkPrimitiveExtensions` recorre todas las capas.
- La cardinalidad de `id` y `extension` (`cardinality.validatePrimitiveElement`) lee los hijos de las capas en vez de
  `GetByType`.
- Las extensiones de `_x` (`extension.go:237`) recorren las capas.

### Fuera del alcance

- `<tipo>.value`: `maxLength`, `fixed`, `pattern`, `minValue` y `maxValue`, binding y constraints. Es la pieza 3b.
- La fase de formato de primitivos (`primitive.go`) no cambia.
- Incremento 2 (s11 y s12) y s16.

### Criterio de salida

- Las 147 sondas por versión y p14, p15, p16 y s22 coinciden con HL7 en R4, R4B y R5, salvo p12 (el tipo de elección
  con varios tipos), que es una divergencia declarada.
- hl7diff contra `main`: los grupos livianos y `r4-core-examples` sin hallazgos nuevos. Si un ejemplo del core usa
  perfiles en primitivos, cada diferencia se clasifica con evidencia.
- Mutaciones: todas muertas, o equivalentes con una prueba medida.
- Bench contra `main`: con cero o un perfil, sin regresión de tiempo; asignaciones +1 % como máximo.
- `make check` pasa.

### Tests y datos

- **Valores esperados:** salen de HL7. Un generador los toma de las salidas de las sondas (`gen_expected.py`, como en la
  pieza 2), sin correr el código propio.
- **Casos de las sondas:**
  - un perfil (raíz, `id`, `extension`, slice obligatorio);
  - sin resolver;
  - dos perfiles: cumplen los dos, uno o ninguno;
  - arreglos;
  - valor ausente y `_x` más largo;
  - misma clave;
  - extensión con `valueString`;
  - slice por tipo;
  - `%resource` y `%rootResource` dentro de un contenido.
- **Metamórficos:** reordenar las claves, y mover `_x` antes o después del valor, dan el mismo resultado.
- **Las tres versiones desde el primer test.**

### Excepciones a la regla 5 y dependencias

Ninguna excepción prevista y ninguna dependencia nueva.

## 3. Ronda adversarial 1 (Codex, 15-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| A1 | El foco de la raíz no puede ser el escalar pelado. `constraint.go:649–674` ya maneja un primitivo con su `_x`. Una expresión con `extension`, `id`, `hasValue()` o `%context` necesita ese dato | alta | **Aceptado.** El foco es el primitivo tipado con su `_x`, como en `constraint.go`. Se agregan sondas con esas expresiones, con el escalar y sin él |
| A2 | La posición necesita un dueño inmediato, distinto del `Owner` (el recurso, que da el Scope). Con `Owner`, `name[0].given[0]` choca con `name[1].given[0]` | alta | **Aceptado.** La identidad es el map padre inmediato, la propiedad y el índice. El `Owner` sigue dando el Scope. Se testean escalares iguales bajo padres distintos y en recursos distintos |
| A3 | El slicing no baja a un primitivo: `slicing/tree.go:239` y `slicing.go:184` solo siguen maps | alta | **Aceptado, verificado en el código.** Se suman los dos caminos, con tests de slices de extensión obligatorios y prohibidos bajo el perfil de un primitivo, incluido un miembro sin valor en un arreglo |
| A4 | La pieza 3b cambia la elección, no solo la validación. En la pieza 3, un perfil que solo restringe `.value` cumple siempre | media | **Aceptado.** Queda registrado como límite temporal de la pieza 3. Se agregan casos pareados: perfiles que solo difieren en `.value`, y perfiles que mezclan `.value` con `id` o `extension`. Hoy los esperados son los de HL7, y la 3b los cambia junto con la elección, el memo, los diagnósticos y la capa que gobierna |
| A5 | Saltear la raíz del tipo base de todo primitivo es una excepción a la regla 5 por clase | media | **Aceptado.** En R4 la raíz de cada primitivo base tiene solo `ele-1` (medido en `hl7.fhir.r4.core`). Se quita la guarda entera: se evalúan las raíces de todas las capas, la del tipo base incluida. El `ele-1` de la raíz y el de `checkValueless` se juntan por la identidad de la pieza 2 (clave, expresión, severidad y ubicación). Se mide con hl7diff, y también en R4B y R5 |
| A6 | El criterio de salida exceptúa solo p12, pero la misma divergencia está en `c5-xc2*` y `c5-oc2*`, también en orden invertido | media | **Aceptado.** El plan enumera esas sondas, con el valor esperado de la especificación. La normalización de ubicación y mensaje, y el `ele-1` doble de R4B, se declaran caso por caso, sin perder severidad, índice ni la identidad de la constraint |

## 2 v2. Plan (después de la ronda 1)

Lo que cambia respecto del §2:
1. **Foco:** el foco de las constraints de la raíz es el primitivo tipado con su `_x` (A1).
2. **Identidad:** el map padre inmediato, la propiedad y el índice. El `Owner` solo da el Scope (A2).
3. **Slicing:** baja a los primitivos en `slicing/tree.go:239` y `slicing.go:184` (A3).
4. **Raíces:** sin guarda por clase. Se evalúan las raíces de todas las capas, y los `ele-1` se juntan por identidad
   (A5).
5. **Límite temporal:** un perfil que solo restringe `.value` cumple siempre hasta la pieza 3b, y los casos pareados lo
   fijan (A4).
6. **Divergencias declaradas:** p12, `c5-xc2*` y `c5-oc2*`, en los dos órdenes, con valor esperado de la
   especificación (A6).
7. **Sondas nuevas antes de los tests:**
   - raíz con `extension`, `id`, `hasValue()` y `%context`;
   - escalares iguales bajo padres distintos;
   - slices de extensión obligatorios y prohibidos bajo el perfil de un primitivo;
   - perfiles pareados por `.value`.

## 3b. Ronda adversarial 2 (Codex, 15-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| B1 | La identidad de A5 no es una sola. `pendingConstraints` (`tree.go:724`) y `firstReport` (`report_scope.go:95`) usan clave y expresión, sin severidad. `layers.go:31` suma la severidad. En R4B, el `ele-1` de la raíz de `string` agrega `or $this is Parameters` y otro texto, mientras que el de `HumanName.given` conserva la expresión vieja | media | **Aceptado.** Una sola identidad en la evaluación, el informe y la unión con `checkValueless`: clave, expresión y severidad. En R4B las dos expresiones de `ele-1` son distintas, así que salen dos líneas, que es lo que HL7 informa en R4B (`c3-vl4`). En R4 y R5 sale una. Se agrega un test con severidades distintas. El criterio «cero perfiles no cambia nada» se precisa así: solo cambia el `ele-1` doble de R4B, que HL7 también informa |
| B2 | A2 da la tupla pero no dice cómo llega. `slicematch.Request`, el `state` y el `end` de `path.go`, y `profileConforms` solo pasan `e.value`. Un primitivo que se alcanza por `resolve()` o en un contenido, o que se comprueba en frío, no tiene ni la identidad ni su `_x` | alta | **Aceptado.** El descriptor del primitivo (escalar, `_x`, padre, propiedad e índice) viaja por las solicitudes, los estados, los extremos y las llamadas a `Conforms`. El memo exige la identidad canónica antes de buscar. Se testea en frío contra en caliente, con escalares iguales y `_x` distinto |
| B3 | Otra compuerta del slicing: `slicing/tree.go:78–90` saltea un escalar sin `_x` si `governedChildren` no expone hijos, así que el slice obligatorio de un perfil del primitivo no se alcanza | alta | **Aceptado.** Se toman las capas elegidas del primitivo antes de decidir si se baja, y se recorre un `_x` vacío cuando haga falta. Test: un escalar sin `_x` cuyo perfil exige un slice de extensión |

## 2 v3. Plan (después de la ronda 2)

Sobre la v2:
1. **Identidad única de una constraint:** clave, expresión y severidad, en la evaluación, el informe y la unión con
   `checkValueless` (B1).
2. **«Cero perfiles no cambia nada»,** salvo el `ele-1` doble de R4B, que coincide con HL7 (B1).
3. **El descriptor del primitivo viaja por todo el camino del slicing y de `Conforms`** (B2).
4. **El slicing baja a un primitivo según sus capas, aunque no tenga `_x`** (B3).

## 3c. Ronda adversarial 3, la última (Codex, 15-11-2026)

B2 y B3 quedan resueltos. Queda un hallazgo medio:

| # | Hallazgo | Sev. | Resolución propuesta |
|---|---|---|---|
| C1 | Sumar la severidad a `pendingConstraints` (`tree.go:708–716`, con `seen` compartido entre slice y elemento) y a `firstReport` (`report_scope.go:86–100`) cambia el comportamiento sin perfiles de primitivo. Por ejemplo, un slice que sube un warning heredado a error pasaría a informar las dos constraints. Así, la garantía de la v3 es falsa | media | **Aceptado, achicando B1.** No se toca ninguna identidad existente. `pendingConstraints` y `firstReport` siguen con clave y expresión, y entre capas sigue la regla de la pieza 2 (`layers.go`: clave, expresión y severidad). Alcanza con eso: el `ele-1` de la raíz del tipo base y el de `checkValueless` tienen la misma severidad. En R4 y R5 la expresión también es la misma, así que salen juntos; en R4B es otra, así que salen dos, como en HL7. El test de severidades distintas sale del plan |

Por la regla de tres rondas, se consulta al usuario.

**Decisión del usuario (15-11-2026):** se aprueba el plan v3 con C1 resuelto: no se toca ninguna identidad existente. No
hay cuarta ronda.

## 5. Tests (paso 5, 15-11-2026)

Los tests se escribieron antes del código, sin tocar nada de producción.

### Sondas nuevas (plan v2, punto 7)

[probe/gen.py](evidence/2026-11-15-primitive-type-profiles/probe/gen.py) pasó de 147 a 346 instancias por versión, medidas con HL7 7.0.4 en R4,
R4B y R5, sin errores de carga ni de snapshot (README y tablas actualizados):
- **Caso 7, raíz con `extension`, `id`, `hasValue()` y `%context`:** ps-hx, ps-rx, ps-ri, ps-rc y ps-rcx, con escalar, con escalar y `_x`,
  sin valor y largo, en Observation y en `given`, más tres pares de perfiles.
- **Caso 8, identidad por posición:** escalares iguales en `name[0].given[0]` y `name[1].given[0]`, en `given[0]` y `given[1]`, y en dos Patient de un
  Bundle, con `_x` distinto, en cuatro combinaciones y con uno o dos perfiles.
- **Caso 9, slices de extensión:** ps-sub (obligatorio) y ps-nosub (`max 0`) sin `_x`, con la extensión del slice, con otra y con ambas; miembros sin valor
  de un arreglo y `_given` más largo que `given`; y ps-sub con ps-nosub y con ps-len.
- **Caso 10 (A4):** cuatro pares que difieren solo en `.value`, y ps-mixe y ps-mixi (`.value` con `extension` o `id`), solos y con ps-len o ps-up.
- **Caso 11:** ocho guardas sin perfiles (valor, `_x`, valor ausente y `_given` más largo), con el `ele-1` doble de R4B.

Hallazgos con HL7 (README, 11 a 16): `%context` es el valor y ve su `_x`; la elección es siempre por posición; un perfil que solo restringe `.value`
cumple siempre, y mezclado con `extension` o `id` cuenta solo por estos; la única diferencia entre versiones sigue siendo el `ele-1` doble de R4B.

### Tests

[pkg/validator/primitive_type_profiles_test.go](../../pkg/validator/primitive_type_profiles_test.go), con `testdata/primitive_type_profiles/` (350 instancias, 135 archivos de perfiles
y expected.json, que genera [tests/gen_expected.py](evidence/2026-11-15-primitive-type-profiles/tests/gen_expected.py) desde las salidas de HL7 con
[tests/mk_testdata.sh](evidence/2026-11-15-primitive-type-profiles/tests/mk_testdata.sh)). Se reusan `classifyLayers`, `matchLayer` y `compareLayers` de la pieza 2,
y `classifyLayers` suma la clase `slicing-cannot-be-evaluated`. p14 y p15 salieron de `piece3Probes` (la lista y su test se quitaron); p16 sigue también en
`TestLayersProfileChoice`.

| Test | Qué cubre | Hoy pasan / fallan |
|---|---|---|
| `TestPrimitiveOneProfile` | c1: raíz, `id`, `extension`, slice, y lo de `.value` | 156 / 36 |
| `TestPrimitiveUnresolvedProfile` | c2 y s22 | 0 / 27 |
| `TestPrimitiveTwoProfiles` | c3, c4, p14, p15, p16 | 36 / 105 |
| `TestPrimitiveChoiceWithOneProfileAndSliceByType` | c5 con un perfil y slice por tipo | 15 / 15 |
| `TestPrimitiveDeclaredDivergences` | A6 | 9 / 18 |
| `TestPrimitiveResourceVariablesInAContainedResource` | c6 | 21 / 15 |
| `TestPrimitiveRootConstraintFocus` | c7 (A1) | 117 / 75 |
| `TestPrimitiveIdentityIsThePosition` | c8 (A2, B2) | 18 / 102 |
| `TestPrimitiveExtensionSlicesUnderTheProfile` | c9 (A3, B3) | 87 / 36 |
| `TestPrimitiveValueOnlyProfilesAreTheTemporaryLimit` | c10 (A4) | 63 / 75 |
| `TestPrimitiveZeroProfileGuard` | c11 en R4 y R5, y en R4B sin el ele-1 doble | 20 / 0 |
| `TestPrimitiveZeroProfileR4BDoubleEle1`, `...ValuelessEle1Count` | el `ele-1` doble de R4B (B1, C1) | 2 / 5 |
| `TestPrimitiveKeyOrderAndUnderscoreOrderDoNotMatter` | metamórfico: claves al revés, `_x` antes y después del valor, en todas las sondas con `_x` | 0 / 3 |
| `TestPrimitiveColdAndWarmMemoGiveTheSameResult` | en frío contra en caliente, escalares iguales con `_x` distinto, en los dos órdenes | 0 / 15 |
| `TestPrimitiveSameInstanceTwiceGivesTheSameResult`, `...ChoiceDoesNotDependOnTheProfileOrder` | idempotencia, y A6 sin depender del orden | 21 / 0 |

### Antes de la implementación

- **Fallan 527 de 1092 subtests** (14 de 17 tests de primer nivel, los de la pieza); ningún fallo trae un informe de más de gofhir: todos son lo que falta.
  Pasan 565: lo que HL7 no informa y hoy tampoco (`.value`, casos válidos) y las guardas sin perfiles.
- Las guardas sin perfiles pasan: 20 de 20. El `ele-1` doble de R4B no es una guarda: hoy gofhir da una línea y el plan lo cambia a dos, como HL7.
- Salidas en el scratchpad `/Volumes/dev/tmp/claude/claude-501/-Users-robertoaraneda-projects-personal-opensource-go-validator/11d0cc65-98da-4af5-bcc8-65c4743d06bd/scratchpad/p3-tests/`.
- La suite compila (`go vet ./pkg/validator/` sin hallazgos) y `TestLayers*` y `TestChildrenPrecedence*` siguen pasando. Corrida de los tests nuevos: unos 5 s; el pico es el de
  cargar un validador por versión, igual que en la pieza 2 (unos 650 MB), más 80 MB de los validadores en frío.

### Divergencias declaradas (valor esperado de la especificación)

- **A6:** `c5-xc2-{both,none,len}`, `c5-oc2-*` y `c5-oc2r-*`, en los dos órdenes de los perfiles. Un tipo de elección con varios tipos usa todos los perfiles
  del tipo del valor («the content must conform to at least one of them»), y el esperado es el de la sonda del mismo valor con los mismos perfiles en un
  elemento de un solo tipo (`c3-*-ext` y `c3-*-obs`). HL7 aplica solo el primero (`InstanceValidator.java:8072` y `:8085`).
- Sin más excepciones: la ubicación de `.ofType(string)` se normaliza como en la pieza 2 (`valueString`), y el `ele-1` doble de R4B se compara como lo da HL7.

## 6. Implementación (paso 6, 15-11-2026)

Los tests del paso 5 pasan sin tocar sus esperados ni `testdata`: `TestPrimitive*`, `TestLayers*` y `TestChildrenPrecedence*`
(5,5 s), el paquete `pkg/validator` y los paquetes afectados. `make check` pasa (salida en el scratchpad `p3-impl/make-check.txt`).

### Diseño como se construyó

- **Registro.** `registry.PrimitiveValue` (escalar, `_x`, map padre, propiedad, índice, tipo) y `Position` (clave comparable).
  `Instance.Primitive` es un valor, no un puntero, para no asignar en el camino sin perfiles. `typeProfilesOf` y `MemberLayers`
  ya no excluyen primitivos. `ChooseTypeProfiles` decide con un primitivo. `Candidates` y `AllUnresolved` cuentan una vez un
  perfil declarado dos veces (HL7: `c4-same-ok` no da `MultipleMatches`, `c2-obs4` da un solo `TYPE_PROFILE_NOT_FOUND`).
  `DeclaresTypeProfile` es la compuerta barata del slicing.
- **Conformidad.** `conformer.Conforms` recibe un `PrimitiveValue`; la clave del memo es `pos` (padre, propiedad, índice) y el
  Scope sigue siendo el del dueño. `checkPrimitive` corre `validateAgainstProfile` sobre el `_x` (un objeto vacío si no hay)
  con la definición del perfil, y la fase de constraints evalúa la raíz del perfil sobre el escalar con su `_x`
  (`ValidateOptions.Primitive`, `walkPrimitiveProfile`). `<tipo>.value` no se aplica (pieza 3b).
- **Constraints.** `walk` lleva el primitivo; `typeRoot` evalúa la raíz de cada capa de tipo, la del tipo base incluida, con
  `primitiveFocus` (escalar más `_x`). Las identidades no cambian (`pendingConstraints` y `firstReport` por clave y expresión; entre capas,
  `layers.go`). `walkPrimitiveExtensions` recorre todas las capas, `valuelessType` evalúa la raíz de un valor sin valor (el `ele-1` doble
  de R4B sale de aquí).
- **Cardinalidad.** `validatePrimitiveElement` toma los hijos de `ChildrenOf` con el primitivo (capas, perfil sin resolver).
- **Extension, binding, reference.** Pasan el primitivo cuando bajan por un `_x`.
- **Slicing.** `slicematch.Request.Primitive`, `state.prim` y `end.prim` llevan el descriptor hasta `profileConforms`. Slicing baja a un
  primitivo por sus capas aunque no tenga `_x`, `checkMember` y `checkChildren` incluidos. s22: un slice que declara para un primitivo
  un perfil sin resolver da `NoteCannotEvaluate` (`unresolvedPrimitiveProfile`).

### Desviaciones y notas

- `checkValueless` y `walk` reciben el primitivo; se ajustó la llamada de `valueless_choice_slice_test.go` (solo la firma).
- `validatePrimitiveElement` toma los hijos con `ChildrenOf`, que también sigue la cadena `SliceOf`: un slice de un primitivo sin hijos
  propios hereda los del elemento que divide (antes caía al tipo base). Lo pasa toda la suite.
- `checkChildren` de slicing con un primitivo anidado no tiene test propio (los 350 casos no lo alcanzan).
- El escalar de una comprobación de conformidad se serializa con `json.Marshal` (un decimal `1.50` se vería como `1.5`).
- Bench contra HEAD (`p3-impl/benchstat.txt`): tiempo ~ en Patient y Batch, +2,9 % en Observation (p=0,04); asignaciones +0,4 a +0,85 %.

## 7. Mutaciones (15-11-2026)

42 mutantes a mano en una copia (`scratchpad/p3mut`, worktree desprendido en HEAD + los cambios sin confirmar, verificado con cmp). Orden: T1 `go test ./pkg/validator -run 'TestPrimitive|TestLayers' -failfast` (4 a 6 s), T2 el paquete tocado, T3 `./pkg/validator` completo (35 s). No se corrió `./...`.

**23 muertos, 19 sobreviven.** Todos los muertos los mató T1. Crudo: `mut-results-run1.tsv` (corrida 1; sus «KILLED T2» eran falsos, ruta de paquete mal escrita), `mut-run2.log`, `mut-run3.log`.

### Muertos (por T1)
M01 TwoProfiles; M03 IdentityIsThePosition; M05 UnresolvedProfile; M07 TwoProfiles; M08 UnresolvedProfile; M09 TwoProfiles; M11 UnresolvedProfile; M13 TwoProfiles; M14 RootConstraintFocus; M15a OneProfile; M17 TwoProfiles; M19 OneProfile; M22 ExtensionSlicesUnderTheProfile; M27 UnresolvedProfile; M28a TwoProfiles; M28b UnresolvedProfile; M29a TwoProfiles; M29b TwoProfiles; M30 UnresolvedProfile; M31 UnresolvedProfile; M32 TwoProfiles; M33 OneProfile; M38 UnresolvedProfile.

### Sobrevivientes
Cobertura (`cover-full.out`, `-coverpkg`, pkg/validator completo): todas las líneas mutadas se ejecutan salvo `slicematch.go:516` (`value = e.prim`), con cero ejecuciones.

| Id | Mutación | Clase | Entrada que lo mata |
|---|---|---|---|
| M26, M23, M24, M25, M37 | descriptor del primitivo no llega a `profileConforms` (`e.prim` no se usa; `Request.Primitive` sin poner; `state.prim`; `next.prim`; `governing`) | hueco de test: la rama `slicematch.go:516` nunca corre. Ningún caso tiene un slice con discriminador `type=profile` sobre un primitivo con perfil que se resuelve | `Patient.name.given` partido con discriminador `{type: profile, path: $this}` y dos slices, cada uno con `type.profile` distinto (uno con `extension` min 1); `given` con y sin `_given`; esperado de HL7 |
| M04 | `Parent` de la posición = recurso, no el map padre (path.go) | hueco, mismo camino | igual, con dos `name` en el mismo Patient y `family` igual y `_family` distinto, discriminador `type=profile` en `name.family` |
| M02 | `Key` fuera de la `Position` | hueco | `HumanName` con `family` y `text` (índice -1, mismo padre), ambos con los mismos dos `type.profile`; `_family` con la extensión y `text` sin ella: la clave choca y `text` hereda la respuesta de `family` |
| M06 | guarda `IsPrimitiveTypeCode` de `MemberLayers` restaurada | hueco | slice `given:s` con `type.profile` de dos perfiles, uno que exige un slice de extensión; `given` que cumple solo el otro |
| M10 | sin comprobación `TypeCode != profile.Type` en `checkPrimitive` | hueco, mismo camino que M26 | discriminador `type=profile` con un perfil de tipo `code` sobre un `string` |
| M12 | `walkPrimitiveProfile` no recorre los hijos `id` y `extension` | hueco: solo se alcanza en `Conforms` (dos o más perfiles) | dos perfiles; uno con una constraint de error sobre `string.extension` o `string.id`; `_x` que la viola. Esperado: ese perfil no cumple (NoMatch o elige el otro) |
| M16 | `walkPrimitiveExtensions` solo recorre la primera capa | hueco | dos perfiles que cumplen, el segundo con una constraint de nivel warning sobre `string.extension`; `_x` con una extensión que la viola: el warning debe salir |
| M18a, M18b | la fase de extensiones recorre `_x` sin el primitivo (sin elegir capas) | hueco | dos perfiles, solo uno cumple; el otro declara un slice de extensión con una Extension de contexto inválido o un valor que falla: no debe informarse bajo el perfil no elegido |
| M35, M36 | igual en binding y en reference | hueco, mismo caso que M18 | el perfil no elegido con un binding `required` sobre `valueCode` de su extensión, o un `targetProfile` de `valueReference`; no debe haber error del perfil descartado |
| M20 | `checkChildren` de slicing no baja a un primitivo con perfil | hueco, el plan ya lo declara | `Patient.name` con un perfil de HumanName cuyo `given` tiene `type.profile` con un slice obligatorio; `given` sin extensión: error esperado |
| M21 | `checkMember` no baja a un primitivo | hueco | slice `given:first` con `type.profile` que exige un slice de extensión; `given[0]` sin `_given` |
| M15b | no se evalúa la raíz del tipo base de un primitivo con valor | equivalente (razonado): esa raíz solo tiene `ele-1`, que con valor siempre cumple (`hasValue()`); sin valor lo hace `valuelessType` (M29b muerto) | ninguna; no existe entrada |
| M34 | sin `DropRepeated` entre capas en `validatePrimitiveElement` | hueco probable: un error de cardinalidad que dos perfiles no elegidos informan igual | dos perfiles que exigen la misma extensión (min 1), `_x` vacío: NoMatch y un solo `Validation_VAL_Profile_Minimum` por clave, no dos |

Resumen: 18 huecos de test (en 8 grupos de entrada), 1 equivalente.

Detalle y salidas: scratchpad `p3-impl/mutations.md`, `p3-impl/mut.py`.

**hl7diff contra `main` (15-11-2026):**
- grupos livianos: 61, sin hallazgos;
- `r4-core-examples`: 5.301 archivos, sin hallazgos ni errores sin explicar.

Las divergencias declaradas son las mismas que con la pieza 2. Las salidas están en el scratchpad, `p3-impl/hl7diff-*.txt`.

### Huecos cerrados (15-11-2026)

Para cada hueco se agregó un grupo de sondas HL7 (`c12-*`, 177 instancias nuevas en `probe/gen.py`; HL7 7.0.4, R4, R4B y R5, consolas sin errores de carga) y `TestPrimitiveMutationGaps` las compara con gofhir en las tres versiones, sin cambios de código de producción y sin un solo hallazgo contra gofhir: todas las sondas coinciden con HL7. Cambios de los ayudantes de prueba: `gen_expected.py` clasifica `Validation_VAL_Profile_NotSlice`, `Terminology_TX_*`, `Reference_REF_BadTargetType` y `Extension_PROF_Type`; `classifyLayers` clasifica `REFERENCE_INVALID_TARGET` y deja pasar el aviso `xw-1`.

Prueba de muerte: copia `scratchpad/p3mut2` (worktree desprendido en HEAD más lo no confirmado, verificado con cmp, retirado al terminar), cada mutación de `p3-impl/mut.py`, T1 `go test ./pkg/validator/ -run 'TestPrimitive|TestLayers' -failfast -count=1`. Crudo: `p3-gaps/mut2-final.log`.

| Mutante | Estado | Entrada que lo mata |
|---|---|---|
| M23, M24, M26, M10 | muertos | `c12-pgp-*` (type=profile en `given`, con un perfil de `code` sobre un `string` para M10) |
| M25, M04 | muertos | `c12-pnf-*` (`Patient.name` partido por type=profile en la ruta `family`; dos `name` iguales con `_family` distinto). Los `c12-pgp-*` no los alcanzan: M25 y M04 viven en el paso de ruta |
| M37 | muerto | `c12-pgp-con-*` (constraint de un slice elegido por type=profile) |
| M02 | muerto | `c12-pft-*` (`family` y `text`; el differential va en el orden del tipo base, HL7 descarta `family` si va antes de `text`) |
| M12, M16 | muertos | `c12-idc-up-*`, `c12-extc-up-*` (el hijo `id` o `extension` de un perfil de dos) y `c12-len-extw-*`, `c12-extw-len-*` (aviso del segundo perfil) |
| M34 | muerto | `c12-ext1x2-*` |
| M35, M36 | muertos | `c12-xs-bind-*`, `c12-xs-ref-*` (la referencia, a un contenido: HL7 no informa el tipo de destino de una referencia sin resolver) |
| M21 | muerto | `c12-pgsx*` (un slice de `given` con un hijo propio, `extension` min 1, y perfil de tipo) |
| M06, M20 | siguen vivos: equivalentes por lo observado | M20: `slicing.go:190` se ejecuta 228 veces en las sondas, y el recorrido de `slicing/tree.go` informa lo mismo por otro camino (sondas `c12-pnh*`, `c12-pnhc*`, `c12-pnhs*`, con slicing cerrado para aislar la fase). M06: `MemberLayers` solo da capas de los perfiles que el valor cumple, y un perfil que cumple no tiene qué informar la fase de slicing; con ninguno que cumpla cae a `MemberTree` igual que el mutante (`c12-pgsd*`, `c12-pgsx2`). No hay entrada que los distinga |
| M18a, M18b | siguen vivos: equivalentes por lo observado | la fase de extensiones sin elegir capas informa lo mismo: un perfil que se elige no tiene errores, y el que no cumple los informa la mezcla de capas (`c12-xs-*`, `c12-xstr-*`: valor, binding, reference, contexto y tipo, con uno, dos que cumplen uno y dos que no cumplen). HL7 tampoco comprueba el contexto del perfil de un slice (ps-xctx no da error) |
| M15b | equivalente (razonado) | la raíz del tipo base de un primitivo solo trae `ele-1`, que con valor siempre cumple (`hasValue()`), y sin valor lo hace `valuelessType` (M29b muerto). No existe entrada |

Resumen: de los 18 huecos, 14 mutantes mueren (M02, M04, M10, M12, M16, M21, M23, M24, M25, M26, M34, M35, M36, M37) y quedan M06, M18a, M18b y M20, más M15b, sin entrada que los mate tras tres rondas de entradas; el motivo está arriba. `make check` en `p3-impl/make-check-2.txt`.

**Bench contra la pieza 2 (15-11-2026, `-count=8`, `p3-impl/bench2-*.txt`):**

| | Tiempo | Bytes | Asignaciones |
|---|---|---|---|
| Patient | +2,44 % | +1,09 % | +0,85 % |
| Observation | sin cambio | +0,86 % | +0,74 % |
| Batch | +0,79 % | +0,38 % | +0,37 % |

Patient se pasa del criterio en tiempo y apenas en bytes. Esos ejemplos no tienen perfiles en primitivos, así que el
costo lo agrega el camino de cero perfiles. Va a la ronda 1 del code review.

## 8. Code review

### Ronda 1 (Codex, 15-11-2026)

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| R1 | **La precisión del decimal cambia la elección.** `constraint.go:280` vuelve a serializar el escalar decodificado. Con `valueDecimal: 1.50` y un perfil cuya raíz exige `precision() = 2`, el valor cumple cuando se valida solo, pero si se agrega una alternativa da `PROFILE_NO_MATCH`. Lo mismo pasa con `toString() = '1.50'` | alta | **Aceptado.** Hay que tomar el valor exacto del JSON original, a partir del padre, la propiedad y el índice (`exactOf`), o llevarlo en el descriptor. Test con una sonda de HL7 |
| R2 | **Los hijos desplegados ocultan los perfiles sin resolver.** `instance_children.go:216` sale antes de juntar los perfiles sin resolver. Con `Patient.name.given.extension` desplegado, `TYPE_PROFILE_NOT_FOUND` desaparece | media | **Aceptado.** Los perfiles sin resolver se juntan aparte de la precedencia de las capas, también en las salidas tempranas. Test con una sonda de HL7, y se revisa si también afecta a un tipo de datos |
| R3 | **Trabajo evitable sin perfiles.** `cardinality.go:244` arma el descriptor y llama a `ChildrenOf` antes del atajo de siempre | media | **Aceptado.** Se vuelve a poner un atajo temprano que sale de la definición: sin hijos, sin contentReference, sin slices y sin perfiles declarados. Se mide con el bench |
| — | M06, M18a, M18b y M20 siguen vivos. Su equivalencia no está probada | — | Siguen como equivalentes razonados; se vuelven a atacar en la ronda 2. M15b queda confirmado en las tres versiones |

### Correcciones de la ronda 1 (15-11-2026)

Cada corrección tiene su test, que falla sin ella (se comprobó revirtiéndola), con el valor esperado de HL7 7.0.4 (R4, R4B, R5; `-tx n/a`; las consolas sin errores de carga, solo el `Can't find ... profile` esperado). Sondas nuevas: caso 13 de `probe/gen.py` (c13-*, 26 instancias); test `TestPrimitiveReviewRound1`, y `TestExactScalar` en `pkg/constraint`.

- **R1.** `walkPrimitiveProfile` toma el escalar de su gemelo exacto (`exactScalar`: el del padre por propiedad e índice, `opts.exact`) y no vuelve a serializar el decodificado. Sondas c13-dec-*: `valueDecimal` 1.50 con `precision() = 2` y con `toString()`, con un perfil y con dos en los dos órdenes, y 1.5 y 1.500 como contraste. Fallaban `p2-pair`, `p2-pairr`, `p2-pair3`, `ts-pair`, `ts-pairr` y `ts-pair3` (PROFILE_NO_MATCH). No hay sonda de un arreglo de decimales (ningún elemento de arreglo de R4 con un decimal admite un perfil de tipo en una extensión usable); el índice se prueba con `TestExactScalar`.
- **R2.** `ChildrenOf` junta los perfiles sin resolver también cuando el nodo tiene hijos desplegados (`withUnresolved`, solo si el nodo declara un perfil de tipo). Afecta también a un tipo de datos (c13-pnu-*: `Patient.name` con un perfil de HumanName que no se resuelve y un hijo propio): HL7 informa el perfil desconocido en los dos casos, así que se corrigió para ambos. El test de la pieza 1 `own children come before an unresolved profile` pedía lo contrario sin evidencia de HL7; ahora espera el perfil sin resolver.
  - **Divergencia declarada A7.** Con el perfil sin resolver, HL7 no aplica el `min` de los hijos que el perfil de la instancia despliega (c13-pgu-extmin-none, c13-pnu-fam-giv); sin perfil de tipo sí lo aplica (controles c13-pgc-extmin-none y c13-pnc-fam-giv). La especificación lo pide siempre; el esperado suma el `min` del control (gen_expected.py, `SPEC_CHILDREN`).
- **R3.** `validatePrimitiveElement` vuelve a salir antes cuando la definición no deja nada más que el tipo (`onlyTheTypeGoverns`: sin hijos, contentReference, slices, SliceOf ni perfil de tipo declarado) y el valor no tiene hermano `_x`. Medido (count=8, uno tras otro, contra el árbol de la pieza 2): asignaciones +0,37 % a +0,85 % (≤ 1 %); tiempo +2,1 % a +3,2 % (`p3-impl/bench3.txt`). El perfil de CPU muestra que el aumento que queda es `typeRoot` (+0,13 s de 5,3 s): las constraints de la raíz de un primitivo (`ele-1`) que ahora se evalúan, que es el comportamiento de la pieza (plan C1), no el descriptor de cardinalidad (0,03 s en total).

**Verificación (Opus, 15-11-2026):**
- `make check-3` pasa.
- El test de la pieza 1 que se cambió sigue exigiendo que los hijos propios gobiernen. Además ahora exige el informe del
  perfil sin resolver, como hace HL7 (sondas c13).
- Antes de la ronda 2 queda abierto el tiempo. El bench da +2,1 % a +3,2 % de tiempo; las asignaciones, +0,37 % a
  +0,85 %. Viene de evaluar la raíz del tipo base (`ele-1`) en cada primitivo, que es lo que pidió A5.
- **Regla rota:** en las sondas c13 se corrieron 3 procesos Java a la vez, y el máximo es 2.

### Ronda 2 (Codex, 15-11-2026)

R1, R2 y R3 están bien. Los decimales se probaron en tipos de elección, Parameters anidados, recursos contenidos,
entradas de Bundle y destinos de `resolve()`. A7 está justificada por `ElementDefinition.min`. M06, M18a, M18b y M20
siguen vivos aun comparando todos los diagnósticos de los 609 casos c12 y c13.

| # | Hallazgo | Sev. | Resolución |
|---|---|---|---|
| S1 | **Un warning del dueño calla un error del perfil del primitivo.** `Patient.name.given` declara el warning `sev-1: false`, y su perfil de `string` declara el error `sev-1: false`. Al agregar el warning, ErrorCount pasa de 1 a 0, porque `pendingConstraints` (`tree.go:813`) y `firstReport` no usan la severidad | alta | **Contradice C1**, que el usuario aprobó: no tocar ninguna identidad. Antes de decidir se mide con HL7: el caso del primitivo, el de un tipo de datos y el de un slice que sube un warning heredado a error, sin perfiles |
| S2 | **El costo no es la evaluación de la raíz.** De los 160 ms de `typeRoot`, 140 son de la rama de no primitivos; la evaluación de constraints de primitivos no aparece en el perfil. Lo que sobra es armar la identidad de cada constraint concatenando cadenas (`tree.go:813`), 15 asignaciones y 960 B por validación. Si se saltea esa raíz, Patient baja 1,6 % | media | **Aceptado.** Se calculan las identidades una sola vez por definición, que no cambia. Se mantienen la regla 5 y todas las constraints de la raíz |

### Ronda 2: evidencia de S1 y corrección de S2 (15-11-2026)

**S1, evidencia (sin cambio de producción).** Caso 14 de las sondas (`probe/gen.py`, 26 instancias; tabla en `probe/README.md`, «Caso 14»; HL7
7.0.4 y gofhir dan lo mismo en R4, R4B y R5; consolas sin errores de carga). Aún no están en `expected.json`.
- **(a) primitivo y (b) tipo de datos:** con la misma clave, HL7 informa UNA vez, la del perfil de tipo, con la severidad del perfil, sea más alta o más baja que la del dueño
  (`a1`/`a2`/`b1`/`b2`), y también con otra expresión (`a3`/`b3`). gofhir hoy informa la del DUEÑO y calla la del perfil de tipo
  (`a1`/`b1`: el warning del dueño calla el error del perfil, 0 errores contra 1 de HL7; `a2`/`b2`: sale el error del dueño donde HL7 da el warning del perfil);
  con otra expresión informa las dos. Coincide en `a4`–`a6`, `b4`–`b6`.
- **(c) sin perfiles de tipo (slice o elemento que redeclara) y (d) un perfil de recurso y su base:** HL7 informa la constraint de la BASE con la severidad de la base; la severidad
  que sube o baja la derivada se ignora (`c1`–`c6`, `d1`–`d4`, también con otra expresión). gofhir hoy coincide en las 14.
- **Consecuencia:** sumar la severidad a la identidad no acerca (a)/(b) a HL7 (informaría las dos, HL7 una) y rompería (c)/(d), que hoy coinciden. Lo que HL7 hace en (a)/(b)
  es una precedencia por clave, el perfil de tipo sobre el dueño. Decisión pendiente del usuario; C1 sigue vigente (ninguna identidad existente se toca).

**S2, corrección.** La identidad de una invariante deja de armarse como cadena (`clave + "\x00" + expresión`) por cada valor: es un valor comparable
`constraintID{key, expr}` (`tree.go`), con `identityOf(c)` como único lugar que la define (sumar la severidad después es un campo y una asignación). El `seen` de la caminata
pasa a `seenSet` (`map[constraintID]bool`), y el scope de informe a `reportKey{at, id, other}` (`report_scope.go`; `firstReportKey` usa `other`). Semántica intacta: clave y
expresión, sin severidad. Se descartó guardar cadenas precalculadas en el `ElementNode` (estado en un árbol del registro y el mismo costo de hash) y un `sync.Map` por puntero (se
fugaría con los árboles que el registro suelta).
Medido (`p3-impl/bench4.txt`, count=8, contra `p3-impl/head`, el árbol de la pieza 2): tiempo geomean +1,1 % (Patient +2,1 %, Observation sin diferencia, Batch +1,9 %), B/op -2,1 %,
allocs/op -2,1 % (el mismo árbol antes de S2 estaba +2,7 % sobre head en `bench3`; el reloj de la máquina derivó ~5 % entre corridas). Tests: `pkg/constraint` y `pkg/validator` completos en verde;
`make check` en `p3-impl/make-check-4.txt`.

**Decisión del usuario sobre S1 (16-11-2026): seguir a HL7.**
- Cuando el elemento y el perfil de su tipo declaran la misma clave, gana la constraint del perfil del tipo, con su
  severidad y su expresión. Vale para primitivos y para tipos de datos.
- Un slice o un perfil derivado que vuelve a declarar la clave sigue como hoy: gana la base, como en HL7.
- No se toca ninguna identidad existente, así que C1 se mantiene.
- Los valores esperados salen de c14, en R4, R4B y R5.

### Corrección de S1 (16-11-2026)

**Diseño.** Precedencia por clave del perfil de tipo sobre el dueño, sin tocar ninguna identidad (`constraintID{key, expr}` y `layers.go` siguen igual).
En `walk` (`pkg/constraint/tree.go`) ahora se resuelven primero, sin evaluar, los dueños (el slice, los que divide y los de su `contentReference`) y las capas
de tipo (`alternativeTypes`, `typeLayerOf`). `shadowedByType` toma las claves que declaran las capas cuya definición es un perfil (`derivation = constraint`; la
definición base del tipo no cuenta) y marca como ya evaluadas (`seen`) las constraints de los dueños con esa clave, aunque la expresión difiera. Luego se evalúan los
dueños, y antes de cada capa de tipo `releaseShadowed` suelta las marcas de las identidades que esa capa declara, así que la capa evalúa e informa la suya con su severidad y
su expresión. Con varias capas (`walkLayers`) la suelta cada capa sobre su copia de `seen`. Un slice o un perfil derivado que vuelve a declarar la clave no son capas de tipo, así que gana la base
como antes (`c14-c*`, `c14-d*` siguen en verde).
**Excepción medida:** si el dueño y la capa de tipo comparten la `Source` (la misma constraint heredada de una definición, el `ele-1` de Element), no se cubre: en R4B sus
expresiones difieren y HL7 informa las dos (`single2`, `ele1` de `TestLayers*` fallaban sin la excepción; `declaredByAnother`).
**Tests.** `gen_expected.py` suma `hl7-c14-<v>-7.0.4.json` (26 instancias; `expected.json` pasa a 579) y `TestTypeProfileConstraintPrecedence` corre `c14-*` en R4, R4B y R5; `classifyLayers` ya no ignora el warning `sev-1`.
Antes del código fallaban exactamente `a1`–`a3` y `b1`–`b3` (6 de 26, en las tres versiones); ahora pasan las 26. `pkg/constraint` y `pkg/validator` completos en verde (`p3-impl/full-5.txt`).
**No cubierto (sin evidencia de HL7):** el primitivo sin valor (`checkValueless`) no aplica la precedencia; el caso 14 no lo mide.

### Mutaciones antes de la ronda 3 (16-11-2026)

59 mutantes en una copia (`p3-impl/mutations-r3.md`, `mut-results-r3.tsv`): los 42 anteriores readaptados y 17 nuevos para las correcciones desde entonces (R1 `exactScalar`, R2 `withUnresolved`, R3 `onlyTheTypeGoverns`, S1 `shadowedByType`/`releaseShadowed`/`declaredByAnother`, A7). Duración: 951 s. **50 muertos, 9 sobreviven.** Conocidos: M06, M15b (equivalente), M18a, M18b, M20. Nuevos: R3a (equivalente: solo el atajo de rendimiento), S1b (probablemente equivalente: la raíz base solo declara `ele-1`, de la misma `Source`), S1g (hueco: `walkLayers` sin `releaseShadowed`, falta un caso con dos perfiles de tipo que repiten la clave del dueño) y S1i (hueco: solo el primer dueño se sombrea, falta un caso con la clave repetida en un slice o en el contentReference).

**c15 encontró un defecto en la corrección de S1 (16-11-2026).** Son 18 instancias de HL7, en R4, R4B y R5. HL7 no
deja que el perfil del tipo reemplace la clave del dueño en dos casos:
1. Cuando se aplican dos o más perfiles de tipo (g1–g5 y h1–h3). Ahí informa la constraint del dueño.
2. Cuando la clave está en el elemento que divide un slice (i1–i4). Ahí informa la de la base y la del perfil.

gofhir callaba la del dueño en los dos casos: 42 fallos, de 14 instancias en tres versiones. Se corrige siguiendo la
decisión del usuario de seguir a HL7: el perfil del tipo solo reemplaza la clave cuando hay un único perfil de tipo y
el dueño es el elemento propio del valor.

### Corrección de c15 (16-11-2026)

Antes del código fallaban 42 de las instancias de c15 (14 instancias en R4, R4B y R5; `p3-impl/c15-before.txt`). Ahora pasan todas, junto con c14.
- `shadowedByType` ya no cubre al dueño con dos o más perfiles de tipo declarados (se cuentan los candidatos de `Alternatives`, no los elegidos: g2 elige uno solo porque el otro no conforma, y HL7 igual informa
  la del dueño). Con eso `walkLayers` no necesita `releaseShadowed` ni el parámetro `shadowed`.
- Solo cubre la constraint del dueño propio (`owners[0]`) y no la de un elemento dentro de un slice (`inSlice`: `SliceOf` o un id con `:`): la snapshot copia en él la constraint del elemento que el slice divide, y
  la copia no se distingue de una propia. Para esos dueños (y los demás) no se marca: se suelta la identidad antes de la capa de tipo, que informa la suya.
- `reportKey` suma la severidad: HL7 informa la misma clave y expresión con dos severidades como dos fallos (i1–i4); sin ella el segundo se perdía (13 fallos al quitarla).
- Mutantes (`p3-impl/mut-results-6.txt`, copia `p3mut5`, 11 mutantes): el hueco S1g ya no existe (el código que probaba se quitó) y se prueba con G1 (quitar `nProfiles > 1`) y G3, muertos; S1i con I1, I2, I3 (la rama de
  slice y de otro dueño), muertos, y S1c, S1d, K muertos. Sobreviven G2 (la guarda `len(layers) != 1` es redundante con `nProfiles > 1` cuando hay capas), S1b (equivalente, como antes) e I5 (la rama `i > 0`: sin
  evidencia de HL7 para un dueño que no sea el propio ni un slice, p. ej. un contentReference; queda sin cubrir).

**Excepción a C1, aprobada por el usuario (16-11-2026).** La severidad en `reportKey` cambia la identidad con que se deduplican los reportes de constraints, que C1 dejaba fija. Se acepta con esta evidencia:
- los 13 casos i1–i4 de c15 coinciden con HL7 solo con la severidad;
- hl7diff contra `main` da 0 hallazgos en los 61 grupos livianos (`p3-impl/hl7diff-light-3.txt`) y en los 5.301 ejemplos del core R4 (`p3-impl/hl7diff-core-4.txt`);
- c14(c), el caso de escalamiento en un slice, sigue en verde.

La primera corrida del core (`hl7diff-core-3.txt`) se trabó 3,5 h sin salida, con hl7diff consumiendo CPU y `corpusrun` bloqueado escribiendo: se mató. La segunda terminó en 34 s.

### Ronda 3 (Codex, 16-11-2026): no converge

Codex no pudo escribir en la copia: el sandbox le negó la escritura. Por eso dejó dos hipótesis medias sin probar.
Rehizo en memoria las 1.791 expectativas de `expected.json` desde las salidas de HL7 y no encontró diferencias. Las
dos hipótesis se verificaron con la sonda c16: HL7 7.0.4, R4, 12 instancias. Los archivos están en
`scratchpad/c16/`, generados con `gen16.py`; gofhir se corrió con un test temporal en `p3rev`.

- **T1 (medio, confirmado): un primitivo sin valor no aplica la precedencia del perfil de tipo.** El problema está
  en `checkValueless` y `valuelessType` de `tree.go`.
  - En c16-v1 a v6 se repiten los casos c14-a1 a a6 con `given:[null]` y `_given` con una extensión. HL7 da lo mismo
    que con valor: informa solo la constraint del perfil de tipo.
  - gofhir informa la del dueño y calla la del perfil en v1 (warning en vez de error) y en v2 (error en vez de
    warning). En v3 informa las dos.
- **T2 (medio, confirmado): un perfil no resuelto no cuenta como perfil declarado.** El problema está en
  `shadowedByType(owners, len(tps.Alternatives.Candidates()), …)` de `tree.go:84`: `Candidates()` deja fuera el
  perfil no resuelto.
  - Instancias: s16-u1, u2 y u3, con `[s14-pe|s14-pw, s16-missing]` y en los dos órdenes, con y sin valor.
  - HL7 informa siempre la constraint del dueño y también la del perfil, como con dos perfiles.
  - gofhir calla la del dueño en u1, u2 y u3 con valor. Sin valor, en u1v y u2v, calla la del perfil (es T1).

### Corrección de la ronda 3 (16-11-2026)

El usuario aprobó corregir T1 y T2 y cerrar. La sonda c16 pasó a la evidencia del repo (`probe/gen.py`, caso 16; HL7 7.0.4 en las tres versiones, sin errores de carga, las tres iguales entre sí y R4 igual a la
sonda original). Sus 12 instancias entraron a `expected.json` y a `TestTypeProfileConstraintPrecedence` (regex `c1[456]-.+`, después `c1[4-7]-.+`).

- **Antes de corregir:** 27 subtests de c16 fallaban (v1..v3 y u1, u1v, u2, u2v, u3, u3v, en las tres versiones).
- **T1 (`pkg/constraint/tree.go`):** `checkValueless` resuelve ahora la definición de tipo antes de evaluar a los dueños y aplica `shadowedByType` igual que `walk`. `valuelessType` recibe lo resuelto y suelta
  (`releaseShadowed`) las marcas en la capa de tipo.
- **T2 (`pkg/registry/type_profile.go`, `tree.go`):** `Alternatives.DeclaredCount()` cuenta los perfiles declarados distintos, resueltos o no. `shadowedByType` lo usa en lugar de `len(Candidates())`. Con dos o más
  declarados, el dueño propio evalúa su constraint y la capa que queda también: `shadowedByType` devuelve las claves del dueño sin marcarlas y `releaseShadowed` las suelta. HL7 informa las dos aunque tengan la misma clave y expresión.
- **Después:** los 36 subtests de c16 en verde y las suites dirigidas y `./pkg/...` en verde (ver `p3-impl/make-check-7.txt`).

**Verificación de la corrección (Opus, 16-11-2026).**
- **Rama quitada.** La corrección traía una rama sin evidencia: con dos o más perfiles declarados, la constraint de
  un dueño que no es el propio (un slice) no se soltaba. Quitarla no hacía fallar ningún test. La sonda c17 la midió:
  - `s17-k1` a `k3`: la clave está en el elemento que el slice divide, y el slice declara
    `[s14-pe|s14-pw, s16-missing]` en los dos órdenes;
  - 6 instancias de HL7 7.0.4 en R4, R4B y R5 (`hl7-c17-<v>-7.0.4.json`), sin errores de carga;
  - HL7 informa las dos constraints, la del dueño y la del perfil. Con la rama, gofhir callaba la del perfil en
    3 instancias; sin ella, coincide.
  - La rama se quitó. c17 entró a `expected.json` (615 instancias), y la regex del test pasó a `c1[4-7]-.+`.
- **Mutaciones** (copia `p3rev`; `p3-impl/mut-r3fix.txt` y `mut-r3fix2.txt`): 8 de 10 muertas.
  - Muertas: B1 (volver a poner la rama), T2a, T2b y T2c (contar solo los resueltos, o no usar el conteo), T1a2
    (sin precedencia en el camino sin valor) y T1b (sin `releaseShadowed`).
  - Vive T2d (`DeclaredCount` sin deduplicar): solo cambia algo si un tipo declara dos veces el mismo canónico, y
    no hay sonda de HL7 para eso.
  - Vive T1c2 (`checkType` en todos los dueños sin valor): es equivalente por razonamiento. `checkType` solo actúa
    en un elemento `[x]`, y el destino de un contentReference es un elemento de backbone.
  - T1a y T1c, en su primera versión, no compilaban y no cuentan.
- **`DeclaredCount`** ya no asigna memoria, porque se llama por cada valor.
- **hl7diff contra `main`:** 61 grupos livianos en PASS (`p3-impl/hl7diff-light-4.txt`) y 5.301 ejemplos del core
  R4 con 0 hallazgos (`p3-impl/hl7diff-core-5.txt`).
- **Bench contra la pieza 2** (`p3-impl/bench7.txt`, corridas intercaladas, 8 por lado): el tiempo no cambia
  (geomean +0,8 %, no significativo). B/op baja 1,5 % y allocs/op baja 1,0 %.
  - Las corridas en orden fijo (`bench5.txt` y `bench6.txt`) daban +3 a +4 % en los dos órdenes. Con las corridas
    intercaladas la diferencia desaparece, así que era un sesgo de medición.
