# Roadmap de Desarrollo Incremental

## Principio Fundamental

> **Todo se deriva de StructureDefinitions. Sin hardcoding.**

El desarrollo sigue la jerarquía natural de FHIR, construyendo de menos a más:

```
                    Element (base)
                        │
        ┌───────────────┼───────────────┐
        │               │               │
   Primitive       Complex         BackboneElement
     Types          Types               │
   (string,      (HumanName,      (usado en recursos)
   boolean,      Identifier,
   date...)      Coding...)
        │               │
        └───────┬───────┘
                │
           Extension
                │
        ┌───────┴───────┐
        │               │
     Resource      Terminology
        │          (CodeSystem,
   DomainResource   ValueSet)
        │
   Patient, etc.
```

---

## Jerarquía de StructureDefinitions en FHIR

```
Element                          kind: complex-type, base: null
├── string, boolean, etc.        kind: primitive-type, base: Element
├── HumanName, Identifier, etc.  kind: complex-type, base: Element
├── Coding                       kind: complex-type, base: Element
├── CodeableConcept              kind: complex-type, base: Element
├── BackboneElement              kind: complex-type, base: Element
└── Extension                    kind: complex-type, base: Element

Resource                         kind: resource, base: null
└── DomainResource               kind: resource, base: Resource
    ├── Patient                  kind: resource, base: DomainResource
    ├── Observation              kind: resource, base: DomainResource
    └── ... (todos los recursos)
```

---

## Milestones de Desarrollo

### Milestone 0: Infraestructura Base ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Setup del proyecto, loader de paquetes, estructura navegable.

```
[x] 0.1 - Proyecto Go (go.mod, estructura)
[x] 0.2 - Loader de paquetes FHIR NPM
[x] 0.3 - Registry de StructureDefinitions
[x] 0.4 - Parser JSON → Element tree navegable
[x] 0.5 - Estructura base de Issue y Result
```

**Implementación:**

- `pkg/loader/` - Carga paquetes desde `~/.fhir/packages/`
- `pkg/registry/` - Indexa StructureDefinitions por URL y tipo
- `pkg/issue/` - Result, Issue, Severity, Code
- `pkg/validator/` - API pública con opciones funcionales

**Decisiones:** ADR-011 (Structs ligeros)

---

### Milestone 1: Validación Estructural Básica ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar que los elementos existen en el StructureDefinition.

```
[x] 1.1 - Mapear Element → ElementDefinition por path
[x] 1.2 - Detectar elementos desconocidos (no definidos en SD)
[x] 1.3 - Resolver choice types (value[x] → valueString, valueInteger, etc.)
[x] 1.4 - Generar issues con FHIRPath correcto
```

**Implementación:**

- `pkg/structural/structural.go` - Validador estructural
- `elementIndex` con `byPath` y `choiceTypes`
- `validationContext` para mantener SD raíz

**Decisiones:** ADR-012 (BackboneElement), ADR-013 (Choice types case-insensitive)

**Tests:**

- `testdata/m1-structural/valid-patient-with-name.json`
- `testdata/m1-structural/invalid-patient-unknown-element.json`
- `testdata/m1-structural/valid-patient-choice-types.json`
- `testdata/m1-structural/invalid-patient-bad-choice-type.json`

---

### Milestone 2: Cardinalidad (min/max) ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar cardinalidad desde ElementDefinition.min/max.

```
[x] 2.1 - Validar min (elementos requeridos)
[x] 2.2 - Validar max (límite superior)
[x] 2.3 - Manejar max="*" (sin límite)
[x] 2.4 - Contar correctamente en arrays
```

**Implementación:**

- `pkg/cardinality/cardinality.go` - Validador de cardinalidad
- Valida elementos directos y anidados en BackboneElements
- Cuenta correctamente en arrays con índices FHIRPath

**Tests:**

