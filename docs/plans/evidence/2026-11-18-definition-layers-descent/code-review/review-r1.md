# Code review r1: definition-layers descent (branch vs ba63bba)

Revisor independiente (Opus). Sondas en `review8/probes/` (`gen.py`, `gen2.py`; trabajo en `w1/`, `w2-4.0.1/`, `w2-5.0.0/`).
Cada perfil pasó por el `snapshot` de HL7 7.0.4 (14/14 y 7/7 en R4 y R5). Salidas: `hl7/*.txt` (HL7), `pb.out` (rama), `pm.out` (main),
generadas con un arnés mínimo (`review8/probe-src/main.go`: `New(WithVersion, WithConformanceResources, WithNoTerminology)`).
Mutaciones propias: `review8/mut.py`, `mut.out`, `mut2.out`, `mut-*.txt`.

## CONFIRMED

### F1 (high): «(defined in …)» en constraints de los hijos de un perfil de tipo; regresión contra main y HL7
`pkg/constraint/constraint.go:676` (`reportedSource`, regla `owner.URL`).
La regla pone la URL del dueño a **toda** constraint propia sin `source` de una definición que no es la validada. HL7 solo
la pone en la raíz del perfil de tipo (la que el generador funde en el elemento); los hijos del perfil de tipo salen sin sufijo.
- `w2-*/q-i` (R4 y R5): `q-idp` con `qr-1` en la raíz y `qc-1` en `Identifier.value`. HL7: `qr-1 … (defined in …/q-idp)` y
  `qc-1` **sin sufijo**. Rama: los dos con sufijo. Main: ninguno.
- `w1/p3-pat-plain-i` y `w2-*/p3-pat-plain-i`: `idp-1` en `Identifier.value` de un perfil de tipo. HL7 y main sin sufijo; rama
  `(defined in …/p3-idp)`. **Regresión.** (Con el hijo desplegado por el perfil, `p3-pat-unroll`, la rama coincide con HL7.)
- Ningún test lo cubre: `TestConstraintDefinedIn` no tiene un hijo de perfil de tipo, y la matriz compara clase de informe, no
  texto (criterio 1).

Arreglo: la URL del dueño, solo para las constraints de la **raíz** de una capa de perfil de tipo (`typeRoot`), no para
`ownerElement` descendientes. Mejor aún: reproducir lo que hace el generador de HL7 (fundir y sellar las de la raíz del
perfil de tipo en el elemento del snapshot) y que el validador imprima solo `c.Source`, sin regla sintética en la validación.
Agregar `q-i` y `p3-pat-plain` a `TestConstraintDefinedIn` con las salidas de HL7.

### F2 (high): «(defined in …)» en las constraints propias de la definición de una extensión; regresión
`pkg/constraint/tree.go:100` (`self != typeSD` salta `layerOfSelf`) junto con `constraint.go:676`.
- `w2-*/q-ext-bare` (extensión sin perfil que la nombre) y `q-ext-slice` (slice con el perfil de la extensión), R4 y R5:
  HL7 da `er-1` (raíz de la extensión) **sin sufijo** en los dos. Rama: `er-1 … (defined in …/q-ext)`. Main: sin sufijo.
- Causa: cuando la definición propia es también el perfil del tipo (`self == typeSD`), la raíz se evalúa por `typeRoot` con
  `opts.profile` del perfil del recurso, y la regla `owner.URL != opts.profile` dispara. `layerOfSelf` solo cubre `self != typeSD`.

Arreglo: con `self == typeSD`, evaluar `typeRoot` con `opts.profile = self.URL` (o no aplicar la regla a la definición propia).
Test con las salidas de HL7 de `q-ext-*`.

### F3 (high): slicing sin discriminador decidido por `fixed`/`pattern`: falso rechazo frente a HL7
`pkg/slicematch/slicematch.go:225` (`discriminatorsOf`).
La premisa del §9 («HL7 decide por conformidad») no se cumple en HL7 7.0.4: en cuatro sondas no asigna **ningún** slice, ni
siquiera a una instancia que cumple el slice entero.
- `w1/p7-match` (cumple `s1`: system fijo y `s1-1`): HL7 «does not match any known slice» (information).
- `w1/p7-inv` (system `urn:a`, valor que viola `s1-1`, slicing open): HL7 **Success**. Rama: **error** `s1-1` (lo asignó a `s1`
  por el `fixedUri` implícito). Main: error «Slicing cannot be evaluated». El veredicto difiere de HL7 en los dos, y la rama
  cambia la causa.
