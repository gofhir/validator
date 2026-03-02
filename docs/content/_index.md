---
title: "GoFHIR Validator"
description: "High-performance FHIR resource validation for Go. Validate R4, R4B, and R5 resources against StructureDefinitions, profiles, and terminology."
layout: hextra-home
---

<div class="hx:text-center hx:mt-24 hx:mb-6">
{{< hextra/hero-badge >}}
  <span>Open Source</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  High-performance FHIR&nbsp;<br class="sm:hx:block hx:hidden" />validation for Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Validate FHIR R4, R4B, and R5 resources against StructureDefinitions, profiles, and terminology&nbsp;<br class="sm:hx:block hx:hidden" />with a fast, embeddable Go library and CLI tool.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get Started" link="docs/getting-started" >}}
{{< hextra/hero-button text="View on GitHub" link="https://github.com/gofhir/validator" style="alt" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Profile-Driven Validation"
    icon="document-text"
    subtitle="All validation rules derived from StructureDefinitions. No hardcoded logic — supports any FHIR profile or Implementation Guide."
  >}}
  {{< hextra/feature-card
    title="HL7 Validator Compatible"
    icon="check-circle"
    subtitle="Designed to produce the same validation results as the HL7 FHIR Validator, with significantly faster startup and lower memory usage."
  >}}
  {{< hextra/feature-card
    title="FHIRPath Constraints"
    icon="code"
    subtitle="Full FHIRPath evaluation engine for invariant constraints defined in ElementDefinitions, including resolve() and memberOf()."
  >}}
{{< /hextra/feature-grid >}}

## Quick Start

{{< callout type="info" >}}
  Requires **Go 1.23** or later.
{{< /callout >}}

Install the validator:

```shell
go get github.com/gofhir/validator
```

Validate a FHIR resource:

```go
package main

import (
    "fmt"

    "github.com/gofhir/validator"
)

func main() {
    v := validator.New()
    result := v.ValidateFile("patient.json")

    for _, issue := range result.Issues() {
        fmt.Printf("[%s] %s: %s\n", issue.Severity, issue.Expression, issue.Diagnostics)
    }
}
```

Or use the CLI:

```shell
gofhir-validator patient.json
```

{{< hextra/hero-button text="Read the full guide" link="docs/getting-started" >}}
