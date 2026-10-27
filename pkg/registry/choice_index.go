package registry

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofhir/validator/v2/internal/versionorder"
)

// choiceIndex dice qué propiedades JSON de una ElementDefinition son un mismo elemento de elección
// (fixed[x], pattern[x]…), según la definición de ElementDefinition de una versión. En JSON, un
// elemento de elección es una sola propiedad, cuyo nombre lleva el tipo con la primera letra en
// mayúscula (json.html#choice), y su id y sus extension van en el hermano "_" (json.html#primitive).
type choiceIndex struct {
	// owner lleva el nombre de cada propiedad, sin "_", al de su elemento de elección
	// ("fixedString" → "fixed").
	owner map[string]string
	// names son los nombres de los elementos de elección ("fixed", "pattern"…).
	names []string
	// children son los nombres de los demás elementos hijos de ElementDefinition ("maxLength").
	children map[string]bool
	// version es la versión FHIR de la definición de la que sale el índice.
	version string
}

// errNoElementDefinition es la falla de un diferencial con elementos cuando no hay definición de
// ElementDefinition: sin ella no se sabe qué propiedades son un mismo elemento de elección.
var errNoElementDefinition = fmt.Errorf("%s is not loaded: the choice elements of the differential cannot be merged", elementDefinitionURL)

// sameRelease dice si la definición sirve para un perfil de esa versión FHIR. Una definición que no
// declara su versión no dice ser de otra, como en fhirRank.
func sameRelease(sd *StructureDefinition, version string) bool {
	return sd.FHIRVersion == "" || versionorder.SameRelease(sd.FHIRVersion, version)
}

// choiceIndexFor arma el índice desde la definición de ElementDefinition de la versión del perfil.
// La busca primero en el registro: la de esa versión o, si no está, la preferida cuando es de la misma
// release (4.0.0 y 4.0.1), sin consultar al ProfileResolver. Si no está y hay resolver, le pide esa
// versión o, si no la tiene, la última de la misma release; si no entrega ninguna, es un error, sin
// caer a otra release, porque una falla del resolver no se distingue de una ausencia. Sin resolver, usa la preferida aunque sea de otra release. Ninguna
// falla de la definición es permanente para el perfil: puede cargarse después.
func (r *Registry) choiceIndexFor(ctx context.Context, version string) (*choiceIndex, error) {
	sd := r.GetByCanonical(elementDefinitionURL, version)
	if sd == nil {
		if loaded := r.GetByCanonical(elementDefinitionURL, ""); loaded != nil && sameRelease(loaded, version) {
			sd = loaded
		}
	}
	if sd == nil {
		r.mu.RLock()
		resolver := r.resolver
		r.mu.RUnlock()
		if resolver != nil {
			sd = r.ResolveByCanonical(ctx, elementDefinitionURL, version)
			if sd == nil && version != "" {
				if resolved := r.ResolveByCanonical(ctx, elementDefinitionURL, ""); resolved != nil && sameRelease(resolved, version) {
					sd = resolved
				}
			}
		} else {
			sd = r.GetByCanonical(elementDefinitionURL, "")
		}
	}
	if sd == nil {
		return nil, errNoElementDefinition
	}
	snapshot, err := r.profileSnapshot(ctx, sd)
	if err != nil {
		// %v y no %w: un permanentError de la definición no es permanente para el perfil.
		return nil, fmt.Errorf("%s: %v", elementDefinitionURL, err) //nolint:errorlint // ver arriba
	}
	if snapshot == nil {
		return nil, fmt.Errorf("%s has no snapshot", elementDefinitionURL)
	}
	idx := &choiceIndex{owner: map[string]string{}, children: map[string]bool{}, version: sd.FHIRVersion}
	for i := range snapshot.Element {
		e := &snapshot.Element[i]
		child, ok := strings.CutPrefix(e.Path, sd.Type+".")
		if !ok || strings.Contains(child, ".") {
			continue
		}
		name, isChoice := strings.CutSuffix(child, "[x]")
		if !isChoice {
			idx.children[child] = true
			continue
		}
		idx.names = append(idx.names, name)
		for _, t := range e.Type {
			idx.owner[choiceProperty(name, t.Code)] = name
		}
	}
	return idx, nil
}

// choiceProperty es el nombre JSON del elemento de elección name con el tipo code.
func choiceProperty(name, code string) string {
	r, size := utf8.DecodeRuneInString(code)
	return name + string(unicode.ToUpper(r)) + code[size:]
}

// choiceOf devuelve el elemento de elección de la propiedad key, con o sin "_".
func (c *choiceIndex) choiceOf(key string) (string, bool) {
	name, ok := c.owner[strings.TrimPrefix(key, "_")]
	return name, ok
}

// foreign devuelve el elemento de elección cuyo nombre lleva key seguido de un tipo que la versión no
// declara (en R4, "fixedInteger64"): no es una propiedad de ElementDefinition.
func (c *choiceIndex) foreign(key string) (string, bool) {
	key = strings.TrimPrefix(key, "_")
	if _, ok := c.owner[key]; ok || c.children[key] {
		return "", false
	}
	for _, name := range c.names {
		if rest, ok := strings.CutPrefix(key, name); ok {
			if r, _ := utf8.DecodeRuneInString(rest); unicode.IsUpper(r) {
				return name, true
			}
		}
	}
	return "", false
}