- `w1/p7b-novalue` (closed): HL7 error «does not match any known slice … CLOSED»; rama, error de cardinalidad del slice
  (`Patient.identifier:s1.value` min 1). Mismo veredicto, informe distinto.
- `w1/p7c-*` (closed; un slice sin valores fijos): la rama sigue dando un **error** «Slicing cannot be evaluated» que HL7 no da.
  Con `rules: open`, ese error es un falso rechazo (ya en main).
- La evidencia del propio plan coincide: en `bad-m-nod` (`tests/hl7/x2-4.0.1-7.0.4.json`) HL7 informa «does not match any known
  slice», y el error viene del `fixedString` del elemento base, no del slice.

Arreglo: decidirlo con evidencia del código de HL7. Si se sigue a HL7, sin discriminador ningún slice se asigna (información si
es open, error si es closed), sin `SLICING_CANNOT_BE_EVALUATED` como error. Si se elige la conformidad completa (la lectura de
la spec), debe ser completa y declararse como divergencia: el aproximado por `fixed`/`pattern` da veredictos que no son ni lo uno
ni lo otro (`p7-inv`).

### F4 (medium): `reportedSource` está ajustado a `meta.profile`; con un perfil explícito, el texto difiere de HL7 (R4)
`pkg/constraint/constraint.go:672-674` (`publishedSource`).
`w1/p2-explicit` y `w2-4.0.1/p2-explicit` (HL7 `-profile`, gofhir `ValidateWithProfile`): HL7 `pat-1 … (defined in …/Patient)`;
gofhir (rama y main) sin sufijo. En R5 coinciden porque el core publica `source`. El comentario de `reportedSource` justifica la
regla en que «HL7 valida primero contra la definición del tipo», lo que pasa con `meta.profile` (`p2-meta`, coincide) pero no con
`-profile`. Ya estaba en main; la rama lo codifica como regla. Arreglo: aplicar `publishedSource` solo cuando el recorrido
también validó contra la definición del tipo, o declararlo.

### F5 (medium): mis mutaciones que sobreviven (en una copia; tests dirigidos y después los paquetes completos)
Ver `mut.out` y `mut2.out` (paquetes `pkg/validator`, `pkg/registry`, `pkg/slicematch`, `pkg/constraint`):
- **F** (`tree.go:365`, `walkPrimitiveExtensions` sin recorrer la cadena `SliceOf`: `n = nil`): sobrevive. El punto 12 del plan
  (los hijos que el slice y su cadena despliegan para `_x`) no tiene test que dependa de la cadena más allá de `node`.
- **L** (`walkLayers` sin `walkSets`: con varios perfiles de tipo no se recorren la cadena `SliceOf` ni la definición propia):
  sobrevive. Es la combinación D/S × varios perfiles de tipo, sin sonda.
- **B** (`carryDeclaredValues` solo con `fixed`): sobrevive; la mitad `pattern` no se prueba.
- **D** (`publishedSource` sin la búsqueda en la raíz del tipo): sobrevive.
- **K** (índice `any` se queda con la última y no con la primera): sobrevive; el desempate por «la primera de su snapshot» no se prueba.
- **I** (discriminador implícito de un `pattern` tratado como `value`): sobrevive; ningún test de slicing sin discriminador usa `pattern`.
Arreglo: una sonda de HL7 por cada una (F y L primero: son reglas del alcance de la pieza).
Las que mueren: A (`divided` sin la guarda del elemento base; `bad-g3`), C (sello sin la búsqueda sin nombre de slice; `s5a-ew`),
E (`walkPrimitiveExtensions` sin ámbito; conteo), G (`alternativeTypes` ignora `self`; `bad-o-alt`), H (`typeBelow` apagado; `bad-g1`).
J fue una mutación inválida (no compila) y no cuenta.

### F6 (low): comentario del paquete `walkprobe` desactualizado
`internal/walkprobe/walkprobe.go:4` cita un `Enabled` que no existe (el mecanismo es `From(ctx) == nil`).

## SUSPECTED

