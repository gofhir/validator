Closing: what remains of this issue is not a defect.

Probes in R4, R4B and R5 (17 cases, HL7 validator_cli vs gofhir `e22324e`; `docs/plans/evidence/2026-11-01-held-meta-profile/probe/` on the branch of the follow-up) show:

- A held resource (Bundle entry, `contained`, `Parameters.parameter.resource`, a Bundle inside a Bundle, a profile named by the holder's `type.profile`) is already validated against its own `meta.profile` by the phases that depend on a profile: a value of a type the profile does not allow is reported at the held resource's path, as HL7 does.
- HL7's parser does not read any profile. With two variants of one choice element (`valueCode` and `valueString`), it reports "Unrecognized property" for the later one in the document, **with or without a profile**, at the root and in a held resource alike. So the structural phase should not read a held resource's `meta.profile` either.

The real difference is how two variants of a choice element are reported, and it is the same at the root and in a held resource. Filed as #196.
