# Issue: `as()` function fails with SingletonExpectedError on collections

## Summary

The `as()` function throws `SingletonExpectedError` when called on a collection, but according to FHIRPath spec, `as()` should work on collections and filter elements that can be cast to the specified type.

## Reproduction

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    resource := `{
        "resourceType": "Patient",
        "id": "123",
        "contained": [{
            "resourceType": "Organization",
            "id": "org1",
            "name": "Test"
        }],
        "managingOrganization": {
            "reference": "#org1"
        }
    }`

    var data json.RawMessage = []byte(resource)

    // This fails with SingletonExpectedError
    expr := "%resource.descendants().as(canonical)"

    compiled, _ := fhirpath.Compile(expr)
    result, err := compiled.Evaluate(data)
    fmt.Printf("Error: %v\n", err)
    // Output: Error: SingletonExpectedError: expected single value, got 8 elements
}
```

## Expected Behavior

According to [FHIRPath spec](http://hl7.org/fhirpath/#as-type-specifier):

> If the left operand is a collection with a single item, this operator returns the value if it is of the specified type, or an empty collection otherwise.
> **If there is more than one item in the input collection, the evaluator will throw an error.**

However, in practice, the FHIR specification uses `as()` on collections in constraints like `dom-3`:

```fhirpath
%resource.descendants().as(canonical)
```

The HL7 FHIR Validator evaluates this successfully, treating it as a filter/projection operation on collections.

## Impact

This prevents evaluation of FHIR constraint `dom-3` which validates contained resource references:

```fhirpath
contained.where((('#'+id in (%resource.descendants().reference | %resource.descendants().as(canonical) | %resource.descendants().as(uri) | %resource.descendants().as(url))) or descendants().where(reference = '#').exists() or descendants().where(as(canonical) = '#').exists() or descendants().where(as(canonical) = '#').exists()).not()).trace('unmatched', id).empty()
```

## Suggested Fix

Two options:

### Option 1: Treat `as()` as a projection/filter on collections

When `as(type)` is called on a collection:
- Return elements that can be successfully cast to `type`
- Return empty collection for elements that cannot be cast

```go
// Instead of:
if len(input) != 1 {
    return nil, SingletonExpectedError{...}
}

// Do:
result := make(Collection, 0)
for _, item := range input {
    if casted, ok := tryCast(item, targetType); ok {
        result = append(result, casted)
    }
}
return result, nil
```

### Option 2: Add `asCollection()` or similar function

Add a separate function that handles the collection case, keeping strict singleton behavior for `as()`.

## References

- FHIRPath spec: http://hl7.org/fhirpath/#as-type-specifier
- dom-3 constraint in DomainResource: http://hl7.org/fhir/R4/domainresource.html
- HL7 Validator behavior: Successfully evaluates dom-3 expressions
