# Descenso por todas las definiciones que gobiernan un valor: revisión previa (paso 1)

Rama `refactor/definition-layers-descent`, desde `main` en `ba63bba` (18-11-2026). Es la parte de **#193** que sigue abierta, y
es previa a los [límites del valor](../../2026-11-17-element-value-limits.md), que quedan en pausa.

## Problema

Bajo un valor, `pkg/constraint` baja por un solo conjunto de hijos (`tree.go:105-125`, `governedChildren`). Las demás definiciones que
gobiernan el valor se aplican en su raíz, pero no en sus descendientes. El issue #193 ya lo describe y propone «a set of governing
definitions per value, with provenance». Las piezas 1 a 3 de #193 cubrieron los perfiles de tipo; este descenso no.

## `gofhir/server` (congelado)

Delega la validación en esta librería (`go-fhir-server/go.mod:38`). **Descartar:** no hay nada que tomar.

## Especificación (profiling.html, «Re-profiling and Re-slicing»; texto idéntico en R4, R4B y R5, leído el 18-11-2026)

- Una restricción sobre X sin `sliceName`: «the profile is adding constraints to all slices of X».
- Con `sliceName`: «the profile is describing a specific slice on X, and the constraints only apply to that slice».
- En un perfil derivado, sin `sliceName`: «the profile is constraining all slices of X».

Lo que se declara sobre el elemento dividido vale, entonces, para todas sus repeticiones, también las de un slice. El perfil de tipo
y la definición propia de una extensión gobiernan por sí mismos (issue #193: extensibility.html, `ElementDefinition.type.profile`).

## HL7 validator 7.0.4

`InstanceValidator.java`, tag 7.0.4:
- `:7957-7968`: cada hijo se comprueba contra su definición y, además, contra su slice (`checkChildByDefinition` dos veces).
- `:8289-8302`: valida contra el perfil del tipo y después sigue por los hijos que el perfil despliega.

Cada comprobación recorre su subárbol, así que el costo se multiplica por capa. gofhir no puede copiar eso (ver «Costo»).

## Medido (R4; HL7 con `-Duser.timezone=UTC`)

Evidencia: [review2/](../2026-11-17-element-value-limits/review2/) y [review1/a1, a1b](../2026-11-17-element-value-limits/review1/).

| Caso | HL7 | gofhir hoy |
|---|---|---|
| `fix-tp`: `fixed` en el perfil de tipo y en el hijo desplegado | F3 y F10 | solo F10 |
| `fix-pe`: `fixed` en un slice de la extensión de un primitivo | error | nada |
| `fp-fix-mrn`: `fixed` del elemento base; el slice declara otro | error contra la base | nada |
| `fp-pat2-mrn`: `pattern` del elemento base; el slice declara otro | error contra la base | nada |
| `fix-resl2`: reslicing | F7 (`a/b`) y F3 (base), sin `a` | por medir |
| `fix-crn`: `fixed` en el destino de un `contentReference` | nada | error (divergencia B-D9, ya declarada) |
| `a1`: el slice no redeclara | error | error (el snapshot lo hereda) |

Son **falsas aceptaciones que ya existen**. Bajo slices hay 163, 166 y 96 elementos con `fixed` o `pattern` en los packages core de R4, R4B y
R5, pero **ninguno difiere del de su elemento base**; sí difiere un binding (`probe/core-scan.txt`).

## Costo: lo que no se puede perder

La pieza 2 midió que recorrer una vez por capa es exponencial en la profundidad (plan `2026-11-06-type-profile-layers.md`, D5) y lo
resolvió con un descenso por valor (`firstDescent`, `layers.go:47`): 130.800 visitas bajaron a 2.696 con D = 12. Este incremento tiene que
llevar **un conjunto de definiciones en un solo descenso**, no un recorrido por definición.

## Preguntas para el plan

1. Cómo se emparejan los hijos entre definiciones: identidad del elemento, `value[x]` renombrado, slices anidados, reslicing.
2. Qué hacen los invariantes, `fixed[x]`, `pattern[x]`, bindings y tipos con varias capas, y cómo se deduplican. HL7 repite un `fixed` igual
   declarado en dos capas (`a1b`); gofhir informa uno.
3. Si la fase de cardinalidad entra aquí (#193 dice que tiene la misma forma, `registry/instance_children.go`) o va aparte.
4. Reslicing y `contentReference`: igualar a HL7 o declarar la divergencia.
5. Cuántos resultados cambian en los ejemplos del core y en los 61 grupos de hl7diff.