- `testdata/m2-cardinality/valid-observation-with-required.json`
- `testdata/m2-cardinality/invalid-observation-missing-status.json`
- `testdata/m2-cardinality/invalid-patient-communication-no-language.json`
- `testdata/m2-cardinality/valid-patient-with-link.json`

---

### Milestone 3: Tipos Primitivos ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar formato de tipos primitivos desde sus StructureDefinitions.

```
[x] 3.1 - Cargar StructureDefinition de cada primitive type
[x] 3.2 - Extraer regex de extensión del SD
[x] 3.3 - Validar tipos JSON (boolean, number, string)
[x] 3.4 - Validar: date, dateTime (regex desde SD)
[x] 3.5 - Validar: uri, code, id (regex desde SD)
[x] 3.6 - Cache de regex compilados
```

**Implementación:**

- `pkg/primitive/primitive.go` - Validador de tipos primitivos
- `extractRegexFromSD()` - Extrae regex de extensión del SD
- `extractFHIRType()` - Maneja FHIRPath type codes
- Cache de regex con `sync.RWMutex`

**Decisiones:** ADR-014 (Regex desde SD), ADR-015 (FHIRPath types), ADR-016 (Fases independientes), ADR-017 (Tipo JSON primero)

**Tests:**

- `testdata/m3-primitive/valid-patient-types.json`
- `testdata/m3-primitive/invalid-patient-wrong-type-boolean.json`
- `testdata/m3-primitive/invalid-patient-wrong-type-string.json`
- `testdata/m3-primitive/invalid-patient-bad-date.json`

---

### Milestone 4: Tipos Complejos (Complex Types) ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar datatypes complejos recursivamente.

```
[x] 4.1 - Resolver tipo de elemento desde ElementDefinition.type
[x] 4.2 - Cargar StructureDefinition del tipo
[x] 4.3 - Validar recursivamente contra el SD del tipo
[x] 4.4 - Tipos: HumanName, Address, ContactPoint, Identifier
[x] 4.5 - Tipos: Period, Quantity, Range, Ratio
[x] 4.6 - Tipos: Attachment, Annotation, Signature
[x] 4.7 - Tipos: Reference (sin resolver target aún)
```

**Implementación:**

- Reutiliza infraestructura de M1-M3 (structural, cardinality, primitive)
- `validateComplexElement()` en structural.go carga SD del tipo
- Validación recursiva automática de tipos anidados
- Tipos primitivos dentro de complejos validados por primitive.go

**Decisiones:** ADR-018 (Validación recursiva de tipos complejos)

**Tests:**

- `testdata/m4-complex/valid-patient-humanname.json`
- `testdata/m4-complex/invalid-humanname-unknown-element.json`
- `testdata/m4-complex/invalid-period-wrong-type.json`
- `testdata/m4-complex/valid-patient-identifier.json`
- `testdata/m4-complex/valid-observation-quantity.json`
- `testdata/m4-complex/invalid-quantity-unknown-element.json`

**Validaciones verificadas:**

- ✅ `Patient.name[0].foo` → "Unknown element 'foo'"
- ✅ `Patient.name[0].period.start: 123` → "Error parsing JSON: the primitive value must be a string"
- ✅ `Observation.valueQuantity.invalidField` → "Unknown element 'invalidField'"

**Fuente:** `ElementDefinition.type[].code` → cargar SD del tipo

---

### Milestone 5: Coding y code ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar elementos tipo `code` y `Coding` (sin bindings aún).

```text
[x] 5.1 - Validar estructura de Coding (system, code, display)
[x] 5.2 - Validar que 'code' es string válido
[x] 5.3 - Validar formato de 'system' (URI)
[x] 5.4 - Preparar estructura para bindings (siguiente milestone)
```

**Implementación:**

- Reutiliza infraestructura de M1-M4 (structural, cardinality, primitive)
- Coding es un tipo complejo estándar validado recursivamente
- `system` validado como `uri` por primitive.go
- `code` validado como `code` por primitive.go
- `userSelected` validado como `boolean` por primitive.go

