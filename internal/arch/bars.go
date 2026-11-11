package arch

import (
	"fmt"
	"maps"
	"slices"
)

// Bars are what a change could loosen to let its own code pass: the model,
// the baseline and the waivers (IMPROVEMENTS.md §6.1).
type Bars struct {
	Model Model
	// Baseline is nil when there is none.
	Baseline *Baseline
	Waivers  []Waiver
}

// Loosened says how next loosens prev, each way a sentence, in a fixed
// order; empty when it loosens nothing. A change that loosens a bar and
// also changes the code measured against it lets itself pass, so the gate
// refuses it: a loosening has to be a change of its own, which a reviewer
// sees as one. Loosening means a higher threshold or limit, a lower floor,
// fewer rules or a weaker gate on one, more findings or worse metrics
// accepted by the baseline, and a waiver added or extended. Tightening
// never counts, and a change that rewrites a rule is judged only by the
// rule's count and gate.
func Loosened(prev, next *Bars) []string {
	return slices.Concat(
		looserThresholds(new(prev.Model.Limits()), new(next.Model.Limits())),
		looserFloors(&prev.Model.Gates, &next.Model.Gates),
		looserConnascence(&prev.Model.Connascence, &next.Model.Connascence),
		looserHistory(&prev.Model.Balance.History, &next.Model.Balance.History),
		fewerRules(&prev.Model, &next.Model),
		weakerGates(&prev.Model, &next.Model),
		looserBaseline(prev.Baseline, next.Baseline),
		looserWaivers(prev.Waivers, next.Waivers),
	)
}

// looserThresholds names each limit next raises.
func looserThresholds(prev, next *Thresholds) []string {
	var out []string
	for _, l := range []struct {
		name       string
		prev, next int
	}{
		{"largeTypeLines", prev.LargeTypeLines, next.LargeTypeLines},
		{"largeFuncLines", prev.LargeFuncLines, next.LargeFuncLines},
		{"largePackageTypes", prev.LargePackageTypes, next.LargePackageTypes},
		{"highComplexity", prev.HighComplexity, next.HighComplexity},
		{"maxTypeCycleGroup", prev.MaxTypeCycleGroup, next.MaxTypeCycleGroup},
		{"largeFileLines", prev.LargeFileLines, next.LargeFileLines},
		{"maxNesting", prev.MaxNesting, next.MaxNesting},
		{"highCognitive", prev.HighCognitive, next.HighCognitive},
	} {
		if l.next > l.prev {
			out = append(
				out,
				fmt.Sprintf("thresholds.%s raised from %d to %d", l.name, l.prev, l.next),
			)
		}
	}
	return out
}

// looserFloors names each floor next lowers.
func looserFloors(prev, next *Gates) []string {
	var out []string
	if next.MinCoverage < prev.MinCoverage {
		out = append(out, fmt.Sprintf("gates.minCoverage lowered from %g to %g",
			prev.MinCoverage, next.MinCoverage))
	}
	if next.MinCompliance < prev.MinCompliance {
		out = append(out, fmt.Sprintf("gates.minCompliance lowered from %g to %g",
			prev.MinCompliance, next.MinCompliance))
	}
	return out
}

// looserHistory names how next narrows the history window or raises the
// hotspot limit: a shorter window counts fewer commits toward every hotspot.
func looserHistory(prev, next *HistoryWindow) []string {
	if prev.Enabled() && !next.Enabled() {
		return []string{"balance.history removed"}
	}
	var out []string
	if next.Commits < prev.Commits {
		out = append(out, fmt.Sprintf("balance.history.commits lowered from %d to %d",
			prev.Commits, next.Commits))
	}
	if prev.MaxHotspot > 0 && (next.MaxHotspot == 0 || next.MaxHotspot > prev.MaxHotspot) {
		out = append(out, fmt.Sprintf("balance.history.maxHotspot raised or removed from %d",
			prev.MaxHotspot))
	}
	return out
}

// looserConnascence names each locality whose strength or degree limit next
// raises or removes, and the connascence findings leaving the gate.
func looserConnascence(prev, next *ConnascenceLimits) []string {
	var out []string
	if prev.Enabled() && !next.Enabled() {
		return []string{"connascence no longer gates"}
	}
	for l := LocalityFunction; l <= LocalityExternal; l++ {
		if next.StrengthLimit(l) > prev.StrengthLimit(l) {
			out = append(out, fmt.Sprintf("connascence.maxStrength.%s raised from %d to %d",
				l, prev.StrengthLimit(l), next.StrengthLimit(l)))
		}
		p, hadDegree := prev.DegreeLimit(l)
		n, hasDegree := next.DegreeLimit(l)
		if hadDegree && (!hasDegree || n > p) {
			out = append(out, fmt.Sprintf("connascence.maxDegree.%s raised or removed", l))
		}
	}
	return out
}

