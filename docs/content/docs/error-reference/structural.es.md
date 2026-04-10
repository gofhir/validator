---
title: "Errores Estructurales"
linkTitle: "Estructurales"
description: "Errores relacionados con la estructura JSON, elementos desconocidos e identificación del tipo de recurso."
weight: 1
---

Los errores estructurales se detectan durante la fase de validación más temprana. Indican problemas con la estructura JSON básica del recurso, la presencia de elementos no reconocidos o problemas con la propiedad `resourceType`. Estos errores deben resolverse antes de que otras fases de validación puedan producir resultados significativos.

## Códigos de Error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `STRUCTURE_UNKNOWN_ELEMENT` | error | Unknown element '{element}' |
| `STRUCTURE_INVALID_JSON` | error | Invalid JSON: {error} |
| `STRUCTURE_NOT_OBJECT` | error | Resource must be a JSON object |
| `STRUCTURE_NO_RESOURCE_TYPE` | error | Missing 'resourceType' property |
| `STRUCTURE_UNKNOWN_RESOURCE` | error | Unknown resourceType '{type}' |
| `STRUCTURE_CHOICE_MUTUAL_EXCLUSION` | error | Only one variant of choice type '{basePath}[x]' is allowed, but found: {variants} |

---

## STRUCTURE_UNKNOWN_ELEMENT

Se encontró un elemento en el recurso que no está definido en el StructureDefinition para este tipo de recurso. Esto frecuentemente indica un error tipográfico en el nombre del elemento o un elemento que pertenece a un tipo de recurso diferente.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "nmae": [
    {
      "family": "Smith"
    }
  ]
}
```

El elemento `nmae` no está definido en el StructureDefinition de Patient. El nombre correcto del elemento es `name`.

**Corrección:**

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

{{< callout type="info" >}}
Los elementos desconocidos se detectan comparando cada propiedad en el JSON contra las entradas `ElementDefinition.path` en el snapshot del StructureDefinition del recurso. No se utilizan listas de elementos hardcodeadas.
{{< /callout >}}

---

## STRUCTURE_INVALID_JSON

La entrada no es JSON válido. El validador no pudo parsear el contenido antes de que cualquier validación a nivel FHIR pudiera comenzar.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Patient",
  "name": [
    { "family": "Smith", }
  ]
}
```

La coma final después de `"Smith"` no es JSON válido.

**Corrección:** Asegúrate de que la entrada sea JSON bien formado. Usa un linter de JSON o la validación integrada de tu editor para detectar errores de sintaxis antes de ejecutar el validador FHIR.

---

## STRUCTURE_NOT_OBJECT

El valor JSON de nivel superior no es un objeto. Los recursos FHIR deben representarse como objetos JSON (encerrados entre llaves).

**Ejemplo -- recurso inválido:**

```json
[
  { "resourceType": "Patient" }
]
```

Se proporcionó un array JSON en lugar de un objeto JSON.

**Corrección:** Asegúrate de que el recurso sea un único objeto JSON en el nivel superior:

```json
{
  "resourceType": "Patient"
}
```

Si necesitas validar múltiples recursos, envuélvelos en un recurso Bundle de FHIR o valida cada uno individualmente.

---

## STRUCTURE_NO_RESOURCE_TYPE

El objeto JSON no contiene una propiedad `resourceType`. Todo recurso FHIR debe declarar su tipo usando esta propiedad.

**Ejemplo -- recurso inválido:**

```json
{
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

**Corrección:** Agrega la propiedad `resourceType`:

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

---

## STRUCTURE_UNKNOWN_RESOURCE

El valor de `resourceType` no coincide con ningún tipo de recurso FHIR conocido. Esto puede indicar un error tipográfico, un tipo de recurso personalizado que no ha sido cargado o una discrepancia de versión.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Pateint",
  "name": [
    {
      "family": "Smith"
    }
  ]
}
```

El valor `Pateint` no es un tipo de recurso FHIR válido (nota el error tipográfico).

**Corrección:** Usa el nombre correcto del tipo de recurso. Los nombres de tipos de recurso FHIR son sensibles a mayúsculas y usan PascalCase:

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

{{< callout type="warning" >}}
El conjunto de tipos de recurso conocidos está determinado por los StructureDefinitions cargados en el validador. Si estás usando un tipo de recurso personalizado definido en una Implementation Guide, asegúrate de que el StructureDefinition correspondiente esté cargado.
{{< /callout >}}

---

## STRUCTURE_CHOICE_MUTUAL_EXCLUSION

Se encontraron múltiples variantes del mismo tipo choice. FHIR solo permite una variante de un tipo choice `[x]` en cualquier elemento dado. Por ejemplo, `Observation.value[x]` puede ser `valueQuantity` O `valueString`, pero no ambos.

**Ejemplo -- recurso inválido:**

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "test"},
  "valueQuantity": {"value": 72, "unit": "bpm"},
  "valueString": "also a string"
}
```

Tanto `valueQuantity` como `valueString` son variantes del mismo tipo choice `value[x]`. Solo se permite una.

**Corrección:** Mantén solo la variante apropiada:

```json
{
  "resourceType": "Observation",
  "status": "final",
  "code": {"text": "test"},
  "valueQuantity": {"value": 72, "unit": "bpm"}
}
```

{{< callout type="info" >}}
Las variantes de tipos choice se detectan comparando los nombres de elementos contra las entradas `ElementDefinition.path` que terminan en `[x]` en el snapshot del StructureDefinition. Los tipos permitidos provienen del array `ElementDefinition.type`. No se hardcodean nombres de elementos.
{{< /callout >}}