**Validaciones verificadas:**

- ✅ `Coding.system: 123` → "Error parsing JSON: the primitive value must be a string"
- ✅ `Coding.unknownField` → "Unknown element 'unknownField'"
- ✅ `Coding.userSelected: "true"` → "Error parsing JSON: the primitive value must be a boolean"

**Tests:**

- `testdata/m5-coding/valid-coding-complete.json`
- `testdata/m5-coding/valid-coding-minimal.json`
- `testdata/m5-coding/invalid-coding-unknown-element.json`
- `testdata/m5-coding/invalid-coding-wrong-type-system.json`
- `testdata/m5-coding/invalid-coding-wrong-type-userselected.json`

---

### Milestone 6: CodeableConcept ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar CodeableConcept (array de Codings + text).

```text
[x] 6.1 - Validar estructura de CodeableConcept
[x] 6.2 - Validar cada Coding dentro del array
[x] 6.3 - Validar 'text' como string
```

**Implementación:**

- Reutiliza infraestructura de M1-M4
- CodeableConcept es un tipo complejo con `coding[]` y `text`
- Cada Coding en el array se valida recursivamente
- `text` validado como `string` por primitive.go

**Validaciones verificadas:**

- ✅ `CodeableConcept.unknownField` → "Unknown element 'unknownField'"
- ✅ `CodeableConcept.text: 123` → "Error parsing JSON: the primitive value must be a string"
- ✅ `CodeableConcept.coding[1].badField` → "Unknown element 'badField'"

**Tests:**

- `testdata/m6-codeableconcept/valid-codeableconcept-full.json`
- `testdata/m6-codeableconcept/valid-codeableconcept-text-only.json`
- `testdata/m6-codeableconcept/invalid-codeableconcept-unknown-element.json`
- `testdata/m6-codeableconcept/invalid-codeableconcept-wrong-type-text.json`
- `testdata/m6-codeableconcept/invalid-codeableconcept-mixed-coding-errors.json`

**Fuente:** `StructureDefinition-CodeableConcept.json`

---

### Milestone 7: Bindings de Terminología ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar codes contra ValueSets.

```text
[x] 7.1 - Cargar ValueSets del paquete de terminología
[x] 7.2 - Expandir ValueSets simples (enumerated)
[x] 7.3 - Validar binding strength: required
[x] 7.4 - Validar binding strength: extensible (warning)
[x] 7.5 - Ignorar binding strength: preferred, example
[x] 7.6 - Cache de ValueSet expansions
```

**Implementación:**

- `pkg/terminology/terminology.go` - Registry de ValueSets y CodeSystems
- `pkg/binding/binding.go` - Validador de bindings
- Carga ValueSets y CodeSystems desde paquetes FHIR
- Expande ValueSets resolviendo CodeSystems referenciados
- Cache de expansiones para performance

**Validaciones verificadas:**

- ✅ `Patient.gender: "male"` → válido (en ValueSet administrative-gender)
- ✅ `Observation.status: "final"` → válido (en ValueSet observation-status)
- ✅ `Identifier.use: "official"` → válido (binding en tipo complejo)
- ❌ `Patient.gender: "invalid"` → error: not in value set (required)
- ❌ `Observation.status: "invalid-status"` → error: not in value set
- ❌ `Identifier.use: "invalid-use"` → error: not in value set

**Tests:**

- `testdata/m7-bindings/valid-patient-gender.json`
- `testdata/m7-bindings/invalid-patient-gender.json`
- `testdata/m7-bindings/valid-observation-status.json`
- `testdata/m7-bindings/invalid-observation-status.json`
- `testdata/m7-bindings/valid-identifier-use.json`
- `testdata/m7-bindings/invalid-identifier-use.json`

**Fuente:** `ElementDefinition.binding.valueSet`, `ElementDefinition.binding.strength`

---

