package registry

import (
	"encoding/json"
	"maps"
)

// Clone returns a registry holding the definitions this one holds, to which more can be loaded
// without changing this one: a base registry loaded once, cloned for each set of profiles added to
// it. The definitions that ship a snapshot are shared, as they are only read. One whose snapshot is
// generated depends on what the registry generating it resolves its base and type profiles to, so
// the clone holds its own copy of each one that does not ship a snapshot, to generate it from the
// definitions it holds. The clone resolves through the same ProfileResolver.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	own := make(map[*StructureDefinition]*StructureDefinition)
	for _, sd := range r.all {
		if c := sd.unpublishedCopy(); c != nil {
			own[sd] = c
		}
	}
	held := func(sd *StructureDefinition) *StructureDefinition {
		if c, ok := own[sd]; ok {
			return c
		}
		return sd
	}
	heldIn := func(m map[string]*StructureDefinition) map[string]*StructureDefinition {
		c := make(map[string]*StructureDefinition, len(m))
		for k, sd := range m {
			c[k] = held(sd)
		}
		return c
	}
	all := make([]*StructureDefinition, len(r.all))
	for i, sd := range r.all {
		all[i] = held(sd)
	}
	c := &Registry{
		all:                all,
		byURL:              heldIn(r.byURL),
		byURLVersion:       heldIn(r.byURLVersion),
		byType:             heldIn(r.byType),
		elementDefCache:    make(map[string]*ElementDefinition),
		fhirVersion:        r.fhirVersion,
		publishers:         r.publishers.Clone(),
		resolver:           r.resolver,
		domainResources:    maps.Clone(r.domainResources),
		canonicalResources: maps.Clone(r.canonicalResources),
		metadataResources:  maps.Clone(r.metadataResources),
	}
	c.model = &FHIRPathModel{reg: c}
	return c
}

// unpublishedCopy returns sd as it was loaded, before a snapshot was generated for it, when it does
// not ship one; nil when it does.
func (sd *StructureDefinition) unpublishedCopy() *StructureDefinition {
	sd.snapshotMu.Lock()
	published := sd.Snapshot != nil && !sd.snapshotGenerated
	sd.snapshotMu.Unlock()
	if published || sd.raw == nil {
		return nil
	}
	var c StructureDefinition
	if err := json.Unmarshal(sd.raw, &c); err != nil {
		return nil
	}
	c.raw = sd.raw
	c.PackageID = sd.PackageID
	applyErrata(&c)
	return &c
}
