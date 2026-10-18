package terminology

// VersionMembership decides whether code of system, in a version of the code system that is not
// loaded, is in the value set at valueSetURL, from its composition alone (valueset.html#compositions:
// the codes its includes select, a union, but those its excludes select): Valid, Invalid, or
// Unresolved where what a clause selects is not known without that version of the code system, or
// the value set is not held or has no composition. A clause selects as clause tells.
//
// The value sets a composition takes in are decided together, as the least fixed point of their
// compositions: each starts not decided and is decided again when one it takes in is, until none
// changes. A value set's answer can only go from not decided to decided (union, intersection and
// exclusion are monotone in that order), so each is decided at most once more than it takes value
// sets in, whatever the order of its includes, and one that takes itself in is decided as far as its
// other clauses tell.
func (r *Registry) VersionMembership(valueSetURL, system, version, code string) Resolution {
	m := &versionWalk{
		r: r, system: system, version: version, code: code,
		value: map[versionKey]Resolution{}, dependents: map[versionKey][]versionKey{},
		seen: map[versionKey]bool{}, queued: map[versionKey]bool{},
	}
	root := versionKey{valueSetURL, false, false}
	m.discover(root)
	for len(m.pending) > 0 {
		key := m.pending[0]
		m.pending = m.pending[1:]
		m.queued[key] = false
		if res := m.decide(key); res != m.value[key] {
			m.value[key] = res
			for _, d := range m.dependents[key] {
				m.enqueue(d)
			}
		}
	}
	return m.value[root]
}

// versionWalk decides the membership of one code in the value sets a composition takes in: value
// holds each one's answer so far (not decided until decided), dependents those that take each in,
// pending those to decide again.
type versionWalk struct {
	r                     *Registry
	system, version, code string
	value                 map[versionKey]Resolution
	dependents            map[versionKey][]versionKey
	seen, queued          map[versionKey]bool
	pending               []versionKey
	current               versionKey
}

// versionKey is a value set, whether the code is taken to exist in the version, and whether a locked
// date of a value set taking it in fixes its code systems' versions (decide).
type versionKey struct {
	url            string
	exists, locked bool
}

func (m *versionWalk) discover(key versionKey) {
	if !m.seen[key] {
		m.seen[key] = true
		m.value[key] = Unresolved
		m.enqueue(key)
	}
}

func (m *versionWalk) enqueue(key versionKey) {
	if !m.queued[key] {
		m.queued[key] = true
		m.pending = append(m.pending, key)
	}
}

// taken returns the answer so far for a value set the one being decided takes in, which is decided
// again when that answer changes.
func (m *versionWalk) taken(url string, exists, locked bool) Resolution {
	key := versionKey{url, exists, locked}
	m.discover(key)
	if deps := m.dependents[key]; len(deps) == 0 || deps[len(deps)-1] != m.current {
		m.dependents[key] = append(deps, m.current)
	}
	return m.value[key]
}

// decide decides the value set of key from its composition and the answers so far of those it takes
// in. Exists takes the code to be one of that version of the code system, as where the value set is
// excluded from another: whether it is excluded is what is asked, its being a code of the version is
// the include's to tell. Locked is a locked date of a value set taking it in, which fixes the versions
// of the code systems and value sets its composition names without one (ValueSet.compose.lockedDate).
func (m *versionWalk) decide(key versionKey) Resolution {
	vs := m.r.GetValueSet(key.url)
	if vs == nil || len(vs.Compose.Include) == 0 {
		return Unresolved
	}
	m.current = key
	exists, locked := key.exists, key.locked || vs.Compose.LockedDate != ""
	in := Invalid
	for _, inc := range vs.Compose.Include {
		switch m.clause(inc, false, exists, locked) {
		case Valid:
			in = Valid
		case Unresolved:
			if in != Valid {
				in = Unresolved
			}
		case Invalid:
		}
	}
	for _, exc := range vs.Compose.Exclude {
		if in == Invalid {
			break
		}
		switch m.clause(exc, true, exists, locked) {
		case Valid:
			in = Invalid // excluded
		case Unresolved:
			if in == Valid {
				in = Unresolved
			}
		case Invalid:
		}
	}
	return in
}

// clause tells whether an include or exclude selects the code in the version: what its parts all
// select (an intersection), its system part as systemClause tells and the value sets it names as
// their compositions tell. Within an exclude the code is taken to exist in the version; within an
// include too where a part lists it, which makes it one of the version's codes. One with a
// modifierExtension selects what is not known here.
func (m *versionWalk) clause(c Include, exclude, exists, locked bool) Resolution {
	if len(c.ModifierExtension) > 0 || (c.System == "" && len(c.ValueSet) == 0) {
		return Unresolved
	}
	exists = exists || exclude
	res, listed := m.parts(c, exists, locked)
	if res == Unresolved && !exists && listed {
		res, _ = m.parts(c, true, locked)
	}
	return res
}

// parts intersects what a clause's parts select, and reports whether one selects the code (Valid).
func (m *versionWalk) parts(c Include, exists, locked bool) (res Resolution, listed bool) {
	res = Valid
	add := func(part Resolution) {
		switch part {
		case Invalid:
			res = Invalid
		case Unresolved:
			if res != Invalid {
				res = Unresolved
			}
		case Valid:
			listed = true
		}
	}
	if c.System != "" {
		add(systemClause(c, exists, locked, m.system, m.version, m.code))
	}
	for _, url := range c.ValueSet {
		if res == Invalid {
			break
		}
		add(m.taken(url, exists, locked))
	}
	return res, listed
}

// systemClause tells whether a clause's system part selects code of system in version: not where it
// names another system, or a version other than that one (none, or "*" for every version, admits
// it), or lists other codes; where it lists the code; where it takes the whole system, where the
// code is taken to exist in the version (exists), else not known; not known where it filters the
// system, lists the code with a modifierExtension, or names no version under a locked date, which
// fixes one that is not known.
func systemClause(c Include, exists, locked bool, system, version, code string) Resolution {
	switch {
	case c.System != system || (c.Version != "" && c.Version != "*" && c.Version != version):
		return Invalid
	case len(c.Filter) > 0:
		return Unresolved
	case len(c.Concept) == 0:
		if exists && (c.Version != "" || !locked) {
			return Valid
		}
		return Unresolved
	}
	for _, concept := range c.Concept {
		if concept.Code == code {
			if len(concept.ModifierExtension) > 0 || (c.Version == "" && locked) {
				return Unresolved // what listing it means is not known (membership.go), or in which version
			}
			return Valid
		}
	}
	return Invalid
}
