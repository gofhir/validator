---
title: GoFHIR Validator
layout: hextra-home
---

{{< hextra/hero-badge >}}
  <div class="hx-w-2 hx-h-2 hx-rounded-full hx-bg-primary-400"></div>
  <span>Open Source &middot; MIT License</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-headline >}}
  High-performance FHIR&nbsp;<br class="sm:block hidden" />validation for Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx-mb-12">
{{< hextra/hero-subtitle >}}
  Validate FHIR R4 resources against StructureDefinitions, profiles, and terminology&nbsp;<br class="sm:block hidden" />with a fast, embeddable Go library and CLI tool.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx-mb-6">
{{< hextra/hero-button text="Get Started" link="docs/getting-started" >}}
</div>

<div class="hx-mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Profile-Driven Validation"
    subtitle="All validation rules derived from StructureDefinitions. No hardcoded logic — supports any FHIR profile or Implementation Guide."
    icon="document-text"
  >}}
  {{< hextra/feature-card
    title="HL7 Validator Compatible"
    subtitle="Designed to produce the same validation results as the HL7 FHIR Validator, with significantly faster startup and lower memory usage."
    icon="check-circle"
  >}}
  {{< hextra/feature-card
    title="FHIRPath Constraints"
    subtitle="Full FHIRPath evaluation engine for invariant constraints defined in ElementDefinitions, including resolve() and memberOf()."
    icon="code"
  >}}
  {{< hextra/feature-card
    title="Terminology Validation"
    subtitle="Local CodeSystem and ValueSet binding validation for required, extensible, and preferred strength levels."
    icon="book-open"
  >}}
  {{< hextra/feature-card
    title="CLI and Library"
    subtitle="Use as a standalone command-line tool or embed as a Go library in your applications. Compatible with CI/CD pipelines."
    icon="terminal"
  >}}
  {{< hextra/feature-card
    title="Fast and Lightweight"
    subtitle="Written in Go with no JVM dependency. Sub-second startup, low memory footprint, and concurrent validation support."
    icon="lightning-bolt"
  >}}
{{< /hextra/feature-grid >}}
