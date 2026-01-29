# CLAUDE.md - Guía de Desarrollo para el Validador FHIR Go

## Principios Fundamentales (CRÍTICOS)

### 1. Fidelidad al Estándar FHIR

**SIEMPRE** confirmar cualquier implementación contra:

1. **Especificación oficial FHIR**: https://hl7.org/fhir/R4/
2. **Validadores de referencia para comparación**:
   - **HAPI FHIR Validator** (Java): El estándar de facto en la industria
   - **HL7 Official Validator** (Java): Validador oficial del consorcio HL7
   - **Firely SDK** (.NET): Implementación de referencia en .NET
   - **Microsoft FHIR Server**: Implementación enterprise de Microsoft
   - **Inferno Framework**: Para testing de conformance

Ante cualquier duda sobre comportamiento esperado:
```bash
# Comparar con HAPI FHIR
java -jar validator_cli.jar resource.json -version 4.0.1

# Verificar contra la especificación
# https://hl7.org/fhir/R4/[resource].html
# https://hl7.org/fhir/R4/elementdefinition.html
```

### 2. Todo Desde StructureDefinitions (NO HARDCODING)

**NUNCA** hardcodear:
- Nombres de elementos o paths
- Cardinalidades
- Tipos de datos permitidos
- Restricciones de binding
- Constraints/invariantes
- Slicing rules
- Valores fijos o patrones

**SIEMPRE** derivar del StructureDefinition:
- Cargar desde `ElementDefinition.path`
- Obtener tipos de `ElementDefinition.type`
- Cardinalidad de `ElementDefinition.min` / `ElementDefinition.max`
- Bindings de `ElementDefinition.binding`
- Constraints de `ElementDefinition.constraint`
- Fixed/Pattern de `ElementDefinition.fixed[x]` / `ElementDefinition.pattern[x]`

```go
// CORRECTO: Derivar del StructureDefinition
for _, elem := range profile.Snapshot.Elements {
    if elem.Min > 0 {
        // Validar cardinalidad mínima
    }
}

// INCORRECTO: Hardcodear
if path == "Patient.identifier" {
    // Asumir que es requerido
}
```

### 3. Perfiles Anidados y Resolución Dinámica

El validador debe:
1. Resolver perfiles base (`StructureDefinition.baseDefinition`)
2. Cargar perfiles de extensiones (`Extension.url` → StructureDefinition)
3. Resolver perfiles de tipos (`ElementDefinition.type.profile`)
4. Soportar perfiles de perfiles (cadenas de derivación)
5. Generar snapshots cuando solo hay differential

```go
// Cadena de resolución
Profile → baseDefinition → baseDefinition → ... → Base Resource
```

### 4. Agnóstico de Versión

La librería NO debe acoplarse a ninguna versión específica de FHIR:
- Los validadores deben funcionar con cualquier StructureDefinition válido
- No asumir existencia de elementos específicos de versión
- Permitir cargar definiciones de R4, R4B, R5, etc.
- La lógica de validación es genérica, los datos vienen del profile

---

## Estructura del Proyecto

```
validator/
├── engine/          # Orquestador principal de validación
├── pipeline/        # Framework de ejecución de fases
├── phase/           # Implementaciones de validación (15+ fases)
├── service/         # Interfaces pequeñas y componibles
├── loader/          # Carga y conversión de perfiles
├── walker/          # Tree traversal con contexto de tipos
├── cache/           # Cache LRU genérico
├── pool/            # Object pooling para performance
├── stream/          # Validación streaming de bundles
├── worker/          # Worker pool para batch processing
└── terminology/     # Validación de terminología
```

---

## Skills Disponibles

Usar la skill apropiada según el contexto de la tarea:

### Planificación y Arquitectura

| Skill | Cuándo Usar | Invocación |
|-------|-------------|------------|
| `/go-planning` | Iniciar nueva feature, diseñar APIs, planificar refactorizaciones | Planificación de tareas |
| `/go-architecture` | Diseñar nuevos módulos, definir capas, organizar código | Decisiones arquitectónicas |
| `/go-project-structure` | Crear proyectos, organizar código existente | Estructura de módulos |

### Implementación

| Skill | Cuándo Usar | Invocación |
|-------|-------------|------------|
| `/go-interfaces` | Definir APIs extensibles, preparar para testing, bajo acoplamiento | Diseño de contratos |
| `/go-methods` | Definir métodos, decidir receptor valor vs puntero | Diseño de tipos |
| `/go-constructors` | Crear constructores, opciones funcionales, builders | APIs configurables |
| `/go-composition` | Extender tipos, combinar comportamientos, embedding | Extensibilidad |
| `/go-patterns` | Elegir patrones, diseñar sistemas extensibles | Patrones de diseño |
| `/go-error-handling` | Diseñar errores para APIs, propagar errores, tipos custom | Manejo de errores |

### Concurrencia y Performance

| Skill | Cuándo Usar | Invocación |
|-------|-------------|------------|
| `/go-concurrency` | Diseñar código concurrente, manejar recursos compartidos | Goroutines, channels |
| `/go-performance` | Optimizar hot paths, reducir allocations, mejorar latencia | Optimización |
| `/go-caching` | Implementar caches, reducir allocations, pooling | Patrones de cache |

### Testing y Calidad

