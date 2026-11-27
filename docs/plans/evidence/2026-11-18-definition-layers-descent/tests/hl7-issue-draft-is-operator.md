# Borrador de issue para hapifhir/org.hl7.fhir.core (sin publicar)

Se publica solo con la aprobación del usuario, y antes hay que buscar duplicados.

**Título:** FHIRPath: `$this is FHIR.string` is false on a string element, while `$this.is(FHIR.string)` is true

**Version:** validator_cli 7.0.4, `-tx n/a`, R4.

## Reproduction

Profile on Patient with constraints on `Patient.identifier.value` (`is-operator/isp.json` in gofhir/validator
`docs/plans/evidence/2026-11-18-definition-layers-descent/tests/`), instance with `identifier[0].value = "x"`.

## Expected

`$this is FHIR.string` holds. FHIRPath N1 §6.3: `is(type : type specifier)` «is the same as the is operator»; the
element is a FHIR `string`.

## Actual

`$this is FHIR.string` and `identifier.value.first() is FHIR.string` fail; `$this.is(FHIR.string)`,
`$this.ofType(FHIR.string).exists()` and `$this is string` hold.
