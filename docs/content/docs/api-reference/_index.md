---
title: "API Reference"
linkTitle: "API Reference"
description: "Complete reference for the GoFHIR Validator Go library -- types, functions, interfaces, and configuration options."
weight: 3
---

The GoFHIR Validator Go library lives under a single import path:

```go
import "github.com/gofhir/validator/pkg/validator"
```

The library follows the **functional options pattern** for configuration. You create a `Validator` once with all the options you need, then call `Validate` or `ValidateJSON` as many times as required.

{{< callout type="info" >}}
**Thread safety** -- After creation, the `Validator` is safe for concurrent use. You can call `Validate` from multiple goroutines without synchronization. All internal caches and indexes are built during construction and are read-only at validation time.
{{< /callout >}}

## Packages

| Package | Import | Purpose |
|---------|--------|---------|
| `validator` | `pkg/validator` | Main entry point -- constructor, options, `Validate` method |
| `issue` | `pkg/issue` | Validation results, issues, severity and code constants |
| `terminology` | `pkg/terminology` | `Provider` interface for external terminology servers |
| `registry` | `pkg/registry` | `ProfileResolver` interface for on-demand SD loading |

## Explore

{{< cards >}}
  {{< card link="validator" title="Validator" subtitle="Constructor, methods, and configuration options" icon="code" >}}
  {{< card link="result" title="Result & Issues" subtitle="Result type, Issue fields, severity and code constants" icon="clipboard-check" >}}
  {{< card link="terminology-provider" title="Terminology Provider" subtitle="Interface for external code system validation" icon="database" >}}
  {{< card link="profile-resolver" title="Profile Resolver" subtitle="Interface for on-demand StructureDefinition loading" icon="search" >}}
{{< /cards >}}