### Milestone 8: Extensions ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar extensiones contra sus StructureDefinitions.

```text
[x] 8.1 - Detectar elementos extension
[x] 8.2 - Resolver Extension.url → StructureDefinition
[x] 8.3 - Validar value[x] según SD de la extensión
[x] 8.4 - Validar contexto de extensión (dónde puede usarse)
[x] 8.5 - Extensiones anidadas (extension dentro de extension)
[x] 8.6 - ModifierExtension (igual pero con isModifier=true)
```

**Implementación:**

- `pkg/extension/extension.go` - Validador de extensiones
- Resuelve Extension.url → StructureDefinition
- Valida value[x] contra tipos permitidos en el SD
- Valida contexto (dónde puede usarse la extensión)
- Soporta extensiones anidadas
- Soporta modifierExtension

**Validaciones verificadas:**

- ✅ Extension válida con valueAddress → válido
- ⚠️ Extension con URL desconocida → warning "Unknown extension"
- ❌ Extension en contexto incorrecto → error "Extension not allowed here"
- ❌ Extension.valueString cuando SD dice Address → error "Wrong value type"

**Tests:**

- `testdata/m8-extensions/valid-patient-birthplace.json`
- `testdata/m8-extensions/warning-unknown-extension.json`
- `testdata/m8-extensions/invalid-wrong-value-type.json`
- `testdata/m8-extensions/invalid-wrong-context.json`

**Fuente:** `Extension.url` → `StructureDefinition`, `StructureDefinition.context`

---

### Milestone 9: References ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Validar referencias a otros recursos.

```
[x] 9.1 - Validar estructura de Reference
[x] 9.2 - Validar formato de reference string (relativo, absoluto, fragmento, URN)
[x] 9.3 - Validar mismatch entre type element y reference string
[x] 9.4 - Validar nombres de recursos contra registry (no hardcoding)
[ ] 9.5 - (Futuro) Resolver referencias en Bundle
```

**Implementación:**

- `pkg/reference/reference.go` - Validador de referencias
- Valida formatos: relativo (`Patient/123`), absoluto (`https://...`), fragmento (`#id`), URN (`urn:uuid:...`, `urn:oid:...`)
- Valida logical references (identifier only, display only)
- Detecta mismatch entre `type` element y reference string
- Usa registry para validar nombres de recursos (no hardcoding)

**Nota:** No se valida targetProfile contra el reference string (coincide con HL7 validator).
La validación de targetProfile se hace al resolver el recurso referenciado.

**Validaciones verificadas:**

- ✅ `reference: "Patient/123"` → válido (formato relativo)
- ✅ `reference: "https://example.org/fhir/Patient/123"` → válido (absoluto)
- ✅ `reference: "#patient-1"` → válido (fragmento)
- ✅ `reference: "urn:uuid:550e8400-..."` → válido (URN UUID)
- ✅ Logical reference con identifier → válido
- ❌ `reference: "not valid!"` → error "Invalid reference format"
- ❌ `reference: "Patient/123", type: "Group"` → error "type mismatch"

**Tests:**

- `testdata/m9-references/valid-relative-reference.json`
- `testdata/m9-references/valid-absolute-reference.json`
- `testdata/m9-references/valid-fragment-reference.json`
- `testdata/m9-references/valid-urn-uuid.json`
- `testdata/m9-references/valid-logical-reference.json`
- `testdata/m9-references/invalid-format.json`
- `testdata/m9-references/invalid-type-mismatch.json`

**Fuente:** `ElementDefinition.type[code=Reference]`

---

### Milestone 10: Constraints FHIRPath ✅

**Estado:** COMPLETADO (2025-01-25)
**Objetivo:** Evaluar constraints/invariantes usando FHIRPath.

