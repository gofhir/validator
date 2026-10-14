package terminology

import "strings"

// answer is whether a code is a member of a ValueSet, as far as the definitions this registry holds
// tell (valueset-definitions.html#ValueSet.compose).
type answer int

const (
	answerOut answer = iota
	answerIn
	// Not known: a definition it depends on cannot be enumerated here (an external code system, one
	// not held, a version not held, or one that does not include all its codes), a filter cannot be
	// evaluated, or a ValueSet is not held, or includes itself; a code is not known not to be a member.
	answerUnknown
)

// maxValueSetDepth bounds the ValueSets a ValueSet includes or excludes, one in another.
const maxValueSetDepth = 16

// either is the answer of one of two alternatives: a member of either is a member.
func either(a, b answer) answer {
	switch {
	case a == answerIn || b == answerIn:
		return answerIn
	case a == answerUnknown || b == answerUnknown:
		return answerUnknown
	}
	return answerOut
}

// both is the answer of two conditions together: a member of both is a member.
func both(a, b answer) answer {
	switch {
	case a == answerOut || b == answerOut:
		return answerOut
	case a == answerUnknown || b == answerUnknown:
		return answerUnknown
	}
	return answerIn
}

// membership asks, for one code, whether it is a member of ValueSets: each ValueSet is decided once
// (a ValueSet several others include), and one that includes itself, through others, is not known.
type membership struct {
	r            *Registry
	system, code string
	decided      map[*ValueSet]answer
	deciding     map[*ValueSet]bool
}

// memberKey is a code of a system ("" for a code element) asked of a ValueSet.
type memberKey struct {
	vs           *ValueSet
	system, code string
}

// memberOf tells whether code, of system ("" for a code element, whose system the ValueSet implies),
// is a member of vs; each answer is worked out once.
func (r *Registry) memberOf(vs *ValueSet, system, code string) answer {
	key := memberKey{vs, system, code}
	r.mu.RLock()
	a, cached := r.answerCache[key]
	gen := r.derivedGen
	r.mu.RUnlock()
	if cached {
		return a
	}
	a = r.decideMember(vs, system, code)
	r.mu.Lock()
	if r.derivedGen == gen {
		if len(r.answerCache) >= maxAnswers {
			clear(r.answerCache) // the codes a long-running registry is asked are not bounded
		}
		r.answerCache[key] = a
	}
	r.mu.Unlock()
	return a
}

// maxAnswers bounds the membership answers kept.
const maxAnswers = 1 << 16

func (r *Registry) decideMember(vs *ValueSet, system, code string) answer {
	if system != "" {
		return r.newMembership(system, code).of(vs, 0)
	}
	// A code element's code is a member when it is one as a code of a system the ValueSet includes:
	// excluding one system's code leaves another's.
	systems, unknown := r.systemsOf(vs)
	member := answerOut
	if unknown {
		member = answerUnknown
	}
	for _, s := range systems {
		if member = either(member, r.newMembership(s, code).of(vs, 0)); member == answerIn {
			break
		}
	}
	return member
}

func (r *Registry) newMembership(system, code string) *membership {
	return &membership{r: r, system: system, code: code, decided: map[*ValueSet]answer{}, deciding: map[*ValueSet]bool{}}
}

// of tells whether the code is a member of vs: one an include selects and no exclude does
// (valueset-definitions.html#ValueSet.compose.exclude: "Exclude one or more codes from the value
// set"); an exclusion that cannot be decided leaves the membership undecided. The composition
// defines the ValueSet; one without is its expansion's codes. An expansion a composition carries is
// not read: it is the codes the composition gave when it was expanded, against code system versions
// that may not be the ones held (a VSAC set's SNOMED CT).
func (m *membership) of(vs *ValueSet, depth int) answer {
	if a, ok := m.decided[vs]; ok {
		return a
	}
	if m.deciding[vs] || depth > maxValueSetDepth {
		return answerUnknown
	}
	m.deciding[vs] = true
	a := m.compose(vs, depth)
	delete(m.deciding, vs)
	m.decided[vs] = a
	return a
}

func (m *membership) compose(vs *ValueSet, depth int) answer {
	if len(vs.Compose.Include) > 0 {
		return m.composition(vs, depth)
	}
	if vs.Expansion == nil {
		return answerUnknown
	}
	exp := m.r.expansionCodes(vs)
	switch {
	case exp.codes[m.system+"|"+m.code]:
		return answerIn
	case vs.Expansion.Total != nil && *vs.Expansion.Total > exp.listed:
		return answerUnknown // a page of the expansion: the code may be on another
	}
	return answerOut
}

// composition decides the code by vs's composition (of).
func (m *membership) composition(vs *ValueSet, depth int) answer {
	in := answerOut
	for i := range vs.Compose.Include {
		if in = either(in, m.selects(&vs.Compose.Include[i], depth)); in == answerIn {
			break
		}
	}
	if in == answerOut {
		return answerOut
	}
	for i := range vs.Compose.Exclude {
		e := &vs.Compose.Exclude[i]
		if m.r.coversSystem(e, m.system, depth, map[*ValueSet]walk{}) {
			// The whole system is excluded: whatever an include selected of it, known or not.
			return answerOut
		}
		switch m.selects(e, depth) {
		case answerIn:
			return answerOut
		case answerUnknown:
			in = answerUnknown
		}
	}
	return in
}

