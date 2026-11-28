# Un resourceType que no es string: error interno y mensaje (grupo 13 de la puerta de mhd)

Rama `fix/numeric-resource-type`, desde `999576e`. Origen: la puerta de adopción de mhd
(`evidence/2026-11-24-mhd-gate-divergences/`, grupo 13, `13-mut-type-007.json`). Sondas de este paso en
`evidence/2026-11-27-numeric-resource-type/probes/`.

**Responsabilidad.** Un validador decide si un JSON es un recurso FHIR válido. Es lo que compara la puerta contra
validator_cli, así que todo el incremento es de `gofhir/validator`. Nada de esto toca lo que hace un servidor (procesar una
transacción, asignar ids).

## 1. Revisión previa

- **Defecto.** Con el perfil de mhd, una entry cuyo `resource` tiene `"resourceType": 42` corta la validación entera:
  `internal error: the position index holds no scope for resource / (asked at ScopeOf)`. Sin perfil, gofhir informa
  `Missing 'resourceType' property` y sigue.
- **Causa.** Hay dos definiciones distintas de «es un recurso»:
  - `internal/scopeindex` (`holdIn`) indexa un mapa solo si su `resourceType` es un string.
  - `slicematch.valueScope` (`slicematch.go:487`) y `walker.within` (`path.go:413`) piden el Scope de todo mapa que
    tenga la clave, sea del tipo que sea.
  - Con el perfil, el discriminador `resource.conformsTo(...)` entra en la entry y pide un Scope que el índice no tiene.
  - Lo mismo hacen `validator.go:983` y `constraint/tree.go:488`, que deciden por la presencia de la clave.
- **HL7 7.0.4** (sondas `rt-*.json`):

  | Caso | Mensaje de HL7 |
  |---|---|
  | `resourceType` número u objeto | `Fatal`: «The JSON element 'resourceType' has the wrong JSON type: Number» (`Object`) |
  | `resourceType` ausente | `Fatal`: «Unable to find resourceType property» |
  | En una entry, además | `bdl-5`: «must be a resource unless there's a request or response» |
  | En la raíz | Solo el `Fatal` |
  | Con el perfil de mhd | El `Fatal` del tipo JSON (`hl7-13.txt`, línea 48) y la entry que no calza con ningún slice |

- **gofhir hoy:** `Missing 'resourceType' property` (error) en todos los casos, también cuando la clave está con otro
  tipo, y nunca `bdl-5`.
- **gofhir/server:** delega la validación en `validator/v2`. Nada que tomar.
- **Especificación:** R4 `json.html#resources`: un recurso se representa como un objeto JSON con la propiedad
  `resourceType`, que tiene el nombre del tipo. Un valor que no es string no es un nombre de tipo: no hay recurso.

## 2. Alcance (v4, reducido por decisión del usuario después de la ronda 3)

Este incremento solo quita el error interno y da el mensaje correcto. Cómo se eligen los slices y qué se evalúa con un
recurso inválido es el incremento siguiente (§2.6).

1. **Un predicado de sintaxis, en un solo lugar.** `slicematch.IsResource(m)`: un mapa cuyo `resourceType` es un
   string. Es lo que el índice ya usa (`holdIn`); el índice y `scopeindex.Resolve` lo importan (scopeindex ya importa
   slicematch).
2. **El punto de paso.** `scopeindex.Resolve` es por donde pasa todo pedido de Scope (`ScopeOf` y `ResolveScoped #id`,
   hallazgo A1). Si el mapa pedido no cumple `IsResource`, no es un recurso: no se registra un `MissingError` y
   `Resolve` devuelve `ok=false`.
   - `ScopeOf` devuelve el Scope de quien preguntó, sin cambios.
   - `ResolveScoped` no resuelve: una referencia a algo que no es un recurso no tiene destino.

   `MissingError` queda solo para un recurso sintáctico que el índice no tiene, que sí sería un defecto del validador.
