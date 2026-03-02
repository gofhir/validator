---
title: "Result & Issues"
linkTitle: "Result & Issues"
description: "Validation results, issue types, severity levels, and code constants."
weight: 2
---

After calling `Validate` or `ValidateJSON`, you receive a `*issue.Result` containing all validation issues and statistics.

```go
import "github.com/gofhir/validator/pkg/issue"
```

## Result

```go
type Result struct {
    Issues []Issue
    Stats  *Stats
}
```

The `Result` type collects every issue found during validation and provides convenience methods for querying them.

### Methods

#### HasErrors

```go
func (r *Result) HasErrors() bool
```

Returns `true` if there is at least one issue with severity `error` or `fatal`.

```go
if result.HasErrors() {
    // resource is not valid
}
```

#### ErrorCount

```go
func (r *Result) ErrorCount() int
```

Returns the number of issues with severity `error` or `fatal`.

#### WarningCount

```go
func (r *Result) WarningCount() int
```

Returns the number of issues with severity `warning`.

#### InfoCount

```go
func (r *Result) InfoCount() int
```

Returns the number of issues with severity `information`.

#### Merge

```go
func (r *Result) Merge(other *Result)
```

Appends all issues from `other` into this result. Useful when combining results from multiple validation passes.

```go
combined := issue.NewResult()
combined.Merge(resultA)
combined.Merge(resultB)
```

#### Filter

```go
func (r *Result) Filter(severity Severity) *Result
```

Returns a new `Result` containing only issues that match the given severity.

```go
errorsOnly := result.Filter(issue.SeverityError)
for _, iss := range errorsOnly.Issues {
    fmt.Println(iss.Diagnostics)
}
```

#### EnrichLocations

```go
func (r *Result) EnrichLocations(locator func(expression string) *Location)
```

Adds line and column information to issues based on their FHIRPath expressions. The validator calls this automatically after validation; you typically do not need to call it yourself.

---

## Issue

```go
type Issue struct {
    Severity    Severity
    Code        Code
    Diagnostics string
    Expression  []string
    Location    *Location
    Source      string
    MessageID   string
}
```

