---
title: "Examples"
linkTitle: "Examples"
description: "Real-world examples showing how to use the GoFHIR Validator for common validation scenarios."
weight: 6
---

This section contains complete, runnable examples that demonstrate common FHIR validation patterns using the GoFHIR Validator. Each example includes both CLI and Go library usage where applicable.

{{< callout type="info" >}}
All Go examples in this section assume you have already installed the GoFHIR Validator library. See [Installation]({{< relref "../getting-started/installation" >}}) if you have not set up the library yet.
{{< /callout >}}

## Examples

{{< cards >}}
  {{< card link="basic-validation" title="Basic Validation" subtitle="Validate resources, process results, and reuse the validator" icon="check-circle" >}}
  {{< card link="profile-validation" title="Profile Validation" subtitle="Validate against US Core, custom profiles, and implementation guides" icon="document-search" >}}
  {{< card link="cicd-integration" title="CI/CD Integration" subtitle="Automate validation in GitHub Actions, GitLab CI, and Docker" icon="chip" >}}
{{< /cards >}}
