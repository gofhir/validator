// Package ucumvalidator validates UCUM codes in Quantity elements.
package ucumvalidator

import (
	"context"
	"sync"

	"github.com/gofhir/validator/v2/internal/elementvalues"
	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/registry"
	"github.com/gofhir/validator/v2/pkg/walker"

	"github.com/gofhir/ucum/v4"
)

const (
	ucumSystem       = "http://unitsofmeasure.org"
	quantityTypeName = "Quantity"
)

// Validator validates UCUM codes in Quantity elements.
type Validator struct {
	registry      *registry.Registry
	walker        *walker.Walker
	ucum          ucum.Service
	quantityCache sync.Map // map[string]bool — caches whether a type derives from Quantity
}

// New creates a new UCUM validator.
func New(reg *registry.Registry, svc ucum.Service) *Validator {
	return &Validator{
		registry: reg,
		walker:   walker.New(reg),
		ucum:     svc,
	}
}

// ValidateData validates UCUM codes in Quantity elements of a pre-parsed resource.
func (v *Validator) ValidateData(data map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	v.ValidateDataContext(context.Background(), data, sd, result)
}

// ValidateDataContext is ValidateData, with what ctx's store found of the resources data holds
// (walker.WithDiscoveries).
func (v *Validator) ValidateDataContext(ctx context.Context, data map[string]any, sd *registry.StructureDefinition, result *issue.Result) {
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType := sd.RootName(data)
	if resourceType == "" {
		return
	}

	v.walkResource(ctx, data, sd, resourceType, result)

	// Every resource it holds, at any depth: walkNode stops at the elements that hold them.
	v.walker.WalkAllContext(ctx, data, resourceType, resourceType, func(rc *walker.ResourceContext) bool {
		if rc.FHIRPath == resourceType {
			return true
		}
		v.walkResource(ctx, rc.Data, rc.SD, rc.FHIRPath, result)
		return true
	})
}

// walkResource checks the Quantities of a resource (or a datatype value validated against its own
// definition), from the root of sd.
func (v *Validator) walkResource(ctx context.Context, data map[string]any, sd *registry.StructureDefinition, fhirPath string, result *issue.Result) {
	if root := sd.Tree().Root(); root != nil {
		v.walkNode(ctx, data, sd, root, "", fhirPath, result)
	}
}

// walkNode checks the Quantities among the values inst holds for the children of node, and walks
// into the others, following the definitions: a backbone's children inline, a contentReference's
// target, a datatype's own definition (registry.ChildrenOf). A resource an element holds is the
// walker's.
func (v *Validator) walkNode(ctx context.Context, inst map[string]any, sd *registry.StructureDefinition, node *registry.ElementNode, typeCode, fhirPath string, result *issue.Result) {
	children := v.registry.ChildrenOf(ctx, sd, node, typeCode, nil)
	for _, child := range children.Nodes {
		if v.registry.HoldsResource(child.Def) {
			continue
		}
		quantity := v.isQuantityType(child.Def)
		for _, cv := range elementvalues.Of(child, inst, v.registry) {
			m, ok := cv.Value.(map[string]any)
			if !ok {
				continue
			}
			path := cv.Path(fhirPath)
			if (cv.TypeCode != "" && v.derivesFromQuantity(cv.TypeCode)) || (cv.TypeCode == "" && quantity) {
				v.validateQuantity(m, path, result)
				continue
			}
			v.walkNode(ctx, m, children.SD, child, cv.TypeCode, path, result)
		}
	}
}

// validateQuantity checks the code field of a Quantity element.
func (v *Validator) validateQuantity(data map[string]any, fhirPath string, result *issue.Result) {
	system, _ := data["system"].(string)
	code, _ := data["code"].(string)

	if system != ucumSystem || code == "" {
		return
	}

	if err := v.ucum.Validate(code); err != nil {
		result.AddErrorWithID(
			issue.DiagUCUMInvalidCode,
			map[string]any{"code": code, "error": err.Error()},
			fhirPath+".code",
		)
	}
}

// isQuantityType checks if any of the element's types derive from Quantity
// by walking the baseDefinition chain in the registry. Results are cached.
func (v *Validator) isQuantityType(elemDef *registry.ElementDefinition) bool {
	for _, t := range elemDef.Type {
		if v.derivesFromQuantity(t.Code) {
			return true
		}
	}
	return false
}

// derivesFromQuantity checks whether a type derives from Quantity by walking
// the StructureDefinition.baseDefinition chain. Results are cached for performance.
func (v *Validator) derivesFromQuantity(typeName string) bool {
	if typeName == quantityTypeName {
		return true
	}

	if cached, ok := v.quantityCache.Load(typeName); ok {
		b, _ := cached.(bool)
		return b
	}

	result := v.walkBaseChain(typeName)
	v.quantityCache.Store(typeName, result)
	return result
}

// walkBaseChain walks the baseDefinition chain to check if a type ultimately derives from Quantity.
func (v *Validator) walkBaseChain(typeName string) bool {
	sd := v.registry.GetByType(typeName)
	if sd == nil {
		return false
	}

	visited := make(map[string]bool)
	for sd != nil {
		if sd.Type == quantityTypeName {
			return true
		}
		if sd.BaseDefinition == "" || visited[sd.BaseDefinition] {
			return false
		}
		visited[sd.BaseDefinition] = true
		sd = v.registry.GetByURL(sd.BaseDefinition)
	}
	return false
}