// selects tells whether a clause (an include or an exclude) selects the code: its system's codes,
// the concepts it lists or its filters select, and, when it names ValueSets, a member of each of
// them too (valueset-definitions.html#ValueSet.compose.include.valueSet: "the content of the value
// set is the intersection").
func (m *membership) selects(c *Include, depth int) answer {
	selected := answerIn
	switch {
	case c.System != "":
		if m.system != c.System {
			return answerOut
		}
		selected = m.r.inSystem(c, m.system, m.code)
	case len(c.ValueSet) == 0:
		return answerOut // selects nothing
	}
	if len(c.ModifierExtension) > 0 && selected != answerOut {
		return answerUnknown // what it selects of the code's system is changed in a way not known here
	}
	for _, url := range c.ValueSet {
		if selected == answerOut {
			return answerOut
		}
		nested := m.r.GetValueSet(url)
		if nested == nil {
			selected = both(selected, answerUnknown)
			continue
		}
		selected = both(selected, m.of(nested, depth+1))
	}
	return selected
}

// coversSystem reports whether clause c selects every code of system, whether or not they are known
// here: it names the system with no concept nor filter, or a ValueSet that includes all of it, and
// any ValueSet it names includes all of it too. Each ValueSet is read once (walked).
func (r *Registry) coversSystem(c *Include, system string, depth int, walked map[*ValueSet]walk) bool {
	if depth > maxValueSetDepth || len(c.Concept) > 0 || len(c.Filter) > 0 || len(c.ModifierExtension) > 0 {
		return false
	}
	switch {
	case c.System == system:
	case c.System == "" && len(c.ValueSet) > 0:
	default:
		return false
	}
	for _, url := range c.ValueSet {
		nested := r.GetValueSet(url)
		if nested == nil || !r.includesSystem(nested, system, depth+1, walked) {
			return false
		}
	}
	return true
}

// includesSystem reports whether vs includes every code of system: a clause covers it, and no
// exclusion can take any away.
func (r *Registry) includesSystem(vs *ValueSet, system string, depth int, walked map[*ValueSet]walk) bool {
	switch walked[vs] {
	case walking:
		return false // it includes itself: not known to include all of it
	case walkedTrue:
		return true
	case walkedFalse:
		return false
	}
	walked[vs] = walking
	includes := r.includesWholeSystem(vs, system, depth, walked)
	walked[vs] = walkedFalse
	if includes {
		walked[vs] = walkedTrue
	}
	return includes
}

func (r *Registry) includesWholeSystem(vs *ValueSet, system string, depth int, walked map[*ValueSet]walk) bool {
	for i := range vs.Compose.Exclude {
		if e := &vs.Compose.Exclude[i]; e.System == system || e.System == "" {
			return false
		}
	}
	for i := range vs.Compose.Include {
		if r.coversSystem(&vs.Compose.Include[i], system, depth, walked) {
			return true
		}
	}
	return false
}

// walk is where a walk over ValueSets, one in another, is with one: being walked (one reached
// again then includes itself), or walked, with its answer. A ValueSet reached by two paths (a
// diamond) is walked once, and does not include itself.
type walk int

const (
	notWalked walk = iota
	walking
	walkedTrue
	walkedFalse
)

// includedSystems are the systems a ValueSet includes (systemsOf).
type includedSystems struct {
	systems []string
	unknown bool
}

// systemsOf returns the systems vs includes, its own and those of the ValueSets it includes, and
// whether one it includes is not held, or includes itself (its systems are not known); worked out
// once per ValueSet.
func (r *Registry) systemsOf(vs *ValueSet) (systems []string, unknown bool) {
	r.mu.RLock()
	s, cached := r.systemsCache[vs]
	gen := r.derivedGen
	r.mu.RUnlock()
	if cached {
		return s.systems, s.unknown
	}
	found := map[string]bool{}
	unknown = r.collectSystems(vs, found, map[*ValueSet]walk{}, 0)
	for system := range found {
		systems = append(systems, system)
	}
	r.mu.Lock()
	if r.derivedGen == gen {
		r.systemsCache[vs] = includedSystems{systems, unknown}
	}
	r.mu.Unlock()
	return systems, unknown
}

func (r *Registry) collectSystems(vs *ValueSet, found map[string]bool, walked map[*ValueSet]walk, depth int) (unknown bool) {
	switch {
	case walked[vs] == walking || depth > maxValueSetDepth:
		return true // it includes itself: its systems are not known
	case walked[vs] != notWalked:
		return false // reached before by another path: its systems are found
	}
	walked[vs] = walking
	defer func() { walked[vs] = walkedTrue }()
	for _, c := range vs.Compose.Include {
		if c.System != "" {
			found[c.System] = true
		}
		for _, url := range c.ValueSet {
			nested := r.GetValueSet(url)
			if nested == nil || r.collectSystems(nested, found, walked, depth+1) {
				unknown = true
			}
		}
	}
	if len(vs.Compose.Include) == 0 && vs.Expansion != nil {
		for key := range r.expansionCodes(vs).codes {
			if i := strings.LastIndex(key, "|"); i > 0 {
				found[key[:i]] = true
			}
		}
	}
	return unknown
}

