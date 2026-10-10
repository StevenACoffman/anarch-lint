package arch

// ratio units of the criteria's raw values.
const (
	unitPercent = "%"
	unitRatio   = "ratio"
	unitCount   = "count"
)

// ruleResults holds the outcome of every percentage predicate, so the MMI
// criteria and the findings always agree.
type ruleResults struct {
	layers, subsystemOrder, iface, roles ruleResult
}

// criteriaOf computes the 22 MMI criteria of Table 4-1, in table order. The
// five reviewer criteria come back as unscored judgements; their slots fill
// them in later.
func criteriaOf(s *subject, z *Sizes, o *Ordering, r *ruleResults) []Criterion {
	var cs []Criterion
	cs = append(cs, modularityCriteria(s, z, r)...)
	cs = append(cs, proportionCriteria(s, z)...)
	cs = append(cs, hierarchyCriteria(s, o, r)...)
	cs = append(cs, patternCriteria(s, z, r)...)
	return cs
}

// modularityCriteria computes groups 1.1 and 1.2.
func modularityCriteria(s *subject, z *Sizes, r *ruleResults) []Criterion {
	subsystems, layers := len(s.m.Subsystems) > 0, len(s.m.Layers) > 0
	const noSubsystems, noLayers = "the model declares no subsystems", "the model declares no layers"
	iface := unmeasured(
		"1.2.1",
		"Subsystems have interfaces (% violating imports)",
		byArchitecture,
		noSubsystems,
	)
	if subsystems {
		iface = measured(iface.ID, iface.Name, byArchitecture,
			percent(r.iface.violations, r.iface.candidates), unitPercent)
	}
	return []Criterion{
		allocation("1.1.1", "Source code allocated to subsystems (domain modules)",
			subsystems, z.Subsystems, z.Lines, noSubsystems),
		allocation("1.1.2", "Source code allocated to technical layers",
			layers, z.Layers, z.Lines, noLayers),
		sizeRelationship(
			"1.1.3",
			"Size relationship of the subsystems",
			subsystems,
			z.Subsystems,
			noSubsystems,
		),
		sizeRelationship(
			"1.1.4",
			"Size relationship of the technical layers",
			layers,
			z.Layers,
			noLayers,
		),
		judgement("1.1.5", "Subsystems, layers, packages and types have clear responsibilities"),
		judgement("1.1.6", "Layers and subsystems are mapped through packages"),
		iface,
		judgement("1.2.2", "Internal interfaces are mapped through packages"),
	}
}

// allocation is criteria 1.1.1 and 1.1.2: the share of all code that some
// group of the model claims.
func allocation(
	id, name string,
	declared bool,
	groups []GroupSize,
	lines int,
	missing string,
) Criterion {
	if !declared {
		return unmeasured(id, name, byArchitecture, missing)
	}
	return measured(id, name, byArchitecture, percent(allocated(groups), lines), unitPercent)
}

// proportionCriteria computes group 1.3.
func proportionCriteria(s *subject, z *Sizes) []Criterion {
	return []Criterion{
		measured("1.3.1", "Source code in large types", byMetrics,
			percent(z.LinesInLargeTypes, z.Lines), unitPercent),
		measured("1.3.2", "Source code in large functions", byMetrics,
			percent(z.LinesInLargeFuncs, z.Lines), unitPercent),
		measured("1.3.3", "Types in large packages", byMetrics,
			percent(z.TypesInLargePackages, z.Types), unitPercent),
		measured("1.3.4", "Functions with high cyclomatic complexity", byMetrics,
			percent(z.ComplexFuncs, len(s.p.Funcs)), unitPercent),
	}
}

