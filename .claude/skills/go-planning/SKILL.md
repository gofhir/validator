---
name: go-planning
description: Planificación de features y tareas en Go. Usar al iniciar una nueva feature, diseñar APIs, o planificar refactorizaciones.
allowed-tools: Read, Glob, Grep
---

# Skill: go-planning

Planificación de features y tareas de desarrollo en Go.

## Cuándo usar este skill

- Al iniciar una nueva feature o módulo
- Al diseñar una API pública
- Al planificar refactorizaciones mayores
- Cuando el usuario pide un plan de implementación

## Proceso de Planificación

### 1. Análisis de Requisitos

```markdown
## Feature: [Nombre]

### Objetivo
[Descripción clara del objetivo]

### Requisitos Funcionales
- [ ] RF1: ...
- [ ] RF2: ...

### Requisitos No Funcionales
- [ ] RNF1: Performance - ...
- [ ] RNF2: Thread-safety - ...
- [ ] RNF3: Extensibilidad - ...

### Restricciones
- Debe ser compatible con Go 1.21+
- No debe romper la API existente
- ...
```

### 2. Diseño de API Pública

Antes de implementar, definir la API que verá el usuario:

```go
// Definir primero cómo se USARÁ el código
// Esto guía todo el diseño

// Uso simple (caso común)
result, err := package.DoSomething(input)

// Uso con opciones (casos avanzados)
result, err := package.DoSomethingWithOptions(input,
    package.WithTimeout(10*time.Second),
    package.WithLogger(logger),
)

// Uso con builder (configuración compleja)
processor := package.NewProcessor(required).
    WithCache(cache).
    WithRetries(3)
result, err := processor.Process(ctx, input)
```

### 3. Identificar Interfaces

Preguntarse:
- ¿Qué dependencias externas necesita? → Interfaces
- ¿Qué puede querer mockear el usuario? → Interfaces
- ¿Qué podría cambiar en el futuro? → Interfaces

### 4. Definir Estructura de Archivos

```
feature/
├── feature.go          # API pública, constructores
├── options.go          # Opciones funcionales
├── interfaces.go       # Interfaces públicas
├── types.go            # Tipos exportados
├── errors.go           # Errores personalizados
├── feature_test.go     # Tests unitarios
├── benchmark_test.go   # Benchmarks
├── doc.go              # Documentación del paquete
└── internal/           # Implementación interna
```

### 5. Plan de Implementación

```markdown
## Plan de Implementación

### Fase 1: Fundamentos
1. [ ] Crear estructura de archivos
2. [ ] Definir interfaces públicas
3. [ ] Implementar tipos básicos
4. [ ] Crear constructores

### Fase 2: Core
5. [ ] Implementar lógica principal
6. [ ] Añadir manejo de errores
7. [ ] Implementar opciones funcionales

### Fase 3: Testing
8. [ ] Tests unitarios (>80% coverage)
9. [ ] Tests de integración
10. [ ] Benchmarks para hot paths

### Fase 4: Pulido
11. [ ] Documentación
12. [ ] Ejemplos de uso
13. [ ] Code review
```

### 6. Checklist de Diseño

```markdown
## Checklist de Diseño

### API
- [ ] ¿La API es intuitiva para el usuario?
- [ ] ¿Los nombres son claros y consistentes?
- [ ] ¿Se sigue el principio de menor sorpresa?
- [ ] ¿El caso común es simple, el avanzado posible?

### Interfaces
- [ ] ¿Las interfaces son pequeñas (1-3 métodos)?
- [ ] ¿Se definen donde se usan, no donde se implementan?
- [ ] ¿Permiten testing fácil con mocks?

### Extensibilidad
- [ ] ¿Se puede extender sin modificar código existente?
- [ ] ¿Las opciones funcionales permiten futuras extensiones?
- [ ] ¿Los cambios internos no romperán usuarios?

### Performance
- [ ] ¿Se identificaron los hot paths?
- [ ] ¿Se necesita caching? ¿Pooling?
- [ ] ¿Hay operaciones que pueden ser concurrentes?
```

## Template de Plan

```markdown
# Plan: [Feature Name]

## Resumen
[1-2 oraciones describiendo la feature]

## Motivación
[Por qué se necesita esta feature]

## API Propuesta

### Uso Básico
\`\`\`go
// Código de ejemplo
\`\`\`

### Uso Avanzado
\`\`\`go
// Código de ejemplo con opciones
\`\`\`

## Diseño Técnico

### Interfaces
\`\`\`go
type Interface interface {
    Method(ctx context.Context) error
}
\`\`\`

## Plan de Implementación
1. [ ] Paso 1
2. [ ] Paso 2
...

## Referencias FHIR (si aplica)
- Fuente: [Nombre]
- URL: [URL]
- Fecha: [YYYY-MM-DD]
```