3. **La resolución en un Bundle pasa por la misma regla** (B3). En `ResolveScoped`, una entry encontrada por `fullUrl`
   cuyo `resource` no cumple `IsResource` no resuelve (`ok=false`), igual que un `#id`.
4. **La sobre-indexación es inocua, y un test lo fija.** El índice puede tomar por recurso un elemento con un
   `resourceType` string (`ExampleScenario.instance`, `Subscription.filterBy`, `Consent.provision`). Solo importaría si
   debajo hubiera un elemento de tipo `Resource`, que el índice dejaría sin indexar. Un test recorre los snapshots de
   los core R4, R4B y R5 y falla si eso ocurre (medido en `plan-r2/snapshot-guard.txt`: no ocurre).
5. **El mensaje.** Si `resourceType` está pero no es un string (número, objeto, arreglo, booleano o `null`), se informa
   el texto de HL7, `RESOURCETYPE_PROPERTY_WRONG_TYPE`: «The JSON element 'resourceType' has the wrong JSON type:
   {Number|Object|Array|Boolean|Null}». Va con un diagnóstico propio, en la raíz y en los recursos anidados. Si falta, queda
   `Missing 'resourceType' property`; la diferencia de texto con HL7 («Unable to find resourceType property») queda
   registrada. La severidad queda en error; HL7 usa `Fatal`, y eso también queda registrado.
6. **Fuera de alcance, para el incremento siguiente: el recurso inválido.**
   - Descartarlo antes de evaluar invariantes, para que `bdl-5` falle como en HL7.
   - Que no se ajuste a ningún perfil de recurso (B1, C1) ni calce en un discriminador `type` (C2), medido contra HL7.
   - La decisión por posición en los consumidores (A2, B2, C3).
   - Los grupos 01, 02, 04 y 11 de mhd.

   No cambia veredictos: el error de estructura ya rechaza la instancia (A6). Con el ejemplo de mhd, la entry rota
   puede seguir calzando con un slice hasta ese incremento; queda registrado como divergencia conocida.

## 3. Criterio de salida

- El ejemplo 13 de mhd no da error interno, y aparece el mensaje del tipo JSON, como en HL7 (`hl7-13.txt`, línea 48).
- Ningún camino registra un `MissingError` por un mapa que no es un recurso. Lo prueba un test que cruza todos los tipos
  JSON de `resourceType` con todas las posiciones y los caminos que piden Scope.
- El mensaje coincide con HL7 para cada tipo JSON, en R4, R4B y R5.
- Datos reales (IPS, mCODE, US Core y core R4, R4B, R5) sin cambios.
- `make check` en verde.

## 4. Tests (esperado de HL7 7.0.4, salidas guardadas en `evidence/.../tests/`)

- **Tipos de `resourceType`:** número entero y decimal, objeto, arreglo, booleano, `null`, string vacío, string
  desconocido y ausente.
- **Posiciones:** la raíz, `Bundle.entry.resource`, `contained`, `Parameters.parameter.resource`, y dentro de un padre
  que también está roto.
- **Caminos que piden Scope**, con el destino roto:
  - un perfil que discrimina por `resource.conformsTo` o por `resource` como perfil;
  - `resolve()` a `#id`, a un `urn:uuid`, a un `fullUrl` absoluto y a una referencia relativa. Se verifica lo que
    devuelve el resolvedor.
- **Regresión de Scope:** `resolve()` a un Bundle anidado sigue con el Bundle como Container
  (`TestScopeInvariantSeesTheResolveChainQueries`).
- **Guarda de sobre-indexación** en R4, R4B y R5 (punto 4).
- **El caso de mhd:** el ejemplo 13 con `ihe.iti.mhd#4.2.5`.

## 5. Mutaciones (paso 7)

- Que `Resolve` vuelva a registrar un `MissingError` para un mapa que no es un recurso.
- Que `ResolveScoped` acepte una entry rota por `fullUrl`.
- Que el índice acepte cualquier tipo.
- Quitar el diagnóstico nuevo, o aplicarlo solo en la raíz.
- Nombrar mal el tipo JSON.
- Que la guarda no mire los descendientes.