Each `Issue` corresponds to a single finding from the validation process, aligned with the FHIR [OperationOutcome.issue](https://hl7.org/fhir/R4/operationoutcome-definitions.html#OperationOutcome.issue) structure.

| Field | Type | Description |
|-------|------|-------------|
| `Severity` | `Severity` | Severity level: `fatal`, `error`, `warning`, or `information` |
| `Code` | `Code` | Issue type code (see [Code Constants](#code-constants) below) |
| `Diagnostics` | `string` | Human-readable description of the issue |
| `Expression` | `[]string` | FHIRPath expression(s) pointing to the element that caused the issue |
| `Location` | `*Location` | Line and column in the source JSON (populated automatically) |
| `Source` | `string` | Name of the validation phase that generated the issue |
| `MessageID` | `string` | Identifier from the error catalog for programmatic handling |

---

## Location

```go
type Location struct {
    Line   int
    Column int
}
```

Represents a position in the source JSON document. Both fields are 1-based. The `Location` is populated automatically by the validator after all phases complete.

---

## Stats

```go
type Stats struct {
    ResourceType    string
    ResourceSize    int
    ProfileURL      string
    IsCustomProfile bool
    Duration        int64 // nanoseconds
    PhasesRun       int
}
```

Validation statistics are attached to every `Result`.

| Field | Type | Description |
|-------|------|-------------|
| `ResourceType` | `string` | FHIR resource type (e.g. `"Patient"`, `"Observation"`) |
| `ResourceSize` | `int` | Size of the input JSON in bytes |
| `ProfileURL` | `string` | Canonical URL of the primary profile used for validation |
| `IsCustomProfile` | `bool` | `true` when a custom profile was used instead of the core resource definition |
| `Duration` | `int64` | Total validation time in nanoseconds |
| `PhasesRun` | `int` | Number of validation phases executed |

### DurationMs

```go
func (s *Stats) DurationMs() float64
```

Returns the duration in milliseconds as a floating-point number.

```go
fmt.Printf("Validated in %.2f ms\n", result.Stats.DurationMs())
```

---

## Severity Constants

Severity levels are aligned with the FHIR [IssueSeverity](https://hl7.org/fhir/R4/valueset-issue-severity.html) value set.

```go
const (
    SeverityFatal       Severity = "fatal"
    SeverityError       Severity = "error"
    SeverityWarning     Severity = "warning"
    SeverityInformation Severity = "information"
)
```

| Constant | Value | Meaning |
|----------|-------|---------|
| `SeverityFatal` | `"fatal"` | The issue caused the validation to abort |
| `SeverityError` | `"error"` | The resource is not valid |
| `SeverityWarning` | `"warning"` | A potential problem was found but the resource may still be valid |
| `SeverityInformation` | `"information"` | Informational message, not a problem |

{{< callout type="info" >}}
When `WithStrictMode(true)` is set, warnings are promoted to errors. `HasErrors()` returns `true` for both `error` and `fatal` severities regardless of strict mode.
{{< /callout >}}

---

## Code Constants

Code constants are aligned with the FHIR [IssueType](https://hl7.org/fhir/R4/valueset-issue-type.html) value set.

```go
const (
    CodeInvalid       Code = "invalid"
    CodeStructure     Code = "structure"
    CodeRequired      Code = "required"
    CodeValue         Code = "value"
    CodeInvariant     Code = "invariant"
    CodeSecurity      Code = "security"
    CodeLogin         Code = "login"
    CodeUnknown       Code = "unknown"
    CodeExpired       Code = "expired"
    CodeForbidden     Code = "forbidden"
    CodeSuppressed    Code = "suppressed"
    CodeProcessing    Code = "processing"
    CodeNotSupported  Code = "not-supported"
    CodeDuplicate     Code = "duplicate"
    CodeMultipleMatch Code = "multiple-matches"
    CodeNotFound      Code = "not-found"
    CodeDeleted       Code = "deleted"
    CodeTooLong       Code = "too-long"
    CodeCodeInvalid   Code = "code-invalid"
    CodeExtension     Code = "extension"
    CodeTooCostly     Code = "too-costly"
    CodeBusinessRule  Code = "business-rule"
    CodeConflict      Code = "conflict"
    CodeTransient     Code = "transient"
    CodeLockError     Code = "lock-error"
    CodeNoStore       Code = "no-store"
    CodeException     Code = "exception"
    CodeTimeout       Code = "timeout"
    CodeIncomplete    Code = "incomplete"
    CodeThrottled     Code = "throttled"
    CodeInformational Code = "informational"
)
```

The most commonly encountered codes during FHIR validation are:

| Code | Typical Use |
|------|-------------|
| `CodeStructure` | Unknown elements, invalid JSON structure |
| `CodeRequired` | Missing required elements (cardinality `min > 0`) |
| `CodeValue` | Invalid primitive values (e.g. malformed date, invalid URI) |
| `CodeInvariant` | Failed FHIRPath constraint |
| `CodeCodeInvalid` | Code not found in the bound ValueSet or CodeSystem |
| `CodeExtension` | Invalid or unknown extension |
| `CodeNotFound` | Referenced profile or resource not found |
| `CodeProcessing` | General processing errors |

---

## Processing Results Programmatically

```go
result, _ := v.Validate(ctx, resource)

// Quick check
if !result.HasErrors() {
    fmt.Println("Resource is valid")
    return
}

// Iterate over all issues
for _, iss := range result.Issues {
    // Build a location string
    loc := ""
    if iss.Location != nil {
        loc = fmt.Sprintf(" (line %d, col %d)", iss.Location.Line, iss.Location.Column)
    }

    // Build an expression string
    expr := ""
    if len(iss.Expression) > 0 {
        expr = fmt.Sprintf(" at %s", iss.Expression[0])
    }

    fmt.Printf("[%s] %s: %s%s%s\n",
        iss.Severity, iss.Code, iss.Diagnostics, expr, loc)
}

// Filter to errors only
errors := result.Filter(issue.SeverityError)
fmt.Printf("\n%d error(s) found\n", len(errors.Issues))

// Access statistics
fmt.Printf("Resource: %s (%d bytes)\n",
    result.Stats.ResourceType, result.Stats.ResourceSize)
fmt.Printf("Profile:  %s\n", result.Stats.ProfileURL)
fmt.Printf("Duration: %.2f ms (%d phases)\n",
    result.Stats.DurationMs(), result.Stats.PhasesRun)
```
