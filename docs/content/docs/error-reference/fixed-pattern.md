---
title: "Fixed/Pattern Errors"
linkTitle: "Fixed/Pattern"
description: "Errors related to fixed value mismatches and pattern matching failures."
weight: 8
---

Fixed and pattern errors occur when an element's value does not meet a `fixed[x]` or `pattern[x]` value declared in an ElementDefinition:

- **fixed[x]** -- the value must be exactly the fixed value: every element the fixed value has must be present with the same value, and "missing elements/attributes must also be missing" (ElementDefinition.fixed[x]).
- **pattern[x]** -- the value must contain the pattern: every element the pattern has must be present with the same value, and each item of an array in the pattern must match an item of the value's array. Other elements are allowed.

A value is checked against every definition that governs it, as its invariants are: the element of the profile, the slice it belongs to, the profile its type declares (`type.profile`), the element a `contentReference` points to, the definition an extension's `url` names, and, for a resource in a Bundle entry or a contained resource, the profiles its `meta.profile` declares. Primitives are compared as the JSON writes them, so a decimal's precision counts (`1.50` is not `1.5`, datatypes.html#decimal).

The issue is reported at the element of the value that differs, not at the element that declares the fixed or pattern value, as the HL7 validator reports it. It is reported once per location, however many definitions or profiles require the same value.

## Error Codes

| ID | Severity | Message |
|----|----------|---------|
| `FIXED_VALUE_MISMATCH` | error | Value is {actual}, but the profile requires {expected} |
| `FIXED_VALUE_MISSING` | error | Missing element '{element}', which the profile requires to be {expected} |
| `FIXED_VALUE_EXTRA` | error | Element '{element}' is present, but the fixed value the profile requires has none |
| `PATTERN_ITEM_UNMATCHED` | error | No item of '{element}' matches {pattern}, which the profile's pattern requires |

---

## FIXED_VALUE_MISMATCH

A primitive of the value differs from the fixed or pattern value's primitive at the same place.

**Example:** a slice of `Patient.identifier` fixes `system` to `http://example.org/mrn`:

```json
{
  "id": "Patient.identifier:mrn.system",
  "path": "Patient.identifier.system",
  "fixedUri": "http://example.org/mrn"
}
```

An identifier of that slice with another system:

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

With a pattern on a complex type, the issue is at the primitive that differs: a `patternCoding` whose `system` is `http://example.org/cs`, on a `valueCoding` with another system, is reported at `Patient.extension[0].valueCoding.system`.

**Fix:** use the value the profile requires.

---

## FIXED_VALUE_MISSING

An element the fixed or pattern value has is missing from the value. This includes:

- the value of a primitive that has only extensions (`"_status": {"extension": [...]}` with no `status`), when the fixed or pattern value has one (`'value'`): the value in the instance "SHALL be" the fixed value (decision B-D12 of plan B; the HL7 validator 6.10.4 checks no fixed or pattern value on such a primitive);
- an extension the fixed or pattern value gives a primitive (its `_x` sibling: `_fixedCode` on the definition, or `_code` within a complex value) when the primitive has none; it is reported at the primitive. When the primitive has extensions but not the one required, a fixed value reports the extension that differs (`FIXED_VALUE_MISMATCH`), and a pattern reports `PATTERN_ITEM_UNMATCHED` at the primitive.

**Example:** a `patternCoding` `{"system": "http://example.org/cs"}` on a `valueCoding` with no `system`:

```text
ERROR: Missing element 'system', which the profile requires to be "http://example.org/cs"
  Path: Patient.extension[0].valueCoding.system
  MessageID: FIXED_VALUE_MISSING
```

**Fix:** add the element with the required value.

---

## FIXED_VALUE_EXTRA

The value has an element its fixed value does not have. A fixed value is matched exactly: "missing elements/attributes must also be missing". This includes an `id` and extensions: an extension is reported at the element that holds it, and a primitive's `id` or extension (its `_x` sibling, as in `"status": "final", "_status": {"extension": [...]}`) at the primitive; on a repeating primitive (`given`), at each item (`Patient.name[0].given[0]`). When the fixed value gives the primitive extensions too, they are compared item by item at their own location: an extension that differs is reported where it differs (`Patient.gender.extension[0].valueCode`), and one more than the fixed value has at its item (`Patient.gender.extension[1]`).

**Example:** `Observation.code` fixed to a `CodeableConcept` with one LOINC coding, on a code with a `text`:

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

The HL7 validator 6.10.4 reports an extra extension (`Extension_EXT_Fixed_Banned`), but accepts an extra `text`, `coding` or `id`; gofhir follows the specification (decision B-D10 of plan B).

**Fix:** remove the extra element, or ask for the profile to use `pattern[x]`, which allows it.

---

## PATTERN_ITEM_UNMATCHED

An item of an array in the pattern matches no item of the value's array. It is reported at the element that holds the array. An item of a primitive array is its value with its id and extensions (its `_x` item): a pattern `"given": ["A"], "_given": [{"extension": [...]}]` needs one given name `A` that has that extension.

**Example:** `Observation.category` with a `patternCodeableConcept` whose coding is `vital-signs`, on a category whose only coding is `laboratory`:

```text
ERROR: No item of 'coding' matches {"code":"vital-signs","system":"http://terminology.hl7.org/CodeSystem/observation-category"}, which the profile's pattern requires
  Path: Observation.category[0]
  MessageID: PATTERN_ITEM_UNMATCHED
```

**Fix:** include an item that matches the pattern; other items are allowed.

Each item of the pattern's array "must (recursively) match at least one element from the instance array" (ElementDefinition.pattern[x]), so one instance item may meet two pattern items. The HL7 validator 6.10.4 requires as many items as the pattern has, for codings (`Terminology_TX_Coding_Count`) and other arrays (`Fixed_Type_Checks_DT_Name_Given`); gofhir follows the specification (decision B-D11 of plan B).

---

### Fixed and pattern compared

| Aspect | fixed[x] | pattern[x] |
|--------|----------|------------|
| Elements the value has that the definition does not | not allowed (`FIXED_VALUE_EXTRA`) | allowed |
| Arrays | the same items, in order | each pattern item matches an item, a primitive item with its id and extensions |
| Primitives | as written | as written |

{{< callout type="info" >}}
An element defined by `contentReference` (`Questionnaire.item.item`) is checked against the element it points to in the same profile, as ElementDefinition.contentReference says ("an element defined elsewhere in the definition whose content rules should be applied"). The HL7 validator 6.10.4 does not apply a profile's fixed values there (decision B-D9 of plan B).
{{< /callout >}}