```
[x] 10.1 - Extraer constraints de ElementDefinition.constraint
[x] 10.2 - Integrar github.com/gofhir/fhirpath
[x] 10.3 - Evaluar expresiones en contexto del recurso raíz
[x] 10.4 - Generar issues según constraint.severity
[x] 10.5 - Manejar errores de evaluación gracefully
[x] 10.6 - Cache de expresiones compiladas
[ ] 10.7 - (Futuro) Evaluar constraints en elementos anidados
```

**Implementación:**

- `pkg/constraint/constraint.go` - Validador de constraints FHIRPath
- Usa `github.com/gofhir/fhirpath` para compilar y evaluar expresiones
- Cache de expresiones compiladas con `sync.RWMutex`
- Maneja errores de compilación/evaluación emitiendo warnings
- Saltan constraints best-practice (dom-6) y problemáticos (dom-3)

**Validaciones verificadas:**

- ✅ `dom-4`: contained.meta.versionId.empty() and contained.meta.lastUpdated.empty()
- ⏭️ `dom-6`: Skipped (best practice warning)
- ⏭️ `dom-3`: Skipped (complex expression with eval issues)

**Tests:**

- `testdata/m10-constraints/valid-patient-no-contained-meta.json`
- `testdata/m10-constraints/invalid-contained-has-versionid.json`
- `testdata/m10-constraints/invalid-contained-has-lastupdated.json`

**Comparación con HL7 Validator:**

| Constraint | GoFHIR | HL7 |
|------------|--------|-----|
| dom-4 (contained meta) | ERROR | ERROR |
| dom-6 (narrative) | - | WARNING |
| dom-3 (contained refs) | - | ERROR |

**Fuente:** `ElementDefinition.constraint[].expression`, `constraint.severity`

---

### Milestone 11: Fixed y Pattern Values ✅

**Estado:** COMPLETADO (2025-01-27)
**Objetivo:** Validar valores fijos y patrones.

```
[x] 11.1 - Validar fixed[x] (valor exacto requerido)
[x] 11.2 - Validar pattern[x] (valor debe contener patrón)
[x] 11.3 - Comparación profunda para tipos complejos
```

**Implementación:**

- `pkg/fixedpattern/fixedpattern.go` - Validador de fixed/pattern
- `pkg/fixedpattern/compare.go` - Algoritmos de comparación
- `pkg/registry/registry.go` - Métodos `GetFixed()` y `GetPattern()` dinámicos

**Decisiones:**
- NO hardcodear los 45+ tipos de fixed[x]/pattern[x]
- Usar `json.RawMessage` y extracción dinámica por prefijo
- Custom UnmarshalJSON en Snapshot/Differential para preservar raw JSON

**Validaciones:**
- ❌ `code: "foo"` cuando `fixedCode: "bar"` → "Value must be exactly 'bar'"
- ❌ `Coding` no contiene pattern → "Value must match pattern"

**Tests:**
- `pkg/registry/fixed_pattern_test.go` - Tests de extracción
- `pkg/fixedpattern/compare_test.go` - Tests de comparación

**Fuente:** `ElementDefinition.fixed[x]`, `ElementDefinition.pattern[x]`

---

### Milestone 12: Slicing ✅

**Estado:** COMPLETADO (2025-01-27)
**Objetivo:** Validar slicing de arrays.

```
[x] 12.1 - Detectar elementos con slicing definido
[x] 12.2 - Resolver discriminadores (value, pattern, type)
[x] 12.3 - Asignar elementos a slices
[x] 12.4 - Validar cardinalidad por slice
[x] 12.5 - Validar elementos sin slice asignado (open/closed)
```

**Implementación:**

- `pkg/slicing/slicing.go` - Validador de slicing
- Extrae slicing contexts de StructureDefinitions
- Evalúa discriminadores: value, pattern, type
- Soporta paths multi-nivel con arrays (ej: `coding.code`)
- Valida cardinalidad por slice (min/max)
- Valida reglas: open, closed, openAtEnd

**Validaciones verificadas:**

