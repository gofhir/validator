# Plan: M13 - Profiles

## Resumen

Mejorar el soporte de validación contra perfiles derivados (profiles), incluyendo múltiples profiles, generación de snapshots desde differential, y mejor manejo de errores.

## Motivación

Los perfiles FHIR permiten:
- Restringir recursos base con reglas más estrictas
- Añadir extensiones requeridas
- Definir slicing específico
- Establecer bindings de terminología más restrictivos

El validador actual ya soporta:
- ✅ Leer `meta.profile` del recurso
- ✅ Cargar profile desde registry
- ✅ Validar todas las fases contra el profile

Lo que falta para M13:
- [ ] Validar contra múltiples profiles (todos en meta.profile)
- [ ] Generar snapshot desde differential si falta
- [ ] Mejor manejo cuando profile no está disponible
- [ ] Emitir issue informativo sobre profile usado

## Análisis del Estado Actual

### Flujo actual (validator.go:201-250)

```go
// 1. Obtener URL del profile
profileURL := registry.GetSDForResource(resourceType)  // Default: core

// 2. Si hay meta.profile, usar el primero
if resourceInfo.Meta != nil && len(resourceInfo.Meta.Profile) > 0 {
    profileURL = resourceInfo.Meta.Profile[0]  // TODO: Support multiple profiles
}

// 3. Buscar en registry
sd := v.registry.GetByURL(profileURL)

// 4. Si no se encuentra, fallback a core
if sd == nil && isCustomProfile {
    profileURL = registry.GetSDForResource(resourceType)
    sd = v.registry.GetByURL(profileURL)
}
```

### Limitaciones actuales

1. **Solo primer profile**: Si hay múltiples profiles en meta.profile, solo valida contra el primero
2. **Fallback silencioso**: Si profile no existe, usa core sin error (solo warning en log)
3. **No genera snapshot**: Si profile solo tiene differential, no funciona
4. **Sin información**: No emite issue informativo sobre qué profile se usó

## Diseño Técnico

### Cambio 1: Validar contra múltiples profiles

```go
// En Validate(), después de obtener el SD principal
profiles := v.getProfilesToValidate(resourceInfo)

for _, profileURL := range profiles {
    sd := v.registry.GetByURL(profileURL)
    if sd == nil {
        // Emitir warning y continuar
        result.AddWarning(issue.CodeNotFound,
            fmt.Sprintf("Profile '%s' not found", profileURL))
        continue
    }

    // Ejecutar todas las fases contra este profile
    v.validateAgainstProfile(resource, sd, result)
}
```

### Cambio 2: Generar snapshot desde differential

Si un profile no tiene snapshot pero tiene differential + baseDefinition:

```go
func (r *Registry) EnsureSnapshot(sd *StructureDefinition) error {
    if sd.Snapshot != nil {
        return nil // Ya tiene snapshot
    }

    if sd.Differential == nil || sd.BaseDefinition == "" {
        return fmt.Errorf("cannot generate snapshot without differential and base")
    }

    // 1. Cargar base
    baseSD := r.GetByURL(sd.BaseDefinition)
    if baseSD == nil {
        return fmt.Errorf("base definition not found: %s", sd.BaseDefinition)
    }

    // 2. Recursivamente asegurar que base tiene snapshot
    if err := r.EnsureSnapshot(baseSD); err != nil {
        return err
    }

    // 3. Aplicar differential sobre base
    sd.Snapshot = applyDifferential(baseSD.Snapshot, sd.Differential)
    return nil
}
```

### Cambio 3: Issue informativo sobre profile usado

```go
// Al inicio de validación, emitir información sobre profiles
for _, profileURL := range profiles {
    result.AddIssue(issue.Issue{
        Severity:    issue.SeverityInformation,
        Code:        issue.CodeInformational,
        Diagnostics: fmt.Sprintf("Validating against profile: %s", profileURL),
    })
}
```

## API Propuesta

### Sin cambios en API pública

La API actual (`Validate(ctx, resource)`) no cambia. Los cambios son internos.

### Nuevas opciones (opcional)