## 6. Dependencias y ADR

- Sin dependencias nuevas.
- **ADR-005, la propiedad `resourceType` del formato JSON** (A6, NUCLEO §2.2). El nombre de la propiedad y la regla «un
  objeto JSON es un recurso si su `resourceType` es un string» vienen de la prosa del formato JSON (R4, R4B y R5,
  `json.html#resources`), no de un StructureDefinition. El ADR cita la sección y los tests.

## 7. Ronda adversarial 1 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-numeric-resource-type/plan-r1/`.

- **A1 (alta, aceptado):** `ResolveScoped #id` (`pkg/validator/slicematch.go:509`) también registraba un
  `MissingError`, con un `resolve()` a un contenido cuyo `resourceType` es numérico o falta. Confirmado con
  `resolve.out`. Lleva al punto de paso único (§2, punto 2).
- **A2 (media, aceptado):** un `resourceType` string no basta. `ExampleScenario.instance.resourceType` (R4, R4B) y
  `Subscription.filterBy.resourceType` (R5) son elementos de tipo de datos. Los consumidores deciden por posición y
  sintaxis (§2, punto 3), y un test fija que la sobre-indexación del índice es inocua (punto 4).
- **A3 (media, aceptado):** el inventario de sitios estaba incompleto. Se clasifica completo (§2, punto 3).
- **A4 (media, aceptado):** faltaban tests: destinos de referencia, elementos legítimos con `resourceType`, padres
  rotos, identidad del Scope, y salidas de HL7 para R4B y R5. Agregados en §4.
- **A5 (baja, aceptado):** la clave de HL7 y los nombres de los tipos. Además, `hl7-13.txt` sí trae el mensaje del tipo
  JSON (línea 48); se corrigió la revisión previa.
- **A6 (baja, aceptado):** postergar `bdl-5` no cambia veredictos. Falta un ADR para la regla del formato JSON (§6).

## 8. Ronda adversarial 2 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-numeric-resource-type/plan-r2/`. A1 queda resuelto: con el punto de paso, los
`MissingError` reproducidos desaparecen. El recorrido de los snapshots (`snapshot-guard.txt`) confirma la guarda del
punto 4: en R4, R4B y R5 ningún elemento con un `resourceType` string tiene debajo un elemento de tipo `Resource`.

- **B1 (media, aceptado):** el cambio de Scope no impide que un valor sin recurso se ajuste a un perfil. `Conforms`
  daba `true` para un `resourceType` 42, `null` o vacío (`v2-probe.out`). Lleva al punto 3b.
- **B2 (media, aceptado):** exigir `HoldsResource` en todos los sitios rompe la raíz de un `resolve()`, cuyo `Type` está
  vacío. `TestScopeInvariantSeesTheResolveChainQueries` falla dos veces (`position-root.out`). Lleva al punto 3: la raíz
  de un recurso cuenta como posición de recurso.
- **B3 (media, aceptado):** `ResolveScoped` devolvía `ok=true` para una entry rota alcanzada por `urn:uuid` o por
  `fullUrl` absoluto (`v2-probe.out`). Lleva al punto 3c.
- Se revisaron los siete llamadores directos de `Resolve`. Todos tratan bien `ok=false`, salvo `ScopeOf`, que ya
  tenía previsto el cambio en el punto 2.

## 9. Ronda adversarial 3 (Codex, 27-11-2026): no converge

Evidencia: `evidence/2026-11-27-numeric-resource-type/plan-r3/`.

Lo que v3 resuelve, según el modelo de Codex (`v3-model.diff`):
- Con el ejemplo de mhd, la entry rota no calza con ningún slice y no hay error interno.
- `TestScopeInvariantSeesTheResolveChainQueries` y las suites de los cuatro paquetes quedan en verde.
- La raíz de un recurso cuenta como posición, y la de un modelo lógico o un `HumanName` no.