// fewerRules names each list of rules next shortens, and the fitness rules
// it removes, by name.
func fewerRules(prev, next *Model) []string {
	var out []string
	for _, l := range []struct {
		name       string
		prev, next int
	}{
		{"layers", len(prev.Layers), len(next.Layers)},
		{"subsystems", len(prev.Subsystems), len(next.Subsystems)},
		{"subsystemOrder", len(prev.SubsystemOrder), len(next.SubsystemOrder)},
		{"roles", len(prev.Roles), len(next.Roles)},
		{"forbiddenImports", len(prev.ForbiddenImports), len(next.ForbiddenImports)},
		{"isolations", len(prev.Isolations), len(next.Isolations)},
	} {
		if l.next < l.prev {
			out = append(out, fmt.Sprintf("%s shrank from %d to %d", l.name, l.prev, l.next))
		}
	}
	kept := map[string]bool{}
	for _, r := range next.Fitness.Rules() {
		kept[r.ModelRule()] = true
	}
	for _, r := range prev.Fitness.Rules() {
		if !kept[r.ModelRule()] {
			out = append(out, r.ModelRule()+" removed")
		}
	}
	if prev.Balance.Enabled() && !next.Balance.Enabled() {
		out = append(out, "balance no longer gates")
	}
	return out
}

// weakerGates names each rule whose gate next weakens: from fail to warn or
// off, or from warn to off.
func weakerGates(prev, next *Model) []string {
	var out []string
	weaker := func(name string, was, is RuleGate) {
		if gateStrength(is) < gateStrength(was) {
			out = append(out, fmt.Sprintf("%s gate weakened from %s to %s", name, was, is))
		}
	}
	for i := range min(len(prev.ForbiddenImports), len(next.ForbiddenImports)) {
		weaker(instanceName("forbiddenImports", i), prev.ForbiddenImports[i].Gate,
			next.ForbiddenImports[i].Gate)
	}
	for i := range min(len(prev.Isolations), len(next.Isolations)) {
		weaker(instanceName("isolations", i), prev.Isolations[i].Gate, next.Isolations[i].Gate)
	}
	gates := map[string]RuleGate{}
	for _, r := range next.Fitness.Rules() {
		gates[r.ModelRule()] = r.Gate
	}
	for _, r := range prev.Fitness.Rules() {
		if gate, ok := gates[r.ModelRule()]; ok {
			weaker(r.ModelRule(), r.Gate, gate)
		}
	}
	for _, rule := range slices.Sorted(maps.Keys(prev.FindingRules)) {
		was, is := prev.FindingRules[rule].Gate, next.FindingRules[rule].Gate
		if was != "" {
			weaker("findingRules."+string(rule), was, is)
		}
	}
	return out
}

// gateStrength ranks a gate: fail, the default, above warn above off.
func gateStrength(g RuleGate) int {
	switch g {
	case GateOff:
		return 0
	case GateWarn:
		return 1
	default:
		return 2
	}
}

// looserBaseline names the findings next's baseline accepts that prev's did
// not, by count, and each metric it records worse.
func looserBaseline(prev, next *Baseline) []string {
	if prev == nil || next == nil {
		return nil
	}
	var out []string
	had := map[string]bool{}
	for _, f := range prev.Findings {
		had[f.Fingerprint] = true
	}
	added := 0
	for _, f := range next.Findings {
		if !had[f.Fingerprint] {
			added++
		}
	}
	if added > 0 {
		out = append(
			out,
			fmt.Sprintf("the baseline accepts %s it did not", Plural(added, "finding")),
		)
	}
	_, worse := metricChanges(next.Metrics, prev.Metrics)
	for _, m := range worse {
		// Comparing next as the report against prev as the baseline, an
		// "improvement" of next over prev is a regression accepted.
		out = append(out, fmt.Sprintf("the baseline's %s loosened from %g to %g",
			m.Metric, m.Current, m.Baseline))
	}
	return out
}

// looserWaivers names each waiver next adds, and each it extends.
func looserWaivers(prev, next []Waiver) []string {
	expires := map[string]Date{}
	for _, w := range prev {
		expires[w.Fingerprint] = w.Expires
	}
	var out []string
	for _, w := range next {
		was, ok := expires[w.Fingerprint]
		switch {
		case !ok:
			out = append(out, "a waiver of "+w.Fingerprint+" added")
		case w.Expires > was:
			out = append(out, fmt.Sprintf("the waiver of %s extended from %s to %s",
				w.Fingerprint, was, w.Expires))
		}
	}
	return out
}