```go
// Opción para requerir que profiles existan
validator.WithStrictProfiles(true)  // Error si profile no existe

// Opción para ignorar meta.profile
validator.WithIgnoreMetaProfile(true)  // Solo usar profiles configurados
```

## Plan de Implementación

### Fase 1: Múltiples Profiles (1 PR) ✅

1. [x] Refactorizar Validate() para iterar sobre profiles
2. [x] Crear `collectProfilesToValidate()` que combina config.Profiles + meta.profile
3. [x] Emitir warnings para profiles no encontrados
4. [x] Tests con recursos que declaran múltiples profiles

### Fase 2: Carga de Paquetes Adicionales ✅

5. [x] Implementar `WithPackage(name, version)` option
6. [x] Cargar paquetes adicionales en `New()`
7. [x] Tests con US Core profiles

### Fase 3: Validar contra TODOS los profiles ✅

8. [x] Refactorizar Validate() para validar contra TODOS los profiles (no solo el primero)
9. [x] Extraer `validateAgainstProfile()` helper method
10. [x] Emitir issue informativo cuando se valida contra múltiples profiles
11. [x] Tests con múltiples profiles

### Fase 4: Generación de Snapshot (Futuro)

12. [ ] Implementar `EnsureSnapshot()` en Registry
13. [ ] Implementar `applyDifferential()` para merge de elementos
14. [ ] Llamar EnsureSnapshot antes de validar
15. [ ] Tests con profiles que solo tienen differential

### Fase 5: Mejor Manejo de Errores (Futuro)

16. [ ] Opción WithStrictProfiles
17. [ ] Warning cuando se usa fallback a core
18. [ ] Tests de escenarios de error

### Fase 4: Integración (1 PR)

13. [ ] Tests con US Core profiles reales
14. [ ] Comparar resultados con HL7 validator
15. [ ] Documentar comportamiento

## Casos de Prueba

### Múltiples Profiles

```json
{
  "resourceType": "Patient",
  "meta": {
    "profile": [
      "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient",
      "http://example.org/StructureDefinition/my-patient"
    ]
  }
}
```

| Caso | Esperado |
|------|----------|
| Ambos profiles válidos | Validar contra ambos |
| Primer profile no existe | Warning, validar contra segundo |
| Ningún profile existe | Warnings, validar contra core |

### Snapshot Generation

```json
// Profile solo con differential
{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/profile",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Patient",
  "differential": {
    "element": [
      {"path": "Patient.identifier", "min": 1}
    ]
  }
  // No tiene snapshot
}
```

| Caso | Esperado |
|------|----------|
| Profile sin snapshot, base disponible | Generar snapshot y validar |
| Profile sin snapshot, base no disponible | Error informativo |

## Mensajes de Error

```
Information: Validating against profile: http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient

Warning: Profile 'http://example.org/unknown-profile' not found, skipping

Error: Profile 'http://example.org/required-profile' not found (strict mode enabled)
```

## Estructura de Archivos

No se necesitan nuevos archivos. Cambios en:
- `pkg/validator/validator.go` - Lógica de múltiples profiles
- `pkg/registry/registry.go` - EnsureSnapshot, applyDifferential

## Limitaciones Conocidas (M13)

- No soporta profiles con extensiones complejas en differential
- Generación de snapshot simplificada (no implementa toda la lógica FHIR)
- No valida que el profile derive del tipo correcto

## Referencias

- [FHIR Profiling](https://hl7.org/fhir/R4/profiling.html)
- [StructureDefinition](https://hl7.org/fhir/R4/structuredefinition.html)
- [Snapshot Generation](https://hl7.org/fhir/R4/profiling.html#snapshot)

## Checklist de Diseño

### API
- [x] ¿La API es intuitiva? → Sin cambios en API pública
- [x] ¿Sin hardcoding? → Todo basado en meta.profile y config

### Extensibilidad
- [x] ¿Se puede extender? → Opciones funcionales para configuración
- [x] ¿Soporta múltiples profiles? → Sí

### Testing
- [ ] ¿Tests con US Core?
- [ ] ¿Tests de generación de snapshot?
- [ ] ¿Comparación con HL7 validator?
