## Summary

An extension inside a complex extension value is not validated: an extension with a value type its definition does not allow, or with an unknown URL, is reported when it is on an element of the resource, and not reported when it is on an element of another extension's value (`Patient.extension[0].valueIdentifier.type.extension[0]`).

Extensions can be on every element (extensibility.html), and an extension's value is an element, so the extension inside should be validated against its definition like any other.

## Reproduction

`github.com/gofhir/validator/v2` at `a127801` (main), FHIR 4.0.1 and 5.0.0, same result.

Extension `id-ext` (contexts `Patient` and `CodeableConcept`, `value[x]` only `Identifier`):

```json
{
  "resourceType": "StructureDefinition",
  "url": "http://example.org/StructureDefinition/id-ext",
  "name": "IdExt",
  "status": "active",
  "kind": "complex-type",
  "abstract": false,
  "type": "Extension",
  "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
  "derivation": "constraint",
  "context": [
    {"type": "element", "expression": "Patient"},
    {"type": "element", "expression": "CodeableConcept"}
  ],
  "differential": {
    "element": [
      {"id": "Extension", "path": "Extension"},
      {"id": "Extension.extension", "path": "Extension.extension", "max": "0"},
      {"id": "Extension.url", "path": "Extension.url", "fixedUri": "http://example.org/StructureDefinition/id-ext"},
      {"id": "Extension.value[x]", "path": "Extension.value[x]", "min": 1, "type": [{"code": "Identifier"}]}
    ]
  }
}
```

`inner-bad.json`: an `id-ext` with a `valueBoolean` inside the `valueIdentifier` of another `id-ext`:

```json
{
  "resourceType": "Patient",
  "extension": [{
    "url": "http://example.org/StructureDefinition/id-ext",
    "valueIdentifier": {
      "type": {
        "text": "x",
        "extension": [{"url": "http://example.org/StructureDefinition/id-ext", "valueBoolean": true}]
      }
    }
  }]
}
```

`element-bad.json`: the same inner extension on an element of the resource:

```json
{
  "resourceType": "Patient",
  "maritalStatus": {
    "text": "x",
    "extension": [{"url": "http://example.org/StructureDefinition/id-ext", "valueBoolean": true}]
  }
}
```

With the extension in a package file (`package/package.json` and `package/StructureDefinition-id-ext.json`, `idext.tgz`):

```
gofhir-validator -version 4.0.1 -package-file idext.tgz -output json element-bad.json inner-bad.json
```

| Instance | Result | Expected |
| --- | --- | --- |
| `element-bad.json` | error `Extension '.../id-ext' has invalid value type 'boolean'` at `Patient.maritalStatus.extension[0].valueBoolean` | the same |
| `inner-bad.json` | **no error** (only a binding warning) | the same error at `Patient.extension[0].valueIdentifier.type.extension[0].valueBoolean` |
| `inner-bad.json` with an unknown URL in the inner extension | **no warning** | `Unknown extension` at the inner extension |

## Probable cause

`pkg/extension/extension.go`:

- `extractValueType` lowercases the first letter of every type name taken from the key: right for a primitive (`valueString` → `string`), wrong for a complex type (`valueIdentifier` → `identifier`). `validateValueContent` then calls `GetByType("identifier")`, finds nothing and returns, so nothing under a complex extension value is checked by this phase: neither its elements nor the extensions inside it. `isTypeAllowed` hides it because it compares the codes lowercased.
- The structural phase reports the unknown elements of the value anyway, so only the extensions inside it go unchecked.
- The value keys are recognized by a hard-coded `value` prefix and a hard-coded `valueSet` exclusion (`findValueKey`, `hasValue`), not by the `value[x]` element of the extension's definition.

## Proposed fix

Take the extension's value keys and their types from its definition: each type code of `Extension.value[x]` gives the key `value` + the code with its first letter capitalized (json.html#choice), and the type is that code. Then validate the content of each complex value against its type. Once that content is validated, check that the unknown elements the structural phase already reports are not reported twice.
