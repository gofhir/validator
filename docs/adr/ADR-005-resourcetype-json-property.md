# ADR-005: la propiedad `resourceType` del formato JSON define qué objeto es un recurso

## Estado

Aceptado (27-11-2026), con el plan [docs/plans/2026-11-27-numeric-resource-type.md](../plans/2026-11-27-numeric-resource-type.md)
(grupo 13 de la puerta de adopción de mhd). Lo verifican:
- `TestResourceTypeJSONType` (`pkg/validator/resource_type_json_test.go`): 36 casos de
  `pkg/validator/testdata/resource_type_json/` (número entero y decimal, objeto, arreglo, booleano, `null`, vacío,
  desconocido y ausente, en la raíz, una entry, un contenido y un `Parameters.parameter.resource`) en 4.0.1, 4.3.0 y
  5.0.0, contra los textos de HL7 7.0.4 de `expected.json`;
- `TestBrokenResourceTypeNeverAbortsAScopeQuery`, `TestResolveScopedBrokenTarget` y
  `TestScopeOfNonResourceKeepsTheCallersScope` (`pkg/validator/resource_type_scope_test.go`): ningún camino que pide un
  Scope aborta con «internal error» por un destino roto;
- `TestResolveOfANonResourceIsNotAFailure` (`internal/scopeindex/scopeindex_test.go`);
- la guarda `TestResourceTypeKeyNeverHasAResourceBelow` y su prueba `TestResourceTypeGuardSeesAResourceBelow`
  (`pkg/registry/resource_type_guard_test.go`), sobre los core R4, R4B y R5.

## Contexto

Ni el nombre de la propiedad `resourceType` ni la regla «este objeto JSON es un recurso» están en un
`StructureDefinition`: están en la prosa del formato JSON. R4, R4B y R5, json.html#resources: un recurso se representa
como un objeto JSON con la propiedad `resourceType`, cuyo valor es el nombre del tipo.

El validador tenía dos definiciones de «es un recurso»:
- `internal/scopeindex` indexaba un mapa solo si su `resourceType` era un string;
- `slicematch` (`valueScope`, `walker.within`) pedía el Scope de todo mapa que tuviera la clave, fuera del tipo que
  fuera.

Con `"resourceType": 42`, el segundo pedía un Scope que el primero no tenía, y la validación entera terminaba en
`internal error: the position index holds no scope for resource`. HL7 7.0.4 informa en ese caso «The JSON element
'resourceType' has the wrong JSON type: Number» (`Object`, `Array`, `Boolean`, `Null`), y sigue.

## Decisión

- **Un objeto JSON es un recurso si su `resourceType` es un string** (`slicematch.IsResource`). Es la única definición
  sintáctica; el índice de posiciones y quien le pide un Scope la comparten. Que el nombre sea o no un tipo conocido
  es otra pregunta (`STRUCTURE_UNKNOWN_RESOURCE`).
- **`scopeindex.Resolve` de algo que no es un recurso es `ok=false` sin registrar un fallo.** `MissingError` queda para
  un recurso sintáctico que el índice no tiene, que sí es un defecto del validador. `ScopeOf` devuelve entonces el Scope
  de quien preguntó, y `ResolveScoped` no resuelve: una referencia a algo que no es un recurso no tiene destino
  (`#id`, `urn:uuid` y `fullUrl` absoluto).
- **`resourceType` presente y que no es un string** se informa con `STRUCTURE_RESOURCE_TYPE_WRONG_JSON_TYPE`, con el
  texto de HL7 y el nombre del tipo JSON (`Number`, `Object`, `Array`, `Boolean`, `Null`), en la raíz y en los
  recursos anidados. Ausente, vacío o desconocido conserva su mensaje.
- **La sobre-indexación es inocua, y una guarda la fija.** Con una regla sintáctica el índice también toma por recurso
  un elemento con un hijo `resourceType` que no es un recurso (`ExampleScenario.instance` en R4 y R4B,
  `Subscription.filterBy` y `Consent.provision` en R5). Solo importaría si debajo hubiera un elemento de tipo
  `Resource`, que el índice dejaría sin indexar. La guarda recorre los snapshots core de las tres versiones y falla si
  eso ocurre; hoy no ocurre.

## Consecuencias

- Divergencias con HL7 que quedan registradas: la propiedad ausente (HL7: «Unable to find resourceType property»), vacía
  y desconocida conservan el texto de gofhir; la severidad es `error` y no `fatal`.
- Fuera de este ADR, para el incremento del recurso inválido: `bdl-5` en una entry rota, que un valor sin recurso no se
  ajuste a un perfil de recurso ni calce en un discriminador `type`, y la decisión por posición en los consumidores
  (plan §2.6).
- Si una versión futura agrega un elemento con un hijo `resourceType` bajo el que haya un recurso, la guarda falla y
  este ADR se revisa.
