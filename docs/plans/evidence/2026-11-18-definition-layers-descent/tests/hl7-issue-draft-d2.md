# Borrador de issue para hapifhir/org.hl7.fhir.core (sin publicar)

Se publica solo con la aprobación del usuario, y antes hay que buscar duplicados.

**Título:** Snapshot of a derived profile that only redeclares a slice drops the base's unrolled children of the sliced element

**Version:** validator_cli 7.0.4, `-tx n/a`. The result is the same in R4, R4B and R5.

## Reproduction

The files are in gofhir/validator `docs/plans/evidence/2026-11-18-definition-layers-descent/`: the generator `tests/gen.py`
(profiles `d2b-inv` and `d2-inv`) and the snapshots in `tests/hl7-d2-snapshot/`.

- Base profile `d2b-inv` on Patient: slices `Patient.identifier` by `system`, adds constraint `dl-a` on
  `Patient.identifier.value` (the sliced element's child), and declares slice `mrn`.
- Derived profile `d2-inv` (`baseDefinition` = `d2b-inv`): its differential only touches `Patient.identifier:mrn` and
  adds `dl-b` on `Patient.identifier:mrn.value`.

## Expected

The snapshot of `d2-inv` keeps every element of the base snapshot, including `Patient.identifier.value` with `dl-a`.
StructureDefinition.derivation `constraint`: «This definition adds additional rules to an existing concrete type»;
profiling.html: «Profiles cannot break the rules established in the base specification».

## Actual

The snapshot of `d2b-inv` has `Patient.identifier.id` … `Patient.identifier.assigner`. The snapshot of `d2-inv` has
only `Patient.identifier` and the `mrn` slice's children: the sliced element's unrolled children are gone. As a
result, an instance in slice `mrn` that violates `dl-a` (`identifier.value = "a"`) is accepted. Validating against the
base profile, or against one profile with both rules (`sid-inv`), rejects it. The same happens with `fixed[x]`,
`pattern[x]`, a type profile and a child slice on the sliced element (instances `m-d2-*-a`).

## Same family: duplicate ids (D2b, 25-11-2026)

If the derived profile constrains a child of the sliced element and the base declares a slice, `snapshot` fails with
«Same id '<slice child>' on multiple elements», and the validation goes on with a broken snapshot. Profiles: `d-type`
and `d-type2` (base `g0`, `g1`: `Observation.component` sliced, the derived constrains `Observation.component.value[x]`),
`b1-ew` and `b1-we` (`Patient.identifier.value`), and `b1` in `named_g.py` (`Bundle.entry.resource`). The same
differential in one profile (`ctl-*`) has a snapshot and gives the expected result.
