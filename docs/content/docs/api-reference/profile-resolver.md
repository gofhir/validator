---
title: "Profile Resolver"
linkTitle: "Profile Resolver"
description: "Interface for on-demand StructureDefinition loading from external sources."
weight: 4
---

The `ProfileResolver` interface enables the validator to fetch StructureDefinitions on demand from an external source, such as a database or a remote FHIR server. This is the bridge between the validator and your conformance resource store.

```go
import "github.com/gofhir/validator/pkg/registry"
```

## Interface

```go
type ProfileResolver interface {
    ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
}
```

### ResolveProfile

```go
ResolveProfile(ctx context.Context, url, version string) ([]byte, error)
```

Fetches a StructureDefinition by its canonical URL and optional version. The method returns the raw JSON bytes of the StructureDefinition resource.

**Return values:**

| Return | Meaning |
|--------|---------|
| `(data, nil)` | StructureDefinition found; `data` is the raw JSON bytes |
| `(nil, nil)` | StructureDefinition not found -- this is **not** an error condition |
| `(nil, error)` | Resolver failure (network, database, etc.) |

When `version` is empty, the resolver should return the latest available version. Per the FHIR specification: *"If no version is specified, the server SHOULD return the latest version it has."*

{{< callout type="warning" >}}
Returning `(nil, nil)` means "not found" and the validator will continue without that profile, potentially emitting a warning. Returning a non-nil error signals a resolver failure and may surface as a validation error.
{{< /callout >}}

---

## When to Use

The `ProfileResolver` is **optional**. The validator works in two modes:

| Mode | Behavior | ProfileResolver |
|------|----------|-----------------|
| **Standalone** (CLI) | All StructureDefinitions are pre-loaded at init from packages | Not needed |
| **Server** (embedded) | The validator is embedded in a FHIR server that stores SDs in a database | Provides on-demand loading from the server's conformance store |

In standalone mode, the validator loads everything into memory during construction. In server mode, the resolver acts as a fallback for profiles not found in the in-memory registry.

---

## Example Implementation

The following example loads StructureDefinitions from a database:

```go
package myresolver

import (
    "context"
    "database/sql"

    "github.com/gofhir/validator/pkg/registry"
)

// Ensure interface compliance at compile time.
var _ registry.ProfileResolver = (*DBProfileResolver)(nil)

// DBProfileResolver loads StructureDefinitions from a SQL database.
type DBProfileResolver struct {
    db *sql.DB
}

// NewDBProfileResolver creates a resolver backed by the given database.
func NewDBProfileResolver(db *sql.DB) *DBProfileResolver {
    return &DBProfileResolver{db: db}
}

func (r *DBProfileResolver) ResolveProfile(
    ctx context.Context, url, version string,
) ([]byte, error) {
    var data []byte
    var err error

    if version != "" {
        // Version-specific lookup
        err = r.db.QueryRowContext(ctx,
            `SELECT resource_json
             FROM structure_definitions
             WHERE url = $1 AND version = $2`,
            url, version,
        ).Scan(&data)
    } else {
        // Latest version (ordered by version descending)
        err = r.db.QueryRowContext(ctx,
            `SELECT resource_json
             FROM structure_definitions
             WHERE url = $1
             ORDER BY version DESC
             LIMIT 1`,
            url,
        ).Scan(&data)
    }

    if err == sql.ErrNoRows {
        return nil, nil // not found -- not an error
    }
    if err != nil {
        return nil, err // resolver failure
    }

    return data, nil
}
```

---

## Wiring the Resolver

Pass your implementation to the validator using `WithProfileResolver`:

```go
db, _ := sql.Open("postgres", connStr)
resolver := myresolver.NewDBProfileResolver(db)

v, err := validator.New(
    validator.WithProfileResolver(resolver),
)
```

When the validator encounters a profile reference (in `meta.profile`, `ElementDefinition.type.profile`, or `Extension.url`) that is not already loaded in the in-memory registry, it calls your resolver to fetch it. The resolved StructureDefinition is then cached in the registry for subsequent lookups.

---

## Use Case: FHIR Server Integration

A typical FHIR server stores StructureDefinitions as regular FHIR resources in its database. When the server needs to validate an incoming resource:

1. The server creates the `Validator` once at startup with `WithProfileResolver` pointing at the database.
2. Base FHIR StructureDefinitions are pre-loaded from embedded packages.
3. When a resource claims conformance to a custom profile (via `meta.profile`), the validator checks the in-memory registry first.
4. If the profile is not in memory, the resolver fetches it from the database.
5. The fetched StructureDefinition is cached in memory for future validations.

```go
// At server startup
v, err := validator.New(
    validator.WithVersion("4.0.1"),
    validator.WithProfileResolver(myresolver.NewDBProfileResolver(db)),
)

// On each incoming request
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)

    result, err := s.validator.Validate(r.Context(), body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if result.HasErrors() {
        // Return OperationOutcome with validation errors
        w.WriteHeader(http.StatusUnprocessableEntity)
        writeOperationOutcome(w, result)
        return
    }

    // Resource is valid -- persist it
    s.store.Create(r.Context(), body)
    w.WriteHeader(http.StatusCreated)
}
```
