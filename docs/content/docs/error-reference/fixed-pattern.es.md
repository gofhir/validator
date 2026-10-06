---
title: "Errores de Fixed/Pattern"
linkTitle: "Fixed/Pattern"
description: "Errores relacionados con discrepancias de valores fijos y fallos en coincidencia de patrones."
weight: 8
---

Los errores de fixed y pattern ocurren cuando el valor de un elemento no cumple un valor `fixed[x]` o `pattern[x]` declarado en un ElementDefinition:

- **fixed[x]** -- el valor debe ser exactamente el valor fijo: cada elemento del valor fijo debe estar presente con el mismo valor, y "missing elements/attributes must also be missing" (ElementDefinition.fixed[x]).
- **pattern[x]** -- el valor debe contener el patrón: cada elemento del patrón debe estar presente con el mismo valor, y cada ítem de un arreglo del patrón debe coincidir con algún ítem del arreglo del valor. Se permiten otros elementos.

Un valor se revisa contra cada definición que lo rige, igual que sus invariantes: el elemento del perfil, el slice al que pertenece, el perfil que declara su tipo (`type.profile`), el elemento al que apunta un `contentReference`, la definición que nombra el `url` de una extensión y, para un recurso en una entrada de Bundle o un recurso contenido, los perfiles que declara su `meta.profile`. Los primitivos se comparan tal como los escribe el JSON, así que la precisión de un decimal cuenta (`1.50` no es `1.5`, datatypes.html#decimal).

El issue se reporta en el elemento del valor que difiere, no en el elemento que declara el valor fijo o el patrón, igual que en el validador de HL7. Se reporta una sola vez por ubicación, aunque varias definiciones o perfiles exijan el mismo valor.

## Códigos de error

| ID | Severidad | Mensaje |
|----|-----------|---------|
| `FIXED_VALUE_MISMATCH` | error | Value is {actual}, but the profile requires {expected} |
| `FIXED_VALUE_MISSING` | error | Missing element '{element}', which the profile requires to be {expected} |
| `FIXED_VALUE_EXTRA` | error | Element '{element}' is present, but the fixed value the profile requires has none |
| `PATTERN_ITEM_UNMATCHED` | error | No item of '{element}' matches {pattern}, which the profile's pattern requires |

---

## FIXED_VALUE_MISMATCH

Un primitivo del valor difiere del primitivo del valor fijo o del patrón en el mismo lugar.

**Ejemplo:** un slice de `Patient.identifier` fija `system` en `http://example.org/mrn`:

```json
{
  "id": "Patient.identifier:mrn.system",
  "path": "Patient.identifier.system",
  "fixedUri": "http://example.org/mrn"
}
```

Un identifier de ese slice con otro system:

```json
{
  "resourceType": "Patient",
  "identifier": [
    {
      "type": { "coding": [{ "system": "http://terminology.hl7.org/CodeSystem/v2-0203", "code": "MR" }] },
      "system": "http://example.org/other",
      "value": "1"
    }
  ]
}
```

```text
ERROR: Value is "http://example.org/other", but the profile requires "http://example.org/mrn"
  Path: Patient.identifier[0].system
  MessageID: FIXED_VALUE_MISMATCH
```

Con un patrón sobre un tipo complejo, el issue va en el primitivo que difiere: un `patternCoding` cuyo `system` es `http://example.org/cs`, sobre un `valueCoding` con otro system, se reporta en `Patient.extension[0].valueCoding.system`.

**Solución:** usar el valor que exige el perfil.

---

## FIXED_VALUE_MISSING

Falta en el valor un elemento que tiene el valor fijo o el patrón. Esto incluye:

- el valor de un primitivo que solo tiene extensions (`"_status": {"extension": [...]}` sin `status`), cuando el valor fijo o el patrón tiene uno (`'value'`): el valor de la instancia "SHALL be" el valor fijo (decisión B-D12 del plan B; el validador de HL7 6.10.4 no revisa valores fijos ni patrones en un primitivo así);
- una extension que el valor fijo o el patrón le da a un primitivo (su hermano `_x`: `_fixedCode` en la definición, o `_code` dentro de un valor complejo) cuando el primitivo no tiene ninguna; se reporta en el primitivo. Cuando el primitivo tiene extensions pero no la exigida, un valor fijo reporta la extension que difiere (`FIXED_VALUE_MISMATCH`), y un patrón reporta `PATTERN_ITEM_UNMATCHED` en el primitivo.

**Ejemplo:** un `patternCoding` `{"system": "http://example.org/cs"}` sobre un `valueCoding` sin `system`:

```text
ERROR: Missing element 'system', which the profile requires to be "http://example.org/cs"
  Path: Patient.extension[0].valueCoding.system
  MessageID: FIXED_VALUE_MISSING
```

**Solución:** agregar el elemento con el valor exigido.

---

## FIXED_VALUE_EXTRA

El valor tiene un elemento que su valor fijo no tiene. Un valor fijo se compara exactamente: "missing elements/attributes must also be missing". Esto incluye un `id` y las extensions: una extension se reporta en el elemento que la contiene, y el `id` o una extension de un primitivo (su hermano `_x`, como en `"status": "final", "_status": {"extension": [...]}`) en el primitivo; en un primitivo que se repite (`given`), en cada ítem (`Patient.name[0].given[0]`). Cuando el valor fijo también le da extensions al primitivo, se comparan ítem por ítem en su propia ubicación: una extension que difiere se reporta donde difiere (`Patient.gender.extension[0].valueCode`), y una de más, en su ítem (`Patient.gender.extension[1]`).

**Ejemplo:** `Observation.code` fijado a un `CodeableConcept` con un coding LOINC, sobre un code con `text`:

```json
{
  "code": {
    "coding": [{ "system": "http://loinc.org", "code": "1-8" }],
    "text": "extra"
  }
}
```

```text
ERROR: Element 'text' is present, but the fixed value the profile requires has none
  Path: Observation.code.text
  MessageID: FIXED_VALUE_EXTRA
```

El validador de HL7 6.10.4 reporta una extension de más (`Extension_EXT_Fixed_Banned`), pero acepta un `text`, un `coding` o un `id` de más; gofhir sigue la especificación (decisión B-D10 del plan B).

**Solución:** quitar el elemento extra, o pedir que el perfil use `pattern[x]`, que lo permite.

---

## PATTERN_ITEM_UNMATCHED

Un ítem de un arreglo del patrón no coincide con ningún ítem del arreglo del valor. Se reporta en el elemento que contiene el arreglo. Un ítem de un arreglo de primitivos es su valor junto con su id y sus extensions (su ítem `_x`): un patrón `"given": ["A"], "_given": [{"extension": [...]}]` exige un nombre `A` que tenga esa extension.

**Ejemplo:** `Observation.category` con un `patternCodeableConcept` cuyo coding es `vital-signs`, sobre una categoría cuyo único coding es `laboratory`:

```text
ERROR: No item of 'coding' matches {"code":"vital-signs","system":"http://terminology.hl7.org/CodeSystem/observation-category"}, which the profile's pattern requires
  Path: Observation.category[0]
  MessageID: PATTERN_ITEM_UNMATCHED
```

**Solución:** incluir un ítem que coincida con el patrón; se permiten otros ítems.

Cada ítem del arreglo del patrón "must (recursively) match at least one element from the instance array" (ElementDefinition.pattern[x]), así que un mismo ítem de la instancia puede cumplir dos ítems del patrón. El validador de HL7 6.10.4 exige tantos ítems como tiene el patrón, en codings (`Terminology_TX_Coding_Count`) y en otros arreglos (`Fixed_Type_Checks_DT_Name_Given`); gofhir sigue la especificación (decisión B-D11 del plan B).

---

### Fixed y pattern comparados

| Aspecto | fixed[x] | pattern[x] |
|---------|----------|------------|
| Elementos del valor que la definición no tiene | no se permiten (`FIXED_VALUE_EXTRA`) | se permiten |
| Arreglos | los mismos ítems, en orden | cada ítem del patrón coincide con algún ítem, un primitivo con su id y sus extensions |
| Primitivos | tal como están escritos | tal como están escritos |

{{< callout type="info" >}}
Un elemento definido por `contentReference` (`Questionnaire.item.item`) se revisa contra el elemento al que apunta en el mismo perfil, como dice ElementDefinition.contentReference ("an element defined elsewhere in the definition whose content rules should be applied"). El validador de HL7 6.10.4 no aplica ahí los valores fijos de un perfil (decisión B-D9 del plan B).
{{< /callout >}}
