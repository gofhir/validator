---
title: "Server Integration"
linkTitle: "Server Integration"
description: "Embed the GoFHIR Validator in FHIR servers, HTTP APIs, and backend services with on-demand profile resolution and terminology delegation."
weight: 4
---

The GoFHIR Validator is designed to be embedded in FHIR servers and backend services. This page covers the key integration patterns: on-demand profile resolution, external terminology delegation, per-call profile selection, and building HTTP validation endpoints.

## Use Case

When the validator runs as a standalone CLI tool, it pre-loads all StructureDefinitions and terminology resources at startup. In a server context, this model may not be sufficient:

- **Profiles may be stored in a database** and created or updated at runtime.
- **Terminology may be managed by an external server** (e.g., a FHIR Terminology Server supporting `$validate-code`).
- **Different requests may require different profiles**, and you cannot predict which profiles will be needed at startup.

The GoFHIR Validator supports all of these scenarios through two extension points: `ProfileResolver` and `TerminologyProvider`.

## ProfileResolver

A `ProfileResolver` allows the validator to load StructureDefinitions on demand when they are not found in the in-memory registry. This is the recommended integration point for FHIR servers that store conformance resources in a database.

### Interface

```go
// ProfileResolver resolves StructureDefinitions by canonical URL.
type ProfileResolver interface {
    // ResolveProfile fetches a StructureDefinition by canonical URL and optional version.
    // When version is empty, the resolver SHOULD return the latest available version.
    // Returns (nil, nil) if the profile is not found (not an error condition).
    // Returns (nil, error) on resolver failure (network, DB, etc.).
    ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
}
```

### Implementation Example

```go
package myserver

import (
    "context"
    "database/sql"
)

// DBProfileResolver loads StructureDefinitions from a PostgreSQL database.
type DBProfileResolver struct {
    db *sql.DB
}

func (r *DBProfileResolver) ResolveProfile(ctx context.Context, url, version string) ([]byte, error) {
    var data []byte
    var err error

    if version != "" {
        err = r.db.QueryRowContext(ctx,
            "SELECT resource FROM structure_definitions WHERE url = $1 AND version = $2",
            url, version,
        ).Scan(&data)
    } else {
        // Return the latest version
        err = r.db.QueryRowContext(ctx,
            "SELECT resource FROM structure_definitions WHERE url = $1 ORDER BY version DESC LIMIT 1",
            url,
        ).Scan(&data)
    }

    if err == sql.ErrNoRows {
        return nil, nil // Not found -- not an error
    }
    return data, err
}
```

### Wiring It Up

```go
resolver := &DBProfileResolver{db: db}

v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithProfileResolver(resolver),
)
```

When the validator encounters a profile URL that is not in its in-memory registry, it calls `ResolveProfile()` to fetch the StructureDefinition. The resolved profile is cached in the registry for subsequent lookups.

## TerminologyProvider

A `TerminologyProvider` allows the validator to delegate code validation to an external service. This is useful for large code systems like SNOMED CT or LOINC that cannot be loaded into memory.

### Interface

```go
// Provider validates codes against external terminology services.
type Provider interface {
    // ValidateCode checks if a code is valid in a given code system.
    // Returns (valid, error). If error is non-nil, the Registry falls back
    // to the wildcard behavior for the system.
    ValidateCode(ctx context.Context, system, code string) (bool, error)

    // ValidateCodeInValueSet checks if a code is a member of a ValueSet.
    // Returns (valid, found, error). If found is false, the ValueSet is not
    // supported by this provider, and the Registry will fall back to
    // system-level validation via ValidateCode.
    ValidateCodeInValueSet(ctx context.Context, system, code, valueSetURL string) (valid bool, found bool, err error)
}
```

### Implementation Example

```go
package myserver

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
)

// FHIRTerminologyProvider delegates to an external FHIR Terminology Server.
type FHIRTerminologyProvider struct {
    baseURL string
    client  *http.Client
}

func NewFHIRTerminologyProvider(baseURL string) *FHIRTerminologyProvider {
    return &FHIRTerminologyProvider{
        baseURL: baseURL,
        client:  &http.Client{},
    }
}

func (p *FHIRTerminologyProvider) ValidateCode(ctx context.Context, system, code string) (bool, error) {
    endpoint := fmt.Sprintf("%s/CodeSystem/$validate-code?system=%s&code=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
    )

    req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    if err != nil {
        return false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    var result struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean bool   `json:"valueBoolean"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return false, err
    }

    for _, param := range result.Parameter {
        if param.Name == "result" {
            return param.ValueBoolean, nil
        }
    }
    return false, fmt.Errorf("unexpected response from terminology server")
}

