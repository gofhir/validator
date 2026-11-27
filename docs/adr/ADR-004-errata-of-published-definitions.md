# ADR-004: las erratas de las definiciones publicadas son datos en Go, y cada definición se corrige con su versión

## Estado

Aceptado (27-11-2026), con el plan [docs/plans/2026-11-27-errata-cross-version.md](../plans/2026-11-27-errata-cross-version.md)
(diseño v6, tres rondas adversariales de la segunda tanda y la decisión del usuario del 27-11-2026). Lo verifican:
- `TestErrataCrossVersion*` (`pkg/registry/errata_cross_version_test.go`): cadenas con versiones mezcladas, procedencia,
  parche de hoja, copias, `bdl-15`, core raíz, y `TestErrataCrossVersionAgainstSource` contra la fuente;
- `TestErrataCrossVersionMatrix` (`pkg/validator/errata_cross_version_test.go`): versión de la base × versión del perfil ×
  versión validada, con el esperado tomado de HL7 validator_cli 7.0.4 (`testdata/errata_cross_version/expected.json`).

## Contexto

Algunas definiciones publicadas están mal en la versión en que se publicaron, y la corrección no está en el package de esa
versión: está en una publicación posterior (otra versión de la misma definición, o una guía que republica la constraint
corregida). Un ejemplo: `ref-1` de R4 falla en una referencia que solo trae `display`; R5 la escribe con `reference.exists()
implies`. HL7 las corrige en código (`InstanceValidator`, `FHIRPathExpressionFixer`).

Un snapshot generado hereda del JSON publicado de su base. Si la errata corrige solo el struct, toda copia que se arma desde
el `raw` (mezcla del diferencial, slices, despliegue de hijos, clon del registro) vuelve a traer el texto publicado. Y si el
snapshot generado se corrige al final con una sola versión, una constraint heredada de una base de otra versión no coincide
con la expresión publicada de esa versión: un perfil con `fhirVersion` 4.0.1 sobre una base R4B daba `ref-1` donde HL7 da 0
errores. Los diseños v1 a v3 del plan fallaron por esa causa común.

## Decisión

**Las erratas son datos escritos en Go** (`pkg/registry/errata.go`: `typeErrata`, `constraintErrata`, `expressionErrata`,
`addedConstraints`, `contextErrata`), cada una con su `source`, la definición oficial que la corrige. No pueden salir del
package de su versión, porque ahí está la definición defectuosa; es la excepción acotada a «todo desde la
StructureDefinition», y por eso este ADR. Solo se corrige donde lo publicado es exactamente lo defectuoso: una definición
distinta no se toca.

**Regla: cada definición se corrige con su propia versión, y lo derivado hereda la corrección** (parche de hoja en el `raw`):
1. Al cargar (`applyErrata`, también en el clon, `unpublishedCopy`), cada definición se corrige con su `fhirVersion` (sin
   versión, la validada), en el struct y en la hoja del `raw`: el `expression` de la constraint y el `valueUrl` de la
   extensión `structuredefinition-fhir-type` del tipo (`patchRaw`). Los demás miembros del JSON no cambian. Toda copia
   hereda el texto corregido, sin adivinar de qué definición vino.
2. Lo que escribe el perfil se corrige antes de la mezcla (`correctPlaced`, en `applyDifferential`), con la versión del
   perfil, sobre el elemento del diferencial ya ubicado. La errata se identifica con el elemento de la base en esa posición
   (su `path` y su `base.path`), porque el diferencial puede no traer `base.path`. Lo heredado ya viene corregido por su
   propia definición y la mezcla no lo reescribe.
3. No hay corrección posterior a la mezcla. Solo se agregan las constraints que la errata añade (`bdl-15`), que viven en el
   struct y nunca en el `raw` (si estuvieran, la mezcla, que deduplica por clave y gana la base, descartaría la constraint
   propia de un perfil con la misma clave). Después de la mezcla, cada elemento las hereda de su elemento en la base (el
   mismo `id`, o el del elemento dividido para el hijo de un slice), donde no tiene esa clave (`inheritAddedConstraints`).
   La base ya las tiene en el struct: se agregaron al cargar la definición publicada con su versión. Así ninguna versión se
   adivina (code review, ronda 2). Hoy la única agregada es `bdl-15`, en la raíz de Bundle. Un test de guarda falla si se
   agrega una errata sobre un elemento que no es la raíz de un recurso: esa herencia no cubre los hijos desplegados ni
   `contentReference` (code review, ronda 3).
