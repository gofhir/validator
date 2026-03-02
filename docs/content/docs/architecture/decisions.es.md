---
title: "Decisiones de Arquitectura"
linkTitle: "Decisiones de Arquitectura"
description: "Los 21 Architecture Decision Records (ADRs) que documentan las decisiones tecnicas clave del GoFHIR Validator."
weight: 2
---

Esta pagina documenta cada decision de arquitectura significativa tomada durante el desarrollo del GoFHIR Validator. Seguimos el formato Architecture Decision Record (ADR): cada decision captura el **contexto** (por que se necesitaba la decision), la **decision** misma (que se eligio) y las **consecuencias** (compromisos e implicaciones).

## Resumen de ADRs

| ADR | Titulo | Estado |
|-----|--------|--------|
| ADR-001 | StructureDefinitions como Unica Fuente de Verdad | Aceptado |
| ADR-002 | Arquitectura de Pipeline con Fases | Aceptado |
| ADR-003 | Interfaces Pequenas (1-2 metodos) | Aceptado |
| ADR-004 | Opciones Funcionales para Configuracion | Aceptado |
| ADR-005 | Usar gofhir/fhirpath para Evaluacion FHIRPath | Aceptado |
| ADR-006 | Usar gofhir/fhir para Structs Tipados | Aceptado |
| ADR-007 | Carga Automatica de Paquetes por Version | Aceptado |
| ADR-008 | OperationOutcome con Extensiones de Ubicacion | Aceptado |
| ADR-009 | Desarrollo Incremental por Milestones | Aceptado |
| ADR-010 | Catalogo Centralizado de Mensajes | Aceptado |
| ADR-011 | Structs Ligeros en el Registro | Aceptado |
| ADR-012 | BackboneElement Usa el SD Raiz | Aceptado |
| ADR-013 | Choice Types con Comparacion Case-Insensitive | Aceptado |
| ADR-014 | Tipos Primitivos desde Regex del SD | Aceptado |
| ADR-015 | Manejo de Codigos de Tipo FHIRPath | Aceptado |
| ADR-016 | Tres Fases de Validacion Independientes | Aceptado |
| ADR-017 | Validacion de Tipo JSON Antes de Regex | Aceptado |
| ADR-018 | Validacion Recursiva de Tipos Complejos | Aceptado |
| ADR-019 | Estrategia de Cache de StructureDefinitions | Propuesto |
| ADR-020 | Paralelismo en Validacion | Propuesto |
| ADR-021 | Generacion de Snapshots | Propuesto |

---

## Arquitectura Central (ADR-001 a ADR-004)

### ADR-001: StructureDefinitions como Unica Fuente de Verdad

**Contexto**: FHIR define cientos de tipos de recursos, perfiles y extensiones. Hardcodear reglas de validacion para cada uno seria inmantenible y especifico de version.

**Decision**: Todas las reglas de validacion se derivan en tiempo de ejecucion desde los snapshots de StructureDefinition. El validador lee las entradas `ElementDefinition` para determinar cardinalidad, tipos, bindings, constraints y toda otra regla. No existe logica especifica de recursos en el codigo.

**Consecuencias**: El validador soporta cualquier version de FHIR o perfil personalizado sin cambios de codigo. El compromiso es que un snapshot de StructureDefinition valido debe estar disponible para cada tipo que se valida.

### ADR-002: Arquitectura de Pipeline con Fases

**Contexto**: La validacion involucra muchas categorias diferentes de verificaciones (estructural, cardinalidad, terminologia, constraints FHIRPath, etc.). Mezclarlas en una sola pasada crearia codigo enredado y dificil de probar.

**Decision**: La validacion esta organizada como un pipeline secuencial de fases independientes. Cada fase es un paquete separado (`pkg/structural`, `pkg/cardinality`, etc.) con su propio tipo `Validator`. El orquestador en `pkg/validator` las ejecuta en orden y fusiona los resultados.

**Consecuencias**: Cada fase puede desarrollarse, probarse y depurarse de forma aislada. Se pueden agregar nuevas fases sin modificar las existentes. El compromiso es que el arbol del recurso se recorre multiples veces (una por fase), pero los beneficios en claridad y mantenibilidad superan el costo en rendimiento.

### ADR-003: Interfaces Pequenas (1-2 metodos)

**Contexto**: El validador necesita puntos de extension para servicios de terminologia, resolucion de perfiles y logging. Las interfaces grandes crean acoplamiento fuerte y son dificiles de implementar.