| Skill | Cuándo Usar | Invocación |
|-------|-------------|------------|
| `/go-testing` | Escribir tests unitarios, diseñar casos de prueba | Tests unitarios |
| `/go-integration-testing` | Testear múltiples componentes, validar flujos e2e | Tests integración |
| `/go-benchmarking` | Medir rendimiento, comparar implementaciones | Benchmarks |
| `/go-code-review` | Revisar PRs, verificar calidad, identificar problemas | Code review |

### Conformance y Comparación

| Skill | Cuándo Usar | Invocación |
|-------|-------------|------------|
| `/validator-compare` | Comparar con HL7 validator, verificar conformance, debuggear discrepancias | Comparación de validadores |

### Mantenimiento

| Skill | Cuándo Usar | Invocación |
|-------|-------------|------------|
| `/go-refactoring` | Mejorar estructura, eliminar duplicación | Refactorización |
| `/go-documentation` | Documentar paquetes, APIs públicas, ejemplos | Documentación |
| `/go-dev-workflow` | Configurar ambiente, comandos make, git hooks | Flujo de trabajo |

---

## Guías de Implementación

### Agregar Nueva Fase de Validación

1. Usar `/go-architecture` para diseñar la fase
2. Implementar la interfaz `Phase`:
```go
type Phase interface {
    Name() string
    Priority() Priority
    Execute(ctx *Context) error
}
```
3. Registrar en el pipeline
4. Usar `/go-testing` para tests unitarios

### Agregar Nuevo Servicio

1. Usar `/go-interfaces` para diseñar la interfaz (máximo 1-2 métodos)
2. Seguir el patrón de interfaces pequeñas del proyecto:
```go
// CORRECTO: Interface pequeña
type CodeValidator interface {
    ValidateCode(ctx context.Context, system, code string, vs *ValueSet) (bool, error)
}

// EVITAR: Interface grande
type TerminologyService interface {
    ValidateCode(...)
    ValidateCoding(...)
    ValidateCodeableConcept(...)
    ExpandValueSet(...)
    // ... muchos métodos
}
```

### Optimización de Performance

1. Usar `/go-benchmarking` para medir estado actual
2. Usar `/go-performance` para identificar optimizaciones
3. Usar `/go-caching` para patrones de cache/pooling
4. Verificar con benchmarks después de cambios

### Manejo de Errores FHIR

Seguir la estructura de issues FHIR:
```go
Issue{
    Severity:    IssueSeverityError,
    Code:        IssueTypeValue,
    Diagnostics: "mensaje descriptivo",
    Expression:  []string{"Patient.birthDate"},
}
```

---

## Verificación de Conformance

Antes de cada release, verificar contra:

### Casos de Prueba Estándar

1. **Ejemplos oficiales FHIR**: https://hl7.org/fhir/R4/downloads.html
2. **Test cases de HAPI**: Comparar resultados
3. **Synthea data**: Datos sintéticos realistas
4. **US Core profiles**: Perfiles de uso común

### Checklist de Validación

- [ ] Cardinalidad (min/max) desde ElementDefinition
- [ ] Tipos permitidos desde ElementDefinition.type
- [ ] Bindings de terminología (required, extensible, preferred)
- [ ] Constraints FHIRPath desde ElementDefinition.constraint
- [ ] Fixed/Pattern values
- [ ] Slicing rules
- [ ] Extensions y sus perfiles
- [ ] Referencias y targets permitidos
- [ ] Elementos desconocidos

---

## CLI - gofhir-validator

Herramienta de línea de comandos comparable con el HL7 FHIR Validator.

### Instalación

```bash
go install github.com/gofhir/validator/cmd/gofhir-validator@latest
```

### Uso Básico

```bash
# Validar recurso
gofhir-validator patient.json

# Especificar versión FHIR
gofhir-validator -version r4 patient.json

# Validar contra profile
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json

# Output JSON
gofhir-validator -output json patient.json

# Desde stdin
cat patient.json | gofhir-validator -
```

### Comparación con HL7 Validator

| gofhir-validator | HL7 validator | Descripción |
|------------------|---------------|-------------|
| `-version r4` | `-version 4.0.1` | Versión FHIR |
| `-ig <url>` | `-ig <url>` | Profile/IG |
| `-output json` | `-output` | Formato salida |
| `-tx n/a` | `-tx n/a` | Deshabilitar terminología |
| `-strict` | - | Warnings como errors |

---

## Comandos de Desarrollo

```bash
# Tests
make test                    # Todos los tests
go test ./phase/... -v       # Tests de una fase específica

# Benchmarks
go test -bench=. -benchmem ./...

# Lint
golangci-lint run

# Compilar CLI
go build -o gofhir-validator ./cmd/gofhir-validator/
```

---

## Referencias

- [FHIR R4 Specification](https://hl7.org/fhir/R4/)
- [StructureDefinition](https://hl7.org/fhir/R4/structuredefinition.html)
- [ElementDefinition](https://hl7.org/fhir/R4/elementdefinition.html)
- [Validation Rules](https://hl7.org/fhir/R4/validation.html)
- [FHIRPath](https://hl7.org/fhirpath/)
- [HAPI FHIR Validator](https://hapifhir.io/hapi-fhir/docs/validation/introduction.html)