// hierarchyCriteria computes groups 2.1 and 2.2. In Go, 2.2.2 and 2.2.4 always
// score 10, because the compiler forbids package cycles (mmi.md §4); the
// report records that as an assumption rather than hiding it.
func hierarchyCriteria(s *subject, o *Ordering, r *ruleResults) []Criterion {
	var cs []Criterion
	if len(s.m.Layers) < 2 {
		cs = append(cs, unmeasured("2.1.1", "Violations of the technical layering", byArchitecture,
			"the model declares fewer than two layers"))
	} else {
		cs = append(cs, measured("2.1.1", "Violations of the technical layering", byArchitecture,
			percent(r.layers.violations, r.layers.candidates), unitPercent))
	}
	if len(s.m.SubsystemOrder) < 2 {
		cs = append(cs, unmeasured("2.1.2", "Violations of the subsystem layering", byArchitecture,
			"the model declares no subsystem order"))
	} else {
		cs = append(cs, measured("2.1.2", "Violations of the subsystem layering", byArchitecture,
			percent(r.subsystemOrder.violations, r.subsystemOrder.candidates), unitPercent))
	}
	return append(cs,
		measured("2.2.1", "Types in cycles", byMetrics,
			percent(o.Class.InCycles, o.Class.Elements), unitPercent),
		measured("2.2.2", "Packages in cycles", byMetrics,
			percent(o.Package.InCycles, o.Package.Elements), unitPercent),
		measured("2.2.3", "Types per cycle (largest cycle group)", byMetrics,
			float64(o.Class.MaxGroup), unitCount),
		measured("2.2.4", "Packages per cycle (largest cycle group)", byMetrics,
			float64(o.Package.MaxGroup), unitCount))
}

// patternCriteria computes group 3.
func patternCriteria(s *subject, z *Sizes, r *ruleResults) []Criterion {
	var cs []Criterion
	if len(s.m.Roles) == 0 {
		cs = append(cs, unmeasured("3.1", "Source code allocated to pattern roles", byArchitecture,
			"the model declares no roles"))
	} else {
		roles, lines := s.roles(), 0
		for i := range s.p.Types {
			if _, ok := roles[s.p.Types[i].ID]; ok {
				lines += s.p.Types[i].Lines
			}
		}
		cs = append(cs, measured("3.1", "Source code allocated to pattern roles", byArchitecture,
			percent(lines, z.Lines), unitPercent))
	}
	if len(s.m.Roles) < 2 {
		cs = append(cs, unmeasured("3.2", "References against the role order", byArchitecture,
			"the model declares fewer than two roles"))
	} else {
		cs = append(cs, measured("3.2", "References against the role order", byArchitecture,
			percent(r.roles.violations, r.roles.candidates), unitPercent))
	}
	return append(cs,
		judgement("3.3", "Explicit mapping of the patterns"),
		judgement("3.4", "Separation of domain and technical source code"))
}

// sizeRelationship is criteria 1.1.3 and 1.1.4: (largest ÷ smallest) ÷ the
// number of groups, the chapter's formula, over the groups that hold code.
func sizeRelationship(
	id, name string,
	declared bool,
	groups []GroupSize,
	missing string,
) Criterion {
	if !declared {
		return unmeasured(id, name, byMetrics, missing)
	}
	largest, smallest, n := 0, 0, 0
	for _, g := range groups {
		if g.Lines == 0 {
			continue
		}
		largest = max(largest, g.Lines)
		if n == 0 || g.Lines < smallest {
			smallest = g.Lines
		}
		n++
	}
	if n == 0 {
		return unmeasured(id, name, byMetrics, "no group contains code")
	}
	return measured(id, name, byMetrics, float64(largest)/float64(smallest)/float64(n), unitRatio)
}

// judgement returns an unscored reviewer criterion.
func judgement(id, name string) Criterion {
	return Criterion{
		ID: id, Name: name, Group: groupOf(id), Kind: Judgement, DeterminedBy: byReviewer,
		Reason: "needs a reviewer; see the judgement slot",
	}
}

// allocated sums the lines of the given groups.
func allocated(groups []GroupSize) int {
	n := 0
	for _, g := range groups {
		n += g.Lines
	}
	return n
}