func (p *FHIRTerminologyProvider) ValidateCodeInValueSet(
    ctx context.Context, system, code, valueSetURL string,
) (bool, bool, error) {
    endpoint := fmt.Sprintf("%s/ValueSet/$validate-code?system=%s&code=%s&url=%s",
        p.baseURL,
        url.QueryEscape(system),
        url.QueryEscape(code),
        url.QueryEscape(valueSetURL),
    )

    req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    if err != nil {
        return false, false, err
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := p.client.Do(req)
    if err != nil {
        return false, false, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNotFound {
        return false, false, nil // ValueSet not supported by this server
    }

    var result struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean bool   `json:"valueBoolean"`
        } `json:"parameter"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return false, false, err
    }

    for _, param := range result.Parameter {
        if param.Name == "result" {
            return param.ValueBoolean, true, nil
        }
    }
    return false, true, nil
}
```

### Wiring It Up

```go
txProvider := NewFHIRTerminologyProvider("https://tx.fhir.org/r4")

v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithTerminologyProvider(txProvider),
)
```

## Per-Call Profile Selection

In a server context, different API calls may need to validate against different profiles. Use `ValidateWithProfile()` and `ValidateWithCanonicalProfile()` to specify profiles per validation call without modifying the validator's construction-time configuration.

### ValidateWithProfile

```go
// Validate against a specific profile for this call only
result, err := v.Validate(ctx, resource,
    validator.ValidateWithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
)
```

### ValidateWithCanonicalProfile

For version-aware profile resolution, use canonical references with the `url|version` syntax:

```go
// Validate against a specific version of a profile
result, err := v.Validate(ctx, resource,
    validator.ValidateWithCanonicalProfile("http://example.org/fhir/StructureDefinition/my-profile|1.2.0"),
)
```

### Multiple Profiles Per Call

You can combine multiple per-call profiles. The resource must be valid against **all** specified profiles:

```go
result, err := v.Validate(ctx, resource,
    validator.ValidateWithProfile("http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"),
    validator.ValidateWithProfile("http://example.org/fhir/StructureDefinition/my-patient"),
)
```

## Complete HTTP Handler Example

Here is a complete example of an HTTP handler that validates incoming FHIR resources and returns an OperationOutcome:

```go
package main

import (
    "context"
    "encoding/json"
    "io"
    "log"
    "net/http"

    "github.com/gofhir/validator/pkg/issue"
    "github.com/gofhir/validator/pkg/validator"
)

var v *validator.Validator

func init() {
    var err error
    v, err = validator.New(
        validator.WithVersion("4.0.1"),
        validator.WithPackage("hl7.fhir.us.core", "6.1.0"),
    )
    if err != nil {
        log.Fatal(err)
    }
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // Check for profile query parameter
    var opts []validator.ValidateOption
    if profile := r.URL.Query().Get("profile"); profile != "" {
        opts = append(opts, validator.ValidateWithProfile(profile))
    }

    result, err := v.Validate(r.Context(), body, opts...)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Build OperationOutcome
    outcome := buildOperationOutcome(result)

    w.Header().Set("Content-Type", "application/fhir+json")
    if result.ErrorCount() > 0 {
        w.WriteHeader(http.StatusUnprocessableEntity)
    } else {
        w.WriteHeader(http.StatusOK)
    }
    json.NewEncoder(w).Encode(outcome)
}

func buildOperationOutcome(result *issue.Result) map[string]any {
    issues := make([]map[string]any, 0, len(result.Issues))
    for _, iss := range result.Issues {
        entry := map[string]any{
            "severity":    string(iss.Severity),
            "code":        string(iss.Code),
            "diagnostics": iss.Diagnostics,
        }
        if len(iss.Expression) > 0 {
            entry["expression"] = iss.Expression
        }
        issues = append(issues, entry)
    }

    return map[string]any{
        "resourceType": "OperationOutcome",
        "issue":        issues,
    }
}

func main() {
    http.HandleFunc("/fhir/$validate", validateHandler)
    log.Println("FHIR Validation server listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Usage

```bash
# Validate a Patient resource
curl -X POST http://localhost:8080/fhir/\$validate \
  -H "Content-Type: application/fhir+json" \
  -d @patient.json

# Validate against a specific profile
curl -X POST "http://localhost:8080/fhir/\$validate?profile=http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient" \
  -H "Content-Type: application/fhir+json" \
  -d @patient.json
```

{{< callout type="info" >}}
The `Validator` is safe for concurrent use. A single instance can serve all HTTP requests without additional synchronization. See the [Configuration](../configuration/#thread-safety) page for details on thread safety.
{{< /callout >}}
