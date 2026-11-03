# ADR-003: un array vacío de un primitivo o de su `_x` se reporta según la versión de json.html

## Estado

Aceptado (03-11-2026), con la decisión del usuario sobre D2 de
[docs/plans/2026-11-02-empty-array.md](../plans/2026-11-02-empty-array.md) (#190). Lo verifican:
- `TestEmptyArray` (`pkg/validator/empty_array_test.go`), con las sondas `ea_*` en 4.0.1, 4.3.0 y 5.0.0;
- `TestEmptyArrayMetamorphic`, sobre los ejemplos oficiales de las tres versiones;
- `TestEmptyPrimitiveArraysForbidden` (`pkg/structural/empty_array_test.go`), la lista por release;
- los grupos `empty-array-probes*` de hl7diff, con las divergencias declaradas.

## Contexto

El formato JSON de FHIR no está en ningún `StructureDefinition`: está en la prosa de json.html, y esa prosa cambió
entre versiones.

- **DSTU2, STU3, R4 y R4B**, json.html («Comparison with XML»): «Just as in XML, JSON objects and arrays are never
  empty, and properties never have null values (except for a special case documented below). Omit a property if it
  is empty». DSTU1 no tiene la frase.
- **R5** (y el build de R6 del 02-11-2026): esa frase ya no está. json.html §2.1.6.4.2 dice: «If the length of a
  JSON "element" array is different from the length of its JSON "_element" array, implementations SHALL infer null
  values in the suffix of the shorter array. If an array has only null elements, implementations SHOULD omit it
  entirely».
  - Un `_x` vacío es, por lo tanto, uno más corto que sus valores: se infieren nulls.
  - Un array de puros nulls (y uno vacío lo es) se SHOULD omitir. Eso es una recomendación, no un error.

**HL7 validator_cli 6.10.2** (y `master` de `org.hl7.fhir.core` al 02-11-2026):
- reporta `ARRAY_CANNOT_BE_EMPTY` solo en `JsonParser.parseChildComplex`, en todas las versiones;
- no lo revisa en `parseChildPrimitive`, así que nunca reporta `given: []` ni `_given: []`.

Evidencia en `docs/plans/evidence/2026-11-02-empty-array/probe/`.

## Decisión

- **Un array vacío de un elemento complejo o de un recurso que se repite** es un error en todas las versiones
  (`STRUCTURE_ARRAY_EMPTY`). En DSTU2, STU3, R4 y R4B lo dice el texto. En R5 coincide con HL7 y con «FHIR elements
  can never be empty» (formats.html).
- **Un array vacío de un primitivo que se repite (`given: []`) o de su `_x` (`_given: []`)** es un error solo en
  las versiones cuyo json.html dice «arrays are never empty»: DSTU2 (1.0), STU3 (3.0), R4 (4.0) y R4B (4.3). En R5
  y después, no se reporta.

Las versiones son datos con su fuente, como las erratas de `pkg/registry/errata.go`: una lista de releases y la
cita que los respalda. Se comparan por release (`versionorder.SameRelease`), contra la versión FHIR que se valida. Una
versión que no está en la lista, o desconocida, sigue la regla de R5: es el texto vigente.

En R4 y R4B, la diferencia con HL7 en primitivos y `_x` se declara en
`testdata/m12-slice-scoping/declared-divergences.json`. Además se redacta un issue para `hapifhir/org.hl7.fhir.core`
sobre `parseChildPrimitive`, acotado a las versiones cuyo texto lo exige.

## Consecuencias

- Una misma instancia con `given: []` es inválida en R4 y válida en R5, como dice cada especificación.
- Si una versión futura vuelve a decir que los arrays nunca están vacíos, hay que agregarla a la lista.
- La lista tiene solo releases publicados. Los ballots 1.4 (2016May), 1.8 (2017Jan), 3.3 (2018May) y 3.5 (2018Sep)
  también tienen la frase (verificado con `curl` el 03-11-2026), pero gofhir no trae sus cores: validados con uno de
  ellos, siguen la regla de R5. Si se llegan a soportar, se agregan.
- La verificación son los tests del plan, con la tabla de sondas en las tres versiones.