Hallazgos pendientes:
- **C1 (media, confirmado):** con `resourceType: ""`, `IsResource` se cumple y `Conforms(Organization)` da `true`
  (`v3-final-probes.out`).
- **C2 (media, sospechado):** un discriminador `type` calza con un `resourceType` numérico o `null`, porque
  `walker.typeOf` usa el único tipo declarado y nunca llama a `Conforms`. No está medido contra HL7.
- **C3 (baja, confirmado):** `identify` también recibe valores sin pasar por la regla de posición
  (`typeChooser.Conforms`, con un `ExampleScenario.instance`). No cambia veredictos.
- **Aparte:** un discriminador `value` o `pattern` sobre `resource.resourceType` da `NoteCannotEvaluate` incluso con
  una Organization válida. Es un defecto previo y distinto, que queda para su propio incremento.

Se llegó al tope de rondas y queda la decisión del usuario.

**Decisión del usuario (27-11-2026):** achicar el alcance (§2, v4). C1, C2 y C3, junto con B1 y la decisión por
posición, pasan al incremento siguiente, el del recurso inválido, medidos contra HL7.

## 10. Valores esperados (paso 5, HL7 7.0.4)

`evidence/2026-11-27-numeric-resource-type/tests/`: 36 casos (9 formas de `resourceType` × raíz, entry, contained y
parameter), validados en 4.0.1, 4.3.0 y 5.0.0. `summarize.py` los resume en `expected.json`.

- El `Fatal` es igual en las tres versiones. Para número (entero o decimal), objeto, arreglo, booleano y `null`, el
  texto es «The JSON element 'resourceType' has the wrong JSON type: …» en las cuatro posiciones.
- Para un string vacío o desconocido, HL7 da otro texto: «Contained resource does not appear to be a FHIR resource
  (unknown name …)» en las posiciones anidadas, y «This content cannot be parsed…» en la raíz. Si la clave falta, da
  «Unable to find resourceType property». gofhir conserva sus mensajes: es una divergencia de texto registrada, sin
  cambio de veredicto.
- Las diferencias entre versiones están solo en `bdl-5`, `bdl-3a` (R5) e `inv-1` de Parameters, y en `Resource_RES_ID_Missing`
  y `dom-3` de los contenidos: todo eso es el recurso inválido (§2.6).
- **gofhir antes del cambio:** los 36 casos son inválidos en las tres versiones, sin error interno, siempre con
  `Missing 'resourceType' property`. El error interno aparece solo con un perfil que pide un Scope (el ejemplo de mhd y
  las sondas `resolve-*` de `plan-r1`).

## 11. Implementación y validación (pasos 5 a 7, 27-11-2026)

- **Tests primero** (subagente Sonnet): `pkg/validator/resource_type_json_test.go` (36 casos × 3 versiones contra
  `testdata/resource_type_json/expected.json` de HL7), `pkg/validator/resource_type_scope_test.go`,
  `internal/scopeindex/scopeindex_test.go` (`TestResolveOfANonResourceIsNotAFailure`) y
  `pkg/registry/resource_type_guard_test.go`. Antes del cambio fallan (`tests/red.txt`); después pasan (`tests/green.txt`).
- **Desvíos del plan:**
  - `ResolveScoped #id` devuelve `nil` además de `ok=false`.
  - No hay test con `resource.conformsTo(...)`, porque gofhir no tiene ese discriminador. Se usan un discriminador
    `profile` sobre `resource` y otro sobre `resolve()`.
  - La sonda `plan-r1/scopeprobe.go` se renombró a `.go.txt` para que `gofmt` no la mire.
- **`make check`** en verde (`GOTOOLCHAIN=go1.25.13`).
- **Datos reales** (`step7/real/*.rep`, `real.sh` de `2026-11-18-definition-layers-descent/step7/`, base `999576e`):
  sin informes nuevos ni perdidos, ni veredictos cambiados, en IPS, mCODE, US Core 6.1.0 y core R4, R4B y R5.
