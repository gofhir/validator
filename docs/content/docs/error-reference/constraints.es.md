---
title: "Errores de Constraints"
linkTitle: "Constraints"
description: "Errores relacionados con fallos en la evaluación de constraints FHIRPath."
weight: 7
---

Los errores de constraints ocurren cuando una invariante FHIRPath definida en el StructureDefinition no se evalúa como `true`, o no se puede compilar o evaluar. FHIR define constraints (invariantes) en elementos usando expresiones FHIRPath. Cada constraint tiene una clave (ej., `ele-1`, `dom-6`), una severidad (`error` o `warning`), una descripción legible por humanos y una expresión FHIRPath que debe evaluarse como `true` para que el recurso sea válido.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `CONSTRAINT_FAILED` | varía | Constraint failed: {constraint}: '{human}' |
| `CONSTRAINT_COMPILE_ERROR` | error (warning en las definiciones de la propia especificación) | Could not compile constraint '{key}': {error} |
| `CONSTRAINT_EVAL_ERROR` | warning | Could not evaluate constraint '{key}': {error} |

---

## CONSTRAINT_FAILED

Un constraint FHIRPath no se evaluó como `true`. La severidad de este issue está determinada por la definición del constraint en sí -- cada constraint declara si la violación es un `error` o un `warning`.

