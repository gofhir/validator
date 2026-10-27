# ADR-001: el generador de snapshots nombra ElementDefinition una sola vez

## Estado

Aceptado (26-10-2026). Lo verifica `TestElementDefinitionURLIsTheSnapshotElementType`
(`pkg/registry/choice_merge_test.go`), que corre en CI.

## Contexto

Para fusionar un diferencial con su base, el generador de snapshots tiene que saber qué propiedades
JSON de un elemento son un mismo elemento de elección (`fixedString` y `fixedInteger` son las dos
`fixed[x]`; json.html#choice). Esos nombres y sus tipos cambian entre versiones FHIR: R4B agrega
`CodeableReference` y quita `Meta`, y R5 agrega `integer64` y quita `Contributor`. Por eso se leen de
la definición de `ElementDefinition` de la versión del perfil (regla 5 de NUCLEO.md). Para encontrar
esa definición, algo tiene que nombrarla.

## Decisión

La constante `elementDefinitionURL` (`pkg/registry/snapshot.go`) es el único nombre del metamodelo
escrito en el generador. Todo lo demás (los elementos `[x]`, sus tipos y los demás hijos) sale de la
definición resuelta con `ResolveByCanonical`.

Fundamento: un snapshot es una lista de ElementDefinition (profiling.html; R4, R4B y R5:
`StructureDefinition.snapshot.element` tiene tipo `ElementDefinition`). El struct `ElementDefinition`
del paquete ya es ese metamodelo escrito en código (sus tags JSON).

## Alternativa descartada

Derivar el nombre del tipo de `StructureDefinition.snapshot.element`. Eso exige escribir otros dos
nombres, el del recurso `StructureDefinition` y el del path, sin hacer el generador más dinámico. Lo
que esa alternativa comprobaría pasa a ser la verificación de este ADR: el test confirma, en R4, R4B y
R5, que el tipo de `StructureDefinition.snapshot.element` es el de la constante.

## Consecuencias

- Sin esa definición, el registro no puede fusionar un diferencial con elementos: la generación falla
  con un error que la nombra. Ninguna falla de la definición (ausente, sin snapshot, o que el
  ProfileResolver no entregó) es permanente para el perfil: si se carga después, el reintento funciona.
- Se usa la definición de la release del perfil (en memoria o desde el ProfileResolver), nunca la de
  otra release mientras haya resolver; sin resolver, la cargada.
- Si una versión futura cambia el tipo de los elementos de un snapshot, el test falla.

Plan: `docs/plans/2026-10-25-snapshot-choice-merge.md` (#175).