**Decision**: Todas las interfaces del proyecto tienen como maximo dos metodos. Ejemplos clave: `terminology.Provider` (2 metodos), `registry.ProfileResolver` (1 metodo). Esto sigue la convencion de la biblioteca estandar de Go de interfaces pequenas y componibles.

**Consecuencias**: Las interfaces son triviales de mockear para testing y faciles de envolver con middleware (cache, logging, metricas). Los consumidores solo implementan lo que necesitan. El compromiso es que operaciones complejas pueden requerir combinar multiples interfaces pequenas en lugar de llamar a una sola grande.

### ADR-004: Opciones Funcionales para Configuracion

**Contexto**: El validador necesita comportamiento configurable (proveedor de terminologia, logger, fases deshabilitadas, etc.) pero debe permanecer simple de usar con valores por defecto sensatos.

**Decision**: Usar el patron de opciones funcionales (funciones `With*` que retornan closures `Option`) para toda configuracion en tiempo de construccion. `validator.New()` funciona con cero argumentos, y las opciones se agregan segun se necesiten.

**Consecuencias**: La API es limpia, descubrible y compatible hacia atras. Agregar nuevas opciones de configuracion nunca cambia la firma de la funcion `New()`. El compromiso es una curva de aprendizaje inicial ligeramente mayor para usuarios no familiarizados con el patron.

---

## Dependencias (ADR-005 a ADR-006)

### ADR-005: Usar gofhir/fhirpath para Evaluacion FHIRPath

**Contexto**: Los constraints de FHIR (`ElementDefinition.constraint`) se expresan como expresiones FHIRPath. Evaluarlos requiere un motor FHIRPath capaz de manejar funciones especificas de FHIR como `resolve()` y `memberOf()`.

**Decision**: Usar la biblioteca `gofhir/fhirpath` como motor de evaluacion FHIRPath. Soporta el conjunto de funciones especificas de FHIR e integra con el sistema de tipos de Go.

**Consecuencias**: La evaluacion de constraints es completamente funcional con soporte para `resolve()`, `memberOf()` y evaluacion consciente del contexto. La dependencia esta bien delimitada y puede reemplazarse si es necesario ya que se accede a traves de una interfaz interna.

### ADR-006: Usar gofhir/fhir para Structs Tipados

**Contexto**: El validador necesita representar tipos de datos FHIR (StructureDefinition, ElementDefinition, OperationOutcome, etc.) en codigo Go. Escribir estos structs a mano es propenso a errores y dificil de mantener sincronizado con la especificacion.

**Decision**: Usar la biblioteca `gofhir/fhir` para structs Go tipados que reflejan las definiciones de recursos FHIR. Estos structs se usan para cargar StructureDefinitions y producir resultados OperationOutcome.

**Consecuencias**: Seguridad de tipos en tiempo de compilacion y serializacion/deserializacion JSON automatica. La dependencia esta limitada a tipos de datos y no afecta la logica de validacion en si.

---

## Infraestructura (ADR-007 a ADR-011)

### ADR-007: Carga Automatica de Paquetes por Version

**Contexto**: Los usuarios necesitan validar recursos contra la version correcta de FHIR (R4, R4B, R5). Requerir configuracion manual de definiciones base crea friccion.

**Decision**: El validador carga automaticamente el paquete de especificacion FHIR apropiado basandose en la version declarada. Las definiciones base de FHIR R4 estan embebidas en el binario via `pkg/specs`, eliminando dependencias de archivos externos.

**Consecuencias**: La validacion sin configuracion funciona directamente. El binario es autocontenido sin necesidad de descargar archivos de especificacion. El compromiso es un mayor tamano del binario debido a las especificaciones embebidas.

### ADR-008: OperationOutcome con Extensiones de Ubicacion

**Contexto**: El `OperationOutcome` de FHIR proporciona campos `expression` (FHIRPath) y `location` (XPath), pero ninguno captura la posicion exacta en JSON (linea y columna) de un error.

**Decision**: Extender `OperationOutcome.issue` con extensiones personalizadas que llevan informacion de ubicacion en el recurso (numero de linea, numero de columna) del JSON original.

**Consecuencias**: Los usuarios obtienen ubicaciones precisas de errores para depuracion. La salida sigue siendo un OperationOutcome FHIR valido ya que las extensiones son parte del modelo de extensibilidad FHIR.

### ADR-009: Desarrollo Incremental por Milestones

**Contexto**: Construir un validador FHIR completo es una empresa grande. Intentar todo de una vez retrasaria la entrega y haria las pruebas dificiles.