### S1 (medium): mutación 28 no es equivalente, pero la regla que protege es la de F1
Quitar `owner.Derivation == DerivationConstraint` en `constraint.go:676` solo cambia algo con un dueño especialización (o sin
`derivation`) con snapshot generado: un modelo lógico con solo diferencial usado como tipo de otro. HL7 recorre el snapshot del
modelo sin sellar sus propias constraints, así que el código actual (sin sufijo) es lo esperado y la mutación imprimiría uno de
más. No la probé con HL7: validar modelos lógicos en las dos herramientas exige montar más. Ojo además: un perfil sin el campo
`derivation` (hay paquetes viejos así) cae en la rama sin regla; con F1 corregido como sello en el snapshot esto desaparece.

### S2 (low): `overlayFixed` con varios perfiles
`pkg/slicematch/path.go:573-605`. La lectura de `type.profile` («must conform to at least one of them») apoya tomar cualquiera de
los perfiles; HL7 despliega solo el primero (`hl7-mut-m19-b.txt`). Es una divergencia declarada y el test fija lo de gofhir, no lo
de HL7: aceptable si se registra como divergencia con la sección de la spec (ElementDefinition.type.profile). Además, `overlayFixed`
cambia los hijos propios por los del perfil salvo los fijos: un discriminador por un camino que pasa por un hijo propio restringido de
otra forma (sub-slicing, perfil de tipo) lo pierde. Sin sonda.

### S3 (low, perf): `children()` llama a `typeChildren` en cada acierto de `snapshotChildren`
`pkg/slicematch/path.go:573`. Resuelve canónicos, arma árboles y crea `fmt.Errorf` para luego descartarlo casi siempre (un solo
perfil). Es el camino de slice matching; el costo medido en el core R4 no lo nota porque casi no tiene slicing. Arreglo barato:
mirar antes si el único tipo de `c` declara más de un perfil. Sin medición en US Core.

### S4 (low, perf): mapas por validación y por primitivo con `_x`
`constraint.go:357-358` crea `nested` y `holders` en cada `Validate` aunque no haya recursos sostenidos; `tree.go:364` abre un
ámbito (dos mapas) en todo primitivo con `_x`, aunque tenga una sola definición. Inicializar perezosamente. Medición de RSS del
paso 7 dentro del 5 %, así que es menor.

### S5 (low): `carryDeclaredValues` copia `fixed` del perfil aunque el clon tenga `pattern` (y viceversa)
`pkg/registry/snapshot.go:854`. Se evalúa por prefijo, así que un hijo puede quedar con `pattern[x]` y `fixed[x]` a la vez (eld-8 lo
prohíbe en un ElementDefinition). El efecto es la conjunción, que es lo que exigen las dos definiciones, pero el snapshot ya no es
válido. Sin sonda. La mutación B (sin `pattern`) sobrevive (ver F5), así que la mitad `pattern` no está cubierta.

## Lo que resistió
- Descenso por la cadena `SliceOf` con recursos sostenidos: `w1/p6-i` (perfil en `entry.resource` base y otro en el slice). HL7
  aplica los dos; la rama también (main perdía `px-1`). La guarda del elemento base en `divided` es necesaria (mutación A muere).
- Cadena de tres perfiles: `w1/p5-*`, `pa-1 (defined in …/p5-a)` como HL7 (main sin sufijo).
- Hijo desplegado de un perfil de tipo: `p3-pat-unroll` como HL7.
- R5 core con `source` (`w2-5.0.0/p2-noprofile`): igual que HL7.
- `CollapseSeverities` lee el scope raíz correcto (`validator.go:785` lo crea en el mismo `ctx`); el filtrado in situ es seguro
  porque se reasigna. `v.published` está acotado por la cantidad de tipos.
- Concurrencia: `nested`, `holders`, `memo` y `descended` viven por llamada a `Validate` y las capas corren en secuencia.

## Estilo y API
- `SnapshotGenerated`, `ElementNode.Definition` y `CollapseSeverities` son API exportada nueva. `CollapseSeverities` existe solo
  para que `pkg/validator` la llame: puede quedar exportada (paquetes distintos), pero el doc debería decir que es interna al
  pipeline. Si F1 se arregla sellando el snapshot, `SnapshotGenerated` sobra.
- Comentarios mezclados: `discriminatorsOf`, `typeBelow`, `kindOfValue` y el test de slicematch están en inglés; la regla del
  proyecto pide comentarios en español (el código existente ya mezcla).