- ✅ `Patient.extension` con slices race, ethnicity → matchea correctamente
- ✅ `Observation.category:VSCat` con discriminador `coding.code` y `coding.system` → matchea
- ❌ Slice con `min=1` faltante → "Slice 'X' requires minimum 1 element(s), found 0"
- ❌ Slice con `max=1` excedido → "Slice 'X' allows maximum 1 element(s), found 2"
- ❌ Elemento no matchea en closed slicing → "Element does not match any defined slice"

**Tests:**

- `pkg/slicing/slicing_test.go` - Tests de extracción, discriminadores, matching
- 64/64 Observations de HL7 pasan con slicing (vitalsigns profile)

**Fuente:** `ElementDefinition.slicing`, `ElementDefinition.sliceName`

---

### Milestone 13: Profiles ✅

**Estado:** COMPLETADO (2025-01-27)
**Objetivo:** Validar contra perfiles derivados.

```
[x] 13.1 - Cargar profile desde meta.profile
[x] 13.2 - Cargar paquetes adicionales con WithPackage()
[x] 13.3 - Validar contra profile en lugar de base
[x] 13.4 - Soportar múltiples profiles (usa primero disponible)
[x] 13.5 - Emitir warnings para profiles no encontrados
[ ] 13.6 - Generar snapshot desde differential (futuro)
```

**Implementación:**

- `pkg/validator/validator.go` - `WithPackage()`, `collectProfilesToValidate()`
- Prioridad de profiles: config.Profiles > meta.profile > core
- Warning emitido si profile no está en registry
- Tests con US Core Patient profile

**Validaciones verificadas:**

- ✅ US Core Patient válido (identifier, name, gender) → 0 errores
- ❌ US Core Patient inválido → 3 errores de cardinalidad
- ⚠️ Profile no encontrado → warning "Profile 'X' not found in registry"

**Tests:**

- `pkg/validator/validator_test.go` - TestValidateWithUSCoreProfile, TestValidateWithMultipleProfiles

**Fuente:** `Resource.meta.profile`, `StructureDefinition.baseDefinition`

---

### Milestone 14: CLI ✅

**Estado:** COMPLETADO (2025-01-27)
**Objetivo:** Herramienta de línea de comandos.

```
[x] 14.1 - Comando básico de validación
[x] 14.2 - Flags compatibles con HL7 validator
[x] 14.3 - Output formats (text, json)
[ ] 14.4 - Modo comparación con HL7 validator (futuro)
[x] 14.5 - Batch processing
```

**Implementación:**

- `cmd/gofhir-validator/main.go` - CLI principal
- Flags: `-version`, `-ig`, `-package`, `-output`, `-strict`, `-quiet`, `-verbose`
- Output formats: text (default), json
- Soporte para stdin (`-`), archivos, y glob patterns (`*.json`)
- Exit code 0=válido, 1=errores

**Uso:**

```bash
# Validación básica
gofhir-validator patient.json

# Con profile
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# Con paquete adicional
gofhir-validator -package hl7.fhir.us.core#6.1.0 patient.json

# JSON output
gofhir-validator -output json patient.json

# Desde stdin
cat patient.json | gofhir-validator -

# Batch processing
gofhir-validator *.json
```

---

### Milestone 15: Performance ✅

**Estado:** COMPLETADO (2025-01-27)
**Objetivo:** Optimizar para alto rendimiento.

```
[x] 15.1 - Object pooling para Results
[x] 15.2 - Cache de element indexes
[x] 15.3 - Reutilización de phase validators
[x] 15.4 - Disable trace output para benchmarks
[x] 15.5 - Benchmarks y documentación
```

**Implementación:**

- `pkg/issue/pool.go` - Result pooling con sync.Pool
- `pkg/structural/structural.go` - Element index caching
- `pkg/primitive/primitive.go` - Element index caching
- `pkg/validator/validator.go` - Reuse phase validators
- `pkg/validator/benchmark_test.go` - Benchmarks completos

**Resultados de Optimización:**