4. Todo parche de hoja refresca la caché de `fixed[x]` y `pattern[x]` (`keepPrefixedValues`).

**Fuera de alcance, registrado:** un perfil que redeclara una clave heredada con otra expresión pierde la mezcla (gana la
base); HL7 rechaza ese perfil. Es el frente del choque de claves, junto con R3-P1 y 10r.

## Erratas y fuentes

Las que citan un core embebido (`hl7.fhir.r4b.core#4.3.0`, `hl7.fhir.r5.core#5.0.0`, `hl7.fhir.r4.core#4.0.1`) las verifica
`TestErrataCrossVersionAgainstSource` contra la definición del package de `pkg/specs`: `corrected` es lo publicado allí, y
`published` no. Son: `que-7`, `ref-1` (R4 y R4B), `bdl-8`, `ras-2`, `que-12`, `tim-9` (R4 y R4B), `con-3`, `csd-0` (por
expresión, 30 recursos), `bdl-15` (agregada), el tipo de `Resource.id` y los contextos de `regex`.

Las que citan un package no embebido se verificaron a mano contra el package extraído en `~/.fhir/packages` el 27-11-2026.
Solo está la carpeta extraída, no el `.tgz` (no se descargó), así que el SHA-256 es el de los archivos citados, y el de todo
el árbol del package (los SHA-256 de cada archivo, ordenados por ruta, con el SHA-256 de esa lista):

| Package | Erratas | Archivo | SHA-256 del archivo | SHA-256 del árbol |
|---|---|---|---|---|
| `hl7.fhir.r6.core#6.0.0-snapshot1` | `eld-11`; tipo de `ElementDefinition.id` (R4B y R5) | `StructureDefinition-ElementDefinition.json` | `69f34f73a7d098ad9069841f5df43370a25c9367354fe8628f110434ed280d45` | `5c66c565aa862ac61146eaedd855d811e4239222665c909935b0ea87645255a0` |
| `hl7.fhir.us.core#9.0.0` | `pd-1`, `us-core-13` | `StructureDefinition-us-core-practitionerrole.json` | `79eaec8c7c40e12508681366c05adb863b68c157bda07558696617ba943c0b18` | `8c1878d5ce7fc184d39a1446676d441c3acc389e9a5b72dd8fbc13b73d7cca21` |
| `hl7.fhir.us.core#9.0.0` | `vs-1` (R4, R4B, R5) | `StructureDefinition-us-core-vital-signs.json` | `9d24c08f9a9061c103674ac3a69cd092114c3155510406ec6b9b757ebbb4493e` | (el mismo) |

Verificado en esos archivos: `eld-11` con `':'`; `ElementDefinition.id` con `fhir-type` `string`; `pd-1` y `us-core-13`
con `.exists()`; `vs-1` con `$this is dateTime implies ...` y `source` `vitalsigns`.

**Divergencia con HL7 en `vs-1` con R5.** HL7 7.0.4 informa `vs-1` en `Observation.effectivePeriod` validando con R5; gofhir
no, porque corrige `vs-1` en R4, R4B y R5 con la corrección ya publicada (PR #142, `docs/content/docs/error-reference/
constraints.md`, fila `vs-1`; fuente: US Core 9.0.0, que republica la constraint corregida). La declara `DIVERGENCES` en
`gen_expected.py` (evidencia de la matriz).

## Consecuencias

- Una erratum nueva es una fila con su `source` y pasa por el test contra la fuente (si está embebida) o por este ADR (si no).
- Un perfil de una versión sobre una base de otra, y toda cadena con versiones mezcladas, queda corregido sin elegir una
  versión para toda la cadena.
- El costo es un parche de hoja por elemento que una errata cambia (decenas por package), medido en el plan.
- Sin dependencias nuevas.