// inSystem tells whether a clause's system part selects code: one of the concepts it lists, or of
// the codes its filters (or, without any, its code system) select.
func (r *Registry) inSystem(c *Include, system, code string) answer {
	if len(c.Concept) == 0 && r.isExternalSystem(c.System) {
		return answerUnknown
	}
	exp := r.clauseCodes(c)
	selected := exp.codes[system+"|"+code]
	switch {
	case exp.modified[system+"|"+code]:
		return answerUnknown // listed with a modifier extension: what that means is not known here
	case selected && !exp.undecided:
		return answerIn
	case selected:
		return answerUnknown // selected by the filters evaluated, not known of the others
	case !exp.complete:
		return answerUnknown
	}
	return answerOut
}

// clauseExpansion is what a clause selects, keyed "system|code": the concepts it lists, or what it
// selects from a code system this registry holds; complete when those are all it may select, and
// undecided when a filter that cannot be evaluated here may take some of them away.
type clauseExpansion struct {
	codes     map[string]bool
	complete  bool
	undecided bool
	// modified are the concepts it lists with a modifier extension.
	modified map[string]bool
}

// clauseCodes returns what clause c selects, built once.
func (r *Registry) clauseCodes(c *Include) clauseExpansion {
	r.mu.RLock()
	exp, cached := r.clauseCache[c]
	gen := r.derivedGen
	r.mu.RUnlock()
	if cached {
		return exp
	}
	exp = r.expandClause(c)
	r.mu.Lock()
	if r.derivedGen == gen {
		r.clauseCache[c] = exp
	}
	r.mu.Unlock()
	return exp
}

func (r *Registry) expandClause(c *Include) clauseExpansion {
	exp := clauseExpansion{codes: map[string]bool{}}
	if len(c.Concept) > 0 {
		for _, concept := range c.Concept {
			exp.codes[c.System+"|"+concept.Code] = true
			if len(concept.ModifierExtension) > 0 {
				if exp.modified == nil {
					exp.modified = map[string]bool{}
				}
				exp.modified[c.System+"|"+concept.Code] = true
			}
		}
		exp.complete = true
		return exp
	}
	var cs *CodeSystem
	if c.Version != "" {
		cs = r.GetCodeSystemVersion(c.System, c.Version)
	} else {
		cs = r.GetCodeSystem(c.System)
	}
	if cs == nil {
		return exp
	}
	// A version asked for that is not held is answered from another, whose codes may differ.
	exp.complete = !cs.partial() && (c.Version == "" || r.CodeSystemVersionMatches(c.System, c.Version))
	// The codes every filter selects (valueset-definitions.html#ValueSet.compose.include.filter:
	// "the filters are combined with AND"); with none evaluated here, every code.
	var selected map[string]bool
	for _, f := range c.Filter {
		if !filterOps[f.Op] || len(f.ModifierExtension) > 0 {
			exp.undecided = true
			continue
		}
		these := map[string]bool{}
		r.applyFilters(these, cs, c.System, []Filter{f})
		if selected == nil {
			selected = these
			continue
		}
		for key := range selected {
			if !these[key] {
				delete(selected, key)
			}
		}
	}
	if selected == nil {
		r.addCodesFromCodeSystem(exp.codes, cs, c.System)
		return exp
	}
	exp.codes = selected
	return exp
}

// expandedCodes are the codes a ValueSet's expansion contains, keyed "system|code", and how many
// entries list one (a code listed twice, in two places of its hierarchy, counts twice, as its
// total counts it).
type expandedCodes struct {
	codes  map[string]bool
	listed int
}

// expansionCodes returns the codes a ValueSet's expansion contains, read once.
func (r *Registry) expansionCodes(vs *ValueSet) expandedCodes {
	r.mu.RLock()
	exp, cached := r.expansionCache[vs]
	gen := r.derivedGen
	r.mu.RUnlock()
	if cached {
		return exp
	}
	exp = expandedCodes{codes: map[string]bool{}}
	var add func([]ExpansionContains)
	add = func(contains []ExpansionContains) {
		for _, c := range contains {
			if c.Code != "" {
				exp.codes[c.System+"|"+c.Code] = true
				exp.listed++
			}
			add(c.Contains)
		}
	}
	add(vs.Expansion.Contains)
	r.mu.Lock()
	if r.derivedGen == gen {
		r.expansionCache[vs] = exp
	}
	r.mu.Unlock()
	return exp
}

// filterOps are the filter operators evaluated here (codesystem-filter-operator.html); a clause
// with another (regex, exists, generalizes), or a filter a modifier extension changes, selects
// codes not known here.
var filterOps = map[string]bool{
	"is-a":          true,
	"descendent-of": true,
	"is-not-a":      true,
	"=":             true,
	"in":            true,
	"not-in":        true,
}
