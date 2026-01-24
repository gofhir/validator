# Diseño: Validación de Código Unificada

## Problema

Actualmente existe duplicación de lógica de validación de códigos/displays entre:
- `TerminologyPhase`: valida bindings en elementos del recurso
- `ExtensionsPhase`: intenta validar bindings en valores de extensiones

## Especificación FHIR

Según [FHIR $validate-code](https://hl7.org/fhir/R4/codesystem-operation-validate-code.html):
- Display mismatch es un **error por defecto** (configurable como warning)
- El resultado incluye: `valid`, `message`, y `display` (recomendado)
- Razón: "the fact that the display name is wrong is an indicator that the wrong code has been chosen"

## Arquitectura Propuesta

### 1. Nuevo Componente: `CodingValidationHelper`

```go
// phase/coding_validation.go

// CodingValidationOptions configura el comportamiento de validación
type CodingValidationOptions struct {
    ValueSet         string  // URL del ValueSet para binding (opcional)
    BindingStrength  string  // required/extensible/preferred/example
    ValidateDisplay  bool    // Validar display contra CodeSystem (default: true)
    DisplayAsWarning bool    // true = warning, false = error (default: true según FHIR recomendación)
    Phase            string  // Nombre de la fase para el issue
}

// CodingValidationResult contiene los resultados de validación
type CodingValidationResult struct {
    Valid   bool      // true si el código es válido según el binding
    Issues  []Issue   // Todos los issues (errores y warnings)
}

// CodingValidationHelper encapsula la lógica de validación de códigos
type CodingValidationHelper struct {
    terminologyService service.TerminologyService
}

// NewCodingValidationHelper crea un nuevo helper
func NewCodingValidationHelper(ts service.TerminologyService) *CodingValidationHelper

// ValidateCoding valida un Coding (map[string]any)
func (h *CodingValidationHelper) ValidateCoding(
    ctx context.Context,
    coding map[string]any,
    path string,
    opts CodingValidationOptions,
) *CodingValidationResult

// ValidateCodeableConcept valida un CodeableConcept (map[string]any)
func (h *CodingValidationHelper) ValidateCodeableConcept(
    ctx context.Context,
    cc map[string]any,
    path string,
    opts CodingValidationOptions,
) *CodingValidationResult
```

### 2. Flujo de Validación

```
ValidateCoding(coding, opts)
    │
    ├── Si opts.ValueSet != "":
    │       └── ValidateCode(system, code, valueSet)
    │           ├── code válido → continuar
    │           └── code inválido → crear issue según BindingStrength
    │
    ├── Si opts.ValidateDisplay && display != "":
    │       └── ValidateCode(system, code, "") // Solo CodeSystem
    │           └── Si display != result.Display
    │               └── crear issue (warning o error según DisplayAsWarning)
    │
    └── return CodingValidationResult{Valid, Issues}
```

### 3. Uso en Fases

```go
// TerminologyPhase
helper := NewCodingValidationHelper(p.terminologyService)
result := helper.ValidateCoding(ctx, coding, path, CodingValidationOptions{
    ValueSet:        binding.ValueSet,
    BindingStrength: binding.Strength,
    ValidateDisplay: true,
    DisplayAsWarning: true,
    Phase:           "terminology",
})

// ExtensionsPhase
helper := NewCodingValidationHelper(p.terminologyService)
result := helper.ValidateCoding(ctx, coding, path, CodingValidationOptions{
    ValueSet:        extBinding.ValueSet, // Puede ser ""
    BindingStrength: extBinding.Strength,
    ValidateDisplay: true,
    DisplayAsWarning: true,
    Phase:           "extensions",
})
```

### 4. Beneficios

1. **DRY**: Lógica de validación en un solo lugar
2. **Testeable**: Helper aislado fácil de testear
3. **Configurable**: Opciones permiten diferentes comportamientos
4. **Consistente con Go**: Composición sobre herencia
5. **Mantiene SRP**: Cada fase mantiene su responsabilidad principal
6. **Extensible**: Fácil agregar nuevas opciones (ej: language para display)

### 5. Archivos a Modificar

1. **Crear**: `phase/coding_validation.go` - Helper
2. **Crear**: `phase/coding_validation_test.go` - Tests
3. **Modificar**: `phase/terminology.go` - Usar helper
4. **Modificar**: `phase/extensions.go` - Usar helper, eliminar código duplicado
5. **Eliminar**: código duplicado de `common.go` si aplica

## Decisiones de Diseño

### ¿Por qué no extender TerminologyService?

La arquitectura actual tiene `CodingValidator` y `CodeableConceptValidator` interfaces,
pero agregarles la lógica de crear Issues mezclaría responsabilidades:
- **Service**: validar código, devolver resultado
- **Phase**: interpretar resultado, crear Issues

El helper mantiene esta separación.

### ¿Por qué usar map[string]any en lugar de service.Coding?

Las fases trabajan con `map[string]any` (recurso parseado).
Convertir a `service.Coding` requeriría conversión en cada llamada.
Mantener `map[string]any` es más eficiente y consistente con el resto del código.

### ¿Display mismatch como error o warning?

Según FHIR oficial: error por defecto.
Pero muchas implementaciones lo tratan como warning (menos estricto).
La opción `DisplayAsWarning` permite configurar esto.
Default: `true` (warning) para ser menos disruptivo.