**Decision**: Desarrollar el validador incrementalmente a traves de 15 milestones, siguiendo la jerarquia de tipos FHIR desde infraestructura (M0) pasando por validacion estructural (M1) hasta caracteristicas avanzadas como slicing (M12) y optimizacion de rendimiento (M15).

**Consecuencias**: Cada milestone entrega funcionalidad funcional y testeable. El enfoque permitio integracion continua de pruebas y retroalimentacion temprana. Consulta la pagina de [Milestones](../milestones) para la hoja de ruta completa.

### ADR-010: Catalogo Centralizado de Mensajes

**Contexto**: La validacion produce mensajes de diagnostico que deben ser consistentes, traducibles y faciles de mantener. Dispersar cadenas de mensajes por las implementaciones de fases los hace dificiles de mantener consistentes.

**Decision**: Todos los mensajes de diagnostico estan definidos en un catalogo centralizado de mensajes en el paquete `pkg/issue`. Cada mensaje tiene un identificador y una cadena plantilla. Las fases referencian mensajes por identificador en lugar de construir cadenas inline.

**Consecuencias**: Los mensajes son consistentes entre fases, faciles de revisar y podrian traducirse en el futuro. El catalogo tambien sirve como documentacion de todas las posibles salidas de validacion.

### ADR-011: Structs Ligeros en el Registro

**Contexto**: El tipo completo `StructureDefinition` de FHIR es grande e incluye campos no necesarios durante la validacion (ej., narrativa, metadatos del publicador). Cargar miles de SDs completos en memoria es un desperdicio.

**Decision**: El paquete `pkg/registry` usa structs internos ligeros que extraen solo los campos necesarios para validacion (elementos del snapshot, informacion de tipos, detalles de binding). Los SDs completos se parsean durante la carga y luego se descartan.

**Consecuencias**: Uso de memoria significativamente reducido al cargar Implementation Guides grandes con cientos de perfiles. El compromiso es un paso de transformacion durante la carga, pero esto ocurre una sola vez al inicio.

---

## Logica de Validacion (ADR-012 a ADR-018)

### ADR-012: BackboneElement Usa el SD Raiz

**Contexto**: Los tipos BackboneElement se definen inline dentro del StructureDefinition de un recurso en lugar de como tipos independientes. El validador necesita localizar los ElementDefinitions correctos para estas estructuras anidadas.

**Decision**: Al validar hijos de BackboneElement, el validador navega el StructureDefinition del recurso raiz en lugar de buscar un SD separado. Paths de elementos como `Patient.contact.name` se resuelven dentro del SD de Patient.

**Consecuencias**: La validacion de BackboneElement funciona correctamente sin requerir SDs separados para tipos inline. El walker mantiene el contexto completo del path del elemento para habilitar esta navegacion.

### ADR-013: Choice Types con Comparacion Case-Insensitive

**Contexto**: Los choice types de FHIR (ej., `value[x]`) pueden aparecer en JSON como `valueString`, `valueQuantity`, etc. El sufijo del tipo debe coincidir con los tipos permitidos del ElementDefinition.

**Decision**: Usar comparacion case-insensitive al hacer match del sufijo de tipo en un nombre de choice element contra las entradas declaradas en `ElementDefinition.type`. Esto maneja casos limite como `valueUri` vs `valueURI`.

**Consecuencias**: Match robusto de choice types que maneja todas las variantes de capitalizacion definidas en la especificacion FHIR.

### ADR-014: Tipos Primitivos desde Regex del SD

**Contexto**: Los tipos primitivos de FHIR (date, dateTime, uri, code, etc.) tienen reglas de formato expresadas como expresiones regulares en sus StructureDefinitions.

**Decision**: Derivar todos los patrones de validacion de primitivos desde la extension `regex` en el StructureDefinition del tipo primitivo en lugar de hardcodear patrones. El regex se lee de `ElementDefinition.type` y se compila una vez.

**Consecuencias**: La validacion de primitivos es agnostica de version y automaticamente correcta para cualquier version de FHIR. La compilacion de regex se cachea para evitar parseo repetido.

### ADR-015: Manejo de Codigos de Tipo FHIRPath

**Contexto**: Las expresiones FHIRPath en constraints referencian codigos de tipo FHIR (ej., `Patient`, `HumanName`). El motor FHIRPath necesita resolver estos a informacion de tipo real.

**Decision**: Los codigos de tipo encontrados durante la evaluacion FHIRPath se resuelven a traves del registro, mapeandolos a sus StructureDefinitions correspondientes. Esto permite al motor FHIRPath navegar jerarquias de tipos correctamente.

