// Package fhirpathcache makes a FHIRPath collection keep what expressions read from it.
package fhirpathcache

import (
	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// Enable makes the resources in col keep what FHIRPath reads from them. The same collection is
// %resource and %rootResource for every constraint on a resource, so what one evaluation navigates
// is what the next starts from: without it, ref-1 on each of the 12,085 references of the R4
// ImplementationGuide-fhir example reads its 2.7 MB root again.
//
// A cached object must not be read from two goroutines at once. Call it only on a collection the
// caller built for one validation, which evaluates in one goroutine.
func Enable(col fhirpath.Collection) {
	for _, v := range col {
		if obj, ok := v.(*types.ObjectValue); ok {
			obj.EnableCaching()
		}
	}
}
