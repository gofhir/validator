package terminology

import (
	"encoding/json"
	"strings"

	"github.com/gofhir/validator/v2/pkg/loader"
)

// A NamingSystem of kind codesystem identifies a code system by its unique ids
// (namingsystem.html), which a Coding's system may be although no CodeSystem with it as its url is
// loaded: the code system is published, its codes are not here.

// hl7Terminology reports whether pkg is HL7's terminology package (hl7.terminology, and its versions
// for a FHIR release), whose NamingSystems name the code systems HL7 publishes; another package's
// NamingSystem does not make a system HL7's.
func hl7Terminology(pkg *loader.Package) bool {
	return pkg.Name == "hl7.terminology" || strings.HasPrefix(pkg.Name, "hl7.terminology.")
}

// indexNamingSystem holds the preferred uri unique ids of a NamingSystem of kind codesystem: the
// identifier the code system is published by (NamingSystem.uniqueId.preferred), not an alias of it.
func (r *Registry) indexNamingSystem(data json.RawMessage) {
	r.nsMu.Lock()
	defer r.nsMu.Unlock()
	r.indexNamingSystemLocked(data)
}

// indexNamingSystemLocked is indexNamingSystem, nsMu held.
func (r *Registry) indexNamingSystemLocked(data json.RawMessage) {
	var ns struct {
		Kind     string `json:"kind"`
		UniqueID []struct {
			Type      string `json:"type"`
			Value     string `json:"value"`
			Preferred bool   `json:"preferred"`
		} `json:"uniqueId"`
	}
	if json.Unmarshal(data, &ns) != nil || ns.Kind != "codesystem" {
		return
	}
	for _, id := range ns.UniqueID {
		if id.Type == "uri" && id.Preferred && id.Value != "" {
			if r.namingSystems == nil {
				r.namingSystems = make(map[string]bool)
			}
			r.namingSystems[id.Value] = true
		}
	}
}

// NamesCodeSystem reports whether a NamingSystem loaded, of kind codesystem, has uri as its preferred
// unique id of type uri. The NamingSystems a package defers are read the first time it is asked, under
// a lock of their own: the registry's other lookups do not wait for them.
func (r *Registry) NamesCodeSystem(uri string) bool {
	r.nsMu.Lock()
	defer r.nsMu.Unlock()
	for _, read := range r.pendingNamingSystems {
		if data, err := read(); err == nil {
			r.indexNamingSystemLocked(data)
		}
	}
	r.pendingNamingSystems = nil
	return r.namingSystems[uri]
}