**Consecuencias**: Los constraints FHIRPath que referencian tipos funcionan correctamente. El registro sirve como la unica fuente de informacion de tipos tanto para validacion como para evaluacion FHIRPath.

### ADR-016: Tres Fases de Validacion Independientes

**Contexto**: Los disenos iniciales consideraron un enfoque de validacion en una sola pasada. Sin embargo, algunas categorias de validacion tienen dependencias naturales (ej., la validacion estructural debe ejecutarse antes de la cardinalidad).

**Decision**: Aunque el pipeline completo tiene 9 fases, pueden agruparse logicamente en tres etapas independientes: validacion estructural (fases 1-3), validacion semantica (fases 4-9) y validacion entre recursos. Cada etapa se construye sobre la confianza establecida por la anterior.

**Consecuencias**: Los errores estructurales fatales previenen validacion semantica innecesaria. La agrupacion proporciona un modelo mental claro para entender el flujo de validacion.

### ADR-017: Validacion de Tipo JSON Antes de Regex

**Contexto**: La validacion de primitivos involucra dos verificaciones: el valor JSON debe ser del tipo correcto (string, number, boolean) y el valor debe coincidir con el regex de formato. Ejecutar regex en un valor de tipo incorrecto produce errores confusos.

**Decision**: Siempre validar el tipo JSON primero. Si el tipo JSON es incorrecto (ej., un numero donde se espera un string), reportar ese error y omitir la verificacion de regex. Esto produce diagnosticos mas claros y accionables.

**Consecuencias**: Los usuarios ven el error mas fundamental primero. Un `birthDate` con valor numerico reporta "se esperaba string, se obtuvo number" en lugar de un mismatch de regex.

### ADR-018: Validacion Recursiva de Tipos Complejos

**Contexto**: Los tipos complejos de FHIR (HumanName, Address, CodeableConcept, etc.) contienen elementos anidados que tambien deben validarse. La validacion debe manejar profundidad de anidamiento arbitraria.

**Decision**: La validacion de tipos complejos se maneja recursivamente. Cuando el walker encuentra un elemento de tipo complejo, resuelve el StructureDefinition del tipo y valida los hijos contra el. Esta recursion maneja naturalmente tipos anidados a cualquier profundidad.

**Consecuencias**: Todos los tipos complejos se validan uniformemente sin importar la profundidad de anidamiento. El mismo camino de codigo maneja tanto elementos de nivel superior como estructuras profundamente anidadas dentro de extensiones o BackboneElements.

---

## Propuestos (ADR-019 a ADR-021)

{{< callout type="warning" >}}
Los siguientes ADRs estan en estado **Propuesto**. Documentan decisiones bajo consideracion que aun no han sido implementadas.
{{< /callout >}}

### ADR-019: Estrategia de Cache de StructureDefinitions

**Contexto**: Cargar y parsear StructureDefinitions es costoso. En escenarios de validacion por lotes, los mismos SDs se usan repetidamente.

**Decision (propuesta)**: Implementar una estrategia de cache multinivel con un cache LRU en memoria para SDs parseados y un cache de sistema de archivos para paquetes descargados. La invalidacion de cache se basaria en la version del paquete.

**Consecuencias**: Mejora significativa de rendimiento para validacion por lotes e integraciones de servidor de larga ejecucion. La capa de cache se ubicaria entre el registro y el loader.

### ADR-020: Paralelismo en Validacion

**Contexto**: Al validar un lote de recursos, cada recurso es independiente y podria validarse concurrentemente.

**Decision (propuesta)**: Proporcionar un modo de validacion paralela opcional usando un pool de workers. El numero de workers seria configurable via opciones funcionales. Cada worker tendria su propia copia del estado mutable mientras comparte StructureDefinitions de solo lectura.

**Consecuencias**: Escalamiento lineal de throughput para cargas de trabajo por lotes en sistemas multi-core. El diseno shared-nothing para estado mutable evitaria contencion de locks.

### ADR-021: Generacion de Snapshots

**Contexto**: Algunos StructureDefinitions solo proporcionan un differential, no un snapshot completo. El validador requiere un snapshot para la validacion.

**Decision (propuesta)**: Implementar generacion de snapshots que fusione el differential de un perfil con el snapshot de su definicion base. El generador recorreria cadenas de `baseDefinition` y aplicaria overrides del differential.

**Consecuencias**: El validador podria trabajar con perfiles que carecen de snapshots precalculados. Esto es particularmente importante para perfiles creados por usuarios e Implementation Guides que solo incluyen differentials.
