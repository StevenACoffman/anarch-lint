package arch

// Level holds the cycle measures of one abstraction level: types, packages or
// subsystems (mmi.md §4). The field names map to the dissertation's measures,
// with "Class", "Pckg" or "Subsys" in place of the level.
type Level struct {
	// Elements is the number of nodes at this level.
	Elements int `json:"elements"`
	// InCycles counts nodes in cycle groups: ClassesInCycles, PckgInCycles,
	// SubsysInCycles.
	InCycles int `json:"inCycles"`
	// Refs counts distinct directed edges inside cycle groups:
	// RefClassesInCycles at the type level.
	Refs int `json:"refs"`
	// Weight sums the weights of those edges. At the package and subsystem
	// levels this is the number of package imports, which is how
	// RefPckgInCycles and RefSubsysInCycles count.
	Weight int `json:"weight"`
	// Direct counts pairs inside one cycle group that reference each other:
	// DirectRefsInClassCycles and its siblings (Verflochtenheit).
	Direct int `json:"direct"`
	// MaxGroup is the size of the largest cycle group (Zyklenumfang).
	MaxGroup int `json:"maxGroup"`
	// Groups lists the cycle groups, as [Graph.CycleGroups] returns them.
	Groups [][]string `json:"groups"`
}

// Ordering holds the dissertation's thirteen ordering measures and cycle size
// (Geordnetheit, mmi.md §4).
//
// In Go, PckgInCycles, RefPckgInCycles, DirectRefsInPackageCycles and the four
// cycle-reach measures are always zero, because the compiler rejects import
// cycles. They are computed, not assumed: a non-zero value means the analysis
// is wrong.
type Ordering struct {
	Class     Level `json:"class"`
	Package   Level `json:"package"`
	Subsystem Level `json:"subsystem"`
	// PackageCycleGroupsFromClassCycles counts package cycle groups that
	// contain a cross-package reference from a type cycle.
	PackageCycleGroupsFromClassCycles int `json:"packageCycleGroupsFromClassCycles"`
	// SubsysCycleGroupsFromClassCycles is the same at the subsystem level.
	SubsysCycleGroupsFromClassCycles int `json:"subsysCycleGroupsFromClassCycles"`
	// RefsInClassCyclesBetweenPackages counts type-cycle references that
	// cross a package boundary.
	RefsInClassCyclesBetweenPackages int `json:"refsInClassCyclesBetweenPackages"`
	// RefsInClassCyclesBetweenSubsystems counts type-cycle references that
	// cross a subsystem boundary.
	RefsInClassCyclesBetweenSubsystems int `json:"refsInClassCyclesBetweenSubsystems"`
}

// cycleReach accumulates the cycle-reach measures over the edges of the type
// graph.
type cycleReach struct {
	packageOf, subsystemOf             map[string]string
	classGroup, pkgGroup, subGroup     map[string]int
	pkgHit, subHit                     map[int]bool
	betweenPackages, betweenSubsystems int
}

// LevelOf returns the cycle measures of g.
func LevelOf(g Graph) Level {
	l := Level{Elements: g.Len(), Groups: g.CycleGroups()}
	group := groupIndex(l.Groups)
	for _, grp := range l.Groups {
		l.InCycles += len(grp)
		l.MaxGroup = max(l.MaxGroup, len(grp))
	}
	for _, e := range g.Edges() {
		gf, okFrom := group[e.From]
		gt, okTo := group[e.To]
		if !okFrom || !okTo || gf != gt {
			continue
		}
		l.Refs++
		l.Weight += e.Weight
		if e.From < e.To && g.Weight(e.To, e.From) > 0 {
			l.Direct++
		}
	}
	return l
}

// newOrdering computes the ordering measures from the three graphs.
// packageOf maps each type to its package, and subsystemOf maps each package
// to its subsystem; an element missing from a map is its own container.
func newOrdering(
	types, packages, subsystems Graph,
	packageOf, subsystemOf map[string]string,
) Ordering {
	o := Ordering{Class: LevelOf(types), Package: LevelOf(packages), Subsystem: LevelOf(subsystems)}
	reach := cycleReach{
		packageOf:   packageOf,
		subsystemOf: subsystemOf,
		classGroup:  groupIndex(o.Class.Groups),
		pkgGroup:    groupIndex(o.Package.Groups),
		subGroup:    groupIndex(o.Subsystem.Groups),
		pkgHit:      map[int]bool{},
		subHit:      map[int]bool{},
	}
	for _, e := range types.Edges() {
		reach.add(e)
	}
	o.RefsInClassCyclesBetweenPackages = reach.betweenPackages
	o.RefsInClassCyclesBetweenSubsystems = reach.betweenSubsystems
	o.PackageCycleGroupsFromClassCycles = len(reach.pkgHit)
	o.SubsysCycleGroupsFromClassCycles = len(reach.subHit)
	return o
}

// add counts e toward cycle reach (Zyklenreichweite) when it lies inside a
// type cycle group and crosses a package or subsystem boundary.
func (r *cycleReach) add(e Edge) {
	gf, okFrom := r.classGroup[e.From]
	gt, okTo := r.classGroup[e.To]
	if !okFrom || !okTo || gf != gt {
		return
	}
	pf, pt := containerOf(r.packageOf, e.From), containerOf(r.packageOf, e.To)
	if pf != pt {
		r.betweenPackages++
		if g, ok := r.pkgGroup[pf]; ok {
			r.pkgHit[g] = true
		}
	}
	if sf, st := containerOf(r.subsystemOf, pf), containerOf(r.subsystemOf, pt); sf != st {
		r.betweenSubsystems++
		if g, ok := r.subGroup[sf]; ok {
			r.subHit[g] = true
		}
	}
}

// containerOf returns the container of k in m, or k itself when m has none.
func containerOf(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

// groupIndex maps every member of a cycle group to the group's position.
func groupIndex(groups [][]string) map[string]int {
	idx := map[string]int{}
	for i, grp := range groups {
		for _, n := range grp {
			idx[n] = i
		}
	}
	return idx
}
