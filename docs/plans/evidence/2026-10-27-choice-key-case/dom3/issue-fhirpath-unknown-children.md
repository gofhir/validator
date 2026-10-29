# children() and descendants() return properties the model does not define

**Library:** github.com/gofhir/fhirpath v1.13.6. The same code is in v1.16.6: `inDefinitionOrder`
orders the children by the model and keeps the ones it does not list.

## Reproduction

With a model (`SetModel`) whose `ChildElements("Patient")` does not list `fooBar`, on
`{"resourceType":"Patient","id":"p","fooBar":{"reference":"#x"},"active":true}`:

| Expression | Got | Expected |
|---|---|---|
| `children().count()` | 3 | 2 (id, active) |
| `descendants().where(reference = '#x').exists()` | true | false |
| `fooBar.exists()` | true | false |

## Why

- FHIRPath §5 navigates "the object model"; for FHIR that model is the StructureDefinitions. A
  JSON property that no definition declares is not an element (json.html: "Property names are
  case-sensitive"; formats.html#choice), so it is not a node.
- The HL7 validator 6.10.2 does not navigate it. On CarePlan-f202 (R4 examples), it reports
  `dom-3` the same way whether `activity[0].detail.productReference` (the only reference to the
  contained resources) is deleted, renamed to `productreference`, or renamed to `fooBar`.
  Validator evidence: `dom3/{del,alt,unk}.json` and `dom3/hl7.json`. The same happens with a local
  canonical: in PlanDefinition-protocol-example, `action[0].definitionCanonical` = `#procedure`,
  renamed to `definitioncanonical` (`dom3/canonical/`).

## Impact on the validator

`dom-3` and every invariant that uses `children()`, `descendants()` or path navigation can see
content inside an unknown property. In the R4, R4B and R5 example packages, 148 of 2736 choice
keys with altered case keep `dom-3` from failing (`sweep-before-classes.txt`, `sweep-after.txt`).

## Proposal

When the context has a model that knows the object (`ElementOrderResolver.ChildElements` returns
non-nil), `TypedChildren` and member navigation drop names it does not list. The `_name` of a listed
name is kept, and so is a choice variant whose `[x]` base is listed.
