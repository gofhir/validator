---
title: "CLI Reference"
linkTitle: "CLI Reference"
description: "Complete reference for the gofhir-validator command-line tool -- options, examples, exit codes, and environment variables."
weight: 4
---

The `gofhir-validator` CLI validates FHIR resources against StructureDefinitions, profiles, and implementation guides. It is designed to be compatible with the [HL7 FHIR Validator](https://confluence.hl7.org/display/FHIR/Using+the+FHIR+Validator), offering familiar flags and equivalent validation output.

## Synopsis

```bash
gofhir-validator [options] <file>...
```

Read from standard input:

```bash
cat resource.json | gofhir-validator -
```

## Options

| Option | Description | Default |
|--------|-------------|---------|
| `-version` | FHIR version (`4.0.1`, `4.3.0`, `5.0.0`) | `4.0.1` |
| `-ig` | Profile URL(s) to validate against (comma-separated) | -- |
| `-package` | Additional FHIR package(s) to load from cache (`name#version`), with the packages they depend on | -- |
| `-package-registry` | Package registry that packages missing from the cache are downloaded from | `https://packages.fhir.org` |
| `-no-download` | Download no package: a dependency missing from the cache is reported and not loaded | `false` |
| `-base-package` | Base package(s) to load from cache instead of the ones embedded for the version (`name#version`, comma-separated) | the embedded core, terminology and extensions |
| `-package-file` | Local `.tgz` package file(s) (comma-separated) | -- |
| `-package-url` | Remote `.tgz` package URL(s) (comma-separated) | -- |
| `-output` | Output format: `text` or `json` | `text` |
| `-strict` | Treat warnings as errors | `false` |
| `-tx` | Terminology server, as in the HL7 validator: `n/a` for none. Codes are checked against the ValueSets and CodeSystems loaded, as without `-tx`; a server URL is not supported | -- |
| `-no-terminology` | Skip all terminology and binding validation | `false` |
| `-quiet` | Only show errors and warnings | `false` |
| `-verbose` | Show detailed output | `false` |
| `-v` | Show version | -- |
| `-help` | Show help | -- |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Valid -- no errors found |
| `1` | Invalid -- one or more errors found |
| `2` | System error (missing files, bad options, package not found) |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `FHIR_PACKAGE_PATH` | Custom path to the FHIR package cache (default: `~/.fhir/packages/`) |

## Examples

### Basic Validation

Validate a single file:

```bash
gofhir-validator patient.json
```

Validate multiple files:

```bash
gofhir-validator patient.json observation.json condition.json
```

Validate with glob patterns:

```bash
gofhir-validator resources/*.json
```

Read from stdin:

```bash
cat patient.json | gofhir-validator -
```

Pipe from another command:

```bash
curl -s https://example.com/fhir/Patient/123 | gofhir-validator -
```

### Version Specification

Validate against FHIR R5:

```bash
gofhir-validator -version 5.0.0 patient.json
```

Validate against FHIR R4B:

```bash
gofhir-validator -version 4.3.0 patient.json
```

### Profile Validation

Validate against a single profile:

```bash
gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
```

Validate against multiple profiles:

```bash
gofhir-validator -ig "http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient,http://hl7.org/fhir/uv/ips/StructureDefinition/Patient-uv-ips" patient.json
```

### Package Loading

Load a package from the NPM cache:

```bash
gofhir-validator -package hl7.fhir.us.core#6.1.0 \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

The packages a package depends on (its `package.json` `dependencies`) are loaded too, transitively, in the versions it declares. A package missing from the cache, given with `-package` or depended on, is downloaded into the cache from the package registry (`-package-registry`; base packages given with `-base-package` must be in the cache); `-no-download` turns that off, and a dependency missing from the cache is then reported and not loaded.

Several versions of a package may be loaded, for example a guide that depends on an older terminology package than the base one. A canonical that names a version resolves to that version; one that names none resolves to the highest version loaded among the definitions written for the FHIR version validated. The R4 and R4B core packages carry copies of the HL7 Terminology code systems and value sets, versioned as the FHIR version (`4.0.1`): when the HL7 Terminology package is loaded, its definitions are used instead, whatever their version. One core package is loaded, the FHIR version validated's: a dependency on the core package of another FHIR version is reported and not loaded.

Load a package from a local `.tgz` file:

```bash
gofhir-validator -package-file ./my-custom-ig.tgz patient.json
```

Load a package from a remote URL:

```bash
gofhir-validator -package-url https://packages.simplifier.net/hl7.fhir.us.core/6.1.0 patient.json
```

Combine multiple package sources:

```bash
gofhir-validator \
    -package hl7.fhir.us.core#6.1.0 \
    -package-file ./custom-ig.tgz \
    -package-url https://example.com/another-ig.tgz \
    -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient \
    patient.json
```

### Base packages

The validator embeds, for each FHIR version, the core package, the terminology package (THO) and the extensions package. `-base-package` loads the packages you give from the cache **instead**: to validate against other versions of them, such as the ones another validator uses.

```bash
gofhir-validator -version 4.0.1 \
    -base-package hl7.fhir.r4.core#4.0.1,hl7.terminology.r4#6.2.0,hl7.fhir.uv.extensions.r4#5.3.0 \
    patient.json
```

`-package hl7.terminology.r4#6.2.0` instead adds that version to the embedded terminology package: canonicals that name no version resolve to the higher of the two.

### JSON Output

Produce JSON output (useful for CI/CD pipelines):

```bash
gofhir-validator -output json patient.json
```

### Strict Mode

Treat all warnings as errors:

```bash
gofhir-validator -strict patient.json
```

### Disabling Terminology Validation

Skip terminology and binding checks entirely, for structural validation only:

```bash
gofhir-validator -no-terminology patient.json
```

`-tx n/a` does not do this. As in the HL7 validator, it only says there is no terminology server, and codes are still checked against the ValueSets and CodeSystems loaded.

{{< callout type="tip" >}}
**CI/CD integration** -- Use `-output json` combined with `jq` to parse validation results programmatically. `-tx n/a` states that no terminology server is used, as in the HL7 validator; codes are still checked against the definitions loaded. The exit code (`0` for valid, `1` for invalid) integrates directly with CI pipeline failure conditions.

```bash
gofhir-validator -output json -tx n/a patient.json | jq '.[0].valid'
```
{{< /callout >}}

## Explore

{{< cards >}}
  {{< card link="output-formats" title="Output Formats" subtitle="Text and JSON output formats, parsing examples, and shell scripting" icon="document-text" >}}
{{< /cards >}}
