// Package ucumvalidator validates UCUM codes in Quantity elements.
package ucumvalidator

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/registry"
	"github.com/gofhir/validator/pkg/walker"

	"github.com/gofhir/ucum"
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
	if sd == nil || sd.Snapshot == nil {
		return
	}

	resourceType, _ := data["resourceType"].(string)
	if resourceType == "" {
		return
	}

	v.walkElement(data, sd, resourceType, resourceType, result)

	v.walker.Walk(data, resourceType, resourceType, func(ctx *walker.ResourceContext) bool {
		if ctx.FHIRPath == resourceType {
			return true
		}
		v.walkElement(ctx.Data, ctx.SD, ctx.ResourceType, ctx.FHIRPath, result)
		return true
	})
}

// walkElement recursively walks an element looking for Quantity types.
func (v *Validator) walkElement(data map[string]any, sd *registry.StructureDefinition, sdPath, fhirPath string, result *issue.Result) {
	for key, value := range data {
		if key == "resourceType" {
			continue
		}

		elemDef := v.findElement(sd, sdPath+"."+key, key)
		if elemDef == nil {
			continue
		}

		elemFHIRPath := fhirPath + "." + key
		if v.isQuantityType(elemDef) {
			v.validateQuantityValue(value, elemFHIRPath, result)
		} else {
			v.walkChildValue(value, elemDef, elemFHIRPath, result)
		}
	}
}

// validateQuantityValue validates UCUM codes for a value that is a Quantity.
func (v *Validator) validateQuantityValue(value any, fhirPath string, result *issue.Result) {
	switch val := value.(type) {
	case map[string]any:
		v.validateQuantity(val, fhirPath, result)
	case []any:
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				v.validateQuantity(m, fmt.Sprintf("%s[%d]", fhirPath, i), result)
			}
		}
	}
}

// walkChildValue recursively walks a non-Quantity child value.
func (v *Validator) walkChildValue(value any, elemDef *registry.ElementDefinition, fhirPath string, result *issue.Result) {
	typeName := v.getTypeName(elemDef)
	if typeName == "" {
		return
	}
	typeSD := v.registry.GetByType(typeName)
	if typeSD == nil || typeSD.Kind == "primitive-type" {
		return
	}

	switch val := value.(type) {
	case map[string]any:
		v.walkElement(val, typeSD, typeName, fhirPath, result)
	case []any:
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				v.walkElement(m, typeSD, typeName, fmt.Sprintf("%s[%d]", fhirPath, i), result)
			}
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

// getTypeName returns the single type name for an element, or empty if ambiguous.
func (v *Validator) getTypeName(elemDef *registry.ElementDefinition) string {
	if len(elemDef.Type) == 1 {
		return elemDef.Type[0].Code
	}
	return ""
}

// findElement looks up an ElementDefinition, handling choice types.
func (v *Validator) findElement(sd *registry.StructureDefinition, path, key string) *registry.ElementDefinition {
	for i := range sd.Snapshot.Element {
		elem := &sd.Snapshot.Element[i]
		if elem.Path == path {
			return elem
		}
		if !strings.HasSuffix(elem.Path, "[x]") {
			continue
		}
		basePath := strings.TrimSuffix(elem.Path, "[x]")
		baseName := basePath[strings.LastIndex(basePath, ".")+1:]
		if strings.HasPrefix(key, baseName) && len(key) > len(baseName) {
			suffix := key[len(baseName):]
			if suffix != "" && suffix[0] >= 'A' && suffix[0] <= 'Z' {
				return elem
			}
		}
	}
	return nil
}
