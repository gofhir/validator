# Plan: a package's example folder loaded as HL7 loads it

Status: done, revised after an adversarial analysis (HL7 6.10.2, `-tx n/a`). Base: `main` at
c582c55.

## Goal

The definitions a package publishes in its example folder (StructureDefinitions, ValueSets,
CodeSystems) are loaded as the package folder's are, as the HL7 validator loads them, for every
package loaded, its dependencies included.

## Analysis (main, c582c55)

The NPM package specification (packages.html) puts example resources in `/package/example`, indexed
as the package folder is: they are the package's content. HL7 loads that folder's conformance
resources for every package it loads, the dependencies included (`-ig fhir.cqf.common#4.0.1`
resolves cpg's `chf-codes` and applies its `chf-bodyweight` profile), and chooses among versions of a
canonical by version, whatever folder it is in. gofhir's `OpenPackage` and `LoadPackage` read the
package folder only, so a CodeSystem, a ValueSet or a profile an IG publishes as an example is not
found (CPG, CQL, CRMI; SDC 3.0.0's `sdc-question-library` binds to a value set of its example
folder); a package read from a `.tgz` was loaded whole, every folder and every resource type.

## Design

1. `OpenPackage` opens the example folder from its `.index.json` (version 2), else from its files:
   StructureDefinitions read, ValueSets and CodeSystems deferred, as from the package folder;
   `LoadPackage` reads its files. Keys apart from the package folder's (`example/...`).
2. A `.tgz` contributes its package folder's resources and its example folder's definitions, no
   other folder's (it took every folder and type).
3. No precedence rule: versions are chosen as for any canonical (`TestExampleCodeSystemByVersion`);
   a package's resources are walked in a fixed order, the package folder's first (`Package.EachResource`),
   its deferred ones too: between equals the package folder's is used (`TestExampleCodeSystemAfterDeferred`).
4. An example profile resolves by its url and is no profile the package declares:
   `GetProfilesByPackage`, which `ValidateWithIG` applies, leaves it out (`TestExampleProfilesAreNotThePackages`).

Decided by the specification, not by HL7: a `.tgz` contributes its example folder's definitions as a package of
the cache does; HL7 with `-ig file.tgz` does not load the example folder (its two paths differ). Not this plan's:
go-fhir-server's IG installer drops the subfolders (a change of that repository).

## Steps

| Step | Status | Content |
| --- | --- | --- |
| 1 | done | Design 1-3. Groups `package-example-cpg-probes` (`xa_*`, `fhir.cqf.common#4.0.1`) and `package-example-cql-probes` (`xb_*`, `hl7.fhir.us.cqfmeasures#5.0.0`): valid and invalid codes of example CodeSystems, an example profile; every corpus passes. |