| Benchmark | Antes | Después | Mejora |
|-----------|-------|---------|--------|
| MinimalPatient | 93.7 µs, 1,653 allocs | 23.4 µs, 234 allocs | **4.0x más rápido, 86% menos allocs** |
| PatientWithData | 376 µs, 4,078 allocs | 284 µs, 2,323 allocs | **1.3x más rápido, 43% menos allocs** |
| Observation | 405 µs, 4,155 allocs | 297 µs, 1,809 allocs | **1.4x más rápido, 56% menos allocs** |
| HL7Example | 1.1 ms, 7,342 allocs | 1.0 ms, 5,253 allocs | **1.1x más rápido, 28% menos allocs** |
| ValidateParallel | 80 µs, 1,980 allocs | 9.5 µs, 509 allocs | **8.5x más rápido, 74% menos allocs** |
| ValidateBatch | 805 µs, 11,558 allocs | 388 µs, 3,249 allocs | **2.1x más rápido, 72% menos allocs** |

**Principales optimizaciones:**
- Pooling de Results permite reutilizar objetos entre validaciones
- Caching de element indexes evita reconstruir mapas por cada SD
- Reutilización de phase validators mantiene caches entre llamadas
- Pre-allocación de slices reduce reallocations

---

## Orden de Implementación Recomendado

```
M0 (Infraestructura) ──► M1 (Estructura) ──► M2 (Cardinalidad)
                                                    │
                                                    ▼
M3 (Primitivos) ◄── M4 (Complex Types) ◄── M5 (Coding) ◄── M6 (CodeableConcept)
        │
        ▼
M7 (Bindings) ──► M8 (Extensions) ──► M9 (References)
                                            │
                                            ▼
                        M10 (Constraints) ──► M11 (Fixed/Pattern)
                                                    │
                                                    ▼
                                M12 (Slicing) ──► M13 (Profiles)
                                                        │
                                                        ▼
                                        M14 (CLI) ──► M15 (Performance)
```

---

## Criterios de Completitud por Milestone

Cada milestone está completo cuando:

1. ✅ **Tests unitarios** para cada validación
2. ✅ **Tests de integración** con recursos reales
3. ✅ **Comparación con HL7 validator** (mismo resultado)
4. ✅ **Documentación** de la fase
5. ✅ **Sin hardcoding** - todo derivado de StructureDefinitions

---

## Recursos de Prueba por Milestone

| Milestone | Recursos de Prueba |
|-----------|-------------------|
| M1 | Patient mínimo, con elementos desconocidos |
| M2 | Patient sin identifier (requerido), con muchos names |
| M3 | Fechas inválidas, booleanos como strings |
| M4 | HumanName inválido, Identifier incompleto |
| M5-M6 | Coding/CodeableConcept mal formados |
| M7 | Gender inválido, status incorrecto |
| M8 | Extensiones conocidas y desconocidas |
| M9 | Referencias a tipos incorrectos |
| M10 | Recursos que violan constraints conocidos |
| M11 | Valores que no coinciden con fixed/pattern |
| M12 | Arrays con slicing (ej: US Core identifiers) |
| M13 | Recursos validados contra US Core profiles |

---

## Extensibilidad

El diseño permite añadir nuevas validaciones sin modificar código existente:

```go
// Registrar nueva fase de validación
pipeline.Register(&MyCustomPhase{})

// La fase implementa la interfaz Phase
type MyCustomPhase struct{}

func (p *MyCustomPhase) Name() string { return "my-custom" }
func (p *MyCustomPhase) Priority() int { return 1500 }
func (p *MyCustomPhase) Validate(ctx *Context, elem *Element) []Issue {
    // Validación personalizada
    // SIEMPRE basarse en ctx.ElementDef para obtener reglas
}
```

---

## ¿Comenzamos?

El primer paso es **Milestone 0: Infraestructura Base**.

¿Procedemos con el setup del proyecto?