- **Ejemplo 13 de mhd** (`step7/mhd-13.head.json`):
  - No da error interno, e informa «The JSON element 'resourceType' has the wrong JSON type: Number», como HL7.
  - Siguen faltando los errores de slicing y del tipo de destino que da HL7: son del recurso inválido (§2.6).
- **Mutaciones** (`step7/mutations.py`, `step7/mut.out`): 10 de 10 muertas.
  - M1: `Resolve` registra un `MissingError` por un mapa que no es un recurso.
  - M2: `ResolveScoped` acepta una entry rota.
  - M3: el índice acepta cualquier tipo.
  - M4: sin el diagnóstico nuevo.
  - M5 y M5b: el diagnóstico solo en la raíz.
  - M6 y M6b: el tipo JSON mal nombrado.
  - M7: la guarda sin descendientes.
  - M8: `ScopeOf` no conserva el Scope de quien pregunta.

## 12. Code review (paso 8)

### Ronda 1 (Codex, 27-11-2026)

Evidencia: `evidence/2026-11-27-numeric-resource-type/code-review/r1/`.

- **CR1 (media, rechazado con evidencia):** HL7 da la expresión `$` al error de la raíz, y gofhir no da ninguna.
  `OperationOutcome.issue.expression` es FHIRPath (`operationoutcome-definitions.html`), y `$` no es FHIRPath: el
  propio HL7 lo rechaza al emitirlo («Internal error in location for message: 'Error @1, 2: Found $ expecting a valid
  token name'», `tests/hl7-4.0.1.log`). gofhir conserva la raíz sin expresión, como sus otros errores de estructura de
  la raíz. Lo que sí se acepta: el test lee la expresión de `expected.json` y convierte `$` en vacío de forma explícita,
  como divergencia documentada.
- **CR2 (media, aceptado):** `crValidator` guardaba en caché bajo `version+"+defs"` sin mirar qué definiciones trae.
  Si `TestContentReferencePrimitives*` corría antes, el test de Scope corría sin sus perfiles y no detectaba la
  mutación de `Resolve`. La clave pasa a depender del contenido de las definiciones, y el test comprueba que sus
  perfiles están cargados.

### Ronda 2 (Codex, 27-11-2026): converge

Evidencia: `code-review/r2/`.

- CR2 queda resuelto: la mutación de `Resolve` falla ahora en las tres versiones, aunque
  `TestContentReferencePrimitivesProfiles` corra antes.
- El test de la expresión es correcto: solo convierte `$` en vacío y toma toda otra expresión de `expected.json`.
- No quedan hallazgos. Antes de la ronda se corrieron todas las mutaciones: 10 de 10 muertas
  (`code-review/r1/mutations.out`).

## 13. Cierre (paso 9, 27-11-2026)

- `make check` en verde (`closing-make-check.txt`).
- ADR-005 escrito (`docs/adr/ADR-005-resourcetype-json-property.md`). El diagnóstico nuevo está en
  `error-reference/structural.md` y `structural.es.md`.
- **Qué quedó:**
  - Ningún pedido de Scope por un mapa cuyo `resourceType` no es un string aborta la validación.
  - `resolve()` no tiene destino en un recurso roto.
  - El mensaje del tipo JSON es el de HL7, en las cuatro posiciones y en las tres versiones.
  - Los datos reales no cambian.
- **Divergencias registradas con HL7:**
  - La severidad es error, no `Fatal`.
  - La raíz no lleva expresión, en vez de `$`.
  - Con un string vacío o desconocido, o con la clave ausente, el texto es otro.
- **Para el incremento siguiente, el recurso inválido (§2.6):**
  - `bdl-5`, `bdl-3a` e `inv-1`.
  - Que la entry rota no calce en ningún slice (B1, C1, C2), con el destino de tipo `Resource` que da HL7.
  - La decisión por posición (A2, B2, C3).
  - `Resource_RES_ID_Missing` y `dom-3` en los contenidos.
  - Los grupos 01, 02, 04 y 11 de mhd.
