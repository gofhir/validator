package pipeline

// PhaseGroup represents a group of phases that can be executed together.
// Phases in the same group have the same priority level.
type PhaseGroup struct {
	// Priority is the execution priority of this group
	Priority PhasePriority

	// Phases contains all phases in this group
	Phases []*PhaseConfig

	// Parallel indicates if phases in this group can run concurrently
	Parallel bool
}

// PhaseCount returns the number of phases in the group.
func (g *PhaseGroup) PhaseCount() int {
	return len(g.Phases)
}

// Names returns the names of all phases in the group.
func (g *PhaseGroup) Names() []string {
	names := make([]string, len(g.Phases))
	for i, cfg := range g.Phases {
		names[i] = cfg.Phase.Name()
	}
	return names
}

// StandardGroups defines the standard phase execution groups for FHIR validation.
// This follows the FHIR specification's validation order.
var StandardGroups = []struct {
	Priority PhasePriority
	Parallel bool
	Phases   []PhaseID
}{
	// Group 1: Structure validation (must run first)
	{
		Priority: PriorityFirst,
		Parallel: true,
		Phases:   []PhaseID{PhaseIDStructure, PhaseIDUnknownElems},
	},

	// Group 2: Type validation
	{
		Priority: PriorityEarly,
		Parallel: true,
		Phases:   []PhaseID{PhaseIDPrimitives, PhaseIDCardinality},
	},

	// Group 3: Value constraints
	{
		Priority: PriorityNormal,
		Parallel: true,
		Phases:   []PhaseID{PhaseIDFixedPattern, PhaseIDSlicing},
	},

	// Group 4: External validation (can be slow due to external calls)
	{
		Priority: PriorityNormal + 100,
		Parallel: true,
		Phases:   []PhaseID{PhaseIDTerminology, PhaseIDReferences, PhaseIDExtensions},
	},

	// Group 5: FHIRPath constraints (depends on all above)
	{
		Priority: PriorityLate,
		Parallel: false, // FHIRPath evaluation can have dependencies
		Phases:   []PhaseID{PhaseIDConstraints},
	},

	// Group 6: Special resource types
	{
		Priority: PriorityLate + 50,
		Parallel: true,
		Phases:   []PhaseID{PhaseIDBundle, PhaseIDQuestionnaire},
	},

	// Group 7: Profile validation (runs last)
	{
		Priority: PriorityLast,
		Parallel: false,
		Phases:   []PhaseID{PhaseIDProfile, PhaseIDMustSupport},
	},
}

// GroupBuilder helps construct custom phase groups.
type GroupBuilder struct {
	groups []*PhaseGroup
}

// NewGroupBuilder creates a new group builder.
func NewGroupBuilder() *GroupBuilder {
	return &GroupBuilder{
		groups: make([]*PhaseGroup, 0, 8),
	}
}

// AddGroup adds a new group with the specified priority.
func (b *GroupBuilder) AddGroup(priority PhasePriority, parallel bool) *GroupBuilder {
	b.groups = append(b.groups, &PhaseGroup{
		Priority: priority,
		Parallel: parallel,
		Phases:   make([]*PhaseConfig, 0, 4),
	})
	return b
}

// AddPhase adds a phase to the last group.
func (b *GroupBuilder) AddPhase(cfg *PhaseConfig) *GroupBuilder {
	if len(b.groups) == 0 {
		b.AddGroup(PriorityNormal, true)
	}
	lastGroup := b.groups[len(b.groups)-1]
	lastGroup.Phases = append(lastGroup.Phases, cfg)
	return b
}

// Build returns the constructed groups.
func (b *GroupBuilder) Build() []*PhaseGroup {
	return b.groups
}

// ExecutionPlan represents a planned order of phase execution.
type ExecutionPlan struct {
	Groups    []*PhaseGroup
	TotalTime int64 // estimated nanoseconds
}

// NewExecutionPlan creates an execution plan from phase groups.
func NewExecutionPlan(groups []*PhaseGroup) *ExecutionPlan {
	return &ExecutionPlan{
		Groups: groups,
	}
}

// PhaseNames returns all phase names in execution order.
func (p *ExecutionPlan) PhaseNames() []string {
	var names []string
	for _, group := range p.Groups {
		names = append(names, group.Names()...)
	}
	return names
}

// TotalPhases returns the total number of phases.
func (p *ExecutionPlan) TotalPhases() int {
	count := 0
	for _, group := range p.Groups {
		count += len(group.Phases)
	}
	return count
}

// ParallelPhases returns the number of phases that can run in parallel.
func (p *ExecutionPlan) ParallelPhases() int {
	count := 0
	for _, group := range p.Groups {
		if group.Parallel && len(group.Phases) > 1 {
			count += len(group.Phases)
		}
	}
	return count
}

// DependencyResolver resolves phase dependencies and creates an execution plan.
type DependencyResolver struct {
	phases  map[PhaseID]*PhaseConfig
	depends map[PhaseID][]PhaseID
}

// NewDependencyResolver creates a new dependency resolver.
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		phases:  make(map[PhaseID]*PhaseConfig),
		depends: make(map[PhaseID][]PhaseID),
	}
}

// AddPhase adds a phase with its dependencies.
func (r *DependencyResolver) AddPhase(id PhaseID, cfg *PhaseConfig) {
	r.phases[id] = cfg
	r.depends[id] = cfg.DependsOn
}

// Resolve creates an execution plan respecting dependencies.
// Uses topological sort to order phases.
func (r *DependencyResolver) Resolve() (*ExecutionPlan, error) {
	// Build the execution order using Kahn's algorithm
	inDegree := make(map[PhaseID]int)
	for id := range r.phases {
		inDegree[id] = 0
	}

	for _, deps := range r.depends {
		for _, dep := range deps {
			if _, ok := r.phases[dep]; ok {
				inDegree[dep]++
			}
		}
	}

	// Find phases with no dependencies
	var queue []PhaseID
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Process phases in dependency order
	var ordered []*PhaseConfig
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		cfg := r.phases[id]
		ordered = append(ordered, cfg)

		// Reduce in-degree of dependents
		for depID, deps := range r.depends {
			for _, dep := range deps {
				if dep == id {
					inDegree[depID]--
					if inDegree[depID] == 0 {
						queue = append(queue, depID)
					}
				}
			}
		}
	}

	// Group by priority
	groups := groupByPriority(ordered)
	return NewExecutionPlan(groups), nil
}

// groupByPriority groups phases by their priority level.
func groupByPriority(phases []*PhaseConfig) []*PhaseGroup {
	byPriority := make(map[PhasePriority][]*PhaseConfig)
	for _, cfg := range phases {
		byPriority[cfg.Priority] = append(byPriority[cfg.Priority], cfg)
	}

	priorities := make([]PhasePriority, 0, len(byPriority))
	for p := range byPriority {
		priorities = append(priorities, p)
	}

	// Sort priorities
	for i := 0; i < len(priorities)-1; i++ {
		for j := i + 1; j < len(priorities); j++ {
			if priorities[i] > priorities[j] {
				priorities[i], priorities[j] = priorities[j], priorities[i]
			}
		}
	}

	groups := make([]*PhaseGroup, 0, len(priorities))
	for _, priority := range priorities {
		cfgs := byPriority[priority]

		// Check if all phases can run in parallel
		canParallel := true
		for _, cfg := range cfgs {
			if !cfg.Parallel {
				canParallel = false
				break
			}
		}

		groups = append(groups, &PhaseGroup{
			Priority: priority,
			Phases:   cfgs,
			Parallel: canParallel,
		})
	}

	return groups
}