Un invariante "must evaluate to true when run on the element" (conformance-rules.html#constraints): un único booleano es su valor, un resultado vacío falla y cualquier otro resultado se cumple. Así, `resolve().code.text = 'target'` falla donde la referencia no se resuelve, como lo evalúa el validador de HL7. Los invariantes se evalúan en todo elemento, incluido un primitivo que solo tiene id o extensions (`"_status": {...}` sin `status`).

Algunos invariantes publicados son vacíos donde no deberían aplicar. Cada uno se corrige, en la versión en que se publicó y solo donde su expresión es la publicada, a la expresión que da una publicación oficial posterior, de la misma definición o del mismo constraint (`Constraint.source`):

| Clave | Publicado en | Corregido desde |
|-------|--------------|-----------------|
| `ref-1` | R4 y R4B `Reference` (una referencia sin `reference`) | R5 |
| `bdl-8` | R4 `Bundle.entry` (una entrada sin `fullUrl`) | R4B |
| `ras-2` | R4 `RiskAssessment.prediction` (una predicción sin probabilidad) | R4B |
| `pd-1`, `us-core-13` | US Core 5.0.1 y 6.1.0 `PractitionerRole` (`telecom or endpoint`, `... or healthcareService or location`: no es booleano con dos de ellos) | US Core 9.0.0 |
| `vs-1` | Perfiles de signos vitales de R4, R4B y R5 (`$this as dateTime` sobre un Period) | US Core 9.0.0, que republica el mismo constraint corregido |
| `vsd-0`, `csd-0`, `que-0`, `lib-0`, … | R4, 30 recursos canónicos (`name.matches(...)`, un recurso sin nombre) | R4B (`csd-0`) |

Otros tres invariantes de R4 están mal publicados y se corrigen igual: `que-12` exige `enableBehavior` desde tres `enableWhen` (R4B: desde dos), `tim-9` no puede evaluarse con varios `when` (R4 y R4B, corregido desde R5) y `con-3` compara un CodeableConcept con un string (R4B, que exige clinicalStatus solo donde hay verificationStatus).

**Ejemplo -- recurso inválido:**

El constraint `ele-1` está definido en el tipo base `Element` con la expresión `hasValue() or (children().count() > id.count())` y la descripción humana "All FHIR elements must have a @value or children". Si un elemento no tiene ni valor ni hijos, este constraint falla:

```json
{
  "resourceType": "Patient",
  "name": [
    {}
  ]
}
```

Salida de validación:

```text
ERROR: Constraint failed: ele-1: 'All FHIR elements must have a @value or children'
  Path: Patient.name[0]
  MessageID: CONSTRAINT_FAILED
```

**Corrección:** Asegúrate de que el elemento tenga un valor o hijos:

```json
{
  "resourceType": "Patient",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

### Constraints Comunes

Aquí hay algunos constraints FHIR encontrados frecuentemente:

| Clave | Contexto | Descripción | Severidad |
|-------|----------|-------------|-----------|
| `ele-1` | Element | All FHIR elements must have a @value or children | error |
| `dom-3` | DomainResource | If the resource is contained in another resource, it SHALL be referred to from elsewhere in the resource | error |
| `dom-6` | DomainResource | A resource should have narrative for robust management | warning |
| `obs-6` | Observation | dataAbsentReason SHALL only be present if Observation.value[x] is not present | error |
| `obs-7` | Observation | If Observation.code is the same as an Observation.component.code then the value element associated with the code SHALL NOT be present | error |
| `pat-1` | Patient.contact | SHALL at least contain a contact's details or a reference to an organization | error |
| `ref-1` | Reference | SHALL have a contained resource if a local reference is provided | error |
| `per-1` | Period | If present, start SHALL have a lower value than end | error |
| `txt-1` | Narrative.div | The narrative SHALL contain only the basic html formatting elements and attributes | error |
| `txt-2` | Narrative.div | The narrative SHALL have some non-whitespace content | error |

### Severidad Desde la Definición del Constraint

La severidad de `CONSTRAINT_FAILED` no es fija. Se lee del campo `ElementDefinition.constraint.severity` en el StructureDefinition:

```json
{
  "key": "ele-1",
  "severity": "error",
  "human": "All FHIR elements must have a @value or children",
  "expression": "hasValue() or (children().count() > id.count())"
}
```

Si `severity` es `"error"`, el issue es un error. Si `severity` es `"warning"`, el issue es un warning. Los perfiles pueden agregar nuevos constraints o cambiar la severidad del constraint (dentro de ciertos límites).

{{< callout type="info" >}}
Los constraints se evalúan usando FHIRPath, un lenguaje de navegación y extracción basado en rutas para FHIR. La expresión del constraint debería retornar un valor booleano. Cuando retorna `false`, o nada, el constraint se considera violado.
{{< /callout >}}

---

## CONSTRAINT_COMPILE_ERROR y CONSTRAINT_EVAL_ERROR

Una expresión que no compila no puede cumplirse: `CONSTRAINT_COMPILE_ERROR` es un error, reportado una vez por ubicación, como lo reporta el validador de HL7. En las definiciones de la propia especificación es un warning, porque el defecto es de la especificación, no de la instancia (el `eld-11` de R5 cita un string con comillas dobles).

Una expresión que compila pero falla al evaluarse es un invariante que no se cumple: `CONSTRAINT_FAILED` con la severidad del propio constraint y el error en su mensaje, como en el validador de HL7, que toma una excepción de su motor FHIRPath como un invariante fallido. Por ejemplo, `or` sobre una colección de dos ítems es un error en FHIRPath ("singleton evaluation of collections"):

```text
ERROR: Constraint failed: inv-1: '...' (could not be evaluated: SingletonExpectedError: or expects a single item on each side, got 2 on the left)
  Path: Patient
  MessageID: CONSTRAINT_FAILED
```

Solo una evaluación detenida por el límite de tiempo del propio validador no dice nada de la instancia: es `CONSTRAINT_EVAL_ERROR`, un warning.

---

## Evaluación de Constraints a Nivel de Tipo

El validador evalúa constraints FHIRPath no solo del StructureDefinition del recurso, sino también de los StructureDefinitions de los tipos de datos complejos utilizados dentro del recurso. Por ejemplo, cuando un recurso `Patient` contiene un elemento `name[0].period` de tipo `Period`, el validador carga el StructureDefinition de Period y evalúa sus constraints (como `per-1`: start <= end) contra cada instancia de Period encontrada en el recurso.

Esta evaluación recursiva cubre todos los niveles de anidamiento. Por ejemplo, `Patient.name` (tipo HumanName) contiene `HumanName.period` (tipo Period), y el constraint `per-1` del SD de Period se evalúa aunque no aparezca en el snapshot del Patient.

### Funciones Adicionales FHIR

El motor de constraints soporta funciones FHIRPath específicas de FHIR definidas en [FHIR R4 §2.9.1.5](https://hl7.org/fhir/R4/fhirpath.html):

| Función | Descripción |
|---------|-------------|
| `resolve()` | Resuelve una referencia FHIR al recurso objetivo |
| `memberOf()` | Verifica si un código está en un ValueSet |
| `extension()` | Retorna extensions que coincidan con una URL |
| `hasExtension()` | Verifica si existe una extension |
| `conformsTo()` | Verifica si un recurso conforma con un perfil |
| `htmlChecks()` | Valida contenido XHTML narrativo según las reglas FHIR |

La función `htmlChecks()` valida que el contenido de `text.div` siga las reglas XHTML de FHIR (R4 §2.4.1): elemento raíz `<div>`, whitelist de elementos HTML permitidos, elementos/atributos prohibidos, y contenido no vacío. Esto permite que los constraints `txt-1` y `txt-2` se evalúen dinámicamente desde el StructureDefinition de Narrative.
