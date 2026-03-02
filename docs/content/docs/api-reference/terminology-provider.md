---
title: "Terminology Provider"
linkTitle: "Terminology Provider"
description: "Interface for delegating code validation to external terminology servers."
weight: 3
---

The `Provider` interface allows the validator to delegate code validation to an external terminology service for code systems that cannot be expanded locally (e.g. SNOMED CT, LOINC, ICD-10).

```go
import "github.com/gofhir/validator/pkg/terminology"
```

## Interface

```go
type Provider interface {
    ValidateCode(ctx context.Context, system, code string) (bool, error)
    ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
}
```

### ValidateCode

```go
ValidateCode(ctx context.Context, system, code string) (bool, error)
```

Checks whether a `code` is valid in the given code `system`. Returns `(true, nil)` if the code is valid, `(false, nil)` if the code is not valid.

If the method returns an error, the validator falls back to fail-open behavior (accepts any code for that system), ensuring that terminology service unavailability does not break validation.

### ValidateCodeInValueSet

```go
ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
```

Checks whether a `code` from `system` is a member of the ValueSet identified by `valueSetURL`.

The `found` return value is critical:

| `found` | Meaning |
|---------|---------|
| `true` | The provider knows about this ValueSet and returned a definitive answer in `valid` |
| `false` | The provider does not support this ValueSet; the validator will fall back to system-level validation via `ValidateCode` |

This two-level approach lets your provider handle only the ValueSets it knows about without blocking validation for the rest.

{{< callout type="info" >}}
**Local terminology is used by default.** The validator ships with embedded CodeSystems and ValueSets from the FHIR specification. An external provider is only needed for large external terminologies (SNOMED CT, LOINC, RxNorm, etc.) that are not bundled with the base specification.
{{< /callout >}}

---

## Example Implementation

The following example wraps a FHIR terminology server that exposes the standard `$validate-code` operation:

```go
package myterm

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"

    "github.com/gofhir/validator/pkg/terminology"
)

// Ensure interface compliance at compile time.
var _ terminology.Provider = (*FHIRTermProvider)(nil)

// FHIRTermProvider validates codes against a remote FHIR terminology server.
type FHIRTermProvider struct {
    baseURL    string
    httpClient *http.Client
}

// NewFHIRTermProvider creates a provider pointing at the given terminology server.
func NewFHIRTermProvider(baseURL string) *FHIRTermProvider {
    return &FHIRTermProvider{
        baseURL:    baseURL,
        httpClient: &http.Client{},
    }
}

func (p *FHIRTermProvider) ValidateCode(
    ctx context.Context, system, code string,
) (bool, error) {
    endpoint := fmt.Sprintf("%s/CodeSystem/$validate-code?system=%s&code=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
    if err != nil {
        return false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return false, err // fail-open: validator will accept the code
    }
    defer resp.Body.Close()

    var params struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean *bool  `json:"valueBoolean,omitempty"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
        return false, err
    }

    for _, param := range params.Parameter {
        if param.Name == "result" && param.ValueBoolean != nil {
            return *param.ValueBoolean, nil
        }
    }

    return false, fmt.Errorf("unexpected response from terminology server")
}

func (p *FHIRTermProvider) ValidateCodeInValueSet(
    ctx context.Context, system, code, valueSetURL string,
) (valid bool, found bool, err error) {
    endpoint := fmt.Sprintf("%s/ValueSet/$validate-code?system=%s&code=%s&url=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
        url.QueryEscape(valueSetURL),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
    if err != nil {
        return false, false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return false, false, err
    }
    defer resp.Body.Close()

    // 404 means the server does not know about this ValueSet
    if resp.StatusCode == http.StatusNotFound {
        return false, false, nil // found=false: fall back to ValidateCode
    }

    var params struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean *bool  `json:"valueBoolean,omitempty"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
        return false, false, err
    }

    for _, param := range params.Parameter {
        if param.Name == "result" && param.ValueBoolean != nil {
            return *param.ValueBoolean, true, nil
        }
    }

    return false, false, fmt.Errorf("unexpected response from terminology server")
}
```

---

## Wiring the Provider

Pass your implementation to the validator using `WithTerminologyProvider`:

```go
provider := myterm.NewFHIRTermProvider("https://tx.fhir.org/r4")

v, err := validator.New(
    validator.WithTerminologyProvider(provider),
)
```

When a binding references an external code system (e.g. `http://snomed.info/sct`), the validator will call your provider instead of silently accepting the code.

If you want to disable all terminology validation entirely, use `WithNoTerminology()` instead:

```go
v, err := validator.New(
    validator.WithNoTerminology(),
)
```
